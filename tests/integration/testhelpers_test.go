//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func requirePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	// Run migrations
	if err := runMigrations(ctx, connStr); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	return pool
}

func runMigrations(ctx context.Context, connStr string) error {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return err
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS citext`)
	if err != nil {
		return fmt.Errorf("create citext: %w", err)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email         CITEXT UNIQUE NOT NULL,
			password_hash TEXT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS feature_requests (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			author_id   UUID NOT NULL REFERENCES users(id),
			title       TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			vote_count  INTEGER NOT NULL DEFAULT 0,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS votes (
			user_id    UUID NOT NULL REFERENCES users(id),
			request_id UUID NOT NULL REFERENCES feature_requests(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, request_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fr_top     ON feature_requests (vote_count DESC, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fr_created ON feature_requests (created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_votes_request ON votes (request_id)`,
	}

	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration %q: %w", m[:40], err)
		}
	}
	return nil
}
