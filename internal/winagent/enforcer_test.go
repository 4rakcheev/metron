package winagent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

// MockMetronClient is a test double for MetronClient
type MockMetronClient struct {
	StatusToReturn *SessionStatus
	ErrorToReturn  error
	CallCount      int
	LastDeviceID   string
}

func (m *MockMetronClient) GetSessionStatus(ctx context.Context, deviceID string) (*SessionStatus, error) {
	m.CallCount++
	m.LastDeviceID = deviceID
	return m.StatusToReturn, m.ErrorToReturn
}

// MockPlatform is a test double for Platform
type MockPlatform struct {
	LockCallCount    int
	LockError        error
	WarningCallCount int
	WarningError     error
	LastWarningTitle string
	LastWarningMsg   string
}

func (m *MockPlatform) LockWorkstation() error {
	m.LockCallCount++
	return m.LockError
}

func (m *MockPlatform) ShowWarningNotification(title, message string) error {
	m.WarningCallCount++
	m.LastWarningTitle = title
	m.LastWarningMsg = message
	return m.WarningError
}

func newTestEnforcer(client MetronClient, platform Platform, clock Clock) *Enforcer {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	config := &Config{
		DeviceID:     "test-device",
		PollInterval: 15 * time.Second,
		GracePeriod:  30 * time.Second,
	}
	return NewEnforcer(client, platform, clock, config, logger)
}

func TestNoSession_Locks(t *testing.T) {
	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     false,
			ServerTime: time.Now(),
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: time.Now()}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 1 {
		t.Errorf("Expected lock to be called once, got %d", platform.LockCallCount)
	}
}

func TestActiveSession_NoLock(t *testing.T) {
	now := time.Now()
	sessionID := "session-123"
	endsAt := now.Add(30 * time.Minute)

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     true,
			SessionID:  &sessionID,
			EndsAt:     &endsAt,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 0 {
		t.Errorf("Expected no lock calls with active session, got %d", platform.LockCallCount)
	}
}

func TestBypassMode_NoLock(t *testing.T) {
	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     false,
			BypassMode: true,
			ServerTime: time.Now(),
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: time.Now()}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 0 {
		t.Errorf("Expected no lock calls with bypass mode, got %d", platform.LockCallCount)
	}
}

func TestWarning_FiveMinutes(t *testing.T) {
	now := time.Now()
	sessionID := "session-123"
	endsAt := now.Add(4 * time.Minute) // Less than 5 minutes
	warnAt := now.Add(-1 * time.Minute)

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     true,
			SessionID:  &sessionID,
			EndsAt:     &endsAt,
			WarnAt:     &warnAt,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.WarningCallCount != 1 {
		t.Errorf("Expected warning to be shown, got %d calls", platform.WarningCallCount)
	}
	if platform.LastWarningTitle != "Screen Time Warning" {
		t.Errorf("Expected title 'Screen Time Warning', got '%s'", platform.LastWarningTitle)
	}
}

func TestWarning_OnlyOnce(t *testing.T) {
	now := time.Now()
	sessionID := "session-123"
	endsAt := now.Add(4 * time.Minute)
	warnAt := now.Add(-1 * time.Minute)

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     true,
			SessionID:  &sessionID,
			EndsAt:     &endsAt,
			WarnAt:     &warnAt,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()

	// First poll - should show warning
	enforcer.poll(ctx)
	if platform.WarningCallCount != 1 {
		t.Errorf("Expected 1 warning call after first poll, got %d", platform.WarningCallCount)
	}

	// Second poll - should NOT show warning again
	enforcer.poll(ctx)
	if platform.WarningCallCount != 1 {
		t.Errorf("Expected still 1 warning call after second poll, got %d", platform.WarningCallCount)
	}
}

func TestWarning_NewSession_Resets(t *testing.T) {
	now := time.Now()
	sessionID1 := "session-1"
	sessionID2 := "session-2"
	endsAt := now.Add(4 * time.Minute)
	warnAt := now.Add(-1 * time.Minute)

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     true,
			SessionID:  &sessionID1,
			EndsAt:     &endsAt,
			WarnAt:     &warnAt,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()

	// First session - show warning
	enforcer.poll(ctx)
	if platform.WarningCallCount != 1 {
		t.Errorf("Expected 1 warning for first session, got %d", platform.WarningCallCount)
	}

	// New session - should reset and show warning again
	client.StatusToReturn.SessionID = &sessionID2
	enforcer.poll(ctx)
	if platform.WarningCallCount != 2 {
		t.Errorf("Expected 2 warnings after new session, got %d", platform.WarningCallCount)
	}
}

func TestSessionExpired_Locks(t *testing.T) {
	now := time.Now()
	sessionID := "session-123"
	endsAt := now.Add(-1 * time.Minute) // Already expired

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     true,
			SessionID:  &sessionID,
			EndsAt:     &endsAt,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 1 {
		t.Errorf("Expected lock when session expired, got %d", platform.LockCallCount)
	}
}

func TestNetworkError_GracePeriod(t *testing.T) {
	now := time.Now()
	lastSuccess := now.Add(-10 * time.Second) // 10 seconds ago - within 30s grace period

	client := &MockMetronClient{
		ErrorToReturn: errors.New("network error"),
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)
	// Set up state as if we had a recent successful poll
	enforcer.state.LastSuccessfulPoll = &lastSuccess

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 0 {
		t.Errorf("Expected no lock during grace period, got %d", platform.LockCallCount)
	}
}

func TestNetworkError_FailClosed(t *testing.T) {
	now := time.Now()
	errorSince := now.Add(-60 * time.Second)  // Errors for 60 seconds
	lastSuccess := now.Add(-60 * time.Second) // Last success 60 seconds ago

	client := &MockMetronClient{
		ErrorToReturn: errors.New("network error"),
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)
	enforcer.state.NetworkErrorSince = &errorSince
	enforcer.state.LastSuccessfulPoll = &lastSuccess

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 1 {
		t.Errorf("Expected lock after grace period exceeded, got %d", platform.LockCallCount)
	}
}

func TestLockDebounce(t *testing.T) {
	now := time.Now()
	lastLock := now.Add(-2 * time.Second) // 2 seconds ago, within 5s debounce

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     false,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)
	enforcer.state.LastLockTime = &lastLock

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 0 {
		t.Errorf("Expected no lock due to debounce, got %d", platform.LockCallCount)
	}
}

func TestLockDebounce_AllowsAfterPeriod(t *testing.T) {
	now := time.Now()
	lastLock := now.Add(-10 * time.Second) // 10 seconds ago, outside 5s debounce

	client := &MockMetronClient{
		StatusToReturn: &SessionStatus{
			Active:     false,
			ServerTime: now,
		},
	}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: now}

	enforcer := newTestEnforcer(client, platform, clock)
	enforcer.state.LastLockTime = &lastLock

	ctx := context.Background()
	enforcer.poll(ctx)

	if platform.LockCallCount != 1 {
		t.Errorf("Expected lock after debounce period, got %d", platform.LockCallCount)
	}
}

func TestGetState_ReturnsCopy(t *testing.T) {
	client := &MockMetronClient{}
	platform := &MockPlatform{}
	clock := &MockClock{CurrentTime: time.Now()}

	enforcer := newTestEnforcer(client, platform, clock)

	sessionID := "test-session"
	enforcer.state.LastSessionID = &sessionID

	state := enforcer.GetState()

	if state.LastSessionID == nil || *state.LastSessionID != "test-session" {
		t.Errorf("Expected state to have session ID 'test-session'")
	}
}
