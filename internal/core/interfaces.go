package core

import "context"

// SessionManagerInterface defines the contract for session management
type SessionManagerInterface interface {
	StartSession(ctx context.Context, deviceID string, childIDs []string, durationMinutes int) (*Session, error)
	StopSession(ctx context.Context, sessionID string) error
	ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*Session, error)
	AddChildrenToSession(ctx context.Context, sessionID string, childIDs []string) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListActiveSessions(ctx context.Context) ([]*Session, error)
	GrantRewardMinutes(ctx context.Context, childID string, minutes int) error
	GetChildStatus(ctx context.Context, childID string) (*ChildStatus, error)
}
