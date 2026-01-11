package winagent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// lockDebounce prevents spamming lock calls
	lockDebounce = 5 * time.Second
)

// EnforcerState tracks the current enforcement state
type EnforcerState struct {
	LastSessionID      *string    // Last known session ID
	WarningSent        bool       // Whether warning was sent for current session
	LastLockTime       *time.Time // When we last locked (debounce)
	LastSuccessfulPoll *time.Time // For network error grace period
	NetworkErrorSince  *time.Time // When network errors started
}

// Enforcer manages the enforcement loop
type Enforcer struct {
	client   MetronClient
	platform Platform
	clock    Clock
	config   *Config
	state    EnforcerState
	logger   *slog.Logger
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewEnforcer creates a new enforcer
func NewEnforcer(client MetronClient, platform Platform, clock Clock, config *Config, logger *slog.Logger) *Enforcer {
	return &Enforcer{
		client:   client,
		platform: platform,
		clock:    clock,
		config:   config,
		state:    EnforcerState{},
		logger:   logger.With("component", "enforcer"),
		stopChan: make(chan struct{}),
	}
}

// Start begins the enforcement loop (blocking)
func (e *Enforcer) Start(ctx context.Context) {
	e.logger.Info("starting enforcement loop",
		"device_id", e.config.DeviceID,
		"poll_interval", e.config.PollInterval,
		"grace_period", e.config.GracePeriod,
	)

	ticker := e.clock.NewTicker(e.config.PollInterval)
	defer ticker.Stop()

	// Do an initial poll immediately
	e.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("enforcement loop stopped (context cancelled)")
			return
		case <-e.stopChan:
			e.logger.Info("enforcement loop stopped")
			return
		case <-ticker.C:
			e.poll(ctx)
		}
	}
}

// Stop signals the enforcer to stop
func (e *Enforcer) Stop() {
	close(e.stopChan)
}

// poll performs a single poll and processes the result
func (e *Enforcer) poll(ctx context.Context) {
	e.logger.Debug("polling session status")

	status, err := e.client.GetSessionStatus(ctx, e.config.DeviceID)
	if err != nil {
		e.handleNetworkError(err)
		return
	}

	e.processStatus(status)
}

// processStatus handles a successful poll result
func (e *Enforcer) processStatus(status *SessionStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.clock.Now()

	// Clear network error state on successful poll
	e.state.LastSuccessfulPoll = &now
	e.state.NetworkErrorSince = nil

	// Check bypass mode first - no enforcement needed
	if status.BypassMode {
		e.logger.Debug("bypass mode active, skipping enforcement")
		return
	}

	// No active session - lock
	if !status.Active {
		e.logger.Info("no active session, locking workstation")
		e.tryLock(now)
		return
	}

	// Active session - check if it's a new session
	sessionID := ""
	if status.SessionID != nil {
		sessionID = *status.SessionID
	}

	if e.state.LastSessionID == nil || *e.state.LastSessionID != sessionID {
		// New session detected
		e.logger.Info("new session detected",
			"session_id", sessionID,
			"ends_at", status.EndsAt,
		)
		e.state.LastSessionID = &sessionID
		e.state.WarningSent = false
	}

	// Check if session has expired (time passed ends_at)
	if status.EndsAt != nil && now.After(*status.EndsAt) {
		e.logger.Info("session expired, locking workstation",
			"session_id", sessionID,
			"ends_at", status.EndsAt,
		)
		e.tryLock(now)
		return
	}

	// Check if warning should be shown (within 5 minutes of end)
	if !e.state.WarningSent && status.WarnAt != nil {
		if now.After(*status.WarnAt) || now.Equal(*status.WarnAt) {
			remaining := time.Duration(0)
			if status.EndsAt != nil {
				remaining = status.EndsAt.Sub(now)
			}
			e.logger.Info("warning threshold reached",
				"session_id", sessionID,
				"remaining", remaining.Round(time.Minute),
			)
			e.showWarning(int(remaining.Minutes()))
			e.state.WarningSent = true
		}
	}

	// Active session, not expired, no warning needed - allow usage
	e.logger.Debug("active session, allowing usage",
		"session_id", sessionID,
		"remaining", status.EndsAt.Sub(now).Round(time.Second),
	)
}

// handleNetworkError implements fail-closed with grace period
func (e *Enforcer) handleNetworkError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.clock.Now()

	e.logger.Warn("network error polling session status", "error", err)

	// Track when errors started
	if e.state.NetworkErrorSince == nil {
		e.state.NetworkErrorSince = &now
	}

	// Check if we're still within grace period
	errorDuration := now.Sub(*e.state.NetworkErrorSince)
	if errorDuration < e.config.GracePeriod {
		// Within grace period - check if we had a recent successful poll
		if e.state.LastSuccessfulPoll != nil {
			timeSinceSuccess := now.Sub(*e.state.LastSuccessfulPoll)
			if timeSinceSuccess < e.config.GracePeriod {
				e.logger.Debug("within grace period, continuing",
					"error_duration", errorDuration,
					"since_last_success", timeSinceSuccess,
				)
				return
			}
		}
	}

	// Grace period exceeded - fail closed (lock)
	e.logger.Warn("grace period exceeded, locking workstation (fail-closed)",
		"error_duration", errorDuration,
		"grace_period", e.config.GracePeriod,
	)
	e.tryLock(now)
}

// tryLock attempts to lock the workstation with debouncing
func (e *Enforcer) tryLock(now time.Time) {
	// Check debounce
	if e.state.LastLockTime != nil {
		timeSinceLock := now.Sub(*e.state.LastLockTime)
		if timeSinceLock < lockDebounce {
			e.logger.Debug("lock debounced",
				"time_since_last", timeSinceLock,
				"debounce", lockDebounce,
			)
			return
		}
	}

	// Attempt to lock
	if err := e.platform.LockWorkstation(); err != nil {
		e.logger.Error("failed to lock workstation", "error", err)
		return
	}

	e.state.LastLockTime = &now
}

// showWarning displays a warning notification
func (e *Enforcer) showWarning(minutesRemaining int) {
	title := "Screen Time Warning"
	var message string
	if minutesRemaining <= 1 {
		message = "Less than 1 minute remaining!"
	} else {
		message = fmt.Sprintf("%d minutes remaining", minutesRemaining)
	}

	if err := e.platform.ShowWarningNotification(title, message); err != nil {
		e.logger.Error("failed to show warning notification", "error", err)
	}
}

// GetState returns a copy of the current state (for testing/debugging)
func (e *Enforcer) GetState() EnforcerState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}
