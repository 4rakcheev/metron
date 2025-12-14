package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"metron/internal/core"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type mockStorage struct {
	sessions   map[string]*core.Session
	children   map[string]*core.Child
	dailyUsage map[string]int
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		sessions:   make(map[string]*core.Session),
		children:   make(map[string]*core.Child),
		dailyUsage: make(map[string]int),
	}
}

func (m *mockStorage) ListActiveSessions(ctx context.Context) ([]*core.Session, error) {
	sessions := make([]*core.Session, 0)
	for _, session := range m.sessions {
		if session.Status == core.SessionStatusActive || session.Status == core.SessionStatusPaused {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *mockStorage) GetSession(ctx context.Context, id string) (*core.Session, error) {
	session, ok := m.sessions[id]
	if !ok {
		return nil, core.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockStorage) UpdateSession(ctx context.Context, session *core.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockStorage) GetChild(ctx context.Context, id string) (*core.Child, error) {
	child, ok := m.children[id]
	if !ok {
		return nil, core.ErrChildNotFound
	}
	return child, nil
}

func (m *mockStorage) IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error {
	key := childID + date.Format("2006-01-02")
	m.dailyUsage[key] += minutes
	return nil
}

func (m *mockStorage) IncrementSessionCount(ctx context.Context, childID string, date time.Time) error {
	return nil
}

func (m *mockStorage) addSession(session *core.Session) {
	m.sessions[session.ID] = session
}

func (m *mockStorage) addChild(child *core.Child) {
	m.children[child.ID] = child
}

type mockDriver struct {
	stopCalls    []string
	warnCalls    []string
	failStop     bool
	failWarn     bool
}

func newMockDriver() *mockDriver {
	return &mockDriver{
		stopCalls: make([]string, 0),
		warnCalls: make([]string, 0),
	}
}

func (m *mockDriver) StopSession(ctx context.Context, session *core.Session) error {
	m.stopCalls = append(m.stopCalls, session.ID)
	if m.failStop {
		return errors.New("stop failed")
	}
	return nil
}

func (m *mockDriver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	m.warnCalls = append(m.warnCalls, session.ID)
	if m.failWarn {
		return errors.New("warn failed")
	}
	return nil
}

type mockDevice struct {
	id     string
	driver string
}

func (m *mockDevice) GetDriver() string {
	return m.driver
}

type mockDeviceRegistry struct {
	devices map[string]*mockDevice
}

func newMockDeviceRegistry() *mockDeviceRegistry {
	return &mockDeviceRegistry{
		devices: make(map[string]*mockDevice),
	}
}

func (m *mockDeviceRegistry) Get(id string) (Device, error) {
	device, ok := m.devices[id]
	if !ok {
		return nil, errors.New("device not found")
	}
	return device, nil
}

func (m *mockDeviceRegistry) addDevice(device *mockDevice) {
	m.devices[device.id] = device
}

type mockDriverRegistry struct {
	driver *mockDriver
}

func (m *mockDriverRegistry) Get(name string) (DeviceDriver, error) {
	if m.driver == nil {
		return nil, errors.New("driver not found")
	}
	return m.driver, nil
}

// Tests

func TestScheduler_ProcessSession_Expired(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create expired session (started 31 minutes ago, duration 30 minutes)
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-31 * time.Minute),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify session was stopped
	assert.Contains(t, driver.stopCalls, "session1")

	// Verify session status updated
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Equal(t, core.SessionStatusExpired, updated.Status)
	assert.Equal(t, 0, updated.CalculateRemainingMinutes())

	// Verify daily usage was updated
	today := time.Now()
	key := "child1" + today.Format("2006-01-02")
	assert.GreaterOrEqual(t, storage.dailyUsage[key], 30)
}

func TestScheduler_ProcessSession_Warning(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create session with 4 minutes remaining
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-26 * time.Minute),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify warning was sent
	assert.Contains(t, driver.warnCalls, "session1")

	// Verify session is still active
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Equal(t, core.SessionStatusActive, updated.Status)
	assert.LessOrEqual(t, updated.CalculateRemainingMinutes(), 5)
}

func TestScheduler_ProcessSession_NoWarning(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create session with 15 minutes remaining
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-15 * time.Minute),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify no warning was sent
	assert.Empty(t, driver.warnCalls)

	// Verify session is still active
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Equal(t, core.SessionStatusActive, updated.Status)
	assert.GreaterOrEqual(t, updated.CalculateRemainingMinutes(), 14)
}

func TestScheduler_ProcessSession_BreakRule(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child with break rule
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
		BreakRule: &core.BreakRule{
			BreakAfterMinutes:   30,
			BreakDurationMinutes: 10,
		},
	}
	storage.addChild(child)

	// Create session that has been running for 31 minutes
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-31 * time.Minute),
		ExpectedDuration: 60,
		Status:           core.SessionStatusActive,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify session was paused for break
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Equal(t, core.SessionStatusPaused, updated.Status)
	assert.NotNil(t, updated.BreakEndsAt)
	assert.NotNil(t, updated.LastBreakAt)

	// Verify warning was called (to notify about break)
	assert.Contains(t, driver.warnCalls, "session1")
}

func TestScheduler_ProcessSession_InBreak(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create session currently in break
	breakEnds := time.Now().Add(5 * time.Minute)
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-20 * time.Minute),
		ExpectedDuration: 60,
		Status:           core.SessionStatusPaused,
		BreakEndsAt:      &breakEnds,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify session is still paused
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Equal(t, core.SessionStatusPaused, updated.Status)
	assert.NotNil(t, updated.BreakEndsAt)

	// Verify no stop or warning calls
	assert.Empty(t, driver.stopCalls)
	assert.Empty(t, driver.warnCalls)
}

func TestScheduler_ProcessSession_BreakEnded(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create session with break that has ended
	breakEnds := time.Now().Add(-1 * time.Minute)
	session := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-20 * time.Minute),
		ExpectedDuration: 60,
		Status:           core.SessionStatusPaused,
		BreakEndsAt:      &breakEnds,
	}
	storage.addSession(session)

	// Process session
	err := scheduler.processSession(context.Background(), session)
	require.NoError(t, err)

	// Verify break ended
	updated, _ := storage.GetSession(context.Background(), "session1")
	assert.Nil(t, updated.BreakEndsAt)
}

func TestScheduler_Tick(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, time.Minute, nil, logger)

	// Create child
	child := &core.Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.addChild(child)

	// Create multiple sessions
	session1 := &core.Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-31 * time.Minute),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
	session2 := &core.Session{
		ID:               "session2",
		DeviceType:       "tv",
		DeviceID:         "tv2",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now().Add(-15 * time.Minute),
		ExpectedDuration: 30,
		Status:           core.SessionStatusActive,
	}
	storage.addSession(session1)
	storage.addSession(session2)

	// Run one tick
	scheduler.tick()

	// Verify session1 was stopped (expired)
	assert.Contains(t, driver.stopCalls, "session1")

	// Verify session2 is still active
	updated, _ := storage.GetSession(context.Background(), "session2")
	assert.Equal(t, core.SessionStatusActive, updated.Status)
}

func TestScheduler_StartStop(t *testing.T) {
	storage := newMockStorage()
	driver := newMockDriver()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := &mockDriverRegistry{driver: driver}

	// Register device
	deviceRegistry.addDevice(&mockDevice{id: "tv1", driver: "aqara"})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewScheduler(storage, deviceRegistry, driverRegistry, 100*time.Millisecond, nil, logger)

	// Start scheduler in goroutine
	go scheduler.Start()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop scheduler
	scheduler.Stop()

	// Wait a bit to ensure it stopped
	time.Sleep(100 * time.Millisecond)
}
