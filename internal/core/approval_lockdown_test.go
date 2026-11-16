package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allDays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

// mockApprovalStorage implements ApprovalGateStorage for tests
type mockApprovalStorage struct {
	approvals map[string]*DailyApproval // key: childID|YYYY-MM-DD
	err       error
}

func newMockApprovalStorage() *mockApprovalStorage {
	return &mockApprovalStorage{approvals: map[string]*DailyApproval{}}
}

func approvalKey(childID string, date time.Time) string {
	return childID + "|" + date.Format("2006-01-02")
}

func (m *mockApprovalStorage) GetDailyApproval(ctx context.Context, childID string, date time.Time) (*DailyApproval, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.approvals[approvalKey(childID, date)], nil
}

func (m *mockApprovalStorage) SaveDailyApproval(ctx context.Context, a *DailyApproval) error {
	m.approvals[approvalKey(a.ChildID, a.Date)] = a
	return nil
}

func (m *mockApprovalStorage) DeleteDailyApproval(ctx context.Context, childID string, date time.Time) error {
	delete(m.approvals, approvalKey(childID, date))
	return nil
}

// --- LockdownRule ---

func TestLockdownRule_Matches(t *testing.T) {
	tests := []struct {
		name     string
		rule     LockdownRule
		deviceID string
		childIDs []string
		want     bool
	}{
		{"all/all matches anything", LockdownRule{ChildID: "*", DeviceID: "*"}, "tv1", []string{"a"}, true},
		{"child/all matches child on any device", LockdownRule{ChildID: "a", DeviceID: "*"}, "ps5", []string{"a"}, true},
		{"child/all ignores other child", LockdownRule{ChildID: "a", DeviceID: "*"}, "ps5", []string{"b"}, false},
		{"all/device matches any child on device", LockdownRule{ChildID: "*", DeviceID: "ps5"}, "ps5", []string{"b"}, true},
		{"all/device ignores other device", LockdownRule{ChildID: "*", DeviceID: "ps5"}, "tv1", []string{"b"}, false},
		{"child/device exact", LockdownRule{ChildID: "a", DeviceID: "ps5"}, "ps5", []string{"a"}, true},
		{"child/device other device", LockdownRule{ChildID: "a", DeviceID: "ps5"}, "tv1", []string{"a"}, false},
		{"child/device shared session with blocked child", LockdownRule{ChildID: "a", DeviceID: "ps5"}, "ps5", []string{"b", "a"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.rule.Matches(tt.deviceID, tt.childIDs))
		})
	}
}

func TestLockdownExpiry_EndOfDay(t *testing.T) {
	riga, err := time.LoadLocation("Europe/Riga")
	require.NoError(t, err)

	// 23:30 Riga time on Nov 16 (21:30 UTC)
	now := time.Date(2026, 11, 16, 23, 30, 0, 0, riga)
	exp, err := LockdownExpiry(LockdownUntilEndOfDay, now, riga)
	require.NoError(t, err)
	require.NotNil(t, exp)
	assert.True(t, exp.Equal(time.Date(2026, 11, 17, 0, 0, 0, 0, riga)))

	rule := &LockdownRule{ChildID: "a", DeviceID: "*", ExpiresAt: exp}
	assert.True(t, rule.IsActive(now))
	assert.True(t, rule.IsActive(time.Date(2026, 11, 16, 23, 59, 59, 0, riga)))
	assert.False(t, rule.IsActive(time.Date(2026, 11, 17, 0, 0, 0, 0, riga)))

	manual, err := LockdownExpiry(LockdownUntilManual, now, riga)
	require.NoError(t, err)
	assert.Nil(t, manual)

	_, err = LockdownExpiry("tomorrow", now, riga)
	assert.Error(t, err)
}

func TestSessionManager_StartSession_ScopedLockdown(t *testing.T) {
	manager, storage := setupLockdownTestManager(t)
	require.NoError(t, storage.CreateChild(context.Background(), &Child{ID: "child2", Name: "Bob", WeekdayLimit: 60, WeekendLimit: 60}))
	manager.deviceRegistry.(*mockDeviceRegistry).addDevice(&mockDevice{id: "ps5", name: "PS5", dtype: "game", driver: "aqara"})

	lockdown := &mockLockdownStorage{rules: []*LockdownRule{{ID: 7, ChildID: "child1", DeviceID: "ps5"}}}
	manager.SetLockdownStorage(lockdown)

	_, err := manager.StartSession(context.Background(), "ps5", []string{"child1"}, 30)
	assert.ErrorIs(t, err, ErrLockdownActive)

	// Shared session including the punished child is blocked too
	_, err = manager.StartSession(context.Background(), "ps5", []string{"child2", "child1"}, 30)
	assert.ErrorIs(t, err, ErrLockdownActive)

	// Other child on the same device is fine
	_, err = manager.StartSession(context.Background(), "ps5", []string{"child2"}, 30)
	assert.NoError(t, err)

	// Same child on another device is fine
	_, err = manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	assert.NoError(t, err)
}

func TestSessionManager_StartSession_ExpiredLockdownIgnored(t *testing.T) {
	manager, _ := setupLockdownTestManager(t)
	past := time.Now().Add(-time.Minute)
	manager.SetLockdownStorage(&mockLockdownStorage{rules: []*LockdownRule{{ID: 1, ChildID: "*", DeviceID: "*", ExpiresAt: &past}}})

	_, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	assert.NoError(t, err)
}

func TestSessionManager_AddChildren_BlockedByLockdown(t *testing.T) {
	manager, storage := setupLockdownTestManager(t)
	require.NoError(t, storage.CreateChild(context.Background(), &Child{ID: "child2", Name: "Bob", WeekdayLimit: 60, WeekendLimit: 60}))
	lockdown := &mockLockdownStorage{}
	manager.SetLockdownStorage(lockdown)

	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 30)
	require.NoError(t, err)

	lockdown.rules = []*LockdownRule{{ID: 3, ChildID: "child2", DeviceID: "tv1"}}
	_, err = manager.AddChildrenToSession(context.Background(), session.ID, []string{"child2"})
	assert.ErrorIs(t, err, ErrLockdownActive)
}

// --- Approval gate ---

func TestApprovalGateService_RequiredDays(t *testing.T) {
	riga, _ := time.LoadLocation("Europe/Riga")
	gate := NewApprovalGateService([]string{"monday", "tuesday", "wednesday", "thursday", "friday"}, newMockApprovalStorage(), riga)

	monday := time.Date(2026, 11, 16, 15, 0, 0, 0, riga)
	saturday := time.Date(2026, 11, 21, 15, 0, 0, 0, riga)
	assert.True(t, gate.IsApprovalRequired(monday))
	assert.False(t, gate.IsApprovalRequired(saturday))

	// Sunday 23:30 UTC is already Monday 01:30 in Riga
	sundayUTC := time.Date(2026, 11, 15, 23, 30, 0, 0, time.UTC)
	assert.True(t, gate.IsApprovalRequired(sundayUTC))
}

func TestApprovalGateService_ApproveRevokeAndDayRollover(t *testing.T) {
	riga, _ := time.LoadLocation("Europe/Riga")
	ctx := context.Background()
	gate := NewApprovalGateService([]string{"monday", "tuesday"}, newMockApprovalStorage(), riga)
	monday := time.Date(2026, 11, 16, 15, 0, 0, 0, riga)

	status, err := gate.GetApprovalStatus(ctx, "child1", monday)
	require.NoError(t, err)
	assert.True(t, status.Required)
	assert.False(t, status.Approved)

	_, err = gate.Approve(ctx, "child1", monday, "telegram")
	require.NoError(t, err)

	status, err = gate.GetApprovalStatus(ctx, "child1", monday.Add(5*time.Hour))
	require.NoError(t, err)
	assert.True(t, status.Approved)
	require.NotNil(t, status.ApprovedAt)

	// Next day the approval does not carry over
	status, err = gate.GetApprovalStatus(ctx, "child1", monday.Add(24*time.Hour))
	require.NoError(t, err)
	assert.False(t, status.Approved)

	// Other child is not affected
	status, err = gate.GetApprovalStatus(ctx, "child2", monday)
	require.NoError(t, err)
	assert.False(t, status.Approved)

	require.NoError(t, gate.Revoke(ctx, "child1", monday))
	status, err = gate.GetApprovalStatus(ctx, "child1", monday)
	require.NoError(t, err)
	assert.False(t, status.Approved)
}

func setupGateManager(t *testing.T, days []string) (*SessionManager, *ApprovalGateService) {
	t.Helper()
	manager, _ := setupLockdownTestManager(t)
	gate := NewApprovalGateService(days, newMockApprovalStorage(), time.UTC)
	manager.SetApprovalGate(gate)
	return manager, gate
}

func TestSessionManager_StartSession_ApprovalGate(t *testing.T) {
	manager, gate := setupGateManager(t, allDays)
	childCtx := WithChildInitiated(context.Background())

	_, err := manager.StartSession(childCtx, "tv1", []string{"child1"}, 10)
	assert.ErrorIs(t, err, ErrApprovalRequired)

	// Parent path (no child marker) is not gated
	_, err = manager.StartSession(context.Background(), "tv1", []string{"child1"}, 10)
	assert.NoError(t, err)

	_, err = gate.Approve(context.Background(), "child1", time.Now(), "test")
	require.NoError(t, err)
	_, err = manager.StartSession(childCtx, "tv1", []string{"child1"}, 10)
	assert.NoError(t, err)
}

func TestSessionManager_StartSession_ApprovalNotRequiredOnFreeDays(t *testing.T) {
	manager, _ := setupGateManager(t, nil)
	_, err := manager.StartSession(WithChildInitiated(context.Background()), "tv1", []string{"child1"}, 10)
	assert.NoError(t, err)
}

func TestSessionManager_StartSession_ApprovalStorageErrorFailsClosed(t *testing.T) {
	manager, _ := setupLockdownTestManager(t)
	storage := newMockApprovalStorage()
	storage.err = errors.New("db error")
	manager.SetApprovalGate(NewApprovalGateService(allDays, storage, time.UTC))

	_, err := manager.StartSession(WithChildInitiated(context.Background()), "tv1", []string{"child1"}, 10)
	assert.Error(t, err)
}

// Downtime has priority over the approval gate: an approved child still cannot play during downtime
func TestSessionManager_StartSession_DowntimeBeatsApproval(t *testing.T) {
	manager, gate := setupGateManager(t, allDays)
	always := &DaySchedule{StartHour: 0, StartMinute: 0, EndHour: 23, EndMinute: 59}
	manager.downtime = NewDowntimeService(&DowntimeSchedule{Weekday: always, Weekend: always}, time.UTC)
	child, err := manager.storage.GetChild(context.Background(), "child1")
	require.NoError(t, err)
	child.DowntimeEnabled = true
	require.NoError(t, manager.storage.UpdateChild(context.Background(), child))

	if !manager.downtime.IsChildInDowntime(child, time.Now()) {
		t.Skip("outside the test downtime window (23:59 UTC)")
	}

	// Not approved: downtime is reported first
	_, err = manager.StartSession(WithChildInitiated(context.Background()), "tv1", []string{"child1"}, 10)
	assert.ErrorIs(t, err, ErrDowntimeActive)

	// Approved: still blocked by downtime
	_, err = gate.Approve(context.Background(), "child1", time.Now(), "test")
	require.NoError(t, err)
	_, err = manager.StartSession(WithChildInitiated(context.Background()), "tv1", []string{"child1"}, 10)
	assert.ErrorIs(t, err, ErrDowntimeActive)
}

func TestSessionManager_ExtendSession_ApprovalGate(t *testing.T) {
	manager, gate := setupGateManager(t, allDays)

	// Parent starts the session without approval
	session, err := manager.StartSession(context.Background(), "tv1", []string{"child1"}, 10)
	require.NoError(t, err)

	_, err = manager.ExtendSession(WithChildInitiated(context.Background()), session.ID, 10)
	assert.ErrorIs(t, err, ErrApprovalRequired)

	_, err = gate.Approve(context.Background(), "child1", time.Now(), "test")
	require.NoError(t, err)
	_, err = manager.ExtendSession(WithChildInitiated(context.Background()), session.ID, 10)
	assert.NoError(t, err)
}
