package devices

import (
	"context"
	"metron/internal/core"
)

// DeviceDriver defines the interface that all device drivers must implement
// Drivers internally look up device from session.DeviceID and merge driver config + device parameters
type DeviceDriver interface {
	// Name returns the unique name of this driver (e.g., "aqara", "ps5", "ipad")
	Name() string

	// StartSession initiates a session on the device
	// Driver internally looks up device from session.DeviceID, merges config, and executes
	StartSession(ctx context.Context, session *core.Session) error

	// StopSession ends a session on the device
	// Driver internally looks up device from session.DeviceID, merges config, and executes
	StopSession(ctx context.Context, session *core.Session) error

	// ApplyWarning sends a warning to the device (optional)
	// Driver internally looks up device from session.DeviceID, merges config, and executes
	ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error

	// GetLiveState retrieves the current state of the device (optional)
	// Returns nil if live state is not supported
	GetLiveState(ctx context.Context, deviceID string) (*DeviceState, error)
}

// DeviceState represents the current state of a device
type DeviceState struct {
	DeviceID  string
	IsActive  bool
	Metadata  map[string]interface{}
}

// DriverCapabilities describes what features a driver supports
type DriverCapabilities struct {
	SupportsWarnings  bool
	SupportsLiveState bool
	SupportsScheduling bool
}

// CapableDriver is an optional interface that drivers can implement
// to declare their capabilities
type CapableDriver interface {
	DeviceDriver
	Capabilities() DriverCapabilities
}

// ExtendableDriver is an optional interface that drivers can implement
// to support session extensions (e.g., adding more time to a running session)
type ExtendableDriver interface {
	DeviceDriver
	// ExtendSession extends an active session by adding more time
	// Driver internally looks up device from session.DeviceID, merges config, and executes
	ExtendSession(ctx context.Context, session *core.Session, additionalMinutes int) error
}
