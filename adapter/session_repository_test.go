package adapter

import (
	"testing"

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

	repo := NewSessionRepository(queries)

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
