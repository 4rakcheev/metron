package core

import (
	"context"
	"fmt"
	"log/slog"
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
	DeleteSession(ctx context.Context, id string) error

	GetDailyUsage(ctx context.Context, childID string, date time.Time) (*DailyUsage, error)
	IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error
	GrantRewardMinutes(ctx context.Context, childID string, date time.Time, minutes int) error
	IncrementSessionCount(ctx context.Context, childID string, date time.Time) error
}

// Device interface for accessing device information
type Device interface {
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
	timezone       *time.Location
	logger         *slog.Logger
}

// NewSessionManager creates a new session manager
func NewSessionManager(storage Storage, deviceRegistry DeviceRegistry, driverRegistry DriverRegistry, timezone *time.Location, logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.Default()
	}
	if timezone == nil {
		timezone = time.UTC
	}
	return &SessionManager{
		storage:        storage,
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
		timezone:       timezone,
		logger:         logger,
	}
}

// StartSession starts a new session for one or more children
func (m *SessionManager) StartSession(ctx context.Context, deviceID string, childIDs []string, durationMinutes int) (*Session, error) {
	m.logger.Info("Starting new session",
		"device_id", deviceID,
		"child_ids", childIDs,
		"duration_minutes", durationMinutes)

	// Validate inputs
	if deviceID == "" {
		m.logger.Error("Session start failed: empty device ID")
		return nil, fmt.Errorf("device ID cannot be empty")
	}
	if len(childIDs) == 0 {
		m.logger.Error("Session start failed: no children specified")
		return nil, ErrNoChildren
	}
	if durationMinutes <= 0 {
		m.logger.Error("Session start failed: invalid duration",
			"duration_minutes", durationMinutes)
		return nil, ErrInvalidDuration
	}

	// Look up device from device registry
	device, err := m.deviceRegistry.Get(deviceID)
	if err != nil {
		m.logger.Error("Failed to get device from registry",
			"device_id", deviceID,
			"error", err)
		return nil, fmt.Errorf("failed to get device %s: %w", deviceID, err)
	}

	m.logger.Debug("Device found",
		"device_id", deviceID,
		"device_type", device.GetType(),
		"driver", device.GetDriver())

	// Validate children exist and have sufficient time
	now := time.Now()
	today := now.In(m.timezone)
	for _, childID := range childIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			m.logger.Error("Failed to get child",
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		// Check daily limit
		dailyLimit := child.GetDailyLimit(today)
		usage, err := m.storage.GetDailyUsage(ctx, childID, today)
		if err != nil {
			m.logger.Error("Failed to get daily usage",
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get daily usage for child %s: %w", childID, err)
		}

		// Include granted rewards in available time calculation
		totalAvailable := dailyLimit + usage.RewardMinutesGranted
		remainingMinutes := totalAvailable - usage.MinutesUsed
		m.logger.Debug("Checking child time availability",
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", dailyLimit,
			"reward_granted", usage.RewardMinutesGranted,
			"total_available", totalAvailable,
			"used", usage.MinutesUsed,
			"remaining", remainingMinutes,
			"requested", durationMinutes)

		if remainingMinutes < durationMinutes {
			m.logger.Warn("Insufficient time for child",
				"child_id", childID,
				"child_name", child.Name,
				"remaining", remainingMinutes,
				"requested", durationMinutes)
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
		Status:           SessionStatusActive,
	}

	// Get device driver
	driver, err := m.driverRegistry.Get(device.GetDriver())
	if err != nil {
		m.logger.Error("Failed to get driver from registry",
			"driver_name", device.GetDriver(),
			"device_id", deviceID,
			"error", err)
		return nil, fmt.Errorf("failed to get driver %s for device %s: %w", device.GetDriver(), deviceID, err)
	}

	m.logger.Debug("Saving session to storage before unlocking device",
		"session_id", session.ID,
		"driver", driver.Name())

	// CRITICAL: Save session to database FIRST before unlocking device
	// This ensures device stays locked if database save fails
	if err := m.storage.CreateSession(ctx, session); err != nil {
		m.logger.Error("Failed to save session to storage",
			"session_id", session.ID,
			"error", err)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Start session on device (unlock it) ONLY after successful database save
	if err := driver.StartSession(ctx, session); err != nil {
		m.logger.Error("Driver failed to start session",
			"session_id", session.ID,
			"driver", driver.Name(),
			"error", err)

		// CRITICAL: Delete the saved session since device unlock failed
		if delErr := m.storage.DeleteSession(ctx, session.ID); delErr != nil {
			m.logger.Error("Failed to cleanup session after driver failure",
				"session_id", session.ID,
				"error", delErr)
		}

		return nil, fmt.Errorf("failed to start session on device: %w", err)
	}

	// Check if immediate warning is needed (for short sessions <= 5 minutes)
	if durationMinutes <= 5 {
		m.logger.Debug("Session duration is short, sending immediate warning",
			"session_id", session.ID,
			"duration_minutes", durationMinutes)

		// Trigger warning immediately for sessions that start with 5 minutes or less
		if err := driver.ApplyWarning(ctx, session, durationMinutes); err != nil {
			// Log but don't fail - session is already created
			m.logger.Warn("Failed to send immediate warning for short session",
				"session_id", session.ID,
				"error", err)
		} else {
			// Mark warning as sent
			now := time.Now()
			session.WarningSentAt = &now
			if err := m.storage.UpdateSession(ctx, session); err != nil {
				m.logger.Warn("Failed to mark warning as sent",
					"session_id", session.ID,
					"error", err)
			}
		}
	}

	// Increment session count for all children in this session
	for _, childID := range childIDs {
		if err := m.storage.IncrementSessionCount(ctx, childID, today); err != nil {
			// Log but don't fail - session is already created
			m.logger.Warn("Failed to increment session count",
				"session_id", session.ID,
				"child_id", childID,
				"error", err)
		}
	}

	m.logger.Info("Session started successfully",
		"session_id", session.ID,
		"device_id", deviceID,
		"child_ids", childIDs,
		"duration_minutes", durationMinutes)

	return session, nil
}

// ExtendSession extends an active session
func (m *SessionManager) ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*Session, error) {
	m.logger.Info("Extending session",
		"session_id", sessionID,
		"additional_minutes", additionalMinutes)

	if additionalMinutes <= 0 {
		m.logger.Error("Invalid extension duration",
			"session_id", sessionID,
			"additional_minutes", additionalMinutes)
		return nil, ErrInvalidDuration
	}

	// Get session
	session, err := m.storage.GetSession(ctx, sessionID)
	if err != nil {
		m.logger.Error("Failed to get session",
			"session_id", sessionID,
			"error", err)
		return nil, err
	}

	if !session.IsActive() {
		m.logger.Warn("Cannot extend inactive session",
			"session_id", sessionID,
			"status", session.Status)
		return nil, ErrSessionNotActive
	}

	m.logger.Debug("Session validation passed",
		"session_id", sessionID,
		"current_duration", session.ExpectedDuration,
		"elapsed", int(time.Since(session.StartTime).Minutes()))

	// Validate children have sufficient time
	today := time.Now().In(m.timezone)
	for _, childID := range session.ChildIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			m.logger.Error("Failed to get child for extension validation",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		dailyLimit := child.GetDailyLimit(today)
		usage, err := m.storage.GetDailyUsage(ctx, childID, today)
		if err != nil {
			m.logger.Error("Failed to get daily usage for extension validation",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get daily usage for child %s: %w", childID, err)
		}

		// Calculate time already consumed in this session
		elapsed := int(time.Since(session.StartTime).Minutes())
		// Include granted rewards in available time calculation
		totalAvailable := dailyLimit + usage.RewardMinutesGranted
		remainingToday := totalAvailable - usage.MinutesUsed - elapsed

		m.logger.Debug("Checking child time availability for extension",
			"session_id", sessionID,
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", dailyLimit,
			"reward_granted", usage.RewardMinutesGranted,
			"total_available", totalAvailable,
			"used", usage.MinutesUsed,
			"elapsed_in_session", elapsed,
			"remaining_today", remainingToday,
			"requested", additionalMinutes)

		if remainingToday < additionalMinutes {
			m.logger.Warn("Insufficient time for extension",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name,
				"remaining", remainingToday,
				"requested", additionalMinutes)
			return nil, fmt.Errorf("%w: child %s would exceed daily limit", ErrInsufficientTime, child.Name)
		}
	}

	// Look up device to get driver name
	device, err := m.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		m.logger.Error("Failed to get device for extension",
			"session_id", sessionID,
			"device_id", session.DeviceID,
			"error", err)
		return nil, fmt.Errorf("failed to get device %s: %w", session.DeviceID, err)
	}

	// Get device driver
	driver, err := m.driverRegistry.Get(device.GetDriver())
	if err != nil {
		m.logger.Error("Failed to get driver for extension",
			"session_id", sessionID,
			"driver_name", device.GetDriver(),
			"device_id", session.DeviceID,
			"error", err)
		return nil, fmt.Errorf("failed to get driver %s for device %s: %w", device.GetDriver(), session.DeviceID, err)
	}

	// If driver supports extension, call it before updating session
	if extendable, ok := driver.(interface {
		ExtendSession(ctx context.Context, session *Session, additionalMinutes int) error
	}); ok {
		m.logger.Debug("Calling driver ExtendSession method",
			"session_id", sessionID,
			"driver", driver.Name())

		if err := extendable.ExtendSession(ctx, session, additionalMinutes); err != nil {
			m.logger.Error("Driver failed to extend session",
				"session_id", sessionID,
				"driver", driver.Name(),
				"error", err)
			return nil, fmt.Errorf("driver failed to extend session: %w", err)
		}
	}

	// Calculate values before extension for logging
	oldExpectedDuration := session.ExpectedDuration

	// Extend session
	session.ExpectedDuration += additionalMinutes

	// Reset warning state so a new warning can be sent when time crosses 5 minutes again
	session.WarningSentAt = nil

	m.logger.Debug("Session duration updated in memory",
		"session_id", sessionID,
		"old_duration", oldExpectedDuration,
		"new_duration", session.ExpectedDuration,
		"additional_minutes", additionalMinutes)

	if err := m.storage.UpdateSession(ctx, session); err != nil {
		m.logger.Error("Failed to persist session extension",
			"session_id", sessionID,
			"error", err)
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	m.logger.Info("Session extended successfully",
		"session_id", sessionID,
		"old_duration", oldExpectedDuration,
		"new_duration", session.ExpectedDuration,
		"additional_minutes", additionalMinutes)

	return session, nil
}

// StopSession stops an active session
func (m *SessionManager) StopSession(ctx context.Context, sessionID string) error {
	m.logger.Info("Stopping session",
		"session_id", sessionID)

	// Get session
	session, err := m.storage.GetSession(ctx, sessionID)
	if err != nil {
		m.logger.Error("Failed to get session for stop",
			"session_id", sessionID,
			"error", err)
		return err
	}

	if !session.IsActive() {
		m.logger.Warn("Cannot stop inactive session",
			"session_id", sessionID,
			"status", session.Status)
		return ErrSessionNotActive
	}

	elapsed := int(time.Since(session.StartTime).Minutes())
	m.logger.Debug("Session details",
		"session_id", sessionID,
		"device_id", session.DeviceID,
		"child_ids", session.ChildIDs,
		"expected_duration", session.ExpectedDuration,
		"elapsed_minutes", elapsed)

	// Look up device to get driver name
	device, err := m.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		m.logger.Error("Failed to get device for stop",
			"session_id", sessionID,
			"device_id", session.DeviceID,
			"error", err)
		return fmt.Errorf("failed to get device %s: %w", session.DeviceID, err)
	}

	// Get device driver
	driver, err := m.driverRegistry.Get(device.GetDriver())
	if err != nil {
		m.logger.Error("Failed to get driver for stop",
			"session_id", sessionID,
			"driver_name", device.GetDriver(),
			"device_id", session.DeviceID,
			"error", err)
		return fmt.Errorf("failed to get driver %s for device %s: %w", device.GetDriver(), session.DeviceID, err)
	}

	m.logger.Debug("Stopping session on device via driver",
		"session_id", sessionID,
		"driver", driver.Name())

	// Stop session on device
	if err := driver.StopSession(ctx, session); err != nil {
		m.logger.Error("Driver failed to stop session",
			"session_id", sessionID,
			"driver", driver.Name(),
			"error", err)
		return fmt.Errorf("failed to stop session on device: %w", err)
	}

	// Update session status
	session.Status = SessionStatusCompleted

	if err := m.storage.UpdateSession(ctx, session); err != nil {
		m.logger.Error("Failed to update session status",
			"session_id", sessionID,
			"error", err)
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Update daily usage for all children
	today := time.Now().In(m.timezone)

	for _, childID := range session.ChildIDs {
		m.logger.Debug("Updating daily usage for child",
			"session_id", sessionID,
			"child_id", childID,
			"elapsed_minutes", elapsed)

		if err := m.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
			m.logger.Error("Failed to update daily usage",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return fmt.Errorf("failed to update daily usage for child %s: %w", childID, err)
		}
	}

	m.logger.Info("Session stopped successfully",
		"session_id", sessionID,
		"elapsed_minutes", elapsed,
		"child_ids", session.ChildIDs)

	return nil
}

// AddChildrenToSession adds one or more children to an active session
func (m *SessionManager) AddChildrenToSession(ctx context.Context, sessionID string, childIDs []string) (*Session, error) {
	m.logger.Info("Adding children to session",
		"session_id", sessionID,
		"child_ids", childIDs)

	// Get session
	session, err := m.storage.GetSession(ctx, sessionID)
	if err != nil {
		m.logger.Error("Failed to get session",
			"session_id", sessionID,
			"error", err)
		return nil, err
	}

	if !session.IsActive() {
		m.logger.Warn("Cannot add children to inactive session",
			"session_id", sessionID,
			"status", session.Status)
		return nil, ErrSessionNotActive
	}

	// Calculate elapsed time since session start
	elapsed := int(time.Since(session.StartTime).Minutes())
	if elapsed < 0 {
		elapsed = 0
	}

	today := time.Now().In(m.timezone)
	newChildIDs := []string{}

	for _, childID := range childIDs {
		// Check if child already in session
		alreadyInSession := false
		for _, existingID := range session.ChildIDs {
			if existingID == childID {
				alreadyInSession = true
				break
			}
		}

		if alreadyInSession {
			m.logger.Debug("Child already in session, skipping",
				"session_id", sessionID,
				"child_id", childID)
			continue
		}

		// Get child to check remaining time
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			m.logger.Error("Failed to get child",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		// Get daily usage
		usage, err := m.storage.GetDailyUsage(ctx, childID, today)
		if err != nil {
			m.logger.Error("Failed to get daily usage",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get daily usage for child %s: %w", childID, err)
		}

		// Calculate remaining time (include granted rewards)
		dailyLimit := child.GetDailyLimit(today)
		totalAvailable := dailyLimit + usage.RewardMinutesGranted
		remaining := totalAvailable - usage.MinutesUsed

		m.logger.Debug("Child time availability",
			"session_id", sessionID,
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", dailyLimit,
			"reward_granted", usage.RewardMinutesGranted,
			"total_available", totalAvailable,
			"used", usage.MinutesUsed,
			"remaining", remaining,
			"elapsed", elapsed)

		// Check if child has enough time for elapsed minutes
		if remaining < elapsed {
			m.logger.Warn("Child has insufficient time for elapsed session time",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name,
				"remaining", remaining,
				"elapsed", elapsed)
			return nil, fmt.Errorf("child %s has insufficient time (has %d min, needs %d min for elapsed time)",
				child.Name, remaining, elapsed)
		}

		// Update daily usage for elapsed time
		if elapsed > 0 {
			if err := m.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
				m.logger.Error("Failed to update daily usage",
					"session_id", sessionID,
					"child_id", childID,
					"error", err)
				return nil, fmt.Errorf("failed to update daily usage for child %s: %w", childID, err)
			}
		}

		// Increment session count for this child
		if err := m.storage.IncrementSessionCount(ctx, childID, today); err != nil {
			// Log but don't fail
			m.logger.Warn("Failed to increment session count",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
		}

		newChildIDs = append(newChildIDs, childID)
	}

	if len(newChildIDs) == 0 {
		m.logger.Info("No new children to add to session",
			"session_id", sessionID)
		return session, nil
	}

	// Add new children to session
	session.ChildIDs = append(session.ChildIDs, newChildIDs...)

	// Update session
	if err := m.storage.UpdateSession(ctx, session); err != nil {
		m.logger.Error("Failed to update session",
			"session_id", sessionID,
			"error", err)
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	m.logger.Info("Children added to session successfully",
		"session_id", sessionID,
		"new_child_ids", newChildIDs,
		"all_child_ids", session.ChildIDs)

	return session, nil
}

// GetSession retrieves a session by ID
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return m.storage.GetSession(ctx, sessionID)
}

// ListActiveSessions retrieves all active sessions
func (m *SessionManager) ListActiveSessions(ctx context.Context) ([]*Session, error) {
	return m.storage.ListActiveSessions(ctx)
}

// GrantRewardMinutes grants bonus minutes to a child for today
func (m *SessionManager) GrantRewardMinutes(ctx context.Context, childID string, minutes int) error {
	m.logger.Info("Granting reward minutes",
		"child_id", childID,
		"minutes", minutes)

	if minutes <= 0 {
		m.logger.Error("Invalid reward minutes",
			"child_id", childID,
			"minutes", minutes)
		return fmt.Errorf("reward minutes must be positive")
	}

	// Verify child exists
	_, err := m.storage.GetChild(ctx, childID)
	if err != nil {
		m.logger.Error("Failed to get child for reward grant",
			"child_id", childID,
			"error", err)
		return err
	}

	// Grant reward for today
	today := time.Now().In(m.timezone)
	if err := m.storage.GrantRewardMinutes(ctx, childID, today, minutes); err != nil {
		m.logger.Error("Failed to grant reward minutes",
			"child_id", childID,
			"minutes", minutes,
			"error", err)
		return fmt.Errorf("failed to grant reward minutes: %w", err)
	}

	m.logger.Info("Reward minutes granted successfully",
		"child_id", childID,
		"minutes", minutes)

	return nil
}

// GetChildStatus retrieves the current status for a child
func (m *SessionManager) GetChildStatus(ctx context.Context, childID string) (*ChildStatus, error) {
	child, err := m.storage.GetChild(ctx, childID)
	if err != nil {
		return nil, err
	}

	today := time.Now().In(m.timezone)
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
	// Include granted rewards in available time
	totalAvailable := dailyLimit + usage.RewardMinutesGranted
	remaining := totalAvailable - totalUsed
	if remaining < 0 {
		remaining = 0
	}

	return &ChildStatus{
		Child:              child,
		TodayUsed:          totalUsed,
		TodayRewardGranted: usage.RewardMinutesGranted,
		TodayRemaining:     remaining,
		TodayLimit:         dailyLimit,
		SessionsToday:      usage.SessionCount,
	}, nil
}

// ChildStatus represents a child's current status
type ChildStatus struct {
	Child               *Child
	TodayUsed           int // regular minutes consumed today
	TodayRewardGranted  int // bonus minutes granted for today
	TodayRemaining      int // calculated as: limit + rewardGranted - used
	TodayLimit          int
	SessionsToday       int
}
