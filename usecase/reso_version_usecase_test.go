package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/image_builder"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResoniteVersionRepo は resonite_versions をメモリ上の list で持つ stub.
// list は「新しい順」で持つ (実装の List と同じ契約).
type stubResoniteVersionRepo struct {
	port.ResoniteVersionRepository

	rows []*entity.ResoniteVersion

	builtCalls []string // SetBuilt された manifest_id
}

func (s *stubResoniteVersionRepo) List(_ context.Context, _ *entity.ResoniteVersionBranch) (entity.ResoniteVersionList, error) {
	return s.rows, nil
}

func (s *stubResoniteVersionRepo) GetLatestByBranch(_ context.Context, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error) {
	for _, r := range s.rows {
		if r.Branch == branch && r.GameVersion != nil {
			return r, nil
		}
	}

	return nil, domain.ErrNotFound
}

func (s *stubResoniteVersionRepo) Get(_ context.Context, manifestID string, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error) {
	for _, r := range s.rows {
		if r.ManifestID == manifestID && r.Branch == branch {
			return r, nil
		}
	}

	return nil, domain.ErrNotFound
}

func (s *stubResoniteVersionRepo) SetBuilding(_ context.Context, _ string, _ entity.ResoniteVersionBranch) error {
	return nil
}

func (s *stubResoniteVersionRepo) SetBuilt(_ context.Context, manifestID string, _ entity.ResoniteVersionBranch, _, _ string) error {
	s.builtCalls = append(s.builtCalls, manifestID)

	return nil
}

// stubHostConnector はローカル image タグの一覧だけを返す stub.
type stubHostConnector struct {
	hostconnector.HostConnector

	localTags []string
}

func (s *stubHostConnector) ListLocalImageTags(_ context.Context) ([]string, error) {
	return s.localTags, nil
}

// stubImageBuilder は実ビルドの代わりに呼ばれた回数と固定の結果を持つ.
type stubImageBuilder struct {
	appVersion string
	result     *image_builder.BuildResult
	buildCalls int
}

func (s *stubImageBuilder) Build(_ context.Context, _ image_builder.BuildParams) (*image_builder.BuildResult, error) {
	s.buildCalls++

	return s.result, nil
}

func (s *stubImageBuilder) CurrentAppVersion(_ context.Context) (string, error) {
	return s.appVersion, nil
}

func ptr[T any](v T) *T { return &v }

func newResoniteVersionUsecaseUnderTest(
	repo port.ResoniteVersionRepository,
	builder imageBuilder,
	connector hostconnector.HostConnector,
) *ResoniteVersionUsecase {
	return NewResoniteVersionUsecase(repo, builder, connector, &config.ResoniteBuildConfig{})
}

// builtHeadlessVersion は built 済み (ローカル image tag つき) の行を組む.
func builtHeadlessVersion(manifestID, gameVersion, imageTag string, released time.Time) *entity.ResoniteVersion {
	v := headlessVersion(manifestID, gameVersion, released)
	v.BuildStatus = entity.ResoniteVersionBuildStatus_Built
	v.ImageTag = ptr(imageTag)

	return v
}

func headlessVersion(manifestID, gameVersion string, released time.Time) *entity.ResoniteVersion {
	return &entity.ResoniteVersion{
		ManifestID:  manifestID,
		Branch:      entity.ResoniteVersionBranch_Headless,
		GameVersion: &gameVersion,
		ReleasedAt:  released,
		BuildStatus: entity.ResoniteVersionBuildStatus_NotBuilt,
	}
}

func TestResoniteVersionUsecase_ResolveForStart_Latest(t *testing.T) {
	t.Parallel()

	older := builtHeadlessVersion("m-old", "2026.7.1", "2026.7.1-headless", time.Now().Add(-24*time.Hour))

	t.Run("成功: 最新版が built でローカルに image があればそのタグ", func(t *testing.T) {
		t.Parallel()

		built := builtHeadlessVersion("m-new", "2026.8.1", "2026.8.1-headless", time.Now())

		uc := newResoniteVersionUsecaseUnderTest(
			&stubResoniteVersionRepo{rows: entity.ResoniteVersionList{built, older}},
			&stubImageBuilder{},
			&stubHostConnector{localTags: []string{"2026.8.1-headless", "2026.7.1-headless"}},
		)

		tag, err := uc.ResolveForStart(t.Context(), "latestRelease")
		require.NoError(t, err)
		assert.Equal(t, "2026.8.1-headless", tag)
	})

	// latestRelease は「built 済みの最新」ではなく「branch の最新版」を指す.
	// 古い built 済みタグに黙って落ちると、ユーザーは最新で起動したつもりのまま
	// 数世代前で動き続けることになる.
	t.Run("成功: 最新版が未 built なら古い built 版に落ちず NotBuiltError", func(t *testing.T) {
		t.Parallel()

		newer := headlessVersion("m-new", "2026.8.1", time.Now())

		uc := newResoniteVersionUsecaseUnderTest(
			&stubResoniteVersionRepo{rows: entity.ResoniteVersionList{newer, older}},
			&stubImageBuilder{},
			&stubHostConnector{localTags: []string{"2026.7.1-headless"}},
		)

		_, err := uc.ResolveForStart(t.Context(), "latestRelease")

		var nbe *NotBuiltError

		require.ErrorAs(t, err, &nbe)
		assert.Equal(t, "m-new", nbe.ManifestID)
		assert.Equal(t, entity.ResoniteVersionBranch_Headless, nbe.Branch)
	})

	t.Run("成功: DB 上 built でも image が消えていれば NotBuiltError", func(t *testing.T) {
		t.Parallel()

		built := builtHeadlessVersion("m-new", "2026.8.1", "2026.8.1-headless", time.Now())

		uc := newResoniteVersionUsecaseUnderTest(
			&stubResoniteVersionRepo{rows: entity.ResoniteVersionList{built}},
			&stubImageBuilder{},
			&stubHostConnector{localTags: nil},
		)

		_, err := uc.ResolveForStart(t.Context(), "latestRelease")

		var nbe *NotBuiltError

		require.ErrorAs(t, err, &nbe)
		assert.Equal(t, "m-new", nbe.ManifestID)
	})
}

// RunBuild は複数経路 (ContentPoller の検知 / ホスト起動・再起動の chain) から
// 同じバージョンに対して投入されうる. 既に最新の AppVersion でビルド済みなら
// ビルドし直さないが、AppVersion が変わっていれば再ビルドする.
func TestResoniteVersionUsecase_RunBuild_SkipsRedundantBuild(t *testing.T) {
	t.Parallel()

	t.Run("成功: 最新 AppVersion で built 済みならビルドせず既存タグを返す", func(t *testing.T) {
		t.Parallel()

		row := builtHeadlessVersion("m-new", "2026.8.1", "2026.8.1-headless", time.Now())
		row.BuiltWithAppVersion = ptr("1.2.3")

		repo := &stubResoniteVersionRepo{rows: entity.ResoniteVersionList{row}}
		builder := &stubImageBuilder{appVersion: "1.2.3"}

		uc := newResoniteVersionUsecaseUnderTest(repo, builder,
			&stubHostConnector{localTags: []string{"2026.8.1-headless"}})

		tag, err := uc.RunBuild(t.Context(), "m-new", entity.ResoniteVersionBranch_Headless)
		require.NoError(t, err)
		assert.Equal(t, "2026.8.1-headless", tag)
		assert.Zero(t, builder.buildCalls)
		assert.Empty(t, repo.builtCalls)
	})

	t.Run("成功: AppVersion が bump していれば built 済みでも再ビルドする", func(t *testing.T) {
		t.Parallel()

		row := builtHeadlessVersion("m-new", "2026.8.1", "2026.8.1-headless", time.Now())
		row.BuiltWithAppVersion = ptr("1.2.3")

		repo := &stubResoniteVersionRepo{rows: entity.ResoniteVersionList{row}}
		builder := &stubImageBuilder{
			appVersion: "1.3.0",
			result: &image_builder.BuildResult{
				ImageTag:        "2026.8.1-headless-2",
				ResoniteVersion: "2026.8.1",
				AppVersion:      "1.3.0",
			},
		}

		uc := newResoniteVersionUsecaseUnderTest(repo, builder,
			&stubHostConnector{localTags: []string{"2026.8.1-headless"}})

		tag, err := uc.RunBuild(t.Context(), "m-new", entity.ResoniteVersionBranch_Headless)
		require.NoError(t, err)
		assert.Equal(t, "2026.8.1-headless-2", tag)
		assert.Equal(t, 1, builder.buildCalls)
		assert.Equal(t, []string{"m-new"}, repo.builtCalls)
	})

	t.Run("成功: 未 built ならビルドする", func(t *testing.T) {
		t.Parallel()

		repo := &stubResoniteVersionRepo{
			rows: entity.ResoniteVersionList{headlessVersion("m-new", "2026.8.1", time.Now())},
		}
		builder := &stubImageBuilder{
			appVersion: "1.2.3",
			result: &image_builder.BuildResult{
				ImageTag:        "2026.8.1-headless",
				ResoniteVersion: "2026.8.1",
				AppVersion:      "1.2.3",
			},
		}

		uc := newResoniteVersionUsecaseUnderTest(repo, builder, &stubHostConnector{})

		tag, err := uc.RunBuild(t.Context(), "m-new", entity.ResoniteVersionBranch_Headless)
		require.NoError(t, err)
		assert.Equal(t, "2026.8.1-headless", tag)
		assert.Equal(t, 1, builder.buildCalls)
	})
}
