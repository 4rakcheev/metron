package api

import (
	"log/slog"
	"metron/internal/api/handlers"
	"metron/internal/api/middleware"
	"metron/internal/core"
	"metron/internal/drivers"
	"metron/internal/storage"

	"github.com/gin-gonic/gin"
)

// RouterConfig holds dependencies for the API router
type RouterConfig struct {
	Storage  storage.Storage
	Manager  *core.SessionManager
	Registry *drivers.Registry
	APIKey   string
	Logger   *slog.Logger
}

// NewRouter creates and configures the Gin router
func NewRouter(config RouterConfig) *gin.Engine {
	// Set Gin mode based on logger
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Apply global middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(config.Logger))
	router.Use(middleware.Logging(config.Logger))
	router.Use(middleware.ContentType())

	// Health check (no auth)
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.GetHealth)

	// API v1 routes (with authentication)
	v1 := router.Group("/v1")
	v1.Use(authMiddleware(config.APIKey))
	{
		// Children endpoints
		childrenHandler := handlers.NewChildrenHandler(
			config.Storage,
			config.Manager,
			config.Logger,
		)
		v1.GET("/children", childrenHandler.ListChildren)
		v1.GET("/children/:id", childrenHandler.GetChild)

		// Devices endpoints
		devicesHandler := handlers.NewDevicesHandler(
			config.Registry,
			config.Logger,
		)
		v1.GET("/devices", devicesHandler.ListDevices)

		// Sessions endpoints
		sessionsHandler := handlers.NewSessionsHandler(
			config.Storage,
			config.Manager,
			config.Logger,
		)
		v1.GET("/sessions", sessionsHandler.ListSessions)
		v1.POST("/sessions", sessionsHandler.CreateSession)
		v1.GET("/sessions/:id", sessionsHandler.GetSession)
		v1.PATCH("/sessions/:id", sessionsHandler.UpdateSession)

		// Stats endpoints
		statsHandler := handlers.NewStatsHandler(
			config.Storage,
			config.Manager,
			config.Logger,
		)
		v1.GET("/stats/today", statsHandler.GetTodayStats)
	}

	return router
}

// authMiddleware verifies API key authentication
func authMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		providedKey := c.GetHeader("X-Metron-Key")
		if providedKey != apiKey {
			c.JSON(401, gin.H{
				"error": "Unauthorized",
				"code":  "UNAUTHORIZED",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
