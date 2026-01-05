package main

import (
	"log/slog"
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/app"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg, err := config.LoadEnvConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid config", "error", err)
		os.Exit(1)
	}

	auth.Init(cfg.Auth.JWTSecret)

	cli := app.InitializeCli(cfg)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
