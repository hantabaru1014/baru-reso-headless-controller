// Package resonitelink は ResoniteLink WebSocket ブリッジを提供する.
//
// 外部の ResoniteLink クライアントからの WebSocket 接続を受け、
// 認証トークンを検証してから headless container の ResoniteLinkStream
// (gRPC bidi stream) に双方向で中継する.
package resonitelink

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"golang.org/x/sync/errgroup"
)

// WSPath は HTTP サーバが Bridge をマウントするパス.
// 発行する ws_path もこのパスを使う (mount 先と発行先のずれを防ぐため共有).
const WSPath = "/resonite-link/ws"

// BuildWSPath は token を埋めた path + query を返す.
// host は呼び出し側 (フロントエンド) で補完する.
func BuildWSPath(token string) string {
	u := url.URL{Path: WSPath, RawQuery: url.Values{"token": {token}}.Encode()}
	return u.String()
}

type Bridge struct {
	hhrepo         port.HeadlessHostRepository
	srepo          port.SessionRepository
	readyTimeout   time.Duration
	allowedOrigins []string
	upgrader       websocket.Upgrader
}

func NewBridge(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, cfg *config.ResoniteLinkConfig) *Bridge {
	b := &Bridge{
		hhrepo:         hhrepo,
		srepo:          srepo,
		readyTimeout:   cfg.ReadyTimeout,
		allowedOrigins: cfg.AllowedOrigins,
	}
	b.upgrader.CheckOrigin = b.checkOrigin

	return b
}

// ServeHTTP は WSPath にマウントする WebSocket ハンドラ.
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseResoniteLinkToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	sess, err := b.srepo.Get(r.Context(), claims.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}

		slog.Error("failed to get session", "session_id", claims.SessionID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client, err := b.hhrepo.GetRpcClient(streamCtx, sess.HostID)
	if err != nil {
		slog.Warn("failed to get headless host rpc client", "host_id", sess.HostID, "error", err)
		http.Error(w, "headless host unavailable", http.StatusBadGateway)

		return
	}

	stream, err := client.ResoniteLinkStream(streamCtx)
	if err != nil {
		slog.Warn("failed to open ResoniteLinkStream", "host_id", sess.HostID, "error", err)
		http.Error(w, "failed to open link stream", http.StatusBadGateway)

		return
	}

	initReq := &headlessv1.ResoniteLinkStreamRequest{
		Payload: &headlessv1.ResoniteLinkStreamRequest_Init{
			Init: &headlessv1.ResoniteLinkInit{SessionId: claims.SessionID},
		},
	}
	if err := stream.Send(initReq); err != nil {
		slog.Warn("failed to send ResoniteLinkInit", "session_id", claims.SessionID, "error", err)
		http.Error(w, "failed to initialize link stream", http.StatusBadGateway)

		return
	}

	if err := waitReady(stream, cancel, b.readyTimeout); err != nil {
		slog.Warn("ResoniteLinkReady not received", "session_id", claims.SessionID, "error", err)
		http.Error(w, "headless host did not become ready", http.StatusBadGateway)

		return
	}

	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader.Upgrade はエラー時に既に WriteHeader 済み.
		return
	}

	defer func() { _ = conn.Close() }()

	if err := pumpFrames(streamCtx, conn, stream); err != nil {
		slog.Debug("resonite link bridge ended", "session_id", claims.SessionID, "error", err)
	}
}

// waitReady は最初のレスポンスとして ResoniteLinkReady を期待する.
// 期限内に来なければ stream の親 ctx を cancel して Recv を起こし、
// goroutine リークを起こさずに返る.
func waitReady(stream headlessv1.HeadlessControlService_ResoniteLinkStreamClient, cancelStream context.CancelFunc, timeout time.Duration) error {
	timer := time.AfterFunc(timeout, cancelStream)
	defer timer.Stop()

	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	if _, ok := msg.GetPayload().(*headlessv1.ResoniteLinkStreamResponse_Ready); !ok {
		return errors.New("expected ResoniteLinkReady as first response")
	}

	return nil
}

func pumpFrames(
	ctx context.Context,
	conn *websocket.Conn,
	stream headlessv1.HeadlessControlService_ResoniteLinkStreamClient,
) error {
	eg, _ := errgroup.WithContext(ctx)

	// WebSocket -> gRPC
	eg.Go(func() error {
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					_ = stream.CloseSend()
					return nil
				}

				return err
			}

			var req *headlessv1.ResoniteLinkStreamRequest

			switch messageType {
			case websocket.TextMessage:
				req = &headlessv1.ResoniteLinkStreamRequest{Payload: &headlessv1.ResoniteLinkStreamRequest_TextFrame{TextFrame: string(data)}}
			case websocket.BinaryMessage:
				req = &headlessv1.ResoniteLinkStreamRequest{Payload: &headlessv1.ResoniteLinkStreamRequest_BinaryFrame{BinaryFrame: data}}
			default:
				continue
			}

			if err := stream.Send(req); err != nil {
				return err
			}
		}
	})

	// gRPC -> WebSocket
	eg.Go(func() error {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					return nil
				}

				return err
			}

			switch p := msg.GetPayload().(type) {
			case *headlessv1.ResoniteLinkStreamResponse_TextFrame:
				if err := conn.WriteMessage(websocket.TextMessage, []byte(p.TextFrame)); err != nil {
					return err
				}
			case *headlessv1.ResoniteLinkStreamResponse_BinaryFrame:
				if err := conn.WriteMessage(websocket.BinaryMessage, p.BinaryFrame); err != nil {
					return err
				}
			}
		}
	})

	return eg.Wait()
}

// checkOrigin は WebSocket upgrade の Origin チェック.
// allowedOrigins が空なら same-origin のみ (Origin の host と Host ヘッダが一致).
// "*" が含まれていれば全許可.
// それ以外は完全一致でマッチング.
func (b *Bridge) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if len(b.allowedOrigins) == 0 {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}

		return strings.EqualFold(u.Host, r.Host)
	}

	for _, allowed := range b.allowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}

	return false
}
