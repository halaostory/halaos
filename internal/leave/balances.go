package leave

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/numericutil"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) ListAllBalances(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid year parameter")
		return
	}
	balances, err := h.queries.ListAllLeaveBalances(c.Request.Context(), store.ListAllLeaveBalancesParams{
		CompanyID: companyID,
		Year:      int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to list leave balances")
		return
	}
	response.OK(c, balances)
}

func (h *Handler) AdjustBalance(c *gin.Context) {
	var req struct {
		EmployeeID  int64   `json:"employee_id" binding:"required"`
		LeaveTypeID int64   `json:"leave_type_id" binding:"required"`
		Year        int32   `json:"year" binding:"required"`
		Adjusted    float64 `json:"adjusted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	var adjusted pgtype.Numeric
	_ = adjusted.Scan(fmt.Sprintf("%.1f", req.Adjusted))
	balance, err := h.queries.AdjustLeaveBalance(c.Request.Context(), store.AdjustLeaveBalanceParams{
		CompanyID:   companyID,
		EmployeeID:  req.EmployeeID,
		LeaveTypeID: req.LeaveTypeID,
		Year:        req.Year,
		Adjusted:    adjusted,
	})
	if err != nil {
		h.logger.Error("failed to adjust leave balance", "error", err)
		response.InternalError(c, "Failed to adjust leave balance")
		return
	}
	response.OK(c, balance)
}

func (h *Handler) Carryover(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		FromYear int32 `json:"from_year" binding:"required"`
		ToYear   int32 `json:"to_year" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "from_year and to_year are required")
		return
	}
	if req.ToYear != req.FromYear+1 {
		response.BadRequest(c, "to_year must be from_year + 1")
		return
	}

	prevBalances, err := h.queries.ListLeaveBalancesForCarryover(c.Request.Context(), store.ListLeaveBalancesForCarryoverParams{
		CompanyID: companyID,
		Year:      req.FromYear,
	})
	if err != nil {
		response.InternalError(c, "Failed to get previous year balances")
		return
	}

	type CarryoverResult struct {
		EmployeeNo   string  `json:"employee_no"`
		EmployeeName string  `json:"employee_name"`
		LeaveType    string  `json:"leave_type"`
		Remaining    float64 `json:"remaining"`
		CarriedOver  float64 `json:"carried_over"`
		Forfeited    float64 `json:"forfeited"`
	}

	var carried int
	var totalForfeited float64
	results := []CarryoverResult{}
	for _, lb := range prevBalances {
		earned := numericutil.ToFloat(lb.Earned)
		used := numericutil.ToFloat(lb.Used)
		prevCarried := numericutil.ToFloat(lb.Carried)
		adjusted := numericutil.ToFloat(lb.Adjusted)
		remaining := earned + prevCarried + adjusted - used

		if remaining <= 0 {
			continue
		}

		maxCarry := numericutil.ToFloat(lb.MaxCarryover)
		if maxCarry <= 0 {
			maxCarry = 5
		}
		carryAmount := remaining
		if carryAmount > maxCarry {
			carryAmount = maxCarry
		}
		carryAmount = math.Round(carryAmount*10) / 10
		forfeited := math.Round((remaining-carryAmount)*10) / 10

		var carriedNum, earnedZero pgtype.Numeric
		_ = carriedNum.Scan(fmt.Sprintf("%.1f", carryAmount))
		_ = earnedZero.Scan("0")

		_, err := h.queries.UpsertLeaveBalance(c.Request.Context(), store.UpsertLeaveBalanceParams{
			CompanyID:   companyID,
			EmployeeID:  lb.EmployeeID,
			LeaveTypeID: lb.LeaveTypeID,
			Year:        req.ToYear,
			Earned:      earnedZero,
			Carried:     carriedNum,
		})
		if err != nil {
			h.logger.Error("failed to carryover leave balance",
				"employee_id", lb.EmployeeID,
				"leave_type_id", lb.LeaveTypeID,
				"error", err)
			continue
		}
		carried++
		totalForfeited += forfeited
		results = append(results, CarryoverResult{
			EmployeeNo:   lb.EmployeeNo,
			EmployeeName: lb.LastName + ", " + lb.FirstName,
			LeaveType:    lb.LeaveTypeName,
			Remaining:    remaining,
			CarriedOver:  carryAmount,
			Forfeited:    forfeited,
		})
	}

	response.OK(c, gin.H{
		"processed":       len(prevBalances),
		"carried":         carried,
		"total_forfeited": totalForfeited,
		"from_year":       req.FromYear,
		"to_year":         req.ToYear,
		"details":         results,
	})
}
