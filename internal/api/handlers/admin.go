package handlers

import (
	"log/slog"
	"metron/internal/drivers/aqara"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AdminHandler handles administrative operations for Aqara driver
type AdminHandler struct {
	storage aqara.AqaraTokenStorage
	logger  *slog.Logger
}

// NewAdminHandler creates a new admin handler for Aqara operations
func NewAdminHandler(storage aqara.AqaraTokenStorage, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{
		storage: storage,
		logger:  logger,
	}
}

// UpdateAqaraRefreshToken updates the Aqara refresh token
// POST /admin/aqara/refresh-token
func (h *AdminHandler) UpdateAqaraRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"code":  "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	// Get existing tokens (if any)
	tokens, err := h.storage.GetAqaraTokens(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get existing tokens",
			"component", "api.admin",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve existing tokens",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Create or update tokens
	if tokens == nil {
		tokens = &aqara.AqaraTokens{
			RefreshToken: req.RefreshToken,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
	} else {
		tokens.RefreshToken = req.RefreshToken
		tokens.UpdatedAt = time.Now()
		// Clear access token to force refresh on next use
		tokens.AccessToken = ""
		tokens.AccessTokenExpiresAt = nil
	}

	// Save to database
	if err := h.storage.SaveAqaraTokens(c.Request.Context(), tokens); err != nil {
		h.logger.Error("Failed to save refresh token",
			"component", "api.admin",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save refresh token",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	h.logger.Info("Aqara refresh token updated successfully",
		"component", "api.admin",
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Refresh token updated successfully",
		"updated_at": tokens.UpdatedAt,
	})
}

// GetAqaraTokenStatus returns the status of Aqara tokens
// GET /admin/aqara/token-status
func (h *AdminHandler) GetAqaraTokenStatus(c *gin.Context) {
	tokens, err := h.storage.GetAqaraTokens(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get tokens",
			"component", "api.admin",
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve token status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	if tokens == nil {
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
			"message": "No refresh token configured. Use POST /v1/admin/aqara/refresh-token to add one.",
		})
		return
	}

	var accessTokenStatus string
	var accessTokenExpiresIn *int

	if tokens.AccessToken == "" {
		accessTokenStatus = "not_cached"
	} else if tokens.AccessTokenExpiresAt == nil {
		accessTokenStatus = "cached_no_expiry"
	} else if time.Now().After(*tokens.AccessTokenExpiresAt) {
		accessTokenStatus = "expired"
	} else {
		accessTokenStatus = "valid"
		expiresIn := int(time.Until(*tokens.AccessTokenExpiresAt).Seconds())
		accessTokenExpiresIn = &expiresIn
	}

	response := gin.H{
		"configured": true,
		"refresh_token_updated_at": tokens.UpdatedAt,
		"access_token_status": accessTokenStatus,
	}

	if accessTokenExpiresIn != nil {
		response["access_token_expires_in_seconds"] = *accessTokenExpiresIn
	}

	c.JSON(http.StatusOK, response)
}
