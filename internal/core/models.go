package core

import (
	"errors"
	"time"
)

// SessionStatus represents the current state of a session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusExpired   SessionStatus = "expired"
)

// Child represents a child with screen-time limits
type Child struct {
	ID              string
	Name            string
	PIN             string // 4-digit PIN for child authentication (hashed with bcrypt)
	WeekdayLimit    int    // minutes per weekday
	WeekendLimit    int    // minutes per weekend day
	BreakRule       *BreakRule
	DowntimeEnabled bool // whether downtime schedule is enforced for this child
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BreakRule defines mandatory break periods
type BreakRule struct {
	BreakAfterMinutes    int // require break after this many minutes
	BreakDurationMinutes int // break must last this many minutes
}

// Session represents an active or completed screen-time session
type Session struct {
	ID               string
	DeviceType       string // "tv", "ps5", "ipad", etc.
	DeviceID         string // specific device identifier
	ChildIDs         []string
	StartTime        time.Time
	ExpectedDuration int // minutes
	Status           SessionStatus
	LastBreakAt      *time.Time
	BreakEndsAt      *time.Time
	WarningSentAt    *time.Time // tracks when time-remaining warning was sent
	LastExtendedAt   *time.Time // tracks when session was last extended (for rate limiting)
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DailyUsage tracks a child's usage for a specific day
type DailyUsage struct {
	ChildID               string
	Date                  time.Time // normalized to start of day
	MinutesUsed           int       // regular minutes consumed
	RewardMinutesGranted  int       // bonus minutes granted for today
	SessionCount          int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Validation errors
var (
	ErrInvalidChildID      = errors.New("invalid child ID")
	ErrInvalidName         = errors.New("child name cannot be empty")
	ErrInvalidWeekdayLimit = errors.New("weekday limit must be positive")
	ErrInvalidWeekendLimit = errors.New("weekend limit must be positive")
	ErrInvalidBreakRule    = errors.New("invalid break rule configuration")
	ErrInvalidDuration     = errors.New("duration must be positive")
	ErrInvalidDeviceType   = errors.New("device type cannot be empty")
	ErrNoChildren          = errors.New("session must have at least one child")
	ErrInsufficientTime    = errors.New("child has insufficient remaining time")
	ErrSessionNotActive    = errors.New("session is not active")
	ErrSessionNotFound     = errors.New("session not found")
	ErrChildNotFound       = errors.New("child not found")
	ErrExtensionTooSoon    = errors.New("extension request too soon after previous extension")
	ErrDowntimeActive      = errors.New("session cannot be started during downtime period")
)

// Validate validates a Child
func (c *Child) Validate() error {
	if c.Name == "" {
		return ErrInvalidName
	}
	if c.WeekdayLimit <= 0 {
		return ErrInvalidWeekdayLimit
	}
	if c.WeekendLimit <= 0 {
		return ErrInvalidWeekendLimit
	}
	if c.BreakRule != nil {
		if c.BreakRule.BreakAfterMinutes <= 0 || c.BreakRule.BreakDurationMinutes <= 0 {
			return ErrInvalidBreakRule
		}
	}
	return nil
}

// GetDailyLimit returns the appropriate daily limit based on the day of week
func (c *Child) GetDailyLimit(date time.Time) int {
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return c.WeekendLimit
	}
	return c.WeekdayLimit
}

// Validate validates a Session
func (s *Session) Validate() error {
	if s.DeviceType == "" {
		return ErrInvalidDeviceType
	}
	if len(s.ChildIDs) == 0 {
		return ErrNoChildren
	}
	if s.ExpectedDuration <= 0 {
		return ErrInvalidDuration
	}
	return nil
}

// IsActive returns true if the session is currently active
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusActive
}

// IsInBreak returns true if the session is currently in a mandatory break
func (s *Session) IsInBreak() bool {
	if s.BreakEndsAt == nil {
		return false
	}
	return time.Now().Before(*s.BreakEndsAt)
}

// NeedsBreak checks if a break is needed based on the break rule and last break time
func (s *Session) NeedsBreak(breakRule *BreakRule) bool {
	if breakRule == nil {
		return false
	}

	// Calculate time since start or last break
	var timeSince time.Time
	if s.LastBreakAt != nil {
		timeSince = *s.LastBreakAt
	} else {
		timeSince = s.StartTime
	}

	minutesSince := int(time.Since(timeSince).Minutes())
	return minutesSince >= breakRule.BreakAfterMinutes
}

// CalculateRemainingMinutes calculates remaining time dynamically
// This is the authoritative calculation based on StartTime + ExpectedDuration
func (s *Session) CalculateRemainingMinutes() int {
	if s.Status != SessionStatusActive {
		return 0
	}

	endTime := s.StartTime.Add(time.Duration(s.ExpectedDuration) * time.Minute)
	remaining := int(time.Until(endTime).Minutes())

	if remaining < 0 {
		return 0
	}

	return remaining
}

// ============================================================================
// NEW MODELS - Refactored Architecture
// ============================================================================

// DailyTimeAllocation represents time allocated to a child for a specific day
// This model answers: "What time budget does this child have TODAY?"
// Responsibilities:
// - Stores base limit (from child's schedule)
// - Stores bonus allocation (rewards)
// - NO calculation logic - pure data storage
// Note: Bonus consumption is calculated from sessions, not stored separately
type DailyTimeAllocation struct {
	ChildID      string    // Foreign key to children table
	Date         time.Time // Normalized to start of day in timezone
	BaseLimit    int       // Weekday/weekend limit for this day
	BonusGranted int       // Total bonus minutes granted for this day
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SessionUsageRecord represents a screen-time usage session
// This model answers: "What happened in this session?"
// Responsibilities:
// - Stores usage metadata (who, what device, when)
// - Stores time tracking (expected and actual duration)
// - NO calculation logic - pure data storage
type SessionUsageRecord struct {
	ID               string
	DeviceType       string // "tv", "ps5", "ipad", etc.
	DeviceID         string // specific device identifier
	ChildIDs         []string
	StartTime        time.Time
	ExpectedDuration int   // Original planned duration in minutes
	ActualDuration   *int  // Actual duration in minutes (set when completed)
	Status           SessionStatus
	LastBreakAt      *time.Time
	BreakEndsAt      *time.Time
	WarningSentAt    *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsActive returns true if the session is currently active
func (s *SessionUsageRecord) IsActive() bool {
	return s.Status == SessionStatusActive
}

// IsInBreak returns true if the session is currently in a mandatory break
func (s *SessionUsageRecord) IsInBreak() bool {
	if s.BreakEndsAt == nil {
		return false
	}
	return time.Now().Before(*s.BreakEndsAt)
}

// NeedsBreak checks if a break is needed based on the break rule and last break time
func (s *SessionUsageRecord) NeedsBreak(breakRule *BreakRule) bool {
	if breakRule == nil {
		return false
	}

	var timeSince time.Time
	if s.LastBreakAt != nil {
		timeSince = *s.LastBreakAt
	} else {
		timeSince = s.StartTime
	}

	minutesSince := int(time.Since(timeSince).Minutes())
	return minutesSince >= breakRule.BreakAfterMinutes
}

// Validate validates a SessionUsageRecord
func (s *SessionUsageRecord) Validate() error {
	if s.DeviceType == "" {
		return ErrInvalidDeviceType
	}
	if len(s.ChildIDs) == 0 {
		return ErrNoChildren
	}
	if s.ExpectedDuration <= 0 {
		return ErrInvalidDuration
	}
	return nil
}

// DailyUsageSummary aggregates completed session usage for a day
// This model answers: "How much time was consumed from completed sessions?"
// Responsibilities:
// - Caches total minutes from completed sessions
// - Counts completed sessions
// - NO calculation logic - pure aggregated data storage
// Note: Active session time is calculated dynamically by TimeCalculationService
type DailyUsageSummary struct {
	ChildID      string
	Date         time.Time // Normalized to start of day
	MinutesUsed  int       // Minutes from completed sessions (active sessions added by calculator)
	SessionCount int       // Number of completed sessions
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
