package handlers

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"metron/internal/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StatsHandler handles statistics-related requests
type StatsHandler struct {
	storage storage.Storage
	manager StatsSessionManager
	logger  *slog.Logger
}

// StatsSessionManager interface for stats operations
type StatsSessionManager interface {
	GetChildStatus(ctx context.Context, childID string) (*core.ChildStatus, error)
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(storage storage.Storage, manager StatsSessionManager, logger *slog.Logger) *StatsHandler {
	return &StatsHandler{
		storage: storage,
		manager: manager,
		logger:  logger,
	}
}

// GetTodayStats returns today's statistics for all children
// GET /stats/today
func (h *StatsHandler) GetTodayStats(c *gin.Context) {
	children, err := h.storage.ListChildren(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list children for stats",
			"component", "api",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve statistics",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	today := time.Now()
	childStats := make([]gin.H, 0, len(children))

	for _, child := range children {
		status, err := h.manager.GetChildStatus(c.Request.Context(), child.ID)
		if err != nil {
			h.logger.Error("Failed to get child status for stats",
				"component", "api",
				"child_id", child.ID,
				"error", err,
			)
			continue
		}

		childStats = append(childStats, gin.H{
			"child_id":        child.ID,
			"child_name":      child.Name,
			"today_used":      status.TodayUsed,
			"today_remaining": status.TodayRemaining,
			"today_limit":     status.TodayLimit,
			"sessions_today":  status.SessionsToday,
			"usage_percent":   calculateUsagePercent(status.TodayUsed, status.TodayLimit),
		})
	}

	// Get active sessions
	activeSessions, err := h.storage.ListActiveSessions(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list active sessions for stats",
			"component", "api",
			"error", err,
		)
		// Continue without active sessions
		activeSessions = []*core.Session{}
	}

	response := gin.H{
		"date":            today.Format("2006-01-02"),
		"children":        childStats,
		"active_sessions": len(activeSessions),
		"total_children":  len(children),
	}

	c.JSON(http.StatusOK, response)
}

func calculateUsagePercent(used, limit int) int {
	if limit == 0 {
		return 0
	}
	percent := (used * 100) / limit
	if percent > 100 {
		return 100
	}
	return percent
}
