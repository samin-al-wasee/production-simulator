package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationLock serializes migration runs across instances. Its value spells
// "FORGELAB" and is arbitrary but fixed.
const migrationLock int64 = 0x464f5247454c4142

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("migration %q: name must start with a number", e.Name())
		}
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: version, name: e.Name(), sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// Migrate applies every embedded migration not yet recorded in
// schema_migrations, oldest first, each in its own transaction, under an
// advisory lock so concurrent instances apply each one exactly once.
func (s *Store) Migrate(ctx context.Context) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLock)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    integer PRIMARY KEY,
		name       text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, m := range migrations {
		var applied bool
		if err := conn.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", m.version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if applied {
			continue
		}
		if err := apply(ctx, conn, m); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return nil
}

func apply(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.version, m.name,
	); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
