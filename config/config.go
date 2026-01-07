package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig    `json:"server"`
	Database DatabaseConfig  `json:"database"`
	Security SecurityConfig  `json:"security"`
	Timezone string          `json:"timezone"` // IANA timezone string (e.g., "Europe/Riga")
	Devices  []DeviceConfig  `json:"devices"`  // Global device registry
	Aqara    AqaraConfig     `json:"aqara"`
	Kidslox  *KidsloxConfig  `json:"kidslox,omitempty"`
	Downtime *DowntimeConfig `json:"downtime,omitempty"`
}

// DeviceConfig represents a device configuration
type DeviceConfig struct {
	ID         string                 `json:"id"`                    // Unique device ID (e.g., "tv1", "ps5")
	Name       string                 `json:"name"`                  // Display name (e.g., "Living Room TV")
	Type       string                 `json:"type"`                  // Device type (e.g., "tv", "ps5") - for display/stats
	Driver     string                 `json:"driver"`                // Driver name (e.g., "aqara") - for control
	Parameters map[string]interface{} `json:"parameters,omitempty"`  // Driver-specific parameters (overrides defaults)
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path string `json:"path"`
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	APIKey        string   `json:"api_key"`
	AllowedIPs    []string `json:"allowed_ips"`
	EnableIPCheck bool     `json:"enable_ip_check"`
}

// AqaraConfig contains Aqara Cloud API settings
type AqaraConfig struct {
	AppID   string      `json:"app_id"`
	AppKey  string      `json:"app_key"`
	KeyID   string      `json:"key_id"`
	BaseURL string      `json:"base_url"`
	Scenes  AqaraScenes `json:"scenes"` // Default scenes (can be overridden per device)
}

// AqaraScenes contains scene IDs for different actions
type AqaraScenes struct {
	TVPINEntry string `json:"tv_pin_entry"`
	TVWarning  string `json:"tv_warning"`
	TVPowerOff string `json:"tv_power_off"`
}

// KidsloxConfig contains Kidslox API settings
type KidsloxConfig struct {
	BaseURL   string `json:"base_url"`   // API base URL
	APIKey    string `json:"api_key"`    // Static API key for authentication
	AccountID string `json:"account_id"` // Account ID for actions
	// Default device parameters (can be overridden by device-specific parameters)
	DeviceID  string `json:"device_id,omitempty"`  // Default Kidslox device ID
	ProfileID string `json:"profile_id,omitempty"` // Default Kidslox profile ID
}

// DayScheduleConfig defines start/end times for a day type (weekday or weekend)
type DayScheduleConfig struct {
	StartTime string `json:"start_time"` // HH:MM format (e.g., "22:00")
	EndTime   string `json:"end_time"`   // HH:MM format (e.g., "10:00")
}

// DowntimeConfig defines the global downtime schedule
// Supports two formats:
// 1. Legacy flat format: {"start_time": "22:00", "end_time": "10:00"} - applies to all days
// 2. New nested format: {"weekday": {...}, "weekend": {...}} - separate schedules
type DowntimeConfig struct {
	// Legacy flat fields (for backward compatibility)
	StartTime string `json:"start_time,omitempty"` // HH:MM format (e.g., "22:00")
	EndTime   string `json:"end_time,omitempty"`   // HH:MM format (e.g., "10:00")

	// New day-specific schedules
	Weekday *DayScheduleConfig `json:"weekday,omitempty"` // Mon-Fri schedule
	Weekend *DayScheduleConfig `json:"weekend,omitempty"` // Sat-Sun schedule
}

// IsLegacyFormat returns true if using old flat start_time/end_time format
func (d *DowntimeConfig) IsLegacyFormat() bool {
	return d.StartTime != "" && d.EndTime != "" && d.Weekday == nil && d.Weekend == nil
}

// GetWeekdaySchedule returns the weekday schedule, falling back to legacy format
func (d *DowntimeConfig) GetWeekdaySchedule() *DayScheduleConfig {
	if d.Weekday != nil {
		return d.Weekday
	}
	if d.IsLegacyFormat() {
		return &DayScheduleConfig{StartTime: d.StartTime, EndTime: d.EndTime}
	}
	return nil
}

// GetWeekendSchedule returns the weekend schedule, falling back to legacy format
func (d *DowntimeConfig) GetWeekendSchedule() *DayScheduleConfig {
	if d.Weekend != nil {
		return d.Weekend
	}
	if d.IsLegacyFormat() {
		return &DayScheduleConfig{StartTime: d.StartTime, EndTime: d.EndTime}
	}
	return nil
}

// Validate validates the downtime configuration
func (d *DowntimeConfig) Validate() error {
	// Check if using legacy format
	if d.IsLegacyFormat() {
		if _, _, err := parseTimeOfDay(d.StartTime); err != nil {
			return fmt.Errorf("invalid downtime start_time '%s': %v", d.StartTime, err)
		}
		if _, _, err := parseTimeOfDay(d.EndTime); err != nil {
			return fmt.Errorf("invalid downtime end_time '%s': %v", d.EndTime, err)
		}
		return nil
	}

	// Check if using new nested format
	if d.Weekday != nil || d.Weekend != nil {
		// At least one of weekday/weekend must be set
		if d.Weekday == nil && d.Weekend == nil {
			return fmt.Errorf("downtime config must have at least weekday or weekend schedule")
		}

		// Validate weekday schedule if present
		if d.Weekday != nil {
			if _, _, err := parseTimeOfDay(d.Weekday.StartTime); err != nil {
				return fmt.Errorf("invalid weekday downtime start_time '%s': %v", d.Weekday.StartTime, err)
			}
			if _, _, err := parseTimeOfDay(d.Weekday.EndTime); err != nil {
				return fmt.Errorf("invalid weekday downtime end_time '%s': %v", d.Weekday.EndTime, err)
			}
		}

		// Validate weekend schedule if present
		if d.Weekend != nil {
			if _, _, err := parseTimeOfDay(d.Weekend.StartTime); err != nil {
				return fmt.Errorf("invalid weekend downtime start_time '%s': %v", d.Weekend.StartTime, err)
			}
			if _, _, err := parseTimeOfDay(d.Weekend.EndTime); err != nil {
				return fmt.Errorf("invalid weekend downtime end_time '%s': %v", d.Weekend.EndTime, err)
			}
		}

		return nil
	}

	// Empty config is valid (downtime disabled)
	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("%w: invalid server port", ErrInvalidConfig)
	}

	if c.Database.Path == "" {
		return fmt.Errorf("%w: database path is required", ErrInvalidConfig)
	}

	if c.Security.APIKey == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidConfig)
	}

	// Validate timezone
	if c.Timezone == "" {
		c.Timezone = "UTC" // Default to UTC if not specified
	}

	// Validate timezone string can be loaded
	_, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return fmt.Errorf("%w: invalid timezone '%s': %v", ErrInvalidConfig, c.Timezone, err)
	}

	// Validate Aqara config (required for now for backward compatibility)
	if c.Aqara.AppID == "" || c.Aqara.AppKey == "" || c.Aqara.KeyID == "" {
		return fmt.Errorf("%w: Aqara credentials are required", ErrInvalidConfig)
	}

	if c.Aqara.BaseURL == "" {
		c.Aqara.BaseURL = "https://open-cn.aqara.com" // default
	}

	// Validate Kidslox config if present
	if c.Kidslox != nil {
		if c.Kidslox.APIKey == "" || c.Kidslox.AccountID == "" {
			return fmt.Errorf("%w: Kidslox API key and account ID are required when Kidslox is configured", ErrInvalidConfig)
		}

		if c.Kidslox.BaseURL == "" {
			c.Kidslox.BaseURL = "https://admin.kdlparentalcontrol.com" // default
		}
	}

	// Validate downtime config if present
	if c.Downtime != nil {
		if err := c.Downtime.Validate(); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
	}

	return nil
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

// Load loads configuration from a JSON file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigFileNotFound
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadFromEnv loads configuration from environment variables
// This is useful for containerized deployments
func LoadFromEnv() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host: getEnv("METRON_HOST", "0.0.0.0"),
			Port: getEnvInt("METRON_PORT", 8080),
		},
		Database: DatabaseConfig{
			Path: getEnv("METRON_DB_PATH", "./metron.db"),
		},
		Security: SecurityConfig{
			APIKey:        getEnv("METRON_API_KEY", ""),
			EnableIPCheck: getEnvBool("METRON_ENABLE_IP_CHECK", false),
		},
		Timezone: getEnv("METRON_TIMEZONE", "UTC"),
		Aqara: AqaraConfig{
			AppID:   getEnv("METRON_AQARA_APP_ID", ""),
			AppKey:  getEnv("METRON_AQARA_APP_KEY", ""),
			KeyID:   getEnv("METRON_AQARA_KEY_ID", ""),
			BaseURL: getEnv("METRON_AQARA_BASE_URL", "https://open-cn.aqara.com"),
			Scenes: AqaraScenes{
				TVPINEntry: getEnv("METRON_AQARA_TV_PIN_SCENE", ""),
				TVWarning:  getEnv("METRON_AQARA_TV_WARNING_SCENE", ""),
				TVPowerOff: getEnv("METRON_AQARA_TV_POWEROFF_SCENE", ""),
			},
		},
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		fmt.Sscanf(value, "%d", &intVal)
		return intVal
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
