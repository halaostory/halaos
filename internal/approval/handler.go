package approval

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries    *store.Queries
	pool       *pgxpool.Pool
	logger     *slog.Logger
	aiProvider provider.Provider
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// SetAIProvider sets the optional AI provider for smart context recommendations.
func (h *Handler) SetAIProvider(p provider.Provider) {
	h.aiProvider = p
}

// ListPending returns pending approvals for the current user's employee record.
func (h *Handler) ListPending(c *gin.Context) {
	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}

	approvals, err := h.queries.ListPendingApprovals(c.Request.Context(), emp.ID)
	if err != nil {
		response.InternalError(c, "Failed to list approvals")
		return
	}
	response.OK(c, approvals)
}

// Approve approves a pending workflow item.
func (h *Handler) Approve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid approval ID")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	var req struct {
		Comments *string `json:"comments"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.queries.ApproveWorkflow(c.Request.Context(), store.ApproveWorkflowParams{
		ID:         id,
		ApproverID: emp.ID,
		Comments:   req.Comments,
	}); err != nil {
		response.NotFound(c, "Approval not found or already processed")
		return
	}

	// Record AI decision feedback if applicable
	h.recordDecisionFeedback(c.Request.Context(), companyID, userID, id, "approved")

	response.OK(c, gin.H{"message": "Approved"})
}

// Reject rejects a pending workflow item.
func (h *Handler) Reject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid approval ID")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}

	var req struct {
		Comments *string `json:"comments"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.queries.RejectWorkflow(c.Request.Context(), store.RejectWorkflowParams{
		ID:         id,
		ApproverID: emp.ID,
		Comments:   req.Comments,
	}); err != nil {
		response.NotFound(c, "Approval not found or already processed")
		return
	}

	// Record AI decision feedback if applicable
	h.recordDecisionFeedback(c.Request.Context(), companyID, userID, id, "rejected")

	response.OK(c, gin.H{"message": "Rejected"})
}

// recordDecisionFeedback checks if there's an AI decision for the entity
// associated with an approval workflow, and records the override if the
// manager's action differs from the AI recommendation.
func (h *Handler) recordDecisionFeedback(ctx context.Context, companyID, userID, approvalWorkflowID int64, managerAction string) {
	// Look up the entity associated with this approval workflow
	entity, err := h.queries.GetApprovalWorkflowEntity(ctx, approvalWorkflowID)
	if err != nil {
		return
	}

	// Check if there's an AI decision for this entity
	decision, err := h.queries.GetDecisionForEntity(ctx, store.GetDecisionForEntityParams{
		EntityType: entity.EntityType,
		EntityID:   entity.EntityID,
	})
	if err != nil {
		return // No AI decision exists
	}

	// Only record override if not already overridden
	if decision.OverriddenAt.Valid {
		return
	}

	_ = h.queries.RecordDecisionOverride(ctx, store.RecordDecisionOverrideParams{
		ID:             decision.ID,
		OverriddenBy:   &userID,
		OverrideAction: &managerAction,
	})
}
