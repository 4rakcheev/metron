package bot

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gin-gonic/gin"
)

// WebhookHandler handles incoming webhook requests from Telegram
type WebhookHandler struct {
	bot    *Bot
	logger *slog.Logger
	secret string
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(bot *Bot, secret string, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		bot:    bot,
		logger: logger,
		secret: secret,
	}
}

// HandleWebhook processes incoming webhook requests
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	// Verify secret token if configured
	if h.secret != "" {
		token := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
		if token != h.secret {
			h.logger.Warn("Invalid webhook secret token",
				"remote_addr", c.ClientIP(),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid secret token",
			})
			return
		}
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request",
		})
		return
	}

	// Parse update
	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		h.logger.Error("Failed to unmarshal update", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid update format",
		})
		return
	}

	h.logger.Debug("Received webhook update",
		"update_id", update.UpdateID,
	)

	// Handle update
	if err := h.bot.HandleUpdate(update); err != nil {
		h.logger.Error("Failed to handle update",
			"update_id", update.UpdateID,
			"error", err,
		)
		// Still return 200 to Telegram to avoid retries
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
	})
}
