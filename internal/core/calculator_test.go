package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage for TimeCalculationService tests
type mockTimeCalcStorage struct {
	allocations map[string]*DailyTimeAllocation // key: "childID-date"
	summaries   map[string]*DailyUsageSummary   // key: "childID-date"
	sessions    []*SessionUsageRecord
	children    map[string]*Child
}

func newMockTimeCalcStorage() *mockTimeCalcStorage {
	return &mockTimeCalcStorage{
		allocations: make(map[string]*DailyTimeAllocation),
		summaries:   make(map[string]*DailyUsageSummary),
		sessions:    make([]*SessionUsageRecord, 0),
		children:    make(map[string]*Child),
	}
}

func (m *mockTimeCalcStorage) GetDailyAllocation(ctx context.Context, childID string, date time.Time) (*DailyTimeAllocation, error) {
	key := childID + "-" + date.Format("2006-01-02")
	alloc, ok := m.allocations[key]
	if !ok {
		return nil, ErrAllocationNotFound
	}
	return alloc, nil
}

func (m *mockTimeCalcStorage) CreateDailyAllocation(ctx context.Context, allocation *DailyTimeAllocation) error {
	key := allocation.ChildID + "-" + allocation.Date.Format("2006-01-02")
	m.allocations[key] = allocation
	return nil
}

func (m *mockTimeCalcStorage) GetDailyUsageSummary(ctx context.Context, childID string, date time.Time) (*DailyUsageSummary, error) {
	key := childID + "-" + date.Format("2006-01-02")
	summary, ok := m.summaries[key]
	if !ok {
		return nil, ErrAllocationNotFound // Using same error for simplicity
	}
	return summary, nil
}

func (m *mockTimeCalcStorage) ListActiveSessionRecords(ctx context.Context) ([]*SessionUsageRecord, error) {
	return m.sessions, nil
}

func (m *mockTimeCalcStorage) GetChild(ctx context.Context, id string) (*Child, error) {
	child, ok := m.children[id]
	if !ok {
		return nil, ErrChildNotFound
	}
	return child, nil
}

// Test helpers

func makeDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func makeWeekday() time.Time {
	// Get a Monday (weekday)
	now := time.Now()
	for now.Weekday() != time.Monday {
		now = now.AddDate(0, 0, 1)
	}
	return makeDate(now.Year(), int(now.Month()), now.Day())
}

func makeWeekend() time.Time {
	// Get a Saturday (weekend)
	now := time.Now()
	for now.Weekday() != time.Saturday {
		now = now.AddDate(0, 0, 1)
	}
	return makeDate(now.Year(), int(now.Month()), now.Day())
}

// Tests

func TestTimeCalculationService_GetAvailableTime_NoAllocation(t *testing.T) {
	storage := newMockTimeCalcStorage()
	storage.children["child1"] = &Child{
		ID:           "child1",
		Name:         "Test Child",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}

	service := NewTimeCalculationService(storage, time.UTC)
	date := makeWeekday()

	result, err := service.GetAvailableTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 60, result.BaseLimit, "Should use weekday limit")
	assert.Equal(t, 0, result.BonusGranted)
	assert.Equal(t, 60, result.TotalAvailable)

	// Verify allocation was created
	key := "child1-" + date.Format("2006-01-02")
	_, exists := storage.allocations[key]
	assert.True(t, exists, "Allocation should be created on first access")
}

func TestTimeCalculationService_GetAvailableTime_WithBonus(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 30,
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetAvailableTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 60, result.BaseLimit)
	assert.Equal(t, 30, result.BonusGranted)
	assert.Equal(t, 90, result.TotalAvailable, "60 base + 30 bonus = 90")
}

func TestTimeCalculationService_GetAvailableTime_WeekendVsWeekday(t *testing.T) {
	storage := newMockTimeCalcStorage()
	storage.children["child1"] = &Child{
		ID:           "child1",
		Name:         "Test Child",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}

	service := NewTimeCalculationService(storage, time.UTC)

	// Test weekday
	weekday := makeWeekday()
	resultWeekday, err := service.GetAvailableTime(context.Background(), "child1", weekday)
	require.NoError(t, err)
	assert.Equal(t, 60, resultWeekday.BaseLimit, "Should use weekday limit")

	// Test weekend
	weekend := makeWeekend()
	resultWeekend, err := service.GetAvailableTime(context.Background(), "child1", weekend)
	require.NoError(t, err)
	assert.Equal(t, 120, resultWeekend.BaseLimit, "Should use weekend limit")
}

func TestTimeCalculationService_GetConsumedTime_NoUsage(t *testing.T) {
	storage := newMockTimeCalcStorage()
	service := NewTimeCalculationService(storage, time.UTC)
	date := makeWeekday()

	result, err := service.GetConsumedTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 0, result.FromCompletedSessions)
	assert.Equal(t, 0, result.FromActiveSessions)
	assert.Equal(t, 0, result.TotalConsumed)
}

func TestTimeCalculationService_GetConsumedTime_CompletedOnly(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:      "child1",
		Date:         date,
		MinutesUsed:  45,
		SessionCount: 2,
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetConsumedTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 45, result.FromCompletedSessions)
	assert.Equal(t, 0, result.FromActiveSessions)
	assert.Equal(t, 45, result.TotalConsumed)
}

func TestTimeCalculationService_GetConsumedTime_WithActiveSession(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()

	// Create an active session that started 20 minutes ago
	startTime := time.Now().Add(-20 * time.Minute)
	storage.sessions = []*SessionUsageRecord{
		{
			ID:               "session1",
			ChildIDs:         []string{"child1"},
			StartTime:        startTime,
			ExpectedDuration: 30,
			Status:           SessionStatusActive,
		},
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetConsumedTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 0, result.FromCompletedSessions)
	assert.Equal(t, 20, result.FromActiveSessions, "Should count 20 elapsed minutes")
	assert.Equal(t, 20, result.TotalConsumed)
}

func TestTimeCalculationService_GetConsumedTime_CompletedPlusActive(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:      "child1",
		Date:         date,
		MinutesUsed:  30,
		SessionCount: 1,
	}

	// Active session started 15 minutes ago
	startTime := time.Now().Add(-15 * time.Minute)
	storage.sessions = []*SessionUsageRecord{
		{
			ID:               "session1",
			ChildIDs:         []string{"child1"},
			StartTime:        startTime,
			ExpectedDuration: 20,
			Status:           SessionStatusActive,
		},
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetConsumedTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 30, result.FromCompletedSessions)
	assert.Equal(t, 15, result.FromActiveSessions)
	assert.Equal(t, 45, result.TotalConsumed, "30 completed + 15 active = 45")
}

func TestTimeCalculationService_GetConsumedTime_MultipleActiveSessionsSameChild(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()

	// Two active sessions for same child
	startTime1 := time.Now().Add(-10 * time.Minute)
	startTime2 := time.Now().Add(-5 * time.Minute)
	storage.sessions = []*SessionUsageRecord{
		{
			ID:               "session1",
			ChildIDs:         []string{"child1"},
			StartTime:        startTime1,
			ExpectedDuration: 20,
			Status:           SessionStatusActive,
		},
		{
			ID:               "session2",
			ChildIDs:         []string{"child1"},
			StartTime:        startTime2,
			ExpectedDuration: 15,
			Status:           SessionStatusActive,
		},
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetConsumedTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 15, result.FromActiveSessions, "10 + 5 = 15 total active minutes")
}

func TestTimeCalculationService_GetRemainingTime_AllBase(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 0,
	}

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:     "child1",
		Date:        date,
		MinutesUsed: 20,
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetRemainingTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 60, result.Available.TotalAvailable)
	assert.Equal(t, 20, result.Consumed.TotalConsumed)
	assert.Equal(t, 40, result.RemainingTotal, "60 - 20 = 40")
}

func TestTimeCalculationService_GetRemainingTime_WithBonus(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 30,
	}

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:     "child1",
		Date:        date,
		MinutesUsed: 20,
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetRemainingTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 90, result.Available.TotalAvailable, "60 base + 30 bonus")
	assert.Equal(t, 20, result.Consumed.TotalConsumed)
	assert.Equal(t, 70, result.RemainingTotal, "90 - 20 = 70")
}

func TestTimeCalculationService_GetRemainingTime_BaseExhausted(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 30,
	}

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:     "child1",
		Date:        date,
		MinutesUsed: 70, // Consumed 70 minutes
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetRemainingTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 90, result.Available.TotalAvailable, "60 base + 30 bonus")
	assert.Equal(t, 70, result.Consumed.TotalConsumed)
	assert.Equal(t, 20, result.RemainingTotal, "90 - 70 = 20")
}

func TestTimeCalculationService_GetRemainingTime_AllExhausted(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 20,
	}

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:     "child1",
		Date:        date,
		MinutesUsed: 80, // All used (60 base + 20 bonus)
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetRemainingTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 0, result.RemainingTotal, "Nothing left")
}

func TestTimeCalculationService_GetRemainingTime_Overconsumption(t *testing.T) {
	storage := newMockTimeCalcStorage()
	date := makeWeekday()
	key := "child1-" + date.Format("2006-01-02")

	storage.allocations[key] = &DailyTimeAllocation{
		ChildID:      "child1",
		Date:         date,
		BaseLimit:    60,
		BonusGranted: 0,
	}

	storage.summaries[key] = &DailyUsageSummary{
		ChildID:     "child1",
		Date:        date,
		MinutesUsed: 70, // Over limit (can happen due to monitoring delay)
	}

	service := NewTimeCalculationService(storage, time.UTC)

	result, err := service.GetRemainingTime(context.Background(), "child1", date)
	require.NoError(t, err)
	assert.Equal(t, 0, result.RemainingTotal, "Should cap at 0, not negative")
}

func TestTimeCalculationService_GetSessionElapsed_Active(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	startTime := time.Now().Add(-15 * time.Minute)
	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        startTime,
		ExpectedDuration: 30,
		Status:           SessionStatusActive,
	}

	elapsed := service.GetSessionElapsed(session)
	assert.Equal(t, 15, elapsed, "Should be 15 minutes elapsed")
}

func TestTimeCalculationService_GetSessionElapsed_ActiveOvertime(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	// Session started 40 minutes ago but expected duration is 30
	startTime := time.Now().Add(-40 * time.Minute)
	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        startTime,
		ExpectedDuration: 30,
		Status:           SessionStatusActive,
	}

	elapsed := service.GetSessionElapsed(session)
	assert.Equal(t, 30, elapsed, "Should clamp to expected duration (30), not count overtime")
}

func TestTimeCalculationService_GetSessionElapsed_Completed(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	actualDuration := 25
	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        time.Now().Add(-30 * time.Minute),
		ExpectedDuration: 30,
		ActualDuration:   &actualDuration,
		Status:           SessionStatusCompleted,
	}

	elapsed := service.GetSessionElapsed(session)
	assert.Equal(t, 25, elapsed, "Should use actual duration for completed sessions")
}

func TestTimeCalculationService_GetSessionRemaining_Active(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	startTime := time.Now().Add(-10 * time.Minute)
	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        startTime,
		ExpectedDuration: 30,
		Status:           SessionStatusActive,
	}

	remaining := service.GetSessionRemaining(session)
	// Allow for test execution time - should be approximately 20 minutes
	assert.GreaterOrEqual(t, remaining, 19, "Should have at least 19 minutes remaining")
	assert.LessOrEqual(t, remaining, 20, "Should have at most 20 minutes remaining")
}

func TestTimeCalculationService_GetSessionRemaining_Expired(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	startTime := time.Now().Add(-40 * time.Minute)
	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        startTime,
		ExpectedDuration: 30,
		Status:           SessionStatusActive,
	}

	remaining := service.GetSessionRemaining(session)
	assert.Equal(t, 0, remaining, "Session expired, should return 0")
}

func TestTimeCalculationService_GetSessionRemaining_NotActive(t *testing.T) {
	service := NewTimeCalculationService(newMockTimeCalcStorage(), time.UTC)

	session := &SessionUsageRecord{
		ID:               "session1",
		StartTime:        time.Now().Add(-10 * time.Minute),
		ExpectedDuration: 30,
		Status:           SessionStatusCompleted,
	}

	remaining := service.GetSessionRemaining(session)
	assert.Equal(t, 0, remaining, "Completed sessions have no remaining time")
}

func TestTimeCalculationService_TimezoneHandling(t *testing.T) {
	storage := newMockTimeCalcStorage()
	storage.children["child1"] = &Child{
		ID:           "child1",
		Name:         "Test Child",
		WeekdayLimit: 60,
		WeekendLimit: 120,
	}

	// Use a different timezone
	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	service := NewTimeCalculationService(storage, location)

	// Pass a time in UTC
	utcTime := time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC)
	result, err := service.GetAvailableTime(context.Background(), "child1", utcTime)
	require.NoError(t, err)

	// The date should be normalized to the timezone's date
	// Verify allocation was created with normalized date
	key := "child1-" + utcTime.In(location).Format("2006-01-02")
	_, exists := storage.allocations[key]
	assert.True(t, exists, "Should normalize date to configured timezone")

	assert.NotNil(t, result)
}
