package handlers

import (
	"context"
	"log/slog"
	"metron/internal/core"
	"metron/internal/idgen"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MovieTimeBypassStorage defines the storage interface for movie time bypass operations
type MovieTimeBypassStorage interface {
	CreateMovieTimeBypass(ctx context.Context, bypass *core.MovieTimeBypass) error
	GetMovieTimeBypass(ctx context.Context, id string) (*core.MovieTimeBypass, error)
	ListMovieTimeBypasses(ctx context.Context) ([]*core.MovieTimeBypass, error)
	ListActiveMovieTimeBypasses(ctx context.Context, date time.Time) ([]*core.MovieTimeBypass, error)
	DeleteMovieTimeBypass(ctx context.Context, id string) error
}

// MovieTimeBypassHandler handles movie time bypass CRUD operations
type MovieTimeBypassHandler struct {
	storage MovieTimeBypassStorage
	logger  *slog.Logger
}

// NewMovieTimeBypassHandler creates a new movie time bypass handler
func NewMovieTimeBypassHandler(storage MovieTimeBypassStorage, logger *slog.Logger) *MovieTimeBypassHandler {
	return &MovieTimeBypassHandler{
		storage: storage,
		logger:  logger,
	}
}

// ListBypasses returns all movie time bypass periods
// GET /admin/movie-time/bypasses
func (h *MovieTimeBypassHandler) ListBypasses(c *gin.Context) {
	bypasses, err := h.storage.ListMovieTimeBypasses(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list movie time bypasses",
			"component", "api.movie_time_bypass",
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list bypasses",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Convert to response format
	response := make([]gin.H, len(bypasses))
	for i, bypass := range bypasses {
		response[i] = gin.H{
			"id":         bypass.ID,
			"reason":     bypass.Reason,
			"start_date": bypass.StartDate.Format("2006-01-02"),
			"end_date":   bypass.EndDate.Format("2006-01-02"),
			"created_at": bypass.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"bypasses": response,
	})
}

// CreateBypass creates a new movie time bypass period
// POST /admin/movie-time/bypasses
func (h *MovieTimeBypassHandler) CreateBypass(c *gin.Context) {
	var req struct {
		Reason    string `json:"reason" binding:"required"`
		StartDate string `json:"start_date" binding:"required"` // YYYY-MM-DD format
		EndDate   string `json:"end_date" binding:"required"`   // YYYY-MM-DD format
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid start_date format, expected YYYY-MM-DD",
			"code":  "INVALID_DATE",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid end_date format, expected YYYY-MM-DD",
			"code":  "INVALID_DATE",
		})
		return
	}

	// Validate date range
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "end_date must be on or after start_date",
			"code":  "INVALID_DATE_RANGE",
		})
		return
	}

	now := time.Now()
	bypass := &core.MovieTimeBypass{
		ID:        idgen.NewBypass(),
		Reason:    req.Reason,
		StartDate: startDate,
		EndDate:   endDate,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.storage.CreateMovieTimeBypass(c.Request.Context(), bypass); err != nil {
		h.logger.Error("Failed to create movie time bypass",
			"component", "api.movie_time_bypass",
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	h.logger.Info("Movie time bypass created",
		"component", "api.movie_time_bypass",
		"bypass_id", bypass.ID,
		"reason", bypass.Reason,
		"start_date", req.StartDate,
		"end_date", req.EndDate)

	c.JSON(http.StatusCreated, gin.H{
		"id":         bypass.ID,
		"reason":     bypass.Reason,
		"start_date": req.StartDate,
		"end_date":   req.EndDate,
		"created_at": bypass.CreatedAt,
	})
}

// GetBypass returns a specific movie time bypass by ID
// GET /admin/movie-time/bypasses/:id
func (h *MovieTimeBypassHandler) GetBypass(c *gin.Context) {
	id := c.Param("id")

	bypass, err := h.storage.GetMovieTimeBypass(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get movie time bypass",
			"component", "api.movie_time_bypass",
			"bypass_id", id,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	if bypass == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Bypass not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         bypass.ID,
		"reason":     bypass.Reason,
		"start_date": bypass.StartDate.Format("2006-01-02"),
		"end_date":   bypass.EndDate.Format("2006-01-02"),
		"created_at": bypass.CreatedAt,
	})
}

// DeleteBypass deletes a movie time bypass by ID
// DELETE /admin/movie-time/bypasses/:id
func (h *MovieTimeBypassHandler) DeleteBypass(c *gin.Context) {
	id := c.Param("id")

	// Check if bypass exists
	bypass, err := h.storage.GetMovieTimeBypass(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get movie time bypass for deletion",
			"component", "api.movie_time_bypass",
			"bypass_id", id,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	if bypass == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Bypass not found",
			"code":  "NOT_FOUND",
		})
		return
	}

	if err := h.storage.DeleteMovieTimeBypass(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete movie time bypass",
			"component", "api.movie_time_bypass",
			"bypass_id", id,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	h.logger.Info("Movie time bypass deleted",
		"component", "api.movie_time_bypass",
		"bypass_id", id)

	c.JSON(http.StatusOK, gin.H{
		"message": "Bypass deleted successfully",
	})
}
