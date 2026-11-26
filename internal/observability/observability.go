// Package observability provides structured request logging with correlation
// IDs and panic recovery. It never logs query strings, headers or request bodies.
package observability

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	actorKey
)

// New returns a JSON or text logger. JSON is the production default.
func New(level, format string, writer io.Writer) (*slog.Logger, error) {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		slogLevel = slog.LevelInfo
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error")
	}
	if writer == nil {
		writer = os.Stdout
	}
	options := &slog.HandlerOptions{Level: slogLevel}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		return slog.New(slog.NewJSONHandler(writer, options)), nil
	case "text":
		return slog.New(slog.NewTextHandler(writer, options)), nil
	default:
		return nil, fmt.Errorf("LOG_FORMAT must be json or text")
	}
}

type actorHolder struct {
	ref string
}

// WithActor records a pseudonymous actor reference for access logs.
func WithActor(ctx context.Context, userID string) context.Context {
	if holder, ok := ctx.Value(actorKey).(*actorHolder); ok {
		holder.ref = actorRef(userID)
		return ctx
	}
	return context.WithValue(ctx, actorKey, &actorHolder{ref: actorRef(userID)})
}

// ActorRef returns the first 16 hex characters of SHA-256(user id).
func ActorRef(userID string) string { return actorRef(userID) }

func actorRef(userID string) string {
	if userID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(sum[:])[:16]
}

func requestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	written, err := w.ResponseWriter.Write(data)
	w.bytes += written
	return written, err
}

// Middleware adds a request ID, logs one event per request and recovers panics.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := requestID()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		ctx = context.WithValue(ctx, actorKey, &actorHolder{})
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id)
		response := &responseWriter{ResponseWriter: w}

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic", "request_id", id, "stack", string(debug.Stack()))
				if response.status == 0 {
					http.Error(response, "temporarily unavailable", http.StatusInternalServerError)
				}
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			attrs := []slog.Attr{
				slog.String("event", "http_request"),
				slog.String("request_id", id),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", response.status),
				slog.Int64("duration_ms", time.Since(started).Milliseconds()),
				slog.Int("bytes_written", response.bytes),
			}
			if holder, ok := ctx.Value(actorKey).(*actorHolder); ok && holder.ref != "" {
				attrs = append(attrs, slog.String("actor_ref", holder.ref))
			}
			level := slog.LevelInfo
			if response.status >= 500 {
				level = slog.LevelError
			} else if response.status >= 400 {
				level = slog.LevelWarn
			}
			logger.LogAttrs(ctx, level, "http_request", attrs...)
		}()

		next.ServeHTTP(response, r)
	})
}
