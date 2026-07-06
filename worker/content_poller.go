// ContentPoller periodically fetches versions.json (from
// resonite-love/resonite-version-monitor) and reconciles the local
// resonite_versions table. It also polls the container repo
// (baru-reso-headless-container) via git for Headless/AppVersion bumps.
//
// It replaces the old GHCR-tag-polling ImageChecker. Two auto-build
// triggers fire from here:
//
//  1. NEW versions detected on the `headless` or `prerelease` branch of
//     versions.json → enqueue BUILD_IMAGE for each.
//  2. `Headless/AppVersion` incremented in the container repo → enqueue
//     BUILD_IMAGE for every currently-built version on `headless` or
//     `prerelease` whose `built_with_app_version` differs from the new
//     value.
//
// Successful builds fire ResoniteVersionUsecase's Subscribe observers so
// HostUpgradeOrchestrator can enroll RUNNING auto-update hosts.

package worker

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

const (
	contentCheckTimeout = 10 * time.Minute
	// staleBuildEnqueueLimit は AppVersion bump 検知時に 1 tick で enqueue する
	// 再ビルド job の上限. 溜まっている built 版が多い場合でも async_job キューを
	// 埋め尽くさないようキャップする. `stale` は released_at DESC で並んでいるので
	// 新しいものから優先的に再ビルドされる. 残りは次 tick で処理される.
	staleBuildEnqueueLimit = 5
)

// ContentPollerVersionAPI narrows ResoniteVersionUsecase to the surface
// this worker needs so tests can substitute a fake.
type ContentPollerVersionAPI interface {
	FetchAndUpsertVersions(ctx context.Context) (entity.ResoniteVersionList, error)
	CurrentAppVersion(ctx context.Context) (string, error)
	ListStaleBuilt(ctx context.Context, currentAppVersion string) (entity.ResoniteVersionList, error)
}

// ContentPollerJobEnqueuer は BUILD_IMAGE job を投入するインタフェース.
// async_job.Usecase.EnqueueBuildImage を裏で叩く.
type ContentPollerJobEnqueuer interface {
	EnqueueBuildImage(
		ctx context.Context,
		manifestID string,
		branch entity.ResoniteVersionBranch,
		thenStart *hdlctrlv1.StartHeadlessHostRequest,
		createdBy *string,
	) (string, error)
}

type ContentPoller struct {
	versions ContentPollerVersionAPI
	jobs     ContentPollerJobEnqueuer

	interval                  time.Duration
	autoBuildNewVersions      bool
	autoBuildOnAppVersionBump bool
}

var _ Runner = (*ContentPoller)(nil)

func NewContentPoller(
	versions ContentPollerVersionAPI,
	jobs ContentPollerJobEnqueuer,
	cfg *config.WorkerConfig,
) *ContentPoller {
	return &ContentPoller{
		versions:                  versions,
		jobs:                      jobs,
		interval:                  cfg.ContentCheckInterval,
		autoBuildNewVersions:      cfg.AutoBuildNewVersions,
		autoBuildOnAppVersionBump: cfg.AutoBuildOnAppVersionBump,
	}
}

func (c *ContentPoller) Name() string { return "content-poller" }

func (c *ContentPoller) Run(ctx context.Context) error {
	c.tick(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

func (c *ContentPoller) tick(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, contentCheckTimeout)
	defer cancel()

	c.checkVersions(ctx)
	c.checkContainerRepo(ctx)
}

func (c *ContentPoller) checkVersions(ctx context.Context) {
	newly, err := c.versions.FetchAndUpsertVersions(ctx)
	if err != nil {
		slog.Warn("content-poller: fetch versions.json failed", "err", err)

		return
	}

	if !c.autoBuildNewVersions {
		return
	}

	// `newly` is sorted newest-first (see FetchAndUpsertVersions).
	// 各 branch あたり最新 1 件だけ enqueue する: 初回ブート時に versions.json 全履歴を
	// 「新規」扱いして 100+ builds が並列 enqueue されるのを防ぐ.
	enqueued := map[entity.ResoniteVersionBranch]bool{}

	for _, v := range newly {
		if !isAutoBuildBranch(v.Branch) {
			continue
		}

		if v.GameVersion == nil {
			continue
		}

		if enqueued[v.Branch] {
			continue
		}

		systemUser := domain.SystemUserID

		jobID, err := c.jobs.EnqueueBuildImage(ctx, v.ManifestID, v.Branch, nil, &systemUser)
		if err != nil {
			slog.Warn("content-poller: enqueue build for new version failed",
				"manifest", v.ManifestID, "branch", v.Branch, "err", err)

			continue
		}

		enqueued[v.Branch] = true

		slog.Info("content-poller: enqueued build for new version",
			"manifest", v.ManifestID, "branch", v.Branch, "gameVersion", *v.GameVersion, "job", jobID)
	}
}

func (c *ContentPoller) checkContainerRepo(ctx context.Context) {
	appVersion, err := c.versions.CurrentAppVersion(ctx)
	if err != nil {
		slog.Warn("content-poller: read container repo AppVersion failed", "err", err)

		return
	}

	if !c.autoBuildOnAppVersionBump {
		return
	}

	stale, err := c.versions.ListStaleBuilt(ctx, appVersion)
	if err != nil {
		slog.Warn("content-poller: list stale built failed", "err", err)

		return
	}

	if len(stale) == 0 {
		return
	}

	slog.Info("content-poller: container repo AppVersion changed",
		"appVersion", appVersion, "staleCount", len(stale), "enqueueLimit", staleBuildEnqueueLimit)

	enqueuedCount := 0

	for _, v := range stale {
		if enqueuedCount >= staleBuildEnqueueLimit {
			slog.Info("content-poller: stale-build enqueue limit reached; remaining will be picked up on next tick",
				"remaining", len(stale)-enqueuedCount)

			break
		}

		systemUser := domain.SystemUserID

		jobID, err := c.jobs.EnqueueBuildImage(ctx, v.ManifestID, v.Branch, nil, &systemUser)
		if err != nil {
			slog.Warn("content-poller: enqueue rebuild failed",
				"manifest", v.ManifestID, "branch", v.Branch, "err", err)

			continue
		}

		enqueuedCount++

		slog.Info("content-poller: enqueued rebuild for stale version",
			"manifest", v.ManifestID, "branch", v.Branch, "job", jobID)
	}
}

func isAutoBuildBranch(b entity.ResoniteVersionBranch) bool {
	return slices.Contains(entity.AutoBuildBranches, b)
}
