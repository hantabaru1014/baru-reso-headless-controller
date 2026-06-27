package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHostDrainer struct {
	drainingIDs map[string]bool
}

func (s stubHostDrainer) IsHostDraining(hostID string) bool { return s.drainingIDs[hostID] }

// stubHostRepo only implements the GetRpcClient method that
// SessionUsecase.StartSession reaches after the drain guard. Every
// other method panics so accidental use shows up loudly.
type stubHostRepo struct {
	port.HeadlessHostRepository

	rpcClientErr error
}

func (s *stubHostRepo) GetRpcClient(_ context.Context, _ string) (headlessv1.HeadlessControlServiceClient, error) {
	return nil, s.rpcClientErr
}

func newUsecaseUnderTest(drainer stubHostDrainer, hostRepo port.HeadlessHostRepository) *SessionUsecase {
	return NewSessionUsecase(
		nil, hostRepo,
		drainer,
		&config.GRPCConfig{CallTimeout: time.Second},
		&config.ServerConfig{},
		&config.ResoniteLinkConfig{TokenTTL: time.Minute},
	)
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

			_, err := suc.StartSession(t.Context(), tc.hostID, nil,
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

	_, err := suc.StartSession(t.Context(), "host-b", nil,
		&headlessv1.WorldStartupParameters{}, nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrHostDraining,
		"draining host A must not affect StartSession on host B")
	require.ErrorIs(t, err, sentinel,
		"StartSession should have progressed past the drain guard to the host repo")
}

