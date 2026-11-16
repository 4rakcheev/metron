package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"metron/internal/core"
	"metron/internal/storage"

	"github.com/gin-gonic/gin"
)

// ApprovalHandler handles the daily approval gate endpoints (parent side).
// On required days a child can start sessions from the child app only after
// a parent approves the child for today.
type ApprovalHandler struct {
	gate    *core.ApprovalGateService
	storage storage.Storage
	logger  *slog.Logger
}

// NewApprovalHandler creates a new approval handler
func NewApprovalHandler(gate *core.ApprovalGateService, storage storage.Storage, logger *slog.Logger) *ApprovalHandler {
	return &ApprovalHandler{gate: gate, storage: storage, logger: logger}
}

func formatApprovalStatus(child *core.Child, st *core.ApprovalStatus) gin.H {
	out := gin.H{
		"child_id":    child.ID,
		"child_name":  child.Name,
		"child_emoji": child.Emoji,
		"required":    st.Required,
		"approved":    st.Approved,
		"approved_at": nil,
		"approved_by": nilIfEmpty(st.ApprovedBy),
	}
	if st.ApprovedAt != nil {
		out["approved_at"] = st.ApprovedAt.Format(time.RFC3339)
	}
	return out
}

// ListTodayApprovals returns today's approval status for every child
// GET /v1/approvals/today
func (h *ApprovalHandler) ListTodayApprovals(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()

	children, err := h.storage.ListChildren(ctx)
	if err != nil {
		h.logger.Error("Failed to list children for approvals", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list children",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	items := make([]gin.H, 0, len(children))
	for _, child := range children {
		st, err := h.gate.GetApprovalStatus(ctx, child.ID, now)
		if err != nil {
			h.logger.Error("Failed to get approval status", "child_id", child.ID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to get approval status",
				"code":  "APPROVAL_ERROR",
			})
			return
		}
		items = append(items, formatApprovalStatus(child, st))
	}

	c.JSON(http.StatusOK, gin.H{
		"required": h.gate.IsApprovalRequired(now),
		"children": items,
	})
}

// ApproveChild approves a child for the rest of today
// POST /v1/children/:id/approval
func (h *ApprovalHandler) ApproveChild(c *gin.Context) {
	ctx := c.Request.Context()
	childID := c.Param("id")

	var req struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"code":    "INVALID_REQUEST",
			"details": err.Error(),
		})
		return
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "api"
	}

	child, err := h.storage.GetChild(ctx, childID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Child not found",
			"code":  "CHILD_NOT_FOUND",
		})
		return
	}

	now := time.Now()
	if _, err := h.gate.Approve(ctx, childID, now, req.ApprovedBy); err != nil {
		h.logger.Error("Failed to approve child", "child_id", childID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to approve child",
			"code":  "APPROVAL_ERROR",
		})
		return
	}

	st, err := h.gate.GetApprovalStatus(ctx, childID, now)
	if err != nil {
		h.logger.Error("Failed to read approval after save", "child_id", childID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get approval status",
			"code":  "APPROVAL_ERROR",
		})
		return
	}

	h.logger.Info("Child approved for today", "child_id", childID, "approved_by", req.ApprovedBy)
	c.JSON(http.StatusCreated, formatApprovalStatus(child, st))
}

// RevokeApproval removes today's approval for a child. Running sessions are not stopped.
// DELETE /v1/children/:id/approval
func (h *ApprovalHandler) RevokeApproval(c *gin.Context) {
	ctx := c.Request.Context()
	childID := c.Param("id")

	if _, err := h.storage.GetChild(ctx, childID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Child not found",
			"code":  "CHILD_NOT_FOUND",
		})
		return
	}

	if err := h.gate.Revoke(ctx, childID, time.Now()); err != nil {
		h.logger.Error("Failed to revoke approval", "child_id", childID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to revoke approval",
			"code":  "APPROVAL_ERROR",
		})
		return
	}

	h.logger.Info("Child approval revoked for today", "child_id", childID)
	c.Status(http.StatusNoContent)
}
