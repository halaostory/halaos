package payroll

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ListBonusStructures(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.DefaultQuery("status", "")

	structures, err := h.queries.ListBonusStructures(c.Request.Context(), store.ListBonusStructuresParams{
		CompanyID: companyID,
		Column2:   status,
	})
	if err != nil {
		response.InternalError(c, "Failed to list bonus structures")
		return
	}
	response.OK(c, structures)
}

func (h *Handler) CreateBonusStructure(c *gin.Context) {
	var req struct {
		Name          string                 `json:"name" binding:"required"`
		Description   *string                `json:"description"`
		BonusType     string                 `json:"bonus_type"`
		BaseAmount    float64                `json:"base_amount"`
		BaseType      string                 `json:"base_type"`
		RatingMap     map[string]interface{} `json:"rating_map"`
		ReviewCycleID *int64                 `json:"review_cycle_id"`
		IsTaxable     *bool                  `json:"is_taxable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	bonusType := req.BonusType
	if bonusType == "" {
		bonusType = "kpi"
	}
	baseType := req.BaseType
	if baseType == "" {
		baseType = "fixed"
	}
	isTaxable := true
	if req.IsTaxable != nil {
		isTaxable = *req.IsTaxable
	}

	ratingMapJSON, _ := json.Marshal(req.RatingMap)
	if req.RatingMap == nil {
		ratingMapJSON = []byte("{}")
	}

	bs, err := h.queries.CreateBonusStructure(c.Request.Context(), store.CreateBonusStructureParams{
		CompanyID:     companyID,
		Name:          req.Name,
		Description:   req.Description,
		BonusType:     bonusType,
		BaseAmount:    numericFromFloat(req.BaseAmount),
		BaseType:      baseType,
		RatingMap:     json.RawMessage(ratingMapJSON),
		ReviewCycleID: req.ReviewCycleID,
		IsTaxable:     isTaxable,
		Status:        "draft",
		CreatedBy:     &userID,
	})
	if err != nil {
		h.logger.Error("create bonus structure failed", "error", err)
		response.InternalError(c, "Failed to create bonus structure")
		return
	}
	response.Created(c, bs)
}

func (h *Handler) GetBonusStructure(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	bs, err := h.queries.GetBonusStructure(c.Request.Context(), store.GetBonusStructureParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Bonus structure not found")
		return
	}
	response.OK(c, bs)
}

func (h *Handler) UpdateBonusStructureStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	bs, err := h.queries.UpdateBonusStructureStatus(c.Request.Context(), store.UpdateBonusStructureStatusParams{
		ID:        id,
		CompanyID: companyID,
		Status:    req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update bonus structure status")
		return
	}
	response.OK(c, bs)
}

func (h *Handler) ListBonusAllocations(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}

	allocations, err := h.queries.ListBonusAllocations(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "Failed to list bonus allocations")
		return
	}
	response.OK(c, allocations)
}

func (h *Handler) CalculateBonusAllocations(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	// Get the bonus structure
	bs, err := h.queries.GetBonusStructure(c.Request.Context(), store.GetBonusStructureParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Bonus structure not found")
		return
	}

	if bs.ReviewCycleID == nil {
		response.BadRequest(c, "No review cycle linked to this bonus structure")
		return
	}

	// Parse rating_map
	var ratingMap map[string]float64
	if err := json.Unmarshal(bs.RatingMap, &ratingMap); err != nil {
		response.BadRequest(c, "Invalid rating map format")
		return
	}

	// Get completed performance reviews for the linked cycle
	reviews, err := h.queries.GetCompletedReviewsForBonusCalc(c.Request.Context(), store.GetCompletedReviewsForBonusCalcParams{
		CompanyID:     companyID,
		ReviewCycleID: *bs.ReviewCycleID,
	})
	if err != nil {
		response.InternalError(c, "Failed to get performance reviews")
		return
	}

	if len(reviews) == 0 {
		response.OK(c, map[string]interface{}{
			"message":    "No completed reviews found for this cycle",
			"calculated": 0,
		})
		return
	}

	baseAmount := numericToFloat(bs.BaseAmount)
	calculated := 0

	for _, review := range reviews {
		if review.FinalRating == nil {
			continue
		}

		rating := int(*review.FinalRating)
		ratingKey := strconv.Itoa(rating)

		multiplier, ok := ratingMap[ratingKey]
		if !ok {
			multiplier = 0
		}

		// Determine base amount
		employeeBase := baseAmount
		if bs.BaseType == "basic_salary_pct" {
			salary, err := h.queries.GetEmployeeSalaryForBonus(c.Request.Context(), store.GetEmployeeSalaryForBonusParams{
				CompanyID:  companyID,
				EmployeeID: review.EmployeeID,
			})
			if err != nil {
				h.logger.Warn("no salary found for employee, using structure base", "employee_id", review.EmployeeID)
				employeeBase = baseAmount
			} else {
				salaryVal := numericToFloat(salary)
				employeeBase = salaryVal * baseAmount / 100
			}
		}

		finalAmount := round2(employeeBase * multiplier)
		reviewID := review.ReviewID
		ratingVal := *review.FinalRating

		_, err := h.queries.CreateBonusAllocation(c.Request.Context(), store.CreateBonusAllocationParams{
			CompanyID:           companyID,
			StructureID:         id,
			EmployeeID:          review.EmployeeID,
			PerformanceReviewID: &reviewID,
			Rating:              &ratingVal,
			Multiplier:          numericFromFloat(multiplier),
			BaseAmount:          numericFromFloat(employeeBase),
			FinalAmount:         numericFromFloat(finalAmount),
			ManualOverride:      pgtype.Numeric{Valid: false},
		})
		if err != nil {
			h.logger.Error("failed to create bonus allocation", "employee_id", review.EmployeeID, "error", err)
			continue
		}
		calculated++
	}

	response.OK(c, map[string]interface{}{
		"message":    fmt.Sprintf("Calculated bonus for %d employees", calculated),
		"calculated": calculated,
	})
}

func (h *Handler) CreateBonusAllocation(c *gin.Context) {
	structureID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid structure ID")
		return
	}

	var req struct {
		EmployeeID     int64    `json:"employee_id" binding:"required"`
		Rating         *int32   `json:"rating"`
		Multiplier     float64  `json:"multiplier"`
		BaseAmount     float64  `json:"base_amount"`
		FinalAmount    float64  `json:"final_amount"`
		ManualOverride *float64 `json:"manual_override"`
		Notes          *string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	manualOverride := pgtype.Numeric{Valid: false}
	if req.ManualOverride != nil {
		manualOverride = numericFromFloat(*req.ManualOverride)
	}

	alloc, err := h.queries.CreateBonusAllocation(c.Request.Context(), store.CreateBonusAllocationParams{
		CompanyID:      companyID,
		StructureID:    structureID,
		EmployeeID:     req.EmployeeID,
		Rating:         req.Rating,
		Multiplier:     numericFromFloat(req.Multiplier),
		BaseAmount:     numericFromFloat(req.BaseAmount),
		FinalAmount:    numericFromFloat(req.FinalAmount),
		ManualOverride: manualOverride,
		Notes:          req.Notes,
	})
	if err != nil {
		h.logger.Error("create bonus allocation failed", "error", err)
		response.InternalError(c, "Failed to create bonus allocation")
		return
	}
	response.Created(c, alloc)
}

func (h *Handler) ApproveBonusAllocations(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := auth.GetUserID(c)

	err := h.queries.BulkApproveBonusAllocations(c.Request.Context(), store.BulkApproveBonusAllocationsParams{
		Column1:    req.IDs,
		ApprovedBy: &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to approve bonus allocations")
		return
	}
	response.OK(c, map[string]interface{}{
		"message":  fmt.Sprintf("Approved %d allocations", len(req.IDs)),
		"approved": len(req.IDs),
	})
}
