package handlers

import (
	"errors"
	"net/http"

	"metron/internal/core"

	"github.com/gin-gonic/gin"
)

// writeChildSessionError maps session start/extend errors for the child app.
// Order matters only for readability: each core error is distinct.
func writeChildSessionError(c *gin.Context, err error, fallbackCode string) {
	switch {
	case errors.Is(err, core.ErrLockdownActive):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Sessions are locked by your parents",
			"code":  "LOCKDOWN_ACTIVE",
		})
	case errors.Is(err, core.ErrDowntimeActive):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "It is downtime now - screen time is not available",
			"code":  "DOWNTIME_ACTIVE",
		})
	case errors.Is(err, core.ErrApprovalRequired):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Your parents need to allow screen time today first",
			"code":  "APPROVAL_REQUIRED",
		})
	case errors.Is(err, core.ErrInsufficientTime):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Not enough time remaining",
			"code":  "INSUFFICIENT_TIME",
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  fallbackCode,
		})
	}
}

// writeParentSessionError maps session errors for the parent/admin API (/v1/sessions)
func writeParentSessionError(c *gin.Context, err error, fallbackCode string) {
	switch {
	case errors.Is(err, core.ErrSessionNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
			"code":  "SESSION_NOT_FOUND",
		})
	case errors.Is(err, core.ErrSessionNotActive):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session is not active",
			"code":  "SESSION_NOT_ACTIVE",
		})
	case errors.Is(err, core.ErrLockdownActive):
		c.JSON(http.StatusConflict, gin.H{
			"error": err.Error(),
			"code":  "LOCKDOWN_ACTIVE",
		})
	case errors.Is(err, core.ErrDowntimeActive):
		c.JSON(http.StatusConflict, gin.H{
			"error": err.Error(),
			"code":  "DOWNTIME_ACTIVE",
		})
	case errors.Is(err, core.ErrInsufficientTime):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "INSUFFICIENT_TIME",
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  fallbackCode,
		})
	}
}
