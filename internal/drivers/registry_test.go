package drivers

import (
	"context"
	"metron/internal/core"
	"metron/internal/devices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDriver is a simple mock implementation of DeviceDriver
type mockDriver struct {
	name string
}

func (m *mockDriver) Name() string {
	return m.name
}

func (m *mockDriver) StartSession(ctx context.Context, session *core.Session) error {
	return nil
}

func (m *mockDriver) StopSession(ctx context.Context, session *core.Session) error {
	return nil
}

func (m *mockDriver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	return nil
}

func (m *mockDriver) GetLiveState(ctx context.Context, deviceID string) (*devices.DeviceState, error) {
	return nil, nil
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	driver1 := &mockDriver{name: "driver1"}
	driver2 := &mockDriver{name: "driver2"}
	driver1Duplicate := &mockDriver{name: "driver1"}

	// Register first driver
	err := registry.Register(driver1)
	require.NoError(t, err)

	// Register second driver
	err = registry.Register(driver2)
	require.NoError(t, err)

	// Attempt to register duplicate
	err = registry.Register(driver1Duplicate)
	assert.ErrorIs(t, err, ErrDriverAlreadyExists)

	// Verify drivers are registered
	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "driver1")
	assert.Contains(t, names, "driver2")
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	driver := &mockDriver{name: "test-driver"}

	// Get non-existent driver
	_, err := registry.Get("test-driver")
	assert.ErrorIs(t, err, ErrDriverNotFound)

	// Register driver
	err = registry.Register(driver)
	require.NoError(t, err)

	// Get existing driver
	retrieved, err := registry.Get("test-driver")
	require.NoError(t, err)
	assert.Equal(t, driver, retrieved)
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Empty registry
	names := registry.List()
	assert.Empty(t, names)

	// Add drivers
	registry.Register(&mockDriver{name: "driver1"})
	registry.Register(&mockDriver{name: "driver2"})
	registry.Register(&mockDriver{name: "driver3"})

	// List all drivers
	names = registry.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "driver1")
	assert.Contains(t, names, "driver2")
	assert.Contains(t, names, "driver3")
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()
	driver := &mockDriver{name: "test-driver"}

	// Unregister non-existent driver
	err := registry.Unregister("test-driver")
	assert.ErrorIs(t, err, ErrDriverNotFound)

	// Register driver
	err = registry.Register(driver)
	require.NoError(t, err)

	// Unregister driver
	err = registry.Unregister("test-driver")
	require.NoError(t, err)

	// Verify driver is removed
	_, err = registry.Get("test-driver")
	assert.ErrorIs(t, err, ErrDriverNotFound)
}

func TestRegistry_Concurrent(t *testing.T) {
	registry := NewRegistry()

	// Test concurrent registration and retrieval
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			driver := &mockDriver{name: string(rune('a' + idx))}
			registry.Register(driver)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all drivers were registered
	names := registry.List()
	assert.Len(t, names, 10)
}
