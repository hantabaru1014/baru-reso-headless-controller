// ContentPoller periodically fetches versions.json (from
// resonite-love/resonite-version-monitor) and reconciles the local
// resonite_versions table. It also pulls the builder image and reads its
// brhc.app-version label to detect Headless/AppVersion bumps.
//
// It replaces the old GHCR-tag-polling ImageChecker. Two auto-build
// triggers fire from here:
//
//  1. NEW versions detected on the `headless` or `prerelease` branch of
//     versions.json → enqueue BUILD_IMAGE for each.
//  2. `Headless/AppVersion` incremented in the builder image → enqueue
//     BUILD_IMAGE for the LATEST built version of `headless` /
//     `prerelease` whose `built_with_app_version` differs from the new
//     value. 古い版は対象にしない (ListStaleBuiltResoniteVersions 参照).
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

const contentCheckTimeout = 10 * time.Minute

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
	EnqueueBuildImage(ctx context.Context, req *hdlctrlv1.BuildResoniteImageRequest, createdBy *string) (string, error)
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
	c.checkAppVersion(ctx)
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

		jobID, err := c.jobs.EnqueueBuildImage(ctx, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: v.ManifestID,
			Branch:     string(v.Branch),
		}, &systemUser)
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

func (c *ContentPoller) checkAppVersion(ctx context.Context) {
	appVersion, err := c.versions.CurrentAppVersion(ctx)
	if err != nil {
		slog.Warn("content-poller: read builder image AppVersion failed", "err", err)

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

	// stale は「auto-build 対象ブランチごとの最新 built 1 件」なので高々ブランチ数.
	slog.Info("content-poller: builder image AppVersion changed",
		"appVersion", appVersion, "staleCount", len(stale))

	for _, v := range stale {
		systemUser := domain.SystemUserID

		jobID, err := c.jobs.EnqueueBuildImage(ctx, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: v.ManifestID,
			Branch:     string(v.Branch),
		}, &systemUser)
		if err != nil {
			slog.Warn("content-poller: enqueue rebuild failed",
				"manifest", v.ManifestID, "branch", v.Branch, "err", err)

			continue
		}

		slog.Info("content-poller: enqueued rebuild for stale version",
			"manifest", v.ManifestID, "branch", v.Branch, "job", jobID)
	}
}

func isAutoBuildBranch(b entity.ResoniteVersionBranch) bool {
	return slices.Contains(entity.AutoBuildBranches, b)
}
