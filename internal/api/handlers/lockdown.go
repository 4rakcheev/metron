package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"metron/internal/core"

	"github.com/gin-gonic/gin"
)

// LockdownHandler handles global lockdown API endpoints
type LockdownHandler struct {
	storage core.LockdownStorage
	manager core.SessionManagerInterface
	logger  *slog.Logger
}

// NewLockdownHandler creates a new lockdown handler
func NewLockdownHandler(storage core.LockdownStorage, manager core.SessionManagerInterface, logger *slog.Logger) *LockdownHandler {
	return &LockdownHandler{
		storage: storage,
		manager: manager,
		logger:  logger,
	}
}

// GetLockdown returns the current lockdown state
// GET /v1/lockdown
func (h *LockdownHandler) GetLockdown(c *gin.Context) {
	state, err := h.storage.GetLockdown(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get lockdown state", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get lockdown state",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	var enabledAt *string
	if state.EnabledAt != nil {
		formatted := state.EnabledAt.Format(time.RFC3339)
		enabledAt = &formatted
	}
	var enabledBy *string
	if state.EnabledBy != "" {
		enabledBy = &state.EnabledBy
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":    state.Enabled,
		"enabled_at": enabledAt,
		"enabled_by": enabledBy,
	})
}

// EnableLockdown enables the global lockdown and stops all active sessions
// POST /v1/lockdown/enable
func (h *LockdownHandler) EnableLockdown(c *gin.Context) {
	ctx := c.Request.Context()

	// Optional body: {"enabled_by": "telegram"} for the audit trail
	var req struct {
		EnabledBy string `json:"enabled_by"`
	}
	_ = c.ShouldBindJSON(&req) // body is optional, ignore parse errors
	enabledBy := req.EnabledBy
	if enabledBy == "" {
		enabledBy = "api"
	}

	// Set the flag first so no new session can slip in while we stop the active ones
	if err := h.storage.SetLockdown(ctx, true, enabledBy); err != nil {
		h.logger.Error("Failed to enable lockdown", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to enable lockdown",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	// The stop-all loop makes external driver calls per session; run it on a
	// detached context so a client disconnect/timeout does not abort it midway.
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()

	stopped, failed := 0, 0
	message := "Lockdown enabled - all sessions are blocked until manually unlocked"

	sessions, err := h.manager.ListActiveSessions(stopCtx)
	if err != nil {
		h.logger.Error("Failed to list active sessions during lockdown enable", "error", err)
		message = "Lockdown enabled, but active sessions could not be listed - they will be ended by the scheduler within a minute"
	} else {
		for _, session := range sessions {
			if err := h.manager.StopSession(stopCtx, session.ID); err != nil {
				h.logger.Error("Failed to stop session during lockdown enable",
					"session_id", session.ID,
					"error", err)
				failed++
				continue
			}
			stopped++
		}
		if failed > 0 {
			message = fmt.Sprintf("Lockdown enabled, but %d session(s) could not be stopped - they will be ended by the scheduler within a minute", failed)
		}
	}

	h.logger.Info("Lockdown enabled", "enabled_by", enabledBy, "stopped_sessions", stopped, "failed_sessions", failed)

	c.JSON(http.StatusOK, gin.H{
		"enabled":          true,
		"stopped_sessions": stopped,
		"failed_sessions":  failed,
		"message":          message,
	})
}

// DisableLockdown disables the global lockdown
// POST /v1/lockdown/disable
func (h *LockdownHandler) DisableLockdown(c *gin.Context) {
	if err := h.storage.SetLockdown(c.Request.Context(), false, "api"); err != nil {
		h.logger.Error("Failed to disable lockdown", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to disable lockdown",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	h.logger.Info("Lockdown disabled")

	c.JSON(http.StatusOK, gin.H{
		"enabled": false,
		"message": "Lockdown disabled - sessions can be started again",
	})
}
