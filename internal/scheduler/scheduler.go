package scheduler

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"time"
)

// Storage interface for scheduler operations
type Storage interface {
	ListActiveSessions(ctx context.Context) ([]*core.Session, error)
	GetSession(ctx context.Context, id string) (*core.Session, error)
	UpdateSession(ctx context.Context, session *core.Session) error
	GetChild(ctx context.Context, id string) (*core.Child, error)
	IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error
}

// Device interface for accessing device information
type Device interface {
	GetDriver() string
}

// DeviceDriver interface for device control
type DeviceDriver interface {
	StopSession(ctx context.Context, session *core.Session) error
	ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error
}

// DriverRegistry interface for getting device drivers
type DriverRegistry interface {
	Get(name string) (DeviceDriver, error)
}

// DeviceRegistry interface for getting devices
type DeviceRegistry interface {
	Get(id string) (Device, error)
}

// Scheduler manages periodic session updates
type Scheduler struct {
	storage        Storage
	deviceRegistry DeviceRegistry
	driverRegistry DriverRegistry
	interval       time.Duration
	timezone       *time.Location
	stopChan       chan struct{}
	logger         *slog.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(storage Storage, deviceRegistry DeviceRegistry, driverRegistry DriverRegistry, interval time.Duration, timezone *time.Location, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	if timezone == nil {
		timezone = time.UTC
	}
	return &Scheduler{
		storage:        storage,
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
		interval:       interval,
		timezone:       timezone,
		stopChan:       make(chan struct{}),
		logger:         logger,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start() {
	s.logger.Info("Scheduler started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stopChan:
			s.logger.Info("Scheduler stopped")
			return
		}
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// getDriverForSession looks up the driver for a session
func (s *Scheduler) getDriverForSession(session *core.Session) (DeviceDriver, error) {
	// Look up device
	device, err := s.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		return nil, err
	}

	// Get driver name from device
	driverName := device.GetDriver()

	// Look up driver
	return s.driverRegistry.Get(driverName)
}

// tick performs one cycle of the scheduler
func (s *Scheduler) tick() {
	ctx := context.Background()

	sessions, err := s.storage.ListActiveSessions(ctx)
	if err != nil {
		s.logger.Error("Failed to list active sessions", "error", err)
		return
	}

	s.logger.Debug("Scheduler tick",
		"active_sessions", len(sessions))

	for _, session := range sessions {
		s.logger.Debug("Processing session",
			"session_id", session.ID,
			"start_time", session.StartTime,
			"expected_duration", session.ExpectedDuration,
			"remaining_minutes", session.CalculateRemainingMinutes())

		if err := s.processSession(ctx, session); err != nil {
			s.logger.Error("Failed to process session", "session_id", session.ID, "error", err)
		}
	}
}

// processSession processes a single session
func (s *Scheduler) processSession(ctx context.Context, session *core.Session) error {
	// Check if session has a break time set
	if session.BreakEndsAt != nil {
		if time.Now().After(*session.BreakEndsAt) {
			// Break has ended, resume session
			session.BreakEndsAt = nil
			session.Status = core.SessionStatusActive
			s.logger.Info("Session break ended, resuming", "session_id", session.ID)
			return s.storage.UpdateSession(ctx, session)
		} else {
			// Still in break
			return nil
		}
	}

	// Check if any child needs a break
	for _, childID := range session.ChildIDs {
		child, err := s.storage.GetChild(ctx, childID)
		if err != nil {
			return err
		}

		if child.BreakRule != nil && session.NeedsBreak(child.BreakRule) {
			// Enforce break
			now := time.Now()
			breakEnds := now.Add(time.Duration(child.BreakRule.BreakDurationMinutes) * time.Minute)
			session.LastBreakAt = &now
			session.BreakEndsAt = &breakEnds
			session.Status = core.SessionStatusPaused

			s.logger.Info("Enforcing mandatory break",
				"session_id", session.ID,
				"break_duration", child.BreakRule.BreakDurationMinutes,
				"child", child.Name)

			// Get driver and trigger warning/pause
			driver, err := s.getDriverForSession(session)
			if err != nil {
				s.logger.Error("Failed to get driver", "session_id", session.ID, "error", err)
			} else {
				// Use warning mechanism to notify about break (driver internally looks up device)
				driver.ApplyWarning(ctx, session, 0)
			}

			return s.storage.UpdateSession(ctx, session)
		}
	}

	// Calculate remaining time for logic (but don't store it)
	minutesElapsed := int(time.Since(session.StartTime).Minutes())
	expectedRemaining := session.ExpectedDuration - minutesElapsed

	if expectedRemaining <= 0 {
		// Session time expired
		s.logger.Info("Session time expired, stopping", "session_id", session.ID)
		return s.endSession(ctx, session)
	}

	// Trigger warning if less than 5 minutes remaining (only once)
	if expectedRemaining <= 5 && expectedRemaining > 0 && session.WarningSentAt == nil {
		driver, err := s.getDriverForSession(session)
		if err == nil {
			s.logger.Info("Sending time remaining warning",
				"session_id", session.ID,
				"minutes_remaining", expectedRemaining)

			if err := driver.ApplyWarning(ctx, session, expectedRemaining); err != nil {
				s.logger.Error("Failed to apply warning",
					"session_id", session.ID,
					"error", err)
			} else {
				// Mark warning as sent and persist
				now := time.Now()
				session.WarningSentAt = &now
				s.logger.Info("Warning sent and marked",
					"session_id", session.ID,
					"minutes_remaining", expectedRemaining)
				// Update session to persist WarningSentAt
				return s.storage.UpdateSession(ctx, session)
			}
		}
	} else if expectedRemaining <= 5 && session.WarningSentAt != nil {
		s.logger.Debug("Warning already sent, skipping",
			"session_id", session.ID,
			"warning_sent_at", session.WarningSentAt,
			"minutes_remaining", expectedRemaining)
	}

	// No state changes, no need to update database
	return nil
}

// endSession ends a session and updates usage
func (s *Scheduler) endSession(ctx context.Context, session *core.Session) error {
	// Get driver
	driver, err := s.getDriverForSession(session)
	if err != nil {
		return err
	}

	// Stop session on device (driver internally looks up device and merges config)
	if err := driver.StopSession(ctx, session); err != nil {
		s.logger.Error("Failed to stop session on device", "session_id", session.ID, "error", err)
		// Continue anyway to update session status
	}

	// Update session status
	session.Status = core.SessionStatusExpired

	if err := s.storage.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Update daily usage for all children
	elapsed := int(time.Since(session.StartTime).Minutes())
	today := time.Now().In(s.timezone)

	for _, childID := range session.ChildIDs {
		if err := s.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
			s.logger.Error("Failed to update daily usage", "child_id", childID, "error", err)
		}
	}

	s.logger.Info("Session ended", "session_id", session.ID, "duration_minutes", elapsed)
	return nil
}
