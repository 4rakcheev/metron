package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleNewSessionFlow handles the multi-step flow for creating a new session
func (b *Bot) handleNewSessionFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("New session flow",
		"step", data.Step,
		"child_index", data.ChildIndex,
		"device", data.Device,
		"duration", data.Duration,
	)

	switch data.Step {
	case 0:
		// Step 0: Back to child selection (from device selection)
		return b.newSessionStep1(ctx, message)
	case 1:
		// Step 1: Child selected (by index), show devices
		// Keep using index to avoid long callback_data
		return b.newSessionStep2(ctx, message, data.ChildIndex)
	case 2:
		// Step 2: Device selected, show durations
		return b.newSessionStep3(ctx, message, data.ChildIndex, data.Device)
	case 3:
		// Step 3: Duration selected, resolve index to ID and create session
		childID, err := b.resolveChildIndex(ctx, data.ChildIndex)
		if err != nil {
			b.logger.Error("Failed to resolve child index",
				"child_index", data.ChildIndex,
				"error", err,
			)
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}
		return b.newSessionCreate(ctx, message, childID, data.Device, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in session creation flow.", nil)
	}
}

// resolveChildIndex resolves a child index to a child ID
// Index -1 is a special marker for "shared" (all children)
func (b *Bot) resolveChildIndex(ctx context.Context, index int) (string, error) {
	// Special case: index -1 means "shared"
	if index == -1 {
		return "shared", nil
	}

	// Fetch children list to resolve index
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return "", err
	}

	if index < 0 || index >= len(children) {
		return "", fmt.Errorf("invalid child index: %d", index)
	}

	return children[index].ID, nil
}

// newSessionStep1 shows child selection
func (b *Bot) newSessionStep1(ctx context.Context, message *tgbotapi.Message) error {
	// Get children list
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	if len(children) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No children configured. Add children first using the API.", nil)
	}

	text := "➕ *New Session*\n\n👶 Step 1/3: Select child(ren)"
	keyboard := BuildChildrenButtons(children, "newsession", 1)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// newSessionStep2 shows device selection
func (b *Bot) newSessionStep2(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
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
	keyboard := BuildDevicesButtons(devices, "newsession", 2, childIndex)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// newSessionStep3 shows duration selection
func (b *Bot) newSessionStep3(ctx context.Context, message *tgbotapi.Message, childIndex int, device string) error {
	emoji := getDeviceEmoji(device)
	text := fmt.Sprintf("➕ *New Session*\n\n%s Device: *%s*\n\n⏱ Step 3/3: Select duration (minutes)",
		emoji, device)

	keyboard := BuildDurationButtons("newsession", 3, childIndex, device)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// newSessionCreate creates the session
func (b *Bot) newSessionCreate(ctx context.Context, message *tgbotapi.Message, childID, device string, duration int) error {
	// Get all children if "shared" was selected
	var childIDs []string

	if childID == "shared" {
		children, err := b.client.ListChildren(ctx)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}

		for _, child := range children {
			childIDs = append(childIDs, child.ID)
		}
	} else {
		childIDs = []string{childID}
	}

	// Create session request
	req := CreateSessionRequest{
		DeviceID: device, // device parameter now holds device ID
		ChildIDs: childIDs,
		Minutes:  duration,
	}

	session, err := b.client.CreateSession(ctx, req)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get children for formatting
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := FormatSessionCreated(session, childrenMap)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}

// handleExtendFlow handles the multi-step flow for extending a session
func (b *Bot) handleExtendFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 0:
		// Step 0: Back to session selection (from duration selection)
		return b.extendStep1(ctx, message)
	case 1:
		// Step 1: Session selected (by index), show durations
		// Keep the session index for the next step
		return b.extendStep2(ctx, message, data.SessionIndex)
	case 2:
		// Step 2: Duration selected, resolve session index and extend
		sessionID, err := b.resolveSessionIndex(ctx, data.SessionIndex)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}
		return b.extendSession(ctx, message, sessionID, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in extend flow.", nil)
	}
}

// resolveSessionIndex resolves a session index to a session ID
func (b *Bot) resolveSessionIndex(ctx context.Context, index int) (string, error) {
	// Fetch active sessions to resolve index
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return "", err
	}

	if index < 0 || index >= len(sessions) {
		return "", fmt.Errorf("invalid session index: %d", index)
	}

	return sessions[index].ID, nil
}

// extendStep1 shows session selection
func (b *Bot) extendStep1(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	if len(sessions) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No active sessions to extend.", BuildQuickActionsButtons())
	}

	// Get children for mapping
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := "⏱ *Extend Session*\n\n" + FormatActiveSessions(sessions, childrenMap)
	text += "Select a session to extend:"

	keyboard := BuildSessionsButtons(sessions, "extend")

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// extendStep2 shows duration selection for extension
func (b *Bot) extendStep2(ctx context.Context, message *tgbotapi.Message, sessionIndex int) error {
	text := "⏱ *Extend Session*\n\nSelect additional minutes:"
	keyboard := BuildExtendDurationButtons(sessionIndex)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// extendSession extends the session
func (b *Bot) extendSession(ctx context.Context, message *tgbotapi.Message, sessionID string, additionalMinutes int) error {
	session, err := b.client.ExtendSession(ctx, sessionID, additionalMinutes)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	text := FormatSessionExtended(session, additionalMinutes)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}

// handleStopFlow handles the flow for stopping an active session early
func (b *Bot) handleStopFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 1:
		// Step 1: Session selected (by index), stop it immediately
		sessionID, err := b.resolveSessionIndex(ctx, data.SessionIndex)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}
		return b.stopSession(ctx, message, sessionID)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in stop flow.", nil)
	}
}

// stopSession stops an active session and returns remaining time to children
func (b *Bot) stopSession(ctx context.Context, message *tgbotapi.Message, sessionID string) error {
	// Get session details before stopping (to calculate returned time)
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	var stoppedSession *Session
	for i := range sessions {
		if sessions[i].ID == sessionID {
			stoppedSession = &sessions[i]
			break
		}
	}

	if stoppedSession == nil {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Session not found or already stopped.", BuildQuickActionsButtons())
	}

	// Stop the session
	if err := b.client.StopSession(ctx, sessionID); err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get children for formatting
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := FormatSessionStopped(stoppedSession, childrenMap)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}
