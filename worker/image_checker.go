package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

const imageCheckTimeout = 5 * time.Minute

// ImageChecker periodically polls the container registry for new tags of
// the configured headless image and, if AUTO_PULL_NEW_IMAGE is enabled,
// pulls them so a new container start does not have to wait for the
// download.
type ImageChecker struct {
	dc *hostconnector.DockerHostConnector

	interval         time.Duration
	autoPullNewImage bool
	lastTag          *port.ContainerImage
}

var _ Runner = (*ImageChecker)(nil)

func NewImageChecker(dc *hostconnector.DockerHostConnector, cfg *config.WorkerConfig) *ImageChecker {
	return &ImageChecker{
		dc:               dc,
		interval:         cfg.ImageCheckInterval,
		autoPullNewImage: cfg.AutoPullNewImage,
	}
}

func (c *ImageChecker) Name() string { return "image-checker" }

func (c *ImageChecker) Run(ctx context.Context) error {
	c.checkNewImages(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.checkNewImages(ctx)
		}
	}
}

func (c *ImageChecker) checkNewImages(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, imageCheckTimeout)
	defer cancel()

	var lastTag *string
	if c.lastTag != nil {
		lastTag = &c.lastTag.Tag
	}

	tags, err := c.dc.ListContainerTags(ctx, lastTag)
	if err != nil {
		slog.Error("failed to list container tags", "error", err)

		return
	}

	if len(tags) == 0 {
		return
	}

	// 最新のタグは配列の最後に入っている
	newestTag := tags[len(tags)-1]

	if c.lastTag != nil && c.lastTag.Tag == newestTag.Tag {
		return
	}

	slog.Info("new container image found", "latestTag", newestTag)
	c.lastTag = newestTag

	if !c.autoPullNewImage {
		return
	}

	slog.Info("pulling latest container image", "tag", newestTag)

	if _, err := c.dc.PullContainerImage(ctx, newestTag.Tag); err != nil {
		slog.Error("failed to pull container image", "tag", newestTag, "error", err)
	} else {
		slog.Info("successfully pulled container image", "tag", newestTag)
	}
}
