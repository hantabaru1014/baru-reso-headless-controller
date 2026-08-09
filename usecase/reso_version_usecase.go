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
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/image_builder"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"golang.org/x/sync/singleflight"
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

// imageBuilder は RunBuild / CurrentAppVersion が使う builder の操作だけを切り出した
// interface. 実体は *image_builder.Builder で、テストでの差し替えのために挟んでいる.
type imageBuilder interface {
	Build(ctx context.Context, p image_builder.BuildParams) (*image_builder.BuildResult, error)
	CurrentAppVersion(ctx context.Context) (string, error)
}

// ResoniteVersionUsecase は versions.json 由来のバージョン管理 + ローカルビルドの
// 中枢. HeadlessHostUsecase / Dispatcher / ContentPoller / RPC handler / Orchestrator
// はここ経由でバージョン情報にアクセスする.
//
// DB の build_status は履歴・診断用の記録であり、「起動に使えるか」の判定には
// 使わない. docker image はユーザー操作 (prune 等) でいつでも消えうるため、
// 候補生成 (List / ListBuiltAsContainerImages) と起動解決 (ResolveForStart) の
// 時点でローカル image の実在を connector 経由で確認する.
type ResoniteVersionUsecase struct {
	repo      port.ResoniteVersionRepository
	builder   imageBuilder
	connector hostconnector.HostConnector
	cfg       *config.ResoniteBuildConfig

	mu        sync.Mutex
	observers []BuildSuccessObserver

	// buildGroup は同一 (manifest, branch) の RunBuild をまとめる. image_builder 側の
	// mutex はビルド本体しか守らないため、「既にビルド済みか」の判定込みで重複を潰すには
	// ここで束ねる必要がある.
	buildGroup singleflight.Group
}

func NewResoniteVersionUsecase(
	repo port.ResoniteVersionRepository,
	builder imageBuilder,
	connector hostconnector.HostConnector,
	cfg *config.ResoniteBuildConfig,
) *ResoniteVersionUsecase {
	return &ResoniteVersionUsecase{
		repo:      repo,
		builder:   builder,
		connector: connector,
		cfg:       cfg,
	}
}

// Subscribe は build 成功時のオブザーバを登録する. wire 時に呼ばれる想定.
func (u *ResoniteVersionUsecase) Subscribe(o BuildSuccessObserver) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.observers = append(u.observers, o)
}

// List はすべての resonite_versions を返す (RPC の ListResoniteVersions で利用).
// build_status が built でもローカルに image が存在しない行は not_built に落として
// 返す (prune 等で image が消えた場合、フロントは再ビルド候補として扱える).
// ローカル image の列挙に失敗した場合は warn ログを出して DB の記録のまま返す.
func (u *ResoniteVersionUsecase) List(ctx context.Context, branch *entity.ResoniteVersionBranch) (entity.ResoniteVersionList, error) {
	rows, err := u.repo.List(ctx, branch)
	if err != nil {
		return nil, err
	}

	local, err := u.localTagSet(ctx)
	if err != nil {
		slog.Warn("resonite versions: failed to list local images; returning DB build status as-is", "err", err)

		return rows, nil
	}

	for _, r := range rows {
		if r.BuildStatus != entity.ResoniteVersionBuildStatus_Built || r.ImageTag == nil {
			continue
		}

		if _, ok := local[*r.ImageTag]; !ok {
			r.BuildStatus = entity.ResoniteVersionBuildStatus_NotBuilt
		}
	}

	return rows, nil
}

// ListBuiltAsContainerImages は既存の ListHeadlessHostImageTags と互換のある
// ContainerImage list を返す. built 済みかつローカルに image が実在する行のみ
// (prune 等で image が消えたタグを起動候補に出さない).
func (u *ResoniteVersionUsecase) ListBuiltAsContainerImages(ctx context.Context) (port.ContainerImageList, error) {
	rows, err := u.repo.List(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	local, err := u.localTagSet(ctx)
	if err != nil {
		return nil, err
	}

	out := make(port.ContainerImageList, 0, len(rows))

	for _, r := range rows {
		if r.BuildStatus != entity.ResoniteVersionBuildStatus_Built {
			continue
		}

		if r.ImageTag == nil || r.GameVersion == nil {
			continue
		}

		if _, ok := local[*r.ImageTag]; !ok {
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
// - "" / "latestRelease" → branch=headless の最新版
// - "latestPreRelease" → branch=prerelease の最新版
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

	return u.builtTagOrNotBuilt(ctx, row)
}

// RunBuild は BUILD_IMAGE async job の handler 本体.
// build 前に status=building へ遷移し、成功時 status=built + image_tag / built_with_app_version 更新、
// 失敗時 status=failed + build_error 記録.
// 成功時は Subscribe した observer 群に built タグを push (Orchestrator が roll 判定).
//
// 同一バージョンに対する BUILD_IMAGE job は複数経路から並行して投入されうる
// (ContentPoller の新版検知 / AppVersion bump 検知、ホスト起動・再起動時の chain).
// singleflight で同一 (manifest, branch) の実行を 1 本にまとめ、さらに実行前に
// 「もうビルド済みか」を見て無駄な再ビルドを避ける. どちらもプロセス内の best-effort
// (複数インスタンス構成では両方が走りうるが、結果は同じイメージになる).
//
// 相乗りした側は自分の ctx が先に切れたらそこで諦める (job の実行時間上限は
// worker 側で job ごとに与えられるため、先行ビルドの残り時間に引きずられない).
// 先行ビルド自体はそのまま走り続けるので、次の job は built 済みとして拾える.
func (u *ResoniteVersionUsecase) RunBuild(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) (string, error) {
	ch := u.buildGroup.DoChan(manifestID+"\x00"+string(branch), func() (any, error) {
		return u.runBuild(ctx, manifestID, branch)
	})

	select {
	case <-ctx.Done():
		return "", errors.Wrap(ctx.Err(), 0)
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}

		built, ok := res.Val.(string)
		if !ok {
			return "", errors.Errorf("unexpected build result type %T", res.Val)
		}

		return built, nil
	}
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

// ListStaleBuilt は auto-build 対象ブランチごとの最新 built 行のうち、
// built_with_app_version が currentAppVersion と一致しないものを返す.
// AppVersion bump 時の再ビルド対象特定に使う.
func (u *ResoniteVersionUsecase) ListStaleBuilt(ctx context.Context, currentAppVersion string) (entity.ResoniteVersionList, error) {
	return u.repo.ListStaleBuilt(ctx, entity.AutoBuildBranches, currentAppVersion)
}

// builtTagOrNotBuilt は row が「今すぐ起動に使えるイメージ」を持っていればそのタグを、
// 持っていなければ *NotBuiltError を返す (呼び出し元が BUILD_IMAGE を chain する).
// DB 上 built でも image が prune 等で消えていれば未 built 扱い.
func (u *ResoniteVersionUsecase) builtTagOrNotBuilt(ctx context.Context, row *entity.ResoniteVersion) (string, error) {
	if row.BuildStatus == entity.ResoniteVersionBuildStatus_Built && row.ImageTag != nil {
		exists, err := u.imageExists(ctx, *row.ImageTag)
		if err != nil {
			return "", err
		}

		if exists {
			return *row.ImageTag, nil
		}
	}

	return "", &NotBuiltError{ManifestID: row.ManifestID, Branch: row.Branch}
}

func (u *ResoniteVersionUsecase) runBuild(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) (string, error) {
	row, err := u.repo.Get(ctx, manifestID, branch)
	if err != nil {
		return "", errors.WrapPrefix(err, "get resonite_version row", 0)
	}

	upToDateTag, err := u.upToDateTag(ctx, row)
	if err != nil {
		return "", err
	}

	if upToDateTag != "" {
		slog.Info("resonite versions: build skipped; already up to date",
			"manifest", manifestID, "branch", branch, "tag", upToDateTag)

		return upToDateTag, nil
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

// upToDateTag は row が「今ビルドし直しても同じ結果になる」状態なら既存の image tag を、
// ビルドが要るなら "" を返す.
//
// 判定材料は build_status ではなく実体: DB 上 built でもローカル image が prune 等で
// 消えていればビルドが要る. また built_with_app_version が builder image の
// AppVersion と違えば、ContentPoller が投入する AppVersion bump 後の再ビルド対象
// なので skip してはいけない.
func (u *ResoniteVersionUsecase) upToDateTag(ctx context.Context, row *entity.ResoniteVersion) (string, error) {
	if row.BuiltWithAppVersion == nil {
		return "", nil
	}

	tag, err := u.builtTagOrNotBuilt(ctx, row)
	if err != nil {
		var nbe *NotBuiltError
		if errors.As(err, &nbe) {
			return "", nil
		}

		return "", err
	}

	appVersion, err := u.CurrentAppVersion(ctx)
	if err != nil {
		// 判定できないときはビルドする. 冗長なビルドのほうが、古いイメージのまま
		// 起動させてしまうより害が小さい.
		slog.Warn("resonite versions: failed to read builder AppVersion; building anyway", "err", err)

		return "", nil
	}

	if *row.BuiltWithAppVersion != appVersion {
		return "", nil
	}

	return tag, nil
}

// localTagSet はローカルに存在する headless image のタグ集合を返す.
func (u *ResoniteVersionUsecase) localTagSet(ctx context.Context) (map[string]struct{}, error) {
	tags, err := u.connector.ListLocalImageTags(ctx)
	if err != nil {
		return nil, errors.WrapPrefix(err, "list local image tags", 0)
	}

	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		set[t] = struct{}{}
	}

	return set, nil
}

// imageExists は tag のローカル image が実在するかを返す.
func (u *ResoniteVersionUsecase) imageExists(ctx context.Context, tag string) (bool, error) {
	local, err := u.localTagSet(ctx)
	if err != nil {
		return false, err
	}

	_, ok := local[tag]

	return ok, nil
}

// resolveLatest は branch の最新版 (versions.json 上の最新) を解決する.
// 「built 済みの中の最新」ではなく常に最新版が対象なので、それが未 built なら
// *NotBuiltError を返し、呼び出し元 (job handler) が BUILD_IMAGE を chain する.
// 古い built 済みタグへのフォールバックはしない: latestRelease / latestPreRelease は
// 「最新版で動かす」指定であり、勝手に数世代前で起動すると気付けないため.
func (u *ResoniteVersionUsecase) resolveLatest(ctx context.Context, branch entity.ResoniteVersionBranch) (string, error) {
	latest, err := u.repo.GetLatestByBranch(ctx, branch)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", errors.Errorf("no resonite_versions found for branch=%s (versions.json fetched?)", branch)
		}

		return "", errors.Wrap(err, 0)
	}

	return u.builtTagOrNotBuilt(ctx, latest)
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
