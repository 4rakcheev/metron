package logging

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"time"
)

// SessionManagerLogger wraps a SessionManager and logs all method calls
type SessionManagerLogger struct {
	manager core.SessionManagerInterface
	logger  *slog.Logger
}

// NewSessionManagerLogger creates a new logging decorator for SessionManager
func NewSessionManagerLogger(manager core.SessionManagerInterface, logger *slog.Logger) core.SessionManagerInterface {
	return &SessionManagerLogger{
		manager: manager,
		logger:  logger.With("interface", "SessionManager"),
	}
}

func (l *SessionManagerLogger) StartSession(ctx context.Context, deviceID string, childIDs []string, durationMinutes int) (*core.Session, error) {
	start := time.Now()
	l.logger.Info("StartSession called",
		"device_id", deviceID,
		"child_ids", childIDs,
		"duration_minutes", durationMinutes)

	session, err := l.manager.StartSession(ctx, deviceID, childIDs, durationMinutes)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("StartSession failed",
			"device_id", deviceID,
			"child_ids", childIDs,
			"duration_minutes", durationMinutes,
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Info("StartSession completed",
		"device_id", deviceID,
		"child_ids", childIDs,
		"duration_minutes", durationMinutes,
		"session_id", session.ID,
		"duration", duration)

	return session, nil
}

func (l *SessionManagerLogger) StopSession(ctx context.Context, sessionID string) error {
	start := time.Now()
	l.logger.Info("StopSession called",
		"session_id", sessionID)

	err := l.manager.StopSession(ctx, sessionID)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("StopSession failed",
			"session_id", sessionID,
			"duration", duration,
			"error", err)
		return err
	}

	l.logger.Info("StopSession completed",
		"session_id", sessionID,
		"duration", duration)

	return nil
}

func (l *SessionManagerLogger) ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*core.Session, error) {
	start := time.Now()
	l.logger.Info("ExtendSession called",
		"session_id", sessionID,
		"additional_minutes", additionalMinutes)

	session, err := l.manager.ExtendSession(ctx, sessionID, additionalMinutes)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("ExtendSession failed",
			"session_id", sessionID,
			"additional_minutes", additionalMinutes,
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Info("ExtendSession completed",
		"session_id", sessionID,
		"additional_minutes", additionalMinutes,
		"new_duration", session.ExpectedDuration,
		"duration", duration)

	return session, nil
}

func (l *SessionManagerLogger) AddChildrenToSession(ctx context.Context, sessionID string, childIDs []string) (*core.Session, error) {
	start := time.Now()
	l.logger.Info("AddChildrenToSession called",
		"session_id", sessionID,
		"child_ids", childIDs)

	session, err := l.manager.AddChildrenToSession(ctx, sessionID, childIDs)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("AddChildrenToSession failed",
			"session_id", sessionID,
			"child_ids", childIDs,
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Info("AddChildrenToSession completed",
		"session_id", sessionID,
		"child_ids", childIDs,
		"total_children", len(session.ChildIDs),
		"duration", duration)

	return session, nil
}

func (l *SessionManagerLogger) GetSession(ctx context.Context, sessionID string) (*core.Session, error) {
	start := time.Now()
	l.logger.Debug("GetSession called",
		"session_id", sessionID)

	session, err := l.manager.GetSession(ctx, sessionID)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("GetSession failed",
			"session_id", sessionID,
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Debug("GetSession completed",
		"session_id", sessionID,
		"status", session.Status,
		"duration", duration)

	return session, nil
}

func (l *SessionManagerLogger) ListActiveSessions(ctx context.Context) ([]*core.Session, error) {
	start := time.Now()
	l.logger.Debug("ListActiveSessions called")

	sessions, err := l.manager.ListActiveSessions(ctx)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("ListActiveSessions failed",
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Debug("ListActiveSessions completed",
		"count", len(sessions),
		"duration", duration)

	return sessions, nil
}

func (l *SessionManagerLogger) GrantRewardMinutes(ctx context.Context, childID string, minutes int) error {
	start := time.Now()
	l.logger.Info("GrantRewardMinutes called",
		"child_id", childID,
		"minutes", minutes)

	err := l.manager.GrantRewardMinutes(ctx, childID, minutes)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("GrantRewardMinutes failed",
			"child_id", childID,
			"minutes", minutes,
			"duration", duration,
			"error", err)
		return err
	}

	l.logger.Info("GrantRewardMinutes completed",
		"child_id", childID,
		"minutes", minutes,
		"duration", duration)

	return nil
}

func (l *SessionManagerLogger) GetChildStatus(ctx context.Context, childID string) (*core.ChildStatus, error) {
	start := time.Now()
	l.logger.Debug("GetChildStatus called",
		"child_id", childID)

	status, err := l.manager.GetChildStatus(ctx, childID)
	duration := time.Since(start)

	if err != nil {
		l.logger.Error("GetChildStatus failed",
			"child_id", childID,
			"duration", duration,
			"error", err)
		return nil, err
	}

	l.logger.Debug("GetChildStatus completed",
		"child_id", childID,
		"today_used", status.TodayUsed,
		"today_remaining", status.TodayRemaining,
		"today_limit", status.TodayLimit,
		"duration", duration)

	return status, nil
}
