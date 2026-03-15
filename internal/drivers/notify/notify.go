// Package notify provides a device driver that sends Telegram notifications
// when sessions start, stop, or warn. Designed for devices managed by external
// apps (e.g., Google Family Link) where enforcement is manual.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"metron/internal/core"
	"metron/internal/devices"
)

const DriverName = "notify"

// ChildLookup resolves child IDs to child objects for name display.
type ChildLookup interface {
	GetChild(ctx context.Context, id string) (*core.Child, error)
}

// TelegramSender sends messages via Telegram Bot API.
type TelegramSender interface {
	SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error
}

// Config contains notify driver configuration.
type Config struct {
	TelegramToken string
	ChatIDs       []int64
}

// Driver implements the DeviceDriver interface by sending Telegram notifications.
type Driver struct {
	config         Config
	childLookup    ChildLookup
	deviceRegistry *devices.Registry
	sender         TelegramSender
	logger         *slog.Logger
}

// NewDriver creates a new notify driver.
func NewDriver(config Config, childLookup ChildLookup, deviceRegistry *devices.Registry, logger *slog.Logger) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Driver{
		config:         config,
		childLookup:    childLookup,
		deviceRegistry: deviceRegistry,
		sender:         newHTTPSender(config.TelegramToken),
		logger:         logger.With("driver", DriverName),
	}
}

// Name returns the driver name.
func (d *Driver) Name() string {
	return DriverName
}

// Capabilities returns the driver capabilities.
func (d *Driver) Capabilities() devices.DriverCapabilities {
	return devices.DriverCapabilities{
		SupportsWarnings:   true,
		SupportsLiveState:  false,
		SupportsScheduling: true,
	}
}

// StartSession sends a notification that a session has started.
func (d *Driver) StartSession(ctx context.Context, session *core.Session) error {
	device, err := d.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		d.logger.Error("Failed to get device", "device_id", session.DeviceID, "error", err)
		return nil
	}

	childNames := d.resolveChildNames(ctx, session.ChildIDs)
	appName := stringParam(device, "app_name", "app")
	appURL := stringParam(device, "app_url", "")
	deviceEmoji := device.Emoji
	if deviceEmoji == "" {
		deviceEmoji = "\U0001f4f1" // default phone emoji
	}

	endTime := session.StartTime.Add(time.Duration(session.ExpectedDuration) * time.Minute)

	isParentOverride := ctx.Value("parent_override") != nil

	var text string
	if isParentOverride {
		text = fmt.Sprintf(
			"%s *Session Started*\n\n%s %s \u2014 %d min on %s\n\U0001f3c1 Ends at: %s\n\nDon't forget to grant time in %s.",
			deviceEmoji,
			childEmojis(childNames),
			joinNames(childNames),
			session.ExpectedDuration,
			device.Name,
			endTime.Format("15:04"),
			appName,
		)
	} else {
		text = fmt.Sprintf(
			"%s *Session Request*\n\n%s %s requested %d min on %s\n\n\u23f1 Duration: %d min\n\U0001f3c1 Ends at: %s\n\nPlease grant time in %s.",
			deviceEmoji,
			childEmojis(childNames),
			joinNames(childNames),
			session.ExpectedDuration,
			device.Name,
			session.ExpectedDuration,
			endTime.Format("15:04"),
			appName,
		)
	}

	var replyMarkup interface{}
	if appURL != "" {
		replyMarkup = inlineURLButton(fmt.Sprintf("\U0001f517 Open %s", appName), appURL)
	}

	d.broadcast(ctx, text, replyMarkup)
	return nil
}

// StopSession sends a notification that a session has ended.
func (d *Driver) StopSession(ctx context.Context, session *core.Session) error {
	device, err := d.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		d.logger.Error("Failed to get device", "device_id", session.DeviceID, "error", err)
		return nil
	}

	childNames := d.resolveChildNames(ctx, session.ChildIDs)
	appName := stringParam(device, "app_name", "app")
	appURL := stringParam(device, "app_url", "")
	deviceEmoji := device.Emoji
	if deviceEmoji == "" {
		deviceEmoji = "\U0001f4f1"
	}

	usedMinutes := int(time.Since(session.StartTime).Minutes())

	text := fmt.Sprintf(
		"%s *Session Ended*\n\n%s %s \u2014 %s (%d min used)\n\nRevoke bonus time in %s.",
		deviceEmoji,
		childEmojis(childNames),
		joinNames(childNames),
		device.Name,
		usedMinutes,
		appName,
	)

	var replyMarkup interface{}
	if appURL != "" {
		replyMarkup = inlineURLButton(fmt.Sprintf("\U0001f517 Open %s", appName), appURL)
	}

	d.broadcast(ctx, text, replyMarkup)
	return nil
}

// ApplyWarning sends a time-remaining warning notification.
func (d *Driver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	device, err := d.deviceRegistry.Get(session.DeviceID)
	if err != nil {
		d.logger.Error("Failed to get device", "device_id", session.DeviceID, "error", err)
		return nil
	}

	childNames := d.resolveChildNames(ctx, session.ChildIDs)

	text := fmt.Sprintf(
		"\u23f1 %d min remaining \u2014 %s %s on %s",
		minutesRemaining,
		childEmojis(childNames),
		joinNames(childNames),
		device.Name,
	)

	d.broadcast(ctx, text, nil)
	return nil
}

// GetLiveState is not supported by the notify driver.
func (d *Driver) GetLiveState(ctx context.Context, deviceID string) (*devices.DeviceState, error) {
	return nil, nil
}

// broadcast sends a message to all configured chat IDs. Errors are logged but never returned.
func (d *Driver) broadcast(ctx context.Context, text string, replyMarkup interface{}) {
	for _, chatID := range d.config.ChatIDs {
		if err := d.sender.SendMessage(ctx, chatID, text, replyMarkup); err != nil {
			d.logger.Error("Failed to send Telegram notification",
				"chat_id", chatID,
				"error", err)
		}
	}
}

// resolveChildNames looks up child names from IDs, falling back to ID on error.
func (d *Driver) resolveChildNames(ctx context.Context, childIDs []string) []childInfo {
	names := make([]childInfo, 0, len(childIDs))
	for _, id := range childIDs {
		child, err := d.childLookup.GetChild(ctx, id)
		if err != nil {
			d.logger.Warn("Failed to resolve child name", "child_id", id, "error", err)
			names = append(names, childInfo{Name: id})
			continue
		}
		names = append(names, childInfo{Name: child.Name, Emoji: child.Emoji})
	}
	return names
}

type childInfo struct {
	Name  string
	Emoji string
}

// childEmojis returns the emoji(s) for children, or a default.
func childEmojis(children []childInfo) string {
	var parts []string
	for _, c := range children {
		if c.Emoji != "" {
			parts = append(parts, c.Emoji)
		}
	}
	if len(parts) == 0 {
		return "\U0001f9d2" // default child emoji
	}
	return strings.Join(parts, "")
}

// joinNames joins child names with commas.
func joinNames(children []childInfo) string {
	names := make([]string, len(children))
	for i, c := range children {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}

// stringParam reads a string parameter from device config with a default.
func stringParam(device *devices.Device, key, defaultVal string) string {
	if v, ok := device.GetParameter(key).(string); ok && v != "" {
		return v
	}
	return defaultVal
}

// inlineURLButton creates a Telegram inline keyboard with a single URL button.
func inlineURLButton(text, url string) map[string]interface{} {
	return map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": text, "url": url},
			},
		},
	}
}

// Ensure Driver implements the interfaces.
var (
	_ devices.DeviceDriver  = (*Driver)(nil)
	_ devices.CapableDriver = (*Driver)(nil)
)
