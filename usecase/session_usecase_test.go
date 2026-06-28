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
	permUC := NewPermissionUsecase(nil, allowAllGroupMemberRepo{}, nil)

	return NewSessionUsecase(
		nil, hostRepo,
		drainer,
		sessionstate.NewMemoryCache(),
		&config.ServerConfig{},
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

