package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig     `json:"server"`
	Database  DatabaseConfig   `json:"database"`
	Security  SecurityConfig   `json:"security"`
	Timezone  string           `json:"timezone"` // IANA timezone string (e.g., "Europe/Riga")
	Devices   []DeviceConfig   `json:"devices"`  // Global device registry
	Aqara     AqaraConfig      `json:"aqara"`
	Kidslox   *KidsloxConfig   `json:"kidslox,omitempty"`
	Downtime  *DowntimeConfig  `json:"downtime,omitempty"`
	MovieTime *MovieTimeConfig `json:"movie_time,omitempty"`
}

// MovieTimeConfig contains settings for weekend shared movie time feature
type MovieTimeConfig struct {
	Enabled          bool     `json:"enabled"`            // Whether movie time feature is enabled
	DurationMinutes  int      `json:"duration_minutes"`   // Movie session duration (default: 120)
	BreakMinutes     int      `json:"break_minutes"`      // Required break after last personal session (default: 60)
	AllowedDeviceIDs []string `json:"allowed_device_ids"` // Devices where movie time can be used (e.g., ["tv1"])
}

// DeviceConfig represents a device configuration
type DeviceConfig struct {
	ID         string                 `json:"id"`                   // Unique device ID (e.g., "tv1", "ps5")
	Name       string                 `json:"name"`                 // Display name (e.g., "Living Room TV")
	Type       string                 `json:"type"`                 // Device type (e.g., "tv", "ps5") - for display/stats
	Driver     string                 `json:"driver"`               // Driver name (e.g., "aqara") - for control
	Parameters map[string]interface{} `json:"parameters,omitempty"` // Driver-specific parameters (overrides defaults)
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

// DayScheduleConfig defines start/end times for a day
type DayScheduleConfig struct {
	StartTime string `json:"start_time"` // HH:MM format (e.g., "22:00")
	EndTime   string `json:"end_time"`   // HH:MM format (e.g., "10:00")
}

// DowntimeConfig defines the global downtime schedule
// Supports three formats (in order of priority):
// 1. Per-day format: {"sunday": {...}, "monday": {...}, ...} - explicit per-day schedules
// 2. Weekday/weekend format: {"weekday": {...}, "weekend": {...}} - grouped schedules
// 3. Legacy flat format: {"start_time": "22:00", "end_time": "10:00"} - applies to all days
type DowntimeConfig struct {
	// Legacy flat fields (for backward compatibility)
	StartTime string `json:"start_time,omitempty"` // HH:MM format (e.g., "22:00")
	EndTime   string `json:"end_time,omitempty"`   // HH:MM format (e.g., "10:00")

	// Grouped schedules (weekday/weekend)
	Weekday *DayScheduleConfig `json:"weekday,omitempty"` // Default for Mon-Fri (if per-day not set)
	Weekend *DayScheduleConfig `json:"weekend,omitempty"` // Default for Sat-Sun (if per-day not set)

	// Explicit per-day schedules (highest priority)
	Sunday    *DayScheduleConfig `json:"sunday,omitempty"`
	Monday    *DayScheduleConfig `json:"monday,omitempty"`
	Tuesday   *DayScheduleConfig `json:"tuesday,omitempty"`
	Wednesday *DayScheduleConfig `json:"wednesday,omitempty"`
	Thursday  *DayScheduleConfig `json:"thursday,omitempty"`
	Friday    *DayScheduleConfig `json:"friday,omitempty"`
	Saturday  *DayScheduleConfig `json:"saturday,omitempty"`
}

// IsLegacyFormat returns true if using old flat start_time/end_time format
func (d *DowntimeConfig) IsLegacyFormat() bool {
	return d.StartTime != "" && d.EndTime != "" &&
		d.Weekday == nil && d.Weekend == nil &&
		d.Sunday == nil && d.Monday == nil && d.Tuesday == nil &&
		d.Wednesday == nil && d.Thursday == nil && d.Friday == nil && d.Saturday == nil
}

// HasPerDayConfig returns true if any per-day schedule is configured
func (d *DowntimeConfig) HasPerDayConfig() bool {
	return d.Sunday != nil || d.Monday != nil || d.Tuesday != nil ||
		d.Wednesday != nil || d.Thursday != nil || d.Friday != nil || d.Saturday != nil
}

// GetScheduleForDay returns the schedule for a specific day of the week
// Priority: per-day > weekday/weekend > legacy
func (d *DowntimeConfig) GetScheduleForDay(dayName string) *DayScheduleConfig {
	// First check explicit per-day config
	switch dayName {
	case "sunday":
		if d.Sunday != nil {
			return d.Sunday
		}
	case "monday":
		if d.Monday != nil {
			return d.Monday
		}
	case "tuesday":
		if d.Tuesday != nil {
			return d.Tuesday
		}
	case "wednesday":
		if d.Wednesday != nil {
			return d.Wednesday
		}
	case "thursday":
		if d.Thursday != nil {
			return d.Thursday
		}
	case "friday":
		if d.Friday != nil {
			return d.Friday
		}
	case "saturday":
		if d.Saturday != nil {
			return d.Saturday
		}
	}

	// Fall back to weekday/weekend
	switch dayName {
	case "saturday", "sunday":
		if d.Weekend != nil {
			return d.Weekend
		}
	default:
		if d.Weekday != nil {
			return d.Weekday
		}
	}

	// Fall back to legacy format
	if d.IsLegacyFormat() {
		return &DayScheduleConfig{StartTime: d.StartTime, EndTime: d.EndTime}
	}

	return nil
}

// GetWeekdaySchedule returns the weekday schedule, falling back to legacy format
// Deprecated: Use GetScheduleForDay instead
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
// Deprecated: Use GetScheduleForDay instead
func (d *DowntimeConfig) GetWeekendSchedule() *DayScheduleConfig {
	if d.Weekend != nil {
		return d.Weekend
	}
	if d.IsLegacyFormat() {
		return &DayScheduleConfig{StartTime: d.StartTime, EndTime: d.EndTime}
	}
	return nil
}

// Validate validates the movie time configuration
func (m *MovieTimeConfig) Validate() error {
	if !m.Enabled {
		return nil // No validation needed if disabled
	}

	if m.DurationMinutes <= 0 {
		return fmt.Errorf("movie time duration_minutes must be positive")
	}
	if m.BreakMinutes < 0 {
		return fmt.Errorf("movie time break_minutes cannot be negative")
	}
	if len(m.AllowedDeviceIDs) == 0 {
		return fmt.Errorf("movie time allowed_device_ids must not be empty when enabled")
	}
	return nil
}

// GetDuration returns the movie time duration, with default fallback
func (m *MovieTimeConfig) GetDuration() int {
	if m.DurationMinutes <= 0 {
		return 120 // Default: 2 hours
	}
	return m.DurationMinutes
}

// GetBreakMinutes returns the required break minutes, with default fallback
func (m *MovieTimeConfig) GetBreakMinutes() int {
	if m.BreakMinutes <= 0 {
		return 60 // Default: 1 hour
	}
	return m.BreakMinutes
}

// Validate validates the downtime configuration
func (d *DowntimeConfig) Validate() error {
	// Helper to validate a single schedule
	validateSchedule := func(name string, sched *DayScheduleConfig) error {
		if sched == nil {
			return nil
		}
		if _, _, err := parseTimeOfDay(sched.StartTime); err != nil {
			return fmt.Errorf("invalid %s downtime start_time '%s': %v", name, sched.StartTime, err)
		}
		if _, _, err := parseTimeOfDay(sched.EndTime); err != nil {
			return fmt.Errorf("invalid %s downtime end_time '%s': %v", name, sched.EndTime, err)
		}
		return nil
	}

	// Validate per-day schedules
	perDaySchedules := map[string]*DayScheduleConfig{
		"sunday":    d.Sunday,
		"monday":    d.Monday,
		"tuesday":   d.Tuesday,
		"wednesday": d.Wednesday,
		"thursday":  d.Thursday,
		"friday":    d.Friday,
		"saturday":  d.Saturday,
	}
	for name, sched := range perDaySchedules {
		if err := validateSchedule(name, sched); err != nil {
			return err
		}
	}

	// Validate weekday/weekend schedules
	if err := validateSchedule("weekday", d.Weekday); err != nil {
		return err
	}
	if err := validateSchedule("weekend", d.Weekend); err != nil {
		return err
	}

	// Validate legacy format
	if d.StartTime != "" || d.EndTime != "" {
		if d.StartTime == "" || d.EndTime == "" {
			return fmt.Errorf("both start_time and end_time must be set for legacy downtime format")
		}
		if _, _, err := parseTimeOfDay(d.StartTime); err != nil {
			return fmt.Errorf("invalid downtime start_time '%s': %v", d.StartTime, err)
		}
		if _, _, err := parseTimeOfDay(d.EndTime); err != nil {
			return fmt.Errorf("invalid downtime end_time '%s': %v", d.EndTime, err)
		}
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

	// Validate movie time config if present
	if c.MovieTime != nil {
		if err := c.MovieTime.Validate(); err != nil {
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
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
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
