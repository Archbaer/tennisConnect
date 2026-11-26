package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"tennisconnect/internal/account"
	"tennisconnect/internal/auth"
	"tennisconnect/internal/config"
	"tennisconnect/internal/discovery"
	"tennisconnect/internal/location"
	"tennisconnect/internal/matches"
	"tennisconnect/internal/moderation"
	"tennisconnect/internal/observability"
	"tennisconnect/internal/photos"
	"tennisconnect/internal/profile"
	"tennisconnect/internal/reviewer"
	"tennisconnect/internal/safety"
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

func NewMux(authSvc *auth.Service, photoSvc *photos.Service, locationSvc *location.Service, discoverySvc *discovery.Service, matchSvc *matches.Service, safetySvc *safety.Service, accountSvc *account.Service, reviewerSvc *reviewer.Service, ping func(context.Context) error) *http.ServeMux {
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
	mux.Handle("GET /api/v1/discovery/{id}", authSvc.RequireUser(discoverySvc.Detail))
	mux.Handle("POST /api/v1/reactions", authSvc.RequireUser(matchSvc.CreateReaction))
	mux.Handle("GET /api/v1/matches", authSvc.RequireUser(matchSvc.List))
	mux.Handle("GET /api/v1/matches/{id}", authSvc.RequireUser(matchSvc.Detail))
	mux.Handle("POST /api/v1/matches/{id}/reveal", authSvc.RequireUser(matchSvc.Reveal))
	mux.Handle("GET /api/v1/matches/{id}/instagram", authSvc.RequireUser(matchSvc.Instagram))
	mux.Handle("POST /api/v1/blocks", authSvc.RequireUser(matchSvc.Block))
	mux.Handle("POST /api/v1/reports", authSvc.RequireUser(safetySvc.Report))
	mux.Handle("DELETE /api/v1/account", authSvc.RequireUser(accountSvc.Delete))
	mux.Handle("GET /api/v1/account/export", authSvc.RequireUser(accountSvc.Export))
	mux.Handle("GET /api/v1/account/recovery", http.HandlerFunc(authSvc.RecoveryStatus))
	mux.Handle("POST /api/v1/account/recovery", http.HandlerFunc(authSvc.Recover))
	mux.Handle("DELETE /api/v1/account/recovery", http.HandlerFunc(authSvc.ContinueDeleting))
	mux.Handle("GET /api/v1/account/appeal", authSvc.RequireAccount(accountSvc.AppealStatus))
	mux.Handle("POST /api/v1/account/appeal", authSvc.RequireAccount(accountSvc.SubmitAppeal))
	mux.Handle("POST /api/v1/reviewer/auth/link", http.HandlerFunc(reviewerSvc.RequestLink))
	mux.Handle("GET /reviewer/verify", http.HandlerFunc(reviewerSvc.ConfirmLink))
	mux.Handle("POST /api/v1/reviewer/auth/verify", http.HandlerFunc(reviewerSvc.VerifyLink))
	mux.Handle("POST /api/v1/reviewer/auth/sign-out", http.HandlerFunc(reviewerSvc.SignOut))
	mux.Handle("POST /api/v1/reviewer/mfa/enroll", http.HandlerFunc(reviewerSvc.EnrollMFA))
	mux.Handle("POST /api/v1/reviewer/mfa/verify", http.HandlerFunc(reviewerSvc.VerifyMFA))
	mux.Handle("GET /api/v1/reviewer/reports", reviewerSvc.RequireReviewer(safetySvc.List))
	mux.Handle("GET /api/v1/reviewer/reports/{id}", reviewerSvc.RequireReviewer(safetySvc.Detail))
	mux.Handle("POST /api/v1/reviewer/reports/{id}/dismiss", reviewerSvc.RequireReviewer(safetySvc.Dismiss))
	mux.Handle("POST /api/v1/reviewer/reports/{id}/ban", reviewerSvc.RequireReviewer(safetySvc.Ban))
	return mux
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger, err := observability.New(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

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
	matchSvc := &matches.Service{DB: db, AppBaseURL: cfg.AppBaseURL}
	if photoStore != nil {
		matchSvc.Presigner = photoStore
	}

	mailer := auth.NewMailer(cfg.ResendAPIKey, cfg.EmailFrom)
	authSvc := &auth.Service{
		DB:                 db,
		AppBaseURL:         cfg.AppBaseURL,
		EmailEncryptionKey: cfg.EmailEncryptionKey,
		EmailHMACKey:       cfg.EmailHMACKey,
		CookieSecure:       cfg.CookieSecure,
		TrustProxy:         cfg.TrustProxy,
		SendMagicLink:      mailer,
	}
	safetySvc := &safety.Service{DB: db, AppBaseURL: cfg.AppBaseURL, HMACKey: cfg.EmailHMACKey}
	accountSvc := &account.Service{DB: db, AppBaseURL: cfg.AppBaseURL}
	reviewerSvc := &reviewer.Service{
		DB:                 db,
		AppBaseURL:         cfg.AppBaseURL,
		EmailEncryptionKey: cfg.EmailEncryptionKey,
		EmailHMACKey:       cfg.EmailHMACKey,
		CookieSecure:       cfg.CookieSecure,
		TrustProxy:         cfg.TrustProxy,
		SendMagicLink:      mailer,
	}
	if err := validateWebDir(cfg.WebDir); err != nil {
		return err
	}
	mux := NewMux(authSvc, photoSvc, locationSvc, discoverySvc, matchSvc, safetySvc, accountSvc, reviewerSvc, db.PingContext)
	if cfg.WebDir != "" {
		mux.Handle("/", spaHandler(cfg.WebDir))
	}
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           observability.Middleware(logger, mux),
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
			if err == nil {
				err = reviewerSvc.CleanupExpired(cleanupCtx)
			}
			if err == nil {
				err = safetySvc.CleanupExpired(cleanupCtx)
			}
			if err == nil {
				err = accountSvc.CleanupExpired(cleanupCtx)
			}
			cancel()
			if err != nil {
				logger.Error("cleanup failed", "event", "cleanup.failed", "error_kind", "cleanup_failed")
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
