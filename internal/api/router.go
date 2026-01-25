package api

import (
	"log/slog"
	"metron/config"
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
	Storage             storage.Storage
	Manager             core.SessionManagerInterface
	DriverRegistry      *drivers.Registry
	DeviceRegistry      *devices.Registry
	Downtime            *core.DowntimeService
	MovieTime           *core.MovieTimeService   // Optional: for weekend movie time feature
	DowntimeSkipStorage core.DowntimeSkipStorage // For skip downtime feature
	APIKey              string
	Logger              *slog.Logger
	AqaraTokenStorage   aqara.AqaraTokenStorage  // Optional: only needed if Aqara driver is used
	Devices             []config.DeviceConfig    // All devices (used for agent auth)
}

// NewRouter creates and configures the Gin router
func NewRouter(config RouterConfig) *gin.Engine {
	// Set Gin mode based on logger
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Apply global middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(config.Logger))
	router.Use(middleware.NoiseFilter(config.Logger))
	router.Use(middleware.Logging(config.Logger))
	router.Use(middleware.ContentType())

	// Apply child API logging middleware (adds detailed logging for child API routes)
	childLogger := config.Logger.With("component", "child-api")
	router.Use(middleware.ChildAPILogging(childLogger))

	// CORS middleware for child web app
	router.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			// Allow the requesting origin (supports credentials)
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// Fallback for non-browser requests
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

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
		v1.POST("/children/:id/rewards", childrenHandler.GrantReward)
		v1.POST("/children/:id/fines", childrenHandler.DeductFine)

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

		// Downtime endpoints (only register if downtime service is configured)
		if config.DowntimeSkipStorage != nil && config.Downtime != nil {
			downtimeHandler := handlers.NewDowntimeHandler(
				config.DowntimeSkipStorage,
				config.Downtime,
				config.Logger,
			)
			v1.POST("/downtime/skip-today", downtimeHandler.SkipDowntimeToday)
			v1.GET("/downtime/skip-status", downtimeHandler.GetSkipStatus)
		}

		// Movie time bypass endpoints (for holiday/vacation periods)
		if config.MovieTime != nil {
			bypassHandler := handlers.NewMovieTimeBypassHandler(
				config.Storage,
				config.Logger,
			)
			v1.GET("/admin/movie-time/bypasses", bypassHandler.ListBypasses)
			v1.POST("/admin/movie-time/bypasses", bypassHandler.CreateBypass)
			v1.GET("/admin/movie-time/bypasses/:id", bypassHandler.GetBypass)
			v1.DELETE("/admin/movie-time/bypasses/:id", bypassHandler.DeleteBypass)
		}
	}

	// Child API routes (for child-facing web app)
	sessionManager := middleware.NewSessionManager()

	childGroup := router.Group("/child")
	{
		childHandler := handlers.NewChildHandler(
			config.Storage,
			config.Manager,
			config.DeviceRegistry,
			sessionManager,
			config.Downtime,
			config.MovieTime,
			config.Logger,
		)

		// Public routes (no auth required)
		authGroup := childGroup.Group("/auth")
		authGroup.GET("/children", childHandler.ListChildrenForAuth)
		authGroup.POST("/login", childHandler.Login)
		authGroup.POST("/logout", childHandler.Logout)

		// Protected routes (require child session)
		protected := childGroup.Group("")
		protected.Use(middleware.ChildAuth(sessionManager))
		protected.GET("/me", childHandler.GetMe)
		protected.GET("/today", childHandler.GetToday)
		protected.GET("/devices", childHandler.ListDevices)
		protected.GET("/sessions", childHandler.ListSessions)
		protected.POST("/sessions", childHandler.CreateSession)
		protected.POST("/sessions/:id/stop", childHandler.StopSession)
		protected.POST("/sessions/:id/extend", childHandler.ExtendSession)

		// Movie time routes (for weekend shared movie time)
		protected.GET("/movie-time", childHandler.GetMovieTimeAvailability)
		protected.POST("/movie-time", childHandler.StartMovieTime)
	}

	// Agent API routes (for external device agents like Windows agent)
	// Only register if any devices have agent tokens configured
	if middleware.HasAgentDevices(config.Devices) {
		agentHandler := handlers.NewAgentHandler(
			config.Storage,
			config.Manager,
			config.Logger,
		)

		agentGroup := router.Group("/v1/agent")
		agentGroup.Use(middleware.AgentAuth(config.Devices))
		{
			agentGroup.GET("/session", agentHandler.GetDeviceSession)
		}

		// Device bypass endpoints (admin auth, not agent auth)
		// These are managed by admin, not by agents themselves
		v1.POST("/devices/:id/bypass", agentHandler.SetDeviceBypass)
		v1.DELETE("/devices/:id/bypass", agentHandler.ClearDeviceBypass)
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
