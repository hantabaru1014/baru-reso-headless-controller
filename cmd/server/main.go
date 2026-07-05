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
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
)

const (
	flagHost        = "host"
	flagFDev        = "fdev"
	flagFDevURL     = "fdev-url"
	flagMigrateOnly = "migrate-only"
)

var (
	hostAddress       = flag.String(flagHost, "", "The address to serve the server (overrides HOST env)")
	isFrontDev        = flag.Bool(flagFDev, false, "Whether to use the front-end development server (overrides FDEV env)")
	frontDevServerUrl = flag.String(flagFDevURL, "", "The URL of the front-end development server (overrides FDEV_URL env)")
	migrateOnly       = flag.Bool(flagMigrateOnly, false, "Run DB migrations and exit without starting the server")
)

func main() {
	flag.Parse()

	cfg, err := config.LoadEnvConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// -migrate-only は最小 config (DB_URL) で走らせられるように、
	// フルの Validate と applyFlagOverrides より前で分岐する.
	if *migrateOnly {
		if cfg.Database.URL == "" {
			slog.Error("DB_URL is required")
			os.Exit(1)
		}

		if err := db.Migrate(cfg.Database.URL); err != nil {
			slog.Error("Failed to run database migrations", "error", err)
			os.Exit(1)
		}

		slog.Info("Migration-only mode: exiting after successful migration")

		return
	}

	applyFlagOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid config", "error", err)
		os.Exit(1)
	}

	if err := db.Migrate(cfg.Database.URL); err != nil {
		slog.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}

	auth.Init(cfg.Auth.JWTSecret)

	s, err := app.InitializeServer(cfg)
	if err != nil {
		slog.Error("Failed to initialize server", "error", err)
		os.Exit(1)
	}

	frontURL := ""
	if cfg.Server.FrontDevMode {
		frontURL = cfg.Server.FrontDevURL
		slog.Info("Using front-end development server", "url", frontURL)
	}

	errCh := make(chan error, 1)

	go func() {
		slog.Info("Starting server", "address", cfg.Server.Host)

		err := s.ListenAndServe(cfg.Server.Host, frontURL)
		if err != nil {
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

// applyFlagOverrides applies command-line flag values to config if explicitly set.
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
