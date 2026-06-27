// Package worker contains long-running background jobs that the server
// process supervises (Docker event listener, headless host event listener,
// periodic image checker, ...). Each job implements Runner; Manager owns
// their lifecycle and coordinates a graceful shutdown.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// Runner is a long-running background job.
//
// Run blocks until the supplied context is cancelled (graceful shutdown) or
// the runner hits a fatal, unrecoverable error. It should return
// context.Canceled or nil on clean shutdown; any other returned error is
// surfaced as a warning in the logs. Implementations are responsible for
// their own retry/backoff on transient errors.
type Runner interface {
	Name() string
	Run(ctx context.Context) error
}

// Manager supervises a fixed set of Runners.
//
// All Runners are started together by Start and stopped together by Stop.
// Stop respects the supplied context, so callers can bound the shutdown
// time.
type Manager struct {
	runners []Runner

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(runners []Runner) *Manager {
	return &Manager{runners: runners}
}

// Start launches every Runner in its own goroutine. It returns immediately;
// runners block in the background until Stop is called.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	for _, r := range m.runners {
		m.wg.Add(1)

		runner := r
		go m.runOne(ctx, runner)
	}

	slog.Info("worker manager started", "count", len(m.runners))
}

// Stop signals every Runner to shut down and waits for them to return.
// It honours the supplied context: if the context expires before all
// runners have finished, Stop returns ctx.Err() but the cancel signal has
// already been delivered so the runners will continue to wind down in the
// background.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()

	if cancel == nil {
		return nil
	}

	cancel()

	done := make(chan struct{})

	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("worker manager stopped")

		return nil
	case <-ctx.Done():
		slog.Warn("worker manager shutdown timed out; runners still draining in background")

		return ctx.Err()
	}
}

func (m *Manager) runOne(ctx context.Context, r Runner) {
	defer m.wg.Done()

	defer func() {
		if rv := recover(); rv != nil {
			slog.Error("worker panicked", "name", r.Name(), "panic", rv)
		}
	}()

	slog.Info("worker starting", "name", r.Name())

	err := r.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("worker exited with error", "name", r.Name(), "error", err)
	} else {
		slog.Info("worker stopped", "name", r.Name())
	}
}
