package adapter_test

import (
	"testing"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionRepository_UpdateAfterWorldSaved_RewritesStartupParameters(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewSessionRepository(queries)

	testutil.CreateTestHeadlessAccount(t, queries, "U-test", "test@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

	t.Run("preset 由来の startup_parameters を loadWorldUrl で置き換え、preset key を消す", func(t *testing.T) {
		preset := testutil.CreateTestSessionWithStartupParameters(t, queries, host.ID, "preset-session", entity.SessionStatus_RUNNING, &headlessv1.WorldStartupParameters{
			LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldPresetName{LoadWorldPresetName: "Default"},
		})

		err := repo.UpdateAfterWorldSaved(t.Context(), preset.ID, "resrec:///U-test/R-saved")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), preset.ID)
		require.NoError(t, err)
		assert.Equal(t, "resrec:///U-test/R-saved", got.StartupParameters.GetLoadWorldUrl(),
			"preset case must be replaced with loadWorldUrl")
		assert.Empty(t, got.StartupParameters.GetLoadWorldPresetName(),
			"preset key must be removed so the oneof has exactly one active case")
	})

	t.Run("既存 loadWorldUrl を別 URL に書き換える", func(t *testing.T) {
		urlSession := testutil.CreateTestSessionWithStartupParameters(t, queries, host.ID, "url-session", entity.SessionStatus_RUNNING, &headlessv1.WorldStartupParameters{
			LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: "resrec:///old"},
		})

		err := repo.UpdateAfterWorldSaved(t.Context(), urlSession.ID, "resrec:///new")
		require.NoError(t, err)

		got, err := repo.Get(t.Context(), urlSession.ID)
		require.NoError(t, err)
		assert.Equal(t, "resrec:///new", got.StartupParameters.GetLoadWorldUrl())
	})
}

// TestSessionRepository_ApplySessionParametersChanged verifies the SQL jsonb merge
// writes the SessionInfo overlap fields into startup_parameters. If a protojson
// key name drifts (e.g. proto field rename), this test fails immediately rather
// than the production session re-loading with stale data.
func TestSessionRepository_ApplySessionParametersChanged_MergesIntoStartupParameters(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewSessionRepository(queries)

	testutil.CreateTestHeadlessAccount(t, queries, "U-test", "test@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

	descBefore := "desc-before"
	sess := testutil.CreateTestSessionWithStartupParameters(t, queries, host.ID, "before", entity.SessionStatus_RUNNING, &headlessv1.WorldStartupParameters{
		LoadWorld:   &headlessv1.WorldStartupParameters_LoadWorldPresetName{LoadWorldPresetName: "Default"},
		Description: &descBefore,
		Tags:        []string{"keep-me"},
	})

	snapshot := &headlessv1.Session{
		Id:                         sess.ID,
		Name:                       "after",
		Description:                "desc-after",
		MaxUsers:                   24,
		AccessLevel:                headlessv1.AccessLevel_ACCESS_LEVEL_ANYONE,
		HideFromPublicListing:      true,
		AwayKickMinutes:            7.5,
		IdleRestartIntervalSeconds: 180,
		SaveOnExit:                 true,
		AutoSaveIntervalSeconds:    300,
		AutoSleep:                  false,
		Tags:                       []string{"newtag"},
	}

	err := repo.ApplySessionParametersChanged(t.Context(), sess.ID, snapshot)
	require.NoError(t, err)

	got, err := repo.Get(t.Context(), sess.ID)
	require.NoError(t, err)

	assert.Equal(t, "after", got.Name, "name 列も更新される")
	sp := got.StartupParameters
	assert.Equal(t, "after", sp.GetName())
	assert.Equal(t, "desc-after", sp.GetDescription())
	assert.Equal(t, int32(24), sp.GetMaxUsers())
	assert.Equal(t, headlessv1.AccessLevel_ACCESS_LEVEL_ANYONE, sp.GetAccessLevel())
	assert.True(t, sp.GetHideFromPublicListing())
	assert.InDelta(t, 7.5, sp.GetAwayKickMinutes(), 0.001)
	assert.Equal(t, int32(180), sp.GetIdleRestartIntervalSeconds())
	assert.True(t, sp.GetSaveOnExit())
	assert.Equal(t, int32(300), sp.GetAutoSaveIntervalSeconds())
	assert.False(t, sp.GetAutoSleep())
	assert.Equal(t, []string{"newtag"}, sp.GetTags(), "tags 配列は丸ごと置換")

	// preset case は merge 対象外なので保持される
	assert.Equal(t, "Default", sp.GetLoadWorldPresetName(),
		"merge は overlap field のみ。load_world oneof などは触らない")
}

func TestSessionRepository_ApplySessionParametersChanged_EmptyTags(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewSessionRepository(queries)

	testutil.CreateTestHeadlessAccount(t, queries, "U-test", "test@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

	sess := testutil.CreateTestSessionWithStartupParameters(t, queries, host.ID, "n", entity.SessionStatus_RUNNING, &headlessv1.WorldStartupParameters{
		Tags: []string{"a", "b"},
	})

	err := repo.ApplySessionParametersChanged(t.Context(), sess.ID, &headlessv1.Session{Id: sess.ID, Tags: nil})
	require.NoError(t, err)

	got, err := repo.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	assert.Empty(t, got.StartupParameters.GetTags(), "空配列を渡すと tags は全消し")
}

func TestSessionRepository_DowngradeToUnknownIfRunning_GuardedAgainstEnded(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewSessionRepository(queries)

	testutil.CreateTestHeadlessAccount(t, queries, "U-test", "test@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

	ended := testutil.CreateTestSession(t, queries, host.ID, "ended-session", entity.SessionStatus_ENDED)
	running := testutil.CreateTestSession(t, queries, host.ID, "running-session", entity.SessionStatus_RUNNING)

	require.NoError(t, repo.DowngradeToUnknownIfRunning(t.Context(), ended.ID))
	require.NoError(t, repo.DowngradeToUnknownIfRunning(t.Context(), running.ID))

	gotEnded, err := repo.Get(t.Context(), ended.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.SessionStatus_ENDED, gotEnded.Status, "ENDED は巻き戻さない")

	gotRunning, err := repo.Get(t.Context(), running.ID)
	require.NoError(t, err)
	assert.Equal(t, entity.SessionStatus_UNKNOWN, gotRunning.Status, "RUNNING のみ UNKNOWN へ降ろす")
}
