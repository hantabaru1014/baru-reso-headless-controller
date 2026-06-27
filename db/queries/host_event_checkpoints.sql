-- name: GetHostEventCheckpoint :one
SELECT last_event_id FROM host_event_checkpoints WHERE host_id = $1;

-- name: UpsertHostEventCheckpoint :exec
INSERT INTO host_event_checkpoints (host_id, last_event_id)
VALUES ($1, $2)
ON CONFLICT (host_id)
DO UPDATE SET last_event_id = EXCLUDED.last_event_id;

-- name: DeleteHostEventCheckpoint :exec
DELETE FROM host_event_checkpoints WHERE host_id = $1;
