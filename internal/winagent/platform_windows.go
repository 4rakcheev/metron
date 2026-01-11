//go:build windows

package winagent

import (
	"errors"
	"log/slog"
	"syscall"
)

// WindowsPlatform implements Platform for Windows
type WindowsPlatform struct {
	logger *slog.Logger
}

// NewWindowsPlatform creates a new Windows platform implementation
func NewWindowsPlatform(logger *slog.Logger) *WindowsPlatform {
	return &WindowsPlatform{
		logger: logger.With("component", "platform"),
	}
}

var ErrLockFailed = errors.New("LockWorkStation failed")

// LockWorkstation locks the Windows workstation using user32.dll
func (p *WindowsPlatform) LockWorkstation() error {
	user32 := syscall.NewLazyDLL("user32.dll")
	lockWorkStation := user32.NewProc("LockWorkStation")

	ret, _, err := lockWorkStation.Call()
	if ret == 0 {
		// LockWorkStation returns 0 on failure
		p.logger.Error("failed to lock workstation", "error", err)
		return ErrLockFailed
	}

	p.logger.Info("workstation locked")
	return nil
}

// ShowWarningNotification displays a Windows toast notification
// For MVP, this logs the warning. In production, use a proper toast library
// like github.com/go-toast/toast or execute PowerShell to show native toast.
func (p *WindowsPlatform) ShowWarningNotification(title, message string) error {
	// Log the warning - this will be visible in the agent's log file
	// The actual notification will appear in logs; in a future version,
	// implement proper Windows toast notifications
	p.logger.Warn("screen time warning",
		"title", title,
		"message", message,
	)

	// TODO: Implement Windows toast notification using one of:
	// 1. PowerShell: exec.Command("powershell", "-Command", script)
	// 2. go-toast: github.com/go-toast/toast
	// 3. beeep: github.com/gen2brain/beeep
	return nil
}

// NewPlatform creates a new platform implementation for the current OS
func NewPlatform(logger *slog.Logger) Platform {
	return NewWindowsPlatform(logger)
}

// Ensure WindowsPlatform implements Platform
var _ Platform = (*WindowsPlatform)(nil)
