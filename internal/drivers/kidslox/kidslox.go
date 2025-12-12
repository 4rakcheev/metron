package kidslox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"metron/internal/core"
	"metron/internal/devices"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	// LockProfileID is the special Kidslox profile ID for locking devices
	LockProfileID = "aaaaaaaa-bbbb-cccc-dddd-000000000001"
)

// Config contains Kidslox API configuration
type Config struct {
	BaseURL   string // API base URL
	APIKey    string // Static API key for authentication
	AccountID string // Account ID for actions
}

// Driver implements the DeviceDriver interface for Kidslox
type Driver struct {
	config     Config
	httpClient *http.Client
}

// NewDriver creates a new Kidslox driver
func NewDriver(config Config) *Driver {
	return &Driver{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the driver name
func (d *Driver) Name() string {
	return "kidslox"
}

// Capabilities returns the driver capabilities
func (d *Driver) Capabilities() devices.DriverCapabilities {
	return devices.DriverCapabilities{
		SupportsWarnings:   false, // Kidslox doesn't support warnings
		SupportsLiveState:  false, // Not implemented in this version
		SupportsScheduling: true,  // Can schedule sessions
	}
}

// StartSession initiates a session by unlocking the device and setting initial time
func (d *Driver) StartSession(ctx context.Context, session *core.Session) error {
	// Get device-specific parameters
	device, err := core.GetDeviceFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	deviceID, ok := device.GetParameter("device_id").(string)
	if !ok || deviceID == "" {
		return fmt.Errorf("device_id parameter is required for Kidslox devices")
	}

	profileID, ok := device.GetParameter("profile_id").(string)
	if !ok || profileID == "" {
		return fmt.Errorf("profile_id parameter is required for Kidslox devices")
	}

	// Step 1: Unlock device (assign child profile)
	if err := d.unlockDevice(ctx, deviceID, profileID); err != nil {
		return fmt.Errorf("failed to unlock device: %w", err)
	}

	// Step 2: Set initial time restriction
	durationSeconds := session.ExpectedDuration * 60
	if err := d.extendTime(ctx, profileID, durationSeconds); err != nil {
		return fmt.Errorf("failed to set initial time: %w", err)
	}

	return nil
}

// StopSession ends a session by locking the device
func (d *Driver) StopSession(ctx context.Context, session *core.Session) error {
	// Get device-specific parameters
	device, err := core.GetDeviceFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	deviceID, ok := device.GetParameter("device_id").(string)
	if !ok || deviceID == "" {
		return fmt.Errorf("device_id parameter is required for Kidslox devices")
	}

	// Lock device (assign lock profile)
	if err := d.lockDevice(ctx, deviceID); err != nil {
		return fmt.Errorf("failed to lock device: %w", err)
	}

	return nil
}

// ApplyWarning is not supported by Kidslox
func (d *Driver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	// Kidslox doesn't support warnings - this is a no-op
	return nil
}

// ExtendSession extends an active session by adding more time
func (d *Driver) ExtendSession(ctx context.Context, session *core.Session, additionalMinutes int) error {
	// Get device-specific parameters
	device, err := core.GetDeviceFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	profileID, ok := device.GetParameter("profile_id").(string)
	if !ok || profileID == "" {
		return fmt.Errorf("profile_id parameter is required for Kidslox devices")
	}

	// Extend time restriction
	additionalSeconds := additionalMinutes * 60
	if err := d.extendTime(ctx, profileID, additionalSeconds); err != nil {
		return fmt.Errorf("failed to extend time: %w", err)
	}

	return nil
}

// GetLiveState retrieves the current state of a device (not supported in this version)
func (d *Driver) GetLiveState(ctx context.Context, deviceID string) (*devices.DeviceState, error) {
	// Not implemented
	return nil, nil
}

// extendTime extends the time restriction for a profile
func (d *Driver) extendTime(ctx context.Context, profileID string, seconds int) error {
	url := fmt.Sprintf("%s/api/profiles/%s/time-restrictions/extensions", d.config.BaseURL, profileID)

	body := map[string]interface{}{
		"timeRestrictionExtension": map[string]int{
			"seconds": seconds,
		},
	}

	req, err := d.newRequest(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("extend time failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// lockDevice locks a device by assigning the lock profile
func (d *Driver) lockDevice(ctx context.Context, deviceID string) error {
	return d.setProfile(ctx, deviceID, LockProfileID)
}

// unlockDevice unlocks a device by assigning a child profile
func (d *Driver) unlockDevice(ctx context.Context, deviceID, profileID string) error {
	return d.setProfile(ctx, deviceID, profileID)
}

// setProfile assigns a profile to a device
func (d *Driver) setProfile(ctx context.Context, deviceID, profileID string) error {
	actionID := uuid.New().String()
	url := fmt.Sprintf("%s/api/actions/%s", d.config.BaseURL, actionID)

	body := map[string]interface{}{
		"action": map[string]string{
			"action":  "profile",
			"creator": d.config.AccountID,
			"device":  deviceID,
			"profile": profileID,
		},
	}

	req, err := d.newRequest(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set profile failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// newRequest creates a new HTTP request with standard headers
func (d *Driver) newRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", d.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
