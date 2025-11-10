package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/app"
)

var (
	hostAddress       = flag.String("host", ":8014", "The address to serve the server")
	isFrontDev        = flag.Bool("fdev", false, "Whether to use the front-end development server")
	frontDevServerUrl = flag.String("fdev-url", "http://localhost:5173", "The URL of the front-end development server")
	shutdownTimeout   = flag.Duration("shutdown-timeout", 10*time.Second, "Maximum time to wait for server to shutdown gracefully")
	sessionPortMin    = flag.Int("session-port-min", 0, "Minimum port number for session allocation (0 = use system ephemeral ports)")
	sessionPortMax    = flag.Int("session-port-max", 0, "Maximum port number for session allocation (0 = use system ephemeral ports)")
)

func init() {
	// 環境変数からも読み込む
	flag.VisitAll(func(f *flag.Flag) {
		if s := os.Getenv(strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")); s != "" {
			f.Value.Set(s)
		}
	})
	flag.Parse()
}

func main() {
	s := app.InitializeServer()

	frontURL := ""
	if isFrontDev != nil && *isFrontDev {
		frontURL = *frontDevServerUrl
		slog.Info("Using front-end development server", "url", frontURL)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("Starting server", "address", *hostAddress)
		if err := s.ListenAndServe(*hostAddress, frontURL); err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
}
