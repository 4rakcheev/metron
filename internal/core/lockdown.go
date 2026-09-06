package core

import (
	"context"
	"time"
)

// LockdownState represents the global lockdown flag.
// When enabled, no session can be started or extended by anyone
// until lockdown is manually disabled. Daily time quotas are preserved.
// Note: the API wire format is built by the handler (explicit nulls),
// so this struct intentionally carries no JSON tags.
type LockdownState struct {
	Enabled   bool
	EnabledAt *time.Time
	EnabledBy string
}

// LockdownStorage defines the interface for lockdown state persistence
type LockdownStorage interface {
	GetLockdown(ctx context.Context) (*LockdownState, error)
	SetLockdown(ctx context.Context, enabled bool, enabledBy string) error
}
