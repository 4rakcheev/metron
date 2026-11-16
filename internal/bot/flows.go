package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		// Step 0: Child selection (from menu or back from device selection)
		if b.globalLockdownActive(ctx) {
			return b.editMessage(message.Chat.ID, message.MessageID, globalLockdownText, BuildLockdownDoneButtons())
		}
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
	case 0:
		// Step 0: Show session selection
		return b.stopStep1(ctx, message)
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

// stopStep1 shows session selection for stopping
func (b *Bot) stopStep1(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildSessionsMenuButtons())
	}

	if len(sessions) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No active sessions to stop.", BuildSessionsMenuButtons())
	}

	// Get children for mapping
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildSessionsMenuButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := "🛑 *Stop Session*\n\n" + FormatActiveSessions(sessions, childrenMap)
	text += "Select a session to stop:"

	keyboard := BuildSessionsButtons(sessions, "stop")

	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
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

	text := FormatRewardGranted(childName, childEmoji, response) +
		"\n\nℹ️ The reward only raises the daily limit. To unlock a device (e.g. the PC), start a session on it."

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildRewardGrantedButtons())
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

// globalLockdownText is shown when a rule for all children on all devices blocks a new session
const globalLockdownText = "🔒 *All sessions are locked*\n\n" +
	"A lockdown for all children on all devices is active. Unlock it first in ⚙️ More, then 🔒 Lockdowns."

// activeLockdowns returns the active lockdown rules, treating errors as "no rules"
// (used for banners and hints only; the server still enforces every rule)
func (b *Bot) activeLockdowns(ctx context.Context) []LockdownRule {
	rules, err := b.client.ListLockdowns(ctx)
	if err != nil {
		b.logger.Warn("Failed to list lockdowns", "error", err)
		return nil
	}
	return rules
}

// globalLockdownActive reports whether a rule blocks all children on all devices.
// Narrower rules are left to the server, which rejects matching sessions with LOCKDOWN_ACTIVE.
func (b *Bot) globalLockdownActive(ctx context.Context) bool {
	for _, rule := range b.activeLockdowns(ctx) {
		if rule.IsGlobal() {
			return true
		}
	}
	return false
}

// handleMoreMenu shows the additional features submenu
func (b *Bot) handleMoreMenu(ctx context.Context, message *tgbotapi.Message) error {
	// Check if downtime is already skipped today
	skipActive, err := b.client.IsDowntimeSkippedToday(ctx)
	if err != nil {
		b.logger.Warn("Failed to check downtime skip status", "error", err)
		skipActive = false
	}

	rules := b.activeLockdowns(ctx)

	text := `⚙️ *Additional Features*

• 🌙 Skip Downtime - Temporarily skip downtime for all children
• 🔒 Lockdowns - Block a child or all children on a device or all devices (until end of day or until you unlock)
• 🔓 Bypass Mode - Disable enforcement for specific devices`

	if len(rules) > 0 {
		text = "🔒 *Active lockdowns*\n" + FormatLockdownLines(rules, time.Now()) + "\n" + text
	}

	keyboard := BuildMoreMenuButtons(skipActive, len(rules))
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// handleLockdownFlow handles the lockdown rules flow: list/unlock and the
// "new lockdown" steps (child, then device, then duration, then create)
func (b *Bot) handleLockdownFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("Lockdown flow",
		"step", data.Step,
		"sub_action", data.SubAction,
		"child_index", data.ChildIndex,
		"device_id", data.Device,
		"until", data.Duration,
		"rule_id", data.RuleID,
	)

	if data.SubAction == lockdownSubUnlock {
		return b.lockdownUnlock(ctx, message, data.RuleID)
	}

	switch data.Step {
	case 0:
		return b.lockdownList(ctx, message, "")
	case 1:
		return b.lockdownPickChild(ctx, message)
	case 2:
		return b.lockdownPickDevice(ctx, message, data.ChildIndex)
	case 3:
		return b.lockdownPickDuration(ctx, message, data.ChildIndex, data.Device)
	case 4:
		return b.lockdownCreate(ctx, message, data)
	default:
		return b.lockdownList(ctx, message, "")
	}
}

// lockdownBackKeyboard returns a keyboard with a single Back button to the lockdown list
func lockdownBackKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
				MarshalCallback(CallbackData{Action: lockdownAction})),
		),
	)
}

// lockdownList shows active lockdown rules with unlock buttons. note is shown on top if not empty.
func (b *Bot) lockdownList(ctx context.Context, message *tgbotapi.Message, note string) error {
	rules, err := b.client.ListLockdowns(ctx)
	if err != nil {
		b.logger.Error("Failed to list lockdowns", "error", err)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
					MarshalCallback(CallbackData{Action: "more_menu"})),
			),
		)
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), keyboard)
	}

	return b.editMessage(message.Chat.ID, message.MessageID,
		FormatLockdownList(rules, note, time.Now()), BuildLockdownListButtons(rules))
}

// lockdownUnlock deletes a rule and shows the list again. A missing rule counts as already removed.
func (b *Bot) lockdownUnlock(ctx context.Context, message *tgbotapi.Message, ruleID int64) error {
	if ruleID <= 0 {
		return b.lockdownList(ctx, message, "❌ Invalid lockdown selection.")
	}

	if err := b.client.DeleteLockdown(ctx, ruleID); err != nil {
		if !isNotFound(err) {
			b.logger.Error("Failed to delete lockdown", "rule_id", ruleID, "error", err)
			return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
		}
		return b.lockdownList(ctx, message, "🔓 Already unlocked (the lockdown was removed or expired).")
	}

	return b.lockdownList(ctx, message, "🔓 *Unlocked*")
}

// lockdownPickChild shows the child picker for a new lockdown
func (b *Bot) lockdownPickChild(ctx context.Context, message *tgbotapi.Message) error {
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	text := "🔒 *New Lockdown*\n\n👶 Step 1/3: Who should be locked?"
	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildLockdownChildButtons(children))
}

// lockdownChildScope resolves the encoded child selection to (child ID, display name).
// lockdownAllChildren maps to ("", "All children").
func (b *Bot) lockdownChildScope(ctx context.Context, childIndex int) (string, string, error) {
	if childIndex == lockdownAllChildren {
		return "", "All children", nil
	}
	if childIndex < 0 {
		return "", "", fmt.Errorf("invalid child index: %d", childIndex)
	}
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return "", "", err
	}
	if childIndex >= len(children) {
		return "", "", fmt.Errorf("invalid child index: %d", childIndex)
	}
	child := children[childIndex]
	return child.ID, strings.TrimSpace(child.Emoji + " " + child.Name), nil
}

// lockdownDeviceScope resolves the encoded device selection to (device ID, display name).
// lockdownAllDevices maps to ("", "all devices").
func (b *Bot) lockdownDeviceScope(ctx context.Context, deviceID string) (string, string, error) {
	if deviceID == lockdownAllDevices {
		return "", "all devices", nil
	}
	device, err := b.resolveDevice(ctx, deviceID)
	if err != nil {
		return "", "", err
	}
	if device == nil {
		return "", "", fmt.Errorf("device %q not found", deviceID)
	}
	return device.ID, resolveDeviceEmoji(*device) + " " + device.Name, nil
}

// lockdownPickDevice shows the device picker for a new lockdown
func (b *Bot) lockdownPickDevice(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	_, childName, err := b.lockdownChildScope(ctx, childIndex)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	devices, err := b.client.ListDevices(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	text := fmt.Sprintf("🔒 *New Lockdown*\n\n👶 %s\n\n📺 Step 2/3: Which device?", escapeMarkdown(childName))
	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildLockdownDeviceButtons(devices, childIndex))
}

// lockdownPickDuration shows the duration picker for a new lockdown
func (b *Bot) lockdownPickDuration(ctx context.Context, message *tgbotapi.Message, childIndex int, deviceID string) error {
	_, childName, err := b.lockdownChildScope(ctx, childIndex)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}
	_, deviceName, err := b.lockdownDeviceScope(ctx, deviceID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	text := fmt.Sprintf("🔒 *New Lockdown*\n\n👶 %s\n📺 %s\n\n⏱ Step 3/3: For how long?\n\n"+
		"Matching active sessions will be stopped right away.",
		escapeMarkdown(childName), escapeMarkdown(deviceName))
	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildLockdownDurationButtons(childIndex, deviceID))
}

// lockdownCreate creates the lockdown rule and shows the result
func (b *Bot) lockdownCreate(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	var until string
	switch data.Duration {
	case lockdownUntilEndOfDayCode:
		until = LockdownUntilEndOfDay
	case lockdownUntilManualCode:
		until = LockdownUntilManual
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid lockdown duration. Please start again.", lockdownBackKeyboard())
	}

	childID, _, err := b.lockdownChildScope(ctx, data.ChildIndex)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}
	deviceID, _, err := b.lockdownDeviceScope(ctx, data.Device)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	result, err := b.client.CreateLockdown(ctx, childID, deviceID, until)
	if err != nil {
		b.logger.Error("Failed to create lockdown", "child_id", childID, "device_id", deviceID, "until", until, "error", err)
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), lockdownBackKeyboard())
	}

	return b.editMessage(message.Chat.ID, message.MessageID, FormatLockdownCreated(result, time.Now()), BuildLockdownDoneButtons())
}

// handleSkipDowntime handles the skip downtime today action
func (b *Bot) handleSkipDowntime(ctx context.Context, message *tgbotapi.Message) error {
	lockdownCount := len(b.activeLockdowns(ctx))

	// Check if already skipped
	alreadySkipped, err := b.client.IsDowntimeSkippedToday(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildMoreMenuButtons(false, lockdownCount))
	}

	if alreadySkipped {
		text := `✅ *Downtime Already Skipped*

Downtime is already skipped for today.
It will resume automatically tomorrow.`
		return b.editMessage(message.Chat.ID, message.MessageID, text, BuildMoreMenuButtons(true, lockdownCount))
	}

	// Skip downtime for today
	if err := b.client.SkipDowntimeToday(ctx); err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildMoreMenuButtons(false, lockdownCount))
	}

	text := `✅ *Downtime Skipped for Today!*

All children can now use screen time without downtime restrictions until midnight.

Downtime will automatically resume tomorrow.`

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildMoreMenuButtons(true, lockdownCount))
}

// approvalDisabledText is shown when the server has the approval gate turned off
const approvalDisabledText = "✅ *Approve day*\n\nApproval gate is disabled on the server (approval\\_gate in config.json)."

// approvalBackKeyboard returns a keyboard with a single Back button to the approval screen
func approvalBackKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("◀️ Back",
				MarshalCallback(CallbackData{Action: approvalAction})),
		),
	)
}

// approvalScreen loads today's approvals and returns the screen text and keyboard.
// note is shown on top if not empty.
func (b *Bot) approvalScreen(ctx context.Context, note string) (string, tgbotapi.InlineKeyboardMarkup) {
	approvals, err := b.client.ListTodayApprovals(ctx)
	if err != nil {
		if isFeatureDisabled(err) {
			return approvalDisabledText, BuildQuickActionsButtons()
		}
		b.logger.Error("Failed to list approvals", "error", err)
		return FormatError(err), BuildQuickActionsButtons()
	}
	return FormatApprovals(approvals, note), BuildApprovalButtons(approvals.Children)
}

// handleApprovalFlow handles the "Approve day" flow: show status, approve, revoke (with confirmation)
func (b *Bot) handleApprovalFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("Approval flow",
		"step", data.Step,
		"sub_action", data.SubAction,
		"child_index", data.ChildIndex,
	)

	switch data.SubAction {
	case approvalSubApprove:
		return b.approvalApprove(ctx, message, data.ChildIndex)
	case approvalSubRevoke:
		if data.Step == 1 {
			return b.approvalRevoke(ctx, message, data.ChildIndex)
		}
		return b.approvalRevokeConfirm(ctx, message, data.ChildIndex)
	default:
		text, keyboard := b.approvalScreen(ctx, "")
		return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
	}
}

// resolveApprovalChild resolves a child index in today's approval list.
// ok is false when the screen was already rendered (feature disabled, error, bad index).
func (b *Bot) resolveApprovalChild(ctx context.Context, message *tgbotapi.Message, childIndex int) (*ChildApproval, bool, error) {
	approvals, err := b.client.ListTodayApprovals(ctx)
	if err != nil {
		if isFeatureDisabled(err) {
			return nil, false, b.editMessage(message.Chat.ID, message.MessageID, approvalDisabledText, BuildQuickActionsButtons())
		}
		return nil, false, b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), approvalBackKeyboard())
	}
	if childIndex < 0 || childIndex >= len(approvals.Children) {
		text, keyboard := b.approvalScreen(ctx, "❌ Invalid child selection.")
		return nil, false, b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
	}
	child := approvals.Children[childIndex]
	return &child, true, nil
}

// approvalApprove approves a child for today and re-renders the screen
func (b *Bot) approvalApprove(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	child, ok, err := b.resolveApprovalChild(ctx, message, childIndex)
	if !ok {
		return err
	}

	if _, err := b.client.ApproveChild(ctx, child.ChildID); err != nil {
		if isFeatureDisabled(err) {
			return b.editMessage(message.Chat.ID, message.MessageID, approvalDisabledText, BuildQuickActionsButtons())
		}
		b.logger.Error("Failed to approve child", "child_id", child.ChildID, "error", err)
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), approvalBackKeyboard())
	}

	note := fmt.Sprintf("✅ %s can now use screen time today.", escapeMarkdown(child.ChildName))
	text, keyboard := b.approvalScreen(ctx, note)
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// approvalRevokeConfirm asks the parent to confirm revoking today's approval
func (b *Bot) approvalRevokeConfirm(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	child, ok, err := b.resolveApprovalChild(ctx, message, childIndex)
	if !ok {
		return err
	}

	text := fmt.Sprintf("↩️ *Revoke approval for %s?*\n\n"+
		"%s will not be able to start new screen time from the app today until you approve again. "+
		"Sessions that are already running keep running.",
		escapeMarkdown(child.ChildName), escapeMarkdown(child.ChildName))
	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildApprovalRevokeConfirmButtons(childIndex))
}

// approvalRevoke revokes today's approval for a child and re-renders the screen
func (b *Bot) approvalRevoke(ctx context.Context, message *tgbotapi.Message, childIndex int) error {
	child, ok, err := b.resolveApprovalChild(ctx, message, childIndex)
	if !ok {
		return err
	}

	if err := b.client.RevokeApproval(ctx, child.ChildID); err != nil {
		if isFeatureDisabled(err) {
			return b.editMessage(message.Chat.ID, message.MessageID, approvalDisabledText, BuildQuickActionsButtons())
		}
		b.logger.Error("Failed to revoke approval", "child_id", child.ChildID, "error", err)
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), approvalBackKeyboard())
	}

	note := fmt.Sprintf("↩️ Approval revoked for %s.", escapeMarkdown(child.ChildName))
	text, keyboard := b.approvalScreen(ctx, note)
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
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

// handleBypassFlow handles the bypass mode flow for devices
func (b *Bot) handleBypassFlow(ctx context.Context, message *tgbotapi.Message, data *CallbackData) error {
	b.logger.Info("Bypass flow",
		"step", data.Step,
		"sub_action", data.SubAction,
		"device_id", data.Device,
		"duration", data.Duration,
	)

	switch data.Step {
	case 0:
		// Step 0: Show device selection
		return b.bypassStep0(ctx, message)
	case 1:
		// Step 1: Device selected, show enable/disable options
		return b.bypassStep1(ctx, message, data.Device)
	case 2:
		// Step 2: Action selected, execute
		switch data.SubAction {
		case "enable":
			return b.bypassEnable(ctx, message, data.Device, data.Duration)
		case "disable":
			return b.bypassDisable(ctx, message, data.Device)
		default:
			return b.editMessage(message.Chat.ID, message.MessageID,
				"❌ Unknown bypass action.", BuildQuickActionsButtons())
		}
	default:
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid step in bypass flow.", BuildQuickActionsButtons())
	}
}

// bypassStep0 shows device selection for bypass
func (b *Bot) bypassStep0(ctx context.Context, message *tgbotapi.Message) error {
	// Get all devices
	devices, err := b.client.ListDevices(ctx)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	devicesWithBypass, err := b.loadDevicesWithBypass(ctx, devices)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(devicesWithBypass) == 0 {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ No devices available for bypass mode.", BuildQuickActionsButtons())
	}

	text := "🔓 *Bypass Mode*\n\n" +
		"Bypass mode temporarily disables screen-time enforcement for a device.\n" +
		"Only agent-controlled devices (Windows PC) support it.\n\n" +
		"✅ = Bypass enabled (no limits)\n" +
		"🔒 = Normal enforcement\n\n" +
		"Select a device:"

	keyboard := BuildBypassDevicesButtons(devicesWithBypass)
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// loadDevicesWithBypass fetches the bypass status for each device that supports bypass.
// Fails instead of silently reporting "disabled" so an active bypass is never hidden.
func (b *Bot) loadDevicesWithBypass(ctx context.Context, devices []Device) ([]DeviceWithBypass, error) {
	result := make([]DeviceWithBypass, 0, len(devices))
	for _, device := range devices {
		// Bypass is only enforced by agents; for other devices it would silently do nothing
		if !device.Capabilities.SupportsBypass {
			continue
		}
		bypass, err := b.client.GetDeviceBypass(ctx, device.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get bypass status for %s: %w", device.Name, err)
		}
		result = append(result, DeviceWithBypass{
			Device:        device,
			BypassEnabled: bypass != nil && bypass.Enabled,
		})
	}
	return result, nil
}

// resolveDevice fetches the device list and returns the device with the given ID,
// or nil if no such device exists
func (b *Bot) resolveDevice(ctx context.Context, deviceID string) (*Device, error) {
	devices, err := b.client.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	for i := range devices {
		if devices[i].ID == deviceID {
			return &devices[i], nil
		}
	}
	return nil, nil
}

// bypassStep1 shows enable/disable options for a device
func (b *Bot) bypassStep1(ctx context.Context, message *tgbotapi.Message, deviceID string) error {
	device, err := b.resolveDevice(ctx, deviceID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}
	if device == nil {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid device selection.", BuildQuickActionsButtons())
	}

	// Get current bypass status
	bypass, err := b.client.GetDeviceBypass(ctx, device.ID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}
	currentlyEnabled := bypass != nil && bypass.Enabled

	emoji := resolveDeviceEmoji(*device)
	var text string
	if currentlyEnabled {
		text = fmt.Sprintf("🔓 *Bypass Mode*\n\n%s *%s*\n\n✅ Bypass is currently *ENABLED* %s\n\n"+
			"Tap below to disable bypass and resume normal enforcement:",
			emoji, device.Name, formatBypassExpiry(bypass))
	} else {
		text = fmt.Sprintf("🔓 *Bypass Mode*\n\n%s *%s*\n\n🔒 Bypass is currently *DISABLED*\n\n"+
			"Select how long to enable bypass mode:",
			emoji, device.Name)
	}

	keyboard := BuildBypassActionsButtons(device.ID, currentlyEnabled)
	return b.editMessage(message.Chat.ID, message.MessageID, text, keyboard)
}

// bypassEnable enables bypass mode for a device
func (b *Bot) bypassEnable(ctx context.Context, message *tgbotapi.Message, deviceID string, durationMinutes int) error {
	device, err := b.resolveDevice(ctx, deviceID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}
	if device == nil {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid device selection.", BuildQuickActionsButtons())
	}

	untilEndOfDay := durationMinutes == bypassUntilEndOfDay
	if untilEndOfDay {
		durationMinutes = minutesUntilEndOfDay(time.Now())
	}
	if durationMinutes <= 0 {
		// Stale buttons from older bot versions sent 0 for "Indefinite"; never create an endless bypass
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid bypass duration. Please open Bypass Mode again.", BuildQuickActionsButtons())
	}

	req := SetDeviceBypassRequest{
		Enabled:          true,
		Reason:           "Telegram Bot",
		ExpiresInMinutes: &durationMinutes,
	}

	// Enable bypass
	_, err = b.client.SetDeviceBypass(ctx, device.ID, req)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	emoji := resolveDeviceEmoji(*device)
	var durationText string
	switch {
	case untilEndOfDay:
		durationText = "until the end of the day"
	case durationMinutes == 60:
		durationText = "for 1 hour"
	case durationMinutes == 120:
		durationText = "for 2 hours"
	case durationMinutes == 480:
		durationText = "until bedtime (~8 hours)"
	default:
		durationText = fmt.Sprintf("for %d minutes", durationMinutes)
	}

	text := fmt.Sprintf("✅ *Bypass Enabled*\n\n%s *%s*\n\n"+
		"Screen-time enforcement is now *disabled* %s.\n\n"+
		"Your child can use this device without any limits.",
		emoji, device.Name, durationText)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}

// bypassDisable disables bypass mode for a device
func (b *Bot) bypassDisable(ctx context.Context, message *tgbotapi.Message, deviceID string) error {
	device, err := b.resolveDevice(ctx, deviceID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}
	if device == nil {
		return b.editMessage(message.Chat.ID, message.MessageID,
			"❌ Invalid device selection.", BuildQuickActionsButtons())
	}

	// Clear bypass
	err = b.client.ClearDeviceBypass(ctx, device.ID)
	if err != nil {
		return b.editMessage(message.Chat.ID, message.MessageID, FormatError(err), BuildQuickActionsButtons())
	}

	emoji := resolveDeviceEmoji(*device)
	text := fmt.Sprintf("🔒 *Bypass Disabled*\n\n%s *%s*\n\n"+
		"Screen-time enforcement is now *resumed*.\n\n"+
		"Normal session rules will apply.",
		emoji, device.Name)

	return b.editMessage(message.Chat.ID, message.MessageID, text, BuildQuickActionsButtons())
}
