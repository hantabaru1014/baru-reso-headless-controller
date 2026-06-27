package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HostEventHandler consumes events streamed from a single headless host.
// Implementations should be cheap and non-blocking; long work belongs in a
// goroutine the handler owns.
//
// Delivery is at-least-once: handlers may see the same event more than
// once after a controller restart or stream reconnect, so they must be
// idempotent.
type HostEventHandler interface {
	HandleHostEvent(ctx context.Context, hostID string, event *headlessv1.HostEvent)
	// HandleHostEventStreamReset is invoked when the watcher receives an
	// OutOfRange response from a host, meaning the requested resume point
	// is no longer in the host's ring buffer. Handlers should treat this
	// as "your view of this host's state may be stale, do a full resync".
	HandleHostEventStreamReset(ctx context.Context, hostID string)
}

// HostEventWatcher maintains one server-streaming subscription per RUNNING
// headless host so the controller can react to events (user joined/left,
// session opened/closed, world saved, ...) without polling.
//
// The watcher periodically reconciles the desired set of streams with the
// set of RUNNING hosts in the DB: any host that transitions to RUNNING
// gets a new stream, any host that leaves RUNNING gets its stream
// cancelled. Each per-host stream owns its own backoff loop and persists
// the last delivered event id (a ULID, monotonic across container
// restarts) to the host_event_checkpoints table so a controller restart
// resumes from where it left off rather than dropping events that
// happened during downtime.
type HostEventWatcher struct {
	hostRepo port.HeadlessHostRepository
	store    HostEventStore
	handlers []HostEventHandler

	pollInterval     time.Duration
	reconnectDelay   time.Duration
	maxReconnectWait time.Duration

	mu      sync.Mutex
	streams map[string]*hostStreamCtl
}

type hostStreamCtl struct {
	cancel context.CancelFunc
	done   chan struct{}
}

var _ Runner = (*HostEventWatcher)(nil)

func NewHostEventWatcher(
	hostRepo port.HeadlessHostRepository,
	store HostEventStore,
	cfg *config.WorkerConfig,
	handlers []HostEventHandler,
) *HostEventWatcher {
	return &HostEventWatcher{
		hostRepo:         hostRepo,
		store:            store,
		handlers:         handlers,
		pollInterval:     cfg.HostEventPollInterval,
		reconnectDelay:   cfg.EventReconnectDelay,
		maxReconnectWait: cfg.EventMaxReconnectWait,
		streams:          make(map[string]*hostStreamCtl),
	}
}

func (w *HostEventWatcher) Name() string { return "host-event-watcher" }

func (w *HostEventWatcher) Run(ctx context.Context) error {
	w.reconcile(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.cancelAllAndWait()

			return ctx.Err()
		case <-ticker.C:
			w.reconcile(ctx)
		}
	}
}

// reconcile aligns the set of running per-host streams with the set of
// hosts that are currently RUNNING according to the DB.
func (w *HostEventWatcher) reconcile(ctx context.Context) {
	ids, err := w.store.ListRunningHostIDs(ctx)
	if err != nil {
		slog.Error("host-event-watcher: failed to list running hosts", "error", err)

		return
	}

	desired := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		desired[id] = struct{}{}
	}

	// Cancel streams for hosts that are no longer running and wait for
	// each goroutine to exit before forgetting it. Waiting prevents a
	// host that flaps RUNNING→stopped→RUNNING across two reconcile ticks
	// from racing two concurrent stream goroutines (with two checkpoint
	// writers and duplicate dispatch) for the same host.
	stopped := make([]*hostStreamCtl, 0)

	w.mu.Lock()

	for id, ctl := range w.streams {
		if _, ok := desired[id]; !ok {
			ctl.cancel()
			stopped = append(stopped, ctl)

			delete(w.streams, id)
		}
	}

	w.mu.Unlock()

	for _, ctl := range stopped {
		<-ctl.done
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Start streams for newly running hosts.
	for id := range desired {
		if _, ok := w.streams[id]; ok {
			continue
		}

		streamCtx, cancel := context.WithCancel(ctx)
		ctl := &hostStreamCtl{cancel: cancel, done: make(chan struct{})}
		w.streams[id] = ctl

		hostID := id

		go func() {
			defer close(ctl.done)

			w.runHostStream(streamCtx, hostID)
		}()
	}
}

// runHostStream keeps an open WatchHostEvents stream for the given host,
// reconnecting with exponential backoff on transient errors and persisting
// the resume token so buffered events survive disconnects and controller
// restarts alike. The checkpoint is read lazily here (not at startup) so a
// host that flaps RUNNING->stopped->RUNNING within one controller session
// still uses the latest persisted resume point.
func (w *HostEventWatcher) runHostStream(ctx context.Context, hostID string) {
	lastEventID, err := w.store.GetCheckpoint(ctx, hostID)
	if err != nil {
		slog.Error("host-event-watcher: failed to load checkpoint; starting live-only",
			"hostID", hostID, "error", err)

		lastEventID = ""
	}

	RetryWithBackoff(
		ctx,
		w.Name()+":"+hostID,
		w.reconnectDelay,
		w.maxReconnectWait,
		stableConnectionThreshold,
		func(ctx context.Context) error {
			return w.streamOnce(ctx, hostID, &lastEventID)
		},
	)
}

func (w *HostEventWatcher) streamOnce(ctx context.Context, hostID string, lastEventID *string) error {
	client, err := w.hostRepo.GetRpcClient(ctx, hostID)
	if err != nil {
		return err
	}

	stream, err := client.WatchHostEvents(ctx, &headlessv1.WatchHostEventsRequest{
		AfterEventId: *lastEventID,
	})
	if err != nil {
		return err
	}

	slog.Info("host event stream opened", "hostID", hostID, "afterEventID", *lastEventID)

	for {
		ev, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return io.EOF
			}

			if status.Code(err) == codes.OutOfRange {
				slog.Warn("host event stream reset; buffered events lost",
					"hostID", hostID, "afterEventID", *lastEventID)

				*lastEventID = ""

				w.deleteCheckpoint(ctx, hostID)
				w.notifyReset(ctx, hostID)

				return err
			}

			return err
		}

		w.dispatch(ctx, hostID, ev)
		*lastEventID = ev.GetId()
		w.persistCheckpoint(ctx, hostID, *lastEventID)
	}
}

func (w *HostEventWatcher) dispatch(ctx context.Context, hostID string, ev *headlessv1.HostEvent) {
	for _, h := range w.handlers {
		w.callHandler(ctx, hostID, ev, h)
	}
}

func (w *HostEventWatcher) notifyReset(ctx context.Context, hostID string) {
	for _, h := range w.handlers {
		w.callReset(ctx, hostID, h)
	}
}

// callHandler isolates each handler invocation with a panic boundary so a
// misbehaving handler cannot kill the per-host stream goroutine (which
// would also leak its entry in w.streams and permanently silence events
// for the host).
func (w *HostEventWatcher) callHandler(ctx context.Context, hostID string, ev *headlessv1.HostEvent, h HostEventHandler) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("host event handler panicked", "hostID", hostID, "panic", r)
		}
	}()

	h.HandleHostEvent(ctx, hostID, ev)
}

func (w *HostEventWatcher) callReset(ctx context.Context, hostID string, h HostEventHandler) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("host event reset handler panicked", "hostID", hostID, "panic", r)
		}
	}()

	h.HandleHostEventStreamReset(ctx, hostID)
}

func (w *HostEventWatcher) persistCheckpoint(ctx context.Context, hostID, eventID string) {
	if err := w.store.UpsertCheckpoint(ctx, hostID, eventID); err != nil && ctx.Err() == nil {
		slog.Error("host-event-watcher: failed to persist checkpoint",
			"hostID", hostID, "eventID", eventID, "error", err)
	}
}

// deleteCheckpoint forgets the persisted resume token for a host so a
// subsequent restart does not retry the same dead resume point. Used after
// an OutOfRange response.
func (w *HostEventWatcher) deleteCheckpoint(ctx context.Context, hostID string) {
	if err := w.store.DeleteCheckpoint(ctx, hostID); err != nil && ctx.Err() == nil {
		slog.Error("host-event-watcher: failed to delete checkpoint",
			"hostID", hostID, "error", err)
	}
}

// cancelAllAndWait stops every per-host stream and blocks until all
// goroutines have exited. Called on Manager shutdown.
func (w *HostEventWatcher) cancelAllAndWait() {
	w.mu.Lock()
	ctls := make([]*hostStreamCtl, 0, len(w.streams))

	for _, ctl := range w.streams {
		ctl.cancel()
		ctls = append(ctls, ctl)
	}

	w.streams = nil
	w.mu.Unlock()

	for _, ctl := range ctls {
		<-ctl.done
	}
}
