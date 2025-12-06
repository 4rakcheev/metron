package drivers

import (
	"errors"
	"fmt"
	"metron/internal/devices"
	"sync"
)

var (
	ErrDriverNotFound      = errors.New("driver not found")
	ErrDriverAlreadyExists = errors.New("driver already registered")
)

// Registry manages all registered device drivers
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]devices.DeviceDriver
}

// NewRegistry creates a new driver registry
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]devices.DeviceDriver),
	}
}

// Register adds a driver to the registry
func (r *Registry) Register(driver devices.DeviceDriver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := driver.Name()
	if _, exists := r.drivers[name]; exists {
		return fmt.Errorf("%w: %s", ErrDriverAlreadyExists, name)
	}

	r.drivers[name] = driver
	return nil
}

// Get retrieves a driver by name
func (r *Registry) Get(name string) (devices.DeviceDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	driver, exists := r.drivers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}

	return driver, nil
}

// List returns all registered driver names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.drivers))
	for name := range r.drivers {
		names = append(names, name)
	}
	return names
}

// Unregister removes a driver from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.drivers[name]; !exists {
		return fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}

	delete(r.drivers, name)
	return nil
}
