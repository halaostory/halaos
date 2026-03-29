package clearance

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

func (h *Handler) Create(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	var req struct {
		EmployeeID      int64  `json:"employee_id" binding:"required"`
		ResignationDate string `json:"resignation_date" binding:"required"`
		LastWorkingDay  string `json:"last_working_day" binding:"required"`
		Reason          string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	resignDate, err := time.Parse("2006-01-02", req.ResignationDate)
	if err != nil {
		response.BadRequest(c, "Invalid resignation date")
		return
	}
	lastDay, err := time.Parse("2006-01-02", req.LastWorkingDay)
	if err != nil {
		response.BadRequest(c, "Invalid last working day")
		return
	}
	cr, err := h.queries.CreateClearanceRequest(c.Request.Context(), store.CreateClearanceRequestParams{
		CompanyID:       companyID,
		EmployeeID:      req.EmployeeID,
		ResignationDate: resignDate,
		LastWorkingDay:  lastDay,
		Reason:          &req.Reason,
		SubmittedBy:     userID,
	})
	if err != nil {
		h.logger.Error("failed to create clearance request", "error", err)
		response.InternalError(c, "Failed to create clearance request")
		return
	}

	// Auto-create clearance items from template
	templates, _ := h.queries.ListClearanceTemplates(c.Request.Context(), companyID)
	if len(templates) == 0 {
		// Default items if no template configured
		defaults := []struct{ dept, item string }{
			{"IT", "Return laptop/equipment"},
			{"IT", "Revoke system access"},
			{"IT", "Email account deactivation"},
			{"HR", "Exit interview completed"},
			{"HR", "ID card returned"},
			{"HR", "Final pay computation"},
			{"HR", "Certificate of Employment"},
			{"Finance", "Cash advance settlement"},
			{"Finance", "Company credit card returned"},
			{"Admin", "Office keys returned"},
			{"Admin", "Parking card returned"},
			{"Direct Manager", "Knowledge transfer completed"},
			{"Direct Manager", "Pending tasks handover"},
		}
		for _, d := range defaults {
			if _, err := h.queries.CreateClearanceItem(c.Request.Context(), store.CreateClearanceItemParams{
				ClearanceID: cr.ID,
				Department:  d.dept,
				ItemName:    d.item,
			}); err != nil {
				h.logger.Error("failed to create default clearance item", "department", d.dept, "error", err)
			}
		}
	} else {
		for _, t := range templates {
			if _, err := h.queries.CreateClearanceItem(c.Request.Context(), store.CreateClearanceItemParams{
				ClearanceID: cr.ID,
				Department:  t.Department,
				ItemName:    t.ItemName,
			}); err != nil {
				h.logger.Error("failed to create clearance item from template", "department", t.Department, "error", err)
			}
		}
	}

	response.Created(c, cr)
}

func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	items, err := h.queries.ListClearanceRequests(c.Request.Context(), store.ListClearanceRequestsParams{
		CompanyID: companyID,
		Column2:   status,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list clearance requests")
		return
	}
	count, _ := h.queries.CountClearanceRequests(c.Request.Context(), store.CountClearanceRequestsParams{
		CompanyID: companyID,
		Column2:   status,
	})
	response.Paginated(c, items, count, offset/limit+1, limit)
}

func (h *Handler) Get(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	cr, err := h.queries.GetClearanceRequest(c.Request.Context(), store.GetClearanceRequestParams{
		ID:        id,
		CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Clearance request not found")
		return
	}
	items, err := h.queries.ListClearanceItems(c.Request.Context(), store.ListClearanceItemsParams{
		ClearanceID: id,
		CompanyID:   companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to list clearance items")
		return
	}
	response.OK(c, map[string]any{
		"request": cr,
		"items":   items,
	})
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	cr, err := h.queries.UpdateClearanceStatus(c.Request.Context(), store.UpdateClearanceStatusParams{
		ID:        id,
		CompanyID: companyID,
		Status:    req.Status,
	})
	if err != nil {
		response.InternalError(c, "Failed to update clearance status")
		return
	}
	response.OK(c, cr)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	userID := auth.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	var req struct {
		Status  string  `json:"status" binding:"required"`
		Remarks *string `json:"remarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	companyID := auth.GetCompanyID(c)
	item, err := h.queries.UpdateClearanceItem(c.Request.Context(), store.UpdateClearanceItemParams{
		ID:        id,
		Status:    req.Status,
		ClearedBy: &userID,
		Remarks:   req.Remarks,
		CompanyID: companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to update clearance item")
		return
	}
	response.OK(c, item)
}

func (h *Handler) ListTemplates(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	templates, err := h.queries.ListClearanceTemplates(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list templates")
		return
	}
	response.OK(c, templates)
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Department string `json:"department" binding:"required"`
		ItemName   string `json:"item_name" binding:"required"`
		SortOrder  int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	tmpl, err := h.queries.CreateClearanceTemplate(c.Request.Context(), store.CreateClearanceTemplateParams{
		CompanyID:  companyID,
		Department: req.Department,
		ItemName:   req.ItemName,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		response.InternalError(c, "Failed to create template")
		return
	}
	response.Created(c, tmpl)
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	if err := h.queries.DeleteClearanceTemplate(c.Request.Context(), store.DeleteClearanceTemplateParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete template")
		return
	}
	response.OK(c, nil)
}
