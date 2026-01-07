package logging

import (
	"log/slog"
	"os"
)

// LoggerConfig holds configuration for creating loggers
type LoggerConfig struct {
	Format string     // "json" or "text"
	Level  slog.Level // Log level
}

// NewLogger creates a new slog.Logger that writes to stdout
func NewLogger(config LoggerConfig) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: config.Level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename timestamp key for better readability
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	}

	var handler slog.Handler
	if config.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// ParseLevel converts a string log level to slog.Level
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
