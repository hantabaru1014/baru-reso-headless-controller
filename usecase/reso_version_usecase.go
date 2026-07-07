package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/image_builder"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// NotBuiltError は resolveTagToUse で「対象のバージョンは DB にあるが未 built」
// であることを示すエラー. Dispatcher.startHost はこれを見て BUILD_IMAGE を chain する.
type NotBuiltError struct {
	ManifestID string
	Branch     entity.ResoniteVersionBranch
}

func (e *NotBuiltError) Error() string {
	return "resonite image not built: manifest=" + e.ManifestID + " branch=" + string(e.Branch)
}

// BuildSuccessObserver は build 成功時に発火するコールバック.
// HostUpgradeOrchestrator は built タグでの roll を判断するためこれを購読する.
type BuildSuccessObserver func(ctx context.Context, image *port.ContainerImage)

// ResoniteVersionUsecase は versions.json 由来のバージョン管理 + ローカルビルドの
// 中枢. HeadlessHostUsecase / Dispatcher / ContentPoller / RPC handler / Orchestrator
// はここ経由でバージョン情報にアクセスする.
type ResoniteVersionUsecase struct {
	repo    port.ResoniteVersionRepository
	builder *image_builder.Builder
	cfg     *config.ResoniteBuildConfig

	mu        sync.Mutex
	observers []BuildSuccessObserver
}

func NewResoniteVersionUsecase(
	repo port.ResoniteVersionRepository,
	builder *image_builder.Builder,
	cfg *config.ResoniteBuildConfig,
) *ResoniteVersionUsecase {
	return &ResoniteVersionUsecase{
		repo:    repo,
		builder: builder,
		cfg:     cfg,
	}
}

// Subscribe は build 成功時のオブザーバを登録する. wire 時に呼ばれる想定.
func (u *ResoniteVersionUsecase) Subscribe(o BuildSuccessObserver) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.observers = append(u.observers, o)
}

// List はすべての resonite_versions を返す (RPC の ListResoniteVersions で利用).
func (u *ResoniteVersionUsecase) List(ctx context.Context, branch *entity.ResoniteVersionBranch) (entity.ResoniteVersionList, error) {
	return u.repo.List(ctx, branch)
}

// ListBuiltAsContainerImages は既存の ListHeadlessHostImageTags と互換のある
// ContainerImage list を返す. built 済み行のみ.
func (u *ResoniteVersionUsecase) ListBuiltAsContainerImages(ctx context.Context) (port.ContainerImageList, error) {
	rows, err := u.repo.List(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	out := make(port.ContainerImageList, 0, len(rows))

	for _, r := range rows {
		if r.BuildStatus != entity.ResoniteVersionBuildStatus_Built {
			continue
		}

		if r.ImageTag == nil || r.GameVersion == nil {
			continue
		}

		appVersion := ""
		if r.BuiltWithAppVersion != nil {
			appVersion = *r.BuiltWithAppVersion
		}

		out = append(out, &port.ContainerImage{
			Tag:             *r.ImageTag,
			ResoniteVersion: *r.GameVersion,
			IsPreRelease:    r.Branch == entity.ResoniteVersionBranch_Prerelease,
			AppVersion:      appVersion,
		})
	}

	return out, nil
}

// ResolveForStart は tagInput を実イメージタグに解決する.
// - "" / "latestRelease" → branch=headless の built 済み最新
// - "latestPreRelease" → branch=prerelease の built 済み最新
// - 明示タグ → image_tag で逆引き
// 対象行が DB にあるが未 built の場合、*NotBuiltError を返す (chain 用).
func (u *ResoniteVersionUsecase) ResolveForStart(ctx context.Context, tagInput string) (string, error) {
	switch tagInput {
	case "", "latestRelease":
		return u.resolveLatest(ctx, entity.ResoniteVersionBranch_Headless)
	case "latestPreRelease":
		return u.resolveLatest(ctx, entity.ResoniteVersionBranch_Prerelease)
	}

	row, err := u.repo.GetByImageTag(ctx, tagInput)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// DB 未登録の tag はそのまま返す (legacy 経路 / debug 用途).
			return tagInput, nil
		}

		return "", errors.Wrap(err, 0)
	}

	if row.BuildStatus != entity.ResoniteVersionBuildStatus_Built || row.ImageTag == nil {
		return "", &NotBuiltError{ManifestID: row.ManifestID, Branch: row.Branch}
	}

	return *row.ImageTag, nil
}

// RunBuild は BUILD_IMAGE async job の handler 本体.
// build 前に status=building へ遷移し、成功時 status=built + image_tag / built_with_app_version 更新、
// 失敗時 status=failed + build_error 記録.
// 成功時は Subscribe した observer 群に built タグを push (Orchestrator が roll 判定).
func (u *ResoniteVersionUsecase) RunBuild(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) (string, error) {
	row, err := u.repo.Get(ctx, manifestID, branch)
	if err != nil {
		return "", errors.WrapPrefix(err, "get resonite_version row", 0)
	}

	if err := u.repo.SetBuilding(ctx, manifestID, branch); err != nil {
		return "", errors.WrapPrefix(err, "mark building", 0)
	}

	res, err := u.builder.Build(ctx, image_builder.BuildParams{
		ManifestID:  manifestID,
		Branch:      branch,
		GameVersion: row.GameVersion,
	})
	if err != nil {
		msg := err.Error()
		if setErr := u.repo.SetFailed(ctx, manifestID, branch, msg); setErr != nil {
			slog.Error("failed to mark version failed", "err", setErr, "manifest", manifestID)
		}

		return "", errors.WrapPrefix(err, "build", 0)
	}

	if err := u.repo.SetBuilt(ctx, manifestID, branch, res.ImageTag, res.AppVersion); err != nil {
		return "", errors.WrapPrefix(err, "mark built", 0)
	}

	u.notify(ctx, &port.ContainerImage{
		Tag:             res.ImageTag,
		ResoniteVersion: res.ResoniteVersion,
		IsPreRelease:    branch == entity.ResoniteVersionBranch_Prerelease,
		AppVersion:      res.AppVersion,
	})

	return res.ImageTag, nil
}

// versionsJSONShape は resonite-love/resonite-version-monitor の JSON 形状.
type versionsJSONShape map[string][]struct {
	ManifestID  string    `json:"manifestId"`
	Timestamp   time.Time `json:"timestamp"`
	GameVersion *string   `json:"gameVersion"`
}

// versionsFetchTimeout は versions.json GET のリクエスト単位タイムアウト.
// 上位の tick timeout (10 分) より短く切って、fetch がハングしても poller が
// 別サブタスク (builder image の AppVersion チェック) に進める.
const versionsFetchTimeout = 30 * time.Second

// FetchAndUpsertVersions は versions.json を fetch し、resonite_versions に upsert する.
// 新規に検知した entries を戻り値で返す (ContentPoller が auto-build 判定に使う).
func (u *ResoniteVersionUsecase) FetchAndUpsertVersions(ctx context.Context) (entity.ResoniteVersionList, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, versionsFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, u.cfg.VersionsJSONURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("versions.json GET %s: %s", u.cfg.VersionsJSONURL, resp.Status)
	}

	var raw versionsJSONShape
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, errors.WrapPrefix(err, "decode versions.json", 0)
	}

	newlyAdded := make(entity.ResoniteVersionList, 0)

	for branchName, entries := range raw {
		branch := entity.ResoniteVersionBranch(branchName)

		for _, e := range entries {
			existing, err := u.repo.Get(ctx, e.ManifestID, branch)

			isNew := errors.Is(err, domain.ErrNotFound)
			if err != nil && !isNew {
				slog.Warn("versions.json: failed to look up existing row", "manifest", e.ManifestID, "branch", branch, "err", err)

				continue
			}

			row, err := u.repo.Upsert(ctx, port.ResoniteVersionUpsertParams{
				ManifestID:  e.ManifestID,
				Branch:      branch,
				GameVersion: e.GameVersion,
				ReleasedAt:  e.Timestamp,
			})
			if err != nil {
				slog.Warn("versions.json: upsert failed", "manifest", e.ManifestID, "branch", branch, "err", err)

				continue
			}

			if isNew {
				newlyAdded = append(newlyAdded, row)
			} else if existing != nil {
				_ = existing // reserved for future diff
			}
		}
	}

	// 新しい順に並べて返す (呼び出し側が最新から扱いやすい).
	slices.SortFunc(newlyAdded, func(a, b *entity.ResoniteVersion) int {
		return b.ReleasedAt.Compare(a.ReleasedAt)
	})

	return newlyAdded, nil
}

// CurrentAppVersion は builder image を pull で最新化しつつ label brhc.app-version を読む.
func (u *ResoniteVersionUsecase) CurrentAppVersion(ctx context.Context) (string, error) {
	return u.builder.CurrentAppVersion(ctx)
}

// ListStaleBuilt は built_with_app_version が currentAppVersion と一致しない built 行を返す.
// AppVersion bump 時の再ビルド対象特定に使う (auto-build 対象ブランチのみ).
func (u *ResoniteVersionUsecase) ListStaleBuilt(ctx context.Context, currentAppVersion string) (entity.ResoniteVersionList, error) {
	return u.repo.ListStaleBuilt(ctx, entity.AutoBuildBranches, currentAppVersion)
}

func (u *ResoniteVersionUsecase) resolveLatest(ctx context.Context, branch entity.ResoniteVersionBranch) (string, error) {
	built, err := u.repo.GetLatestBuiltByBranch(ctx, branch)
	if err == nil {
		if built.ImageTag != nil {
			return *built.ImageTag, nil
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return "", errors.Wrap(err, 0)
	}

	// built が無ければ、未 built の中で最新のものを chain 対象として返す.
	all, err := u.repo.List(ctx, &branch)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	for _, r := range all {
		if r.GameVersion == nil {
			continue
		}

		return "", &NotBuiltError{ManifestID: r.ManifestID, Branch: r.Branch}
	}

	return "", errors.Errorf("no resonite_versions found for branch=%s (versions.json fetched?)", branch)
}

func (u *ResoniteVersionUsecase) notify(ctx context.Context, image *port.ContainerImage) {
	u.mu.Lock()
	obs := append([]BuildSuccessObserver(nil), u.observers...)
	u.mu.Unlock()

	for _, o := range obs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("build-success observer panicked", "panic", r)
				}
			}()

			o(ctx, image)
		}()
	}
}
