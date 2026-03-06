package policy

import (
	"log/slog"
	"strconv"
	"time"

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

func (h *Handler) ListPolicies(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	policies, err := h.queries.ListPolicies(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list policies")
		return
	}
	response.OK(c, policies)
}

func (h *Handler) GetPolicy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	policy, err := h.queries.GetPolicy(c.Request.Context(), store.GetPolicyParams{
		ID: id, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Policy not found")
		return
	}
	response.OK(c, policy)
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	var req struct {
		Title                  string `json:"title" binding:"required"`
		Content                string `json:"content" binding:"required"`
		Category               string `json:"category"`
		EffectiveDate          string `json:"effective_date"`
		RequiresAcknowledgment bool   `json:"requires_acknowledgment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	category := req.Category
	if category == "" {
		category = "general"
	}
	effDate := time.Now()
	if req.EffectiveDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.EffectiveDate); err == nil {
			effDate = parsed
		}
	}
	policy, err := h.queries.CreatePolicy(c.Request.Context(), store.CreatePolicyParams{
		CompanyID:              companyID,
		Title:                  req.Title,
		Content:                req.Content,
		Category:               category,
		Version:                1,
		EffectiveDate:          effDate,
		RequiresAcknowledgment: req.RequiresAcknowledgment,
		CreatedBy:              &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to create policy")
		return
	}
	response.Created(c, policy)
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Title                  string `json:"title" binding:"required"`
		Content                string `json:"content" binding:"required"`
		Category               string `json:"category"`
		Version                int32  `json:"version"`
		EffectiveDate          string `json:"effective_date"`
		RequiresAcknowledgment bool   `json:"requires_acknowledgment"`
		IsActive               bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	effDate := time.Now()
	if req.EffectiveDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.EffectiveDate); err == nil {
			effDate = parsed
		}
	}
	policy, err := h.queries.UpdatePolicy(c.Request.Context(), store.UpdatePolicyParams{
		ID:                     id,
		CompanyID:              companyID,
		Title:                  req.Title,
		Content:                req.Content,
		Category:               req.Category,
		Version:                req.Version,
		EffectiveDate:          effDate,
		RequiresAcknowledgment: req.RequiresAcknowledgment,
		IsActive:               req.IsActive,
	})
	if err != nil {
		response.InternalError(c, "Failed to update policy")
		return
	}
	response.OK(c, policy)
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	_, err := h.queries.DeactivatePolicy(c.Request.Context(), store.DeactivatePolicyParams{
		ID: id, CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to deactivate policy")
		return
	}
	response.OK(c, gin.H{"message": "Policy deactivated"})
}

func (h *Handler) AcknowledgePolicy(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.BadRequest(c, "Employee profile not found")
		return
	}
	ipAddr := c.ClientIP()
	ack, err := h.queries.AcknowledgePolicy(c.Request.Context(), store.AcknowledgePolicyParams{
		CompanyID:  companyID,
		PolicyID:   id,
		EmployeeID: emp.ID,
		IpAddress:  &ipAddr,
	})
	if err != nil {
		response.InternalError(c, "Failed to acknowledge policy")
		return
	}
	response.OK(c, ack)
}

func (h *Handler) ListAcknowledgments(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	acks, err := h.queries.ListPolicyAcknowledgments(c.Request.Context(), store.ListPolicyAcknowledgmentsParams{
		PolicyID: id, CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list acknowledgments")
		return
	}
	response.OK(c, acks)
}

func (h *Handler) GetAckStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	stats, err := h.queries.GetPolicyAckStats(c.Request.Context(), store.GetPolicyAckStatsParams{
		CompanyID: companyID, PolicyID: id,
	})
	if err != nil {
		response.InternalError(c, "Failed to get stats")
		return
	}
	response.OK(c, stats)
}

func (h *Handler) ListPending(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	emp, err := h.queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
		CompanyID: companyID, UserID: &userID,
	})
	if err != nil {
		response.OK(c, []any{})
		return
	}
	policies, err := h.queries.ListUnacknowledgedPolicies(c.Request.Context(), store.ListUnacknowledgedPoliciesParams{
		CompanyID:  companyID,
		EmployeeID: emp.ID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list pending policies")
		return
	}
	response.OK(c, policies)
}
