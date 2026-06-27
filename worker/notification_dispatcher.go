package worker

import (
	"context"

	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"

	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

// NotificationDispatcher は container 由来の HostEvent を NotificationEvent に
// 変換して bus に publish する HostEventHandler. 1 つの HostEvent からは
// 1 つの NotificationEvent しか発行しない (フロント側の dispatch table が
// 1 つの event 種別を必要な invalidate に展開する).
type NotificationDispatcher struct {
	bus notification.Bus
}

func NewNotificationDispatcher(bus notification.Bus) *NotificationDispatcher {
	return &NotificationDispatcher{bus: bus}
}

var _ HostEventHandler = (*NotificationDispatcher)(nil)

func (d *NotificationDispatcher) HandleHostEvent(_ context.Context, hostID string, ev *headlessv1.HostEvent) {
	id := ev.GetId()
	occurredAt := ev.GetOccurredAt()

	switch p := ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionParametersChanged:
		d.bus.Publish(notification.SessionUpdated(p.SessionParametersChanged.GetSessionId(), hostID, id, occurredAt))
	case *headlessv1.HostEvent_WorldSaved:
		d.bus.Publish(notification.SessionUpdated(p.WorldSaved.GetSessionId(), hostID, id, occurredAt))
	case *headlessv1.HostEvent_UserJoinedSession:
		d.bus.Publish(notification.SessionUserChanged(
			p.UserJoinedSession.GetSessionId(), hostID, p.UserJoinedSession.GetUserName(), id,
			hdlctrlv1.SessionUserChangedEvent_KIND_JOINED, occurredAt,
		))
	case *headlessv1.HostEvent_UserLeftSession:
		d.bus.Publish(notification.SessionUserChanged(
			p.UserLeftSession.GetSessionId(), hostID, p.UserLeftSession.GetUserName(), id,
			hdlctrlv1.SessionUserChangedEvent_KIND_LEFT, occurredAt,
		))
	case *headlessv1.HostEvent_SessionStarted:
		d.bus.Publish(notification.SessionLifecycle(
			p.SessionStarted.GetSessionId(), hostID, id,
			hdlctrlv1.SessionLifecycleEvent_KIND_STARTED, occurredAt,
		))
	case *headlessv1.HostEvent_SessionEnded:
		d.bus.Publish(notification.SessionLifecycle(
			p.SessionEnded.GetSessionId(), hostID, id,
			hdlctrlv1.SessionLifecycleEvent_KIND_ENDED, occurredAt,
		))
	}
}

// HandleHostEventStreamReset は stream 切断/再接続時に呼ばれる. ホスト集合は
// 変わっていない (reset したホスト 1 台分の最新状態を再取得すれば足る) ため
// HostUpdated のみ publish する. session 系は SessionStateSyncHandler の
// resync 後に container 由来の publish が走る.
func (d *NotificationDispatcher) HandleHostEventStreamReset(_ context.Context, hostID string) {
	d.bus.Publish(notification.HostUpdated(hostID, "", nil))
}
