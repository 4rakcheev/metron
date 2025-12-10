package core

import (
	"context"
	"fmt"
	"metron/internal/idgen"
	"time"
)

// Storage interface defines required storage operations
type Storage interface {
	CreateChild(ctx context.Context, child *Child) error
	GetChild(ctx context.Context, id string) (*Child, error)
	ListChildren(ctx context.Context) ([]*Child, error)
	UpdateChild(ctx context.Context, child *Child) error

	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	ListActiveSessions(ctx context.Context) ([]*Session, error)
	UpdateSession(ctx context.Context, session *Session) error

	GetDailyUsage(ctx context.Context, childID string, date time.Time) (*DailyUsage, error)
	IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error
}

// DeviceDriver interface defines device control operations
type DeviceDriver interface {
	Name() string
	StartSession(ctx context.Context, session *Session) error
	StopSession(ctx context.Context, session *Session) error
	ApplyWarning(ctx context.Context, session *Session, minutesRemaining int) error
}

// DriverRegistry interface defines driver management operations
type DriverRegistry interface {
	Get(name string) (DeviceDriver, error)
}

// SessionManager manages screen-time sessions
type SessionManager struct {
	storage  Storage
	registry DriverRegistry
}

// NewSessionManager creates a new session manager
func NewSessionManager(storage Storage, registry DriverRegistry) *SessionManager {
	return &SessionManager{
		storage:  storage,
		registry: registry,
	}
}

// StartSession starts a new session for one or more children
func (m *SessionManager) StartSession(ctx context.Context, deviceType string, deviceID string, childIDs []string, durationMinutes int) (*Session, error) {
	// Validate inputs
	if deviceType == "" {
		return nil, ErrInvalidDeviceType
	}
	if len(childIDs) == 0 {
		return nil, ErrNoChildren
	}
	if durationMinutes <= 0 {
		return nil, ErrInvalidDuration
	}

	// Validate children exist and have sufficient time
	today := time.Now()
	for _, childID := range childIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		// Check daily limit
		dailyLimit := child.GetDailyLimit(today)
		usage, err := m.storage.GetDailyUsage(ctx, childID, today)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily usage for child %s: %w", childID, err)
		}

		remainingMinutes := dailyLimit - usage.MinutesUsed
		if remainingMinutes < durationMinutes {
			return nil, fmt.Errorf("%w: child %s has only %d minutes remaining", ErrInsufficientTime, child.Name, remainingMinutes)
		}
	}

	// Create session
	session := &Session{
		ID:               idgen.NewSession(),
		DeviceType:       deviceType,
		DeviceID:         deviceID,
		ChildIDs:         childIDs,
		StartTime:        time.Now(),
		ExpectedDuration: durationMinutes,
		RemainingMinutes: durationMinutes,
		Status:           SessionStatusActive,
	}

	// Get device driver
	driver, err := m.registry.Get(deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver for %s: %w", deviceType, err)
	}

	// Start session on device
	if err := driver.StartSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to start session on device: %w", err)
	}

	// Save session
	if err := m.storage.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// ExtendSession extends an active session
func (m *SessionManager) ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*Session, error) {
	if additionalMinutes <= 0 {
		return nil, ErrInvalidDuration
	}

	// Get session
	session, err := m.storage.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if !session.IsActive() {
		return nil, ErrSessionNotActive
	}

	// Validate children have sufficient time
	today := time.Now()
	for _, childID := range session.ChildIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		dailyLimit := child.GetDailyLimit(today)
		usage, err := m.storage.GetDailyUsage(ctx, childID, today)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily usage for child %s: %w", childID, err)
		}

		// Calculate time already consumed in this session
		elapsed := int(time.Since(session.StartTime).Minutes())
		remainingToday := dailyLimit - usage.MinutesUsed - elapsed

		if remainingToday < additionalMinutes {
			return nil, fmt.Errorf("%w: child %s would exceed daily limit", ErrInsufficientTime, child.Name)
		}
	}

	// Extend session
	session.RemainingMinutes += additionalMinutes
	session.ExpectedDuration += additionalMinutes

	if err := m.storage.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, nil
}

// StopSession stops an active session
func (m *SessionManager) StopSession(ctx context.Context, sessionID string) error {
	// Get session
	session, err := m.storage.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if !session.IsActive() {
		return ErrSessionNotActive
	}

	// Get device driver
	driver, err := m.registry.Get(session.DeviceType)
	if err != nil {
		return fmt.Errorf("failed to get driver for %s: %w", session.DeviceType, err)
	}

	// Stop session on device
	if err := driver.StopSession(ctx, session); err != nil {
		return fmt.Errorf("failed to stop session on device: %w", err)
	}

	// Update session status
	session.Status = SessionStatusCompleted
	session.RemainingMinutes = 0

	if err := m.storage.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Update daily usage for all children
	elapsed := int(time.Since(session.StartTime).Minutes())
	today := time.Now()

	for _, childID := range session.ChildIDs {
		if err := m.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
			return fmt.Errorf("failed to update daily usage for child %s: %w", childID, err)
		}
	}

	return nil
}

// GetSession retrieves a session by ID
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return m.storage.GetSession(ctx, sessionID)
}

// ListActiveSessions retrieves all active sessions
func (m *SessionManager) ListActiveSessions(ctx context.Context) ([]*Session, error) {
	return m.storage.ListActiveSessions(ctx)
}

// GetChildStatus retrieves the current status for a child
func (m *SessionManager) GetChildStatus(ctx context.Context, childID string) (*ChildStatus, error) {
	child, err := m.storage.GetChild(ctx, childID)
	if err != nil {
		return nil, err
	}

	today := time.Now()
	usage, err := m.storage.GetDailyUsage(ctx, childID, today)
	if err != nil {
		return nil, err
	}

	dailyLimit := child.GetDailyLimit(today)
	remaining := dailyLimit - usage.MinutesUsed

	return &ChildStatus{
		Child:          child,
		TodayUsed:      usage.MinutesUsed,
		TodayRemaining: remaining,
		TodayLimit:     dailyLimit,
		SessionsToday:  usage.SessionCount,
	}, nil
}

// ChildStatus represents a child's current status
type ChildStatus struct {
	Child          *Child
	TodayUsed      int
	TodayRemaining int
	TodayLimit     int
	SessionsToday  int
}
