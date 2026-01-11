// Package passive provides a no-op device driver for devices that are controlled
// by external agents (e.g., Windows agent). The driver logs actions but doesn't
// perform any actual device control.
package passive

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"metron/internal/devices"
)

const DriverName = "passive"

// Driver implements the DeviceDriver interface with no-op behavior.
// It's used for devices that are controlled by external agents.
type Driver struct {
	logger *slog.Logger
}

// NewDriver creates a new passive driver
func NewDriver(logger *slog.Logger) *Driver {
	return &Driver{
		logger: logger.With("driver", DriverName),
	}
}

// Name returns the driver name
func (d *Driver) Name() string {
	return DriverName
}

// StartSession logs the session start but performs no action.
// The actual device control is handled by the external agent.
func (d *Driver) StartSession(ctx context.Context, session *core.Session) error {
	d.logger.Info("passive driver: session started",
		"session_id", session.ID,
		"device_id", session.DeviceID,
		"expected_duration", session.ExpectedDuration,
	)
	return nil
}

// StopSession logs the session stop but performs no action.
// The actual device control is handled by the external agent.
func (d *Driver) StopSession(ctx context.Context, session *core.Session) error {
	d.logger.Info("passive driver: session stopped",
		"session_id", session.ID,
		"device_id", session.DeviceID,
	)
	return nil
}

// ApplyWarning logs the warning but performs no action.
// The actual warning display is handled by the external agent.
func (d *Driver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	d.logger.Info("passive driver: warning applied",
		"session_id", session.ID,
		"device_id", session.DeviceID,
		"minutes_remaining", minutesRemaining,
	)
	return nil
}

// GetLiveState returns nil as passive devices don't support live state queries.
// The external agent maintains its own state.
func (d *Driver) GetLiveState(ctx context.Context, deviceID string) (*devices.DeviceState, error) {
	return nil, nil
}

// Capabilities returns the driver capabilities
func (d *Driver) Capabilities() devices.DriverCapabilities {
	return devices.DriverCapabilities{
		SupportsWarnings:   true, // Agent handles warnings
		SupportsLiveState:  false,
		SupportsScheduling: false,
	}
}

// Ensure Driver implements the interfaces
var (
	_ devices.DeviceDriver  = (*Driver)(nil)
	_ devices.CapableDriver = (*Driver)(nil)
)
