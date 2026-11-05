package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	RunTestMain(m)
}

func TestDatabaseURLGuardRequiresExactTestDatabase(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "exact", url: "postgres://user:password@localhost/tennisconnect_test?sslmode=disable", want: true},
		{name: "suffix", url: "postgres://user:password@localhost/tennisconnect_test_backup", want: false},
		{name: "different", url: "postgres://user:password@localhost/tennisconnect", want: false},
		{name: "nested path", url: "postgres://user:password@localhost/tennisconnect_test/other", want: false},
		{name: "dbname override", url: "postgres://user:password@localhost/tennisconnect_test?dbname=tennisconnect", want: false},
		{name: "database override", url: "postgres://user:password@localhost/tennisconnect_test?database=tennisconnect", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validTestDatabaseURL(test.url); got != test.want {
				t.Fatalf("validTestDatabaseURL() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	requireDB(t)
	ctx := context.Background()
	conn, err := DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{"ROLLBACK", "SET search_path TO public", "DROP SCHEMA IF EXISTS migration_guard CASCADE"} {
			if _, err := conn.ExecContext(ctx, query); err != nil {
				t.Errorf("migration cleanup %q: %v", query, err)
			}
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close migration test connection: %v", err)
		}
	})
	if _, err := conn.ExecContext(ctx, "DROP SCHEMA IF EXISTS migration_guard CASCADE; CREATE SCHEMA migration_guard; SET search_path TO migration_guard;"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(MigrationsDir(), "000001_users.sql"))
	if err != nil {
		t.Fatal(err)
	}
	insertAt := strings.LastIndex(string(contents), ");")
	if insertAt < 0 {
		t.Fatal("migration has no table terminator")
	}
	migration := string(contents[:insertAt+2]) + "\nSELECT 1/0;\n" + string(contents[insertAt+2:])
	if _, err := conn.ExecContext(ctx, migration); err == nil {
		t.Fatal("fault-injected migration unexpectedly succeeded")
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatalf("rollback fault-injected migration: %v", err)
	}
	var exists bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'migration_guard' AND table_name = 'users')").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("failed migration left a partially created users table")
	}
}

func TestMigrationGuardRestoresPublicSchema(t *testing.T) {
	requireDB(t)
	var searchPath string
	if err := DB.QueryRow("SHOW search_path").Scan(&searchPath); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(searchPath, "migration_guard") {
		t.Fatalf("test connection retained migration schema: %s", searchPath)
	}
	var exists bool
	if err := DB.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'login_tokens')").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("public auth schema unavailable after migration guard test")
	}
}
