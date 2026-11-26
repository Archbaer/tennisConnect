package main

import (
	"log/slog"
	"os"

	"tennisconnect/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := app.Run(); err != nil {
		logger.Error("startup failed", "event", "startup.failed", "error_kind", "startup_failed")
		os.Exit(1)
	}
}
