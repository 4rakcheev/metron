package devices

import (
	"fmt"
	"sync"
)

// Device represents a controllable device
type Device struct {
	ID         string                 // Unique device identifier (e.g., "tv1", "ps5")
	Name       string                 // User-friendly name (e.g., "Living Room TV", "PlayStation 5")
	Type       string                 // Device type for display/stats (e.g., "tv", "ps5", "ipad")
	Driver     string                 // Driver to use for control (e.g., "aqara", "mock")
	Parameters map[string]interface{} // Driver-specific parameters (optional overrides)
}

// GetID returns the device ID
func (d *Device) GetID() string {
	return d.ID
}

// GetName returns the device name
func (d *Device) GetName() string {
	return d.Name
}

// GetType returns the device type
func (d *Device) GetType() string {
	return d.Type
}

// GetDriver returns the driver name
func (d *Device) GetDriver() string {
	return d.Driver
}

// GetParameters returns the device-specific parameters
func (d *Device) GetParameters() map[string]interface{} {
	return d.Parameters
}

// GetParameter returns a specific parameter value, or nil if not set
func (d *Device) GetParameter(key string) interface{} {
	if d.Parameters == nil {
		return nil
	}
	return d.Parameters[key]
}

// Registry manages registered devices
type Registry struct {
	devices map[string]*Device // device ID -> device
	mu      sync.RWMutex
}

// NewRegistry creates a new device registry
func NewRegistry() *Registry {
	return &Registry{
		devices: make(map[string]*Device),
	}
}

// Register adds a device to the registry
func (r *Registry) Register(device *Device) error {
	if device.ID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}
	// Telegram callback data has a 64-byte limit, so device IDs should be kept short
	// Typical callback: {"a":"newsession","s":3,"ci":0,"d":"DEVICE_ID","m":120}
	// Recommend max 15 chars for device ID to stay well under 64 bytes
	if len(device.ID) > 15 {
		return fmt.Errorf("device ID '%s' is too long (max 15 characters for Telegram compatibility)", device.ID)
	}
	if device.Name == "" {
		return fmt.Errorf("device name cannot be empty")
	}
	if device.Type == "" {
		return fmt.Errorf("device type cannot be empty")
	}
	if device.Driver == "" {
		return fmt.Errorf("device driver cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[device.ID]; exists {
		return fmt.Errorf("device %s already registered", device.ID)
	}

	r.devices[device.ID] = device
	return nil
}

// Get retrieves a device by ID
func (r *Registry) Get(id string) (*Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	device, exists := r.devices[id]
	if !exists {
		return nil, fmt.Errorf("device %s not found", id)
	}

	return device, nil
}

// List returns all registered devices
func (r *Registry) List() []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	devices := make([]*Device, 0, len(r.devices))
	for _, device := range r.devices {
		devices = append(devices, device)
	}

	return devices
}

// ListByDriver returns all devices using a specific driver
func (r *Registry) ListByDriver(driverName string) []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, device := range r.devices {
		if device.Driver == driverName {
			devices = append(devices, device)
		}
	}

	return devices
}
