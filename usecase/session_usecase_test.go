package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/sessionstate"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHostDrainer struct {
	drainingIDs map[string]bool
}

func (s stubHostDrainer) IsHostDraining(hostID string) bool { return s.drainingIDs[hostID] }

// stubHostRepo only implements the GetRpcClient / GetGroupID methods that
// SessionUsecase.StartSession reaches after the drain guard. Every
// other method panics so accidental use shows up loudly.
type stubHostRepo struct {
	port.HeadlessHostRepository

	rpcClientErr error
	groupID      string
}

func (s *stubHostRepo) GetRpcClient(_ context.Context, _ string) (headlessv1.HeadlessControlServiceClient, error) {
	return nil, s.rpcClientErr
}

func (s *stubHostRepo) GetGroupID(_ context.Context, _ string) (string, error) {
	if s.groupID == "" {
		return "test-group", nil
	}

	return s.groupID, nil
}

// allowAllGroupMemberRepo は permission チェックを実質バイパスするための stub.
// ListUserSystemPermissions に system:group.manage を含めることで
// PermissionUsecase.HasPermission は normal-scope を全て true で返す.
type allowAllGroupMemberRepo struct {
	port.GroupMemberRepository
}

func (allowAllGroupMemberRepo) ListUserSystemPermissions(_ context.Context, _ string) ([]string, error) {
	return []string{entity.PermKey_SystemGroupManage}, nil
}

func (allowAllGroupMemberRepo) GetUserPermissionsForGroup(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func newUsecaseUnderTest(drainer stubHostDrainer, hostRepo port.HeadlessHostRepository) *SessionUsecase {
	return newUsecaseWithServerConfig(drainer, hostRepo, &config.ServerConfig{})
}

func newUsecaseWithServerConfig(drainer stubHostDrainer, hostRepo port.HeadlessHostRepository, serverCfg *config.ServerConfig) *SessionUsecase {
	permUC := NewPermissionUsecase(nil, allowAllGroupMemberRepo{}, nil)

	return NewSessionUsecase(
		nil, hostRepo,
		drainer,
		sessionstate.NewMemoryCache(),
		serverCfg,
		&config.ResoniteLinkConfig{TokenTTL: time.Minute},
		permUC,
	)
}

// actAsTestUser は permission チェックの caller を埋め込んだ ctx を返す.
func actAsTestUser(ctx context.Context) context.Context {
	return auth.WithActAsUser(ctx, "test-user")
}

// TestStartSession_RejectsDrainingHost guards the contract the upgrade
// orchestrator relies on: StartSession must refuse to land a new session
// on a host enrolled for drain, otherwise the host's user count would
// never reach zero and the restart would stall.
func TestStartSession_RejectsDrainingHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		hostID string
	}{
		{name: "the enrolled host", hostID: "draining-host"},
		{name: "an unrelated draining id", hostID: "some-other-host"},
	}

	draining := map[string]bool{
		"draining-host":   true,
		"some-other-host": true,
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			suc := newUsecaseUnderTest(stubHostDrainer{drainingIDs: draining}, &stubHostRepo{})

			_, err := suc.StartSession(actAsTestUser(t.Context()), tc.hostID, "test-group", nil,
				&headlessv1.WorldStartupParameters{}, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrHostDraining)
		})
	}
}

// TestStartSession_BypassesDrainCheckForUnrelatedHost verifies the drainer
// is consulted with the exact host id rather than broadly applied: a
// request for host B must NOT be rejected just because host A is draining.
// We arrange for the host repo to return a sentinel error after the drain
// guard so we can distinguish "drain guard fired" from "code progressed
// past it".
func TestStartSession_BypassesDrainCheckForUnrelatedHost(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("expected: stub host repo refused")
	suc := newUsecaseUnderTest(
		stubHostDrainer{drainingIDs: map[string]bool{"host-a": true}},
		&stubHostRepo{rpcClientErr: sentinel},
	)

	_, err := suc.StartSession(actAsTestUser(t.Context()), "host-b", "test-group", nil,
		&headlessv1.WorldStartupParameters{}, nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHostDraining,
		"draining host A must not affect StartSession on host B")
	require.ErrorIs(t, err, sentinel,
		"StartSession should have progressed past the drain guard to the host repo")
}


func newUsecaseWithPortConfig(portMin, portMax int, publicIP string) *SessionUsecase {
	return newUsecaseWithServerConfig(stubHostDrainer{}, &stubHostRepo{},
		&config.ServerConfig{SessionPortMin: portMin, SessionPortMax: portMax, PublicIP: publicIP})
}

func forcePortMap(params *headlessv1.WorldStartupParameters) map[headlessv1.NetworkProtocol]uint32 {
	ports := make(map[headlessv1.NetworkProtocol]uint32, len(params.GetForcePorts()))
	for _, forcePort := range params.GetForcePorts() {
		ports[forcePort.GetProtocol()] = forcePort.GetPort()
	}

	return ports
}

// TestWithAutoAssignedForcePorts は、プロトコルごとのポート指定 (force_ports) の
// 解決を検証する. QUIC は PublicIP が設定されている時だけ自動割り当てされる.
func TestWithAutoAssignedForcePorts(t *testing.T) {
	t.Parallel()

	const (
		portMin = 45000
		portMax = 45100
	)

	inRange := func(t *testing.T, port uint32) {
		t.Helper()
		assert.GreaterOrEqual(t, port, uint32(portMin))
		assert.LessOrEqual(t, port, uint32(portMax))
	}

	t.Run("ポート範囲未設定: 何も割り当てず params をそのまま返す", func(t *testing.T) {
		t.Parallel()

		suc := newUsecaseWithPortConfig(0, 0, "")
		params := &headlessv1.WorldStartupParameters{}

		got, err := suc.withAutoAssignedForcePorts(t.Context(), params)
		require.NoError(t, err)
		assert.Same(t, params, got)
	})

	t.Run("ポート範囲未設定: legacy な force_port は force_ports に変換される", func(t *testing.T) {
		t.Parallel()

		suc := newUsecaseWithPortConfig(0, 0, "")

		got, err := suc.withAutoAssignedForcePorts(t.Context(), &headlessv1.WorldStartupParameters{
			ForcePort: 12345,
		})
		require.NoError(t, err)
		assert.Equal(t, map[headlessv1.NetworkProtocol]uint32{
			headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL: 12345,
		}, forcePortMap(got))
	})

	t.Run("PublicIP 未設定: LNL のみ自動割り当てされる", func(t *testing.T) {
		t.Parallel()

		suc := newUsecaseWithPortConfig(portMin, portMax, "")

		got, err := suc.withAutoAssignedForcePorts(t.Context(), &headlessv1.WorldStartupParameters{})
		require.NoError(t, err)

		ports := forcePortMap(got)
		require.Len(t, ports, 1)
		inRange(t, ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL])
		// force_ports を解釈しない古いコンテナ向けに legacy フィールドも埋まる
		assert.Equal(t, ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL], got.GetForcePort()) //nolint:staticcheck // 後方互換の確認
	})

	t.Run("PublicIP 設定済み: LNL と QUIC に別々のポートが割り当てられる", func(t *testing.T) {
		t.Parallel()

		suc := newUsecaseWithPortConfig(portMin, portMax, "203.0.113.10")

		got, err := suc.withAutoAssignedForcePorts(t.Context(), &headlessv1.WorldStartupParameters{})
		require.NoError(t, err)

		ports := forcePortMap(got)
		require.Len(t, ports, 2)
		inRange(t, ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL])
		inRange(t, ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_QUIC])
		assert.NotEqual(t,
			ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL],
			ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_QUIC],
			"同じセッションの LNL と QUIC に同じポートを割り当ててはいけない")
	})

	t.Run("指定済みのポートは上書きされない", func(t *testing.T) {
		t.Parallel()

		suc := newUsecaseWithPortConfig(portMin, portMax, "203.0.113.10")

		got, err := suc.withAutoAssignedForcePorts(t.Context(), &headlessv1.WorldStartupParameters{
			ForcePorts: []*headlessv1.ForcePort{
				{Protocol: headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL, Port: 12345},
				{Protocol: headlessv1.NetworkProtocol_NETWORK_PROTOCOL_TCP, Port: 12346},
				// 範囲外のポートと未知のプロトコルは無視する
				{Protocol: headlessv1.NetworkProtocol_NETWORK_PROTOCOL_UNSPECIFIED, Port: 12347},
				{Protocol: headlessv1.NetworkProtocol_NETWORK_PROTOCOL_QUIC, Port: 70000},
			},
		})
		require.NoError(t, err)

		ports := forcePortMap(got)
		require.Len(t, ports, 3)
		assert.Equal(t, uint32(12345), ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_LNL])
		assert.Equal(t, uint32(12346), ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_TCP])
		inRange(t, ports[headlessv1.NetworkProtocol_NETWORK_PROTOCOL_QUIC])
	})
}
