// Package workplace manages a company's work locations (Warp:
// "workplaces"). Employees are assigned to one; time-off policy rules can
// target workplaces.
package workplace

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/access"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler serves /workplaces.
type Handler struct{ q *store.Queries }

// NewHandler builds the handler.
func NewHandler(q *store.Queries) *Handler { return &Handler{q: q} }

// RegisterRoutes registers workplace routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/workplaces", auth.ManagerOrAbove(), h.list)
	protected.POST("/workplaces", auth.AdminOnly(), h.create)
	protected.PUT("/workplaces/:id", auth.AdminOnly(), h.update)
	protected.GET("/employees/:id/workplace", auth.ManagerOrAbove(), access.EmployeeInCompany(h.q, "id"), h.getEmployee)
	protected.PUT("/employees/:id/workplace", auth.AdminOnly(), access.EmployeeInCompany(h.q, "id"), h.setEmployee)
}

type request struct {
	Name     string `json:"name" binding:"required"`
	Address  string `json:"address"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	IsRemote bool   `json:"is_remote"`
	IsActive *bool  `json:"is_active"`
}

func (r *request) clean() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len(r.Name) > 120 {
		return errors.New("name is required (max 120 characters)")
	}
	r.Country = strings.ToUpper(strings.TrimSpace(r.Country))
	if r.Country == "" {
		r.Country = "PHL"
	}
	if len(r.Country) != 3 {
		return errors.New("country must be a 3-letter code such as PHL")
	}
	return nil
}

func (h *Handler) list(c *gin.Context) {
	rows, err := h.q.ListWorkplaces(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to load workplaces")
		return
	}
	if rows == nil {
		rows = []store.ListWorkplacesRow{}
	}
	response.OK(c, rows)
}

func (h *Handler) create(c *gin.Context) {
	var req request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}
	if err := req.clean(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	w, err := h.q.CreateWorkplace(c.Request.Context(), store.CreateWorkplaceParams{
		CompanyID: auth.GetCompanyID(c), Name: req.Name, Address: req.Address, City: req.City, Region: req.Region,
		Country: req.Country, IsRemote: req.IsRemote,
	})
	if err != nil {
		response.BadRequest(c, "A workplace with this name already exists")
		return
	}
	response.Created(c, w)
}

func (h *Handler) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	var req request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}
	if err := req.clean(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	active := req.IsActive == nil || *req.IsActive
	w, err := h.q.UpdateWorkplace(c.Request.Context(), store.UpdateWorkplaceParams{
		ID: id, CompanyID: auth.GetCompanyID(c), Name: req.Name, Address: req.Address, City: req.City, Region: req.Region,
		Country: req.Country, IsRemote: req.IsRemote, IsActive: active,
	})
	if err != nil {
		response.NotFound(c, "Workplace not found")
		return
	}
	response.OK(c, w)
}

func (h *Handler) getEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	w, err := h.q.GetEmployeeWorkplace(c.Request.Context(), store.GetEmployeeWorkplaceParams{EmployeeID: id, CompanyID: auth.GetCompanyID(c)})
	if errors.Is(err, pgx.ErrNoRows) {
		response.OK(c, nil)
		return
	}
	if err != nil {
		response.InternalError(c, "Failed to load workplace")
		return
	}
	response.OK(c, w)
}

func (h *Handler) setEmployee(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	var req struct {
		WorkplaceID *int64 `json:"workplace_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	if _, err := h.q.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	if req.WorkplaceID == nil {
		_ = h.q.ClearEmployeeWorkplace(ctx, store.ClearEmployeeWorkplaceParams{EmployeeID: id, CompanyID: companyID})
		response.OK(c, nil)
		return
	}
	w, err := h.q.GetWorkplace(ctx, store.GetWorkplaceParams{ID: *req.WorkplaceID, CompanyID: companyID})
	if err != nil {
		response.BadRequest(c, "Unknown workplace")
		return
	}
	if err := h.q.SetEmployeeWorkplace(ctx, store.SetEmployeeWorkplaceParams{EmployeeID: id, CompanyID: companyID, WorkplaceID: w.ID}); err != nil {
		response.InternalError(c, "Failed to save")
		return
	}
	response.OK(c, w)
}
