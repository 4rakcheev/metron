package kidslox

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

// mockDevice is a test mock for devices.Device
type mockDevice struct {
	id         string
	params     map[string]interface{}
}

func (m *mockDevice) GetID() string     { return m.id }
func (m *mockDevice) GetName() string   { return "Test Device" }
func (m *mockDevice) GetType() string   { return "ipad" }
func (m *mockDevice) GetDriver() string { return "kidslox" }
func (m *mockDevice) GetParameter(key string) interface{} {
	if m.params == nil {
		return nil
	}
	return m.params[key]
}
func (m *mockDevice) GetParameters() map[string]interface{} {
	return m.params
}

func TestDriver_Name(t *testing.T) {
	driver := NewDriver(Config{})
	assert.Equal(t, "kidslox", driver.Name())
}

func TestDriver_Capabilities(t *testing.T) {
	driver := NewDriver(Config{})
	caps := driver.Capabilities()

	assert.False(t, caps.SupportsWarnings, "Kidslox doesn't support warnings")
	assert.False(t, caps.SupportsLiveState, "Kidslox doesn't support live state")
	assert.True(t, caps.SupportsScheduling, "Kidslox supports scheduling")
}

func TestDriver_StartSession(t *testing.T) {
	// Track API calls
	var unlockCalled, extendCalled bool
	var unlockBody, extendBody map[string]interface{}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body
		bodyBytes, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		json.Unmarshal(bodyBytes, &body)

		// Check headers
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		if r.URL.Path == "/api/actions/"+r.URL.Path[len("/api/actions/"):] {
			// Unlock call
			unlockCalled = true
			unlockBody = body
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Action sent"})
		} else if r.URL.Path == "/api/profiles/test-profile-123/time-restrictions/extensions" {
			// Extend time call
			extendCalled = true
			extendBody = body
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Success"})
		}
	}))
	defer server.Close()

	// Create driver
	config := Config{
		BaseURL:   server.URL,
		APIKey:    "test-api-key",
		AccountID: "test-account-123",
	}
	driver := NewDriver(config)

	// Create test session
	session := &core.Session{
		ID:               "session1",
		DeviceID:         "ipad1",
		ExpectedDuration: 30,
	}

	// Create mock device with parameters
	device := &mockDevice{
		id: "ipad1",
		params: map[string]interface{}{
			"device_id":  "test-device-456",
			"profile_id": "test-profile-123",
		},
	}

	// Add device to context
	ctx := core.WithDevice(context.Background(), device)

	// Call StartSession
	err := driver.StartSession(ctx, session)
	require.NoError(t, err)

	// Verify unlock was called
	assert.True(t, unlockCalled, "Unlock should be called")
	action := unlockBody["action"].(map[string]interface{})
	assert.Equal(t, "profile", action["action"])
	assert.Equal(t, "test-account-123", action["creator"])
	assert.Equal(t, "test-device-456", action["device"])
	assert.Equal(t, "test-profile-123", action["profile"])

	// Verify extend time was called
	assert.True(t, extendCalled, "Extend time should be called")
	ext := extendBody["timeRestrictionExtension"].(map[string]interface{})
	assert.Equal(t, float64(1800), ext["seconds"]) // 30 minutes = 1800 seconds
}

func TestDriver_StartSession_MissingDeviceID(t *testing.T) {
	driver := NewDriver(Config{})
	session := &core.Session{}

	// Create mock device without device_id parameter
	device := &mockDevice{
		params: map[string]interface{}{
			"profile_id": "test-profile",
		},
	}

	ctx := core.WithDevice(context.Background(), device)

	err := driver.StartSession(ctx, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "device_id parameter is required")
}

func TestDriver_StartSession_MissingProfileID(t *testing.T) {
	driver := NewDriver(Config{})
	session := &core.Session{}

	// Create mock device without profile_id parameter
	device := &mockDevice{
		params: map[string]interface{}{
			"device_id": "test-device",
		},
	}

	ctx := core.WithDevice(context.Background(), device)

	err := driver.StartSession(ctx, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "profile_id parameter is required")
}

func TestDriver_StopSession(t *testing.T) {
	// Track API call
	var lockCalled bool
	var lockBody map[string]interface{}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		json.Unmarshal(bodyBytes, &body)

		// Check headers
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		lockCalled = true
		lockBody = body
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Action sent"})
	}))
	defer server.Close()

	// Create driver
	config := Config{
		BaseURL:   server.URL,
		APIKey:    "test-api-key",
		AccountID: "test-account-123",
	}
	driver := NewDriver(config)

	// Create test session
	session := &core.Session{
		ID:       "session1",
		DeviceID: "ipad1",
	}

	// Create mock device
	device := &mockDevice{
		id: "ipad1",
		params: map[string]interface{}{
			"device_id":  "test-device-456",
			"profile_id": "test-profile-123",
		},
	}

	ctx := core.WithDevice(context.Background(), device)

	// Call StopSession
	err := driver.StopSession(ctx, session)
	require.NoError(t, err)

	// Verify lock was called with correct profile
	assert.True(t, lockCalled)
	action := lockBody["action"].(map[string]interface{})
	assert.Equal(t, "profile", action["action"])
	assert.Equal(t, "test-account-123", action["creator"])
	assert.Equal(t, "test-device-456", action["device"])
	assert.Equal(t, LockProfileID, action["profile"]) // Should use lock profile
}

func TestDriver_ExtendSession(t *testing.T) {
	// Track API call
	var extendCalled bool
	var extendBody map[string]interface{}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		json.Unmarshal(bodyBytes, &body)

		// Check headers
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		extendCalled = true
		extendBody = body
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Success"})
	}))
	defer server.Close()

	// Create driver
	config := Config{
		BaseURL:   server.URL,
		APIKey:    "test-api-key",
		AccountID: "test-account-123",
	}
	driver := NewDriver(config)

	// Create test session
	session := &core.Session{
		ID:               "session1",
		DeviceID:         "ipad1",
		StartTime:        time.Now().Add(-10 * time.Minute),
		ExpectedDuration: 20,
	}

	// Create mock device
	device := &mockDevice{
		id: "ipad1",
		params: map[string]interface{}{
			"device_id":  "test-device-456",
			"profile_id": "test-profile-123",
		},
	}

	ctx := core.WithDevice(context.Background(), device)

	// Call ExtendSession
	err := driver.ExtendSession(ctx, session, 15)
	require.NoError(t, err)

	// Verify extend was called
	assert.True(t, extendCalled)
	ext := extendBody["timeRestrictionExtension"].(map[string]interface{})
	assert.Equal(t, float64(900), ext["seconds"]) // 15 minutes = 900 seconds
}

func TestDriver_ApplyWarning(t *testing.T) {
	driver := NewDriver(Config{})
	session := &core.Session{}

	// Warning should be a no-op
	err := driver.ApplyWarning(context.Background(), session, 5)
	assert.NoError(t, err)
}

func TestDriver_GetLiveState(t *testing.T) {
	driver := NewDriver(Config{})

	// Live state not supported
	state, err := driver.GetLiveState(context.Background(), "device1")
	assert.NoError(t, err)
	assert.Nil(t, state)
}

func TestDriver_APIError(t *testing.T) {
	// Create server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request"))
	}))
	defer server.Close()

	config := Config{
		BaseURL:   server.URL,
		APIKey:    "test-api-key",
		AccountID: "test-account-123",
	}
	driver := NewDriver(config)

	session := &core.Session{
		ID:               "session1",
		ExpectedDuration: 30,
	}

	device := &mockDevice{
		params: map[string]interface{}{
			"device_id":  "test-device",
			"profile_id": "test-profile",
		},
	}

	ctx := core.WithDevice(context.Background(), device)

	// Should fail
	err := driver.StartSession(ctx, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestDriver_InterfaceImplementation(t *testing.T) {
	driver := NewDriver(Config{})

	// Verify implements DeviceDriver
	var _ devices.DeviceDriver = driver

	// Verify implements CapableDriver
	var _ devices.CapableDriver = driver

	// Verify implements ExtendableDriver
	var _ devices.ExtendableDriver = driver
}
