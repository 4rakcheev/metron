package scheduler

import (
	"context"
	"log"
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

// DeviceDriver interface for device control
type DeviceDriver interface {
	StopSession(ctx context.Context, session *core.Session) error
	ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error
}

// DriverRegistry interface for getting device drivers
type DriverRegistry interface {
	Get(name string) (DeviceDriver, error)
}

// Scheduler manages periodic session updates
type Scheduler struct {
	storage   Storage
	registry  DriverRegistry
	interval  time.Duration
	stopChan  chan struct{}
	logger    *log.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(storage Storage, registry DriverRegistry, interval time.Duration, logger *log.Logger) *Scheduler {
	if logger == nil {
		logger = log.Default()
	}
	return &Scheduler{
		storage:  storage,
		registry: registry,
		interval: interval,
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start() {
	s.logger.Println("Scheduler started")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stopChan:
			s.logger.Println("Scheduler stopped")
			return
		}
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// tick performs one cycle of the scheduler
func (s *Scheduler) tick() {
	ctx := context.Background()

	sessions, err := s.storage.ListActiveSessions(ctx)
	if err != nil {
		s.logger.Printf("Error listing active sessions: %v", err)
		return
	}

	for _, session := range sessions {
		if err := s.processSession(ctx, session); err != nil {
			s.logger.Printf("Error processing session %s: %v", session.ID, err)
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
			s.logger.Printf("Session %s: break ended, resuming", session.ID)
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

			s.logger.Printf("Session %s: enforcing %d minute break for child %s",
				session.ID, child.BreakRule.BreakDurationMinutes, child.Name)

			// Get driver and trigger warning/pause
			driver, err := s.registry.Get(session.DeviceType)
			if err != nil {
				s.logger.Printf("Error getting driver for session %s: %v", session.ID, err)
			} else {
				// Use warning mechanism to notify about break
				driver.ApplyWarning(ctx, session, 0)
			}

			return s.storage.UpdateSession(ctx, session)
		}
	}

	// Decrement remaining time
	minutesElapsed := int(time.Since(session.StartTime).Minutes())
	expectedRemaining := session.ExpectedDuration - minutesElapsed

	if expectedRemaining <= 0 {
		// Session time expired
		s.logger.Printf("Session %s: time expired, stopping", session.ID)
		return s.endSession(ctx, session)
	}

	// Update remaining minutes
	session.RemainingMinutes = expectedRemaining

	// Trigger warning if less than 5 minutes remaining
	if expectedRemaining <= 5 && expectedRemaining > 0 {
		driver, err := s.registry.Get(session.DeviceType)
		if err == nil {
			s.logger.Printf("Session %s: %d minutes remaining, sending warning", session.ID, expectedRemaining)
			driver.ApplyWarning(ctx, session, expectedRemaining)
		}
	}

	return s.storage.UpdateSession(ctx, session)
}

// endSession ends a session and updates usage
func (s *Scheduler) endSession(ctx context.Context, session *core.Session) error {
	// Get driver
	driver, err := s.registry.Get(session.DeviceType)
	if err != nil {
		return err
	}

	// Stop session on device
	if err := driver.StopSession(ctx, session); err != nil {
		s.logger.Printf("Error stopping session %s on device: %v", session.ID, err)
		// Continue anyway to update session status
	}

	// Update session status
	session.Status = core.SessionStatusExpired
	session.RemainingMinutes = 0

	if err := s.storage.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Update daily usage for all children
	elapsed := int(time.Since(session.StartTime).Minutes())
	today := time.Now()

	for _, childID := range session.ChildIDs {
		if err := s.storage.IncrementDailyUsage(ctx, childID, today, elapsed); err != nil {
			s.logger.Printf("Error updating daily usage for child %s: %v", childID, err)
		}
	}

	s.logger.Printf("Session %s ended after %d minutes", session.ID, elapsed)
	return nil
}
