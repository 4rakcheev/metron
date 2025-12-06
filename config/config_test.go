package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Database: DatabaseConfig{
					Path: "/path/to/db",
				},
				Security: SecurityConfig{
					APIKey: "test-key",
				},
				Telegram: TelegramConfig{
					BotToken: "test-token",
				},
				Aqara: AqaraConfig{
					AppID:  "app-id",
					AppKey: "app-key",
					KeyID:  "key-id",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: Config{
				Server: ServerConfig{
					Port: 0,
				},
				Database: DatabaseConfig{Path: "/path/to/db"},
				Security: SecurityConfig{APIKey: "test-key"},
				Telegram: TelegramConfig{BotToken: "test-token"},
				Aqara:    AqaraConfig{AppID: "app-id", AppKey: "app-key", KeyID: "key-id"},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too large",
			config: Config{
				Server: ServerConfig{
					Port: 70000,
				},
				Database: DatabaseConfig{Path: "/path/to/db"},
				Security: SecurityConfig{APIKey: "test-key"},
				Telegram: TelegramConfig{BotToken: "test-token"},
				Aqara:    AqaraConfig{AppID: "app-id", AppKey: "app-key", KeyID: "key-id"},
			},
			wantErr: true,
		},
		{
			name: "missing database path",
			config: Config{
				Server:   ServerConfig{Port: 8080},
				Security: SecurityConfig{APIKey: "test-key"},
				Telegram: TelegramConfig{BotToken: "test-token"},
				Aqara:    AqaraConfig{AppID: "app-id", AppKey: "app-key", KeyID: "key-id"},
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			config: Config{
				Server:   ServerConfig{Port: 8080},
				Database: DatabaseConfig{Path: "/path/to/db"},
				Telegram: TelegramConfig{BotToken: "test-token"},
				Aqara:    AqaraConfig{AppID: "app-id", AppKey: "app-key", KeyID: "key-id"},
			},
			wantErr: true,
		},
		{
			name: "missing Telegram bot token",
			config: Config{
				Server:   ServerConfig{Port: 8080},
				Database: DatabaseConfig{Path: "/path/to/db"},
				Security: SecurityConfig{APIKey: "test-key"},
				Aqara:    AqaraConfig{AppID: "app-id", AppKey: "app-key", KeyID: "key-id"},
			},
			wantErr: true,
		},
		{
			name: "missing Aqara credentials",
			config: Config{
				Server:   ServerConfig{Port: 8080},
				Database: DatabaseConfig{Path: "/path/to/db"},
				Security: SecurityConfig{APIKey: "test-key"},
				Telegram: TelegramConfig{BotToken: "test-token"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	validConfig := `{
		"server": {
			"host": "0.0.0.0",
			"port": 8080
		},
		"database": {
			"path": "/path/to/db"
		},
		"security": {
			"api_key": "test-key"
		},
		"telegram": {
			"bot_token": "test-token",
			"webhook_url": "https://example.com/webhook",
			"webhook_secret": "webhook-secret"
		},
		"aqara": {
			"app_id": "app-id",
			"app_key": "app-key",
			"key_id": "key-id",
			"base_url": "https://open-cn.aqara.com",
			"scenes": {
				"tv_pin_entry": "scene-1",
				"tv_warning": "scene-2",
				"tv_power_off": "scene-3"
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Test loading valid config
	config, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "/path/to/db", config.Database.Path)
	assert.Equal(t, "test-key", config.Security.APIKey)
	assert.Equal(t, "test-token", config.Telegram.BotToken)
	assert.Equal(t, "app-id", config.Aqara.AppID)
	assert.Equal(t, "scene-1", config.Aqara.Scenes.TVPINEntry)

	// Test loading non-existent file
	_, err = Load("/nonexistent/config.json")
	assert.ErrorIs(t, err, ErrConfigFileNotFound)

	// Test loading invalid JSON
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	err = os.WriteFile(invalidPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = Load(invalidPath)
	assert.Error(t, err)
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("METRON_HOST", "127.0.0.1")
	os.Setenv("METRON_PORT", "9090")
	os.Setenv("METRON_DB_PATH", "/custom/db/path")
	os.Setenv("METRON_API_KEY", "env-api-key")
	os.Setenv("METRON_TELEGRAM_BOT_TOKEN", "env-bot-token")
	os.Setenv("METRON_AQARA_APP_ID", "env-app-id")
	os.Setenv("METRON_AQARA_APP_KEY", "env-app-key")
	os.Setenv("METRON_AQARA_KEY_ID", "env-key-id")
	os.Setenv("METRON_ENABLE_IP_CHECK", "true")

	defer func() {
		os.Unsetenv("METRON_HOST")
		os.Unsetenv("METRON_PORT")
		os.Unsetenv("METRON_DB_PATH")
		os.Unsetenv("METRON_API_KEY")
		os.Unsetenv("METRON_TELEGRAM_BOT_TOKEN")
		os.Unsetenv("METRON_AQARA_APP_ID")
		os.Unsetenv("METRON_AQARA_APP_KEY")
		os.Unsetenv("METRON_AQARA_KEY_ID")
		os.Unsetenv("METRON_ENABLE_IP_CHECK")
	}()

	config, err := LoadFromEnv()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, "/custom/db/path", config.Database.Path)
	assert.Equal(t, "env-api-key", config.Security.APIKey)
	assert.Equal(t, "env-bot-token", config.Telegram.BotToken)
	assert.Equal(t, "env-app-id", config.Aqara.AppID)
	assert.Equal(t, true, config.Security.EnableIPCheck)
}
