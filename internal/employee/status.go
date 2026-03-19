package employee

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) ChangeStatus(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	var req struct {
		Status  string  `json:"status" binding:"required"`
		Remarks *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	oldEmp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{
		ID: empID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	updated, err := h.queries.UpdateEmployee(c.Request.Context(), store.UpdateEmployeeParams{
		ID:             empID,
		CompanyID:      companyID,
		FirstName:      oldEmp.FirstName,
		LastName:       oldEmp.LastName,
		MiddleName:     oldEmp.MiddleName,
		DisplayName:    oldEmp.DisplayName,
		Email:          oldEmp.Email,
		Phone:          oldEmp.Phone,
		DepartmentID:   oldEmp.DepartmentID,
		PositionID:     oldEmp.PositionID,
		CostCenterID:   oldEmp.CostCenterID,
		ManagerID:      oldEmp.ManagerID,
		EmploymentType: oldEmp.EmploymentType,
		Status:         req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update status")
		return
	}

	actionType := req.Status
	if req.Status == "active" && oldEmp.Status == "probationary" {
		actionType = "regularized"
	} else if req.Status == "separated" {
		actionType = "separated"
	} else if req.Status == "active" && oldEmp.Status == "separated" {
		actionType = "reinstated"
	} else if req.Status == "suspended" {
		actionType = "suspended"
	}

	if _, err := h.queries.CreateEmploymentHistory(c.Request.Context(), store.CreateEmploymentHistoryParams{
		CompanyID:     companyID,
		EmployeeID:    empID,
		ActionType:    actionType,
		EffectiveDate: time.Now(),
		Remarks:       req.Remarks,
		CreatedBy:     &userID,
	}); err != nil {
		h.logger.Error("failed to create employment history", "employee_id", empID, "action", actionType, "error", err)
	}

	// Emit accounting event (async, non-blocking)
	if h.accounting != nil {
		go func() {
			if req.Status == "separated" || req.Status == "terminated" {
				reason := req.Status
				if req.Remarks != nil {
					reason = *req.Remarks
				}
				if err := h.accounting.EmitEmployeeTerminated(context.Background(), companyID, empID, reason); err != nil {
					h.logger.Error("failed to emit employee.terminated accounting event", "employee_id", empID, "error", err)
				}
			} else {
				if err := h.accounting.EmitEmployeeUpserted(context.Background(), companyID, empID); err != nil {
					h.logger.Error("failed to emit employee.upserted accounting event", "employee_id", empID, "error", err)
				}
			}
		}()
	}

	response.OK(c, updated)
}

func (h *Handler) GetTimeline(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	items, err := h.queries.ListEmployeeTimeline(c.Request.Context(), store.ListEmployeeTimelineParams{
		EmployeeID: empID,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to load timeline")
		return
	}
	response.OK(c, items)
}
