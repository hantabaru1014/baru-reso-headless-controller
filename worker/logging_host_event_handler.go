package worker

import (
	"context"
	"log/slog"

	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// LoggingHostEventHandler is the default HostEventHandler — it just logs
// each event. It exists so HostEventWatcher always has at least one
// consumer even before any "real" handlers are wired up.
type LoggingHostEventHandler struct{}

func NewLoggingHostEventHandler() *LoggingHostEventHandler {
	return &LoggingHostEventHandler{}
}

var _ HostEventHandler = (*LoggingHostEventHandler)(nil)

func (h *LoggingHostEventHandler) HandleHostEvent(_ context.Context, hostID string, ev *headlessv1.HostEvent) {
	slog.Info("host event received",
		"hostID", hostID,
		"eventID", ev.GetId(),
		"payload", payloadKind(ev),
	)
}

func (h *LoggingHostEventHandler) HandleHostEventStreamReset(_ context.Context, hostID string) {
	slog.Warn("host event stream was reset; downstream consumers should resync", "hostID", hostID)
}

func payloadKind(ev *headlessv1.HostEvent) string {
	switch ev.GetPayload().(type) {
	case *headlessv1.HostEvent_SessionStarted:
		return "session_started"
	case *headlessv1.HostEvent_SessionEnded:
		return "session_ended"
	case *headlessv1.HostEvent_UserJoinedSession:
		return "user_joined_session"
	case *headlessv1.HostEvent_UserLeftSession:
		return "user_left_session"
	case *headlessv1.HostEvent_WorldSaved:
		return "world_saved"
	case *headlessv1.HostEvent_SessionParametersChanged:
		return "session_parameters_changed"
	default:
		return "unknown"
	}
}
