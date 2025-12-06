package devices

import (
	"context"
	"metron/internal/core"
)

// DeviceDriver defines the interface that all device drivers must implement
type DeviceDriver interface {
	// Name returns the unique name of this driver (e.g., "aqara", "ps5", "ipad")
	Name() string

	// StartSession initiates a session on the device
	// It should trigger any necessary actions to allow usage (e.g., turn on TV, unlock device)
	StartSession(ctx context.Context, session *core.Session) error

	// StopSession ends a session on the device
	// It should trigger any necessary actions to stop usage (e.g., turn off TV, lock device)
	StopSession(ctx context.Context, session *core.Session) error

	// ApplyWarning sends a warning to the device (optional)
	// Returns nil if warnings are not supported
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
