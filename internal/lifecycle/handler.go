package lifecycle

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler exposes the lifecycle service over HTTP.
type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	svc     *Service
}

// NewHandler creates a lifecycle handler.
func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, svc: NewService(queries, pool, logger)}
}

// Service returns the underlying lifecycle service (shared with the worker).
func (h *Handler) Service() *Service { return h.svc }

func (h *Handler) writeErr(c *gin.Context, err error, fallback string) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		response.BadRequest(c, ve.Msg)
	case errors.Is(err, ErrNotFound):
		response.NotFound(c, "Employee not found")
	case errors.Is(err, ErrConflict):
		response.Conflict(c, strings.TrimPrefix(err.Error(), ErrConflict.Error()+": "))
	default:
		h.logger.Error("lifecycle: "+fallback, "error", err)
		response.InternalError(c, fallback)
	}
}

func parseID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid "+name)
		return 0, false
	}
	return id, true
}

// Hire handles POST /lifecycle/hire.
func (h *Handler) Hire(c *gin.Context) {
	var in HireInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	res, err := h.svc.Hire(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), in)
	if err != nil {
		h.writeErr(c, err, "Failed to hire employee")
		return
	}
	response.Created(c, res)
}

// Terminate handles POST /lifecycle/employees/:id/terminate.
func (h *Handler) Terminate(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in TerminateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	res, err := h.svc.Terminate(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), id, in)
	if err != nil {
		h.writeErr(c, err, "Failed to terminate employee")
		return
	}
	response.OK(c, res)
}

// CancelTermination handles DELETE /lifecycle/employees/:id/terminate.
func (h *Handler) CancelTermination(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	sep, err := h.svc.CancelSeparation(c.Request.Context(), auth.GetCompanyID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "No scheduled separation for this employee")
			return
		}
		h.writeErr(c, err, "Failed to cancel separation")
		return
	}
	response.OK(c, sep)
}

// Overview is the per-employee lifecycle summary.
type Overview struct {
	EmployeeID   int64                                   `json:"employee_id"`
	Status       string                                  `json:"status"`
	Stage        string                                  `json:"stage"`
	Onboarding   *store.GetOnboardingProgressRow         `json:"onboarding"`
	Offboarding  *store.GetOnboardingProgressRow         `json:"offboarding"`
	Separation   *store.EmployeeSeparation               `json:"separation"`
	Clearance    *store.ClearanceRequest                 `json:"clearance"`
	FinalPay     *store.GetFinalPayStatusForEmployeeRow  `json:"final_pay"`
	Provisioning []store.ListEmployeeProvisioningJobsRow `json:"provisioning"`
}

// GetOverview handles GET /lifecycle/employees/:id.
func (h *Handler) GetOverview(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)

	emp, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: id, CompanyID: companyID})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	ov := Overview{EmployeeID: id, Status: emp.Status, Provisioning: []store.ListEmployeeProvisioningJobsRow{}}

	if rows, err := h.queries.GetOnboardingProgress(ctx, store.GetOnboardingProgressParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		for i := range rows {
			r := rows[i]
			switch r.WorkflowType {
			case "onboarding":
				ov.Onboarding = &r
			case "offboarding":
				ov.Offboarding = &r
			}
		}
	}
	if sep, err := h.queries.GetLatestEmployeeSeparation(ctx, store.GetLatestEmployeeSeparationParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		ov.Separation = &sep
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.logger.Warn("lifecycle: separation lookup", "error", err)
	}
	if cr, err := h.queries.GetLatestClearanceForEmployee(ctx, store.GetLatestClearanceForEmployeeParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		ov.Clearance = &cr
	}
	if fp, err := h.queries.GetFinalPayStatusForEmployee(ctx, store.GetFinalPayStatusForEmployeeParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		ov.FinalPay = &fp
	}
	if jobs, err := h.queries.ListEmployeeProvisioningJobs(ctx, store.ListEmployeeProvisioningJobsParams{EmployeeID: id, CompanyID: companyID}); err == nil {
		ov.Provisioning = jobs
	}

	var remaining int64
	if ov.Onboarding != nil {
		remaining = ov.Onboarding.Remaining
	}
	ov.Stage = ComputeStage(StageInput{
		Status:              emp.Status,
		HireDate:            emp.HireDate,
		HasActiveSeparation: ov.Separation != nil && ov.Separation.Status == "scheduled",
		OnboardingRemaining: remaining,
		Today:               time.Now(),
	})
	response.OK(c, ov)
}

// InboxResponse is the unified HR to-do list.
type InboxResponse struct {
	Items  []store.ListHRInboxItemsRow `json:"items"`
	Counts map[string]int              `json:"counts"`
	Total  int                         `json:"total"`
	High   int                         `json:"high"`
}

// GetInbox handles GET /inbox: everything across modules waiting on HR.
func (h *Handler) GetInbox(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	items, err := h.queries.ListHRInboxItems(c.Request.Context(), store.ListHRInboxItemsParams{
		CompanyID: auth.GetCompanyID(c),
		Lim:       int32(limit),
	})
	if err != nil {
		h.logger.Error("lifecycle: inbox query failed", "error", err)
		response.InternalError(c, "Failed to load inbox")
		return
	}
	response.OK(c, summarizeInbox(items, c.Query("kind")))
}

func summarizeInbox(items []store.ListHRInboxItemsRow, kind string) InboxResponse {
	out := InboxResponse{Items: []store.ListHRInboxItemsRow{}, Counts: map[string]int{}}
	for _, it := range items {
		out.Counts[it.Kind]++
		out.Total++
		if it.Priority == "high" {
			out.High++
		}
		if kind == "" || kind == it.Kind {
			out.Items = append(out.Items, it)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Custom fields
// ---------------------------------------------------------------------------

type customFieldRequest struct {
	Key         string   `json:"key"`
	Label       string   `json:"label" binding:"required"`
	FieldType   string   `json:"field_type"`
	Options     []string `json:"options"`
	IsRequired  bool     `json:"is_required"`
	IsSensitive bool     `json:"is_sensitive"`
	SortOrder   int32    `json:"sort_order"`
	IsActive    *bool    `json:"is_active"`
}

func (r *customFieldRequest) normalize() error {
	r.Label = strings.TrimSpace(r.Label)
	if r.Label == "" {
		return errors.New("label is required")
	}
	if r.FieldType == "" {
		r.FieldType = "text"
	}
	if !ValidFieldType(r.FieldType) {
		return errors.New("invalid field_type")
	}
	if r.Options == nil {
		r.Options = []string{}
	}
	if r.FieldType == "select" && len(r.Options) == 0 {
		return errors.New("select fields need at least one option")
	}
	return nil
}

// ListCustomFields handles GET /custom-fields.
func (h *Handler) ListCustomFields(c *gin.Context) {
	fields, err := h.queries.ListCustomFields(c.Request.Context(), store.ListCustomFieldsParams{
		CompanyID:       auth.GetCompanyID(c),
		IncludeInactive: c.Query("include_inactive") == "true",
	})
	if err != nil {
		response.InternalError(c, "Failed to list custom fields")
		return
	}
	response.OK(c, fields)
}

// CreateCustomField handles POST /custom-fields.
func (h *Handler) CreateCustomField(c *gin.Context) {
	var req customFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := req.normalize(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	key := NormalizeFieldKey(req.Key)
	if key == "" {
		key = NormalizeFieldKey(req.Label)
	}
	if key == "" {
		response.BadRequest(c, "Could not derive a field key; provide one")
		return
	}
	f, err := h.queries.CreateCustomField(c.Request.Context(), store.CreateCustomFieldParams{
		CompanyID:   auth.GetCompanyID(c),
		FieldKey:    key,
		Label:       req.Label,
		FieldType:   req.FieldType,
		Options:     req.Options,
		IsRequired:  req.IsRequired,
		IsSensitive: req.IsSensitive,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.Conflict(c, "A field with this key already exists")
		return
	}
	response.Created(c, f)
}

// UpdateCustomField handles PUT /custom-fields/:id. The key and type are immutable.
func (h *Handler) UpdateCustomField(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req customFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := req.normalize(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	active := req.IsActive == nil || *req.IsActive
	f, err := h.queries.UpdateCustomField(c.Request.Context(), store.UpdateCustomFieldParams{
		ID:          id,
		CompanyID:   auth.GetCompanyID(c),
		Label:       req.Label,
		Options:     req.Options,
		IsRequired:  req.IsRequired,
		IsSensitive: req.IsSensitive,
		SortOrder:   req.SortOrder,
		IsActive:    active,
	})
	if err != nil {
		response.NotFound(c, "Custom field not found")
		return
	}
	response.OK(c, f)
}

// GetEmployeeCustomFields handles GET /employees/:id/custom-fields.
// Sensitive values are only returned to admins.
func (h *Handler) GetEmployeeCustomFields(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	companyID := auth.GetCompanyID(c)
	if _, err := h.queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	rows, err := h.queries.ListEmployeeCustomFieldValues(c.Request.Context(), store.ListEmployeeCustomFieldValuesParams{
		EmployeeID: id,
		CompanyID:  companyID,
	})
	if err != nil {
		response.InternalError(c, "Failed to load custom fields")
		return
	}
	if !isAdmin(auth.GetRole(c)) {
		for i := range rows {
			if rows[i].IsSensitive {
				rows[i].Value = nil
			}
		}
	}
	response.OK(c, rows)
}

// UpdateEmployeeCustomFields handles PUT /employees/:id/custom-fields with
// {"values": {"field_key": "value"}}. An empty value clears the field.
func (h *Handler) UpdateEmployeeCustomFields(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Values map[string]string `json:"values" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	if _, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{ID: id, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	specs, ids, err := h.svc.fieldSpecs(ctx, companyID)
	if err != nil {
		response.InternalError(c, "Failed to load custom fields")
		return
	}
	values, err := ValidateFieldValues(specs, req.Values, false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	for k, v := range values {
		if v == "" {
			err = h.queries.DeleteCustomFieldValue(ctx, store.DeleteCustomFieldValueParams{EmployeeID: id, FieldID: ids[k], CompanyID: companyID})
		} else {
			err = h.queries.UpsertCustomFieldValue(ctx, store.UpsertCustomFieldValueParams{
				CompanyID: companyID, EmployeeID: id, FieldID: ids[k], Value: v, UpdatedBy: &userID,
			})
		}
		if err != nil {
			response.InternalError(c, "Failed to save custom fields")
			return
		}
	}
	h.GetEmployeeCustomFields(c)
}

func isAdmin(r auth.Role) bool { return r == auth.RoleAdmin || r == auth.RoleSuperAdmin }
