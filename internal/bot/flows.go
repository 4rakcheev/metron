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

	// Add parent override context (Telegram bot requests are always from parent)
	ctx = context.WithValue(ctx, "parent_override", true)

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

// handleManageFlow handles the unified session management flow
func (b *Bot) handleManageFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("Manage session flow",
		"sub_action", data.SubAction,
		"step", data.Step,
		"session_index", data.SessionIndex,
	)

	switch data.Step {
	case 0:
		// Step 0: Back to session list
		return b.manageStep0(ctx, message)
	case 1:
		// Step 1: Action selected for a session
		switch data.SubAction {
		case "extend":
			// Show duration selection
			return b.manageExtendStep1(ctx, message, data.SessionIndex)
		case "stop":
			// Stop immediately
			sessionID, err := b.resolveSessionIndex(ctx, data.SessionIndex)
			if err != nil {
				return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
			}
			return b.stopSession(ctx, message, sessionID)
		case "add_kid":
			// Show available children to add
			return b.manageAddKidStep1(ctx, message, data.SessionIndex)
		default:
			return b.editMessage(message.Chat.ID, message.MessageID,
				"❌ Unknown action.", BuildQuickActionsButtons())
		}
	case 2:
		// Step 2: Second-level action
		switch data.SubAction {
		case "extend":
			// Duration selected, extend session
			sessionID, err := b.resolveSessionIndex(ctx, data.SessionIndex)
			if err != nil {
				return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
			}
			return b.extendSession(ctx, message, sessionID, data.Duration)
		case "add_kid":
			// Child selected, add to session
			return b.manageAddKidStep2(ctx, message, data.SessionIndex, data.ChildIndex)
		default:
			return b.editMessage(message.Chat.ID, message.MessageID,
				"❌ Unknown action.", BuildQuickActionsButtons())
		}
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in manage flow.", BuildQuickActionsButtons())
	}
}

// manageStep0 shows the session management list
func (b *Bot) manageStep0(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(sessions) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No active sessions.", BuildQuickActionsButtons())
	}

	// Get children for mapping
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := "⏱ *Manage Sessions*\n\nSelect an action for each session:\n" +
		"• ⏱ Extend - Add more minutes\n" +
		"• 🛑 Stop - End session early\n" +
		"• 👶 Add Kid - Share with another child\n"

	keyboard := BuildSessionManagementButtons(sessions, childrenMap)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// manageExtendStep1 shows duration selection for extending
func (b *Bot) manageExtendStep1(ctx context.Context, message *tgbotapi.Message, sessionIndex int) error {
	text := "⏱ *Extend Session*\n\nSelect additional minutes:"
	keyboard := BuildExtendDurationButtons(sessionIndex)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// manageAddKidStep1 shows child selection for adding to session
func (b *Bot) manageAddKidStep1(ctx context.Context, message *tgbotapi.Message, sessionIndex int) error {
	// Get all children
	allChildren, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get the session to see which children are already in it
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	if sessionIndex < 0 || sessionIndex >= len(sessions) {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid session.", BuildQuickActionsButtons())
	}

	session := sessions[sessionIndex]

	// Filter out children already in session
	var availableChildren []Child
	for _, child := range allChildren {
		alreadyIn := false
		for _, childID := range session.ChildIDs {
			if child.ID == childID {
				alreadyIn = true
				break
			}
		}
		if !alreadyIn {
			availableChildren = append(availableChildren, child)
		}
	}

	if len(availableChildren) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ All children are already in this session.", BuildQuickActionsButtons())
	}

	alreadyShared := len(session.ChildIDs) == len(allChildren)

	text := "👶 *Add Child to Session*\n\nSelect a child to add:"
	keyboard := BuildAddKidButtons(sessionIndex, availableChildren, alreadyShared)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// manageAddKidStep2 adds the selected child to the session
func (b *Bot) manageAddKidStep2(ctx context.Context, message *tgbotapi.Message, sessionIndex int, childIndex int) error {
	// Get session
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	if sessionIndex < 0 || sessionIndex >= len(sessions) {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid session.", BuildQuickActionsButtons())
	}

	session := sessions[sessionIndex]

	// Get all children
	allChildren, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	// Determine which children to add
	var childIDsToAdd []string

	if childIndex == -1 {
		// "Mark as Shared" - add all available children
		for _, child := range allChildren {
			alreadyIn := false
			for _, childID := range session.ChildIDs {
				if child.ID == childID {
					alreadyIn = true
					break
				}
			}
			if !alreadyIn {
				childIDsToAdd = append(childIDsToAdd, child.ID)
			}
		}
	} else {
		// Add specific child
		// Get available children (those not in session)
		var availableChildren []Child
		for _, child := range allChildren {
			alreadyIn := false
			for _, childID := range session.ChildIDs {
				if child.ID == childID {
					alreadyIn = true
					break
				}
			}
			if !alreadyIn {
				availableChildren = append(availableChildren, child)
			}
		}

		if childIndex < 0 || childIndex >= len(availableChildren) {
			return b.editMessage(message.Chat.ID, message.MessageID,
				"❌ Invalid child selection.", BuildQuickActionsButtons())
		}

		childIDsToAdd = []string{availableChildren[childIndex].ID}
	}

	if len(childIDsToAdd) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No children to add.", BuildQuickActionsButtons())
	}

	// Call API to add children
	updatedSession, err := b.client.AddChildrenToSession(ctx, session.ID, childIDsToAdd)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get children for formatting
	childrenMap := make(map[string]Child)
	for _, child := range allChildren {
		childrenMap[child.ID] = child
	}

	text := FormatChildrenAddedToSession(updatedSession, childIDsToAdd, childrenMap)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}

// handleRewardFlow handles the multi-step flow for granting rewards
func (b *Bot) handleRewardFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 0:
		// Step 0: Back to child selection
		return b.rewardStep1(ctx, message)
	case 1:
		// Step 1: Child selected (by index), show reward durations
		return b.rewardStep2(ctx, message, data.ChildIndex)
	case 2:
		// Step 2: Duration selected, grant reward
		childID, err := b.resolveChildIndex(ctx, data.ChildIndex)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}
		return b.grantReward(ctx, message, childID, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in reward flow.", nil)
	}
}

// rewardStep1 shows child selection
func (b *Bot) rewardStep1(ctx context.Context, message *tgbotapi.Message) error {
	// Get children list
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	if len(children) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No children configured. Add children first using the API.", nil)
	}

	text := "🎁 *Grant Reward*\n\n👶 Step 1/2: Select child"
	keyboard := BuildChildrenButtons(children, "reward", 1)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// rewardStep2 shows reward duration selection
func (b *Bot) rewardStep2(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	text := "🎁 *Grant Reward*\n\n⏱ Step 2/2: Select bonus minutes"
	keyboard := BuildRewardDurationButtons(childIndex)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// grantReward grants the reward to the child
func (b *Bot) grantReward(ctx context.Context, message *tgbotapi.Message, childID string, minutes int) error {
	// Get child info for formatting
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	var childName, childEmoji string
	for _, child := range children {
		if child.ID == childID {
			childName = child.Name
			childEmoji = child.Emoji
			break
		}
	}

	// Grant reward
	response, err := b.client.GrantReward(ctx, childID, minutes)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	text := FormatRewardGranted(childName, childEmoji, response)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}

// handleFineFlow handles the multi-step flow for applying fines
func (b *Bot) handleFineFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	switch data.Step {
	case 0:
		// Step 0: Back to child selection
		return b.fineStep1(ctx, message)
	case 1:
		// Step 1: Child selected (by index), show fine durations
		return b.fineStep2(ctx, message, data.ChildIndex)
	case 2:
		// Step 2: Duration selected, apply fine
		childID, err := b.resolveChildIndex(ctx, data.ChildIndex)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
		}
		return b.applyFine(ctx, message, childID, data.Duration)
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"Invalid step in fine flow.", nil)
	}
}

// fineStep1 shows child selection
func (b *Bot) fineStep1(ctx context.Context, message *tgbotapi.Message) error {
	// Get children list
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
	}

	if len(children) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"No children configured. Add children first using the API.", nil)
	}

	text := "*Apply Fine*\n\nStep 1/2: Select child"
	keyboard := BuildChildrenButtons(children, "fine", 1)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// fineStep2 shows fine duration selection
func (b *Bot) fineStep2(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	text := "*Apply Fine*\n\nStep 2/2: Select deduction amount"
	keyboard := BuildFineDurationButtons(childIndex)

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// applyFine applies the fine to the child
func (b *Bot) applyFine(ctx context.Context, message *tgbotapi.Message, childID string, minutes int) error {
	// Get child info for formatting
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	var childName, childEmoji string
	for _, child := range children {
		if child.ID == childID {
			childName = child.Name
			childEmoji = child.Emoji
			break
		}
	}

	// Apply fine
	response, err := b.client.DeductFine(ctx, childID, minutes)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	text := FormatFineApplied(childName, childEmoji, response)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
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

// handleDowntimeFlow handles downtime toggle callbacks
func (b *Bot) handleDowntimeFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("Downtime flow",
		"sub_action", data.SubAction,
		"child_index", data.ChildIndex,
	)

	if data.SubAction == "toggle" {
		// Get all children
		children, err := b.client.ListChildren(ctx)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
		}

		// Validate child index
		if data.ChildIndex < 0 || data.ChildIndex >= len(children) {
			return b.editMessage(message.Chat.ID, message.MessageID,
				"❌ Invalid child selection.", nil)
		}

		child := children[data.ChildIndex]

		// Toggle downtime
		newStatus := !child.DowntimeEnabled
		if err := b.client.UpdateChildDowntime(ctx, child.ID, newStatus); err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
		}

		// Refresh children list
		children, err = b.client.ListChildren(ctx)
		if err != nil {
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), nil)
		}

		// Build confirmation message
		emoji := child.Emoji
		statusText := "enabled"
		if !newStatus {
			statusText = "disabled"
		}
		confirmText := fmt.Sprintf("✅ Downtime %s for %s %s\n\n", statusText, emoji, child.Name)
		confirmText += FormatChildren(children)
		confirmText += "\nTap a child below to toggle downtime:"

		return b.editMessage(message.Chat.ID, message.MessageID, confirmText, BuildDowntimeToggleButtons(children))
	}

	return b.editMessage(message.Chat.ID, message.MessageID,
		"❌ Unknown downtime action.", nil)
}

// handleMainMenu returns to the main menu
func (b *Bot) handleMainMenu(ctx context.Context, message *tgbotapi.Message) error {
	text := `👋 *Metron Screen Time Bot*

Use the buttons below to manage screen time.`

	keyboard := BuildMainMenuButtons()
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// handleSessionsMenu shows the sessions management submenu
func (b *Bot) handleSessionsMenu(ctx context.Context, message *tgbotapi.Message) error {
	text := `🎬 *Session Management*

• 🎁 Give Reward - Grant bonus time for today
• 🛑 Stop All Sessions - End all active sessions
• 🛑 Stop Specific Session - Select session to stop`

	keyboard := BuildSessionsMenuButtons()
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// handleMoreMenu shows the additional features submenu
func (b *Bot) handleMoreMenu(ctx context.Context, message *tgbotapi.Message) error {
	// Check if downtime is already skipped today
	skipActive, err := b.client.IsDowntimeSkippedToday(ctx)
	if err != nil {
		b.logger.Warn("Failed to check downtime skip status", "error", err)
		skipActive = false
	}

	text := `⚙️ *Additional Features*

Manage advanced options:`

	keyboard := BuildMoreMenuButtons(skipActive)
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// handleSkipDowntime handles the skip downtime today action
func (b *Bot) handleSkipDowntime(ctx context.Context, message *tgbotapi.Message) error {
	// Check if already skipped
	alreadySkipped, err := b.client.IsDowntimeSkippedToday(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildMoreMenuButtons(false))
	}

	if alreadySkipped {
		text := `✅ *Downtime Already Skipped*

Downtime is already skipped for today.
It will resume automatically tomorrow.`
		return b.editMessage(message.Chat.ID, message.MessageID, text, BuildMoreMenuButtons(true))
	}

	// Skip downtime for today
	if err := b.client.SkipDowntimeToday(ctx); err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildMoreMenuButtons(false))
	}

	text := `✅ *Downtime Skipped for Today!*

All children can now use screen time without downtime restrictions until midnight.

Downtime will automatically resume tomorrow.`

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildMoreMenuButtons(true))
}

// handleStopAll stops all active sessions
func (b *Bot) handleStopAll(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildSessionsMenuButtons())
	}

	if len(sessions) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No active sessions to stop.", BuildSessionsMenuButtons())
	}

	// Stop all sessions
	stoppedCount := 0
	for _, session := range sessions {
		if err := b.client.StopSession(ctx, session.ID); err != nil {
			b.logger.Error("Failed to stop session", "session_id", session.ID, "error", err)
		} else {
			stoppedCount++
		}
	}

	text := fmt.Sprintf("🛑 *Sessions Stopped*\n\nStopped %d active session(s).", stoppedCount)
	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}
