package employee

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/pdf"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

func (h *Handler) GenerateCOE(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeForCOE(c.Request.Context(), store.GetEmployeeForCOEParams{
		ID:        empID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	comp, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	pdfBytes, err := pdf.GenerateCOE(comp, emp)
	if err != nil {
		h.logger.Error("failed to generate COE PDF", "error", err)
		response.InternalError(c, "Failed to generate PDF")
		return
	}

	fileName := fmt.Sprintf("COE_%s_%s.pdf", emp.EmployeeNo, time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Data(200, "application/pdf", pdfBytes)
}

func (h *Handler) GenerateLetter(c *gin.Context) {
	empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	companyID := auth.GetCompanyID(c)

	var req struct {
		LetterType string `json:"letter_type" binding:"required"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
		Violations string `json:"violations"`
		Deadline   string `json:"deadline"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	emp, err := h.queries.GetEmployeeForCOE(c.Request.Context(), store.GetEmployeeForCOEParams{
		ID:        empID,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	comp, err := h.queries.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get company")
		return
	}

	var salaryAmount float64
	if req.LetterType == "coec" {
		sal, err := h.queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
			CompanyID:     companyID,
			EmployeeID:    empID,
			EffectiveFrom: time.Now(),
		})
		if err == nil {
			var n pgtype.Numeric
			n = sal.BasicSalary
			if n.Valid {
				f, _ := n.Float64Value()
				if f.Valid {
					salaryAmount = f.Float64
				}
			}
		}
	}

	pdfBytes, err := pdf.GenerateLetter(comp, emp, req.LetterType, req.Subject, req.Body, req.Violations, req.Deadline, salaryAmount)
	if err != nil {
		h.logger.Error("failed to generate letter PDF", "error", err)
		response.InternalError(c, "Failed to generate PDF")
		return
	}

	fileName := fmt.Sprintf("%s_%s_%s.pdf", strings.ToUpper(req.LetterType), emp.EmployeeNo, time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Data(200, "application/pdf", pdfBytes)
}
