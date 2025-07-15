// Package logger provides a logger implementation using slog
package logger

import (
	"log/slog"
	"os"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/golang-cz/devslog"
)

// New creates a new logger with configured formatting and logging level
func New(env config.Env) *slog.Logger {
	var log *slog.Logger

	if env == config.Prod {
		slogOpts := &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelInfo,
		}
		log = slog.New(slog.NewJSONHandler(os.Stdout, slogOpts))
	} else {
		slogOpts := &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}
		opts := &devslog.Options{
			HandlerOptions:    slogOpts,
			MaxSlicePrintSize: 10,
			SortKeys:          true,
			NewLineAfterLog:   true,
			StringerFormatter: true,
			TimeFormat:        "[15:04:05.000]",
		}

		log = slog.New(devslog.NewHandler(os.Stdout, opts))
	}

	// Set the logger as the default logger
	slog.SetDefault(log)

	return log
}
