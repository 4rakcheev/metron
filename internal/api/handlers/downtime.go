package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"metron/internal/core"

	"github.com/gin-gonic/gin"
)

// DowntimeHandler handles downtime-related API endpoints
type DowntimeHandler struct {
	storage  core.DowntimeSkipStorage
	downtime *core.DowntimeService
	logger   *slog.Logger
}

// NewDowntimeHandler creates a new downtime handler
func NewDowntimeHandler(storage core.DowntimeSkipStorage, downtime *core.DowntimeService, logger *slog.Logger) *DowntimeHandler {
	return &DowntimeHandler{
		storage:  storage,
		downtime: downtime,
		logger:   logger,
	}
}

// SkipDowntimeToday skips downtime for today (all children)
// POST /v1/downtime/skip-today
func (h *DowntimeHandler) SkipDowntimeToday(c *gin.Context) {
	ctx := c.Request.Context()
	today := time.Now()

	if err := h.storage.SetDowntimeSkipDate(ctx, today); err != nil {
		h.logger.Error("Failed to set downtime skip date", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to skip downtime",
			"code":  "SKIP_DOWNTIME_ERROR",
		})
		return
	}

	h.logger.Info("Downtime skipped for today", "date", today.Format("2006-01-02"))

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"skip_date": today.Format("2006-01-02"),
		"message":   "Downtime skipped for today (all children)",
	})
}

// GetSkipStatus returns the current skip status for downtime
// GET /v1/downtime/skip-status
func (h *DowntimeHandler) GetSkipStatus(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()

	skippedToday := h.downtime.IsDowntimeSkippedToday(ctx, now)

	var skipDateStr *string
	skipDate, err := h.storage.GetDowntimeSkipDate(context.Background())
	if err == nil && skipDate != nil {
		formatted := skipDate.Format("2006-01-02")
		skipDateStr = &formatted
	}

	c.JSON(http.StatusOK, gin.H{
		"skipped_today": skippedToday,
		"skip_date":     skipDateStr,
	})
}
