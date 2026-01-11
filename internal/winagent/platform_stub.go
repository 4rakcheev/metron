//go:build !windows

package winagent

import (
	"errors"
	"log/slog"
)

var ErrNotWindows = errors.New("this operation is only supported on Windows")

// StubPlatform implements Platform for non-Windows platforms.
// It logs actions but cannot perform actual workstation control.
type StubPlatform struct {
	logger *slog.Logger
}

// NewStubPlatform creates a new stub platform implementation
func NewStubPlatform(logger *slog.Logger) *StubPlatform {
	return &StubPlatform{
		logger: logger.With("component", "platform-stub"),
	}
}

// LockWorkstation logs the lock attempt but returns an error on non-Windows
func (p *StubPlatform) LockWorkstation() error {
	p.logger.Warn("LockWorkstation called on non-Windows platform")
	return ErrNotWindows
}

// ShowWarningNotification logs the notification but returns an error on non-Windows
func (p *StubPlatform) ShowWarningNotification(title, message string) error {
	p.logger.Warn("ShowWarningNotification called on non-Windows platform",
		"title", title,
		"message", message,
	)
	return ErrNotWindows
}

// NewPlatform creates a new platform implementation for the current OS
func NewPlatform(logger *slog.Logger) Platform {
	return NewStubPlatform(logger)
}

// Ensure StubPlatform implements Platform
var _ Platform = (*StubPlatform)(nil)
