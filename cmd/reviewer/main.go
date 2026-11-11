// Command reviewer creates a separate human review account. It is the only
// supported way to add a reviewer; players cannot promote themselves.
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"tennisconnect/internal/reviewer"
)

func main() {
	email := flag.String("email", "", "reviewer e-mail address")
	flag.Parse()
	if *email == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/reviewer -email reviewer@example.com")
		os.Exit(2)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	encryptionKey, err := decodeKey("EMAIL_ENCRYPTION_KEY")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	hmacKey, err := decodeKey("EMAIL_HMAC_KEY")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not configure database")
		os.Exit(1)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "database is unavailable")
		os.Exit(1)
	}
	id, err := reviewer.Create(ctx, db, *email, encryptionKey, hmacKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(id)
}

func decodeKey(name string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv(name))
	if err != nil || len(key) != 32 {
		return nil, errors.New(name + " must be a base64-encoded 32-byte key")
	}
	return key, nil
}
