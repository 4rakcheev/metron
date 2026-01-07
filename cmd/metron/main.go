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
	"metron/internal/logging"
	"metron/internal/scheduler"
	"metron/internal/storage/sqlite"
)

const (
	shutdownTimeout   = 10 * time.Second
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

// parseTimeOfDay parses a time string in HH:MM format and returns hour and minute
func parseTimeOfDay(timeStr string) (hour, minute int, err error) {
	n, err := fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM: %w", err)
	}
	if n != 2 {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM")
	}
	if hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("hour must be between 0 and 23, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("minute must be between 0 and 59, got %d", minute)
	}
	return hour, minute, nil
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	useEnv := flag.Bool("env", false, "Load configuration from environment variables")
	logFormat := flag.String("log-format", "json", "Log format: json or text")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	// Parse log level and create logger (writes to stdout)
	level := logging.ParseLevel(*logLevel)
	logger := logging.NewLogger(logging.LoggerConfig{
		Format: *logFormat,
		Level:  level,
	})
	slog.SetDefault(logger)

	// Create main component logger
	mainLogger := logger.With("component", "main")

	if err := run(*configPath, *useEnv, logger); err != nil {
		mainLogger.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, useEnv bool, logger *slog.Logger) error {
	mainLogger := logger.With("component", "main")

	// Load configuration
	mainLogger.Info("Loading configuration", "use_env", useEnv, "config_path", configPath)
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
	mainLogger.Info("Application timezone configured", "timezone", cfg.Timezone)

	// Initialize database
	mainLogger.Info("Initializing database", "path", cfg.Database.Path)
	db, err := sqlite.New(cfg.Database.Path, timezone)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Initialize device registry first (needed by drivers)
	mainLogger.Info("Initializing device registry")
	deviceRegistry := devices.NewRegistry()

	// Initialize driver registry
	mainLogger.Info("Initializing device driver registry")
	driverRegistry := drivers.NewRegistry()

	// Register Aqara driver
	mainLogger.Info("Registering Aqara Cloud driver",
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
	aqaraLogger := logger.With("component", "driver.aqara")
	aqaraDriver := aqara.NewDriver(aqaraConfig, db, aqaraLogger)
	driverRegistry.Register(aqaraDriver)

	// Register Kidslox driver if configured
	if cfg.Kidslox != nil {
		mainLogger.Info("Registering Kidslox driver")
		kidsloxConfig := kidslox.Config{
			BaseURL:   cfg.Kidslox.BaseURL,
			APIKey:    cfg.Kidslox.APIKey,
			AccountID: cfg.Kidslox.AccountID,
			DeviceID:  cfg.Kidslox.DeviceID,
			ProfileID: cfg.Kidslox.ProfileID,
		}
		kidsloxLogger := logger.With("component", "driver.kidslox")
		kidsloxDriver := kidslox.NewDriver(kidsloxConfig, deviceRegistry, kidsloxLogger)
		driverRegistry.Register(kidsloxDriver)
	}

	// Register devices from configuration
	mainLogger.Info("Registering devices", "count", len(cfg.Devices))
	for _, deviceCfg := range cfg.Devices {
		device := &devices.Device{
			ID:         deviceCfg.ID,
			Name:       deviceCfg.Name,
			Type:       deviceCfg.Type,
			Driver:     deviceCfg.Driver,
			Parameters: deviceCfg.Parameters,
		}
		if err := deviceRegistry.Register(device); err != nil {
			mainLogger.Error("Failed to register device",
				"device_id", deviceCfg.ID,
				"device_name", deviceCfg.Name,
				"error", err)
			return fmt.Errorf("failed to register device %s: %w", deviceCfg.ID, err)
		}
		mainLogger.Info("Device registered",
			"id", device.ID,
			"name", device.Name,
			"type", device.Type,
			"driver", device.Driver)
	}

	// Create component-specific loggers
	managerLogger := logger.With("component", "manager")
	schedulerLogger := logger.With("component", "scheduler")
	apiLogger := logger.With("component", "api")

	// Initialize time calculation service
	mainLogger.Info("Initializing time calculation service")
	calculator := core.NewTimeCalculationService(db, timezone)

	// Initialize downtime service
	var downtimeService *core.DowntimeService
	if cfg.Downtime != nil {
		schedule := &core.DowntimeSchedule{}

		// Parse weekday schedule
		weekdayCfg := cfg.Downtime.GetWeekdaySchedule()
		if weekdayCfg != nil {
			startHour, startMinute, err := parseTimeOfDay(weekdayCfg.StartTime)
			if err != nil {
				mainLogger.Error("Invalid weekday downtime start_time", "error", err)
				os.Exit(1)
			}
			endHour, endMinute, err := parseTimeOfDay(weekdayCfg.EndTime)
			if err != nil {
				mainLogger.Error("Invalid weekday downtime end_time", "error", err)
				os.Exit(1)
			}
			schedule.Weekday = &core.DaySchedule{
				StartHour:   startHour,
				StartMinute: startMinute,
				EndHour:     endHour,
				EndMinute:   endMinute,
			}
			mainLogger.Info("Weekday downtime configured",
				"start", weekdayCfg.StartTime,
				"end", weekdayCfg.EndTime)
		}

		// Parse weekend schedule
		weekendCfg := cfg.Downtime.GetWeekendSchedule()
		if weekendCfg != nil {
			startHour, startMinute, err := parseTimeOfDay(weekendCfg.StartTime)
			if err != nil {
				mainLogger.Error("Invalid weekend downtime start_time", "error", err)
				os.Exit(1)
			}
			endHour, endMinute, err := parseTimeOfDay(weekendCfg.EndTime)
			if err != nil {
				mainLogger.Error("Invalid weekend downtime end_time", "error", err)
				os.Exit(1)
			}
			schedule.Weekend = &core.DaySchedule{
				StartHour:   startHour,
				StartMinute: startMinute,
				EndHour:     endHour,
				EndMinute:   endMinute,
			}
			mainLogger.Info("Weekend downtime configured",
				"start", weekendCfg.StartTime,
				"end", weekendCfg.EndTime)
		}

		downtimeService = core.NewDowntimeService(schedule, timezone)
		// Wire up skip storage
		downtimeService.SetSkipStorage(db)
	} else {
		mainLogger.Info("Downtime service disabled (no configuration)")
		downtimeService = core.NewDowntimeService(nil, timezone)
	}

	// Initialize session manager
	mainLogger.Info("Initializing session manager")
	baseManager := core.NewSessionManager(db, &coreDeviceRegistry{deviceRegistry}, &coreDriverRegistry{driverRegistry}, calculator, downtimeService, timezone, managerLogger)

	// Wrap session manager with logging decorator
	sessionManager := logging.NewSessionManagerLogger(baseManager, logger)

	// Start scheduler
	mainLogger.Info("Starting session scheduler", "interval", "1m")
	sched := scheduler.NewScheduler(db, &schedulerDeviceRegistry{deviceRegistry}, &schedulerDriverRegistry{driverRegistry}, downtimeService, 1*time.Minute, timezone, schedulerLogger)
	go sched.Start()

	// Initialize REST API with Gin
	mainLogger.Info("Initializing REST API server")
	router := api.NewRouter(api.RouterConfig{
		Storage:             db,
		Manager:             sessionManager,
		DriverRegistry:      driverRegistry,
		DeviceRegistry:      deviceRegistry,
		Downtime:            downtimeService,
		DowntimeSkipStorage: db, // SQLite storage also implements core.DowntimeSkipStorage
		APIKey:              cfg.Security.APIKey,
		Logger:              apiLogger,
		AqaraTokenStorage:   db, // SQLite storage also implements aqara.AqaraTokenStorage
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
		mainLogger.Info("HTTP server starting",
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
		mainLogger.Info("Shutdown signal received", "signal", sig.String())

		// Stop scheduler
		mainLogger.Info("Stopping scheduler")
		sched.Stop()

		// Shutdown HTTP server
		mainLogger.Info("Shutting down HTTP server", "timeout", shutdownTimeout)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		mainLogger.Info("Graceful shutdown complete")
	}

	return nil
}
