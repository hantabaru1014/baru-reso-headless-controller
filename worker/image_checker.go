package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

const imageCheckTimeout = 5 * time.Minute

// NewImageObserver receives notifications when ImageChecker detects a
// brand-new container tag (one not seen on the previous poll). Handlers
// must be cheap and non-blocking; long work belongs in a goroutine the
// handler owns. The upgrade orchestrator subscribes here so the registry
// is only polled in one place.
type NewImageObserver func(ctx context.Context, latest *port.ContainerImage)

// ImageChecker periodically polls the container registry for new tags of
// the configured headless image. When a brand-new tag is observed it (a)
// optionally pulls it locally if AUTO_PULL_NEW_IMAGE is enabled and (b)
// notifies all subscribed observers — the upgrade orchestrator uses this
// to enrol opted-in hosts without running its own polling loop.
type ImageChecker struct {
	dc *hostconnector.DockerHostConnector

	interval         time.Duration
	autoPullNewImage bool
	lastTag          *port.ContainerImage

	mu        sync.Mutex
	observers []NewImageObserver
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

// Subscribe registers a new-tag observer. Intended for wire-time setup;
// observers added after Run has begun will still receive future events.
func (c *ImageChecker) Subscribe(handler NewImageObserver) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.observers = append(c.observers, handler)
}

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

	if c.autoPullNewImage {
		slog.Info("pulling latest container image", "tag", newestTag)

		if _, err := c.dc.PullContainerImage(ctx, newestTag.Tag); err != nil {
			slog.Error("failed to pull container image", "tag", newestTag, "error", err)
		} else {
			slog.Info("successfully pulled container image", "tag", newestTag)
		}
	}

	// Observers are notified independently of autoPull so the upgrade
	// orchestrator can react even when the controller does not
	// pre-pull. The pull above (when enabled) just warms the local
	// cache so the eventual restart is faster.
	c.notify(ctx, newestTag)
}

func (c *ImageChecker) notify(ctx context.Context, latest *port.ContainerImage) {
	c.mu.Lock()
	observers := append([]NewImageObserver(nil), c.observers...)
	c.mu.Unlock()

	for _, h := range observers {
		c.callObserver(ctx, h, latest)
	}
}

// callObserver isolates each observer with a panic boundary so a
// misbehaving subscriber cannot kill ImageChecker's tick loop.
func (c *ImageChecker) callObserver(ctx context.Context, h NewImageObserver, latest *port.ContainerImage) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("new-image observer panicked", "panic", r)
		}
	}()

	h(ctx, latest)
}
