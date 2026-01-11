// Package winagent implements a Windows agent for enforcing Metron screen-time sessions.
package winagent

import (
	"errors"
	"time"
)

var (
	ErrMissingDeviceID  = errors.New("device_id is required")
	ErrMissingToken     = errors.New("agent_token is required")
	ErrMissingURL       = errors.New("metron_base_url is required")
	ErrInvalidInterval  = errors.New("poll_interval must be positive")
	ErrInvalidGrace     = errors.New("grace_period must be positive")
)

// Config holds the Windows agent configuration
type Config struct {
	DeviceID      string        // Device ID registered in Metron
	AgentToken    string        // Bearer token for authentication
	MetronBaseURL string        // Metron API base URL (e.g., "https://metron.example.com")
	PollInterval  time.Duration // How often to poll the backend (default: 15s)
	GracePeriod   time.Duration // Grace period on network error before locking (default: 30s)
	LogPath       string        // Log file path (empty = stdout)
	LogLevel      string        // Log level: debug, info, warn, error
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		PollInterval: 15 * time.Second,
		GracePeriod:  30 * time.Second,
		LogLevel:     "info",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DeviceID == "" {
		return ErrMissingDeviceID
	}
	if c.AgentToken == "" {
		return ErrMissingToken
	}
	if c.MetronBaseURL == "" {
		return ErrMissingURL
	}
	if c.PollInterval <= 0 {
		return ErrInvalidInterval
	}
	if c.GracePeriod <= 0 {
		return ErrInvalidGrace
	}
	return nil
}
