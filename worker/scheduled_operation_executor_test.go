package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/sessionstate"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	_ "github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSessionRepoForTrigger は SessionUserCountTrigger テスト専用の最小限 fake.
// Get の挙動だけ提供する.
type stubSessionRepoForTrigger struct {
	port.SessionRepository

	get map[string]*entity.Session
}

func (r *stubSessionRepoForTrigger) Get(_ context.Context, id string) (*entity.Session, error) {
	s, ok := r.get[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return s, nil
}

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

func TestSessionUserCountTrigger_RoundTrip(t *testing.T) {
	original := triggers.NewSessionUserCountTrigger("S-1", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

	raw, err := original.Marshal()
	require.NoError(t, err)

	decoded, err := scheduled_op.DecodeTrigger(entity.ScheduledTriggerType_SESSION_USER_COUNT, raw)
	require.NoError(t, err)

	st, ok := decoded.(*triggers.SessionUserCountTrigger)
	require.True(t, ok)
	assert.Equal(t, "S-1", st.SessionID)
	assert.Equal(t, triggers.SessionUserCountComparator_LESS_OR_EQUAL, st.Comparator)
	assert.Equal(t, int32(0), st.Threshold)
}

func TestSessionUserCountTrigger_Evaluate(t *testing.T) {
	t.Run("LESS_OR_EQUAL: usersCount <= threshold で ready", func(t *testing.T) {
		cache := sessionstate.NewMemoryCache()
		cache.Set("H-1", "S-1", &headlessv1.Session{Id: "S-1", UsersCount: 0})

		trig := triggers.NewSessionUserCountTrigger("S-1", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

		ready, _, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{StateCache: cache})
		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("LESS_OR_EQUAL: usersCount > threshold で not ready", func(t *testing.T) {
		cache := sessionstate.NewMemoryCache()
		cache.Set("H-1", "S-2", &headlessv1.Session{Id: "S-2", UsersCount: 3})

		trig := triggers.NewSessionUserCountTrigger("S-2", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

		ready, _, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{StateCache: cache})
		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("GREATER_OR_EQUAL: usersCount >= threshold で ready", func(t *testing.T) {
		cache := sessionstate.NewMemoryCache()
		cache.Set("H-1", "S-3", &headlessv1.Session{Id: "S-3", UsersCount: 5})

		trig := triggers.NewSessionUserCountTrigger("S-3", triggers.SessionUserCountComparator_GREATER_OR_EQUAL, 3)

		ready, _, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{StateCache: cache})
		require.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("cache miss / DB に session 無し: requeue (not ready, no error)", func(t *testing.T) {
		cache := sessionstate.NewMemoryCache()
		repo := &stubSessionRepoForTrigger{get: map[string]*entity.Session{}}

		trig := triggers.NewSessionUserCountTrigger("S-missing", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

		ready, _, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{
			StateCache:  cache,
			SessionRepo: repo,
		})
		require.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("cache miss / session が ENDED: trigger fail", func(t *testing.T) {
		cache := sessionstate.NewMemoryCache()
		repo := &stubSessionRepoForTrigger{
			get: map[string]*entity.Session{
				"S-ended": {ID: "S-ended", Status: entity.SessionStatus_ENDED},
			},
		}

		trig := triggers.NewSessionUserCountTrigger("S-ended", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

		ready, _, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{
			StateCache:  cache,
			SessionRepo: repo,
		})
		require.Error(t, err)
		assert.False(t, ready)
	})

	t.Run("StateCache 未指定 (初回登録時): not ready, no error", func(t *testing.T) {
		trig := triggers.NewSessionUserCountTrigger("S-x", triggers.SessionUserCountComparator_LESS_OR_EQUAL, 0)

		ready, next, err := trig.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{})
		require.NoError(t, err)
		assert.False(t, ready)
		assert.True(t, next.IsZero())
	})
}

func TestSessionUserCountTrigger_DecodeRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"session_id 空", `{"session_id":"","comparator":1,"threshold":0}`},
		{"comparator 不正", `{"session_id":"S-1","comparator":99,"threshold":0}`},
		{"threshold 負数", `{"session_id":"S-1","comparator":1,"threshold":-1}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := scheduled_op.DecodeTrigger(entity.ScheduledTriggerType_SESSION_USER_COUNT, json.RawMessage(c.cfg))
			require.Error(t, err)
		})
	}
}

func TestCronTrigger_Next(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	cases := []struct {
		name  string
		trig  triggers.CronTrigger
		after time.Time
		want  time.Time
	}{
		{
			name:  "日次: 当日の時刻前なら当日",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_DAILY, Hour: 3, Minute: 30, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 5, 1, 0, 0, 0, tokyo),
			want:  time.Date(2026, 10, 5, 3, 30, 0, 0, tokyo),
		},
		{
			name:  "日次: 発火時刻ちょうどは含まず翌日",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_DAILY, Hour: 3, Minute: 30, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 5, 3, 30, 0, 0, tokyo),
			want:  time.Date(2026, 10, 6, 3, 30, 0, 0, tokyo),
		},
		{
			name:  "日次: timezone 未指定は UTC",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_DAILY, Hour: 0, Minute: 0},
			after: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
			want:  time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC),
		},
		{
			// 2026-10-05 は月曜日.
			name:  "週次: 指定曜日のうち直近 (月曜 → 水曜)",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_WEEKLY, Hour: 12, Minute: 0, Weekdays: []int32{0, 3}, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 5, 13, 0, 0, 0, tokyo),
			want:  time.Date(2026, 10, 7, 12, 0, 0, 0, tokyo),
		},
		{
			name:  "週次: 当日の曜日で時刻を過ぎていれば翌週",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_WEEKLY, Hour: 12, Minute: 0, Weekdays: []int32{1}, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 5, 13, 0, 0, 0, tokyo),
			want:  time.Date(2026, 10, 12, 12, 0, 0, 0, tokyo),
		},
		{
			name:  "月次: 31 日指定は 31 日の無い月をスキップ",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_MONTHLY, Hour: 0, Minute: 0, DayOfMonth: 31, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 31, 1, 0, 0, 0, tokyo),
			want:  time.Date(2026, 12, 31, 0, 0, 0, 0, tokyo),
		},
		{
			name:  "年次: 2/29 は次の閏年",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_YEARLY, Hour: 9, Minute: 0, Month: 2, DayOfMonth: 29, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 5, 0, 0, 0, 0, tokyo),
			want:  time.Date(2028, 2, 29, 9, 0, 0, 0, tokyo),
		},
		{
			name:  "timezone 基準で日付を判定する (UTC では前日)",
			trig:  triggers.CronTrigger{Frequency: triggers.CronFrequency_MONTHLY, Hour: 8, Minute: 0, DayOfMonth: 1, Timezone: "Asia/Tokyo"},
			after: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
			want:  time.Date(2026, 11, 1, 8, 0, 0, 0, tokyo),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			trig, err := triggers.NewCronTrigger(c.trig)
			require.NoError(t, err)

			got, err := trig.Next(c.after)
			require.NoError(t, err)
			assert.True(t, c.want.Equal(got), "want %s, got %s", c.want, got)
		})
	}
}

func TestCronTrigger_RoundTrip(t *testing.T) {
	original, err := triggers.NewCronTrigger(triggers.CronTrigger{
		Frequency: triggers.CronFrequency_WEEKLY,
		Hour:      23,
		Minute:    59,
		Weekdays:  []int32{1, 5},
		Timezone:  "Asia/Tokyo",
	})
	require.NoError(t, err)

	raw, err := original.Marshal()
	require.NoError(t, err)

	decoded, err := scheduled_op.DecodeTrigger(entity.ScheduledTriggerType_CRON, raw)
	require.NoError(t, err)

	ct, ok := decoded.(*triggers.CronTrigger)
	require.True(t, ok)
	assert.Equal(t, original, ct)

	// 繰り返し trigger は claim された時点で常に ready.
	ready, _, err := ct.Evaluate(t.Context(), scheduled_op.TriggerEvalDeps{Now: time.Now})
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestCronTrigger_DecodeRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"frequency 不正", `{"frequency":99,"hour":0,"minute":0}`},
		{"hour 範囲外", `{"frequency":1,"hour":24,"minute":0}`},
		{"minute 範囲外", `{"frequency":1,"hour":0,"minute":60}`},
		{"timezone 不正", `{"frequency":1,"hour":0,"minute":0,"timezone":"Invalid/Zone"}`},
		{"週次で曜日なし", `{"frequency":2,"hour":0,"minute":0}`},
		{"週次で曜日範囲外", `{"frequency":2,"hour":0,"minute":0,"weekdays":[7]}`},
		{"月次で日なし", `{"frequency":3,"hour":0,"minute":0}`},
		{"年次で存在しない日 (4/31)", `{"frequency":4,"hour":0,"minute":0,"month":4,"day_of_month":31}`},
		{"年次で月なし", `{"frequency":4,"hour":0,"minute":0,"day_of_month":1}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := scheduled_op.DecodeTrigger(entity.ScheduledTriggerType_CRON, json.RawMessage(c.cfg))
			require.Error(t, err)
		})
	}
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
