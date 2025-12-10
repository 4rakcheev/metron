package bot

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// RouterConfig holds dependencies for the bot router
type RouterConfig struct {
	Bot           *Bot
	WebhookSecret string
	Logger        *slog.Logger
}

// NewRouter creates and configures the Gin router for the bot webhook
func NewRouter(config RouterConfig) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())

	// Create webhook handler
	webhookHandler := NewWebhookHandler(
		config.Bot,
		config.WebhookSecret,
		config.Logger,
	)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "UP",
			"service": "metron-bot",
		})
	})

	// Webhook endpoint
	router.POST("/telegram/webhook", webhookHandler.HandleWebhook)

	return router
}
