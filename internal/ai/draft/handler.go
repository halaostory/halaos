package draft

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/pkg/response"
)

// ToolExecutor executes a tool by name and returns the result string.
type ToolExecutor interface {
	Execute(ctx context.Context, name string, companyID, userID int64, input map[string]any) (string, error)
}

// Handler handles draft-related HTTP endpoints.
type Handler struct {
	service  *Service
	executor ToolExecutor
}

// NewHandler creates a draft handler.
func NewHandler(service *Service, executor ToolExecutor) *Handler {
	return &Handler{service: service, executor: executor}
}

// RegisterRoutes adds draft routes to the router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	drafts := rg.Group("/ai/drafts")
	{
		drafts.GET("", h.ListPending)
		drafts.GET("/:id", h.GetDraft)
		drafts.POST("/:id/confirm", h.ConfirmDraft)
		drafts.POST("/:id/reject", h.RejectDraft)
	}
}

// ListPending returns pending drafts for the current user.
func (h *Handler) ListPending(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	drafts, err := h.service.ListPending(c.Request.Context(), companyID, userID)
	if err != nil {
		response.InternalError(c, "Failed to list drafts")
		return
	}
	response.OK(c, drafts)
}

// GetDraft returns a specific draft.
func (h *Handler) GetDraft(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	draftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid draft ID")
		return
	}

	draft, err := h.service.Get(c.Request.Context(), companyID, userID, draftID)
	if err != nil {
		response.NotFound(c, "Draft not found")
		return
	}
	response.OK(c, draft)
}

// ConfirmDraft confirms and executes a pending draft.
func (h *Handler) ConfirmDraft(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	draftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid draft ID")
		return
	}

	// Confirm the draft (checks pending + not expired)
	d, err := h.service.Confirm(c.Request.Context(), companyID, userID, draftID)
	if err != nil {
		response.BadRequest(c, "Draft cannot be confirmed (expired or already processed)")
		return
	}

	// Parse stored tool input
	var toolInput map[string]any
	if err := json.Unmarshal(d.ToolInput, &toolInput); err != nil {
		response.InternalError(c, "Invalid draft data")
		return
	}

	// Execute the tool
	result, execErr := h.executor.Execute(c.Request.Context(), d.ToolName, companyID, userID, toolInput)
	if execErr != nil {
		errMsg := execErr.Error()
		_ = h.service.MarkFailed(c.Request.Context(), draftID, errMsg)
		c.JSON(http.StatusOK, gin.H{
			"success":  false,
			"draft_id": draftID.String(),
			"error":    fmt.Sprintf("Tool execution failed: %s", errMsg),
		})
		return
	}

	// Mark as executed
	_ = h.service.MarkExecuted(c.Request.Context(), draftID, result)

	response.OK(c, gin.H{
		"draft_id": draftID.String(),
		"status":   "executed",
		"result":   json.RawMessage(result),
	})
}

// RejectDraft rejects a pending draft.
func (h *Handler) RejectDraft(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	draftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid draft ID")
		return
	}

	if err := h.service.Reject(c.Request.Context(), companyID, userID, draftID); err != nil {
		response.InternalError(c, "Failed to reject draft")
		return
	}

	response.OK(c, gin.H{
		"draft_id": draftID.String(),
		"status":   "rejected",
	})
}
