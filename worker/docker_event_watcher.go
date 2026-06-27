package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
	"github.com/jackc/pgx/v5/pgtype"
)

// shortContainerIDLen is the canonical Docker short-ID length used in log
// output.
const shortContainerIDLen = 12

// DockerEventWatcher reflects the Docker daemon's container event stream
// into the hosts table so we always converge on the real container state.
// It also performs a one-shot reconciliation on startup to fix any drift
// that happened while the controller was offline.
type DockerEventWatcher struct {
	dc  *hostconnector.DockerHostConnector
	q   *db.Queries
	bus notification.Bus

	reconnectDelay   time.Duration
	maxReconnectWait time.Duration
}

var _ Runner = (*DockerEventWatcher)(nil)

func NewDockerEventWatcher(
	dc *hostconnector.DockerHostConnector,
	q *db.Queries,
	bus notification.Bus,
	cfg *config.WorkerConfig,
) *DockerEventWatcher {
	return &DockerEventWatcher{
		dc:               dc,
		q:                q,
		bus:              bus,
		reconnectDelay:   cfg.EventReconnectDelay,
		maxReconnectWait: cfg.EventMaxReconnectWait,
	}
}

func (w *DockerEventWatcher) Name() string { return "docker-event-watcher" }

func (w *DockerEventWatcher) Run(ctx context.Context) error {
	w.syncAllStatuses(ctx)

	RetryWithBackoff(
		ctx,
		w.Name(),
		w.reconnectDelay,
		w.maxReconnectWait,
		stableConnectionThreshold,
		w.watchOnce,
	)

	return ctx.Err()
}

func (w *DockerEventWatcher) watchOnce(ctx context.Context) error {
	events, errChan, err := w.dc.SubscribeEvents(ctx)
	if err != nil {
		return err
	}

	slog.Info("connected to docker events stream")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return errors.New("docker events channel closed")
			}

			w.handleEvent(ctx, event)
		case err, ok := <-errChan:
			if !ok {
				return errors.New("docker events error channel closed")
			}

			if err != nil {
				return err
			}
		}
	}
}

func (w *DockerEventWatcher) syncAllStatuses(ctx context.Context) {
	slog.Info("syncing all container statuses on startup")

	statuses, err := w.dc.ListAllContainerStatuses(ctx)
	if err != nil {
		slog.Error("failed to list container statuses", "error", err)

		return
	}

	hosts, err := w.q.ListHosts(ctx)
	if err != nil {
		slog.Error("failed to list hosts", "error", err)

		return
	}

	for _, host := range hosts {
		containerID, _, err := hostconnector.ParseConnectString(hostconnector.HostConnectString(host.ConnectString))
		if err != nil {
			continue
		}

		var newStatus entity.HeadlessHostStatus
		if status, exists := statuses[containerID]; exists {
			newStatus = status
		} else {
			// Container is gone from Docker. If we still think it's
			// alive, mark it crashed; otherwise leave whatever terminal
			// state we already recorded alone.
			if host.Status == int32(entity.HeadlessHostStatus_RUNNING) ||
				host.Status == int32(entity.HeadlessHostStatus_STARTING) {
				newStatus = entity.HeadlessHostStatus_CRASHED
			} else {
				continue
			}
		}

		if host.Status == int32(newStatus) {
			continue
		}

		if err := w.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
			ID:     host.ID,
			Status: int32(newStatus),
		}); err != nil {
			slog.Error("failed to update host status", "hostID", host.ID, "error", err)

			continue
		}

		w.publishHostUpdated(host.ID)

		slog.Info("synced host status", "hostID", host.ID,
			"oldStatus", host.Status, "newStatus", newStatus)
	}
}

func (w *DockerEventWatcher) publishHostUpdated(hostID string) {
	w.bus.Publish(notification.HostUpdated(hostID, "", nil))
}

func (w *DockerEventWatcher) handleEvent(ctx context.Context, event hostconnector.ContainerEvent) {
	shortID := event.ContainerID
	if len(shortID) > shortContainerIDLen {
		shortID = shortID[:shortContainerIDLen]
	}

	slog.Debug("received container event", "containerID", shortID, "action", event.Action)

	host, err := w.q.GetHostByContainerID(ctx, pgtype.Text{String: event.ContainerID, Valid: true})
	if err != nil {
		return // not one of ours
	}

	newStatus := w.eventToStatus(event)
	if newStatus == entity.HeadlessHostStatus_UNKNOWN {
		return
	}

	if host.Status == int32(newStatus) {
		return
	}

	if err := w.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     host.ID,
		Status: int32(newStatus),
	}); err != nil {
		slog.Error("failed to update host status from event", "hostID", host.ID, "error", err)

		return
	}

	w.publishHostUpdated(host.ID)

	slog.Info("updated host status from docker event",
		"hostID", host.ID, "action", event.Action, "newStatus", newStatus)
}

func (w *DockerEventWatcher) eventToStatus(event hostconnector.ContainerEvent) entity.HeadlessHostStatus {
	switch event.Action {
	case hostconnector.ContainerEventStart, hostconnector.ContainerEventRestart:
		return entity.HeadlessHostStatus_RUNNING
	case hostconnector.ContainerEventStop:
		return entity.HeadlessHostStatus_EXITED
	case hostconnector.ContainerEventDie:
		if event.ExitCode == "0" {
			return entity.HeadlessHostStatus_EXITED
		}

		return entity.HeadlessHostStatus_CRASHED
	case hostconnector.ContainerEventKill, hostconnector.ContainerEventOOM:
		return entity.HeadlessHostStatus_CRASHED
	case hostconnector.ContainerEventDestroy:
		// Container removed — leave status alone, Delete will handle it.
		return entity.HeadlessHostStatus_UNKNOWN
	default:
		return entity.HeadlessHostStatus_UNKNOWN
	}
}

