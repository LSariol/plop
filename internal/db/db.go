package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS


func New(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	// Every new connection in the pool runs this on first use, so all
	// unqualified table references resolve to the plop schema first.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO plop, public")
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}


func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Ensure the schema exists before anything else (handles fresh DB or DROP SCHEMA).
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS plop`); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Create a tracking table if it doesn't exist
	_, err := pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS plop.schema_migrations (
            filename   TEXT PRIMARY KEY,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )
    `)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// Read all migration files from the embedded FS, in order
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		filename := entry.Name()

		// Check if already applied
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM plop.schema_migrations WHERE filename = $1)`,
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if exists {
			continue
		}

		// Read and run the SQL file
		sql, err := fs.ReadFile(migrationsFS, "migrations/"+filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}
		_, err = pool.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}

		// Record it as applied
		_, err = pool.Exec(ctx,
			`INSERT INTO plop.schema_migrations (filename) VALUES ($1)`,
			filename,
		)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		log.Printf("applied migration: %s", filename)
	}
	return nil
}
