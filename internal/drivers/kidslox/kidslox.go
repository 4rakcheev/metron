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
	// Default device parameters (can be overridden by device-specific parameters)
	DeviceID  string // Default Kidslox device ID
	ProfileID string // Default Kidslox profile ID
}

// Driver implements the DeviceDriver interface for Kidslox
type Driver struct {
	config         Config
	deviceRegistry *devices.Registry
	httpClient     *http.Client
}

// NewDriver creates a new Kidslox driver
func NewDriver(config Config, deviceRegistry *devices.Registry) *Driver {
	return &Driver{
		config:         config,
		deviceRegistry: deviceRegistry,
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

// getDeviceConfig looks up device and merges driver config + device parameters
// Device parameters override driver defaults
func (d *Driver) getDeviceConfig(session *core.Session) (deviceID, profileID string, err error) {
	// Look up device
	device, err := d.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get device %s: %w", session.DeviceID, err)
	}

	// Start with driver defaults
	deviceID = d.config.DeviceID
	profileID = d.config.ProfileID

	// Override with device-specific parameters if present
	if devID, ok := device.GetParameter("device_id").(string); ok && devID != "" {
		deviceID = devID
	}
	if profID, ok := device.GetParameter("profile_id").(string); ok && profID != "" {
		profileID = profID
	}

	// Validate required parameters
	if deviceID == "" {
		return "", "", fmt.Errorf("device_id is required (set in driver config or device parameters)")
	}
	if profileID == "" {
		return "", "", fmt.Errorf("profile_id is required (set in driver config or device parameters)")
	}

	return deviceID, profileID, nil
}

// StartSession initiates a session by unlocking the device and setting initial time
func (d *Driver) StartSession(ctx context.Context, session *core.Session) error {
	// Get merged config (driver defaults + device overrides)
	deviceID, profileID, err := d.getDeviceConfig(session)
	if err != nil {
		return err
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
	// Get merged config (driver defaults + device overrides)
	deviceID, _, err := d.getDeviceConfig(session)
	if err != nil {
		return err
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
	// Get merged config (driver defaults + device overrides)
	_, profileID, err := d.getDeviceConfig(session)
	if err != nil {
		return err
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
