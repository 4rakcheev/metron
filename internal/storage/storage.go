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

	// ============================================================================
	// Storage Methods - Refactored Architecture
	// ============================================================================

	// Daily Time Allocation - stores what time is available
	GetDailyAllocation(ctx context.Context, childID string, date time.Time) (*core.DailyTimeAllocation, error)
	CreateDailyAllocation(ctx context.Context, allocation *core.DailyTimeAllocation) error
	UpdateDailyAllocation(ctx context.Context, allocation *core.DailyTimeAllocation) error

	// Daily Usage Summary - stores what time was consumed
	GetDailyUsageSummary(ctx context.Context, childID string, date time.Time) (*core.DailyUsageSummary, error)
	IncrementDailyUsageSummary(ctx context.Context, childID string, date time.Time, minutes int) error
	IncrementSessionCountSummary(ctx context.Context, childID string, date time.Time) error

	// Session Usage Records - stores session history
	ListActiveSessionRecords(ctx context.Context) ([]*core.SessionUsageRecord, error)

	// Device Bypass - stores bypass mode for agent-controlled devices
	GetDeviceBypass(ctx context.Context, deviceID string) (*core.DeviceBypass, error)
	SetDeviceBypass(ctx context.Context, bypass *core.DeviceBypass) error
	ClearDeviceBypass(ctx context.Context, deviceID string) error
	ListActiveBypassDevices(ctx context.Context) ([]*core.DeviceBypass, error)

	// Movie Time Usage - stores weekend shared movie time usage
	GetMovieTimeUsage(ctx context.Context, date time.Time) (*core.MovieTimeUsage, error)
	SaveMovieTimeUsage(ctx context.Context, usage *core.MovieTimeUsage) error

	// Movie Time Bypass - stores bypass periods for holidays/vacations
	CreateMovieTimeBypass(ctx context.Context, bypass *core.MovieTimeBypass) error
	GetMovieTimeBypass(ctx context.Context, id string) (*core.MovieTimeBypass, error)
	ListMovieTimeBypasses(ctx context.Context) ([]*core.MovieTimeBypass, error)
	ListActiveMovieTimeBypasses(ctx context.Context, date time.Time) ([]*core.MovieTimeBypass, error)
	DeleteMovieTimeBypass(ctx context.Context, id string) error

	// Lifecycle
	Close() error
}
