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

	// Daily Time Allocation
	GetDailyAllocation(ctx context.Context, childID string, date time.Time) (*DailyTimeAllocation, error)
	CreateDailyAllocation(ctx context.Context, allocation *DailyTimeAllocation) error
	UpdateDailyAllocation(ctx context.Context, allocation *DailyTimeAllocation) error

	// Daily Usage Summary
	GetDailyUsageSummary(ctx context.Context, childID string, date time.Time) (*DailyUsageSummary, error)
	IncrementDailyUsageSummary(ctx context.Context, childID string, date time.Time, minutes int) error
	IncrementSessionCountSummary(ctx context.Context, childID string, date time.Time) error
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
	calculator     *TimeCalculationService
	downtime       *DowntimeService
	timezone       *time.Location
	logger         *slog.Logger
}

// NewSessionManager creates a new session manager
func NewSessionManager(storage Storage, deviceRegistry DeviceRegistry, driverRegistry DriverRegistry, calculator *TimeCalculationService, downtime *DowntimeService, timezone *time.Location, logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.Default()
	}
	if timezone == nil {
		timezone = time.UTC
	}
	if calculator == nil {
		// Create a default calculator
		// Note: This requires storage to implement TimeCalculationStorage interface
		// In production, calculator should be explicitly provided
		calculator = NewTimeCalculationService(storage.(TimeCalculationStorage), timezone)
	}

	return &SessionManager{
		storage:        storage,
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
		calculator:     calculator,
		downtime:       downtime,
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

	// Validate children exist and check time availability
	now := time.Now()
	today := now.In(m.timezone)
	minRemainingTime := durationMinutes // Start with requested duration

	// Check for parent override context
	isParentOverride := ctx.Value("parent_override") != nil

	for _, childID := range childIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			m.logger.Error("Failed to get child",
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		// Check downtime (unless parent override)
		if !isParentOverride && m.downtime != nil && m.downtime.IsChildInDowntime(child, now) {
			m.logger.Warn("Session start blocked by downtime",
				"child_id", childID,
				"child_name", child.Name,
				"downtime_enabled", child.DowntimeEnabled)
			return nil, ErrDowntimeActive
		}

		// Use calculator to check time availability
		remaining, err := m.calculator.GetRemainingTime(ctx, childID, today)
		if err != nil {
			m.logger.Error("Failed to get remaining time",
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get remaining time for child %s: %w", childID, err)
		}

		m.logger.Debug("Checking child time availability",
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", remaining.Available.BaseLimit,
			"reward_granted", remaining.Available.BonusGranted,
			"total_available", remaining.Available.TotalAvailable,
			"used", remaining.Consumed.TotalConsumed,
			"remaining", remaining.RemainingTotal,
			"requested", durationMinutes)

		// If child has no time left, reject the session
		if remaining.RemainingTotal == 0 {
			m.logger.Warn("No time remaining for child",
				"child_id", childID,
				"child_name", child.Name)
			return nil, fmt.Errorf("%w: child %s has no time remaining", ErrInsufficientTime, child.Name)
		}

		// Track minimum remaining time to cap the session
		if remaining.RemainingTotal < minRemainingTime {
			minRemainingTime = remaining.RemainingTotal
			m.logger.Debug("Capping session duration to child's remaining time",
				"child_id", childID,
				"child_name", child.Name,
				"remaining", remaining.RemainingTotal,
				"original_duration", durationMinutes)
		}
	}

	// If parent override, disable downtime for all children
	if isParentOverride {
		for _, childID := range childIDs {
			child, err := m.storage.GetChild(ctx, childID)
			if err != nil {
				m.logger.Error("Failed to get child for downtime override",
					"child_id", childID,
					"error", err)
				continue
			}

			if child.DowntimeEnabled {
				child.DowntimeEnabled = false
				if err := m.storage.UpdateChild(ctx, child); err != nil {
					m.logger.Error("Failed to disable downtime for child",
						"child_id", childID,
						"error", err)
				} else {
					m.logger.Info("Downtime disabled via parent override",
						"child_id", childID,
						"child_name", child.Name)
				}
			}
		}
	}

	// Cap the duration to the minimum remaining time
	actualDuration := minRemainingTime
	if actualDuration < durationMinutes {
		m.logger.Info("Session duration capped to available time",
			"requested", durationMinutes,
			"actual", actualDuration)
	}

	// Create session
	session := &Session{
		ID:               idgen.NewSession(),
		DeviceType:       device.GetType(), // Use device type from device registry
		DeviceID:         deviceID,
		ChildIDs:         childIDs,
		StartTime:        time.Now(),
		ExpectedDuration: actualDuration,
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
		if err := m.storage.IncrementSessionCountSummary(ctx, childID, today); err != nil {
			// Log but don't fail - session is already created
			m.logger.Warn("Failed to increment session count summary",
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

	// Cap individual extension requests to prevent excessive grants
	const MaxExtensionPerRequest = 30
	if additionalMinutes > MaxExtensionPerRequest {
		m.logger.Info("Extension request capped to maximum allowed",
			"session_id", sessionID,
			"requested", additionalMinutes,
			"capped_to", MaxExtensionPerRequest)
		additionalMinutes = MaxExtensionPerRequest
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

	// Rate limiting: Prevent rapid-fire extensions
	const ExtensionCooldownSeconds = 30
	if session.LastExtendedAt != nil {
		timeSinceLastExtend := time.Since(*session.LastExtendedAt)
		if timeSinceLastExtend < ExtensionCooldownSeconds*time.Second {
			m.logger.Warn("Extension rejected due to rate limiting",
				"session_id", sessionID,
				"time_since_last_extend_seconds", int(timeSinceLastExtend.Seconds()),
				"cooldown_seconds", ExtensionCooldownSeconds)
			return nil, ErrExtensionTooSoon
		}
	}

	m.logger.Debug("Session validation passed",
		"session_id", sessionID,
		"current_duration", session.ExpectedDuration,
		"elapsed", int(time.Since(session.StartTime).Minutes()))

	// Calculate maximum extension allowed based on children's remaining time
	// Cap the extension to what's actually available instead of rejecting it
	today := time.Now().In(m.timezone)
	maxExtension := additionalMinutes // Start with requested amount

	for _, childID := range session.ChildIDs {
		child, err := m.storage.GetChild(ctx, childID)
		if err != nil {
			m.logger.Error("Failed to get child for extension validation",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get child %s: %w", childID, err)
		}

		// Check downtime (no parent override allowed for extensions)
		if m.downtime != nil && m.downtime.IsChildInDowntime(child, time.Now()) {
			m.logger.Warn("Session extension blocked by downtime",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name,
				"downtime_enabled", child.DowntimeEnabled)
			return nil, ErrDowntimeActive
		}

		// Use calculator to get accurate remaining time for extension validation
		// CRITICAL: Use GetRemainingTimeForExtension which uses ExpectedDuration
		// instead of elapsed time to prevent rapid-fire extension exploit
		remaining, err := m.calculator.GetRemainingTimeForExtension(ctx, childID, today, sessionID)
		if err != nil {
			m.logger.Error("Failed to get remaining time for extension validation",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get remaining time for child %s: %w", childID, err)
		}

		m.logger.Debug("Checking child time availability for extension",
			"session_id", sessionID,
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", remaining.Available.BaseLimit,
			"reward_granted", remaining.Available.BonusGranted,
			"total_available", remaining.Available.TotalAvailable,
			"total_consumed", remaining.Consumed.TotalConsumed,
			"remaining_today", remaining.RemainingTotal,
			"requested", additionalMinutes)

		// Cap extension to this child's remaining time
		if remaining.RemainingTotal < maxExtension {
			m.logger.Warn("Extension capped due to insufficient remaining time",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name,
				"requested_minutes", additionalMinutes,
				"granted_minutes", remaining.RemainingTotal,
				"total_available_today", remaining.Available.TotalAvailable,
				"total_consumed_today", remaining.Consumed.TotalConsumed)
			maxExtension = remaining.RemainingTotal
		}
	}

	// If no time available at all, return error
	if maxExtension <= 0 {
		m.logger.Warn("No time available for any child in session",
			"session_id", sessionID,
			"requested", additionalMinutes)
		return nil, ErrInsufficientTime
	}

	// Use the capped extension amount
	actualExtension := maxExtension
	m.logger.Info("Extension amount determined",
		"session_id", sessionID,
		"requested", additionalMinutes,
		"actual", actualExtension)

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
			"driver", driver.Name(),
			"extension_minutes", actualExtension)

		if err := extendable.ExtendSession(ctx, session, actualExtension); err != nil {
			m.logger.Error("Driver failed to extend session",
				"session_id", sessionID,
				"driver", driver.Name(),
				"error", err)
			return nil, fmt.Errorf("driver failed to extend session: %w", err)
		}
	}

	// Calculate values before extension for logging
	oldExpectedDuration := session.ExpectedDuration

	// Extend session by the actual (possibly capped) amount
	session.ExpectedDuration += actualExtension

	// Update last extended timestamp for rate limiting
	now := time.Now()
	session.LastExtendedAt = &now

	// Reset warning state so a new warning can be sent when time crosses 5 minutes again
	session.WarningSentAt = nil

	m.logger.Debug("Session duration updated in memory",
		"session_id", sessionID,
		"old_duration", oldExpectedDuration,
		"new_duration", session.ExpectedDuration,
		"requested_minutes", additionalMinutes,
		"actual_minutes", actualExtension)

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
		"requested_minutes", additionalMinutes,
		"actual_minutes", actualExtension,
		"was_capped", actualExtension < additionalMinutes)

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

	// Update daily usage summary for all children
	today := time.Now().In(m.timezone)

	for _, childID := range session.ChildIDs {
		m.logger.Debug("Updating daily usage summary for child",
			"session_id", sessionID,
			"child_id", childID,
			"elapsed_minutes", elapsed)

		if err := m.storage.IncrementDailyUsageSummary(ctx, childID, today, elapsed); err != nil {
			m.logger.Error("Failed to update daily usage summary",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return fmt.Errorf("failed to update daily usage summary for child %s: %w", childID, err)
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

		// Use calculator to get accurate remaining time (includes all active sessions)
		remainingTime, err := m.calculator.GetRemainingTime(ctx, childID, today)
		if err != nil {
			m.logger.Error("Failed to get remaining time",
				"session_id", sessionID,
				"child_id", childID,
				"error", err)
			return nil, fmt.Errorf("failed to get remaining time for child %s: %w", childID, err)
		}

		m.logger.Debug("Child time availability",
			"session_id", sessionID,
			"child_id", childID,
			"child_name", child.Name,
			"daily_limit", remainingTime.Available.BaseLimit,
			"reward_granted", remainingTime.Available.BonusGranted,
			"total_available", remainingTime.Available.TotalAvailable,
			"total_consumed", remainingTime.Consumed.TotalConsumed,
			"remaining", remainingTime.RemainingTotal,
			"elapsed", elapsed)

		// Check if child has any time left
		if remainingTime.RemainingTotal == 0 {
			m.logger.Warn("Child has no time remaining",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name)
			return nil, fmt.Errorf("child %s has no time remaining", child.Name)
		}

		// Cap the charged time to what the child has available
		chargedTime := elapsed
		if remainingTime.RemainingTotal < elapsed {
			chargedTime = remainingTime.RemainingTotal
			m.logger.Info("Capping charged time to child's remaining time",
				"session_id", sessionID,
				"child_id", childID,
				"child_name", child.Name,
				"remaining", remainingTime.RemainingTotal,
				"elapsed", elapsed,
				"charged", chargedTime)
		}

		// Update daily usage summary for charged time
		if chargedTime > 0 {
			if err := m.storage.IncrementDailyUsageSummary(ctx, childID, today, chargedTime); err != nil {
				m.logger.Error("Failed to update daily usage summary",
					"session_id", sessionID,
					"child_id", childID,
					"error", err)
				return nil, fmt.Errorf("failed to update daily usage summary for child %s: %w", childID, err)
			}
		}

		// Increment session count for this child
		if err := m.storage.IncrementSessionCountSummary(ctx, childID, today); err != nil {
			// Log but don't fail
			m.logger.Warn("Failed to increment session count summary",
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

	// Grant reward for today using new allocation system
	today := time.Now().In(m.timezone)

	// Get or create allocation for today
	allocation, err := m.calculator.GetAvailableTime(ctx, childID, today)
	if err != nil {
		m.logger.Error("Failed to get allocation for reward grant",
			"child_id", childID,
			"error", err)
		return fmt.Errorf("failed to get allocation: %w", err)
	}

	// Update the allocation with new bonus
	normalizedDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, m.timezone)
	newAllocation := &DailyTimeAllocation{
		ChildID:      childID,
		Date:         normalizedDate,
		BaseLimit:    allocation.BaseLimit,
		BonusGranted: allocation.BonusGranted + minutes,
		UpdatedAt:    time.Now(),
	}

	// Try to update first, create if it doesn't exist
	if err := m.storage.UpdateDailyAllocation(ctx, newAllocation); err != nil {
		// If update fails, try to create
		newAllocation.CreatedAt = time.Now()
		if createErr := m.storage.CreateDailyAllocation(ctx, newAllocation); createErr != nil {
			m.logger.Error("Failed to grant reward minutes",
				"child_id", childID,
				"minutes", minutes,
				"error", createErr)
			return fmt.Errorf("failed to grant reward minutes: %w", createErr)
		}
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

	// Use calculator for all time calculations
	remaining, err := m.calculator.GetRemainingTime(ctx, childID, today)
	if err != nil {
		return nil, err
	}

	// Get session count from daily usage summary
	summary, err := m.storage.GetDailyUsageSummary(ctx, childID, today)
	sessionCount := 0
	if err == nil {
		sessionCount = summary.SessionCount
	}

	return &ChildStatus{
		Child:              child,
		TodayUsed:          remaining.Consumed.TotalConsumed,
		TodayRewardGranted: remaining.Available.BonusGranted,
		TodayRemaining:     remaining.RemainingTotal,
		TodayLimit:         remaining.Available.TotalAvailable,
		SessionsToday:      sessionCount,
	}, nil
}

// ChildStatus represents a child's current status
type ChildStatus struct {
	Child               *Child
	TodayUsed           int // regular minutes consumed today
	TodayRewardGranted  int // bonus minutes granted for today
	TodayRemaining      int // calculated as: limit + rewardGranted - used
	TodayLimit          int // total available today (base + rewards)
	SessionsToday       int
}
