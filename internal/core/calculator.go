package core

import (
	"context"
	"fmt"
	"time"
)

// TimeCalculationService provides centralized time calculation logic
// This service is the single source of truth for ALL time calculations
// It answers questions like:
// - How much time does a child have available today?
// - How much time has been consumed today?
// - How much time remains for a child today?
// - How much time has elapsed in a session?
type TimeCalculationService struct {
	storage  TimeCalculationStorage
	timezone *time.Location
}

// TimeCalculationStorage defines the storage interface needed for calculations
type TimeCalculationStorage interface {
	// Allocation queries
	GetDailyAllocation(ctx context.Context, childID string, date time.Time) (*DailyTimeAllocation, error)
	CreateDailyAllocation(ctx context.Context, allocation *DailyTimeAllocation) error

	// Usage queries
	GetDailyUsageSummary(ctx context.Context, childID string, date time.Time) (*DailyUsageSummary, error)
	ListActiveSessionRecords(ctx context.Context) ([]*SessionUsageRecord, error)

	// Child queries
	GetChild(ctx context.Context, id string) (*Child, error)
}

// AvailableTimeResult contains calculated available time
type AvailableTimeResult struct {
	BaseLimit      int // From schedule (weekday/weekend)
	BonusGranted   int // Rewards granted
	TotalAvailable int // base + bonus
}

// ConsumedTimeResult contains calculated consumed time
type ConsumedTimeResult struct {
	FromCompletedSessions int // Minutes from daily_usage_summaries
	FromActiveSessions    int // Minutes from active sessions (elapsed)
	TotalConsumed         int // completed + active
}

// RemainingTimeResult contains calculated remaining time
type RemainingTimeResult struct {
	Available      AvailableTimeResult
	Consumed       ConsumedTimeResult
	RemainingTotal int // available.TotalAvailable - consumed.TotalConsumed
}

// NewTimeCalculationService creates a new time calculation service
func NewTimeCalculationService(storage TimeCalculationStorage, timezone *time.Location) *TimeCalculationService {
	if timezone == nil {
		timezone = time.UTC
	}
	return &TimeCalculationService{
		storage:  storage,
		timezone: timezone,
	}
}

// GetAvailableTime calculates total time allocated for a child today
func (s *TimeCalculationService) GetAvailableTime(ctx context.Context, childID string, date time.Time) (*AvailableTimeResult, error) {
	normalizedDate := s.normalizeDate(date)

	// Get or create allocation
	allocation, err := s.getOrCreateAllocation(ctx, childID, normalizedDate)
	if err != nil {
		return nil, err
	}

	return &AvailableTimeResult{
		BaseLimit:      allocation.BaseLimit,
		BonusGranted:   allocation.BonusGranted,
		TotalAvailable: allocation.BaseLimit + allocation.BonusGranted,
	}, nil
}

// GetConsumedTime calculates total time consumed by a child today
func (s *TimeCalculationService) GetConsumedTime(ctx context.Context, childID string, date time.Time) (*ConsumedTimeResult, error) {
	normalizedDate := s.normalizeDate(date)

	// Get completed session usage
	summary, err := s.storage.GetDailyUsageSummary(ctx, childID, normalizedDate)
	if err != nil {
		// If summary doesn't exist, that's okay - means no completed sessions yet
		summary = &DailyUsageSummary{
			ChildID:      childID,
			Date:         normalizedDate,
			MinutesUsed:  0,
			SessionCount: 0,
		}
	}

	// Calculate active session usage
	activeSessions, err := s.storage.ListActiveSessionRecords(ctx)
	if err != nil {
		return nil, err
	}

	activeMinutes := 0
	for _, session := range activeSessions {
		// Check if this session includes the child
		for _, sid := range session.ChildIDs {
			if sid == childID {
				elapsed := s.GetSessionElapsed(session)
				activeMinutes += elapsed
				break
			}
		}
	}

	return &ConsumedTimeResult{
		FromCompletedSessions: summary.MinutesUsed,
		FromActiveSessions:    activeMinutes,
		TotalConsumed:         summary.MinutesUsed + activeMinutes,
	}, nil
}

// GetRemainingTime calculates remaining time for a child today
func (s *TimeCalculationService) GetRemainingTime(ctx context.Context, childID string, date time.Time) (*RemainingTimeResult, error) {
	available, err := s.GetAvailableTime(ctx, childID, date)
	if err != nil {
		return nil, err
	}

	consumed, err := s.GetConsumedTime(ctx, childID, date)
	if err != nil {
		return nil, err
	}

	totalRemaining := available.TotalAvailable - consumed.TotalConsumed
	if totalRemaining < 0 {
		totalRemaining = 0
	}

	return &RemainingTimeResult{
		Available:      *available,
		Consumed:       *consumed,
		RemainingTotal: totalRemaining,
	}, nil
}

// GetRemainingTimeForExtension calculates remaining time for extending a specific session
// This differs from GetRemainingTime by using the current session's ExpectedDuration
// instead of elapsed time, preventing the rapid-fire extension exploit
//
// For the session being extended (currentSessionID), we use ExpectedDuration because
// the child has already "committed" to that time even if it hasn't elapsed yet.
// This prevents the exploit where children spam extend immediately after starting.
func (s *TimeCalculationService) GetRemainingTimeForExtension(
	ctx context.Context,
	childID string,
	date time.Time,
	currentSessionID string,
) (*RemainingTimeResult, error) {
	normalizedDate := s.normalizeDate(date)

	available, err := s.GetAvailableTime(ctx, childID, normalizedDate)
	if err != nil {
		return nil, err
	}

	// Get completed session usage
	summary, err := s.storage.GetDailyUsageSummary(ctx, childID, normalizedDate)
	if err != nil {
		summary = &DailyUsageSummary{
			ChildID:      childID,
			Date:         normalizedDate,
			MinutesUsed:  0,
			SessionCount: 0,
		}
	}

	// Calculate active session usage
	// KEY DIFFERENCE: For the current session, use ExpectedDuration instead of elapsed
	activeSessions, err := s.storage.ListActiveSessionRecords(ctx)
	if err != nil {
		return nil, err
	}

	activeMinutes := 0
	for _, session := range activeSessions {
		// Check if this session includes the child
		for _, sid := range session.ChildIDs {
			if sid == childID {
				// For the session being extended, use ExpectedDuration (committed time)
				// For other sessions, use elapsed time
				if session.ID == currentSessionID {
					activeMinutes += session.ExpectedDuration
				} else {
					elapsed := s.GetSessionElapsed(session)
					activeMinutes += elapsed
				}
				break
			}
		}
	}

	consumed := ConsumedTimeResult{
		FromCompletedSessions: summary.MinutesUsed,
		FromActiveSessions:    activeMinutes,
		TotalConsumed:         summary.MinutesUsed + activeMinutes,
	}

	totalRemaining := available.TotalAvailable - consumed.TotalConsumed
	if totalRemaining < 0 {
		totalRemaining = 0
	}

	return &RemainingTimeResult{
		Available:      *available,
		Consumed:       consumed,
		RemainingTotal: totalRemaining,
	}, nil
}

// GetSessionElapsed calculates elapsed time for a session
func (s *TimeCalculationService) GetSessionElapsed(session *SessionUsageRecord) int {
	if session.Status != SessionStatusActive {
		// For completed/expired sessions, use actual duration if set
		if session.ActualDuration != nil {
			return *session.ActualDuration
		}
		// Fallback to expected duration
		return session.ExpectedDuration
	}

	// For active sessions, calculate elapsed time
	elapsed := int(time.Since(session.StartTime).Minutes())

	// Clamp to expected duration (don't count overtime)
	if elapsed > session.ExpectedDuration {
		elapsed = session.ExpectedDuration
	}

	return elapsed
}

// GetSessionRemaining calculates remaining time for a session
func (s *TimeCalculationService) GetSessionRemaining(session *SessionUsageRecord) int {
	if session.Status != SessionStatusActive {
		return 0
	}

	endTime := session.StartTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)
	remaining := int(time.Until(endTime).Minutes())

	if remaining < 0 {
		return 0
	}

	return remaining
}

// GetSessionEndTime calculates when a session will end
func (s *TimeCalculationService) GetSessionEndTime(session *SessionUsageRecord) time.Time {
	return session.StartTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)
}

// getOrCreateAllocation gets existing or creates new allocation for a day
func (s *TimeCalculationService) getOrCreateAllocation(ctx context.Context, childID string, date time.Time) (*DailyTimeAllocation, error) {
	allocation, err := s.storage.GetDailyAllocation(ctx, childID, date)
	if err == nil {
		return allocation, nil
	}

	// Check if error is "not found" - if it's another error, return it
	if err != ErrAllocationNotFound {
		return nil, err
	}

	// Doesn't exist, create it
	child, err := s.storage.GetChild(ctx, childID)
	if err != nil {
		return nil, err
	}

	baseLimit := child.GetDailyLimit(date)

	allocation = &DailyTimeAllocation{
		ChildID:      childID,
		Date:         date,
		BaseLimit:    baseLimit,
		BonusGranted: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Store it
	if err := s.storage.CreateDailyAllocation(ctx, allocation); err != nil {
		return nil, fmt.Errorf("failed to create daily allocation: %w", err)
	}

	return allocation, nil
}

// normalizeDate normalizes a date to start of day in the configured timezone
func (s *TimeCalculationService) normalizeDate(t time.Time) time.Time {
	inTZ := t.In(s.timezone)
	year, month, day := inTZ.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, s.timezone)
}

// Additional error for allocation not found
var ErrAllocationNotFound = fmt.Errorf("allocation not found")
