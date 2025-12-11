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
	return b.sendMessage(message.Chat.ID, text, BuildQuickActionsButtons())
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

// handleExtend handles the /extend command (step 0)
func (b *Bot) handleExtend(ctx context.Context, message *tgbotapi.Message) error {
	// Get active sessions
	sessions, err := b.client.ListSessions(ctx, true, "")
	if err != nil {
		return b.sendMessage(message.Chat.ID, FormatError(err), BuildQuickActionsButtons())
	}

	if len(sessions) == 0 {
		return b.sendMessage(message.Chat.ID,
			"❌ No active sessions to extend.", BuildQuickActionsButtons())
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

	text := "⏱ *Extend Session*\n\n" + FormatActiveSessions(sessions, childrenMap)
	text += "Select a session to extend:"

	keyboard := BuildSessionsButtons(sessions, "extend")

	return b.sendMessage(message.Chat.ID, text, keyboard)
}
