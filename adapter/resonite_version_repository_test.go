package adapter_test

import (
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResoniteVersionRepository_ListStaleBuilt は AppVersion bump 時の再ビルド対象が
// 「ブランチごとの最新 built 1 件」に絞られていることを固定する.
// 過去に built した全バージョンを対象にすると、container のコードが要求する Resonite の
// API を持たない古い版まで再ビルドしようとして必ず失敗する.
func TestResoniteVersionRepository_ListStaleBuilt(t *testing.T) {
	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	repo := adapter.NewResoniteVersionRepository(queries)
	branches := []entity.ResoniteVersionBranch{
		entity.ResoniteVersionBranch_Headless,
		entity.ResoniteVersionBranch_Prerelease,
	}

	baseTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	build := func(manifestID string, branch entity.ResoniteVersionBranch, releasedAt time.Time, appVersion string) {
		t.Helper()

		gameVersion := "2026.6." + manifestID
		_, err := repo.Upsert(t.Context(), port.ResoniteVersionUpsertParams{
			ManifestID:  manifestID,
			Branch:      branch,
			GameVersion: &gameVersion,
			ReleasedAt:  releasedAt,
		})
		require.NoError(t, err)

		if appVersion != "" {
			require.NoError(t, repo.SetBuilt(t.Context(), manifestID, branch, manifestID+"-"+appVersion, appVersion))
		}
	}

	// headless: 古い版 2 件 (0.8.1) + 最新 (0.8.1)
	build("h1", entity.ResoniteVersionBranch_Headless, baseTime, "0.8.1")
	build("h2", entity.ResoniteVersionBranch_Headless, baseTime.Add(24*time.Hour), "0.8.1")
	build("h3", entity.ResoniteVersionBranch_Headless, baseTime.Add(48*time.Hour), "0.8.1")
	// prerelease: 最新は未 built、その 1 つ前が built
	build("p1", entity.ResoniteVersionBranch_Prerelease, baseTime, "0.8.1")
	build("p2", entity.ResoniteVersionBranch_Prerelease, baseTime.Add(72*time.Hour), "")
	// public は auto-build 対象外
	build("x1", entity.ResoniteVersionBranch_Public, baseTime, "0.8.1")

	t.Run("ブランチごとの最新 built だけを返す", func(t *testing.T) {
		got, err := repo.ListStaleBuilt(t.Context(), branches, "0.10.0")
		require.NoError(t, err)
		require.Len(t, got, 2)

		// released_at 降順. prerelease は最新 (p2) が未 built なので built 済みの p1 が対象.
		assert.Equal(t, "h3", got[0].ManifestID)
		assert.Equal(t, entity.ResoniteVersionBranch_Headless, got[0].Branch)
		assert.Equal(t, "p1", got[1].ManifestID)
		assert.Equal(t, entity.ResoniteVersionBranch_Prerelease, got[1].Branch)
	})

	t.Run("最新版の再ビルドが失敗しても古い版に遡らない", func(t *testing.T) {
		// 再ビルド失敗で h3 は build_status が built でなくなる. ここで候補選択を
		// build_status で行っていると h2 が「最新の built」に繰り上がり、1 tick ごとに
		// 過去へ遡って必ず失敗する連鎖になる.
		require.NoError(t, repo.SetFailed(t.Context(), "h3", entity.ResoniteVersionBranch_Headless, "boom"))

		got, err := repo.ListStaleBuilt(t.Context(), branches, "0.10.0")
		require.NoError(t, err)

		require.Len(t, got, 1)
		assert.Equal(t, "p1", got[0].ManifestID, "headless は候補が尽き、prerelease だけが残る")
	})

	t.Run("AppVersion が一致していれば対象にならない", func(t *testing.T) {
		require.NoError(t, repo.SetBuilt(t.Context(), "h3", entity.ResoniteVersionBranch_Headless, "h3-0.10.0", "0.10.0"))
		require.NoError(t, repo.SetBuilt(t.Context(), "p1", entity.ResoniteVersionBranch_Prerelease, "p1-0.10.0", "0.10.0"))

		got, err := repo.ListStaleBuilt(t.Context(), branches, "0.10.0")
		require.NoError(t, err)

		// 最新が追いついたので対象なし. 古い h1 / h2 が繰り上がることもない.
		assert.Empty(t, got)
	})
}
