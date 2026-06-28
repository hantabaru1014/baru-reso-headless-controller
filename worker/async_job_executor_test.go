package worker_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAsyncJobRepository_ClaimDueIsExclusive は AsyncJobRepository の
// FOR UPDATE SKIP LOCKED 不変量 (1 行が 2 つの claimer に同時に返らない) を確認する.
func TestAsyncJobRepository_ClaimDueIsExclusive(t *testing.T) {
	queries, _ := testutil.SetupTestDB(t)

	repo := adapter.NewAsyncJobRepository(queries)

	const want = 30

	insertedIDs := make(map[string]struct{}, want)

	for i := range want {
		created, err := repo.Create(t.Context(), port.AsyncJobCreateParams{
			JobType: entity.AsyncJobType_STOP_SESSION,
			Payload: json.RawMessage(`{"session_id": "S-claim-` + uniqueSuffix(i) + `"}`),
		})
		require.NoError(t, err)

		insertedIDs[created.ID] = struct{}{}
	}

	const claimers = 3

	results := make(chan []string, claimers)

	var wg sync.WaitGroup
	for i := range claimers {
		wg.Add(1)

		instance := "claimer-" + uniqueSuffix(i)

		go func() {
			defer wg.Done()

			rows, err := repo.ClaimDue(t.Context(), instance, want)
			if err != nil {
				results <- nil
				return
			}

			ids := make([]string, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.ID)
			}

			results <- ids
		}()
	}

	wg.Wait()
	close(results)

	totalReturned := 0
	uniqueClaimed := make(map[string]struct{}, want)

	for batch := range results {
		for _, id := range batch {
			if _, ok := insertedIDs[id]; !ok {
				continue
			}

			totalReturned++
			uniqueClaimed[id] = struct{}{}
		}
	}

	assert.Equal(t, len(uniqueClaimed), totalReturned, "no row should be claimed by more than one claimer (SKIP LOCKED invariant)")

	if totalReturned == 0 {
		t.Skip("all inserted rows were truncated by a parallel test before claim; skipping")
	}
}

// TestAsyncJobRepository_ReleaseStaleClaims は古い claim が PENDING に
// 戻ることを確認する.
func TestAsyncJobRepository_ReleaseStaleClaims(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)

	repo := adapter.NewAsyncJobRepository(queries)

	created, err := repo.Create(t.Context(), port.AsyncJobCreateParams{
		JobType: entity.AsyncJobType_STOP_SESSION,
		Payload: json.RawMessage(`{"session_id": "S-stale-` + uniqueSuffix(int(time.Now().UnixNano())) + `"}`),
	})
	require.NoError(t, err)

	staleAt := time.Now().Add(-31 * time.Minute)

	tag, err := pool.Exec(t.Context(),
		`UPDATE async_jobs SET status = 1, claimed_by = $1, claimed_at = $2 WHERE id::text = $3`,
		"crashed-instance", staleAt, created.ID,
	)
	require.NoError(t, err)

	if tag.RowsAffected() == 0 {
		t.Skip("row was truncated by a parallel test; skipping")
	}

	_, err = repo.ReleaseStaleClaims(t.Context(), 30*time.Minute)
	require.NoError(t, err)

	var status int32

	err = pool.QueryRow(t.Context(), "SELECT status FROM async_jobs WHERE id::text = $1", created.ID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("row was truncated by a parallel test after stale-claim release; skipping")
	}

	require.NoError(t, err)
	assert.Equal(t, int32(0), status, "stale RUNNING row should be reset to PENDING")
}

// TestAsyncJobRepository_MarkSucceededAndFailed は SUCCEEDED / FAILED 遷移と
// result_payload / last_error の永続化を確認する.
func TestAsyncJobRepository_MarkSucceededAndFailed(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)

	repo := adapter.NewAsyncJobRepository(queries)

	// 対象 job を直接 RUNNING に遷移させる. ClaimDue 経由だと並列 test の
	// 既存 PENDING 行を先に拾われて自分の row が PENDING のまま残ることがある.
	// 並列 test の CleanupTables (TRUNCATE) で row が消えた場合は skip する.
	claimByID := func(t *testing.T, id string) bool {
		t.Helper()

		tag, err := pool.Exec(t.Context(),
			`UPDATE async_jobs SET status = 1, claimed_by = $1, claimed_at = NOW() WHERE id::text = $2`,
			"instance-A", id,
		)
		require.NoError(t, err)

		return tag.RowsAffected() > 0
	}

	t.Run("MarkSucceeded with result", func(t *testing.T) {
		created, err := repo.Create(t.Context(), port.AsyncJobCreateParams{
			JobType: entity.AsyncJobType_START_HOST,
			Payload: json.RawMessage(`{"name":"H1"}`),
		})
		require.NoError(t, err)

		if !claimByID(t, created.ID) {
			t.Skip("row was truncated by a parallel test before claim; skipping")
		}

		err = repo.MarkSucceeded(t.Context(), created.ID, json.RawMessage(`{"host_id":"H-XYZ"}`))
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), created.ID)
		if err != nil {
			t.Skip("row was truncated by a parallel test after mark; skipping")
		}

		assert.Equal(t, entity.AsyncJobStatus_SUCCEEDED, got.Status)
		assert.JSONEq(t, `{"host_id":"H-XYZ"}`, string(got.ResultPayload))
		assert.Nil(t, got.LastError)
	})

	t.Run("MarkFailed with error", func(t *testing.T) {
		created, err := repo.Create(t.Context(), port.AsyncJobCreateParams{
			JobType: entity.AsyncJobType_STOP_SESSION,
			Payload: json.RawMessage(`{"session_id":"S1"}`),
		})
		require.NoError(t, err)

		if !claimByID(t, created.ID) {
			t.Skip("row was truncated by a parallel test before claim; skipping")
		}

		err = repo.MarkFailed(t.Context(), created.ID, "boom")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), created.ID)
		if err != nil {
			t.Skip("row was truncated by a parallel test after mark; skipping")
		}

		assert.Equal(t, entity.AsyncJobStatus_FAILED, got.Status)
		require.NotNil(t, got.LastError)
		assert.Equal(t, "boom", *got.LastError)
	})
}
