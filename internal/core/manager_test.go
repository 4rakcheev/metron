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

func (m *mockStorage) GetDailyUsage(ctx context.Context, childID string, date time.Time) (*DailyUsage, error) {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		return &DailyUsage{
			ChildID:      childID,
			Date:         date,
			MinutesUsed:  0,
			SessionCount: 0,
		}, nil
	}
	return usage, nil
}

func (m *mockStorage) IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error {
	key := childID + date.Format("2006-01-02")
	usage, ok := m.dailyUsage[key]
	if !ok {
		usage = &DailyUsage{
			ChildID:      childID,
			Date:         date,
			MinutesUsed:  0,
			SessionCount: 0,
		}
	}
	usage.MinutesUsed += minutes
	m.dailyUsage[key] = usage
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

type mockRegistry struct {
	drivers map[string]*mockDriver
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		drivers: make(map[string]*mockDriver),
	}
}

func (m *mockRegistry) Get(name string) (DeviceDriver, error) {
	driver, ok := m.drivers[name]
	if !ok {
		return nil, errors.New("driver not found")
	}
	return driver, nil
}

func (m *mockRegistry) addDriver(driver *mockDriver) {
	m.drivers[driver.name] = driver
}

// Tests

func TestSessionManager_StartSession(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Test StartSession
	session, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1"}, 30)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "tv", session.DeviceType)
	assert.Equal(t, "tv1", session.DeviceID)
	assert.Equal(t, 30, session.ExpectedDuration)
	assert.Equal(t, 30, session.RemainingMinutes)
	assert.Equal(t, SessionStatusActive, session.Status)
	assert.True(t, driver.startCalled)
}

func TestSessionManager_StartSession_InsufficientTime(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Set usage to 50 minutes
	today := time.Now()
	storage.IncrementDailyUsage(context.Background(), "child1", today, 50)

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Try to start session for 30 minutes (only 10 remaining)
	_, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1"}, 30)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientTime)
}

func TestSessionManager_StartSession_InvalidInputs(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	tests := []struct {
		name           string
		deviceType     string
		deviceID       string
		childIDs       []string
		duration       int
		expectedError  error
	}{
		{
			name:          "empty device type",
			deviceType:    "",
			deviceID:      "tv1",
			childIDs:      []string{"child1"},
			duration:      30,
			expectedError: ErrInvalidDeviceType,
		},
		{
			name:          "no children",
			deviceType:    "tv",
			deviceID:      "tv1",
			childIDs:      []string{},
			duration:      30,
			expectedError: ErrNoChildren,
		},
		{
			name:          "zero duration",
			deviceType:    "tv",
			deviceID:      "tv1",
			childIDs:      []string{"child1"},
			duration:      0,
			expectedError: ErrInvalidDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.StartSession(context.Background(), tt.deviceType, tt.deviceID, tt.childIDs, tt.duration)
			assert.ErrorIs(t, err, tt.expectedError)
		})
	}
}

func TestSessionManager_ExtendSession(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Start session
	session, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1"}, 20)
	require.NoError(t, err)

	// Extend session
	extended, err := manager.ExtendSession(context.Background(), session.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, 30, extended.ExpectedDuration)
	assert.Equal(t, 30, extended.RemainingMinutes)
}

func TestSessionManager_ExtendSession_InsufficientTime(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create test child with limited time
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Use 40 minutes already
	today := time.Now()
	storage.IncrementDailyUsage(context.Background(), "child1", today, 40)

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Start session for 10 minutes (total 50)
	session, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1"}, 10)
	require.NoError(t, err)

	// Modify session start time to simulate 8 minutes elapsed
	session.StartTime = time.Now().Add(-8 * time.Minute)
	storage.UpdateSession(context.Background(), session)

	// Try to extend by 15 minutes (would exceed daily limit)
	// Current usage: 40 (stored) + 8 (elapsed) = 48, limit 60, remaining 12
	// Extension request: 15 minutes (exceeds remaining 12)
	_, err = manager.ExtendSession(context.Background(), session.ID, 15)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientTime)
}

func TestSessionManager_StopSession(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create test child
	child := &Child{
		ID:           "child1",
		Name:         "Alice",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}
	storage.CreateChild(context.Background(), child)

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Start session
	session, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1"}, 30)
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
	assert.Equal(t, 0, stopped.RemainingMinutes)

	// Verify daily usage was updated
	today := time.Now()
	usage, err := storage.GetDailyUsage(context.Background(), "child1", today)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, usage.MinutesUsed, 15)
}

func TestSessionManager_StopSession_NotActive(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

	// Create completed session directly in storage
	session := &Session{
		ID:               "session1",
		DeviceType:       "tv",
		DeviceID:         "tv1",
		ChildIDs:         []string{"child1"},
		StartTime:        time.Now(),
		ExpectedDuration: 30,
		RemainingMinutes: 0,
		Status:           SessionStatusCompleted,
	}
	storage.CreateSession(context.Background(), session)

	// Try to stop already completed session
	err := manager.StopSession(context.Background(), session.ID)
	assert.ErrorIs(t, err, ErrSessionNotActive)
}

func TestSessionManager_GetChildStatus(t *testing.T) {
	storage := newMockStorage()
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

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
	registry := newMockRegistry()
	manager := NewSessionManager(storage, registry)

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

	// Create mock driver
	driver := &mockDriver{name: "tv"}
	registry.addDriver(driver)

	// Start session for both children
	session, err := manager.StartSession(context.Background(), "tv", "tv1", []string{"child1", "child2"}, 30)
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
