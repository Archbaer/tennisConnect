package testutil

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// DB is nil unless TEST_DATABASE_URL points at the test database.
var DB *sql.DB

// testLockKey serialises packages that reset the shared test schema.
const testLockKey = 0x74656e6e6973

func validTestDatabaseURL(raw string) bool {
	parsed, err := pgconn.ParseConfig(raw)
	return err == nil && parsed.Database == "tennisconnect_test"
}

// MigrationsDir returns the repository migrations directory from this source
// file's location so tests do not depend on the working directory.
func MigrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testutil: cannot locate source file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// RunTestMain prepares a clean schema once per package and holds a PostgreSQL
// advisory lock for the whole run so parallel packages serialise on it.
func RunTestMain(m *testing.M) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		os.Exit(m.Run())
	}
	if !validTestDatabaseURL(databaseURL) {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL must use the tennisconnect_test database")
		os.Exit(1)
	}

	var err error
	DB, err = sql.Open("pgx", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open test database: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	lockConn, err := DB.Conn(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire test lock connection: %v\n", err)
		os.Exit(1)
	}
	if _, err := lockConn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", int64(testLockKey)); err != nil {
		fmt.Fprintf(os.Stderr, "lock test database: %v\n", err)
		os.Exit(1)
	}
	if _, err := DB.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		fmt.Fprintf(os.Stderr, "reset test database: %v\n", err)
		os.Exit(1)
	}
	migrations, err := filepath.Glob(filepath.Join(MigrationsDir(), "*.sql"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "find test migrations: %v\n", err)
		os.Exit(1)
	}
	for _, path := range migrations {
		contents, err := os.ReadFile(path)
		if err == nil {
			_, err = DB.Exec(string(contents))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "apply test migration %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	code := m.Run()
	_ = lockConn.Close()
	_ = DB.Close()
	os.Exit(code)
}

// ResetTables empties every table the test suite writes to.
func ResetTables(t *testing.T) {
	t.Helper()
	requireDB(t)
	if _, err := DB.Exec("TRUNCATE login_tokens, sessions, auth_rate_limits, users CASCADE"); err != nil {
		t.Fatalf("reset auth tables: %v", err)
	}
}

// InsertUser stores an account with the supplied email and returns its id.
func InsertUser(t *testing.T, email string) string {
	t.Helper()
	requireDB(t)
	digest := sha256.Sum256([]byte(email))
	var userID string
	err := DB.QueryRow(`
		INSERT INTO users (email_encrypted, email_lookup_hmac, birth_date)
		VALUES ($1, $2, '1990-01-01') RETURNING id`, []byte{1}, digest[:]).Scan(&userID)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return userID
}

func requireDB(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
}
