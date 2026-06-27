package notification

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBus_SubscribePublish(t *testing.T) {
	bus := NewBus()

	ch, cancel := bus.Subscribe(context.Background(), "user-1")
	defer cancel()

	ev := &hdlctrlv1.NotificationEvent{
		Payload: &hdlctrlv1.NotificationEvent_HostUpdated{
			HostUpdated: &hdlctrlv1.HostUpdatedEvent{HostId: "host-A"},
		},
	}
	bus.Publish(ev)

	select {
	case got := <-ch:
		require.NotNil(t, got)
		assert.Equal(t, "host-A", got.GetHostUpdated().GetHostId())
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestMemoryBus_MultipleSubscribersReceiveSameEvent(t *testing.T) {
	bus := NewBus()

	ch1, cancel1 := bus.Subscribe(context.Background(), "u1")
	defer cancel1()

	ch2, cancel2 := bus.Subscribe(context.Background(), "u2")
	defer cancel2()

	ev := &hdlctrlv1.NotificationEvent{}
	bus.Publish(ev)

	for _, ch := range []<-chan *hdlctrlv1.NotificationEvent{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive event")
		}
	}
}

func TestMemoryBus_CleanupRemovesSubscriber(t *testing.T) {
	bus := NewBus()
	ch, cancel := bus.Subscribe(context.Background(), "u1")

	cancel()

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after cleanup")

	// publishing afterwards must not panic and must not deliver.
	bus.Publish(&hdlctrlv1.NotificationEvent{})

	bus.mu.RLock()
	defer bus.mu.RUnlock()

	assert.Empty(t, bus.subs)
}

func TestMemoryBus_FullBufferDropsOldest(t *testing.T) {
	bus := NewBus()

	ch, cancel := bus.Subscribe(context.Background(), "u1")
	defer cancel()

	total := subscriberBufferSize + 5
	for i := range total {
		bus.Publish(&hdlctrlv1.NotificationEvent{Id: strconv.Itoa(i)})
	}

	got := make([]string, 0, subscriberBufferSize)

DRAIN:
	for {
		select {
		case ev := <-ch:
			got = append(got, ev.GetId())
		case <-time.After(50 * time.Millisecond):
			break DRAIN
		}
	}

	require.NotEmpty(t, got)
	assert.LessOrEqual(t, len(got), subscriberBufferSize, "buffer must not grow beyond capacity")

	for i := 1; i < len(got); i++ {
		prev, _ := strconv.Atoi(got[i-1])
		cur, _ := strconv.Atoi(got[i])
		assert.Less(t, prev, cur, "ids must remain ordered")
	}

	// 最後の id は drop されていないはず (drop-oldest semantics).
	assert.Equal(t, strconv.Itoa(total-1), got[len(got)-1])
}

func TestMemoryBus_PublishIsConcurrentSafe(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup

	subs := make([]func(), 0, 4)

	for range 4 {
		ch, cancel := bus.Subscribe(context.Background(), "u")
		subs = append(subs, cancel)

		wg.Go(func() {
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
				case <-time.After(200 * time.Millisecond):
					return
				}
			}
		})
	}

	var pubWg sync.WaitGroup
	for range 8 {
		pubWg.Go(func() {
			for range 50 {
				bus.Publish(&hdlctrlv1.NotificationEvent{})
			}
		})
	}

	pubWg.Wait()

	for _, c := range subs {
		c()
	}

	wg.Wait()
}
