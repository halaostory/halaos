package breaks

import (
	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// ListPolicies returns all active break policies for the company.
func (h *Handler) ListPolicies(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	policies, err := h.queries.ListBreakPolicies(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to list break policies", "error", err)
		response.InternalError(c, "Failed to list break policies")
		return
	}

	response.OK(c, policies)
}

type policyItem struct {
	BreakType  string `json:"break_type" binding:"required"`
	MaxMinutes int32  `json:"max_minutes" binding:"required,min=1"`
}

type upsertPoliciesRequest struct {
	Policies []policyItem `json:"policies" binding:"required,dive"`
}

// UpsertPolicies creates or updates break policies for the company.
func (h *Handler) UpsertPolicies(c *gin.Context) {
	var req upsertPoliciesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if len(req.Policies) == 0 {
		response.BadRequest(c, "At least one policy is required")
		return
	}

	companyID := auth.GetCompanyID(c)

	// Validate all break types before any writes
	for _, p := range req.Policies {
		if !validBreakTypes[p.BreakType] {
			response.BadRequest(c, "Invalid break_type: "+p.BreakType+". Must be one of: meal, bathroom, rest, leave_post")
			return
		}
	}

	results := make([]store.BreakPolicy, 0, len(req.Policies))
	for _, p := range req.Policies {
		policy, err := h.queries.UpsertBreakPolicy(c.Request.Context(), store.UpsertBreakPolicyParams{
			CompanyID:  companyID,
			BreakType:  p.BreakType,
			MaxMinutes: p.MaxMinutes,
		})
		if err != nil {
			h.logger.Error("failed to upsert break policy", "error", err, "break_type", p.BreakType)
			response.InternalError(c, "Failed to save break policy")
			return
		}
		results = append(results, policy)
	}

	response.OK(c, results)
}
