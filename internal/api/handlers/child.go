package handlers

import (
	"log/slog"
	"metron/internal/api/middleware"
	"metron/internal/core"
	"metron/internal/devices"
	"metron/internal/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ChildHandler handles child-facing API requests
type ChildHandler struct {
	storage        storage.Storage
	manager        FullSessionManager
	deviceRegistry *devices.Registry
	sessionManager *middleware.SessionManager
	downtime       *core.DowntimeService
	logger         *slog.Logger
}

// NewChildHandler creates a new child handler
func NewChildHandler(
	storage storage.Storage,
	manager FullSessionManager,
	deviceRegistry *devices.Registry,
	sessionManager *middleware.SessionManager,
	downtime *core.DowntimeService,
	logger *slog.Logger,
) *ChildHandler {
	return &ChildHandler{
		storage:        storage,
		manager:        manager,
		deviceRegistry: deviceRegistry,
		sessionManager: sessionManager,
		downtime:       downtime,
		logger:         logger,
	}
}

// ListChildrenForAuth returns all children for the login screen
// GET /child/auth/children (PUBLIC - no auth required)
func (h *ChildHandler) ListChildrenForAuth(c *gin.Context) {
	children, err := h.storage.ListChildren(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list children for auth",
			"component", "child-api",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve children",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Return only ID and name (no PINs!)
	response := make([]gin.H, 0, len(children))
	for _, child := range children {
		response = append(response, gin.H{
			"id":   child.ID,
			"name": child.Name,
		})
	}

	c.JSON(http.StatusOK, response)
}

// Login handles child authentication
// POST /child/auth/login (PUBLIC - no auth required)
func (h *ChildHandler) Login(c *gin.Context) {
	var req struct {
		ChildID string `json:"child_id" binding:"required"`
		PIN     string `json:"pin" binding:"required,len=4"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Get child from database
	child, err := h.storage.GetChild(c.Request.Context(), req.ChildID)
	if err != nil {
		if err == core.ErrChildNotFound {
			// Don't reveal if child exists or not
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid credentials",
				"code":  "INVALID_CREDENTIALS",
			})
			return
		}
		h.logger.Error("Failed to get child for login",
			"child_id", req.ChildID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to authenticate",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Verify PIN (compare with bcrypt hash if PIN is hashed, otherwise direct comparison)
	// For now, we'll do direct comparison for simplicity
	// In production, you should hash PINs with bcrypt
	if child.PIN != req.PIN {
		// If stored PIN starts with $2, it's bcrypt hashed
		if len(child.PIN) > 2 && child.PIN[:2] == "$2" {
			// Verify bcrypt hash
			if err := bcrypt.CompareHashAndPassword([]byte(child.PIN), []byte(req.PIN)); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid credentials",
					"code":  "INVALID_CREDENTIALS",
				})
				return
			}
		} else {
			// Direct comparison for non-hashed PINs
			if child.PIN != req.PIN {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid credentials",
					"code":  "INVALID_CREDENTIALS",
				})
				return
			}
		}
	}

	// Create session
	sessionID := h.sessionManager.CreateSession(child.ID)

	// Set cookie
	c.SetCookie(
		"child_session", // name
		sessionID,       // value
		24*60*60,        // maxAge (24 hours)
		"/",             // path
		"",              // domain (empty = current domain)
		false,           // secure (set to true in production with HTTPS)
		true,            // httpOnly
	)

	// Return session info and child data
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"child": gin.H{
			"id":            child.ID,
			"name":          child.Name,
			"weekday_limit": child.WeekdayLimit,
			"weekend_limit": child.WeekendLimit,
		},
	})

	h.logger.Info("Child logged in",
		"child_id", child.ID,
		"child_name", child.Name,
	)
}

// Logout handles child logout
// POST /child/auth/logout (PUBLIC - no auth required, but session ID needed)
func (h *ChildHandler) Logout(c *gin.Context) {
	// Get session ID from cookie or header
	sessionID, err := c.Cookie("child_session")
	if err != nil || sessionID == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			sessionID = authHeader[7:]
		}
	}

	if sessionID != "" {
		h.sessionManager.DeleteSession(sessionID)

		// Clear cookie
		c.SetCookie(
			"child_session",
			"",
			-1, // maxAge = -1 deletes the cookie
			"/",
			"",
			false,
			true,
		)
	}

	c.Status(http.StatusNoContent)
}

// GetMe returns the authenticated child's profile
// GET /child/me (PROTECTED)
func (h *ChildHandler) GetMe(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)

	child, err := h.storage.GetChild(c.Request.Context(), childID)
	if err != nil {
		if err == core.ErrChildNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Child not found",
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}
		h.logger.Error("Failed to get child",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve profile",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Return child data without PIN
	c.JSON(http.StatusOK, gin.H{
		"id":            child.ID,
		"name":          child.Name,
		"weekday_limit": child.WeekdayLimit,
		"weekend_limit": child.WeekendLimit,
	})
}

// GetToday returns today's usage stats for the authenticated child
// GET /child/today (PROTECTED)
func (h *ChildHandler) GetToday(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)

	status, err := h.manager.GetChildStatus(c.Request.Context(), childID)
	if err != nil {
		h.logger.Error("Failed to get child status",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Get child to check downtime status
	child, err := h.storage.GetChild(c.Request.Context(), childID)
	if err != nil {
		h.logger.Error("Failed to get child for downtime check",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve child",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	response := gin.H{
		"used_minutes":      status.TodayUsed,
		"remaining_minutes": status.TodayRemaining,
		"daily_limit":       status.TodayLimit,
		"sessions_count":    status.SessionsToday,
		"downtime_enabled":  child.DowntimeEnabled,
	}

	// Add downtime active status if downtime is enabled
	if h.downtime != nil && child.DowntimeEnabled {
		response["in_downtime"] = h.downtime.IsInDowntime(time.Now())
		if h.downtime.IsInDowntime(time.Now()) {
			downtimeEnd := h.downtime.GetCurrentDowntimeEnd(time.Now())
			if !downtimeEnd.IsZero() {
				response["downtime_end"] = downtimeEnd.Format("2006-01-02T15:04:05Z07:00")
			}
		}
	} else {
		response["in_downtime"] = false
	}

	c.JSON(http.StatusOK, response)
}

// ListDevices returns available devices
// GET /child/devices (PROTECTED)
func (h *ChildHandler) ListDevices(c *gin.Context) {
	deviceList := h.deviceRegistry.List()

	response := make([]gin.H, 0, len(deviceList))
	for _, device := range deviceList {
		response = append(response, gin.H{
			"id":   device.ID,
			"name": device.Name,
			"type": device.Type,
		})
	}

	c.JSON(http.StatusOK, response)
}

// ListSessions returns the child's active sessions
// GET /child/sessions (PROTECTED)
func (h *ChildHandler) ListSessions(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)

	allSessions, err := h.storage.ListActiveSessions(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list active sessions",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve sessions",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Filter to sessions involving this child
	childSessions := make([]*core.Session, 0)
	for _, session := range allSessions {
		for _, sid := range session.ChildIDs {
			if sid == childID {
				childSessions = append(childSessions, session)
				break
			}
		}
	}

	response := make([]gin.H, 0, len(childSessions))
	for _, session := range childSessions {
		response = append(response, gin.H{
			"id":                session.ID,
			"device_id":         session.DeviceID,
			"device_type":       session.DeviceType,
			"start_time":        session.StartTime.Format("2006-01-02T15:04:05Z07:00"),
			"remaining_minutes": session.CalculateRemainingMinutes(),
			"status":            string(session.Status),
		})
	}

	c.JSON(http.StatusOK, response)
}

// CreateSession starts a new session for the authenticated child
// POST /child/sessions (PROTECTED)
func (h *ChildHandler) CreateSession(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)

	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
		Minutes  int    `json:"minutes" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Start session (only for this child)
	session, err := h.manager.StartSession(c.Request.Context(), req.DeviceID, []string{childID}, req.Minutes)
	if err != nil {
		h.logger.Error("Failed to start session",
			"child_id", childID,
			"device_id", req.DeviceID,
			"minutes", req.Minutes,
			"error", err,
		)

		if err == core.ErrInsufficientTime {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Not enough time remaining",
				"code":  "INSUFFICIENT_TIME",
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "SESSION_CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":                session.ID,
		"device_id":         session.DeviceID,
		"device_type":       session.DeviceType,
		"start_time":        session.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		"remaining_minutes": session.CalculateRemainingMinutes(),
		"status":            string(session.Status),
	})
}

// StopSession stops a session (validates ownership)
// POST /child/sessions/:id/stop (PROTECTED)
func (h *ChildHandler) StopSession(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)
	sessionID := c.Param("id")

	// Get session to validate ownership
	session, err := h.manager.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		if err == core.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Session not found",
				"code":  "SESSION_NOT_FOUND",
			})
			return
		}
		h.logger.Error("Failed to get session",
			"session_id", sessionID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve session",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Validate child is part of this session
	isOwner := false
	for _, sid := range session.ChildIDs {
		if sid == childID {
			isOwner = true
			break
		}
	}

	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have permission to stop this session",
			"code":  "FORBIDDEN",
		})
		return
	}

	// Stop the session
	if err := h.manager.StopSession(c.Request.Context(), sessionID); err != nil {
		h.logger.Error("Failed to stop session",
			"child_id", childID,
			"session_id", sessionID,
			"error", err,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "SESSION_STOP_FAILED",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// ExtendSession extends a session (validates ownership)
// POST /child/sessions/:id/extend (PROTECTED)
func (h *ChildHandler) ExtendSession(c *gin.Context) {
	childID, _ := middleware.GetChildID(c)
	sessionID := c.Param("id")

	// Parse request body
	var req struct {
		AdditionalMinutes int `json:"additional_minutes" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request. additional_minutes must be a positive integer",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Get session to validate ownership
	session, err := h.manager.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		if err == core.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Session not found",
				"code":  "SESSION_NOT_FOUND",
			})
			return
		}
		h.logger.Error("Failed to get session",
			"session_id", sessionID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve session",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Validate child is part of this session
	isOwner := false
	for _, sid := range session.ChildIDs {
		if sid == childID {
			isOwner = true
			break
		}
	}

	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You don't have permission to extend this session",
			"code":  "FORBIDDEN",
		})
		return
	}

	// Extend the session
	extendedSession, err := h.manager.ExtendSession(c.Request.Context(), sessionID, req.AdditionalMinutes)
	if err != nil {
		h.logger.Error("Failed to extend session",
			"child_id", childID,
			"session_id", sessionID,
			"additional_minutes", req.AdditionalMinutes,
			"error", err,
		)

		if err == core.ErrInsufficientTime {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
				"code":  "INSUFFICIENT_TIME",
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "SESSION_EXTEND_FAILED",
		})
		return
	}

	// Return extended session
	c.JSON(http.StatusOK, gin.H{
		"id":                extendedSession.ID,
		"device_type":       extendedSession.DeviceType,
		"device_id":         extendedSession.DeviceID,
		"start_time":        extendedSession.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		"remaining_minutes": extendedSession.CalculateRemainingMinutes(),
		"status":            string(extendedSession.Status),
	})
}
