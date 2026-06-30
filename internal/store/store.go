// Package store owns the SQLite connection and the startup schema migration.
// It exposes a thin wrapper over *sql.DB that the domain packages embed; it
// holds no business logic itself, keeping persistence concerns in one place.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ArfaMujahid/invoice-generator/migrations"

	// modernc.org/sqlite is a pure-Go (cgo-free) SQLite driver. It is chosen
	// over mattn/go-sqlite3 so the application cross-compiles to a single
	// static Linux binary with no C toolchain (SRS §2.5, NFR-5).
	_ "modernc.org/sqlite"
)

// Store wraps the database handle. Domain repositories take a *Store so they all
// share one connection pool and the same migration-applied schema.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path, configures it for
// safe concurrent use, and applies the embedded schema. The provided context
// bounds the connection check and migration.
func Open(ctx context.Context, path string) (*Store, error) {
	// _pragma DSN options are applied to every pooled connection. foreign_keys
	// enforces FK constraints (off by default in SQLite); busy_timeout makes
	// writers wait briefly instead of failing immediately under contention;
	// journal_mode=WAL improves read/write concurrency.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %q: %w", path, err)
	}

	// SQLite tolerates a single writer; capping connections avoids spurious
	// "database is locked" errors while still allowing concurrent readers.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to sqlite %q: %w", path, err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// migrate applies the embedded schema. Every statement is idempotent, so this is
// safe to run on every startup.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, migrations.Schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	return nil
}

// DB returns the underlying *sql.DB for use by domain repositories. They share
// this single handle rather than opening their own.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close releases the database handle. It is safe to call once during shutdown.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing store: %w", err)
	}
	return nil
}
