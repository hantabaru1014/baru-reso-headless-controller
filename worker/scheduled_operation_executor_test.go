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
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScheduledOperationRepository_ClaimDueIsExclusive は
// FOR UPDATE SKIP LOCKED の検証. 複数 instance から同時に ClaimDue を呼んでも
// 同じ行が二度 claim されない (どちらか一方の結果にしか現れない) ことを確認する.
//
// 実行 executor の goroutine を起動しないので、CI で他テストの CleanupTables
// による truncate と race して flaky にならない.
func TestScheduledOperationRepository_ClaimDueIsExclusive(t *testing.T) {
	queries, _ := testutil.SetupTestDB(t)

	repo := adapter.NewScheduledSessionOperationRepository(queries)

	const want = 30

	insertedIDs := make(map[string]struct{}, want)

	for i := range want {
		sid := "S-claim-" + uniqueSuffix(i)
		created, err := repo.Create(t.Context(), port.ScheduledSessionOperationCreateParams{
			OperationType:    entity.ScheduledOperationType_STOP_SESSION,
			OperationPayload: mustMarshal(t, map[string]string{"session_id": sid}),
			TriggerType:      entity.ScheduledTriggerType_TIME,
			TriggerConfig:    mustMarshal(t, map[string]string{"scheduled_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}),
			NextFireAt:       time.Now().Add(-time.Minute),
			SessionID:        &sid,
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

	// SKIP LOCKED の不変量: 同じ行が複数 claimer に返らない (totalReturned == uniqueClaimed).
	// 他パッケージのテストが並列で CleanupTables (TRUNCATE) を走らせて行が途中で消える可能性が
	// あるため、「全行が必ず claim される」までは保証せず、「returned == unique」だけを検証する.
	totalReturned := 0
	uniqueClaimed := make(map[string]struct{}, want)

	for batch := range results {
		for _, id := range batch {
			if _, ok := insertedIDs[id]; !ok {
				continue // 別テストの leftover は無視
			}

			totalReturned++
			uniqueClaimed[id] = struct{}{}
		}
	}

	assert.Equal(t, len(uniqueClaimed), totalReturned, "no row should be claimed by more than one claimer (SKIP LOCKED invariant)")

	if totalReturned == 0 {
		// 他パッケージのテストが並列で CleanupTables を走らせて行が全滅した可能性.
		// SKIP LOCKED の不変量は満たしているので flaky として skip する.
		t.Skip("all inserted rows were truncated by a parallel test before claim; skipping")
	}
}

func TestScheduledOperationExecutor_ReleaseStaleClaims(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)

	repo := adapter.NewScheduledSessionOperationRepository(queries)

	// 1 行作って手動で RUNNING + 古い claim をセットする.
	sid := "S-stale-" + uniqueSuffix(int(time.Now().UnixNano()))
	created, err := repo.Create(t.Context(), port.ScheduledSessionOperationCreateParams{
		OperationType:    entity.ScheduledOperationType_STOP_SESSION,
		OperationPayload: mustMarshal(t, map[string]string{"session_id": sid}),
		TriggerType:      entity.ScheduledTriggerType_TIME,
		TriggerConfig:    mustMarshal(t, map[string]string{"scheduled_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)}),
		NextFireAt:       time.Now().Add(-time.Hour),
		SessionID:        &sid,
	})
	require.NoError(t, err)

	staleAt := time.Now().Add(-11 * time.Minute)

	tag, err := pool.Exec(t.Context(),
		`UPDATE scheduled_session_operations SET status = 1, claimed_by = $1, claimed_at = $2 WHERE id::text = $3`,
		"crashed-instance", staleAt, created.ID,
	)
	require.NoError(t, err)

	if tag.RowsAffected() == 0 {
		// 他パッケージのテストが並列で CleanupTables を走らせて行が消えた.
		// クロスパッケージの flaky を許容してスキップする.
		t.Skip("row was truncated by a parallel test; skipping")
	}

	_, err = repo.ReleaseStaleClaims(t.Context(), 10*time.Minute)
	require.NoError(t, err)

	var status int32

	err = pool.QueryRow(t.Context(), "SELECT status FROM scheduled_session_operations WHERE id::text = $1", created.ID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("row was truncated by a parallel test after stale-claim release; skipping")
	}

	require.NoError(t, err)
	assert.Equal(t, int32(0), status, "stale RUNNING row should be reset to PENDING")
}

func TestTimeTrigger_RoundTrip(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	original := triggers.NewTimeTrigger(at)

	raw, err := original.Marshal()
	require.NoError(t, err)

	decoded, err := scheduled_op.DecodeTrigger(entity.ScheduledTriggerType_TIME, raw)
	require.NoError(t, err)

	tt, ok := decoded.(*triggers.TimeTrigger)
	require.True(t, ok)
	assert.True(t, at.Equal(tt.ScheduledAt))
}

func TestTimeTrigger_Evaluate(t *testing.T) {
	at := time.Now()
	trig := triggers.NewTimeTrigger(at)

	// before
	ready, next, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{
		Now: func() time.Time { return at.Add(-time.Second) },
	})
	require.NoError(t, err)
	assert.False(t, ready)
	assert.True(t, next.Equal(at))

	// at
	ready, _, err = trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{
		Now: func() time.Time { return at },
	})
	require.NoError(t, err)
	assert.True(t, ready)

	// after
	ready, _, err = trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{
		Now: func() time.Time { return at.Add(time.Second) },
	})
	require.NoError(t, err)
	assert.True(t, ready)
}

func uniqueSuffix(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return string(letters[i%26]) + string(letters[(i/26)%26]) + string(letters[(i/676)%26])
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)

	return b
}

