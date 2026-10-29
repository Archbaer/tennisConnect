package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

var testDB *sql.DB

func validTestDatabaseURL(raw string) bool {
	parsed, err := pgconn.ParseConfig(raw)
	return err == nil && parsed.Database == "tennisconnect_test"
}

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		os.Exit(m.Run())
	}
	if !validTestDatabaseURL(databaseURL) {
		fmt.Fprintln(os.Stderr, "TEST_DATABASE_URL must use the tennisconnect_test database")
		os.Exit(1)
	}

	var err error
	testDB, err = sql.Open("pgx", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open test database: %v\n", err)
		os.Exit(1)
	}
	if _, err = testDB.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		fmt.Fprintf(os.Stderr, "reset test database: %v\n", err)
		os.Exit(1)
	}
	migrations, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "find test migrations: %v\n", err)
		os.Exit(1)
	}
	for _, path := range migrations {
		contents, err := os.ReadFile(path)
		if err == nil {
			_, err = testDB.Exec(string(contents))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "apply test migration %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	code := m.Run()
	_ = testDB.Close()
	os.Exit(code)
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

func resetAuthTables(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	if _, err := testDB.Exec("TRUNCATE login_tokens, sessions, auth_rate_limits, users CASCADE"); err != nil {
		t.Fatalf("reset auth tables: %v", err)
	}
}
