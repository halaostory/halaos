package employee

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
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

type createEmployeeRequest struct {
	EmployeeNo     string  `json:"employee_no" binding:"required"`
	FirstName      string  `json:"first_name" binding:"required"`
	LastName       string  `json:"last_name" binding:"required"`
	MiddleName     *string `json:"middle_name"`
	Suffix         *string `json:"suffix"`
	DisplayName    *string `json:"display_name"`
	Email          *string `json:"email"`
	Phone          *string `json:"phone"`
	BirthDate      *string `json:"birth_date"`
	Gender         *string `json:"gender"`
	CivilStatus    *string `json:"civil_status"`
	DepartmentID   *int64  `json:"department_id"`
	PositionID     *int64  `json:"position_id"`
	CostCenterID   *int64  `json:"cost_center_id"`
	ManagerID      *int64  `json:"manager_id"`
	HireDate       string  `json:"hire_date" binding:"required"`
	EmploymentType string  `json:"employment_type"`
}

func (h *Handler) ListEmployees(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	pg := pagination.Parse(c)

	status := c.Query("status")
	deptID := c.Query("department_id")

	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	var deptIDPtr *int64
	if deptID != "" {
		if id, err := strconv.ParseInt(deptID, 10, 64); err == nil {
			deptIDPtr = &id
		}
	}

	employees, err := h.queries.ListEmployees(c.Request.Context(), store.ListEmployeesParams{
		CompanyID:    companyID,
		Status:       statusPtr,
		DepartmentID: deptIDPtr,
		Limit:        int32(pg.Limit),
		Offset:       int32(pg.Offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list employees")
		return
	}

	count, _ := h.queries.CountEmployees(c.Request.Context(), store.CountEmployeesParams{
		CompanyID:    companyID,
		Status:       statusPtr,
		DepartmentID: deptIDPtr,
	})

	response.Paginated(c, employees, count, pg.Page, pg.Limit)
}

func (h *Handler) CreateEmployee(c *gin.Context) {
	var req createEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	hireDate, err := time.Parse("2006-01-02", req.HireDate)
	if err != nil {
		response.BadRequest(c, "Invalid hire_date format, use YYYY-MM-DD")
		return
	}

	var birthDate *time.Time
	if req.BirthDate != nil {
		if bd, err := time.Parse("2006-01-02", *req.BirthDate); err == nil {
			birthDate = &bd
		}
	}

	empType := req.EmploymentType
	if empType == "" {
		empType = "regular"
	}

	emp, err := h.queries.CreateEmployee(c.Request.Context(), store.CreateEmployeeParams{
		CompanyID:      companyID,
		EmployeeNo:     req.EmployeeNo,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		MiddleName:     req.MiddleName,
		Suffix:         req.Suffix,
		DisplayName:    req.DisplayName,
		Email:          req.Email,
		Phone:          req.Phone,
		BirthDate:      birthDate,
		Gender:         req.Gender,
		CivilStatus:    req.CivilStatus,
		DepartmentID:   req.DepartmentID,
		PositionID:     req.PositionID,
		CostCenterID:   req.CostCenterID,
		ManagerID:      req.ManagerID,
		HireDate:       hireDate,
		EmploymentType: empType,
	})
	if err != nil {
		h.logger.Error("failed to create employee", "error", err)
		response.Conflict(c, "Employee number already exists")
		return
	}
	response.Created(c, emp)
}

func (h *Handler) GetEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	emp, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	response.OK(c, emp)
}

func (h *Handler) UpdateEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	var req struct {
		FirstName      *string `json:"first_name"`
		LastName       *string `json:"last_name"`
		MiddleName     *string `json:"middle_name"`
		DisplayName    *string `json:"display_name"`
		Email          *string `json:"email"`
		Phone          *string `json:"phone"`
		DepartmentID   *int64  `json:"department_id"`
		PositionID     *int64  `json:"position_id"`
		CostCenterID   *int64  `json:"cost_center_id"`
		ManagerID      *int64  `json:"manager_id"`
		EmploymentType *string `json:"employment_type"`
		Status         *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	emp, err := h.queries.UpdateEmployee(c.Request.Context(), store.UpdateEmployeeParams{
		ID:             id,
		CompanyID:      companyID,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		MiddleName:     req.MiddleName,
		DisplayName:    req.DisplayName,
		Email:          req.Email,
		Phone:          req.Phone,
		DepartmentID:   req.DepartmentID,
		PositionID:     req.PositionID,
		CostCenterID:   req.CostCenterID,
		ManagerID:      req.ManagerID,
		EmploymentType: req.EmploymentType,
		Status:         req.Status,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	response.OK(c, emp)
}

func (h *Handler) GetProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	profile, err := h.queries.GetEmployeeProfile(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Profile not found")
		return
	}
	response.OK(c, profile)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	var req store.UpsertEmployeeProfileParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.EmployeeID = id

	profile, err := h.queries.UpsertEmployeeProfile(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("failed to update profile", "error", err)
		response.InternalError(c, "Failed to update profile")
		return
	}
	response.OK(c, profile)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}

	docs, err := h.queries.ListEmployeeDocuments(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "Failed to list documents")
		return
	}
	response.OK(c, docs)
}

func (h *Handler) UploadDocument(c *gin.Context) {
	// TODO: implement file upload with multipart form
	response.OK(c, gin.H{"message": "document upload placeholder"})
}
