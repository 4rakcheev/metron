package core

import (
	"context"
	"log/slog"
	"metron/config"
	"metron/internal/idgen"
	"time"
)

// MovieTimeStorage defines storage interface for movie time operations
type MovieTimeStorage interface {
	GetMovieTimeUsage(ctx context.Context, date time.Time) (*MovieTimeUsage, error)
	SaveMovieTimeUsage(ctx context.Context, usage *MovieTimeUsage) error
	ListChildren(ctx context.Context) ([]*Child, error)
	ListActiveSessions(ctx context.Context) ([]*Session, error)
	CreateSession(ctx context.Context, session *Session) error
	UpdateSession(ctx context.Context, session *Session) error
	IncrementSessionCountSummary(ctx context.Context, childID string, date time.Time) error
	// Bypass methods
	ListActiveMovieTimeBypasses(ctx context.Context, date time.Time) ([]*MovieTimeBypass, error)
}

// MovieTimeService handles weekend shared movie time feature
type MovieTimeService struct {
	storage        MovieTimeStorage
	deviceRegistry DeviceRegistry
	driverRegistry DriverRegistry
	config         *config.MovieTimeConfig
	timezone       *time.Location
	logger         *slog.Logger
}

// MovieTimeAvailability represents the current movie time availability status
type MovieTimeAvailability struct {
	IsWeekend        bool       `json:"is_weekend"`
	IsBypassActive   bool       `json:"is_bypass_active"`   // Bypass allows movie time on non-weekends
	BypassReason     string     `json:"bypass_reason,omitempty"` // Why bypass is active (e.g., "School vacation")
	IsAvailable      bool       `json:"is_available"`       // Overall availability
	IsUsedToday      bool       `json:"is_used_today"`      // Already used today
	BreakRequired    bool       `json:"break_required"`     // Still in break period
	BreakMinutesLeft int        `json:"break_minutes_left"` // Minutes until break ends (0 if met)
	LastSessionEnd   *time.Time `json:"last_session_end,omitempty"`
	CanStart         bool       `json:"can_start"`  // Final decision
	Reason           string     `json:"reason,omitempty"` // Human-readable reason if can't start
	AllowedDevices   []string   `json:"allowed_devices"`
	DurationMinutes  int        `json:"duration_minutes"`
}

// NewMovieTimeService creates a new movie time service
func NewMovieTimeService(
	storage MovieTimeStorage,
	deviceRegistry DeviceRegistry,
	driverRegistry DriverRegistry,
	cfg *config.MovieTimeConfig,
	timezone *time.Location,
	logger *slog.Logger,
) *MovieTimeService {
	if logger == nil {
		logger = slog.Default()
	}
	if timezone == nil {
		timezone = time.UTC
	}

	return &MovieTimeService{
		storage:        storage,
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
		config:         cfg,
		timezone:       timezone,
		logger:         logger,
	}
}

// GetAvailability returns the current movie time availability status
func (s *MovieTimeService) GetAvailability(ctx context.Context) (*MovieTimeAvailability, error) {
	now := time.Now().In(s.timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.timezone)

	result := &MovieTimeAvailability{
		IsWeekend:       s.isWeekend(now),
		DurationMinutes: s.config.GetDuration(),
		AllowedDevices:  s.config.AllowedDeviceIDs,
	}

	// Check for active bypass (allows movie time on non-weekends)
	bypasses, err := s.storage.ListActiveMovieTimeBypasses(ctx, today)
	if err != nil {
		s.logger.Warn("Failed to check movie time bypasses", "error", err)
		// Continue without bypass - fail safe
	} else if len(bypasses) > 0 {
		result.IsBypassActive = true
		result.BypassReason = bypasses[0].Reason // Use first active bypass reason
	}

	// Not a weekend and no bypass active
	if !result.IsWeekend && !result.IsBypassActive {
		result.Reason = "Movie time is only available on weekends"
		return result, nil
	}

	// Check if already used today
	usage, err := s.storage.GetMovieTimeUsage(ctx, today)
	if err != nil {
		return nil, err
	}

	if usage != nil && (usage.Status == MovieTimeStatusActive || usage.Status == MovieTimeStatusUsed) {
		result.IsUsedToday = true
		result.Reason = "Movie time already used today"
		return result, nil
	}

	// Check if there's an active movie session
	activeSessions, err := s.storage.ListActiveSessions(ctx)
	if err != nil {
		return nil, err
	}

	for _, session := range activeSessions {
		if session.IsMovieSession {
			result.IsUsedToday = true
			result.Reason = "A movie session is already active"
			return result, nil
		}
	}

	// Find last session end time for today
	lastSessionEnd := s.getLastSessionEndTime(ctx, activeSessions, today)
	result.LastSessionEnd = lastSessionEnd

	// Check break requirement
	if lastSessionEnd != nil {
		breakMinutes := s.config.GetBreakMinutes()
		breakEndTime := lastSessionEnd.Add(time.Duration(breakMinutes) * time.Minute)

		if now.Before(breakEndTime) {
			result.BreakRequired = true
			result.BreakMinutesLeft = int(time.Until(breakEndTime).Minutes())
			if result.BreakMinutesLeft < 0 {
				result.BreakMinutesLeft = 0
			}
			result.Reason = "Break period not yet completed"
			return result, nil
		}
	}

	// All checks passed
	result.IsAvailable = true
	result.CanStart = true

	return result, nil
}

// StartMovieTime starts a new movie time session
func (s *MovieTimeService) StartMovieTime(ctx context.Context, deviceID, initiatorChildID string) (*Session, error) {
	s.logger.Info("Starting movie time",
		"device_id", deviceID,
		"initiator_child_id", initiatorChildID)

	// Check availability first
	availability, err := s.GetAvailability(ctx)
	if err != nil {
		return nil, err
	}

	if !availability.CanStart {
		s.logger.Warn("Movie time not available",
			"reason", availability.Reason)
		// Return specific error based on availability
		if !availability.IsWeekend && !availability.IsBypassActive {
			return nil, ErrNotWeekend
		}
		if availability.IsUsedToday {
			return nil, ErrMovieTimeAlreadyUsed
		}
		if availability.BreakRequired {
			return nil, ErrBreakNotMet
		}
		return nil, ErrMovieTimeDisabled
	}

	// Validate device is allowed for movie time
	isAllowed := false
	for _, allowedID := range s.config.AllowedDeviceIDs {
		if allowedID == deviceID {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		s.logger.Warn("Device not allowed for movie time",
			"device_id", deviceID,
			"allowed_devices", s.config.AllowedDeviceIDs)
		return nil, ErrInvalidMovieDevice
	}

	// Look up device from device registry
	device, err := s.deviceRegistry.Get(deviceID)
	if err != nil {
		s.logger.Error("Failed to get device from registry",
			"device_id", deviceID,
			"error", err)
		return nil, err
	}

	// Get all children for shared session
	allChildren, err := s.storage.ListChildren(ctx)
	if err != nil {
		s.logger.Error("Failed to list children",
			"error", err)
		return nil, err
	}

	childIDs := make([]string, len(allChildren))
	for i, child := range allChildren {
		childIDs[i] = child.ID
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.timezone)

	// Create session with IsMovieSession flag
	session := &Session{
		ID:               idgen.NewSession(),
		DeviceType:       device.GetType(),
		DeviceID:         deviceID,
		ChildIDs:         childIDs,
		StartTime:        now,
		ExpectedDuration: s.config.GetDuration(),
		Status:           SessionStatusActive,
		IsMovieSession:   true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Get device driver
	driver, err := s.driverRegistry.Get(device.GetDriver())
	if err != nil {
		s.logger.Error("Failed to get driver from registry",
			"driver_name", device.GetDriver(),
			"device_id", deviceID,
			"error", err)
		return nil, err
	}

	// Save session first (fail-safe pattern)
	if err := s.storage.CreateSession(ctx, session); err != nil {
		s.logger.Error("Failed to save session to storage",
			"session_id", session.ID,
			"error", err)
		return nil, err
	}

	// Start session on device
	if err := driver.StartSession(ctx, session); err != nil {
		s.logger.Error("Driver failed to start session",
			"session_id", session.ID,
			"driver", driver.Name(),
			"error", err)
		// Note: We don't delete the session here since it's marked as movie session
		// The session will be cleaned up by scheduler if needed
		return nil, err
	}

	// Save movie time usage record
	usage := &MovieTimeUsage{
		Date:      today,
		SessionID: session.ID,
		StartedAt: &now,
		StartedBy: initiatorChildID,
		Status:    MovieTimeStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.storage.SaveMovieTimeUsage(ctx, usage); err != nil {
		s.logger.Error("Failed to save movie time usage",
			"session_id", session.ID,
			"error", err)
		// Don't fail the session - usage tracking is secondary
	}

	// Increment session count for all children
	for _, childID := range childIDs {
		if err := s.storage.IncrementSessionCountSummary(ctx, childID, today); err != nil {
			s.logger.Warn("Failed to increment session count summary",
				"session_id", session.ID,
				"child_id", childID,
				"error", err)
		}
	}

	s.logger.Info("Movie time started successfully",
		"session_id", session.ID,
		"device_id", deviceID,
		"child_ids", childIDs,
		"duration_minutes", session.ExpectedDuration)

	return session, nil
}

// MarkMovieTimeUsed marks movie time as used when the session ends
func (s *MovieTimeService) MarkMovieTimeUsed(ctx context.Context, sessionID string) error {
	now := time.Now().In(s.timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.timezone)

	usage, err := s.storage.GetMovieTimeUsage(ctx, today)
	if err != nil {
		return err
	}

	if usage == nil || usage.SessionID != sessionID {
		// No matching usage record
		return nil
	}

	usage.Status = MovieTimeStatusUsed
	usage.UpdatedAt = now

	return s.storage.SaveMovieTimeUsage(ctx, usage)
}

// isWeekend checks if the given time is on a weekend
func (s *MovieTimeService) isWeekend(t time.Time) bool {
	weekday := t.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// getLastSessionEndTime finds the end time of the last completed session today
func (s *MovieTimeService) getLastSessionEndTime(ctx context.Context, activeSessions []*Session, today time.Time) *time.Time {
	var lastEnd *time.Time

	// Check active sessions first - they haven't ended yet
	for _, session := range activeSessions {
		if session.IsMovieSession {
			continue // Skip movie sessions
		}
		// Active session means there's a session that hasn't ended
		// For break calculation, we use current time as "last session end" isn't applicable
		// until the session actually ends
	}

	// For simplicity, we'll just look at when the last non-movie active session started
	// and calculate when it would end based on expected duration
	// In a more complete implementation, we'd query completed sessions too
	for _, session := range activeSessions {
		if session.IsMovieSession {
			continue
		}
		// Session is active - it hasn't ended yet
		// The break period should start after it ends
		endTime := session.StartTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)
		if lastEnd == nil || endTime.After(*lastEnd) {
			lastEnd = &endTime
		}
	}

	return lastEnd
}

// IsEnabled returns whether movie time feature is enabled
func (s *MovieTimeService) IsEnabled() bool {
	return s.config != nil && s.config.Enabled
}
