package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Security SecurityConfig `json:"security"`
	Aqara    AqaraConfig    `json:"aqara"`
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
	AppID       string        `json:"app_id"`
	AppKey      string        `json:"app_key"`
	KeyID       string        `json:"key_id"`
	AccessToken string        `json:"access_token"`
	BaseURL     string        `json:"base_url"`
	Devices     []AqaraDevice `json:"devices"`
	Scenes      AqaraScenes   `json:"scenes"`
}

// AqaraDevice represents a configured Aqara device
type AqaraDevice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DeviceType string `json:"device_type"` // "tv", etc.
}

// AqaraScenes contains scene IDs for different actions
type AqaraScenes struct {
	TVPINEntry string `json:"tv_pin_entry"`
	TVWarning  string `json:"tv_warning"`
	TVPowerOff string `json:"tv_power_off"`
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

	if c.Aqara.AppID == "" || c.Aqara.AppKey == "" || c.Aqara.KeyID == "" {
		return fmt.Errorf("%w: Aqara credentials are required", ErrInvalidConfig)
	}

	if c.Aqara.BaseURL == "" {
		c.Aqara.BaseURL = "https://open-cn.aqara.com" // default
	}

	return nil
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
		Aqara: AqaraConfig{
			AppID:       getEnv("METRON_AQARA_APP_ID", ""),
			AppKey:      getEnv("METRON_AQARA_APP_KEY", ""),
			KeyID:       getEnv("METRON_AQARA_KEY_ID", ""),
			AccessToken: getEnv("METRON_AQARA_ACCESS_TOKEN", ""),
			BaseURL:     getEnv("METRON_AQARA_BASE_URL", "https://open-cn.aqara.com"),
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
