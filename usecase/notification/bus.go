// Package notification provides an in-memory pub/sub bus for delivering
// NotificationEvent to subscribed frontend clients via the
// NotificationService server-streaming RPC.
//
// The bus is intentionally process-local: there is no persistence and no
// fan-out across controller instances. If multi-instance deployment is
// added later, swap the implementation behind the Bus interface (e.g.
// Redis pub/sub) without touching publishers or the RPC handler.
package notification

import (
	"context"
	"log/slog"
	"sync"

	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

// Bus is the publish/subscribe boundary used by both event producers
// (workers, usecases) and the NotificationService RPC handler.
//
// Publish must never block on slow subscribers; see MemoryBus for the
// drop-oldest behavior.
type Bus interface {
	// Publish delivers ev to every active subscriber. It returns quickly
	// and does not retry slow subscribers.
	Publish(ev *hdlctrlv1.NotificationEvent)

	// PublishTo は userID に紐付いた subscriber にのみ ev を配信する。
	// 同じ user が複数タブ/デバイスから接続している場合は全 subscriber に届く。
	// 該当 subscriber が居なければ no-op (event は失われる)。
	// 非同期 job の完了 toast 等、リクエスト元 user にだけ届けたい用途で使う。
	PublishTo(userID string, ev *hdlctrlv1.NotificationEvent)

	// Subscribe registers a new subscriber and returns a receive-only
	// channel of events plus a cleanup function that the caller MUST
	// invoke (via defer) to release resources. The channel is closed by
	// the cleanup function.
	//
	// userID は PublishTo 経路の宛先キーとしても使う (空文字 = anonymous は
	// PublishTo の対象にならない).
	Subscribe(ctx context.Context, userID string) (<-chan *hdlctrlv1.NotificationEvent, func())
}

// subscriberBufferSize は 1 subscriber あたりの未読バッファ容量.
// burst (UserJoined 連続発生など) を吸収しつつ, 滞留しているクライアントを
// 検知できる程度の値.
const subscriberBufferSize = 64

// MemoryBus は process-local の Bus 実装.
type MemoryBus struct {
	mu     sync.RWMutex
	nextID uint64
	subs   map[uint64]*subscriber
}

type subscriber struct {
	userID string
	ch     chan *hdlctrlv1.NotificationEvent
}

// NewBus は新しい MemoryBus を返す.
func NewBus() *MemoryBus {
	return &MemoryBus{
		subs: make(map[uint64]*subscriber),
	}
}

// Publish delivers ev to every active subscriber. If a subscriber's
// buffer is full, the oldest pending event is dropped to make room.
// The channel is never closed by Publish (that would terminate the RPC
// stream prematurely).
func (b *MemoryBus) Publish(ev *hdlctrlv1.NotificationEvent) {
	if ev == nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for id, sub := range b.subs {
		busSend(id, sub, ev)
	}
}

// PublishTo は userID に一致する subscriber にのみ ev を配信する.
// 同じ user が複数 stream を張っている場合は全部に届ける.
func (b *MemoryBus) PublishTo(userID string, ev *hdlctrlv1.NotificationEvent) {
	if ev == nil || userID == "" {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for id, sub := range b.subs {
		if sub.userID != userID {
			continue
		}

		busSend(id, sub, ev)
	}
}

// Subscribe registers a subscriber. The returned cleanup function removes
// the subscriber from the bus and closes the event channel. Calling it
// more than once panics on the channel close — defer it exactly once.
func (b *MemoryBus) Subscribe(_ context.Context, userID string) (<-chan *hdlctrlv1.NotificationEvent, func()) {
	ch := make(chan *hdlctrlv1.NotificationEvent, subscriberBufferSize)
	sub := &subscriber{userID: userID, ch: ch}

	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subs[id] = sub
	b.mu.Unlock()

	cleanup := func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()

		close(ch)
	}

	return ch, cleanup
}

// busSend は単一 subscriber への配信ロジック (Publish/PublishTo 共通).
// バッファ満杯時は古い 1 件を drop して空きを作り再投入する.
// 呼び出し側で b.mu の RLock を取得済みであることを期待する.
func busSend(id uint64, sub *subscriber, ev *hdlctrlv1.NotificationEvent) {
	select {
	case sub.ch <- ev:
	default:
		// Publish はこのレシーバー以外から並行に同じチャネルへ書かないので,
		// drain 直後の Send は確実に通る.
		<-sub.ch

		sub.ch <- ev

		slog.Warn("notification bus: dropped oldest event for slow subscriber",
			"subscriberID", id, "userID", sub.userID)
	}
}
