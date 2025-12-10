package handlers

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"metron/internal/storage"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ChildrenHandler handles children-related requests
type ChildrenHandler struct {
	storage storage.Storage
	manager SessionManager
	logger  *slog.Logger
}

// SessionManager interface for child status operations
type SessionManager interface {
	GetChildStatus(ctx context.Context, childID string) (*core.ChildStatus, error)
}

// NewChildrenHandler creates a new children handler
func NewChildrenHandler(storage storage.Storage, manager SessionManager, logger *slog.Logger) *ChildrenHandler {
	return &ChildrenHandler{
		storage: storage,
		manager: manager,
		logger:  logger,
	}
}

// ListChildren returns all children
// GET /children
func (h *ChildrenHandler) ListChildren(c *gin.Context) {
	children, err := h.storage.ListChildren(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list children",
			"component", "api",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve children",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Transform to response format
	response := make([]gin.H, 0, len(children))
	for _, child := range children {
		response = append(response, gin.H{
			"id":            child.ID,
			"name":          child.Name,
			"weekday_limit": child.WeekdayLimit,
			"weekend_limit": child.WeekendLimit,
			"break_rule":    formatBreakRule(child.BreakRule),
			"created_at":    child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at":    child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetChild returns a single child by ID
// GET /children/:id
func (h *ChildrenHandler) GetChild(c *gin.Context) {
	childID := c.Param("id")

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
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve child",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Get child status
	status, err := h.manager.GetChildStatus(c.Request.Context(), childID)
	if err != nil {
		h.logger.Error("Failed to get child status",
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve child status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              child.ID,
		"name":            child.Name,
		"weekday_limit":   child.WeekdayLimit,
		"weekend_limit":   child.WeekendLimit,
		"break_rule":      formatBreakRule(child.BreakRule),
		"created_at":      child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":      child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"today_used":      status.TodayUsed,
		"today_remaining": status.TodayRemaining,
		"today_limit":     status.TodayLimit,
		"sessions_today":  status.SessionsToday,
	})
}

func formatBreakRule(rule *core.BreakRule) interface{} {
	if rule == nil {
		return nil
	}
	return gin.H{
		"break_after_minutes":    rule.BreakAfterMinutes,
		"break_duration_minutes": rule.BreakDurationMinutes,
	}
}
