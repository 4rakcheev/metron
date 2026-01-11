package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metron/internal/logging"
	"metron/internal/winagent"
)

const (
	defaultPollInterval = 15
	defaultGracePeriod  = 30
)

func main() {
	// Parse command-line flags
	deviceID := flag.String("device-id", "", "Device ID registered in Metron (required)")
	token := flag.String("token", "", "Agent authentication token (required)")
	metronURL := flag.String("url", "", "Metron API base URL (required)")
	pollInterval := flag.Int("poll-interval", defaultPollInterval, "Polling interval in seconds")
	gracePeriod := flag.Int("grace-period", defaultGracePeriod, "Grace period before locking on network error (seconds)")
	logPath := flag.String("log-path", "", "Log file path (stdout if empty)")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	logFormat := flag.String("log-format", "json", "Log format: json or text")
	flag.Parse()

	// Validate required flags
	if *deviceID == "" {
		fmt.Fprintln(os.Stderr, "Error: -device-id is required")
		flag.Usage()
		os.Exit(1)
	}
	if *token == "" {
		fmt.Fprintln(os.Stderr, "Error: -token is required")
		flag.Usage()
		os.Exit(1)
	}
	if *metronURL == "" {
		fmt.Fprintln(os.Stderr, "Error: -url is required")
		flag.Usage()
		os.Exit(1)
	}

	// Setup logging
	level := logging.ParseLevel(*logLevel)
	logConfig := logging.LoggerConfig{
		Format: *logFormat,
		Level:  level,
	}

	// If log path is specified, set up file logging
	var logger *slog.Logger
	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		// Create logger writing to file
		var handler slog.Handler
		if logConfig.Format == "json" {
			handler = slog.NewJSONHandler(file, &slog.HandlerOptions{Level: level})
		} else {
			handler = slog.NewTextHandler(file, &slog.HandlerOptions{Level: level})
		}
		logger = slog.New(handler)
	} else {
		logger = logging.NewLogger(logConfig)
	}
	slog.SetDefault(logger)

	mainLogger := logger.With("component", "main")
	mainLogger.Info("Metron Windows Agent starting",
		"device_id", *deviceID,
		"metron_url", *metronURL,
		"poll_interval", *pollInterval,
		"grace_period", *gracePeriod,
	)

	// Create configuration
	config := &winagent.Config{
		DeviceID:      *deviceID,
		AgentToken:    *token,
		MetronBaseURL: *metronURL,
		PollInterval:  time.Duration(*pollInterval) * time.Second,
		GracePeriod:   time.Duration(*gracePeriod) * time.Second,
		LogPath:       *logPath,
		LogLevel:      *logLevel,
	}

	if err := config.Validate(); err != nil {
		mainLogger.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Create components
	client := winagent.NewHTTPMetronClient(config.MetronBaseURL, config.AgentToken, logger)
	platform := winagent.NewPlatform(logger)
	clock := winagent.RealClock{}

	// Create enforcer
	enforcer := winagent.NewEnforcer(client, platform, clock, config, logger)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start enforcer in background
	go func() {
		enforcer.Start(ctx)
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	mainLogger.Info("Shutdown signal received", "signal", sig.String())

	// Cancel context to stop enforcer
	cancel()

	// Give enforcer time to stop gracefully
	time.Sleep(1 * time.Second)

	mainLogger.Info("Metron Windows Agent stopped")
}
