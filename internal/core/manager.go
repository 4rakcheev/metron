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

// Device interface for accessing device information
type Device interface {
	GetID() string
	GetName() string
	GetType() string
	GetDriver() string
}

// DeviceRegistry interface defines device management operations
type DeviceRegistry interface {
	Get(id string) (Device, error)
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
	storage        Storage
	deviceRegistry DeviceRegistry
	driverRegistry DriverRegistry
}

// NewSessionManager creates a new session manager
func NewSessionManager(storage Storage, deviceRegistry DeviceRegistry, driverRegistry DriverRegistry) *SessionManager {
	return &SessionManager{
		storage:        storage,
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
	}
}

// StartSession starts a new session for one or more children
func (m *SessionManager) StartSession(ctx context.Context, deviceID string, childIDs []string, durationMinutes int) (*Session, error) {
	// Validate inputs
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}
	if len(childIDs) == 0 {
		return nil, ErrNoChildren
	}
	if durationMinutes <= 0 {
		return nil, ErrInvalidDuration
	}

	// Look up device from device registry
	device, err := m.deviceRegistry.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device %s: %w", deviceID, err)
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
		DeviceType:       device.GetType(), // Use device type from device registry
		DeviceID:         deviceID,
		ChildIDs:         childIDs,
		StartTime:        time.Now(),
		ExpectedDuration: durationMinutes,
		RemainingMinutes: durationMinutes,
		Status:           SessionStatusActive,
	}

	// Get device driver
	driver, err := m.driverRegistry.Get(device.GetDriver())
	if err != nil {
		return nil, fmt.Errorf("failed to get driver %s for device %s: %w", device.GetDriver(), deviceID, err)
	}

	// Start session on device
	if err := driver.StartSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to start session on device: %w", err)
	}

	// Save session
	if err := m.storage.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Check if immediate warning is needed (for short sessions <= 5 minutes)
	if durationMinutes <= 5 {
		// Trigger warning immediately for sessions that start with 5 minutes or less
		if err := driver.ApplyWarning(ctx, session, durationMinutes); err != nil {
			// Log but don't fail - session is already created
			fmt.Printf("Warning: failed to send immediate warning for new session %s: %v\n", session.ID, err)
		} else {
			// Mark warning as sent
			now := time.Now()
			session.WarningSentAt = &now
			if err := m.storage.UpdateSession(ctx, session); err != nil {
				fmt.Printf("Warning: failed to mark warning as sent for session %s: %v\n", session.ID, err)
			}
		}
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

	// Calculate values before extension for logging
	oldExpectedDuration := session.ExpectedDuration
	oldRemainingMinutes := session.RemainingMinutes

	// Extend session
	session.ExpectedDuration += additionalMinutes

	// Recalculate remaining minutes based on new expected duration
	endTime := session.StartTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)
	session.RemainingMinutes = int(time.Until(endTime).Minutes())
	if session.RemainingMinutes < 0 {
		session.RemainingMinutes = 0
	}

	// Reset warning state so a new warning can be sent when time crosses 5 minutes again
	session.WarningSentAt = nil

	// Log extension details
	fmt.Printf("Session extended: session_id=%s, added=%d, duration: %d→%d, remaining: %d→%d\n",
		session.ID, additionalMinutes, oldExpectedDuration, session.ExpectedDuration,
		oldRemainingMinutes, session.RemainingMinutes)

	if err := m.storage.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	fmt.Printf("Session extension persisted: session_id=%s, new_duration=%d\n",
		session.ID, session.ExpectedDuration)

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

	// Look up device to get driver name
	device, err := m.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", session.DeviceID, err)
	}

	// Get device driver
	driver, err := m.driverRegistry.Get(device.GetDriver())
	if err != nil {
		return fmt.Errorf("failed to get driver %s for device %s: %w", device.GetDriver(), session.DeviceID, err)
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

	// Get active sessions to include their elapsed time
	activeSessions, err := m.storage.ListActiveSessions(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate elapsed time from active sessions for this child
	activeMinutes := 0
	for _, session := range activeSessions {
		// Check if this session includes the child
		for _, sessionChildID := range session.ChildIDs {
			if sessionChildID == childID {
				// Calculate elapsed time (clamped to expected duration)
				elapsed := int(time.Since(session.StartTime).Minutes())
				if elapsed > session.ExpectedDuration {
					elapsed = session.ExpectedDuration
				}
				activeMinutes += elapsed
				break
			}
		}
	}

	// Total used time = completed sessions + active sessions
	totalUsed := usage.MinutesUsed + activeMinutes
	dailyLimit := child.GetDailyLimit(today)
	remaining := dailyLimit - totalUsed
	if remaining < 0 {
		remaining = 0
	}

	return &ChildStatus{
		Child:          child,
		TodayUsed:      totalUsed,
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
