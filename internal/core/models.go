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
	ID           string
	Name         string
	WeekdayLimit int // minutes per weekday
	WeekendLimit int // minutes per weekend day
	BreakRule    *BreakRule
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BreakRule defines mandatory break periods
type BreakRule struct {
	BreakAfterMinutes   int // require break after this many minutes
	BreakDurationMinutes int // break must last this many minutes
}

// Session represents an active or completed screen-time session
type Session struct {
	ID                string
	DeviceType        string // "tv", "ps5", "ipad", etc.
	DeviceID          string // specific device identifier
	ChildIDs          []string
	StartTime         time.Time
	ExpectedDuration  int // minutes
	RemainingMinutes  int
	Status            SessionStatus
	LastBreakAt       *time.Time
	BreakEndsAt       *time.Time
	WarningSentAt     *time.Time // tracks when time-remaining warning was sent
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// DailyUsage tracks a child's usage for a specific day
type DailyUsage struct {
	ChildID      string
	Date         time.Time // normalized to start of day
	MinutesUsed  int
	SessionCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validation errors
var (
	ErrInvalidChildID        = errors.New("invalid child ID")
	ErrInvalidName           = errors.New("child name cannot be empty")
	ErrInvalidWeekdayLimit   = errors.New("weekday limit must be positive")
	ErrInvalidWeekendLimit   = errors.New("weekend limit must be positive")
	ErrInvalidBreakRule      = errors.New("invalid break rule configuration")
	ErrInvalidDuration       = errors.New("duration must be positive")
	ErrInvalidDeviceType     = errors.New("device type cannot be empty")
	ErrNoChildren            = errors.New("session must have at least one child")
	ErrInsufficientTime      = errors.New("child has insufficient remaining time")
	ErrSessionNotActive      = errors.New("session is not active")
	ErrSessionNotFound       = errors.New("session not found")
	ErrChildNotFound         = errors.New("child not found")
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
