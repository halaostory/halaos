package employee

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/numericutil"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) GetSalary(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	salary, err := h.queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    id,
		EffectiveFrom: time.Now(),
	})
	if err != nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, salary)
}

func (h *Handler) AssignSalary(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	var req struct {
		BasicSalary   float64 `json:"basic_salary" binding:"required"`
		StructureID   *int64  `json:"structure_id"`
		EffectiveFrom string  `json:"effective_from" binding:"required"`
		EffectiveTo   *string `json:"effective_to"`
		Remarks       *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)

	var basicSalary pgtype.Numeric
	_ = basicSalary.Scan(fmt.Sprintf("%.2f", req.BasicSalary))

	var effTo pgtype.Date
	if req.EffectiveTo != nil {
		parsed, _ := time.Parse("2006-01-02", *req.EffectiveTo)
		effTo = pgtype.Date{Time: parsed, Valid: true}
	}

	salary, err := h.queries.CreateEmployeeSalary(c.Request.Context(), store.CreateEmployeeSalaryParams{
		CompanyID:     companyID,
		EmployeeID:    id,
		StructureID:   req.StructureID,
		BasicSalary:   basicSalary,
		EffectiveFrom: effFrom,
		EffectiveTo:   effTo,
		Remarks:       req.Remarks,
		CreatedBy:     &userID,
	})
	if err != nil {
		response.InternalError(c, "Failed to assign salary")
		return
	}
	response.Created(c, salary)
}

func (h *Handler) BulkUpdateSalary(c *gin.Context) {
	var req struct {
		EmployeeIDs   []int64 `json:"employee_ids" binding:"required"`
		UpdateType    string  `json:"update_type" binding:"required"`
		Value         float64 `json:"value" binding:"required"`
		EffectiveFrom string  `json:"effective_from" binding:"required"`
		Remarks       *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.UpdateType != "percentage" && req.UpdateType != "fixed" {
		response.BadRequest(c, "update_type must be 'percentage' or 'fixed'")
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)

	var updated, failed int
	type Result struct {
		EmployeeID int64   `json:"employee_id"`
		OldSalary  float64 `json:"old_salary"`
		NewSalary  float64 `json:"new_salary"`
	}
	var results []Result

	for _, empID := range req.EmployeeIDs {
		currentSalary, err := h.queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
			CompanyID:     companyID,
			EmployeeID:    empID,
			EffectiveFrom: time.Now(),
		})
		if err != nil {
			failed++
			continue
		}

		oldSalary := numericutil.ToFloat(currentSalary.BasicSalary)
		var newSalary float64
		if req.UpdateType == "percentage" {
			newSalary = oldSalary * (1 + req.Value/100)
		} else {
			newSalary = oldSalary + req.Value
		}

		var basicNum pgtype.Numeric
		_ = basicNum.Scan(fmt.Sprintf("%.2f", newSalary))

		_, err = h.queries.CreateEmployeeSalary(c.Request.Context(), store.CreateEmployeeSalaryParams{
			CompanyID:     companyID,
			EmployeeID:    empID,
			StructureID:   currentSalary.StructureID,
			BasicSalary:   basicNum,
			EffectiveFrom: effFrom,
			Remarks:       req.Remarks,
			CreatedBy:     &userID,
		})
		if err != nil {
			failed++
			continue
		}
		updated++
		results = append(results, Result{EmployeeID: empID, OldSalary: oldSalary, NewSalary: newSalary})
	}

	response.OK(c, gin.H{
		"updated": updated,
		"failed":  failed,
		"results": results,
	})
}
