package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleNewSessionFlow handles the multi-step flow for creating a new session
func (b *Bot) handleNewSessionFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 1:
		// Step 1: Child selected, show devices
		return b.newSessionStep2(ctx, message, data.ChildID)
	case 2:
		// Step 2: Device selected, show durations
		return b.newSessionStep3(ctx, message, data.ChildID, data.Device)
	case 3:
		// Step 3: Duration selected, create session
		return b.newSessionCreate(ctx, message, data.ChildID, data.Device, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in session creation flow.", nil)
	}
}

// newSessionStep2 shows device selection
func (b *Bot) newSessionStep2(ctx context.Context, message *tgbotapi.Message, childID string) error {
	// Get devices list
	devices, err := b.client.ListDevices(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	if len(devices) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No devices configured.", nil)
	}

	text := "➕ *New Session*\n\n📺 Step 2/3: Select device"
	keyboard := BuildDevicesButtons(devices, "newsession", 2, childID)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// newSessionStep3 shows duration selection
func (b *Bot) newSessionStep3(ctx context.Context, message *tgbotapi.Message, childID, device string) error {
	emoji := getDeviceEmoji(device)
	text := fmt.Sprintf("➕ *New Session*\n\n%s Device: *%s*\n\n⏱ Step 3/3: Select duration (minutes)",
		emoji, device)

	keyboard := BuildDurationButtons("newsession", 3, childID, device)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// newSessionCreate creates the session
func (b *Bot) newSessionCreate(ctx context.Context, message *tgbotapi.Message, childID, device string, duration int) error {
	// Get all children if "shared" was selected
	var childIDs []string

	if childID == "shared" {
		children, err := b.client.ListChildren(ctx)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
		}

		for _, child := range children {
			childIDs = append(childIDs, child.ID)
		}
	} else {
		childIDs = []string{childID}
	}

	// Create session request
	req := CreateSessionRequest{
		DeviceType: device,
		ChildIDs:   childIDs,
		Minutes:    duration,
	}

	session, err := b.client.CreateSession(ctx, req)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	// Get children for formatting
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := FormatSessionCreated(session, childrenMap)

	return b.editMessage(message.Chat.ID, message.MessageID, text, nil)
}

// handleExtendFlow handles the multi-step flow for extending a session
func (b *Bot) handleExtendFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 1:
		// Step 1: Session selected, show durations
		return b.extendStep2(ctx, message, data.Session)
	case 2:
		// Step 2: Duration selected, extend session
		return b.extendSession(ctx, message, data.Session, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in extend flow.", nil)
	}
}

// extendStep2 shows duration selection for extension
func (b *Bot) extendStep2(ctx context.Context, message *tgbotapi.Message, sessionID string) error {
	text := "⏱ *Extend Session*\n\nSelect additional minutes:"
	keyboard := BuildExtendDurationButtons(sessionID)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// extendSession extends the session
func (b *Bot) extendSession(ctx context.Context, message *tgbotapi.Message, sessionID string, additionalMinutes int) error {
	session, err := b.client.ExtendSession(ctx, sessionID, additionalMinutes)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	text := FormatSessionExtended(session, additionalMinutes)

	return b.editMessage(message.Chat.ID, message.MessageID, text, nil)
}
