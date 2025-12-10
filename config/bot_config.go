package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// BotConfig represents the Telegram bot configuration
type BotConfig struct {
	Telegram TelegramBotConfig `json:"telegram"`
	Metron   MetronAPIConfig   `json:"metron"`
}

// TelegramBotConfig contains Telegram bot settings
type TelegramBotConfig struct {
	Token         string  `json:"token"`
	AllowedUsers  []int64 `json:"allowed_users"`
	WebhookURL    string  `json:"webhook_url"`
	WebhookSecret string  `json:"webhook_secret"`
}

// MetronAPIConfig contains Metron API connection settings
type MetronAPIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// LoadBotConfig loads bot configuration from a file
func LoadBotConfig(path string) (*BotConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigFileNotFound, path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg BotConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *BotConfig) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("%w: telegram.token is required", ErrInvalidConfig)
	}

	if len(c.Telegram.AllowedUsers) == 0 {
		return fmt.Errorf("%w: telegram.allowed_users cannot be empty", ErrInvalidConfig)
	}

	if c.Telegram.WebhookURL == "" {
		return fmt.Errorf("%w: telegram.webhook_url is required", ErrInvalidConfig)
	}

	if c.Metron.BaseURL == "" {
		return fmt.Errorf("%w: metron.base_url is required", ErrInvalidConfig)
	}

	if c.Metron.APIKey == "" {
		return fmt.Errorf("%w: metron.api_key is required", ErrInvalidConfig)
	}

	return nil
}

// IsUserAllowed checks if a user ID is in the whitelist
func (c *BotConfig) IsUserAllowed(userID int64) bool {
	for _, allowedID := range c.Telegram.AllowedUsers {
		if allowedID == userID {
			return true
		}
	}
	return false
}
