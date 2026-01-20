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

// ShowWarningNotification plays a warning sound to alert the user
// that screen time is running out. Uses Windows Beep API to play
// a distinctive tone pattern without interrupting the current activity.
// The melody plays in a goroutine to avoid blocking the enforcer loop.
func (p *WindowsPlatform) ShowWarningNotification(title, message string) error {
	p.logger.Warn("screen time warning",
		"title", title,
		"message", message,
	)

	// Play warning sound in a goroutine to avoid blocking the enforcer loop
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("panic in warning melody", "recover", r)
			}
		}()

		p.logger.Info("starting warning melody")
		if err := p.playWarningSound(); err != nil {
			p.logger.Error("failed to play warning sound", "error", err)
		}
	}()

	return nil
}

// playWarningSound plays a gentle melody to alert the user (~4-5 seconds)
// Plays through both audio output and attempts to use motherboard PC speaker
func (p *WindowsPlatform) playWarningSound() error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	beep := kernel32.NewProc("Beep")
	sleep := kernel32.NewProc("Sleep")

	// Musical notes frequencies (Hz) - C major scale for pleasant sound
	// C4=262, D4=294, E4=330, F4=349, G4=392, A4=440, B4=494, C5=523
	const (
		C4 = 262
		D4 = 294
		E4 = 330
		F4 = 349
		G4 = 392
		A4 = 440
		B4 = 494
		C5 = 523
		D5 = 587
		E5 = 659
	)

	// Gentle reminder melody (~4.5 seconds)
	// Sounds like a friendly "time to wrap up" chime
	melody := []struct {
		freq     uint32
		duration uint32
	}{
		// Opening phrase - ascending
		{C5, 200},
		{0, 50},
		{E5, 200},
		{0, 50},
		{G4, 300},
		{0, 150},

		// Middle phrase - descending
		{E5, 200},
		{0, 50},
		{D5, 200},
		{0, 50},
		{C5, 300},
		{0, 150},

		// Repeat opening - creates recognition
		{C5, 200},
		{0, 50},
		{E5, 200},
		{0, 50},
		{G4, 300},
		{0, 150},

		// Closing phrase - resolves nicely
		{G4, 150},
		{0, 50},
		{E5, 150},
		{0, 50},
		{C5, 500}, // Long final note
	}

	// Play melody through audio output
	for _, tone := range melody {
		if tone.freq == 0 {
			sleep.Call(uintptr(tone.duration))
		} else {
			beep.Call(uintptr(tone.freq), uintptr(tone.duration))
		}
	}

	// Also try to trigger motherboard PC speaker (may not work on all systems)
	// The PC speaker on modern Windows is often disabled, but worth trying
	p.tryPCSpeakerBeep()

	p.logger.Info("warning melody played")
	return nil
}

// tryPCSpeakerBeep attempts to beep through the motherboard speaker
// This uses MessageBeep which may trigger the PC speaker on some systems
func (p *WindowsPlatform) tryPCSpeakerBeep() {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBeep := user32.NewProc("MessageBeep")

	// MB_ICONEXCLAMATION (0x30) - system exclamation sound
	// This may trigger PC speaker on systems where it's enabled
	messageBeep.Call(uintptr(0x30))
}

// NewPlatform creates a new platform implementation for the current OS
func NewPlatform(logger *slog.Logger) Platform {
	return NewWindowsPlatform(logger)
}

// Ensure WindowsPlatform implements Platform
var _ Platform = (*WindowsPlatform)(nil)
