package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"tennisconnect/internal/auth"
	"tennisconnect/internal/config"
	"tennisconnect/internal/discovery"
	"tennisconnect/internal/location"
	"tennisconnect/internal/moderation"
	"tennisconnect/internal/photos"
	"tennisconnect/internal/profile"
	"tennisconnect/internal/storage"
)

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

func NewMux(authSvc *auth.Service, photoSvc *photos.Service, locationSvc *location.Service, discoverySvc *discovery.Service, ping func(context.Context) error) *http.ServeMux {
	mux := http.NewServeMux()
	profiles := &profile.Service{DB: authSvc.DB, AppBaseURL: authSvc.AppBaseURL}
	mux.Handle("GET /healthz", healthHandler(ping))
	mux.Handle("POST /api/v1/auth/link", http.HandlerFunc(authSvc.RequestLink))
	mux.Handle("GET /auth/verify", http.HandlerFunc(authSvc.ConfirmLink))
	mux.Handle("POST /api/v1/auth/verify", http.HandlerFunc(authSvc.VerifyLink))
	mux.Handle("POST /api/v1/auth/register", http.HandlerFunc(authSvc.Register))
	mux.Handle("POST /api/v1/auth/sign-out", http.HandlerFunc(authSvc.SignOut))
	mux.Handle("GET /api/v1/auth/session", http.HandlerFunc(authSvc.Session))
	mux.Handle("GET /api/v1/profile", authSvc.RequireUser(profiles.Get))
	mux.Handle("PUT /api/v1/profile", authSvc.RequireUser(profiles.Put))
	mux.Handle("POST /api/v1/photos/upload-url", authSvc.RequireUser(photoSvc.UploadURL))
	mux.Handle("POST /api/v1/photos/{id}/process", authSvc.RequireUser(photoSvc.Process))
	mux.Handle("GET /api/v1/photos", authSvc.RequireUser(photoSvc.List))
	mux.Handle("GET /api/v1/photos/{id}/url", authSvc.RequireUser(photoSvc.URL))
	mux.Handle("PUT /api/v1/location", authSvc.RequireUser(locationSvc.Put))
	mux.Handle("GET /api/v1/location", authSvc.RequireUser(locationSvc.Get))
	mux.Handle("PUT /api/v1/location/travel", authSvc.RequireUser(locationSvc.PutTravel))
	mux.Handle("DELETE /api/v1/location/travel", authSvc.RequireUser(locationSvc.DeleteTravel))
	mux.Handle("GET /api/v1/discovery", authSvc.RequireUser(discoverySvc.Get))
	return mux
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
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

	photoStore, err := storage.New(context.Background(), cfg)
	if err != nil {
		return err
	}
	photoSvc := &photos.Service{DB: db, AppBaseURL: cfg.AppBaseURL}
	if photoStore != nil {
		photoSvc.Storage = photoStore
	}
	if cfg.SightengineUser != "" {
		photoSvc.Moderator = moderation.New(cfg.SightengineUser, cfg.SightengineSecret)
	}

	locationSvc := &location.Service{DB: db, AppBaseURL: cfg.AppBaseURL}
	if cfg.MapboxAccessToken != "" {
		locationSvc.Geocoder = location.NewMapbox(cfg.MapboxAccessToken)
	}
	discoverySvc := &discovery.Service{DB: db}
	if photoStore != nil {
		discoverySvc.Presigner = photoStore
	}

	authSvc := &auth.Service{
		DB:                 db,
		AppBaseURL:         cfg.AppBaseURL,
		EmailEncryptionKey: cfg.EmailEncryptionKey,
		EmailHMACKey:       cfg.EmailHMACKey,
		CookieSecure:       cfg.CookieSecure,
		TrustProxy:         cfg.TrustProxy,
		SendMagicLink:      auth.NewMailer(cfg.ResendAPIKey, cfg.EmailFrom),
	}
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           NewMux(authSvc, photoSvc, locationSvc, discoverySvc, db.PingContext),
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
			err := authSvc.CleanupExpired(cleanupCtx)
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
