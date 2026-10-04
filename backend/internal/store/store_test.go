package store

import (
	"context"
	"os"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one embedded migration")
	}
	for i, m := range migrations {
		if i > 0 && m.version <= migrations[i-1].version {
			t.Fatalf("migrations out of order: %d after %d", m.version, migrations[i-1].version)
		}
		if m.sql == "" {
			t.Fatalf("migration %s is empty", m.name)
		}
	}
	if migrations[0].name != "0001_identity.sql" {
		t.Fatalf("first migration = %q", migrations[0].name)
	}
}

// TestMigrateIntegration exercises the real migrator against Postgres. It is
// skipped unless FORGELAB_TEST_DATABASE_URL points at a database the caller
// is happy to migrate (CI provides one).
func TestMigrateIntegration(t *testing.T) {
	dsn := os.Getenv("FORGELAB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FORGELAB_TEST_DATABASE_URL to run store integration tests")
	}
	ctx := context.Background()
	st, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("second migrate must be a no-op: %v", err)
	}
	for _, table := range []string{"schema_migrations", "users", "oauth_accounts", "sessions"} {
		var exists bool
		if err := st.Pool().QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("table %q was not created", table)
		}
	}
}
