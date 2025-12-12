package bot

import (
	"context"
	"fmt"
	"log/slog"
	"metron/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot
type Bot struct {
	api    *tgbotapi.BotAPI
	client *MetronAPI
	config *config.BotConfig
	logger *slog.Logger
}

// NewBot creates a new Telegram bot instance
func NewBot(cfg *config.BotConfig, logger *slog.Logger) (*Bot, error) {
	// Create Telegram bot API client
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	// Create Metron API client
	metronClient := NewMetronAPI(
		cfg.Metron.BaseURL,
		cfg.Metron.APIKey,
		logger,
	)

	bot := &Bot{
		api:    api,
		client: metronClient,
		config: cfg,
		logger: logger,
	}

	return bot, nil
}

// SetWebhook configures the webhook for the bot
func (b *Bot) SetWebhook() error {
	webhookConfig, _ := tgbotapi.NewWebhook(b.config.Telegram.WebhookURL)

	// Note: SecretToken is not available in all versions of the telegram bot API
	// It will be validated in the webhook handler instead

	_, err := b.api.Request(webhookConfig)
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	info, err := b.api.GetWebhookInfo()
	if err != nil {
		return fmt.Errorf("failed to get webhook info: %w", err)
	}

	b.logger.Info("Webhook configured",
		"url", info.URL,
		"pending_updates", info.PendingUpdateCount,
	)

	return nil
}

// HandleUpdate processes a Telegram update
func (b *Bot) HandleUpdate(update tgbotapi.Update) error {
	ctx := context.Background()

	// Check authorization for all updates
	var userID int64
	if update.Message != nil {
		userID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	} else {
		// Ignore updates without user info
		return nil
	}

	if !b.config.IsUserAllowed(userID) {
		b.logger.Warn("Unauthorized access attempt",
			"user_id", userID,
		)
		return b.sendUnauthorizedMessage(update)
	}

	// Route update to appropriate handler
	if update.Message != nil {
		return b.handleMessage(ctx, update.Message)
	}

	if update.CallbackQuery != nil {
		return b.handleCallback(ctx, update.CallbackQuery)
	}

	return nil
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	b.logger.Info("Received message",
		"user_id", message.From.ID,
		"username", message.From.UserName,
		"text", message.Text,
	)

	if !message.IsCommand() {
		// Ignore non-command messages
		return nil
	}

	switch message.Command() {
	case "start":
		return b.handleStart(ctx, message)
	case "today":
		return b.handleToday(ctx, message)
	case "newsession":
		return b.handleNewSession(ctx, message)
	case "extend":
		return b.handleExtend(ctx, message)
	case "children":
		return b.handleChildren(ctx, message)
	case "devices":
		return b.handleDevices(ctx, message)
	default:
		return b.sendMessage(message.Chat.ID,
			"Unknown command. Use /start to see available commands.", nil)
	}
}

// handleCallback processes callback queries from inline buttons
func (b *Bot) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	b.logger.Info("Received callback",
		"user_id", callback.From.ID,
		"data", callback.Data,
		"data_length", len(callback.Data),
	)

	// Answer callback to remove loading state
	answer := tgbotapi.NewCallback(callback.ID, "")
	if _, err := b.api.Request(answer); err != nil {
		b.logger.Error("Failed to answer callback", "error", err)
	}

	// Check if callback data is a raw command (starts with /)
	if len(callback.Data) > 0 && callback.Data[0] == '/' {
		// Handle as command
		msg := &tgbotapi.Message{
			Chat:    callback.Message.Chat,
			From:    callback.From,
			Text:    callback.Data,
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: len(callback.Data)},
			},
		}
		return b.handleMessage(ctx, msg)
	}

	// Parse callback data as JSON
	data, err := UnmarshalCallback(callback.Data)
	if err != nil {
		b.logger.Error("Failed to unmarshal callback data",
			"raw_data", callback.Data,
			"error", err,
		)
		return b.sendMessage(callback.Message.Chat.ID, FormatError(err), nil)
	}

	b.logger.Info("Parsed callback data",
		"action", data.Action,
		"step", data.Step,
		"child_index", data.ChildIndex,
		"device", data.Device,
		"duration", data.Duration,
	)

	// Route to flow handler
	switch data.Action {
	case "cancel":
		return b.handleCancel(ctx, callback.Message)
	case "newsession":
		return b.handleNewSessionFlow(ctx, callback.Message, data)
	case "extend":
		return b.handleExtendFlow(ctx, callback.Message, data)
	default:
		return b.sendMessage(callback.Message.Chat.ID,
			"Unknown action.", nil)
	}
}

// sendMessage sends a text message
func (b *Bot) sendMessage(chatID int64, text string, keyboard interface{}) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	if keyboard != nil {
		switch kb := keyboard.(type) {
		case tgbotapi.InlineKeyboardMarkup:
			msg.ReplyMarkup = kb
		}
	}

	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("Failed to send message",
			"chat_id", chatID,
			"error", err,
		)
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// editMessage edits an existing message
func (b *Bot) editMessage(chatID int64, messageID int, text string, keyboard interface{}) error {
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	msg.ParseMode = "Markdown"

	if keyboard != nil {
		switch kb := keyboard.(type) {
		case tgbotapi.InlineKeyboardMarkup:
			msg.ReplyMarkup = &kb
		}
	}

	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("Failed to edit message",
			"chat_id", chatID,
			"message_id", messageID,
			"error", err,
		)
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

// sendUnauthorizedMessage sends an unauthorized access message
func (b *Bot) sendUnauthorizedMessage(update tgbotapi.Update) error {
	var chatID int64
	if update.Message != nil {
		chatID = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Chat.ID
	} else {
		return nil
	}

	return b.sendMessage(chatID,
		"⛔ You are not authorized to use this bot.", nil)
}

// handleCancel handles the cancel action
func (b *Bot) handleCancel(ctx context.Context, message *tgbotapi.Message) error {
	return b.editMessage(message.Chat.ID, message.MessageID,
		"❌ Cancelled.", BuildQuickActionsButtons())
}
