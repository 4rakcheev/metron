package aqara

import (
	"context"
	"encoding/json"
	"io"
	"metron/internal/core"
	"metron/internal/devices"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriver_Name(t *testing.T) {
	driver := NewDriver(Config{})
	assert.Equal(t, "aqara", driver.Name())
}

func TestDriver_Capabilities(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantWarn   bool
		wantLive   bool
		wantSched  bool
	}{
		{
			name: "with warning scene",
			config: Config{
				WarnSceneID: "warn-123",
			},
			wantWarn:  true,
			wantLive:  false,
			wantSched: true,
		},
		{
			name:      "without warning scene",
			config:    Config{},
			wantWarn:  false,
			wantLive:  false,
			wantSched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := NewDriver(tt.config)
			caps := driver.Capabilities()
			assert.Equal(t, tt.wantWarn, caps.SupportsWarnings)
			assert.Equal(t, tt.wantLive, caps.SupportsLiveState)
			assert.Equal(t, tt.wantSched, caps.SupportsScheduling)
		})
	}
}

func TestDriver_StartSession(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v3.0/open/api", r.URL.Path)

		// Verify headers
		assert.NotEmpty(t, r.Header.Get("Accesstoken"))
		assert.NotEmpty(t, r.Header.Get("Appid"))
		assert.NotEmpty(t, r.Header.Get("Keyid"))
		assert.NotEmpty(t, r.Header.Get("Time"))
		assert.NotEmpty(t, r.Header.Get("Nonce"))
		assert.NotEmpty(t, r.Header.Get("Sign"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(body, &req)
		require.NoError(t, err)
		assert.Equal(t, "config.scene.run", req["intent"])
		data, ok := req["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map")
		assert.Equal(t, "pin-scene-123", data["sceneId"])

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      0,
			"message":   "success",
			"result":    map[string]interface{}{},
			"requestId": "test-request-id",
		})
	}))
	defer server.Close()

	// Create driver
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
		PINSceneID:  "pin-scene-123",
	})

	// Test StartSession
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.StartSession(context.Background(), session)
	assert.NoError(t, err)
}

func TestDriver_StartSession_NoPINScene(t *testing.T) {
	driver := NewDriver(Config{
		AppID:   "test-app-id",
		AppKey:  "test-app-key",
		KeyID:   "test-key-id",
		BaseURL: "http://localhost",
	})

	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.StartSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PIN scene ID not configured")
}

func TestDriver_StopSession(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v3.0/open/api", r.URL.Path)

		// Verify body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(body, &req)
		require.NoError(t, err)
		assert.Equal(t, "config.scene.run", req["intent"])
		data, ok := req["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map")
		assert.Equal(t, "off-scene-456", data["sceneId"])

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      0,
			"message":   "success",
			"result":    map[string]interface{}{},
			"requestId": "test-request-id",
		})
	}))
	defer server.Close()

	// Create driver
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
		OffSceneID:  "off-scene-456",
	})

	// Test StopSession
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.StopSession(context.Background(), session)
	assert.NoError(t, err)
}

func TestDriver_ApplyWarning(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v3.0/open/api", r.URL.Path)

		// Verify body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(body, &req)
		require.NoError(t, err)
		assert.Equal(t, "config.scene.run", req["intent"])
		data, ok := req["data"].(map[string]interface{})
		require.True(t, ok, "data field should be a map")
		assert.Equal(t, "warn-scene-789", data["sceneId"])

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      0,
			"message":   "success",
			"result":    map[string]interface{}{},
			"requestId": "test-request-id",
		})
	}))
	defer server.Close()

	// Create driver with warning scene
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
		WarnSceneID: "warn-scene-789",
	})

	// Test ApplyWarning
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.ApplyWarning(context.Background(), session, 5)
	assert.NoError(t, err)
}

func TestDriver_ApplyWarning_NoScene(t *testing.T) {
	// Create driver without warning scene
	driver := NewDriver(Config{
		AppID:   "test-app-id",
		AppKey:  "test-app-key",
		KeyID:   "test-key-id",
		BaseURL: "http://localhost",
	})

	// Test ApplyWarning - should succeed but do nothing
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.ApplyWarning(context.Background(), session, 5)
	assert.NoError(t, err)
}

func TestDriver_APIError(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    1001,
			"message": "Invalid scene ID",
		})
	}))
	defer server.Close()

	// Create driver
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
		PINSceneID:  "invalid-scene",
	})

	// Test StartSession with API error
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.StartSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error code 1001")
}

func TestDriver_HTTPError(t *testing.T) {
	// Create mock server that returns HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create driver
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
		BaseURL:     server.URL,
		PINSceneID:  "pin-scene-123",
	})

	// Test StartSession with HTTP error
	session := &core.Session{
		ID:         "session-1",
		DeviceType: "tv",
		DeviceID:   "tv-1",
	}

	err := driver.StartSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with status 500")
}

func TestDriver_GetLiveState(t *testing.T) {
	driver := NewDriver(Config{})

	// GetLiveState is not implemented in MVP
	state, err := driver.GetLiveState(context.Background(), "device-1")
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestDriver_InterfaceImplementation(t *testing.T) {
	// Verify that Driver implements DeviceDriver
	var _ devices.DeviceDriver = (*Driver)(nil)

	// Verify that Driver implements CapableDriver
	var _ devices.CapableDriver = (*Driver)(nil)
}

func TestGenerateSignature(t *testing.T) {
	driver := NewDriver(Config{
		AppID:       "test-app-id",
		AppKey:      "test-app-key",
		KeyID:       "test-key-id",
		AccessToken: "test-access-token",
	})

	timestamp := int64(1638360000000)
	nonce := "123456789"

	// Generate signature
	sig1 := driver.generateSignature(timestamp, nonce)
	assert.NotEmpty(t, sig1)
	assert.Equal(t, 32, len(sig1)) // MD5 hash is 32 hex characters

	// Same input should produce same signature
	sig2 := driver.generateSignature(timestamp, nonce)
	assert.Equal(t, sig1, sig2)

	// Different input should produce different signature
	sig3 := driver.generateSignature(timestamp+1, nonce)
	assert.NotEqual(t, sig1, sig3)
}

func TestGenerateNonce(t *testing.T) {
	nonce1 := generateNonce()
	assert.NotEmpty(t, nonce1)

	// Sleep to ensure different timestamp
	time.Sleep(1 * time.Millisecond)

	nonce2 := generateNonce()
	assert.NotEmpty(t, nonce2)
	assert.NotEqual(t, nonce1, nonce2)
}
