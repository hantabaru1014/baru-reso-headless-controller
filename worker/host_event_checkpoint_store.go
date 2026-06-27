package worker

import (
	"context"
	"errors"

	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/jackc/pgx/v5"
)

// HostEventStore is the persistence boundary HostEventWatcher uses to
// (a) discover which hosts it should currently be streaming events from
// and (b) persist per-host resume tokens so the watcher survives a
// controller restart.
//
// Get returns "" (with a nil error) when no checkpoint has been recorded
// for the host yet.
type HostEventStore interface {
	ListRunningHostIDs(ctx context.Context) ([]string, error)
	GetCheckpoint(ctx context.Context, hostID string) (string, error)
	UpsertCheckpoint(ctx context.Context, hostID, eventID string) error
	DeleteCheckpoint(ctx context.Context, hostID string) error
}

// SQLHostEventStore is the production implementation, backed by the
// hosts and host_event_checkpoints tables via sqlc.
type SQLHostEventStore struct {
	q *db.Queries
}

func NewSQLHostEventStore(q *db.Queries) *SQLHostEventStore {
	return &SQLHostEventStore{q: q}
}

var _ HostEventStore = (*SQLHostEventStore)(nil)

func (s *SQLHostEventStore) ListRunningHostIDs(ctx context.Context) ([]string, error) {
	hosts, err := s.q.ListHostsByStatus(ctx, int32(entity.HeadlessHostStatus_RUNNING))
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(hosts))
	for i, h := range hosts {
		ids[i] = h.ID
	}

	return ids, nil
}

func (s *SQLHostEventStore) GetCheckpoint(ctx context.Context, hostID string) (string, error) {
	id, err := s.q.GetHostEventCheckpoint(ctx, hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}

	return id, err
}

func (s *SQLHostEventStore) UpsertCheckpoint(ctx context.Context, hostID, eventID string) error {
	return s.q.UpsertHostEventCheckpoint(ctx, db.UpsertHostEventCheckpointParams{
		HostID:      hostID,
		LastEventID: eventID,
	})
}

func (s *SQLHostEventStore) DeleteCheckpoint(ctx context.Context, hostID string) error {
	return s.q.DeleteHostEventCheckpoint(ctx, hostID)
}
