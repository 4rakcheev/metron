package handlers

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"metron/internal/storage"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionsHandler handles session-related requests
type SessionsHandler struct {
	storage storage.Storage
	manager FullSessionManager
	logger  *slog.Logger
}

// FullSessionManager interface for all session operations
type FullSessionManager interface {
	GetChildStatus(ctx context.Context, childID string) (*core.ChildStatus, error)
	StartSession(ctx context.Context, deviceID string, childIDs []string, durationMinutes int) (*core.Session, error)
	ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*core.Session, error)
	StopSession(ctx context.Context, sessionID string) error
	AddChildrenToSession(ctx context.Context, sessionID string, childIDs []string) (*core.Session, error)
	GetSession(ctx context.Context, sessionID string) (*core.Session, error)
	ListActiveSessions(ctx context.Context) ([]*core.Session, error)
}

// NewSessionsHandler creates a new sessions handler
func NewSessionsHandler(storage storage.Storage, manager FullSessionManager, logger *slog.Logger) *SessionsHandler {
	return &SessionsHandler{
		storage: storage,
		manager: manager,
		logger:  logger,
	}
}

// ListSessions returns sessions with optional filtering
// GET /sessions?childId=&active=&date=
func (h *SessionsHandler) ListSessions(c *gin.Context) {
	childID := c.Query("childId")
	activeStr := c.Query("active")
	dateStr := c.Query("date")

	var sessions []*core.Session
	var err error

	// Filter by active status
	if activeStr == "true" {
		sessions, err = h.manager.ListActiveSessions(c.Request.Context())
		if err != nil {
			h.logger.Error("Failed to list active sessions",
				"component", "api",
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve sessions",
				"code":  "INTERNAL_ERROR",
			})
			return
		}
	} else if childID != "" {
		sessions, err = h.storage.ListSessionsByChild(c.Request.Context(), childID)
		if err != nil {
			h.logger.Error("Failed to list sessions by child",
				"component", "api",
				"child_id", childID,
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve sessions",
				"code":  "INTERNAL_ERROR",
			})
			return
		}
	} else {
		// No filters specified - return all sessions
		sessions, err = h.storage.ListAllSessions(c.Request.Context())
		if err != nil {
			h.logger.Error("Failed to list all sessions",
				"component", "api",
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve sessions",
				"code":  "INTERNAL_ERROR",
			})
			return
		}
	}

	// Filter by date if specified
	if dateStr != "" {
		filterDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid date format. Use YYYY-MM-DD",
				"code":  "INVALID_DATE_FORMAT",
			})
			return
		}

		filtered := make([]*core.Session, 0)
		for _, session := range sessions {
			if isSameDay(session.StartTime, filterDate) {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}

	// Transform to response format
	response := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		response = append(response, formatSessionResponse(session))
	}

	c.JSON(http.StatusOK, response)
}

// CreateSession creates a new session
// POST /sessions
func (h *SessionsHandler) CreateSession(c *gin.Context) {
	var req struct {
		DeviceID string   `json:"device_id" binding:"required"`
		ChildIDs []string `json:"child_ids" binding:"required"`
		Minutes  int      `json:"minutes" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	session, err := h.manager.StartSession(c.Request.Context(), req.DeviceID, req.ChildIDs, req.Minutes)
	if err != nil {
		h.logger.Error("Failed to start session",
			"component", "api",
			"device_id", req.DeviceID,
			"child_ids", req.ChildIDs,
			"minutes", req.Minutes,
			"error", err,
		)

		// Map known errors to appropriate status codes
		if err == core.ErrInsufficientTime {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
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

	c.JSON(http.StatusCreated, formatSessionResponse(session))
}

// GetSession returns a single session by ID
// GET /sessions/:id
func (h *SessionsHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")

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
			"component", "api",
			"session_id", sessionID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve session",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, formatSessionResponse(session))
}

// UpdateSession updates a session (extend or stop)
// PATCH /sessions/:id
func (h *SessionsHandler) UpdateSession(c *gin.Context) {
	sessionID := c.Param("id")

	var req struct {
		Action            string   `json:"action"` // "extend", "stop", or "add_children"
		AdditionalMinutes int      `json:"additional_minutes,omitempty"`
		ChildIDs          []string `json:"child_ids,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	switch strings.ToLower(req.Action) {
	case "extend":
		if req.AdditionalMinutes <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "additional_minutes must be positive",
				"code":  "INVALID_MINUTES",
			})
			return
		}

		session, err := h.manager.ExtendSession(c.Request.Context(), sessionID, req.AdditionalMinutes)
		if err != nil {
			h.logger.Error("Failed to extend session",
				"component", "api",
				"session_id", sessionID,
				"additional_minutes", req.AdditionalMinutes,
				"error", err,
			)

			if err == core.ErrSessionNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Session not found",
					"code":  "SESSION_NOT_FOUND",
				})
				return
			}

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

		c.JSON(http.StatusOK, formatSessionResponse(session))

	case "stop":
		err := h.manager.StopSession(c.Request.Context(), sessionID)
		if err != nil {
			h.logger.Error("Failed to stop session",
				"component", "api",
				"session_id", sessionID,
				"error", err,
			)

			if err == core.ErrSessionNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Session not found",
					"code":  "SESSION_NOT_FOUND",
				})
				return
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
				"code":  "SESSION_STOP_FAILED",
			})
			return
		}

		c.JSON(http.StatusNoContent, nil)

	case "add_children":
		if len(req.ChildIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "child_ids must not be empty",
				"code":  "INVALID_CHILD_IDS",
			})
			return
		}

		session, err := h.manager.AddChildrenToSession(c.Request.Context(), sessionID, req.ChildIDs)
		if err != nil {
			h.logger.Error("Failed to add children to session",
				"component", "api",
				"session_id", sessionID,
				"child_ids", req.ChildIDs,
				"error", err,
			)

			if err == core.ErrSessionNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Session not found",
					"code":  "SESSION_NOT_FOUND",
				})
				return
			}

			if err == core.ErrSessionNotActive {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Session is not active",
					"code":  "SESSION_NOT_ACTIVE",
				})
				return
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
				"code":  "ADD_CHILDREN_FAILED",
			})
			return
		}

		c.JSON(http.StatusOK, formatSessionResponse(session))

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action. Must be 'extend', 'stop', or 'add_children'",
			"code":  "INVALID_ACTION",
		})
	}
}

// Helper functions

func formatSessionResponse(session *core.Session) gin.H {
	response := gin.H{
		"id":                session.ID,
		"device_type":       session.DeviceType,
		"device_id":         session.DeviceID,
		"child_ids":         session.ChildIDs,
		"start_time":        session.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		"expected_duration": session.ExpectedDuration,
		"remaining_minutes": session.CalculateRemainingMinutes(),
		"status":            string(session.Status),
		"created_at":        session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":        session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if session.LastBreakAt != nil {
		response["last_break_at"] = session.LastBreakAt.Format("2006-01-02T15:04:05Z07:00")
	}

	if session.BreakEndsAt != nil {
		response["break_ends_at"] = session.BreakEndsAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return response
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
