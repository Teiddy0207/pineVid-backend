// Package pgtest gives repository tests a throwaway PostgreSQL database with
// every migration applied. It is only imported from _test files.
//
// Set TEST_PG_URL to any database on the server the tests may use (for the
// compose stack: postgres://user:<password>@localhost:5435/db). Each call to
// New creates a fresh database next to it and drops it when the test ends,
// so the developer's own data is never touched. Without TEST_PG_URL the
// calling test is skipped.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// New returns a Postgres handle on a new, fully migrated database.
func New(t *testing.T) *postgres.Postgres {
	t.Helper()

	adminURL := os.Getenv("TEST_PG_URL")
	if adminURL == "" {
		t.Skip("TEST_PG_URL not set; skipping PostgreSQL-backed test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbName := "pipevid_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("pgtest: connect %s: %v", redact(adminURL), err)
	}
	defer admin.Close(ctx)

	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			t.Logf("pgtest: drop %s: %v", dbName, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); err != nil {
			t.Logf("pgtest: drop %s: %v", dbName, err)
		}
	})

	dbURL, err := withDatabase(adminURL, dbName)
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}

	if err := migrate(ctx, dbURL); err != nil {
		t.Fatalf("pgtest: migrate: %v", err)
	}

	pg, err := postgres.New(dbURL, postgres.MaxPoolSize(20))
	if err != nil {
		t.Fatalf("pgtest: open pool: %v", err)
	}
	t.Cleanup(pg.Close)

	return pg
}

func withDatabase(rawURL, dbName string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse TEST_PG_URL: %w", err)
	}
	u.Path = "/" + dbName
	q := u.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// migrate applies every *.up.sql in backend/migrations in filename order,
// the same order golang-migrate uses.
func migrate(ctx context.Context, dbURL string) error {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "migrations")

	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found in %s", dir)
	}
	sort.Strings(files)

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		// Simple protocol: a migration file may hold several statements.
		if _, err := conn.PgConn().Exec(ctx, string(sql)).ReadAll(); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

func redact(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "TEST_PG_URL"
	}
	return u.Redacted()
}
