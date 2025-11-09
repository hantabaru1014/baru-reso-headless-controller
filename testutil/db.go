package testutil

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetTestDBURL returns the test database URL by appending "_test" to the database name
func GetTestDBURL() string {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		panic("DB_URL environment variable is not set")
	}

	// Extract database name and replace with test database name
	re := regexp.MustCompile(`/([^/?]+)(\?|$)`)
	testDBURL := re.ReplaceAllString(dbURL, "/${1}_test$2")

	return testDBURL
}

// SetupTestDB creates a connection pool for the test database
func SetupTestDB(t *testing.T) (*db.Queries, *pgxpool.Pool) {
	t.Helper()

	testDBURL := GetTestDBURL()

	pool, err := pgxpool.New(t.Context(), testDBURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return db.New(pool), pool
}

// RunMigrations runs all database migrations for the test database
func RunMigrations(t *testing.T) {
	t.Helper()

	testDBURL := GetTestDBURL()
	m, err := migrate.New(
		"file://db/migrations",
		testDBURL,
	)
	if err != nil {
		t.Fatalf("failed to create migrate instance: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}
}

// CleanupTables truncates all tables in the test database
func CleanupTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	tables := []string{
		"sessions",
		"hosts",
		"headless_accounts",
		"users",
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := pool.Exec(t.Context(), query); err != nil {
			t.Logf("warning: failed to truncate table %s: %v", table, err)
		}
	}
}
