package adapter_test

import (
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionRepository_UpdateAfterWorldSaved_JSONBRewrite verifies the
// server-side jsonb_set rewrite of startup_parameters.loadWorldUrl and
// current_state.worldUrl. The exact protojson key names must match the
// SQL constants in db/queries/sessions.sql or the rewrite silently no-ops.
func TestSessionRepository_UpdateAfterWorldSaved_JSONBRewrite(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewSessionRepository(queries)

	testutil.CreateTestHeadlessAccount(t, queries, "U-test", "test@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

	t.Run("preset 由来の startup_parameters を loadWorldUrl で置き換え、preset key を消す", func(t *testing.T) {
		err := repo.Upsert(t.Context(), &entity.Session{
			ID:     "s-preset",
			Name:   "preset-session",
			HostID: host.ID,
			Status: entity.SessionStatus_RUNNING,
			StartupParameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldPresetName{LoadWorldPresetName: "Default"},
			},
			CurrentState: &headlessv1.Session{Id: "s-preset", WorldUrl: ""},
		})
		require.NoError(t, err)

		err = repo.UpdateAfterWorldSaved(t.Context(), "s-preset", "resrec:///U-test/R-saved")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "s-preset")
		require.NoError(t, err)

		assert.Equal(t, "resrec:///U-test/R-saved", got.StartupParameters.GetLoadWorldUrl(),
			"preset case must be replaced with loadWorldUrl")
		assert.Empty(t, got.StartupParameters.GetLoadWorldPresetName(),
			"preset key must be removed so the oneof has exactly one active case")
		assert.Equal(t, "resrec:///U-test/R-saved", got.CurrentState.GetWorldUrl())
	})

	t.Run("既存 loadWorldUrl を別 URL に書き換える", func(t *testing.T) {
		err := repo.Upsert(t.Context(), &entity.Session{
			ID:     "s-url",
			Name:   "url-session",
			HostID: host.ID,
			Status: entity.SessionStatus_RUNNING,
			StartupParameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "resrec:///old"},
			},
			CurrentState: &headlessv1.Session{Id: "s-url", WorldUrl: "resrec:///old"},
		})
		require.NoError(t, err)

		err = repo.UpdateAfterWorldSaved(t.Context(), "s-url", "resrec:///new")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "s-url")
		require.NoError(t, err)
		assert.Equal(t, "resrec:///new", got.StartupParameters.GetLoadWorldUrl())
		assert.Equal(t, "resrec:///new", got.CurrentState.GetWorldUrl())
	})

	t.Run("current_state が NULL のまま session でも startup_parameters は更新される", func(t *testing.T) {
		err := repo.Upsert(t.Context(), &entity.Session{
			ID:     "s-no-cs",
			Name:   "no-currentstate",
			HostID: host.ID,
			Status: entity.SessionStatus_RUNNING,
			StartupParameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "resrec:///old"},
			},
			CurrentState: nil,
		})
		require.NoError(t, err)

		err = repo.UpdateAfterWorldSaved(t.Context(), "s-no-cs", "resrec:///new")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "s-no-cs")
		require.NoError(t, err)
		assert.Equal(t, "resrec:///new", got.StartupParameters.GetLoadWorldUrl())
		// MINOR-1 regression guard: handler must not have created an empty
		// CurrentState that the UI would render as "0/0 users".
		assert.Nil(t, got.CurrentState, "NULL current_state must stay NULL")
	})

	t.Run("DowngradeToUnknownIfRunning は ENDED を巻き戻さない", func(t *testing.T) {
		// MINOR-4 regression guard.
		err := repo.Upsert(t.Context(), &entity.Session{
			ID:     "s-ended",
			Name:   "ended-session",
			HostID: host.ID,
			Status: entity.SessionStatus_ENDED,
			StartupParameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "u"},
			},
		})
		require.NoError(t, err)

		err = repo.DowngradeToUnknownIfRunning(t.Context(), "s-ended")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "s-ended")
		require.NoError(t, err)
		assert.Equal(t, entity.SessionStatus_ENDED, got.Status, "ENDED session must not be silently demoted to UNKNOWN")
	})

	t.Run("DowngradeToUnknownIfRunning は RUNNING のみ UNKNOWN へ", func(t *testing.T) {
		err := repo.Upsert(t.Context(), &entity.Session{
			ID:     "s-running",
			Name:   "running-session",
			HostID: host.ID,
			Status: entity.SessionStatus_RUNNING,
			StartupParameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "u"},
			},
		})
		require.NoError(t, err)

		err = repo.DowngradeToUnknownIfRunning(t.Context(), "s-running")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "s-running")
		require.NoError(t, err)
		assert.Equal(t, entity.SessionStatus_UNKNOWN, got.Status)
	})
}

// TestSessionRepository_LifecycleQueries exercises the partial-update SQL
// (ApplySessionStarted / ApplySessionEnded), the host_id+status list query,
// and the ON CONFLICT DO NOTHING insert that back SessionLifecycleHandler.
// The handler-level tests use an in-memory fake; this test verifies the
// actual pgx/sqlc bindings, the WHERE clause logic, and the RowsAffected
// behaviour of `:execrows` against a real Postgres.
//
// Requires the test database to be migrated (see `make test.setup`).
func TestSessionRepository_LifecycleQueries(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)

	t.Run("ApplySessionStarted は新しい occurred_at で UPDATE する", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)
		original := testutil.CreateTestSession(t, queries, host.ID, "session", entity.SessionStatus_RUNNING)

		newStart := original.StartedAt.Time.Add(time.Hour)

		applied, err := repo.ApplySessionStarted(t.Context(), original.ID, host.ID, "new-name", newStart)
		require.NoError(t, err)
		assert.True(t, applied, "newer occurred_at must report applied=true")

		updated, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, "new-name", updated.Name)
		require.NotNil(t, updated.StartedAt)
		assert.True(t, updated.StartedAt.Equal(newStart))
		assert.Equal(t, entity.SessionStatus_RUNNING, updated.Status)
		assert.Nil(t, updated.EndedAt, "ApplySessionStarted must clear ended_at")
	})

	t.Run("ApplySessionStarted は古い occurred_at を skip (RowsAffected==0)", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)
		original := testutil.CreateTestSession(t, queries, host.ID, "session", entity.SessionStatus_RUNNING)

		stale := original.StartedAt.Time.Add(-time.Hour)

		applied, err := repo.ApplySessionStarted(t.Context(), original.ID, host.ID, "stale-name", stale)
		require.NoError(t, err)
		assert.False(t, applied, "older occurred_at must be skipped (RowsAffected==0)")

		unchanged, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, original.Name, unchanged.Name, "stale event must not overwrite name")
	})

	t.Run("ApplySessionStarted は別 host を渡すと host_id を更新する", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		hostA := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host-a", entity.HeadlessHostStatus_RUNNING)
		hostB := testutil.CreateTestHeadlessHost(t, queries, "U-b", "host-b", entity.HeadlessHostStatus_RUNNING)

		original := testutil.CreateTestSession(t, queries, hostA.ID, "session", entity.SessionStatus_RUNNING)
		newStart := original.StartedAt.Time.Add(time.Hour)

		applied, err := repo.ApplySessionStarted(t.Context(), original.ID, hostB.ID, "session", newStart)
		require.NoError(t, err)
		assert.True(t, applied)

		updated, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, hostB.ID, updated.HostID, "ApplySessionStarted must allow host migration")
	})

	t.Run("ApplySessionEnded は新しい occurred_at で UPDATE する", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)
		original := testutil.CreateTestSession(t, queries, host.ID, "session", entity.SessionStatus_RUNNING)
		endedAt := original.StartedAt.Time.Add(2 * time.Hour)

		applied, err := repo.ApplySessionEnded(t.Context(), original.ID, host.ID, endedAt)
		require.NoError(t, err)
		assert.True(t, applied)

		updated, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, entity.SessionStatus_ENDED, updated.Status)
		require.NotNil(t, updated.EndedAt)
		assert.True(t, updated.EndedAt.Equal(endedAt))
	})

	t.Run("ApplySessionEnded は古い ended_at を skip", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)
		original := testutil.CreateTestSession(t, queries, host.ID, "session", entity.SessionStatus_RUNNING)
		firstEnd := original.StartedAt.Time.Add(2 * time.Hour)

		_, err := repo.ApplySessionEnded(t.Context(), original.ID, host.ID, firstEnd)
		require.NoError(t, err)

		earlier := firstEnd.Add(-time.Hour)
		applied, err := repo.ApplySessionEnded(t.Context(), original.ID, host.ID, earlier)
		require.NoError(t, err)
		assert.False(t, applied, "earlier occurred_at must be skipped (RowsAffected==0)")

		got, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		require.NotNil(t, got.EndedAt)
		assert.True(t, got.EndedAt.Equal(firstEnd))
	})

	t.Run("ApplySessionEnded は host_id 不一致なら SQL レベルで skip", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		hostA := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host-a", entity.HeadlessHostStatus_RUNNING)
		hostB := testutil.CreateTestHeadlessHost(t, queries, "U-b", "host-b", entity.HeadlessHostStatus_RUNNING)

		original := testutil.CreateTestSession(t, queries, hostA.ID, "session", entity.SessionStatus_RUNNING)
		endedAt := original.StartedAt.Time.Add(2 * time.Hour)

		applied, err := repo.ApplySessionEnded(t.Context(), original.ID, hostB.ID, endedAt)
		require.NoError(t, err)
		assert.False(t, applied,
			"SessionEnded from a non-owning host must be skipped at the SQL level")

		got, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status)
		assert.Nil(t, got.EndedAt)
	})

	t.Run("InsertFromEvent は既存 row を上書きしない (ON CONFLICT DO NOTHING)", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)
		original := testutil.CreateTestSession(t, queries, host.ID, "original-name", entity.SessionStatus_RUNNING)

		newStart := original.StartedAt.Time.Add(time.Hour)
		err := repo.InsertFromEvent(t.Context(), &entity.Session{
			ID:        original.ID,
			Name:      "conflicting-name",
			Status:    entity.SessionStatus_RUNNING,
			HostID:    host.ID,
			StartedAt: &newStart,
		})
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), original.ID)
		require.NoError(t, err)
		assert.Equal(t, "original-name", got.Name, "InsertFromEvent must not overwrite an existing row")
	})

	t.Run("InsertFromEvent は新しい id なら row を作る", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		host := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host", entity.HeadlessHostStatus_RUNNING)

		now := time.Now().UTC().Truncate(time.Microsecond)
		err := repo.InsertFromEvent(t.Context(), &entity.Session{
			ID:        "fresh-session-id",
			Name:      "fresh-name",
			Status:    entity.SessionStatus_RUNNING,
			HostID:    host.ID,
			StartedAt: &now,
		})
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), "fresh-session-id")
		require.NoError(t, err)
		assert.Equal(t, "fresh-name", got.Name)
		assert.Equal(t, host.ID, got.HostID)
		assert.Equal(t, entity.SessionStatus_RUNNING, got.Status)
	})

	t.Run("ListByHostAndStatus は host_id と status の両方で絞り込む", func(t *testing.T) {
		testutil.CleanupTables(t, pool)

		repo := adapter.NewSessionRepository(queries)
		hostA := testutil.CreateTestHeadlessHost(t, queries, "U-a", "host-a", entity.HeadlessHostStatus_RUNNING)
		hostB := testutil.CreateTestHeadlessHost(t, queries, "U-b", "host-b", entity.HeadlessHostStatus_RUNNING)

		wantRunning := testutil.CreateTestSession(t, queries, hostA.ID, "wanted", entity.SessionStatus_RUNNING)
		_ = testutil.CreateTestSession(t, queries, hostA.ID, "ended-same-host", entity.SessionStatus_ENDED)
		_ = testutil.CreateTestSession(t, queries, hostA.ID, "starting-same-host", entity.SessionStatus_STARTING)
		_ = testutil.CreateTestSession(t, queries, hostB.ID, "running-other-host", entity.SessionStatus_RUNNING)

		got, err := repo.ListByHostAndStatus(t.Context(), hostA.ID, entity.SessionStatus_RUNNING)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, wantRunning.ID, got[0].ID)
	})
}
