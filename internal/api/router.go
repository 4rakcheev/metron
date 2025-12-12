package api

import (
	"log/slog"
	"metron/internal/api/handlers"
	"metron/internal/api/middleware"
	"metron/internal/core"
	"metron/internal/devices"
	"metron/internal/drivers"
	"metron/internal/drivers/aqara"
	"metron/internal/storage"

	"github.com/gin-gonic/gin"
)

// RouterConfig holds dependencies for the API router
type RouterConfig struct {
	Storage           storage.Storage
	Manager           *core.SessionManager
	DriverRegistry    *drivers.Registry
	DeviceRegistry    *devices.Registry
	APIKey            string
	Logger            *slog.Logger
	AqaraTokenStorage aqara.AqaraTokenStorage // Optional: only needed if Aqara driver is used
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
		v1.POST("/children", childrenHandler.CreateChild)
		v1.GET("/children/:id", childrenHandler.GetChild)
		v1.PATCH("/children/:id", childrenHandler.UpdateChild)
		v1.DELETE("/children/:id", childrenHandler.DeleteChild)

		// Devices endpoints
		devicesHandler := handlers.NewDevicesHandler(
			config.DeviceRegistry,
			config.DriverRegistry,
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

		// Admin endpoints (only register if Aqara token storage is provided)
		if config.AqaraTokenStorage != nil {
			adminHandler := handlers.NewAdminHandler(
				config.AqaraTokenStorage,
				config.Logger,
			)
			v1.POST("/admin/aqara/refresh-token", adminHandler.UpdateAqaraRefreshToken)
			v1.GET("/admin/aqara/token-status", adminHandler.GetAqaraTokenStatus)
		}
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
