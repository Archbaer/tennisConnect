package location

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"tennisconnect/internal/testutil"
)

func migrationFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(testutil.MigrationsDir(), "*.sql"))
	if err != nil {
		t.Fatalf("find migrations: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func applyMigrationsUpTo(t *testing.T, ctx context.Context, conn *sql.Conn, limit string) {
	t.Helper()
	for _, path := range migrationFiles(t) {
		if filepath.Base(path) > limit {
			continue
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
			t.Fatalf("apply migration %s: %v", path, err)
		}
	}
}

func applyMigrationFile(t *testing.T, ctx context.Context, conn *sql.Conn, name string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(testutil.MigrationsDir(), name))
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	if _, err := conn.ExecContext(ctx, string(contents)); err != nil {
		t.Fatalf("apply migration %s: %v", name, err)
	}
}

func TestMigration000005(t *testing.T) {
	if testutil.DB == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	conn, err := testutil.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{
			"ROLLBACK",
			"SET search_path TO public",
			"DROP SCHEMA IF EXISTS migration_000005 CASCADE",
			"DROP SCHEMA IF EXISTS migration_000005_empty CASCADE",
		} {
			if _, err := conn.ExecContext(ctx, query); err != nil {
				t.Errorf("migration cleanup %q: %v", query, err)
			}
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close migration test connection: %v", err)
		}
	})

	// Upgrade path: existing users, profiles and photos survive 000005.
	for _, statement := range []string{
		"DROP SCHEMA IF EXISTS migration_000005 CASCADE",
		"CREATE SCHEMA migration_000005",
		"SET search_path TO migration_000005, public",
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare upgrade schema: %v", err)
		}
	}
	applyMigrationsUpTo(t, ctx, conn, "000004_photos.sql")

	var userID string
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date)
		VALUES ('\x01', decode(md5('migration-user-a') || md5('migration-user-b'), 'hex'), '1990-01-01')
		RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO profiles (user_id, display_name, gender, skill_level, gender_preference, min_age, max_age)
		VALUES ($1, 'Player', 'woman', 'intermediate', 'all', 18, 120)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO photos (owner_id, object_key, display_order, moderation_status)
		VALUES ($1, 'photos/migration', 1, 'approved')`, userID); err != nil {
		t.Fatal(err)
	}

	applyMigrationFile(t, ctx, conn, "000005_locations.sql")

	var users, photos int
	if err := conn.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM photos)").Scan(&users, &photos); err != nil {
		t.Fatal(err)
	}
	if users != 1 || photos != 1 {
		t.Fatalf("upgrade preserved users=%d photos=%d", users, photos)
	}

	// Both GiST indexes must exist for the discovery branches.
	for _, index := range []string{"locations_home_gist", "locations_travel_gist"} {
		var definition string
		if err := conn.QueryRowContext(ctx,
			"SELECT indexdef FROM pg_indexes WHERE schemaname = 'migration_000005' AND indexname = $1",
			index).Scan(&definition); err != nil {
			t.Fatalf("index %s: %v", index, err)
		}
		if !strings.Contains(strings.ToUpper(definition), "USING GIST") {
			t.Fatalf("index %s is not GiST: %s", index, definition)
		}
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label, travel_area_label)
		VALUES ($1, ST_SetSRID(ST_MakePoint(-0.1611, 51.4649), 4326)::geography, 'Battersea', 'Edinburgh')`,
		userID); err == nil {
		t.Fatal("half-set travel was accepted")
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label, travel_area_label, travel_expires_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint(-0.1611, 51.4649), 4326)::geography, 'Battersea', $2, now())`,
		userID, strings.Repeat("x", 81)); err == nil {
		t.Fatal("over-long travel label was accepted")
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label)
		VALUES ($1, ST_SetSRID(ST_MakePoint(-0.1611, 51.4649), 4326)::geography, '')`, userID); err == nil {
		t.Fatal("empty home label was accepted")
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO locations (user_id, home, home_area_label, travel, travel_area_label, travel_expires_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint(-0.1611, 51.4649), 4326)::geography, 'Battersea',
			ST_SetSRID(ST_MakePoint(-3.1883, 55.9533), 4326)::geography, 'Edinburgh', now() + interval '7 days')`,
		userID); err != nil {
		t.Fatalf("complete travel row rejected: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	var locations int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM locations").Scan(&locations); err != nil {
		t.Fatal(err)
	}
	if locations != 0 {
		t.Fatalf("deleting a user left %d location rows", locations)
	}

	// Empty-schema path: every migration applies in order.
	for _, statement := range []string{
		"SET search_path TO public",
		"DROP SCHEMA IF EXISTS migration_000005_empty CASCADE",
		"CREATE SCHEMA migration_000005_empty",
		"SET search_path TO migration_000005_empty, public",
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare empty schema: %v", err)
		}
	}
	applyMigrationsUpTo(t, ctx, conn, "999999")
	var table string
	if err := conn.QueryRowContext(ctx, "SELECT to_regclass('migration_000005_empty.locations')::text").Scan(&table); err != nil {
		t.Fatal(err)
	}
	if table == "" {
		t.Fatal("empty-schema migration did not create locations")
	}
}
