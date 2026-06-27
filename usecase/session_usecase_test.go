package usecase

import (
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHostDrainer struct {
	drainingIDs map[string]bool
}

func (s stubHostDrainer) IsHostDraining(hostID string) bool { return s.drainingIDs[hostID] }

func newUsecaseUnderTest(drainer stubHostDrainer) *SessionUsecase {
	return NewSessionUsecase(
		nil, nil,
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

			suc := newUsecaseUnderTest(stubHostDrainer{drainingIDs: draining})

			_, err := suc.StartSession(t.Context(), tc.hostID, nil,
				&headlessv1.WorldStartupParameters{}, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrHostDraining)
		})
	}
}

// TestStartSession_BypassesDrainCheckForUnrelatedHost verifies the
// drainer is consulted with the exact host id (not broadly applied) by
// confirming that a request for host B is NOT rejected just because host
// A is draining. We use a nil hostRepo so any code path past the drain
// guard panics — we recover and assert nothing returned ErrHostDraining.
func TestStartSession_BypassesDrainCheckForUnrelatedHost(t *testing.T) {
	t.Parallel()

	suc := newUsecaseUnderTest(stubHostDrainer{drainingIDs: map[string]bool{"host-a": true}})

	defer func() {
		_ = recover()
	}()

	_, err := suc.StartSession(t.Context(), "host-b", nil,
		&headlessv1.WorldStartupParameters{}, nil)
	if err != nil {
		assert.NotErrorIs(t, err, ErrHostDraining,
			"draining host A must not affect StartSession on host B")
	}
}
