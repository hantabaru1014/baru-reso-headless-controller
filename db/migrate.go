package db

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate runs all pending migrations against dbURL using the embedded
// migration files. Idempotent — safe to call on every server startup.
// Errors returned from underlying drivers may embed dbURL (e.g. dsn parse
// failures); Migrate rewrites them to use the redacted form so credentials
// never reach caller-level logging.
func Migrate(dbURL string) error {
	d, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	m, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		return fmt.Errorf("init migration: %w", redactURL(err, dbURL))
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			slog.Warn("failed to close migration source", "error", srcErr)
		}

		if dbErr != nil {
			slog.Warn("failed to close migration db", "error", dbErr)
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("Database schema is up to date")

			return nil
		}

		return redactURL(err, dbURL)
	}

	slog.Info("Database migration applied")

	return nil
}

// redactURL replaces any occurrence of dbURL inside err.Error() with a form
// whose password is masked. Safeguards against drivers that echo the raw DSN
// in error messages.
func redactURL(err error, dbURL string) error {
	if err == nil {
		return nil
	}

	parsed, perr := url.Parse(dbURL)
	if perr != nil {
		return err
	}

	redacted := parsed.Redacted()
	if redacted == dbURL {
		return err
	}

	return errors.New(strings.ReplaceAll(err.Error(), dbURL, redacted))
}
