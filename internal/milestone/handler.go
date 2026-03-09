package milestone

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// ListMilestones returns paginated contract milestones filtered by status and type.
func (h *Handler) ListMilestones(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	milestoneType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	milestones, err := h.queries.ListContractMilestones(c.Request.Context(), store.ListContractMilestonesParams{
		CompanyID:     companyID,
		Status:        status,
		MilestoneType: milestoneType,
		Lim:           int32(limit),
		Off:           int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list milestones")
		return
	}
	count, err := h.queries.CountContractMilestones(c.Request.Context(), store.CountContractMilestonesParams{
		CompanyID:     companyID,
		Status:        status,
		MilestoneType: milestoneType,
	})
	if err != nil {
		response.InternalError(c, "Failed to count milestones")
		return
	}
	response.OK(c, gin.H{"data": milestones, "total": count, "page": page, "limit": limit})
}

// ListPending returns pending milestones for the company.
func (h *Handler) ListPending(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	milestones, err := h.queries.ListPendingMilestonesByCompany(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list pending milestones")
		return
	}
	response.OK(c, milestones)
}

// Acknowledge marks a milestone as acknowledged.
func (h *Handler) Acknowledge(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)
	milestone, err := h.queries.AcknowledgeMilestone(c.Request.Context(), store.AcknowledgeMilestoneParams{
		ID:             id,
		CompanyID:      companyID,
		AcknowledgedBy: &userID,
		Notes:          &req.Notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to acknowledge milestone")
		return
	}
	response.OK(c, milestone)
}

// Action records an action taken on a milestone.
func (h *Handler) Action(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)
	milestone, err := h.queries.ActionMilestone(c.Request.Context(), store.ActionMilestoneParams{
		ID:        id,
		CompanyID: companyID,
		Notes:     &req.Notes,
	})
	if err != nil {
		response.InternalError(c, "Failed to action milestone")
		return
	}
	response.OK(c, milestone)
}
