package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"metron/config"
	"metron/internal/core"
	"metron/internal/drivers/aqara"
	"metron/internal/storage/sqlite"
	"os"
	"time"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.json", "Path to configuration file")
	action := flag.String("action", "pin", "Action to perform: pin, warn, off")
	refreshToken := flag.String("refresh-token", "", "Aqara refresh token (required)")
	flag.Parse()

	// Validate refresh token
	if *refreshToken == "" {
		log.Fatal("❌ Error: -refresh-token is required\n\n" +
			"To get a refresh token:\n" +
			"1. Go to https://developer.aqara.com/console/app-management/\n" +
			"2. Application Details → Authorization Management → Aqara account authorization\n" +
			"3. Enter credentials and obtain access token\n" +
			"4. Go to Authorization Details and copy the Refresh Token\n" +
			"5. Run this test with: -refresh-token <token>\n")
	}

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create temporary in-memory database for token storage
	db, err := sqlite.New(":memory:")
	if err != nil {
		log.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Store the refresh token in the database
	tokens := &aqara.AqaraTokens{
		RefreshToken: *refreshToken,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.SaveAqaraTokens(context.Background(), tokens); err != nil {
		log.Fatalf("Failed to save refresh token: %v", err)
	}

	// Create Aqara driver with storage
	driver := aqara.NewDriver(aqara.Config{
		AppID:       cfg.Aqara.AppID,
		AppKey:      cfg.Aqara.AppKey,
		KeyID:       cfg.Aqara.KeyID,
		BaseURL:     cfg.Aqara.BaseURL,
		PINSceneID:  cfg.Aqara.Scenes.TVPINEntry,
		WarnSceneID: cfg.Aqara.Scenes.TVWarning,
		OffSceneID:  cfg.Aqara.Scenes.TVPowerOff,
	}, db)

	// Create dummy session for testing
	session := &core.Session{
		ID:         "test-session",
		DeviceType: "tv",
		DeviceID:   "tv1",
		ChildIDs:   []string{"test-child"},
		StartTime:  time.Now(),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform the requested action
	fmt.Printf("Testing Aqara Cloud API...\n")
	fmt.Printf("Action: %s\n", *action)
	fmt.Printf("Base URL: %s\n", cfg.Aqara.BaseURL)
	fmt.Printf("\n")

	var sceneID string
	var actionErr error

	switch *action {
	case "pin":
		fmt.Printf("Triggering PIN entry scene: %s\n", cfg.Aqara.Scenes.TVPINEntry)
		sceneID = cfg.Aqara.Scenes.TVPINEntry
		actionErr = driver.StartSession(ctx, session)

	case "warn":
		fmt.Printf("Triggering warning scene: %s\n", cfg.Aqara.Scenes.TVWarning)
		sceneID = cfg.Aqara.Scenes.TVWarning
		actionErr = driver.ApplyWarning(ctx, session, 5)

	case "off":
		fmt.Printf("Triggering power-off scene: %s\n", cfg.Aqara.Scenes.TVPowerOff)
		sceneID = cfg.Aqara.Scenes.TVPowerOff
		actionErr = driver.StopSession(ctx, session)

	default:
		log.Fatalf("Unknown action: %s. Use: pin, warn, or off", *action)
	}

	if actionErr != nil {
		log.Fatalf("❌ Error: %v", actionErr)
	}

	fmt.Printf("\n✅ Success! Scene %s triggered successfully.\n", sceneID)
}

func loadConfig(path string) (*config.Config, error) {
	// Try to load from file first
	cfg, err := config.Load(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// If file doesn't exist or failed, try environment variables
	if err != nil {
		fmt.Printf("Config file not found at %s, trying environment variables...\n", path)
		cfg, err = config.LoadFromEnv()
		if err != nil {
			return nil, fmt.Errorf("failed to load config from environment: %w", err)
		}
	}

	return cfg, nil
}
