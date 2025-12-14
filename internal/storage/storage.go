package storage

import (
	"context"
	"metron/internal/core"
	"time"
)

// Storage defines the interface for core data persistence
// Driver-specific storage needs (like Aqara tokens) should use separate interfaces
type Storage interface {
	// Children
	CreateChild(ctx context.Context, child *core.Child) error
	GetChild(ctx context.Context, id string) (*core.Child, error)
	ListChildren(ctx context.Context) ([]*core.Child, error)
	UpdateChild(ctx context.Context, child *core.Child) error
	DeleteChild(ctx context.Context, id string) error

	// Sessions
	CreateSession(ctx context.Context, session *core.Session) error
	GetSession(ctx context.Context, id string) (*core.Session, error)
	ListActiveSessions(ctx context.Context) ([]*core.Session, error)
	ListAllSessions(ctx context.Context) ([]*core.Session, error)
	ListSessionsByChild(ctx context.Context, childID string) ([]*core.Session, error)
	UpdateSession(ctx context.Context, session *core.Session) error
	DeleteSession(ctx context.Context, id string) error

	// Daily Usage
	GetDailyUsage(ctx context.Context, childID string, date time.Time) (*core.DailyUsage, error)
	UpdateDailyUsage(ctx context.Context, usage *core.DailyUsage) error
	IncrementDailyUsage(ctx context.Context, childID string, date time.Time, minutes int) error
	IncrementSessionCount(ctx context.Context, childID string, date time.Time) error

	// Lifecycle
	Close() error
}
