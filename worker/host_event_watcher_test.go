package worker

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeHostRepo is a minimal stub of port.HeadlessHostRepository — only the
// methods HostEventWatcher actually uses are implemented; everything else
// panics so accidental usage shows up loudly.
type fakeHostRepo struct {
	port.HeadlessHostRepository

	mu      sync.Mutex
	hosts   entity.HeadlessHostList
	clients map[string]headlessv1.HeadlessControlServiceClient
	listErr error

	listCalls atomic.Int32
}

func (r *fakeHostRepo) ListAll(_ context.Context, _ port.HeadlessHostFetchOptions) (entity.HeadlessHostList, error) {
	r.listCalls.Add(1)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make(entity.HeadlessHostList, len(r.hosts))
	copy(out, r.hosts)

	return out, nil
}

func (r *fakeHostRepo) GetRpcClient(_ context.Context, id string) (headlessv1.HeadlessControlServiceClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.clients[id]
	if !ok {
		return nil, errors.New("no client for host: " + id)
	}

	return c, nil
}

func (r *fakeHostRepo) setHosts(hosts entity.HeadlessHostList) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hosts = hosts
}

// fakeClient implements only WatchHostEvents; everything else panics.
type fakeClient struct {
	headlessv1.HeadlessControlServiceClient

	mu    sync.Mutex
	calls []*headlessv1.WatchHostEventsRequest
	// streamFactory builds the stream returned by the Nth call.
	streamFactory func(call int, req *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error)
}

func (c *fakeClient) WatchHostEvents(ctx context.Context, in *headlessv1.WatchHostEventsRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
	c.mu.Lock()
	idx := len(c.calls)
	c.calls = append(c.calls, in)
	c.mu.Unlock()

	stream, err := c.streamFactory(idx, in)
	if err != nil {
		return nil, err
	}
	// Inject ctx into the fake stream so that cancelling the per-host
	// context (e.g. from reconcile) makes Recv return — same behaviour as
	// a real gRPC client.
	if fs, ok := stream.(*fakeStream); ok {
		fs.ctx = ctx
	}

	return stream, nil
}

// fakeStream implements grpc.ServerStreamingClient[headlessv1.HostEvent].
// The ctx is the per-call gRPC context, injected by fakeClient.WatchHostEvents;
// it lives inside the struct (containedctx lint exception) to mirror the
// real grpc client stream API.
//
//nolint:containedctx // mirrors grpc.ClientStream which also owns its ctx
type fakeStream struct {
	grpc.ClientStream

	ctx    context.Context
	events chan recvResult
}

type recvResult struct {
	event *headlessv1.HostEvent
	err   error
}

func (s *fakeStream) Recv() (*headlessv1.HostEvent, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case r, ok := <-s.events:
		if !ok {
			return nil, io.EOF
		}

		return r.event, r.err
	}
}

func (s *fakeStream) Header() (metadata.MD, error) { return nil, nil }
func (s *fakeStream) Trailer() metadata.MD         { return nil }
func (s *fakeStream) CloseSend() error             { return nil }
func (s *fakeStream) Context() context.Context     { return s.ctx }
func (s *fakeStream) SendMsg(any) error            { return nil }
func (s *fakeStream) RecvMsg(any) error            { return nil }

// recordingHandler captures everything dispatched to it for assertions.
type recordingHandler struct {
	mu     sync.Mutex
	events []*headlessv1.HostEvent
	resets []string
}

func (h *recordingHandler) HandleHostEvent(_ context.Context, _ string, ev *headlessv1.HostEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.events = append(h.events, ev)
}

func (h *recordingHandler) HandleHostEventStreamReset(_ context.Context, hostID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.resets = append(h.resets, hostID)
}

func (h *recordingHandler) snapshot() ([]*headlessv1.HostEvent, []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]*headlessv1.HostEvent(nil), h.events...), append([]string(nil), h.resets...)
}

func newRunningHost(id string) *entity.HeadlessHost {
	return &entity.HeadlessHost{ID: id, Status: entity.HeadlessHostStatus_RUNNING}
}

func newWatcher(repo port.HeadlessHostRepository, store HostEventStore, handlers []HostEventHandler) *HostEventWatcher {
	return NewHostEventWatcher(repo, store, &config.WorkerConfig{
		EventReconnectDelay:   5 * time.Millisecond,
		EventMaxReconnectWait: 20 * time.Millisecond,
		HostEventPollInterval: 10 * time.Millisecond,
	}, handlers)
}

// fakeHostEventStore is an in-memory HostEventStore for tests. The set of
// "running" host IDs is derived from the fakeHostRepo the test wires up
// (so callers don't have to keep two sources of truth in sync).
type fakeHostEventStore struct {
	repo *fakeHostRepo

	mu          sync.Mutex
	checkpoints map[string]string

	upsertCalls atomic.Int32
}

func newFakeHostEventStore(repo *fakeHostRepo) *fakeHostEventStore {
	return &fakeHostEventStore{
		repo:        repo,
		checkpoints: make(map[string]string),
	}
}

func (s *fakeHostEventStore) ListRunningHostIDs(_ context.Context) ([]string, error) {
	s.repo.mu.Lock()
	defer s.repo.mu.Unlock()

	if s.repo.listErr != nil {
		return nil, s.repo.listErr
	}

	ids := make([]string, 0, len(s.repo.hosts))
	for _, h := range s.repo.hosts {
		if h.Status == entity.HeadlessHostStatus_RUNNING {
			ids = append(ids, h.ID)
		}
	}

	return ids, nil
}

func (s *fakeHostEventStore) GetCheckpoint(_ context.Context, hostID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.checkpoints[hostID], nil
}

func (s *fakeHostEventStore) UpsertCheckpoint(_ context.Context, hostID, eventID string) error {
	s.upsertCalls.Add(1)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.checkpoints[hostID] = eventID

	return nil
}

func (s *fakeHostEventStore) DeleteCheckpoint(_ context.Context, hostID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.checkpoints, hostID)

	return nil
}

func (s *fakeHostEventStore) get(hostID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.checkpoints[hostID]
}

func (s *fakeHostEventStore) seed(hostID, eventID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.checkpoints[hostID] = eventID
}

func waitFor(t *testing.T, label string, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", label)
}

func TestHostEventWatcher_DispatchesEventsToHandlers(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 4)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	handler := &recordingHandler{}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{handler})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HONE"}}

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HTWO"}}

	waitFor(t, "two events dispatched", func() bool {
		evs, _ := handler.snapshot()

		return len(evs) >= 2
	})

	evs, _ := handler.snapshot()
	assert.Equal(t, "01HONE", evs[0].GetId())
	assert.Equal(t, "01HTWO", evs[1].GetId())
}

func TestHostEventWatcher_ResumesWithLastEventIDOnReconnect(t *testing.T) {
	t.Parallel()

	streams := []*fakeStream{
		{events: make(chan recvResult, 4)},
		{events: make(chan recvResult, 4)},
	}
	client := &fakeClient{streamFactory: func(idx int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		if idx >= len(streams) {
			// Block forever (well, until ctx cancels and the watcher stops).
			s := &fakeStream{events: make(chan recvResult)}

			return s, nil
		}

		return streams[idx], nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	handler := &recordingHandler{}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{handler})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	streams[0].events <- recvResult{event: &headlessv1.HostEvent{Id: "01HSEVEN"}}
	// Simulate the stream dropping.
	streams[0].events <- recvResult{err: errors.New("transient")}

	waitFor(t, "second WatchHostEvents call to happen", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 2
	})

	client.mu.Lock()
	secondCall := client.calls[1]
	client.mu.Unlock()

	assert.Equal(t, "01HSEVEN", secondCall.GetAfterEventId(),
		"after disconnect watcher must resume after the last delivered event id")
}

func TestHostEventWatcher_OutOfRangeResetsLastEventIDAndNotifiesHandlers(t *testing.T) {
	t.Parallel()

	streams := []*fakeStream{
		{events: make(chan recvResult, 4)},
		{events: make(chan recvResult, 4)},
	}
	client := &fakeClient{streamFactory: func(idx int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		if idx >= len(streams) {
			return &fakeStream{events: make(chan recvResult)}, nil
		}

		return streams[idx], nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	handler := &recordingHandler{}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{handler})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	streams[0].events <- recvResult{event: &headlessv1.HostEvent{Id: "01H42"}}

	streams[0].events <- recvResult{err: status.Error(codes.OutOfRange, "buffer overflow")}

	waitFor(t, "handler reset notification", func() bool {
		_, resets := handler.snapshot()

		return len(resets) >= 1
	})

	waitFor(t, "second WatchHostEvents call", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 2
	})

	client.mu.Lock()
	secondCall := client.calls[1]
	client.mu.Unlock()

	assert.Empty(t, secondCall.GetAfterEventId(),
		"OutOfRange must reset the resume token to empty (full resync)")

	_, resets := handler.snapshot()
	assert.Equal(t, []string{"host-1"}, resets)
}

func TestHostEventWatcher_CancelsStreamForRemovedHost(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 1)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	waitFor(t, "first stream subscription", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 1
	})

	// Remove the host from the desired set.
	repo.setHosts(nil)

	waitFor(t, "stream entry removed", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()

		_, still := w.streams["host-1"]

		return !still
	})
}

func TestHostEventWatcher_ShutdownCancelsAllStreams(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 1)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1"), newRunningHost("host-2")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client, "host-2": client},
	}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{&recordingHandler{}})

	ctx, cancel := context.WithCancel(context.Background())

	go func() { _ = w.Run(ctx) }()

	waitFor(t, "both streams started", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()

		return len(w.streams) == 2
	})

	cancel()

	waitFor(t, "all streams torn down", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()

		return len(w.streams) == 0
	})
}

func TestHostEventWatcher_NonRunningHostsAreIgnored(t *testing.T) {
	t.Parallel()

	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return &fakeStream{events: make(chan recvResult)}, nil
	}}
	repo := &fakeHostRepo{
		hosts: entity.HeadlessHostList{
			{ID: "stopped", Status: entity.HeadlessHostStatus_EXITED},
			{ID: "starting", Status: entity.HeadlessHostStatus_STARTING},
			newRunningHost("running"),
		},
		clients: map[string]headlessv1.HeadlessControlServiceClient{
			"stopped":  client,
			"starting": client,
			"running":  client,
		},
	}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	waitFor(t, "running host subscribed", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 1
	})

	// Give the watcher time to confirm it didn't also start streams for
	// non-running hosts.
	time.Sleep(50 * time.Millisecond)

	w.mu.Lock()
	defer w.mu.Unlock()

	require.Len(t, w.streams, 1)
	_, ok := w.streams["running"]
	assert.True(t, ok)
}

func TestHostEventWatcher_ResumesFromPersistedCheckpointOnStartup(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 1)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	store := newFakeHostEventStore(repo)
	store.seed("host-1", "01HABC")

	w := newWatcher(repo, store, []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	waitFor(t, "first WatchHostEvents call", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 1
	})

	client.mu.Lock()
	firstCall := client.calls[0]
	client.mu.Unlock()

	assert.Equal(t, "01HABC", firstCall.GetAfterEventId(),
		"watcher must seed each new stream with the persisted checkpoint")
}

func TestHostEventWatcher_PersistsCheckpointAfterEachEvent(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 2)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	store := newFakeHostEventStore(repo)
	w := newWatcher(repo, store, []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HFIRST"}}

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HSECOND"}}

	waitFor(t, "checkpoint advances to second event", func() bool {
		return store.get("host-1") == "01HSECOND"
	})

	assert.GreaterOrEqual(t, store.upsertCalls.Load(), int32(2),
		"watcher must persist a checkpoint after every dispatched event")
}

func TestHostEventWatcher_HandlerPanicDoesNotKillStream(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 4)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	panicker := &panickingHandler{}
	recorder := &recordingHandler{}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{panicker, recorder})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HONE"}}

	stream.events <- recvResult{event: &headlessv1.HostEvent{Id: "01HTWO"}}

	waitFor(t, "both events reached the non-panicking handler", func() bool {
		evs, _ := recorder.snapshot()

		return len(evs) >= 2
	})
}

type panickingHandler struct{}

func (h *panickingHandler) HandleHostEvent(_ context.Context, _ string, _ *headlessv1.HostEvent) {
	panic("simulated handler bug")
}

func (h *panickingHandler) HandleHostEventStreamReset(_ context.Context, _ string) {}

func TestHostEventWatcher_FlappingHostDoesNotSpawnDuplicateStream(t *testing.T) {
	t.Parallel()

	stream := &fakeStream{events: make(chan recvResult, 1)}
	client := &fakeClient{streamFactory: func(_ int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		return stream, nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	w := newWatcher(repo, newFakeHostEventStore(repo), []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	waitFor(t, "first stream subscription", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 1
	})

	// Capture the first ctl so we can later assert its goroutine exited.
	w.mu.Lock()
	firstCtl := w.streams["host-1"]
	w.mu.Unlock()

	// Flap: stop the host, let reconcile observe it, then bring it back.
	repo.setHosts(nil)
	waitFor(t, "stream torn down on stop", func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()

		_, still := w.streams["host-1"]

		return !still
	})

	select {
	case <-firstCtl.done:
	case <-time.After(2 * time.Second):
		t.Fatal("first stream goroutine did not exit when host went non-RUNNING")
	}

	repo.setHosts(entity.HeadlessHostList{newRunningHost("host-1")})
	waitFor(t, "second stream subscription", func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()

		return len(client.calls) >= 2
	})

	w.mu.Lock()
	require.Len(t, w.streams, 1, "exactly one stream goroutine must be active after the flap")
	w.mu.Unlock()
}

func TestHostEventWatcher_OutOfRangeClearsPersistedCheckpoint(t *testing.T) {
	t.Parallel()

	streams := []*fakeStream{
		{events: make(chan recvResult, 1)},
		{events: make(chan recvResult, 1)},
	}
	client := &fakeClient{streamFactory: func(idx int, _ *headlessv1.WatchHostEventsRequest) (grpc.ServerStreamingClient[headlessv1.HostEvent], error) {
		if idx >= len(streams) {
			return &fakeStream{events: make(chan recvResult)}, nil
		}

		return streams[idx], nil
	}}
	repo := &fakeHostRepo{
		hosts:   entity.HeadlessHostList{newRunningHost("host-1")},
		clients: map[string]headlessv1.HeadlessControlServiceClient{"host-1": client},
	}
	store := newFakeHostEventStore(repo)
	store.seed("host-1", "01HOLD")

	w := newWatcher(repo, store, []HostEventHandler{&recordingHandler{}})

	ctx := t.Context()

	go func() { _ = w.Run(ctx) }()

	streams[0].events <- recvResult{err: status.Error(codes.OutOfRange, "buffer overflow")}

	waitFor(t, "checkpoint cleared", func() bool {
		return store.get("host-1") == ""
	})
}
