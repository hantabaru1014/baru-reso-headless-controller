package notification

import (
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 単一の発生事実に対して 1 つの NotificationEvent を作るファクトリ群.
// publisher (worker / rpc) はここを通すことで proto レイアウトの差し替え
// (id 付与方針, 追加フィールド) を 1 箇所で吸収できる. occurredAt が nil の
// 場合は now() を充てる.

func occurredOrNow(t *timestamppb.Timestamp) *timestamppb.Timestamp {
	if t != nil {
		return t
	}

	return timestamppb.Now()
}

func HostUpdated(hostID, id string, occurredAt *timestamppb.Timestamp) *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		Id:         id,
		OccurredAt: occurredOrNow(occurredAt),
		Payload: &hdlctrlv1.NotificationEvent_HostUpdated{
			HostUpdated: &hdlctrlv1.HostUpdatedEvent{HostId: hostID},
		},
	}
}

func HostListChanged() *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &hdlctrlv1.NotificationEvent_HostListChanged{
			HostListChanged: &hdlctrlv1.HostListChangedEvent{},
		},
	}
}

func SessionUpdated(sessionID, hostID, id string, occurredAt *timestamppb.Timestamp) *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		Id:         id,
		OccurredAt: occurredOrNow(occurredAt),
		Payload: &hdlctrlv1.NotificationEvent_SessionUpdated{
			SessionUpdated: &hdlctrlv1.SessionUpdatedEvent{
				SessionId: sessionID,
				HostId:    hostID,
			},
		},
	}
}

func SessionUserChanged(
	sessionID, hostID, userName, id string,
	kind hdlctrlv1.SessionUserChangedEvent_Kind,
	occurredAt *timestamppb.Timestamp,
) *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		Id:         id,
		OccurredAt: occurredOrNow(occurredAt),
		Payload: &hdlctrlv1.NotificationEvent_SessionUserChanged{
			SessionUserChanged: &hdlctrlv1.SessionUserChangedEvent{
				SessionId: sessionID,
				HostId:    hostID,
				Kind:      kind,
				UserName:  userName,
			},
		},
	}
}

func SessionLifecycle(
	sessionID, hostID, id string,
	kind hdlctrlv1.SessionLifecycleEvent_Kind,
	occurredAt *timestamppb.Timestamp,
) *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		Id:         id,
		OccurredAt: occurredOrNow(occurredAt),
		Payload: &hdlctrlv1.NotificationEvent_SessionLifecycle{
			SessionLifecycle: &hdlctrlv1.SessionLifecycleEvent{
				SessionId: sessionID,
				HostId:    hostID,
				Kind:      kind,
			},
		},
	}
}

// JobCompleted は非同期 job (ホスト/セッションの起動・停止等) の完了通知を作る.
// bus.PublishTo(createdBy, ...) で投入元の user にだけ届けるのが想定用法.
func JobCompleted(jobID, message string, level hdlctrlv1.JobCompletedEvent_Level) *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &hdlctrlv1.NotificationEvent_JobCompleted{
			JobCompleted: &hdlctrlv1.JobCompletedEvent{
				JobId:   jobID,
				Level:   level,
				Message: message,
			},
		},
	}
}

// KeepAlive はサーバが定期送出する no-op イベント. timestamp は呼び出し時刻.
func KeepAlive() *hdlctrlv1.NotificationEvent {
	return &hdlctrlv1.NotificationEvent{
		OccurredAt: timestamppb.Now(),
		Payload:    &hdlctrlv1.NotificationEvent_KeepAlive{KeepAlive: &hdlctrlv1.KeepAlive{}},
	}
}
