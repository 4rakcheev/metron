package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStart handles the /start command
func (b *Bot) handleStart(ctx context.Context, message *tgbotapi.Message) error {
	text := `👋 *Welcome to Metron Screen Time Bot!*

I help you manage your children's screen time across all devices.

*Available Commands:*

📊 /today - View today's screen time summary
➕ /newsession - Start a new screen time session
⏱ /extend - Extend an active session
🛑 /stop - Stop an active session early
🎁 /reward - Grant bonus time to a child
👶 /children - List all children
📺 /devices - List available devices

*Quick Actions:*`

	keyboard := BuildMainMenuButtons()
	return b.sendMessage(message.Chat.ID, text, keyboard)
}

// handleToday handles the /today command
func (b *Bot) handleToday(ctx context.Context, message *tgbotapi.Message) error {
	// Get today's stats
	stats, err := b.client.GetTodayStats(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get children for mapping
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := FormatTodayStats(stats, sessions, childrenMap)
	return b.sendMessage(message.Chat.ID, text, BuildQuickActionsButtons())
}

// handleChildren handles the /children command
func (b *Bot) handleChildren(ctx context.Context, message *tgbotapi.Message) error {
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	text := FormatChildren(children)
	// Add instruction about downtime toggle
	text += "\nTap a child below to toggle downtime:"
	return b.sendMessage(message.Chat.ID, text, BuildDowntimeToggleButtons(children))
}

// handleDevices handles the /devices command
func (b *Bot) handleDevices(ctx context.Context, message *tgbotapi.Message) error {
	devices, err := b.client.ListDevices(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	text := FormatDevices(devices)
	return b.sendMessage(message.Chat.ID, text, BuildQuickActionsButtons())
}

// handleNewSession handles the /newsession command (step 0)
func (b *Bot) handleNewSession(ctx context.Context, message *tgbotapi.Message) error {
	// Get children list
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(children) == 0 {
		return b.sendMessage(message.Chat.ID,
			"❌ No children configured. Please add children first.", BuildQuickActionsButtons())
	}

	text := "➕ *New Session*\n\n👶 Step 1/3: Select child(ren)"
	keyboard := BuildChildrenButtons(children, "newsession", 1)

	return b.sendMessage(message.Chat.ID, text, keyboard)
}

// handleExtend handles the /extend command - now shows session management UI
func (b *Bot) handleExtend(ctx context.Context, message *tgbotapi.Message) error {
	// Get children for use in both cases
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	// If no active sessions, offer to grant rewards instead
	if len(sessions) == 0 {
		text := "⏱ *Extend / Grant Rewards*\n\n" +
			"❌ No active sessions to extend.\n\n" +
			"💡 You can grant reward minutes to give children extra time for today:"

		keyboard := BuildChildrenButtons(children, "reward", 1)
		return b.sendMessage(message.Chat.ID, text, keyboard)
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := "⏱ *Manage Sessions*\n\nSelect an action for each session:\n" +
		"• ⏱ Extend - Add more minutes\n" +
		"• 🛑 Stop - End session early\n" +
		"• 👶 Add Kid - Share with another child\n\n" +
		"Or grant reward minutes for later:"

	keyboard := BuildSessionManagementButtons(sessions, childrenMap)

	return b.sendMessage(message.Chat.ID, text, keyboard)
}

// handleStop handles the /stop command - allows stopping active sessions early
func (b *Bot) handleStop(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(sessions) == 0 {
		return b.sendMessage(message.Chat.ID,
			"❌ No active sessions to stop.", BuildQuickActionsButtons())
	}

	// Get children for mapping
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	childrenMap := make(map[string]Child)
	for _, child := range children {
		childrenMap[child.ID] = child
	}

	text := "🛑 *Stop Session*\n\n" + FormatActiveSessions(sessions, childrenMap)
	text += "Select a session to stop:"

	keyboard := BuildSessionsButtons(sessions, "stop")

	return b.sendMessage(message.Chat.ID, text, keyboard)
}

// handleReward handles the /reward command - allows granting reward minutes to children
func (b *Bot) handleReward(ctx context.Context, message *tgbotapi.Message) error {
	// Get children list
	children, err := b.client.ListChildren(ctx)
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(children) == 0 {
		return b.sendMessage(message.Chat.ID,
			"❌ No children configured. Please add children first.", BuildQuickActionsButtons())
	}

	text := "🎁 *Grant Reward*\n\n👶 Step 1/2: Select child"
	keyboard := BuildChildrenButtons(children, "reward", 1)

	return b.sendMessage(message.Chat.ID, text, keyboard)
}
