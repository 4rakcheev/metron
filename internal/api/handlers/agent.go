package handlers

import (
	"context"
	"log/slog"
	"metron/internal/api/middleware"
	"metron/internal/core"
	"metron/internal/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const warningMinutes = 5

// AgentHandler handles agent-related requests
type AgentHandler struct {
	storage storage.Storage
	manager AgentSessionManager
	logger  *slog.Logger
}

// AgentSessionManager interface for session operations needed by agents
type AgentSessionManager interface {
	ListActiveSessions(ctx context.Context) ([]*core.Session, error)
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(storage storage.Storage, manager AgentSessionManager, logger *slog.Logger) *AgentHandler {
	return &AgentHandler{
		storage: storage,
		manager: manager,
		logger:  logger.With("component", "agent-api"),
	}
}

// GetDeviceSession returns the session status for a specific device.
// Used by external agents (e.g., Windows agent) to poll for active sessions.
// GET /v1/agent/session?device_id=xxx
func (h *AgentHandler) GetDeviceSession(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "device_id query parameter required",
			"code":  "DEVICE_ID_REQUIRED",
		})
		return
	}

	// Verify the authenticated agent is authorized for this device
	// The middleware sets the device_id from the token
	authorizedDeviceID, exists := c.Get(middleware.AgentDeviceIDKey)
	if !exists || authorizedDeviceID != deviceID {
		h.logger.Warn("agent attempted to access unauthorized device",
			"requested_device", deviceID,
			"authorized_device", authorizedDeviceID,
		)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Not authorized for this device",
			"code":  "DEVICE_NOT_AUTHORIZED",
		})
		return
	}

	ctx := c.Request.Context()
	now := time.Now()

	// Check bypass mode first
	bypass, err := h.storage.GetDeviceBypass(ctx, deviceID)
	if err != nil {
		h.logger.Error("failed to get device bypass",
			"device_id", deviceID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check bypass status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// If bypass is active, return early
	if bypass != nil && bypass.IsActive() {
		h.logger.Debug("device bypass active",
			"device_id", deviceID,
			"reason", bypass.Reason,
			"expires_at", bypass.ExpiresAt,
		)
		c.JSON(http.StatusOK, gin.H{
			"active":      false,
			"bypass_mode": true,
			"server_time": now.Format(time.RFC3339),
		})
		return
	}

	// If bypass expired, clear it
	if bypass != nil && bypass.IsExpired() {
		if err := h.storage.ClearDeviceBypass(ctx, deviceID); err != nil {
			h.logger.Warn("failed to clear expired bypass",
				"device_id", deviceID,
				"error", err,
			)
		}
	}

	// Get active sessions and find one for this device
	sessions, err := h.manager.ListActiveSessions(ctx)
	if err != nil {
		h.logger.Error("failed to list active sessions",
			"device_id", deviceID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve sessions",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	// Find active session for this device
	var activeSession *core.Session
	for _, session := range sessions {
		if session.DeviceID == deviceID && session.Status == core.SessionStatusActive {
			activeSession = session
			break
		}
	}

	// No active session
	if activeSession == nil {
		c.JSON(http.StatusOK, gin.H{
			"active":      false,
			"bypass_mode": false,
			"server_time": now.Format(time.RFC3339),
		})
		return
	}

	// Calculate times
	endsAt := activeSession.StartTime.Add(time.Duration(activeSession.ExpectedDuration) * time.Minute)
	warnAt := endsAt.Add(-warningMinutes * time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"active":      true,
		"session_id":  activeSession.ID,
		"ends_at":     endsAt.Format(time.RFC3339),
		"warn_at":     warnAt.Format(time.RFC3339),
		"server_time": now.Format(time.RFC3339),
		"bypass_mode": false,
	})
}

// SetDeviceBypass enables or disables bypass mode for a device.
// POST /v1/devices/:id/bypass
func (h *AgentHandler) SetDeviceBypass(c *gin.Context) {
	deviceID := c.Param("id")

	var req struct {
		Enabled          bool   `json:"enabled"`
		Reason           string `json:"reason,omitempty"`
		ExpiresInMinutes *int   `json:"expires_in_minutes,omitempty"` // nil = indefinite
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	now := time.Now()

	if !req.Enabled {
		// Clearing bypass
		if err := h.storage.ClearDeviceBypass(ctx, deviceID); err != nil {
			h.logger.Error("failed to clear device bypass",
				"device_id", deviceID,
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to clear bypass",
				"code":  "INTERNAL_ERROR",
			})
			return
		}

		h.logger.Info("device bypass cleared",
			"device_id", deviceID,
		)

		c.JSON(http.StatusOK, gin.H{
			"device_id": deviceID,
			"enabled":   false,
		})
		return
	}

	// Setting bypass
	bypass := &core.DeviceBypass{
		DeviceID:  deviceID,
		Enabled:   true,
		Reason:    req.Reason,
		EnabledAt: now,
		EnabledBy: "api", // Could be enriched with user info from context
	}

	if req.ExpiresInMinutes != nil && *req.ExpiresInMinutes > 0 {
		expiresAt := now.Add(time.Duration(*req.ExpiresInMinutes) * time.Minute)
		bypass.ExpiresAt = &expiresAt
	}

	if err := h.storage.SetDeviceBypass(ctx, bypass); err != nil {
		h.logger.Error("failed to set device bypass",
			"device_id", deviceID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to set bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	h.logger.Info("device bypass enabled",
		"device_id", deviceID,
		"reason", req.Reason,
		"expires_in_minutes", req.ExpiresInMinutes,
	)

	response := gin.H{
		"device_id":  deviceID,
		"enabled":    true,
		"reason":     bypass.Reason,
		"enabled_at": bypass.EnabledAt.Format(time.RFC3339),
	}
	if bypass.ExpiresAt != nil {
		response["expires_at"] = bypass.ExpiresAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}

// ClearDeviceBypass removes bypass mode for a device.
// DELETE /v1/devices/:id/bypass
func (h *AgentHandler) ClearDeviceBypass(c *gin.Context) {
	deviceID := c.Param("id")
	ctx := c.Request.Context()

	if err := h.storage.ClearDeviceBypass(ctx, deviceID); err != nil {
		h.logger.Error("failed to clear device bypass",
			"device_id", deviceID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clear bypass",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	h.logger.Info("device bypass cleared via DELETE",
		"device_id", deviceID,
	)

	c.JSON(http.StatusNoContent, nil)
}
