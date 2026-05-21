package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lsariol/plop/internal/db"
)

// NewTestDB connects to TEST_DATABASE_URL, runs migrations, and registers a
// cleanup that truncates all plop tables between tests. The test is skipped if
// TEST_DATABASE_URL is not set.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping database tests")
	}

	ctx := context.Background()
	pool, err := db.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.RunMigrations(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			TRUNCATE plop.pairing_codes, plop.desktops, plop.payloads,
			         plop.sessions, plop.users
			CASCADE
		`)
		pool.Close()
	})

	return pool
}
