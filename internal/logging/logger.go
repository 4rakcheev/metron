package logging

import (
	"io"
	"log/slog"
	"os"
)

// LoggerConfig holds configuration for creating loggers
type LoggerConfig struct {
	Format string      // "json" or "text"
	Level  slog.Level  // Log level
	Output io.Writer   // Where to write logs (file or stdout)
}

// NewLogger creates a new slog.Logger with the given configuration
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
		handler = slog.NewTextHandler(config.Output, opts)
	} else {
		handler = slog.NewJSONHandler(config.Output, opts)
	}

	return slog.New(handler)
}

// MultiLogger holds loggers for different components
type MultiLogger struct {
	Core  *slog.Logger // Core/internal components (metron.log)
	Child *slog.Logger // Child API (metron-child.log)
	Bot   *slog.Logger // Bot API (metron-bot.log)
	files []*os.File   // Keep track of files to close
}

// Close closes all log files
func (m *MultiLogger) Close() error {
	for _, f := range m.files {
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// MultiLoggerConfig holds configuration for creating a MultiLogger
type MultiLoggerConfig struct {
	Format       string     // "json" or "text"
	Level        slog.Level // Log level
	CoreLogPath  string     // Path to metron.log
	ChildLogPath string     // Path to metron-child.log
	BotLogPath   string     // Path to metron-bot.log
}

// NewMultiLogger creates a MultiLogger with separate outputs for each component
func NewMultiLogger(config MultiLoggerConfig) (*MultiLogger, error) {
	ml := &MultiLogger{
		files: make([]*os.File, 0, 3),
	}

	// Open core log file
	coreFile, err := os.OpenFile(config.CoreLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	ml.files = append(ml.files, coreFile)
	ml.Core = NewLogger(LoggerConfig{
		Format: config.Format,
		Level:  config.Level,
		Output: coreFile,
	})

	// Open child API log file
	childFile, err := os.OpenFile(config.ChildLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		ml.Close()
		return nil, err
	}
	ml.files = append(ml.files, childFile)
	ml.Child = NewLogger(LoggerConfig{
		Format: config.Format,
		Level:  config.Level,
		Output: childFile,
	})

	// Open bot API log file
	botFile, err := os.OpenFile(config.BotLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		ml.Close()
		return nil, err
	}
	ml.files = append(ml.files, botFile)
	ml.Bot = NewLogger(LoggerConfig{
		Format: config.Format,
		Level:  config.Level,
		Output: botFile,
	})

	return ml, nil
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
