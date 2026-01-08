package handlers

import (
	"context"
	"log/slog"
	"math/rand"
	"metron/internal/core"
	"metron/internal/idgen"
	"metron/internal/storage"
	"net/http"
	"strings"
	"time"

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
	GrantRewardMinutes(ctx context.Context, childID string, minutes int) error
	DeductFineMinutes(ctx context.Context, childID string, minutes int) error
}

// NewChildrenHandler creates a new children handler
func NewChildrenHandler(storage storage.Storage, manager SessionManager, logger *slog.Logger) *ChildrenHandler {
	return &ChildrenHandler{
		storage: storage,
		manager: manager,
		logger:  logger,
	}
}

// getRandomEmoji returns a random emoji from a predefined list of child-appropriate emojis
func getRandomEmoji() string {
	emojis := []string{
		"👦", // Boy
		"👧", // Girl
		"👶", // Baby
		"🧒", // Child
		"🧑", // Person
		"😊", // Smiling face
		"😀", // Grinning face
		"🎮", // Video game
		"🎨", // Artist palette
		"🎭", // Performing arts
		"🎪", // Circus tent
		"🎯", // Direct hit
		"🎸", // Guitar
		"🎺", // Trumpet
		"🎹", // Musical keyboard
		"⚽", // Soccer ball
		"🏀", // Basketball
		"🎾", // Tennis
		"🏐", // Volleyball
		"🎳", // Bowling
		"🎲", // Game die
		"🧩", // Puzzle piece
		"🎬", // Clapper board
		"📚", // Books
		"🚀", // Rocket
		"🌟", // Glowing star
		"⭐", // Star
		"🌈", // Rainbow
		"🦄", // Unicorn
		"🐶", // Dog face
		"🐱", // Cat face
		"🐼", // Panda
		"🐨", // Koala
		"🦁", // Lion
		"🐯", // Tiger face
		"🦊", // Fox
		"🐰", // Rabbit face
		"🐻", // Bear
	}

	// Use crypto-based random for better randomness
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	return emojis[r.Intn(len(emojis))]
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
			"id":               child.ID,
			"name":             child.Name,
			"emoji":            child.Emoji,
			"weekday_limit":    child.WeekdayLimit,
			"weekend_limit":    child.WeekendLimit,
			"break_rule":       formatBreakRule(child.BreakRule),
			"downtime_enabled": child.DowntimeEnabled,
			"created_at":       child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at":       child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
		"id":                   child.ID,
		"name":                 child.Name,
		"emoji":                child.Emoji,
		"pin":                  child.PIN,
		"weekday_limit":        child.WeekdayLimit,
		"weekend_limit":        child.WeekendLimit,
		"break_rule":           formatBreakRule(child.BreakRule),
		"downtime_enabled":     child.DowntimeEnabled,
		"created_at":           child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":           child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"today_used":           status.TodayUsed,
		"today_reward_granted": status.TodayRewardGranted,
		"today_remaining":      status.TodayRemaining,
		"today_limit":          status.TodayLimit,
		"sessions_today":       status.SessionsToday,
	})
}

// CreateChild creates a new child
// POST /children
func (h *ChildrenHandler) CreateChild(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Emoji        string `json:"emoji,omitempty"` // Optional emoji, will be randomly assigned if empty
		PIN          string `json:"pin,omitempty"`   // Optional 4-digit PIN
		WeekdayLimit int    `json:"weekday_limit" binding:"required,gt=0"`
		WeekendLimit int    `json:"weekend_limit" binding:"required,gt=0"`
		BreakRule    *struct {
			BreakAfterMinutes    int `json:"break_after_minutes" binding:"required,gt=0"`
			BreakDurationMinutes int `json:"break_duration_minutes" binding:"required,gt=0"`
		} `json:"break_rule,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Assign random emoji if not provided
	emoji := req.Emoji
	if emoji == "" {
		emoji = getRandomEmoji()
	}

	// Create child model
	child := &core.Child{
		ID:           idgen.NewChild(),
		Name:         req.Name,
		Emoji:        emoji,
		PIN:          req.PIN, // Store PIN (can be empty string)
		WeekdayLimit: req.WeekdayLimit,
		WeekendLimit: req.WeekendLimit,
	}

	// Add break rule if provided
	if req.BreakRule != nil {
		child.BreakRule = &core.BreakRule{
			BreakAfterMinutes:    req.BreakRule.BreakAfterMinutes,
			BreakDurationMinutes: req.BreakRule.BreakDurationMinutes,
		}
	}

	// Validate
	if err := child.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "VALIDATION_ERROR",
		})
		return
	}

	// Save to storage
	if err := h.storage.CreateChild(c.Request.Context(), child); err != nil {
		h.logger.Error("Failed to create child",
			"component", "api",
			"name", req.Name,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create child",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":               child.ID,
		"name":             child.Name,
		"emoji":            child.Emoji,
		"pin":              child.PIN,
		"weekday_limit":    child.WeekdayLimit,
		"weekend_limit":    child.WeekendLimit,
		"break_rule":       formatBreakRule(child.BreakRule),
		"downtime_enabled": child.DowntimeEnabled,
		"created_at":       child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":       child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateChild updates an existing child
// PATCH /children/:id
func (h *ChildrenHandler) UpdateChild(c *gin.Context) {
	childID := c.Param("id")

	// Get existing child
	child, err := h.storage.GetChild(c.Request.Context(), childID)
	if err != nil {
		if err == core.ErrChildNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Child not found",
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}

		h.logger.Error("Failed to get child for update",
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

	// Parse update request
	var req struct {
		Name            *string `json:"name,omitempty"`
		Emoji           *string `json:"emoji,omitempty"`
		PIN             *string `json:"pin,omitempty"` // Optional PIN update
		WeekdayLimit    *int    `json:"weekday_limit,omitempty"`
		WeekendLimit    *int    `json:"weekend_limit,omitempty"`
		DowntimeEnabled *bool   `json:"downtime_enabled,omitempty"`
		BreakRule       *struct {
			BreakAfterMinutes    int `json:"break_after_minutes" binding:"required,gt=0"`
			BreakDurationMinutes int `json:"break_duration_minutes" binding:"required,gt=0"`
		} `json:"break_rule,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Update fields if provided
	if req.Name != nil {
		child.Name = *req.Name
	}
	if req.Emoji != nil {
		if *req.Emoji == "" {
			// If explicitly set to empty, assign a random emoji
			child.Emoji = getRandomEmoji()
		} else {
			child.Emoji = *req.Emoji
		}
	}
	if req.PIN != nil {
		child.PIN = *req.PIN
	}
	if req.WeekdayLimit != nil {
		child.WeekdayLimit = *req.WeekdayLimit
	}
	if req.WeekendLimit != nil {
		child.WeekendLimit = *req.WeekendLimit
	}
	if req.DowntimeEnabled != nil {
		child.DowntimeEnabled = *req.DowntimeEnabled
	}
	if req.BreakRule != nil {
		child.BreakRule = &core.BreakRule{
			BreakAfterMinutes:    req.BreakRule.BreakAfterMinutes,
			BreakDurationMinutes: req.BreakRule.BreakDurationMinutes,
		}
	}

	// Validate
	if err := child.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "VALIDATION_ERROR",
		})
		return
	}

	// Save
	if err := h.storage.UpdateChild(c.Request.Context(), child); err != nil {
		h.logger.Error("Failed to update child",
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update child",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":               child.ID,
		"name":             child.Name,
		"emoji":            child.Emoji,
		"pin":              child.PIN,
		"weekday_limit":    child.WeekdayLimit,
		"weekend_limit":    child.WeekendLimit,
		"break_rule":       formatBreakRule(child.BreakRule),
		"downtime_enabled": child.DowntimeEnabled,
		"created_at":       child.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":       child.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GrantReward grants reward minutes to a child
// POST /children/:id/rewards
func (h *ChildrenHandler) GrantReward(c *gin.Context) {
	childID := c.Param("id")

	var req struct {
		Minutes int `json:"minutes" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Validate minutes is one of the allowed values
	validMinutes := map[int]bool{15: true, 30: true, 60: true}
	if !validMinutes[req.Minutes] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Minutes must be one of: 15, 30, or 60",
			"code":  "INVALID_MINUTES",
		})
		return
	}

	// Grant reward minutes
	if err := h.manager.GrantRewardMinutes(c.Request.Context(), childID, req.Minutes); err != nil {
		if err == core.ErrChildNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Child not found",
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}

		h.logger.Error("Failed to grant reward minutes",
			"component", "api",
			"child_id", childID,
			"minutes", req.Minutes,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to grant reward minutes",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Get updated child status
	status, err := h.manager.GetChildStatus(c.Request.Context(), childID)
	if err != nil {
		h.logger.Error("Failed to get child status after reward grant",
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve updated child status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Reward granted successfully",
		"minutes_granted":      req.Minutes,
		"today_reward_granted": status.TodayRewardGranted,
		"today_remaining":      status.TodayRemaining,
		"today_limit":          status.TodayLimit,
	})
}

// DeductFine deducts fine minutes from a child
// POST /children/:id/fines
func (h *ChildrenHandler) DeductFine(c *gin.Context) {
	childID := c.Param("id")

	var req struct {
		Minutes int `json:"minutes" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Validate minutes is one of the allowed values
	validMinutes := map[int]bool{15: true, 30: true, 60: true}
	if !validMinutes[req.Minutes] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Minutes must be one of: 15, 30, or 60",
			"code":  "INVALID_MINUTES",
		})
		return
	}

	// Deduct fine minutes
	if err := h.manager.DeductFineMinutes(c.Request.Context(), childID, req.Minutes); err != nil {
		if err == core.ErrChildNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Child not found",
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}

		// Check for insufficient time error
		if strings.HasPrefix(err.Error(), "insufficient time") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
				"code":  "INSUFFICIENT_TIME",
			})
			return
		}

		h.logger.Error("Failed to deduct fine minutes",
			"component", "api",
			"child_id", childID,
			"minutes", req.Minutes,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to deduct fine minutes",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Get updated child status
	status, err := h.manager.GetChildStatus(c.Request.Context(), childID)
	if err != nil {
		h.logger.Error("Failed to get child status after fine deduction",
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve updated child status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Calculate fines deducted (negative portion of TodayRewardGranted)
	todayFinesDeducted := 0
	if status.TodayRewardGranted < 0 {
		todayFinesDeducted = -status.TodayRewardGranted
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Fine applied successfully",
		"minutes_deducted":     req.Minutes,
		"today_fines_deducted": todayFinesDeducted,
		"today_remaining":      status.TodayRemaining,
		"today_limit":          status.TodayLimit,
	})
}

// DeleteChild deletes a child
// DELETE /children/:id
func (h *ChildrenHandler) DeleteChild(c *gin.Context) {
	childID := c.Param("id")

	// Check if child exists
	_, err := h.storage.GetChild(c.Request.Context(), childID)
	if err != nil {
		if err == core.ErrChildNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Child not found",
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}

		h.logger.Error("Failed to get child for deletion",
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

	// Delete
	if err := h.storage.DeleteChild(c.Request.Context(), childID); err != nil {
		h.logger.Error("Failed to delete child",
			"component", "api",
			"child_id", childID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete child",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
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
