package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"metron/internal/core"
	"metron/internal/devices"
	"metron/internal/storage"

	"github.com/gin-gonic/gin"
)

// LockdownHandler handles lockdown rule API endpoints.
// A rule blocks a child (or all children) on a device (or all devices)
// until it is removed or, for "end_of_day" rules, until local midnight.
type LockdownHandler struct {
	lockdown       core.LockdownStorage
	storage        storage.Storage
	manager        core.SessionManagerInterface
	deviceRegistry *devices.Registry
	timezone       *time.Location
	logger         *slog.Logger
}

// NewLockdownHandler creates a new lockdown handler
func NewLockdownHandler(
	lockdown core.LockdownStorage,
	storage storage.Storage,
	manager core.SessionManagerInterface,
	deviceRegistry *devices.Registry,
	timezone *time.Location,
	logger *slog.Logger,
) *LockdownHandler {
	if timezone == nil {
		timezone = time.UTC
	}
	return &LockdownHandler{
		lockdown:       lockdown,
		storage:        storage,
		manager:        manager,
		deviceRegistry: deviceRegistry,
		timezone:       timezone,
		logger:         logger,
	}
}

// formatRule builds the wire format of a rule. Wildcards are returned as null IDs.
func (h *LockdownHandler) formatRule(ctx context.Context, r *core.LockdownRule) gin.H {
	out := gin.H{
		"id":          r.ID,
		"child_id":    nil,
		"child_name":  "All children",
		"device_id":   nil,
		"device_name": "All devices",
		"reason":      nilIfEmpty(r.Reason),
		"enabled_at":  r.EnabledAt.Format(time.RFC3339),
		"enabled_by":  nilIfEmpty(r.EnabledBy),
		"expires_at":  nil,
	}
	if r.ChildID != core.LockdownAll {
		out["child_id"] = r.ChildID
		out["child_name"] = r.ChildID
		if child, err := h.storage.GetChild(ctx, r.ChildID); err == nil {
			out["child_name"] = child.Name
		}
	}
	if r.DeviceID != core.LockdownAll {
		out["device_id"] = r.DeviceID
		out["device_name"] = r.DeviceID
		if h.deviceRegistry != nil {
			if device, err := h.deviceRegistry.Get(r.DeviceID); err == nil {
				out["device_name"] = device.Name
			}
		}
	}
	if r.ExpiresAt != nil {
		out["expires_at"] = r.ExpiresAt.Format(time.RFC3339)
	}
	return out
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ListLockdowns returns all active lockdown rules
// GET /v1/lockdowns
func (h *LockdownHandler) ListLockdowns(c *gin.Context) {
	ctx := c.Request.Context()
	rules, err := h.lockdown.ListActiveLockdowns(ctx, time.Now())
	if err != nil {
		h.logger.Error("Failed to list lockdown rules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list lockdowns",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	out := make([]gin.H, 0, len(rules))
	for _, r := range rules {
		out = append(out, h.formatRule(ctx, r))
	}
	c.JSON(http.StatusOK, out)
}

// CreateLockdown creates (or replaces) a lockdown rule and stops matching active sessions
// POST /v1/lockdowns
func (h *LockdownHandler) CreateLockdown(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		ChildID   string `json:"child_id"`  // empty = all children
		DeviceID  string `json:"device_id"` // empty = all devices
		Until     string `json:"until"`     // "manual" (default) or "end_of_day"
		Reason    string `json:"reason"`
		EnabledBy string `json:"enabled_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}

	rule := &core.LockdownRule{
		ChildID:   core.LockdownAll,
		DeviceID:  core.LockdownAll,
		Reason:    req.Reason,
		EnabledAt: time.Now(),
		EnabledBy: req.EnabledBy,
	}
	if rule.EnabledBy == "" {
		rule.EnabledBy = "api"
	}

	if req.ChildID != "" && req.ChildID != core.LockdownAll {
		if _, err := h.storage.GetChild(ctx, req.ChildID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("Child %s not found", req.ChildID),
				"code":  "CHILD_NOT_FOUND",
			})
			return
		}
		rule.ChildID = req.ChildID
	}
	if req.DeviceID != "" && req.DeviceID != core.LockdownAll {
		if h.deviceRegistry != nil {
			if _, err := h.deviceRegistry.Get(req.DeviceID); err != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error": fmt.Sprintf("Device %s not found", req.DeviceID),
					"code":  "DEVICE_NOT_FOUND",
				})
				return
			}
		}
		rule.DeviceID = req.DeviceID
	}

	expiresAt, err := core.LockdownExpiry(req.Until, rule.EnabledAt, h.timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "INVALID_REQUEST",
		})
		return
	}
	rule.ExpiresAt = expiresAt

	// Save the rule first so no new session can slip in while we stop the active ones
	if err := h.lockdown.AddLockdown(ctx, rule); err != nil {
		h.logger.Error("Failed to create lockdown", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create lockdown",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	stopped, failed, message := h.stopMatchingSessions(ctx, rule)

	h.logger.Info("Lockdown created",
		"lockdown_id", rule.ID,
		"child_id", rule.ChildID,
		"device_id", rule.DeviceID,
		"expires_at", rule.ExpiresAt,
		"enabled_by", rule.EnabledBy,
		"stopped_sessions", stopped,
		"failed_sessions", failed)

	resp := h.formatRule(ctx, rule)
	resp["stopped_sessions"] = stopped
	resp["failed_sessions"] = failed
	resp["message"] = message
	c.JSON(http.StatusCreated, resp)
}

// stopMatchingSessions stops active sessions covered by the rule.
// The loop makes external driver calls per session; it runs on a detached
// context so a client disconnect/timeout does not abort it midway.
func (h *LockdownHandler) stopMatchingSessions(ctx context.Context, rule *core.LockdownRule) (stopped, failed int, message string) {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()

	message = "Lockdown created - matching sessions are blocked"

	sessions, err := h.manager.ListActiveSessions(stopCtx)
	if err != nil {
		h.logger.Error("Failed to list active sessions during lockdown", "error", err)
		return 0, 0, "Lockdown created, but active sessions could not be listed - they will be ended by the scheduler within a minute"
	}

	for _, session := range sessions {
		if !rule.Matches(session.DeviceID, session.ChildIDs) {
			continue
		}
		if err := h.manager.StopSession(stopCtx, session.ID); err != nil {
			h.logger.Error("Failed to stop session during lockdown",
				"session_id", session.ID,
				"error", err)
			failed++
			continue
		}
		stopped++
	}
	if failed > 0 {
		message = fmt.Sprintf("Lockdown created, but %d session(s) could not be stopped - they will be ended by the scheduler within a minute", failed)
	}
	return stopped, failed, message
}

// DeleteLockdown removes a lockdown rule
// DELETE /v1/lockdowns/:id
func (h *LockdownHandler) DeleteLockdown(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid lockdown ID",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	if err := h.lockdown.RemoveLockdown(c.Request.Context(), id); err != nil {
		if errors.Is(err, core.ErrLockdownNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Lockdown not found",
				"code":  "LOCKDOWN_NOT_FOUND",
			})
			return
		}
		h.logger.Error("Failed to remove lockdown", "lockdown_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to remove lockdown",
			"code":  "LOCKDOWN_ERROR",
		})
		return
	}

	h.logger.Info("Lockdown removed", "lockdown_id", id)
	c.Status(http.StatusNoContent)
}
