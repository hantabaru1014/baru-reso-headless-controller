package worker

import (
	"context"
	"sync"
	"testing"

	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeBus struct {
	mu     sync.Mutex
	events []*hdlctrlv1.NotificationEvent
}

func (b *fakeBus) Publish(ev *hdlctrlv1.NotificationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, ev)
}

func (b *fakeBus) PublishTo(_ string, ev *hdlctrlv1.NotificationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, ev)
}

func (*fakeBus) Subscribe(_ context.Context, _ string) (<-chan *hdlctrlv1.NotificationEvent, func()) {
	ch := make(chan *hdlctrlv1.NotificationEvent)
	return ch, func() { close(ch) }
}

func (b *fakeBus) snapshot() []*hdlctrlv1.NotificationEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]*hdlctrlv1.NotificationEvent, len(b.events))
	copy(out, b.events)

	return out
}

func TestNotificationDispatcher_SessionParametersChanged(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		Id:         "ev1",
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_SessionParametersChanged{
			SessionParametersChanged: &headlessv1.SessionParametersChanged{
				SessionId: "sess-1",
			},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	su := evs[0].GetSessionUpdated()
	require.NotNil(t, su)
	assert.Equal(t, "sess-1", su.GetSessionId())
	assert.Equal(t, "host-A", su.GetHostId())
}

func TestNotificationDispatcher_WorldSaved(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_WorldSaved{
			WorldSaved: &headlessv1.WorldSaved{SessionId: "sess-1", WorldUrl: "u"},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	assert.Equal(t, "sess-1", evs[0].GetSessionUpdated().GetSessionId())
}

func TestNotificationDispatcher_UserJoinedSession(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_UserJoinedSession{
			UserJoinedSession: &headlessv1.UserJoinedSession{
				SessionId: "sess-1", UserId: "u-1", UserName: "alice",
			},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	uc := evs[0].GetSessionUserChanged()
	require.NotNil(t, uc)
	assert.Equal(t, hdlctrlv1.SessionUserChangedEvent_KIND_JOINED, uc.GetKind())
	assert.Equal(t, "alice", uc.GetUserName())
	assert.Equal(t, "sess-1", uc.GetSessionId())
	assert.Equal(t, "host-A", uc.GetHostId())
}

func TestNotificationDispatcher_UserLeftSession(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_UserLeftSession{
			UserLeftSession: &headlessv1.UserLeftSession{SessionId: "sess-1", UserName: "bob"},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	assert.Equal(t, hdlctrlv1.SessionUserChangedEvent_KIND_LEFT, evs[0].GetSessionUserChanged().GetKind())
}

func TestNotificationDispatcher_SessionStarted(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_SessionStarted{
			SessionStarted: &headlessv1.SessionStarted{SessionId: "sess-1"},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	assert.Equal(t, hdlctrlv1.SessionLifecycleEvent_KIND_STARTED, evs[0].GetSessionLifecycle().GetKind())
}

func TestNotificationDispatcher_SessionEnded(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &headlessv1.HostEvent_SessionEnded{
			SessionEnded: &headlessv1.SessionEnded{SessionId: "sess-1"},
		},
	})

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	assert.Equal(t, hdlctrlv1.SessionLifecycleEvent_KIND_ENDED, evs[0].GetSessionLifecycle().GetKind())
}

func TestNotificationDispatcher_StreamResetPublishesHostUpdated(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEventStreamReset(context.Background(), "host-A")

	evs := bus.snapshot()
	require.Len(t, evs, 1)
	assert.Equal(t, "host-A", evs[0].GetHostUpdated().GetHostId())
}

func TestNotificationDispatcher_UnhandledPayloadDoesNothing(t *testing.T) {
	bus := &fakeBus{}
	d := NewNotificationDispatcher(bus)

	d.HandleHostEvent(context.Background(), "host-A", &headlessv1.HostEvent{
		OccurredAt: timestamppb.Now(),
		// no payload
	})

	assert.Empty(t, bus.snapshot())
}
