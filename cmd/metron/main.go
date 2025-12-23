package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"metron/config"
	"metron/internal/api"
	"metron/internal/core"
	"metron/internal/devices"
	"metron/internal/drivers"
	"metron/internal/drivers/aqara"
	"metron/internal/drivers/kidslox"
	"metron/internal/scheduler"
	"metron/internal/storage/sqlite"
)

const (
	shutdownTimeout = 10 * time.Second
	defaultConfigPath = "config.json"
)

// Adapter types to bridge interface differences between packages

type coreDeviceRegistry struct {
	registry *devices.Registry
}

func (r *coreDeviceRegistry) Get(id string) (core.Device, error) {
	device, err := r.registry.Get(id)
	if err != nil {
		return nil, err
	}
	return device, nil
}

type coreDriverRegistry struct {
	registry *drivers.Registry
}

func (r *coreDriverRegistry) Get(name string) (core.DeviceDriver, error) {
	driver, err := r.registry.Get(name)
	if err != nil {
		return nil, err
	}
	return &coreDriverAdapter{driver}, nil
}

type coreDriverAdapter struct {
	devices.DeviceDriver
}

type schedulerDeviceRegistry struct {
	registry *devices.Registry
}

func (r *schedulerDeviceRegistry) Get(id string) (scheduler.Device, error) {
	return r.registry.Get(id)
}

type schedulerDriverRegistry struct {
	registry *drivers.Registry
}

func (r *schedulerDriverRegistry) Get(name string) (scheduler.DeviceDriver, error) {
	driver, err := r.registry.Get(name)
	if err != nil {
		return nil, err
	}
	return &schedulerDriverAdapter{driver}, nil
}

type schedulerDriverAdapter struct {
	devices.DeviceDriver
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	useEnv := flag.Bool("env", false, "Load configuration from environment variables")
	logFormat := flag.String("log-format", "json", "Log format: json or text")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Configure structured logger
	var handler slog.Handler
	var level slog.Level

	// Parse log level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Configure handler based on format
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename timestamp key for better readability
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	}

	if *logFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Create main component logger
	mainLogger := slog.Default().With("component", "main")

	if err := run(*configPath, *useEnv, mainLogger); err != nil {
		mainLogger.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, useEnv bool, logger *slog.Logger) error {
	// Load configuration
	logger.Info("Loading configuration", "use_env", useEnv, "config_path", configPath)
	var cfg *config.Config
	var err error

	if useEnv {
		cfg, err = config.LoadFromEnv()
	} else {
		cfg, err = config.Load(configPath)
	}

	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load timezone location
	timezone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone '%s': %w", cfg.Timezone, err)
	}
	logger.Info("Application timezone configured", "timezone", cfg.Timezone)

	// Initialize database
	logger.Info("Initializing database", "path", cfg.Database.Path)
	db, err := sqlite.New(cfg.Database.Path, timezone)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Initialize device registry first (needed by drivers)
	logger.Info("Initializing device registry")
	deviceRegistry := devices.NewRegistry()

	// Initialize driver registry
	logger.Info("Initializing device driver registry")
	driverRegistry := drivers.NewRegistry()

	// Register Aqara driver
	logger.Info("Registering Aqara Cloud driver",
		"base_url", cfg.Aqara.BaseURL,
		"pin_scene", cfg.Aqara.Scenes.TVPINEntry,
		"warn_scene", cfg.Aqara.Scenes.TVWarning,
		"off_scene", cfg.Aqara.Scenes.TVPowerOff)

	aqaraConfig := aqara.Config{
		AppID:       cfg.Aqara.AppID,
		AppKey:      cfg.Aqara.AppKey,
		KeyID:       cfg.Aqara.KeyID,
		BaseURL:     cfg.Aqara.BaseURL,
		PINSceneID:  cfg.Aqara.Scenes.TVPINEntry,
		WarnSceneID: cfg.Aqara.Scenes.TVWarning,
		OffSceneID:  cfg.Aqara.Scenes.TVPowerOff,
	}
	aqaraLogger := slog.Default().With("component", "driver.aqara")
	aqaraDriver := aqara.NewDriver(aqaraConfig, db, aqaraLogger)
	driverRegistry.Register(aqaraDriver)

	// Register Kidslox driver if configured
	if cfg.Kidslox != nil {
		logger.Info("Registering Kidslox driver")
		kidsloxConfig := kidslox.Config{
			BaseURL:   cfg.Kidslox.BaseURL,
			APIKey:    cfg.Kidslox.APIKey,
			AccountID: cfg.Kidslox.AccountID,
			DeviceID:  cfg.Kidslox.DeviceID,
			ProfileID: cfg.Kidslox.ProfileID,
		}
		kidsloxLogger := slog.Default().With("component", "driver.kidslox")
		kidsloxDriver := kidslox.NewDriver(kidsloxConfig, deviceRegistry, kidsloxLogger)
		driverRegistry.Register(kidsloxDriver)
	}

	// Register devices from configuration
	logger.Info("Registering devices", "count", len(cfg.Devices))
	for _, deviceCfg := range cfg.Devices {
		device := &devices.Device{
			ID:         deviceCfg.ID,
			Name:       deviceCfg.Name,
			Type:       deviceCfg.Type,
			Driver:     deviceCfg.Driver,
			Parameters: deviceCfg.Parameters,
		}
		if err := deviceRegistry.Register(device); err != nil {
			logger.Error("Failed to register device",
				"device_id", deviceCfg.ID,
				"device_name", deviceCfg.Name,
				"error", err)
			return fmt.Errorf("failed to register device %s: %w", deviceCfg.ID, err)
		}
		logger.Info("Device registered",
			"id", device.ID,
			"name", device.Name,
			"type", device.Type,
			"driver", device.Driver)
	}

	// Create component-specific loggers
	managerLogger := slog.Default().With("component", "manager")
	schedulerLogger := slog.Default().With("component", "scheduler")
	apiLogger := slog.Default().With("component", "api")

	// Initialize time calculation service
	logger.Info("Initializing time calculation service")
	calculator := core.NewTimeCalculationService(db, timezone)

	// Initialize session manager
	logger.Info("Initializing session manager")
	sessionManager := core.NewSessionManager(db, &coreDeviceRegistry{deviceRegistry}, &coreDriverRegistry{driverRegistry}, calculator, timezone, managerLogger)

	// Start scheduler
	logger.Info("Starting session scheduler", "interval", "1m")
	sched := scheduler.NewScheduler(db, &schedulerDeviceRegistry{deviceRegistry}, &schedulerDriverRegistry{driverRegistry}, 1*time.Minute, timezone, schedulerLogger)
	go sched.Start()

	// Initialize REST API with Gin
	logger.Info("Initializing REST API server")
	router := api.NewRouter(api.RouterConfig{
		Storage:           db,
		Manager:           sessionManager,
		DriverRegistry:    driverRegistry,
		DeviceRegistry:    deviceRegistry,
		APIKey:            cfg.Security.APIKey,
		Logger:            apiLogger,
		AqaraTokenStorage: db, // SQLite storage also implements aqara.AqaraTokenStorage
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port,
			"endpoint", fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port))
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		logger.Info("Shutdown signal received", "signal", sig.String())

		// Stop scheduler
		logger.Info("Stopping scheduler")
		sched.Stop()

		// Shutdown HTTP server
		logger.Info("Shutting down HTTP server", "timeout", shutdownTimeout)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		logger.Info("Graceful shutdown complete")
	}

	return nil
}
