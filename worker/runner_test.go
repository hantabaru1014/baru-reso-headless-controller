package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	name      string
	runErr    error
	started   chan struct{}
	startOnce sync.Once
	ran       atomic.Int64
	blockFor  time.Duration
}

func (f *fakeRunner) Name() string { return f.name }

func (f *fakeRunner) Run(ctx context.Context) error {
	f.ran.Add(1)
	f.startOnce.Do(func() {
		if f.started != nil {
			close(f.started)
		}
	})

	if f.blockFor > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.blockFor):
		}
	} else {
		<-ctx.Done()
	}

	if f.runErr != nil {
		return f.runErr
	}

	return ctx.Err()
}

func TestManager_StartStopWaitsForAllRunners(t *testing.T) {
	t.Parallel()

	r1 := &fakeRunner{name: "r1", started: make(chan struct{})}
	r2 := &fakeRunner{name: "r2", started: make(chan struct{})}
	m := NewManager([]Runner{r1, r2})

	m.Start()

	<-r1.started
	<-r2.started

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, m.Stop(ctx))
	assert.Equal(t, int64(1), r1.ran.Load())
	assert.Equal(t, int64(1), r2.ran.Load())
}

func TestManager_StopIsIdempotent(t *testing.T) {
	t.Parallel()

	r := &fakeRunner{name: "r", started: make(chan struct{})}
	m := NewManager([]Runner{r})
	m.Start()
	<-r.started

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	require.NoError(t, m.Stop(ctx))
	require.NoError(t, m.Stop(ctx))
}

func TestManager_StopHonoursContextDeadline(t *testing.T) {
	t.Parallel()

	// Runner ignores cancellation, simulating a stuck worker.
	stuck := &stuckRunner{started: make(chan struct{}), released: make(chan struct{})}
	m := NewManager([]Runner{stuck})
	m.Start()
	<-stuck.started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := m.Stop(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	close(stuck.released) // unblock so the goroutine doesn't leak
}

type stuckRunner struct {
	startOnce sync.Once
	started   chan struct{}
	released  chan struct{}
}

func (s *stuckRunner) Name() string { return "stuck" }

func (s *stuckRunner) Run(_ context.Context) error {
	s.startOnce.Do(func() { close(s.started) })
	<-s.released

	return nil
}

func TestManager_RecoversFromPanic(t *testing.T) {
	t.Parallel()

	good := &fakeRunner{name: "good", started: make(chan struct{})}
	panicker := &panickyRunner{started: make(chan struct{})}
	m := NewManager([]Runner{panicker, good})
	m.Start()

	<-panicker.started
	<-good.started

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Manager should still shut down cleanly even though one runner panicked.
	require.NoError(t, m.Stop(ctx))
}

type panickyRunner struct {
	started chan struct{}
	once    sync.Once
}

func (p *panickyRunner) Name() string { return "panicky" }

func (p *panickyRunner) Run(_ context.Context) error {
	p.once.Do(func() { close(p.started) })

	panic(errors.New("boom"))
}
