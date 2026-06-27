package worker_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSessionOperator は executor が呼ぶ SessionOperator のスパイ.
// scheduled_op.SessionOperator を満たす.
type fakeSessionOperator struct {
	mu        sync.Mutex
	stopCalls []string
	stopErr   error
}

func (f *fakeSessionOperator) StartSession(_ context.Context, _ string, _ *string, _ *headlessv1.WorldStartupParameters, _ *string) (*entity.Session, error) {
	return nil, nil
}

func (f *fakeSessionOperator) StopSession(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stopCalls = append(f.stopCalls, sessionID)

	return f.stopErr
}

func (f *fakeSessionOperator) UpdateSessionParameters(_ context.Context, _ string, _ *headlessv1.UpdateSessionParametersRequest) error {
	return nil
}

func (f *fakeSessionOperator) UpdateSessionExtraSettings(_ context.Context, _ string, _ *bool, _ *string) error {
	return nil
}

func (f *fakeSessionOperator) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, len(f.stopCalls))
	copy(out, f.stopCalls)

	return out
}

func TestScheduledOperationExecutor_ConcurrentInstancesNoDoubleExec(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewScheduledSessionOperationRepository(queries)

	// 30 行を due 状態で仕込む. 全テスト同時実行下でも 30s 以内に終わることを期待.
	want := 30
	for i := 0; i < want; i++ {
		sid := "S-" + uniqueSuffix(i)
		_, err := repo.Create(t.Context(), port.ScheduledSessionOperationCreateParams{
			OperationType:    entity.ScheduledOperationType_STOP_SESSION,
			OperationPayload: mustMarshal(t, map[string]string{"session_id": sid}),
			TriggerType:      entity.ScheduledTriggerType_TIME,
			TriggerConfig:    mustMarshal(t, map[string]string{"scheduled_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}),
			NextFireAt:       time.Now().Add(-time.Minute),
			SessionID:        &sid,
		})
		require.NoError(t, err)
	}

	op := &fakeSessionOperator{}

	makeExec := func(id string) *worker.ScheduledOperationExecutor {
		return worker.NewScheduledOperationExecutor(repo, op, nil, nil, worker.ScheduledOperationExecutorOptions{
			InstanceID:   id,
			TickInterval: 50 * time.Millisecond,
			BatchSize:    32,
			Concurrency:  8,
			StaleSweep:   1 * time.Hour,
			StaleAfter:   1 * time.Hour,
		})
	}

	ctx, cancel := context.WithCancel(t.Context())

	var wg sync.WaitGroup

	for _, id := range []string{"A", "B", "C"} {
		exec := makeExec(id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = exec.Run(ctx)
		}()
	}

	// 全行が SUCCEEDED になるまで待つ (最大 30 秒).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var succeeded int
		err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM scheduled_session_operations WHERE status = 2").Scan(&succeeded)
		require.NoError(t, err)

		if succeeded >= want {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	// 全行 SUCCEEDED であること.
	var succeeded int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM scheduled_session_operations WHERE status = 2").Scan(&succeeded))
	assert.Equal(t, want, succeeded, "all rows should be SUCCEEDED exactly once")

	// 二重実行が発生していないこと: StopSession の呼び出し回数 == 行数.
	assert.Len(t, op.calls(), want, "stop should be called exactly once per row")
}

func TestScheduledOperationExecutor_ReleaseStaleClaims(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewScheduledSessionOperationRepository(queries)

	// 1 行作って手動で RUNNING + 古い claim をセットする.
	sid := "S-stale"
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
	_, err = pool.Exec(t.Context(),
		`UPDATE scheduled_session_operations SET status = 1, claimed_by = $1, claimed_at = $2 WHERE id = $3`,
		"crashed-instance", staleAt, created.ID,
	)
	require.NoError(t, err)

	released, err := repo.ReleaseStaleClaims(t.Context(), 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), released)

	var status int32
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT status FROM scheduled_session_operations WHERE id::text = $1", created.ID).Scan(&status))
	assert.Equal(t, int32(0), status, "stale RUNNING row should be reset to PENDING")
}

func TestScheduledOperationExecutor_MarkFailedOnActionError(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewScheduledSessionOperationRepository(queries)

	sid := "S-fail"
	_, err := repo.Create(t.Context(), port.ScheduledSessionOperationCreateParams{
		OperationType:    entity.ScheduledOperationType_STOP_SESSION,
		OperationPayload: mustMarshal(t, map[string]string{"session_id": sid}),
		TriggerType:      entity.ScheduledTriggerType_TIME,
		TriggerConfig:    mustMarshal(t, map[string]string{"scheduled_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}),
		NextFireAt:       time.Now().Add(-time.Minute),
		SessionID:        &sid,
	})
	require.NoError(t, err)

	op := &fakeSessionOperator{stopErr: assertOpErr}

	exec := worker.NewScheduledOperationExecutor(repo, op, nil, nil, worker.ScheduledOperationExecutorOptions{
		InstanceID:   "X",
		TickInterval: 50 * time.Millisecond,
		BatchSize:    8,
		Concurrency:  2,
		StaleSweep:   1 * time.Hour,
		StaleAfter:   1 * time.Hour,
	})

	ctx, cancel := context.WithCancel(t.Context())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = exec.Run(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var failed int
		require.NoError(t, pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM scheduled_session_operations WHERE status = 3").Scan(&failed))
		if failed > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	var status int32
	var lastErr pgtype.Text
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT status, last_error FROM scheduled_session_operations").Scan(&status, &lastErr))
	assert.Equal(t, int32(3), status)
	assert.True(t, lastErr.Valid)
	assert.NotEmpty(t, lastErr.String)
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

// assertOpErr is a sentinel used by TestScheduledOperationExecutor_MarkFailedOnActionError.
var assertOpErr = stubError("simulated failure")

type stubError string

func (s stubError) Error() string { return string(s) }
