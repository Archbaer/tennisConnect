package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type config struct {
	databaseURL        string
	address            string
	appBaseURL         string
	emailFrom          string
	resendAPIKey       string
	emailEncryptionKey []byte
	emailHMACKey       []byte
	cookieSecure       bool
	trustProxy         bool
}

func loadConfig() (config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" ||
		(parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return config{}, errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return config{}, errors.New("PORT must be between 1 and 65535")
	}

	cfg := config{databaseURL: databaseURL, address: ":" + port}
	if err := loadAuthConfig(&cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func healthHandler(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "unavailable\n")
			return
		}
		_, _ = io.WriteString(w, "ok\n")
	}
}

func newMux(auth *authService, ping func(context.Context) error) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthHandler(ping))
	mux.Handle("POST /api/v1/auth/link", http.HandlerFunc(auth.requestLink))
	mux.Handle("GET /auth/verify", http.HandlerFunc(auth.confirmLink))
	mux.Handle("POST /api/v1/auth/verify", http.HandlerFunc(auth.verifyLink))
	mux.Handle("POST /api/v1/auth/register", http.HandlerFunc(auth.register))
	mux.Handle("POST /api/v1/auth/sign-out", http.HandlerFunc(auth.signOut))
	mux.Handle("GET /api/v1/auth/session", http.HandlerFunc(auth.session))
	return mux
}

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.databaseURL)
	if err != nil {
		return errors.New("could not configure database connection")
	}
	defer db.Close()
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(15)

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := db.PingContext(pingCtx); err != nil {
		return errors.New("database is unavailable")
	}

	mailer := resendMailer{
		apiKey:   cfg.resendAPIKey,
		from:     cfg.emailFrom,
		endpoint: "https://api.resend.com/emails",
		client:   &http.Client{Timeout: 5 * time.Second},
	}
	auth := &authService{
		db:                 db,
		appBaseURL:         cfg.appBaseURL,
		emailEncryptionKey: cfg.emailEncryptionKey,
		emailHMACKey:       cfg.emailHMACKey,
		cookieSecure:       cfg.cookieSecure,
		trustProxy:         cfg.trustProxy,
		sendMagicLink:      mailer.sendMagicLink,
	}
	server := &http.Server{
		Addr:              cfg.address,
		Handler:           newMux(auth, db.PingContext),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		serverError <- server.ListenAndServe()
	}()

	cleanupTicker := time.NewTicker(time.Hour)
	defer cleanupTicker.Stop()
	for {
		select {
		case err := <-serverError:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("HTTP server failed: %w", err)
		case <-cleanupTicker.C:
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := auth.cleanupExpired(cleanupCtx)
			cancel()
			if err != nil {
				log.Printf("auth cleanup failed: %v", err)
			}
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				return errors.New("HTTP server shutdown failed")
			}
			return nil
		}
	}
}
