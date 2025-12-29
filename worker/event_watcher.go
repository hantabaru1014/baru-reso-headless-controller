package worker

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventWatcher struct {
	dc     *hostconnector.DockerHostConnector
	q      *db.Queries
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Configuration
	reconnectDelay   time.Duration
	maxReconnectWait time.Duration
}

func NewEventWatcher(
	dc *hostconnector.DockerHostConnector,
	q *db.Queries,
) *EventWatcher {
	return &EventWatcher{
		dc:               dc,
		q:                q,
		reconnectDelay:   lib.GetEnvDuration("EVENT_WATCHER_RECONNECT_DELAY", 5*time.Second),
		maxReconnectWait: lib.GetEnvDuration("EVENT_WATCHER_MAX_RECONNECT_WAIT", 5*time.Minute),
	}
}

func (ew *EventWatcher) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	ew.cancel = cancel

	// Initial sync on startup
	ew.syncAllStatuses(ctx)

	// Start event listener with reconnection
	ew.wg.Add(1)
	go ew.watchEvents(ctx)

	slog.Debug("Event watcher started")
}

func (ew *EventWatcher) Stop() {
	if ew.cancel != nil {
		ew.cancel()
	}
	ew.wg.Wait()
	slog.Debug("Event watcher stopped")
}

func (ew *EventWatcher) syncAllStatuses(ctx context.Context) {
	slog.Info("Syncing all container statuses on startup")

	// Get all containers from Docker
	statuses, err := ew.dc.ListAllContainerStatuses(ctx)
	if err != nil {
		slog.Error("Failed to list container statuses", "error", err)
		return
	}

	// Get all hosts from DB
	hosts, err := ew.q.ListHosts(ctx)
	if err != nil {
		slog.Error("Failed to list hosts", "error", err)
		return
	}

	// Update each host status based on actual container status
	for _, host := range hosts {
		containerID := extractContainerID(host.ConnectString)
		if containerID == "" {
			continue
		}

		var newStatus entity.HeadlessHostStatus
		if status, exists := statuses[containerID]; exists {
			newStatus = status
		} else {
			// Container not found - mark as exited or crashed based on current DB status
			if host.Status == int32(entity.HeadlessHostStatus_RUNNING) ||
				host.Status == int32(entity.HeadlessHostStatus_STARTING) {
				newStatus = entity.HeadlessHostStatus_CRASHED
			} else {
				continue // No update needed
			}
		}

		if host.Status != int32(newStatus) {
			err := ew.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
				ID:     host.ID,
				Status: int32(newStatus),
			})
			if err != nil {
				slog.Error("Failed to update host status", "hostID", host.ID, "error", err)
			} else {
				slog.Info("Synced host status", "hostID", host.ID,
					"oldStatus", host.Status, "newStatus", newStatus)
			}
		}
	}
}

func (ew *EventWatcher) watchEvents(ctx context.Context) {
	defer ew.wg.Done()

	reconnectWait := ew.reconnectDelay

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		events, errChan, err := ew.dc.SubscribeEvents(ctx)
		if err != nil {
			slog.Error("Failed to subscribe to Docker events", "error", err)
			time.Sleep(reconnectWait)
			reconnectWait = min(reconnectWait*2, ew.maxReconnectWait)
			continue
		}

		// Reset reconnect delay on successful connection
		reconnectWait = ew.reconnectDelay
		slog.Info("Connected to Docker events stream")

		// Process events
	eventLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					// Channel closed, need to reconnect
					slog.Warn("Docker events channel closed, reconnecting...")
					break eventLoop
				}
				ew.handleEvent(ctx, event)
			case err := <-errChan:
				slog.Error("Docker events error, reconnecting...", "error", err)
				time.Sleep(reconnectWait)
				reconnectWait = min(reconnectWait*2, ew.maxReconnectWait)
				break eventLoop
			}
		}
	}
}

func (ew *EventWatcher) handleEvent(ctx context.Context, event hostconnector.ContainerEvent) {
	shortID := event.ContainerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	slog.Debug("Received container event",
		"containerID", shortID,
		"action", event.Action)

	// Find host by container ID
	host, err := ew.q.GetHostByContainerID(ctx, pgtype.Text{String: event.ContainerID, Valid: true})
	if err != nil {
		// Not our container, ignore
		return
	}

	newStatus := ew.eventToStatus(event)
	if newStatus == entity.HeadlessHostStatus_UNKNOWN {
		return
	}

	// Skip update if status hasn't changed
	if host.Status == int32(newStatus) {
		return
	}

	err = ew.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     host.ID,
		Status: int32(newStatus),
	})
	if err != nil {
		slog.Error("Failed to update host status from event",
			"hostID", host.ID, "error", err)
		return
	}

	slog.Info("Updated host status from Docker event",
		"hostID", host.ID,
		"action", event.Action,
		"newStatus", newStatus)
}

func (ew *EventWatcher) eventToStatus(event hostconnector.ContainerEvent) entity.HeadlessHostStatus {
	switch event.Action {
	case hostconnector.ContainerEventStart, hostconnector.ContainerEventRestart:
		return entity.HeadlessHostStatus_RUNNING
	case hostconnector.ContainerEventStop:
		return entity.HeadlessHostStatus_EXITED
	case hostconnector.ContainerEventDie:
		// Check exit code to determine if it crashed
		if event.ExitCode == "0" {
			return entity.HeadlessHostStatus_EXITED
		}
		return entity.HeadlessHostStatus_CRASHED
	case hostconnector.ContainerEventKill, hostconnector.ContainerEventOOM:
		return entity.HeadlessHostStatus_CRASHED
	case hostconnector.ContainerEventDestroy:
		// Container removed, don't change status (let Delete handle it)
		return entity.HeadlessHostStatus_UNKNOWN
	default:
		return entity.HeadlessHostStatus_UNKNOWN
	}
}

// extractContainerID extracts container ID from connect_string format "containerID:port"
func extractContainerID(connectString string) string {
	parts := strings.Split(connectString, ":")
	if len(parts) >= 1 && len(parts[0]) > 0 {
		return parts[0]
	}
	return ""
}
