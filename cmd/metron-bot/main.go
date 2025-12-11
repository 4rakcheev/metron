package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"metron/config"
	"metron/internal/bot"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultConfigPath = "bot-config.json"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	logFormat := flag.String("log-format", "json", "Log format (json or text)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logger
	logger := initLogger(*logFormat, *logLevel)

	logger.Info("Starting Metron Telegram Bot",
		"config", *configPath,
	)

	// Load configuration
	cfg, err := config.LoadBotConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("Configuration loaded",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"webhook_url", cfg.Telegram.WebhookURL,
		"metron_url", cfg.Metron.BaseURL,
		"allowed_users", len(cfg.Telegram.AllowedUsers),
	)

	// Create bot instance
	telegramBot, err := bot.NewBot(cfg, logger)
	if err != nil {
		logger.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	logger.Info("Bot created successfully")

	// Configure webhook
	if err := telegramBot.SetWebhook(); err != nil {
		logger.Error("Failed to set webhook", "error", err)
		os.Exit(1)
	}

	logger.Info("Webhook configured successfully")

	// Create HTTP router
	router := bot.NewRouter(bot.RouterConfig{
		Bot:           telegramBot,
		WebhookSecret: cfg.Telegram.WebhookSecret,
		Logger:        logger,
	})

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &gin.Engine{}
	server = router

	// Start server in goroutine
	go func() {
		logger.Info("Starting HTTP server",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port,
			"addr", addr)
		if err := server.Run(addr); err != nil {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down bot...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// In a real implementation, we would shut down the HTTP server gracefully here
	// For now, we just log the shutdown
	_ = ctx

	logger.Info("Bot stopped")
}

// initLogger initializes the structured logger
func initLogger(format, levelStr string) *slog.Logger {
	// Parse log level
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
