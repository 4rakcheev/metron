package core

import (
	"context"
	"fmt"
	"time"
)

// LockdownAll is the wildcard value for LockdownRule.ChildID and LockdownRule.DeviceID.
// A wildcard (not NULL) is used so the (child_id, device_id) UNIQUE constraint works in SQLite.
const LockdownAll = "*"

// Lockdown durations accepted when creating a rule
const (
	LockdownUntilManual   = "manual"     // stays until a parent removes it
	LockdownUntilEndOfDay = "end_of_day" // expires at the next local midnight
)

// LockdownRule blocks sessions for a child (or all children) on a device (or all devices).
// While active, no matching session can be started, extended or joined by anyone
// (child web app, Telegram bot or direct API), and matching active sessions are ended.
// Daily time quotas are preserved.
// Note: the API wire format is built by the handler, so this struct carries no JSON tags.
type LockdownRule struct {
	ID        int64
	ChildID   string // child ID or LockdownAll
	DeviceID  string // device ID or LockdownAll
	Reason    string
	EnabledAt time.Time
	EnabledBy string
	ExpiresAt *time.Time // nil = until manually removed
}

// IsActive reports whether the rule is in force at the given time
func (r *LockdownRule) IsActive(now time.Time) bool {
	return r.ExpiresAt == nil || now.Before(*r.ExpiresAt)
}

// IsGlobal reports whether the rule blocks all children on all devices
func (r *LockdownRule) IsGlobal() bool {
	return r.ChildID == LockdownAll && r.DeviceID == LockdownAll
}

// CoversChild reports whether the rule applies to the given child
func (r *LockdownRule) CoversChild(childID string) bool {
	return r.ChildID == LockdownAll || r.ChildID == childID
}

// CoversDevice reports whether the rule applies to the given device
func (r *LockdownRule) CoversDevice(deviceID string) bool {
	return r.DeviceID == LockdownAll || r.DeviceID == deviceID
}

// Matches reports whether the rule blocks a session on deviceID with any of childIDs
func (r *LockdownRule) Matches(deviceID string, childIDs []string) bool {
	if !r.CoversDevice(deviceID) {
		return false
	}
	for _, childID := range childIDs {
		if r.CoversChild(childID) {
			return true
		}
	}
	return false
}

// Describe returns a short human-readable scope, e.g. "child simon on device ps51"
func (r *LockdownRule) Describe() string {
	child := "all children"
	if r.ChildID != LockdownAll {
		child = "child " + r.ChildID
	}
	device := "all devices"
	if r.DeviceID != LockdownAll {
		device = "device " + r.DeviceID
	}
	return fmt.Sprintf("%s on %s", child, device)
}

// FindMatchingLockdown returns the first active rule matching the session scope, or nil
func FindMatchingLockdown(rules []*LockdownRule, now time.Time, deviceID string, childIDs []string) *LockdownRule {
	for _, r := range rules {
		if r.IsActive(now) && r.Matches(deviceID, childIDs) {
			return r
		}
	}
	return nil
}

// LockdownExpiry computes ExpiresAt for the given "until" value.
// Returns (nil, nil) for manual rules and an error for unknown values.
func LockdownExpiry(until string, now time.Time, tz *time.Location) (*time.Time, error) {
	switch until {
	case "", LockdownUntilManual:
		return nil, nil
	case LockdownUntilEndOfDay:
		if tz == nil {
			tz = time.UTC
		}
		local := now.In(tz)
		midnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, tz)
		return &midnight, nil
	default:
		return nil, fmt.Errorf("invalid lockdown duration %q (use %q or %q)", until, LockdownUntilManual, LockdownUntilEndOfDay)
	}
}

// LockdownStorage defines the interface for lockdown rule persistence
type LockdownStorage interface {
	// ListActiveLockdowns returns rules that are active at now (expired rules are excluded)
	ListActiveLockdowns(ctx context.Context, now time.Time) ([]*LockdownRule, error)
	// AddLockdown creates a rule, or replaces the existing rule with the same (child, device) scope.
	// On return rule.ID is set.
	AddLockdown(ctx context.Context, rule *LockdownRule) error
	// RemoveLockdown deletes a rule by ID. Returns ErrLockdownNotFound if it does not exist.
	RemoveLockdown(ctx context.Context, id int64) error
}
