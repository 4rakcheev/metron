package core

import "time"

// DeviceBypass represents a temporary bypass of screen-time enforcement for a device.
// When enabled, the associated agent will allow access without requiring an active session.
type DeviceBypass struct {
	DeviceID  string     `json:"device_id"`            // Device this bypass applies to
	Enabled   bool       `json:"enabled"`              // Whether bypass is currently active
	Reason    string     `json:"reason,omitempty"`     // Optional reason (e.g., "homework", "special occasion")
	EnabledAt time.Time  `json:"enabled_at"`           // When the bypass was enabled
	EnabledBy string     `json:"enabled_by,omitempty"` // Who enabled it (e.g., Telegram user ID)
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // When it expires (nil = indefinite)
}

// IsExpired returns true if the bypass has expired
func (b *DeviceBypass) IsExpired() bool {
	if b.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*b.ExpiresAt)
}

// IsActive returns true if the bypass is enabled and not expired
func (b *DeviceBypass) IsActive() bool {
	return b.Enabled && !b.IsExpired()
}

// RemainingTime returns the time remaining until expiration, or nil if indefinite
func (b *DeviceBypass) RemainingTime() *time.Duration {
	if b.ExpiresAt == nil {
		return nil
	}
	remaining := time.Until(*b.ExpiresAt)
	if remaining < 0 {
		remaining = 0
	}
	return &remaining
}
