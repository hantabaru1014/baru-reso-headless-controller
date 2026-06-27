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
	"sync"
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

// peer が無言で死んだ場合に gRPC stream を握り続けて
// 接続数カウントが減らなくなるのを防ぐための liveness 設定.
const (
	// pongWait は次の pong / メッセージを受信するまで待てる最大時間.
	pongWait = 60 * time.Second
	// pingPeriod は ping を送る周期. pongWait より十分短く.
	pingPeriod = (pongWait * 9) / 10 //nolint:mnd // 9/10: pong 到着前に次の ping を送るためのマージン
	// writeWait は 1 フレームの書き込みに許容するタイムアウト.
	writeWait = 10 * time.Second
)

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

	// token の有効期限を streamCtx の deadline に載せる. これで token 期限到達時に
	// gRPC stream と pumpFrames 配下の goroutine が ctx 経由で自然に終わり、
	// container 側の clients count もそのまま減る. ParseResoniteLinkToken は
	// jwt.WithExpirationRequired を強制しているので ExpiresAt は必ず非 nil.
	streamCtx, cancel := context.WithDeadline(r.Context(), claims.ExpiresAt.Time)
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

	if err := pumpFrames(streamCtx, cancel, conn, stream); err != nil {
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

// cancelStream はこの bridge に対応する gRPC streamCtx を cancel するためのもの.
// pumpFrames 内で片方の経路が終了したとき、もう片方が ReadMessage / stream.Recv で
// 永久ブロックしないよう、conn.Close と合わせて gRPC stream も起こす.
func pumpFrames(
	ctx context.Context,
	cancelStream context.CancelFunc,
	conn *websocket.Conn,
	stream headlessv1.HeadlessControlService_ResoniteLinkStreamClient,
) error {
	// peer が無言で死んだら ReadMessage を確実に起こすため
	// pongWait の deadline を設定し、pong / メッセージ受信のたびに延長する.
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// conn.WriteMessage / WriteControl は並行呼び出し不可なので writer をシリアライズする.
	var writeMu sync.Mutex

	writeFrame := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()

		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))

		return conn.WriteMessage(messageType, data)
	}

	// どこかの経路が終わったら conn を閉じて gRPC stream を cancel し、
	// 反対側の Read / Recv を起こす. conn.Close も CancelFunc も idempotent なので
	// 複数 goroutine の defer から呼ばれて問題ない. cancelStream は ctx も cancel するため
	// ping ticker は ctx.Done() で抜ける.
	closeAll := func() {
		_ = conn.Close()

		cancelStream()
	}

	var eg errgroup.Group

	// ping ticker: peer 生存確認.
	eg.Go(func() error {
		defer closeAll()

		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := writeFrame(websocket.PingMessage, nil); err != nil {
					return err
				}
			}
		}
	})

	// WebSocket -> gRPC
	eg.Go(func() error {
		defer closeAll()

		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					_ = stream.CloseSend()
					return nil
				}

				return err
			}

			// メッセージが届いている間は peer 生存なので deadline 延長.
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))

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
		defer closeAll()

		for {
			msg, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					_ = writeFrame(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					return nil
				}

				return err
			}

			switch p := msg.GetPayload().(type) {
			case *headlessv1.ResoniteLinkStreamResponse_TextFrame:
				if err := writeFrame(websocket.TextMessage, []byte(p.TextFrame)); err != nil {
					return err
				}
			case *headlessv1.ResoniteLinkStreamResponse_BinaryFrame:
				if err := writeFrame(websocket.BinaryMessage, p.BinaryFrame); err != nil {
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
