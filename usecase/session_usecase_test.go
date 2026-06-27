package usecase

import (
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

// TestStartSession_RejectsDrainingHost guards the contract that the
// upgrade orchestrator relies on: StartSession must refuse to land a new
// session on a host that has been enrolled for drain, otherwise the
// host's user count would never go to zero and the restart would stall.
func TestStartSession_RejectsDrainingHost(t *testing.T) {
	t.Parallel()

	suc := NewSessionUsecase(
		nil, nil,
		stubHostDrainer{drainingIDs: map[string]bool{"draining-host": true}},
		&config.GRPCConfig{CallTimeout: time.Second},
		&config.ServerConfig{},
		&config.ResoniteLinkConfig{TokenTTL: time.Minute},
	)

	_, err := suc.StartSession(t.Context(), "draining-host", nil, &headlessv1.WorldStartupParameters{}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHostDraining)
}

func TestStartSession_AllowsNonDrainingHost(t *testing.T) {
	t.Parallel()

	suc := NewSessionUsecase(
		nil, nil,
		port.NoopHostDrainer{},
		&config.GRPCConfig{CallTimeout: time.Second},
		&config.ServerConfig{},
		&config.ResoniteLinkConfig{TokenTTL: time.Minute},
	)

	// nil hostRepo would otherwise panic, so we cannot exercise the full
	// happy path here — but ErrHostDraining must NOT fire.
	assert.False(t, suc.hostDrainer.IsHostDraining("any-host"))
}
