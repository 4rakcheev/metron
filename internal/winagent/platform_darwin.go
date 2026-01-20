//go:build darwin

package winagent

import (
	"log/slog"
)

// DarwinPlatform implements Platform for macOS (for debugging purposes).
// It logs actions instead of performing actual workstation control.
type DarwinPlatform struct {
	logger *slog.Logger
}

// NewDarwinPlatform creates a new macOS platform implementation
func NewDarwinPlatform(logger *slog.Logger) *DarwinPlatform {
	return &DarwinPlatform{
		logger: logger.With("component", "platform-darwin"),
	}
}

// LockWorkstation logs a lock action for debugging purposes
func (p *DarwinPlatform) LockWorkstation() error {
	p.logger.Warn("LOCK_WORKSTATION",
		"action", "lock",
		"platform", "darwin",
		"note", "debug mode - no actual lock performed",
	)
	return nil
}

// ShowWarningNotification logs a warning notification for debugging purposes
// On Windows, this would play a 3-beep warning sound pattern
func (p *DarwinPlatform) ShowWarningNotification(title, message string) error {
	p.logger.Warn("WARNING_SOUND",
		"action", "warn",
		"platform", "darwin",
		"title", title,
		"message", message,
		"note", "debug mode - on Windows would play 3-beep warning sound",
	)
	return nil
}

// NewPlatform creates a new platform implementation for the current OS
func NewPlatform(logger *slog.Logger) Platform {
	return NewDarwinPlatform(logger)
}

// Ensure DarwinPlatform implements Platform
var _ Platform = (*DarwinPlatform)(nil)
