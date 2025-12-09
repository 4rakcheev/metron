package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	"metron/internal/scheduler"
	"metron/internal/storage/sqlite"
)

const (
	shutdownTimeout = 10 * time.Second
	defaultConfigPath = "config.json"
)

// Adapter types to bridge interface differences between packages

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
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	useEnv := flag.Bool("env", false, "Load configuration from environment variables")
	flag.Parse()

	// Load configuration
	log.Println("Loading configuration...")
	var cfg *config.Config
	var err error

	if *useEnv {
		cfg, err = config.LoadFromEnv()
	} else {
		cfg, err = config.Load(*configPath)
	}

	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	log.Printf("Initializing SQLite database at %s...", cfg.Database.Path)
	db, err := sqlite.New(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Initialize driver registry
	log.Println("Initializing device driver registry...")
	driverRegistry := drivers.NewRegistry()

	// Register Aqara driver
	log.Println("Registering Aqara Cloud driver...")
	aqaraConfig := aqara.Config{
		AppID:       cfg.Aqara.AppID,
		AppKey:      cfg.Aqara.AppKey,
		KeyID:       cfg.Aqara.KeyID,
		AccessToken: cfg.Aqara.AccessToken,
		BaseURL:     cfg.Aqara.BaseURL,
		PINSceneID:  cfg.Aqara.Scenes.TVPINEntry,
		WarnSceneID: cfg.Aqara.Scenes.TVWarning,
		OffSceneID:  cfg.Aqara.Scenes.TVPowerOff,
	}
	aqaraDriver := aqara.NewDriver(aqaraConfig)
	driverRegistry.Register(aqaraDriver)

	// Initialize session manager
	log.Println("Initializing session manager...")
	sessionManager := core.NewSessionManager(db, &coreDriverRegistry{driverRegistry})

	// Start scheduler
	log.Println("Starting session scheduler...")
	sched := scheduler.NewScheduler(db, &schedulerDriverRegistry{driverRegistry}, 1*time.Minute, nil)
	go sched.Start()

	// Initialize REST API
	log.Println("Initializing REST API server...")
	apiInstance := api.NewAPI(sessionManager, db, cfg.Security.APIKey, nil)
	mux := http.NewServeMux()
	apiInstance.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting HTTP server on %s:%d...", cfg.Server.Host, cfg.Server.Port)
		log.Printf("API endpoints available at http://%s:%d", cfg.Server.Host, cfg.Server.Port)
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Printf("Received signal: %v. Starting graceful shutdown...", sig)

		// Stop scheduler
		log.Println("Stopping scheduler...")
		sched.Stop()

		// Shutdown HTTP server
		log.Println("Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		log.Println("Graceful shutdown complete")
	}

	return nil
}
