// Package store is the application layer's Postgres persistence: a connection
// pool and the embedded schema migrations the API applies at startup. It is
// the only package that talks to the database, and no simulation package
// imports it, so the core stays headless and DB-free (ADR-0030).
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the application database.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to Postgres at url. Call Migrate before serving.
func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Pool exposes the connection pool to the application packages that build on
// the store (identity, ownership, progress, submissions).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
