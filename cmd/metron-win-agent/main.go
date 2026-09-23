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

	modeAgent   = "agent"
	modeUpdater = "updater"

	// agentTaskName is the user-session scheduled task created by the installer
	agentTaskName = "MetronAgent"
	// updaterTimeout bounds one updater run (manifest check, download, self-check)
	updaterTimeout = 10 * time.Minute
	// exeWatchInterval is how often the agent checks whether its binary was replaced
	exeWatchInterval = 30 * time.Second
)

// Set at build time via -ldflags "-X main.version=... -X main.updatePublicKey=..."
var (
	version = "dev"
	// updatePublicKey is the base64 ed25519 key that must sign published updates (empty = hash only)
	updatePublicKey = ""
)

func main() {
	// Parse command-line flags
	mode := flag.String("mode", modeAgent, "Run mode: agent (enforce sessions) or updater (self-update and watchdog, run as SYSTEM)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	deviceID := flag.String("device-id", "", "Device ID registered in Metron (required)")
	token := flag.String("token", "", "Agent authentication token (required)")
	metronURL := flag.String("url", "", "Metron API base URL (required)")
	pollInterval := flag.Int("poll-interval", defaultPollInterval, "Polling interval in seconds")
	gracePeriod := flag.Int("grace-period", defaultGracePeriod, "Grace period before locking on network error (seconds)")
	logPath := flag.String("log-path", "", "Log file path (stdout if empty)")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	logFormat := flag.String("log-format", "json", "Log format: json or text")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

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
	if *mode != modeAgent && *mode != modeUpdater {
		fmt.Fprintf(os.Stderr, "Error: unknown -mode %q\n", *mode)
		os.Exit(1)
	}

	logger, closeLog, err := setupLogger(*logPath, *logFormat, *logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer closeLog()
	slog.SetDefault(logger)

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

	mainLogger := logger.With("component", "main")
	if err := config.Validate(); err != nil {
		mainLogger.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		mainLogger.Error("Cannot resolve executable path", "error", err)
		os.Exit(1)
	}

	client := winagent.NewHTTPMetronClient(config.MetronBaseURL, config.AgentToken, logger)

	if *mode == modeUpdater {
		os.Exit(runUpdater(client, exePath, logger))
	}
	runAgent(client, config, exePath, logger)
}

// runAgent enforces sessions until a shutdown signal or until the binary is replaced by the updater
func runAgent(client *winagent.HTTPMetronClient, config *winagent.Config, exePath string, logger *slog.Logger) {
	mainLogger := logger.With("component", "main")
	mainLogger.Info("Metron Windows Agent starting",
		"version", version,
		"device_id", config.DeviceID,
		"metron_url", config.MetronBaseURL,
		"poll_interval", config.PollInterval,
		"grace_period", config.GracePeriod,
	)

	platform := winagent.NewPlatform(logger)
	enforcer := winagent.NewEnforcer(client, platform, winagent.RealClock{}, config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Restart into the new binary as soon as the updater swaps it
	restarted := make(chan struct{})
	go winagent.WatchExecutable(ctx, exePath, exeWatchInterval, logger, func() {
		if err := winagent.RestartSelf(exePath); err != nil {
			// Keep enforcing with the old image; the next logon starts the new binary
			mainLogger.Error("Failed to start updated agent", "error", err)
			return
		}
		close(restarted)
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go enforcer.Start(ctx)

	select {
	case sig := <-sigChan:
		mainLogger.Info("Shutdown signal received", "signal", sig.String())
	case <-restarted:
		mainLogger.Info("Handed over to updated agent binary")
	}

	cancel()
	// Give enforcer time to stop gracefully
	time.Sleep(1 * time.Second)
	mainLogger.Info("Metron Windows Agent stopped")
}

// runUpdater performs one update check and the agent watchdog. Returns the process exit code.
func runUpdater(client *winagent.HTTPMetronClient, exePath string, logger *slog.Logger) int {
	mainLogger := logger.With("component", "main")
	mainLogger.Debug("Metron updater run", "version", version, "exe", exePath)

	ctx, cancel := context.WithTimeout(context.Background(), updaterTimeout)
	defer cancel()

	updater := winagent.NewUpdater(client, winagent.VerifyBinaryVersion, winagent.UpdaterConfig{
		ExePath:        exePath,
		CurrentVersion: version,
		PublicKey:      updatePublicKey,
	}, logger)

	exitCode := 0
	if _, err := updater.RunOnce(ctx); err != nil {
		mainLogger.Error("Update failed", "error", err)
		exitCode = 1
	}

	if err := winagent.EnsureAgentRunning(exePath, agentTaskName, logger); err != nil {
		mainLogger.Warn("Watchdog could not start agent", "error", err)
	}
	return exitCode
}

// setupLogger writes to logPath when set, otherwise to stdout
func setupLogger(logPath, format, levelName string) (*slog.Logger, func(), error) {
	level := logging.ParseLevel(levelName)
	if logPath == "" {
		return logging.NewLogger(logging.LoggerConfig{Format: format, Level: level}), func() {}, nil
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, err
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(file, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(file, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler), func() { _ = file.Close() }, nil
}
