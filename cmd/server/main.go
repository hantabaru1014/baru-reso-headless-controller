package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/app"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
)

const (
	flagHost    = "host"
	flagFDev    = "fdev"
	flagFDevURL = "fdev-url"
)

var (
	hostAddress       = flag.String(flagHost, "", "The address to serve the server (overrides HOST env)")
	isFrontDev        = flag.Bool(flagFDev, false, "Whether to use the front-end development server (overrides FDEV env)")
	frontDevServerUrl = flag.String(flagFDevURL, "", "The URL of the front-end development server (overrides FDEV_URL env)")
)

func main() {
	flag.Parse()

	cfg, err := config.LoadEnvConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	applyFlagOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid config", "error", err)
		os.Exit(1)
	}

	auth.Init(cfg.Auth.JWTSecret)

	s := app.InitializeServer(cfg)

	frontURL := ""
	if cfg.Server.FrontDevMode {
		frontURL = cfg.Server.FrontDevURL
		slog.Info("Using front-end development server", "url", frontURL)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("Starting server", "address", cfg.Server.Host)
		if err := s.ListenAndServe(cfg.Server.Host, frontURL); err != nil {
			errCh <- errors.Wrap(err, 0)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		slog.Info("Shutdown signal received")
	case err := <-errCh:
		slog.Error("Server error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
}

// applyFlagOverrides applies command-line flag values to config if explicitly set
func applyFlagOverrides(cfg *config.EnvConfig) {
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case flagHost:
			cfg.Server.Host = *hostAddress
		case flagFDev:
			cfg.Server.FrontDevMode = *isFrontDev
		case flagFDevURL:
			cfg.Server.FrontDevURL = *frontDevServerUrl
		}
	})
}
