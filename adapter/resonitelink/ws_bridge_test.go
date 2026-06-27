package resonitelink

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const (
	testSessionID = "S-test-session"
	testHostID    = "H-test-host"
)

// --- fakes ---

type recvItem struct {
	msg *headlessv1.ResoniteLinkStreamResponse
	err error
}

type fakeStream struct {
	grpc.ClientStream
	ctx    context.Context //nolint:containedctx // テスト用 fake: gRPC stream の ctx cancel 挙動を模倣するため保持する
	sentCh chan *headlessv1.ResoniteLinkStreamRequest
	recvCh chan recvItem
}

func (f *fakeStream) Send(m *headlessv1.ResoniteLinkStreamRequest) error {
	select {
	case f.sentCh <- m:
		return nil
	case <-f.ctx.Done():
		return f.ctx.Err()
	}
}

func (f *fakeStream) Recv() (*headlessv1.ResoniteLinkStreamResponse, error) {
	select {
	case item, ok := <-f.recvCh:
		if !ok {
			return nil, io.EOF
		}

		return item.msg, item.err
	case <-f.ctx.Done():
		return nil, f.ctx.Err()
	}
}

func (f *fakeStream) CloseSend() error { return nil }

type fakeRPCClient struct {
	headlessv1.HeadlessControlServiceClient
	stream *fakeStream
}

func (c *fakeRPCClient) ResoniteLinkStream(ctx context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[headlessv1.ResoniteLinkStreamRequest, headlessv1.ResoniteLinkStreamResponse], error) {
	c.stream.ctx = ctx
	return c.stream, nil
}

type fakeHostRepo struct {
	port.HeadlessHostRepository
	client headlessv1.HeadlessControlServiceClient
}

func (r *fakeHostRepo) GetRpcClient(_ context.Context, _ string) (headlessv1.HeadlessControlServiceClient, error) {
	return r.client, nil
}

type fakeSessionRepo struct {
	port.SessionRepository
	sessions map[string]*entity.Session
}

func (r *fakeSessionRepo) Get(_ context.Context, id string) (*entity.Session, error) {
	s, ok := r.sessions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return s, nil
}

// --- helpers ---

type testBridge struct {
	server   *httptest.Server
	stream   *fakeStream
	sessRepo *fakeSessionRepo
}

func startTestBridge(t *testing.T) *testBridge {
	t.Helper()

	auth.Init("test-jwt-secret-for-testing")

	stream := &fakeStream{
		sentCh: make(chan *headlessv1.ResoniteLinkStreamRequest, 16),
		recvCh: make(chan recvItem, 16),
	}
	rpcClient := &fakeRPCClient{stream: stream}
	sessRepo := &fakeSessionRepo{sessions: map[string]*entity.Session{
		testSessionID: {ID: testSessionID, HostID: testHostID},
	}}
	hostRepo := &fakeHostRepo{client: rpcClient}
	bridge := NewBridge(hostRepo, sessRepo, &config.ResoniteLinkConfig{
		TokenTTL:       time.Minute,
		ReadyTimeout:   2 * time.Second,
		AllowedOrigins: []string{"*"},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", bridge.ServeHTTP)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &testBridge{server: ts, stream: stream, sessRepo: sessRepo}
}

func wsURL(server *httptest.Server, query string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/?" + query
}

func issueToken(t *testing.T, ttl time.Duration) string {
	t.Helper()

	token, _, err := auth.GenerateResoniteLinkToken("U-test", testSessionID, ttl)
	require.NoError(t, err)

	return token
}

// readyOnce pushes a single Ready response to be returned by the first Recv().
func readyOnce(stream *fakeStream) {
	stream.recvCh <- recvItem{msg: &headlessv1.ResoniteLinkStreamResponse{
		Payload: &headlessv1.ResoniteLinkStreamResponse_Ready{Ready: &headlessv1.ResoniteLinkReady{}},
	}}
}

// --- tests ---

func TestBridge_HappyPath_TextAndBinary(t *testing.T) {
	tb := startTestBridge(t)
	readyOnce(tb.stream)

	token := issueToken(t, time.Minute)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(token)), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()
	defer func() { _ = conn.Close() }()

	// First message sent on the gRPC stream must be the Init.
	select {
	case sent := <-tb.stream.sentCh:
		init, ok := sent.GetPayload().(*headlessv1.ResoniteLinkStreamRequest_Init)
		require.True(t, ok, "expected Init payload, got %T", sent.GetPayload())
		assert.Equal(t, testSessionID, init.Init.GetSessionId())
	case <-time.After(2 * time.Second):
		t.Fatal("Init was not forwarded to gRPC stream")
	}

	// WS -> gRPC : text frame
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"$type":"requestSessionData"}`)))

	select {
	case sent := <-tb.stream.sentCh:
		tf, ok := sent.GetPayload().(*headlessv1.ResoniteLinkStreamRequest_TextFrame)
		require.True(t, ok, "expected TextFrame payload, got %T", sent.GetPayload())
		assert.JSONEq(t, `{"$type":"requestSessionData"}`, tf.TextFrame)
	case <-time.After(2 * time.Second):
		t.Fatal("text frame was not forwarded")
	}

	// gRPC -> WS : text frame
	tb.stream.recvCh <- recvItem{msg: &headlessv1.ResoniteLinkStreamResponse{
		Payload: &headlessv1.ResoniteLinkStreamResponse_TextFrame{TextFrame: `{"$type":"sessionData","slots":[]}`},
	}}

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	typ, data, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, typ)
	assert.JSONEq(t, `{"$type":"sessionData","slots":[]}`, string(data))

	// WS -> gRPC : binary frame
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02, 0x03}))

	select {
	case sent := <-tb.stream.sentCh:
		bf, ok := sent.GetPayload().(*headlessv1.ResoniteLinkStreamRequest_BinaryFrame)
		require.True(t, ok, "expected BinaryFrame payload, got %T", sent.GetPayload())
		assert.Equal(t, []byte{0x01, 0x02, 0x03}, bf.BinaryFrame)
	case <-time.After(2 * time.Second):
		t.Fatal("binary frame was not forwarded")
	}

	// gRPC -> WS : binary frame
	tb.stream.recvCh <- recvItem{msg: &headlessv1.ResoniteLinkStreamResponse{
		Payload: &headlessv1.ResoniteLinkStreamResponse_BinaryFrame{BinaryFrame: []byte{0x10, 0x20, 0x30}},
	}}

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	typ, data, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, typ)
	assert.Equal(t, []byte{0x10, 0x20, 0x30}, data)
}

func TestBridge_NoToken_Returns401(t *testing.T) {
	tb := startTestBridge(t)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, ""), nil)
	require.Error(t, err)
	require.NotNil(t, resp)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestBridge_InvalidToken_Returns401(t *testing.T) {
	tb := startTestBridge(t)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token=not-a-jwt"), nil)
	require.Error(t, err)
	require.NotNil(t, resp)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestBridge_SessionNotFound_Returns404(t *testing.T) {
	tb := startTestBridge(t)
	delete(tb.sessRepo.sessions, testSessionID)

	token := issueToken(t, time.Minute)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(token)), nil)
	require.Error(t, err)
	require.NotNil(t, resp)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestBridge_WrongAudienceToken_Returns401(t *testing.T) {
	tb := startTestBridge(t)

	// 通常のアクセストークン (AuthClaims) を流用しようとしても弾かれること
	accessToken, err := auth.GenerateToken(auth.AuthClaims{UserID: "U-test"}, time.Minute)
	require.NoError(t, err)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(accessToken)), nil)
	require.Error(t, err)
	require.NotNil(t, resp)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// WebSocket クライアントが切断したら gRPC streamCtx も cancel され、
// container 側の Recv() が起きて clients count が減ることを保証する.
// (これが効かないと token 期限切れ後のゾンビ接続が container にも残る.)
func TestBridge_ClientClose_CancelsGrpcStream(t *testing.T) {
	tb := startTestBridge(t)
	readyOnce(tb.stream)

	token := issueToken(t, time.Minute)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(token)), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	// Init が gRPC stream に届くまで待って stream の ctx が確立されたことを担保.
	select {
	case <-tb.stream.sentCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Init was not forwarded to gRPC stream")
	}

	streamCtx := tb.stream.ctx

	// 正常クローズ送信. bridge は CloseSend してこちらの送信ループを抜ける.
	require.NoError(t, conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
	_ = conn.Close()

	// クライアント切断が伝播して gRPC streamCtx が cancel されることを確認.
	// (これが起きないと container 側の Recv が永久に起きず、bridge の clients count が減らない)
	select {
	case <-streamCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("gRPC stream ctx was not cancelled after WS close")
	}
}

// token 期限切れ到達で WS と gRPC stream の両方が閉じられることを保証する.
// (これがないと長時間 WS を維持された場合に期限切れ token のまま接続が居座り、
// container 側の clients count も減らない.)
func TestBridge_TokenExpiry_ClosesConnection(t *testing.T) {
	tb := startTestBridge(t)
	readyOnce(tb.stream)

	shortToken := issueToken(t, 2*time.Second)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(shortToken)), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()
	defer func() { _ = conn.Close() }()

	// Init が流れたことを確認して streamCtx を握っておく.
	select {
	case <-tb.stream.sentCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Init was not forwarded to gRPC stream")
	}

	streamCtx := tb.stream.ctx

	// token 有効期限到達で WS / gRPC stream が閉じられる. WS read が起きる.
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "WS read should fail after token expiry")

	select {
	case <-streamCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("gRPC stream ctx was not cancelled after token expiry")
	}
}

func TestBridge_ReadyTimeout_Returns502(t *testing.T) {
	tb := startTestBridge(t)
	// Ready を一切送らない -> readyTimeout 経過で 502
	token := issueToken(t, time.Minute)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL(tb.server, "token="+url.QueryEscape(token)), nil)
	require.Error(t, err)
	require.NotNil(t, resp)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}
