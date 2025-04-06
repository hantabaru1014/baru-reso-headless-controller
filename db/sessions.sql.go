// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: sessions.sql

package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const deleteSession = `-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1
`

func (q *Queries) DeleteSession(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, deleteSession, id)
	return err
}

const getSession = `-- name: GetSession :one
SELECT id, name, status, started_at, ended_at, host_id, startup_parameters, auto_upgrade, created_at, updated_at FROM sessions WHERE id = $1 LIMIT 1
`

func (q *Queries) GetSession(ctx context.Context, id string) (Session, error) {
	row := q.db.QueryRow(ctx, getSession, id)
	var i Session
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Status,
		&i.StartedAt,
		&i.EndedAt,
		&i.HostID,
		&i.StartupParameters,
		&i.AutoUpgrade,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listSessions = `-- name: ListSessions :many
SELECT id, name, status, started_at, ended_at, host_id, startup_parameters, auto_upgrade, created_at, updated_at FROM sessions ORDER BY started_at DESC
`

func (q *Queries) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := q.db.Query(ctx, listSessions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Status,
			&i.StartedAt,
			&i.EndedAt,
			&i.HostID,
			&i.StartupParameters,
			&i.AutoUpgrade,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listSessionsByStatus = `-- name: ListSessionsByStatus :many
SELECT id, name, status, started_at, ended_at, host_id, startup_parameters, auto_upgrade, created_at, updated_at FROM sessions WHERE status = $1 ORDER BY started_at DESC
`

func (q *Queries) ListSessionsByStatus(ctx context.Context, status int32) ([]Session, error) {
	rows, err := q.db.Query(ctx, listSessionsByStatus, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Session
	for rows.Next() {
		var i Session
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Status,
			&i.StartedAt,
			&i.EndedAt,
			&i.HostID,
			&i.StartupParameters,
			&i.AutoUpgrade,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateSessionStatus = `-- name: UpdateSessionStatus :exec
UPDATE sessions SET status = $2 WHERE id = $1
`

type UpdateSessionStatusParams struct {
	ID     string
	Status int32
}

func (q *Queries) UpdateSessionStatus(ctx context.Context, arg UpdateSessionStatusParams) error {
	_, err := q.db.Exec(ctx, updateSessionStatus, arg.ID, arg.Status)
	return err
}

const upsertSession = `-- name: UpsertSession :one
INSERT INTO sessions (
    id,
    name,
    status,
    started_at,
    ended_at,
    host_id,
    startup_parameters,
    auto_upgrade
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    started_at = EXCLUDED.started_at,
    ended_at = EXCLUDED.ended_at,
    host_id = EXCLUDED.host_id,
    startup_parameters = EXCLUDED.startup_parameters,
    auto_upgrade = EXCLUDED.auto_upgrade
RETURNING id, name, status, started_at, ended_at, host_id, startup_parameters, auto_upgrade, created_at, updated_at
`

type UpsertSessionParams struct {
	ID                string
	Name              string
	Status            int32
	StartedAt         pgtype.Timestamptz
	EndedAt           pgtype.Timestamptz
	HostID            string
	StartupParameters []byte
	AutoUpgrade       bool
}

func (q *Queries) UpsertSession(ctx context.Context, arg UpsertSessionParams) (Session, error) {
	row := q.db.QueryRow(ctx, upsertSession,
		arg.ID,
		arg.Name,
		arg.Status,
		arg.StartedAt,
		arg.EndedAt,
		arg.HostID,
		arg.StartupParameters,
		arg.AutoUpgrade,
	)
	var i Session
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Status,
		&i.StartedAt,
		&i.EndedAt,
		&i.HostID,
		&i.StartupParameters,
		&i.AutoUpgrade,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}
