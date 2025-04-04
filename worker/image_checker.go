package worker

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type ImageChecker struct {
	scheduler *gocron.Scheduler
	repo      port.HeadlessHostRepository
	interval  time.Duration
	lastTag   *string
}

func NewImageChecker(repo port.HeadlessHostRepository) *ImageChecker {
	// 環境変数から設定を読み込む (秒単位)
	interval := 15 * time.Second // デフォルトは15秒に1回
	if envInterval := os.Getenv("IMAGE_CHECK_INTERVAL_SEC"); envInterval != "" {
		if seconds, err := strconv.Atoi(envInterval); err == nil && seconds > 0 {
			interval = time.Duration(seconds) * time.Second
		}
	}

	return &ImageChecker{
		scheduler: gocron.NewScheduler(time.UTC),
		repo:      repo,
		interval:  interval,
		lastTag:   nil,
	}
}

func (ic *ImageChecker) Start() {
	// 初回実行
	ic.checkNewImages()

	// スケジュールを設定
	_, err := ic.scheduler.Every(ic.interval).Do(ic.checkNewImages)
	if err != nil {
		slog.Error("Failed to schedule image check", "error", err)
		return
	}

	// スケジューラを開始
	ic.scheduler.StartAsync()
	slog.Debug("Container image checker started", "interval", ic.interval)
}

func (ic *ImageChecker) Stop() {
	ic.scheduler.Stop()
	slog.Debug("Container image checker stopped")
}

func (ic *ImageChecker) checkNewImages() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tags, err := ic.repo.ListContainerTags(ctx, ic.lastTag)
	if err != nil {
		slog.Error("Failed to list container tags", "error", err)
		return
	}

	if len(tags) == 0 {
		return
	}

	// 最新のタグは配列の最後に入っている
	newestTag := tags[len(tags)-1]

	// 前回の最新タグがない、またはタグが変わった場合に通知
	if ic.lastTag == nil || *ic.lastTag != newestTag {
		slog.Info("New container image found", "latestTag", newestTag)
		// 最新のタグを保存
		ic.lastTag = &newestTag

		// 必要に応じて新しいイメージをプル
		if os.Getenv("AUTO_PULL_NEW_IMAGE") == "true" {
			slog.Info("Pulling latest container image", "tag", newestTag)
			if err := ic.repo.PullContainerImage(ctx, newestTag); err != nil {
				slog.Error("Failed to pull container image", "tag", newestTag, "error", err)
			} else {
				slog.Info("Successfully pulled container image", "tag", newestTag)
			}
		}
	}
}
