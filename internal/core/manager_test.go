package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type mockStorage struct {
	children     map[string]*Child
	sessions     map[string]*Session
	dailyUsage   map[string]*DailyUsage
	failCreate   bool
	failGet      bool
	failUpdate   bool
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		children:   make(map[string]*Child),
		sessions:   make(map[string]*Session),
		dailyUsage: make(map[string]*DailyUsage),
	}
}

func (m *mockStorage) CreateChild(ctx context.Context, child *Child) error {
	if m.failCreate {
		return errors.New("create failed")
	}
	m.children[child.ID] = child
	return nil
}

func (m *mockStorage) GetChild(ctx context.Context, id string) (*Child, error) {
	if m.failGet {
		return nil, errors.New("get failed")
	}
	child, ok := m.children[id]
	if !ok {
		return nil, ErrChildNotFound
	}
	return child, nil
}

func (m *mockStorage) ListChildren(ctx context.Context) ([]*Child, error) {
	children := make([]*Child, 0, len(m.children))
	for _, child := range m.children {
		children = append(children, child)
	}
	return children, nil
}

func (m *mockStorage) UpdateChild(ctx context.Context, child *Child) error {
	if m.failUpdate {
		return errors.New("update failed")
	}
	if _, ok := m.children[child.ID]; !ok {
		return ErrChildNotFound
	}
	m.children[child.ID] = child
	return nil
}

func (m *mockStorage) CreateSession(ctx context.Context, session *Session) error {
	if m.failCreate {
		return errors.New("create failed")
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	if m.failGet {
		return nil, errors.New("get failed")
	}
	session, ok := m.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (m *mockStorage) ListActiveSessions(ctx context.Context) ([]*Session, error) {
	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		if session.Status == SessionStatusActive {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *mockStorage) UpdateSession(ctx context.Context, session *Session) error {
	if m.failUpdate {
		return errors.New("update failed")
	}
	if _, ok := m.sessions[session.ID]; !ok {
		return ErrSessionNotFound
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockStorage) DeleteSession(ctx context.Context, id string) error {
	if _, ok := m.sessions[id]; !ok {
		return ErrSessionNotFound
	}
	delete(m.sessions, id)
	return nil
}

func (m *mockStorage) GetDailyUsage(ctx context.Context, childID string, date time.Time) (*DailyUsage, error) {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		return &DailyUsage{
			ChildID:              childID,
			Date:                 date,
			MinutesUsed:          0,
			RewardMinutesGranted: 0,
			SessionCount:         0,
		}, nil
	}
	return usage, nil
}

func (m *mockStorage) IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		usage = &DailyUsage{
			ChildID:              childID,
			Date:                 date,
			MinutesUsed:          0,
			RewardMinutesGranted: 0,
			SessionCount:         0,
		}
	}
	usage.MinutesUsed += minutes
	m.dailyUsage[key] = usage
	return nil
}

func (m *mockStorage) IncrementSessionCount(ctx context.Context, childID string, date time.Time) error {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		usage = &DailyUsage{
			ChildID:              childID,
			Date:                 date,
			MinutesUsed:          0,
			RewardMinutesGranted: 0,
			SessionCount:         0,
		}
	}
	usage.SessionCount++
	m.dailyUsage[key] = usage
	return nil
}

func (m *mockStorage) DeleteChild(ctx context.Context, id string) error {
	if _, ok := m.children[id]; !ok {
		return ErrChildNotFound
	}
	delete(m.children, id)
	return nil
}

func (m *mockStorage) UpdateDailyUsage(ctx context.Context, usage *DailyUsage) error {
	key := usage.ChildID + usage.Date.Format("2006-01-02")
	m.dailyUsage[key] = usage
	return nil
}

func (m *mockStorage) GrantRewardMinutes(ctx context.Context, childID string, date time.Time, minutes int) error {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		usage = &DailyUsage{
			ChildID:              childID,
			Date:                 date,
			MinutesUsed:          0,
			RewardMinutesGranted: 0,
			SessionCount:         0,
		}
	}
	usage.RewardMinutesGranted += minutes
	m.dailyUsage[key] = usage
	return nil
}

func (m *mockStorage) ListAllSessions(ctx context.Context) ([]*Session, error) {
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (m *mockStorage) ListSessionsByChild(ctx context.Context, childID string) ([]*Session, error) {
	sessions := make([]*Session, 0)
	for _, session := range m.sessions {
		for _, cid := range session.ChildIDs {
			if cid == childID {
				sessions = append(sessions, session)
				break
			}
		}
	}
	return sessions, nil
}

func (m *mockStorage) Close() error {
	return nil
}

type mockDriver struct {
	name         string
	startCalled  bool
	stopCalled   bool
	warnCalled   bool
	failStart    bool
	failStop     bool
}

func (m *mockDriver) Name() string {
	return m.name
}

func (m *mockDriver) StartSession(ctx context.Context, session *Session) error {
	m.startCalled = true
	if m.failStart {
		return errors.New("start failed")
	}
	return nil
}

func (m *mockDriver) StopSession(ctx context.Context, session *Session) error {
	m.stopCalled = true
	if m.failStop {
		return errors.New("stop failed")
	}
	return nil
}

func (m *mockDriver) ApplyWarning(ctx context.Context, session *Session, minutesRemaining int) error {
	m.warnCalled = true
	return nil
}

type mockDevice struct {
	id     string
	name   string
	dtype  string
	driver string
}

func (m *mockDevice) GetID() string     { return m.id }
func (m *mockDevice) GetName() string   { return m.name }
func (m *mockDevice) GetType() string   { return m.dtype }
func (m *mockDevice) GetDriver() string { return m.driver }
func (m *mockDevice) GetParameter(key string) interface{} { return nil }
func (m *mockDevice) GetParameters() map[string]interface{} { return nil }

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
	drivers map[string]*mockDriver
}

func newMockDriverRegistry() *mockDriverRegistry {
	return &mockDriverRegistry{
		drivers: make(map[string]*mockDriver),
	}
}

func (m *mockDriverRegistry) Get(name string) (DeviceDriver, error) {
	driver, ok := m.drivers[name]
	if !ok {
		return nil, errors.New("driver not found")
	}
	return driver, nil
}

func (m *mockDriverRegistry) addDriver(driver *mockDriver) {
	m.drivers[driver.name] = driver
}

// Tests

func TestSessionManager_StartSession(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)

	device := &mockDevice{
		id:     "tv1",
		name:   "Living Room TV",
		dtype:  "tv",
		driver: "aqara",
	}
	deviceRegistry.addDevice(device)

	// Test StartSession
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "tv", session.DeviceType)
	assert.Equal(t, "tv1", session.DeviceID)
	assert.Equal(t, 30, session.ExpectedDuration)
	// RemainingMinutes is now calculated dynamically, not stored
	assert.GreaterOrEqual(t, session.CalculateRemainingMinutes(), 29)
	assert.LessOrEqual(t, session.CalculateRemainingMinutes(), 30)
	assert.Equal(t, SessionStatusActive, session.Status)
	assert.True(t, driver.startCalled)
}

func TestSessionManager_StartSession_InsufficientTime(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child with same limits for both weekday and weekend
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 60, // Set to same as weekday to make test consistent
	}
	storage.CreateChild(context.Background(), child)

	// Set usage to 50 minutes
	today := time.Now()
	storage.IncrementDailyUsage(context.Background(), "child1", today, 50)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Try to start session for 30 minutes (only 10 remaining)
	_, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientTime)
}

func TestSessionManager_StartSession_InvalidInputs(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Setup valid device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	tests := []struct {
		name          string
		deviceID      string
		childIDs      []string
		duration      int
		expectedError string
	}{
		{
			name:          "empty device ID",
			deviceID:      "",
			childIDs:      []string{"child1"},
			duration:      30,
			expectedError: "device ID cannot be empty",
		},
		{
			name:          "no children",
			deviceID:      "tv1",
			childIDs:      []string{},
			duration:      30,
			expectedError: ErrNoChildren.Error(),
		},
		{
			name:          "zero duration",
			deviceID:      "tv1",
			childIDs:      []string{"child1"},
			duration:      0,
			expectedError: ErrInvalidDuration.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.StartSession(context.Background(), tt.deviceID, tt.childIDs, tt.duration)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestSessionManager_ExtendSession(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Start session
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 20)
	require.NoError(t, err)

	// Extend session
	extended, err := manager.ExtendSession(context.Background(), session.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, 30, extended.ExpectedDuration)
	// RemainingMinutes should be close to 30, but may be slightly less due to elapsed time
	assert.GreaterOrEqual(t, extended.CalculateRemainingMinutes(), 29)
	assert.LessOrEqual(t, extended.CalculateRemainingMinutes(), 30)
}

func TestSessionManager_ExtendSession_InsufficientTime(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child with limited time (same for weekday and weekend)
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 60, // Set to same as weekday to make test consistent
	}
	storage.CreateChild(context.Background(), child)

	// Use 40 minutes already
	today := time.Now()
	storage.IncrementDailyUsage(context.Background(), "child1", today, 40)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Start session for 10 minutes (total 50)
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 10)
	require.NoError(t, err)

	// Modify session start time to simulate 8 minutes elapsed
	session.StartTime = time.Now().Add(-8 * time.Minute)
	storage.UpdateSession(context.Background(), session)

	// Try to extend by 15 minutes (would exceed daily limit)
	// Current usage: 40 (stored) + 8 (elapsed) = 48, limit 60, remaining 12
	// Extension request: 15 minutes (exceeds remaining 12)
	// Expected behavior: Extension should be capped to available 12 minutes
	extendedSession, err := manager.ExtendSession(context.Background(), session.ID, 15)
	assert.NoError(t, err, "Extension should succeed but be capped to available time")
	assert.NotNil(t, extendedSession)

	// Session duration should be increased by 12 (capped), not 15 (requested)
	// Original duration: 10, expected after extension: 10 + 12 = 22
	assert.Equal(t, 22, extendedSession.ExpectedDuration, "Extension should be capped to remaining 12 minutes")
}

func TestSessionManager_StopSession(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Start session
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	require.NoError(t, err)

	// Modify session start time to simulate elapsed time
	session.StartTime = time.Now().Add(-15 * time.Minute)
	storage.UpdateSession(context.Background(), session)

	// Stop session
	err = manager.StopSession(context.Background(), session.ID)
	require.NoError(t, err)
	assert.True(t, driver.stopCalled)

	// Verify session is completed
	stopped, err := manager.GetSession(context.Background(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, SessionStatusCompleted, stopped.Status)
	// Completed sessions should return 0 remaining minutes
	assert.Equal(t, 0, stopped.CalculateRemainingMinutes())

	// Verify daily usage was updated
	today := time.Now()
	usage, err := storage.GetDailyUsage(context.Background(), "child1", today)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, usage.MinutesUsed, 15)
}

func TestSessionManager_StopSession_NotActive(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Create completed session directly in storage
	session := &Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now(),
		ExpectedDuration: 30,
		Status:           SessionStatusCompleted,
	}
	storage.CreateSession(context.Background(), session)

	// Try to stop already completed session
	err := manager.StopSession(context.Background(), session.ID)
	assert.ErrorIs(t, err, ErrSessionNotActive)
}

func TestSessionManager_GetChildStatus(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Add some usage
	today := time.Now()
	storage.IncrementDailyUsage(context.Background(), "child1", today, 25)

	// Get status
	status, err := manager.GetChildStatus(context.Background(), "child1")
	require.NoError(t, err)
	assert.Equal(t, "Alice", status.Child.Name)
	assert.Equal(t, 25, status.TodayUsed)

	// Check if it's weekend or weekday
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		assert.Equal(t, 120, status.TodayLimit)
		assert.Equal(t, 95, status.TodayRemaining)
	} else {
		assert.Equal(t, 60, status.TodayLimit)
		assert.Equal(t, 35, status.TodayRemaining)
	}
}

func TestSessionManager_MultipleChildren(t *testing.T) {
	storage := newMockStorage()
	deviceRegistry := newMockDeviceRegistry()
	driverRegistry := newMockDriverRegistry()
	manager := NewSessionManager(storage, deviceRegistry, driverRegistry, nil, nil)

	// Create two children
	child1 := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	child2 := &Child{
		ID:           "child2",
		Name:         "Bob",
		WeekdayLimit: 45,
		WeekendLimit: 90,
	}
	storage.CreateChild(context.Background(), child1)
	storage.CreateChild(context.Background(), child2)

	// Create mock driver and device
	driver := &mockDriver{name: "aqara"}
	driverRegistry.addDriver(driver)
	device := &mockDevice{id: "tv1", name: "TV", dtype: "tv", driver: "aqara"}
	deviceRegistry.addDevice(device)

	// Start session for both children
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1", "child2"}, 30)
	require.NoError(t, err)
	assert.Len(t, session.ChildIDs, 2)

	// Modify session start time to simulate elapsed time
	session.StartTime = time.Now().Add(-20 * time.Minute)
	storage.UpdateSession(context.Background(), session)

	// Stop session
	err = manager.StopSession(context.Background(), session.ID)
	require.NoError(t, err)

	// Verify both children's usage was updated
	today := time.Now()
	usage1, err := storage.GetDailyUsage(context.Background(), "child1", today)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, usage1.MinutesUsed, 20)

	usage2, err := storage.GetDailyUsage(context.Background(), "child2", today)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, usage2.MinutesUsed, 20)
	assert.Equal(t, usage1.MinutesUsed, usage2.MinutesUsed)
}
