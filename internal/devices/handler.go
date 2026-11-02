package devices

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler exposes device endpoints (admin).
type Handler struct {
	svc *Service
	q   *store.Queries
}

// NewHandler creates the handler.
func NewHandler(svc *Service, q *store.Queries) *Handler { return &Handler{svc: svc, q: q} }

// RegisterRoutes registers /devices routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/devices", auth.AdminOnly())
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PUT("/:id", h.Update)
	g.POST("/:id/assign", h.Assign)
	g.POST("/:id/unassign", h.Unassign)
	g.POST("/:id/actions", h.RequestAction)
	g.GET("/actions", h.ListActions)
	g.POST("/actions/:id/approve", h.Approve)
	g.POST("/actions/:id/cancel", h.CancelAction)
	g.POST("/actions/:id/complete", h.CompleteManual)
	g.GET("/mdm", h.GetMDM)
	g.PUT("/mdm", h.SaveMDM)
}

func pid(c *gin.Context) (int64, bool) {
	v, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || v <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return v, true
}

type deviceReq struct {
	AssetTag    string  `json:"asset_tag" binding:"required"`
	Kind        string  `json:"kind"`
	MakeModel   string  `json:"make_model"`
	SerialNo    string  `json:"serial_no"`
	OS          string  `json:"os"`
	MDMDeviceID *string `json:"mdm_device_id"`
	Notes       *string `json:"notes"`
	Status      string  `json:"status"`
}

var kinds = map[string]bool{"laptop": true, "desktop": true, "phone": true, "tablet": true, "monitor": true, "other": true}

func (h *Handler) List(c *gin.Context) {
	emp, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	rows, err := h.q.ListDevices(c.Request.Context(), store.ListDevicesParams{CompanyID: auth.GetCompanyID(c), EmployeeID: emp})
	if err != nil {
		response.InternalError(c, "Failed to list devices")
		return
	}
	response.OK(c, rows)
}

func (h *Handler) Create(c *gin.Context) {
	var r deviceReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if r.Kind == "" {
		r.Kind = "laptop"
	}
	if !kinds[r.Kind] {
		response.BadRequest(c, "Invalid kind")
		return
	}
	d, err := h.q.CreateDevice(c.Request.Context(), store.CreateDeviceParams{
		CompanyID: auth.GetCompanyID(c), AssetTag: strings.TrimSpace(r.AssetTag), Kind: r.Kind, MakeModel: r.MakeModel,
		SerialNo: r.SerialNo, Os: r.OS, MdmDeviceID: r.MDMDeviceID, Notes: r.Notes,
	})
	if err != nil {
		response.Conflict(c, "Asset tag already exists")
		return
	}
	response.Created(c, d)
}

var settable = map[string]bool{"": true, "in_stock": true, "lost": true, "retired": true, "assigned": true}

func (h *Handler) Update(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r deviceReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if r.Kind == "" {
		r.Kind = "laptop"
	}
	if !kinds[r.Kind] || !settable[r.Status] {
		response.BadRequest(c, "Invalid kind or status")
		return
	}
	d, err := h.q.UpdateDevice(c.Request.Context(), store.UpdateDeviceParams{
		ID: id, CompanyID: auth.GetCompanyID(c), Kind: r.Kind, MakeModel: r.MakeModel, SerialNo: r.SerialNo, Os: r.OS,
		MdmDeviceID: r.MDMDeviceID, Notes: r.Notes, NewStatus: r.Status,
	})
	if err != nil {
		response.NotFound(c, "Device not found")
		return
	}
	response.OK(c, d)
}

func (h *Handler) Assign(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r struct {
		EmployeeID int64 `json:"employee_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	if _, err := h.q.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{ID: r.EmployeeID, CompanyID: companyID}); err != nil {
		response.NotFound(c, "Employee not found")
		return
	}
	d, err := h.q.AssignDevice(c.Request.Context(), store.AssignDeviceParams{ID: id, CompanyID: companyID, AssignedEmployeeID: &r.EmployeeID})
	if err != nil {
		response.Conflict(c, "Device is not available for assignment")
		return
	}
	response.OK(c, d)
}

func (h *Handler) Unassign(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	d, err := h.q.UnassignDevice(c.Request.Context(), store.UnassignDeviceParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.NotFound(c, "Device not found")
		return
	}
	response.OK(c, d)
}

func (h *Handler) RequestAction(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := auth.GetUserID(c)
	a, err := h.svc.RequestAction(c.Request.Context(), auth.GetCompanyID(c), id, r.Action, "manual", &uid, false)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			response.BadRequest(c, strings.TrimPrefix(err.Error(), "invalid: "))
			return
		}
		response.InternalError(c, "Failed to queue action")
		return
	}
	response.Created(c, a)
}

func (h *Handler) ListActions(c *gin.Context) {
	dev, _ := strconv.ParseInt(c.Query("device_id"), 10, 64)
	rows, err := h.q.ListDeviceActions(c.Request.Context(), store.ListDeviceActionsParams{CompanyID: auth.GetCompanyID(c), DeviceID: dev})
	if err != nil {
		response.InternalError(c, "Failed to list actions")
		return
	}
	response.OK(c, rows)
}

func (h *Handler) Approve(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	uid := auth.GetUserID(c)
	a, err := h.q.ApproveDeviceAction(c.Request.Context(), store.ApproveDeviceActionParams{ID: id, CompanyID: auth.GetCompanyID(c), ApprovedBy: &uid})
	if err != nil {
		response.Conflict(c, "Action is not waiting for approval")
		return
	}
	response.OK(c, a)
}

func (h *Handler) CancelAction(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	a, err := h.q.CancelDeviceAction(c.Request.Context(), store.CancelDeviceActionParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.Conflict(c, "Action cannot be cancelled")
		return
	}
	response.OK(c, a)
}

func (h *Handler) CompleteManual(c *gin.Context) {
	id, ok := pid(c)
	if !ok {
		return
	}
	var r struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&r)
	companyID := auth.GetCompanyID(c)
	a, err := h.q.CompleteManualDeviceAction(c.Request.Context(), store.CompleteManualDeviceActionParams{ID: id, CompanyID: companyID, Result: &r.Note})
	if err != nil {
		response.Conflict(c, "Only manual or failed actions can be marked done")
		return
	}
	switch a.Action {
	case "return_request":
		_, _ = h.q.UnassignDevice(c.Request.Context(), store.UnassignDeviceParams{ID: a.DeviceID, CompanyID: companyID})
	case "lock":
		_ = h.q.SetDeviceStatus(c.Request.Context(), store.SetDeviceStatusParams{ID: a.DeviceID, Status: "locked"})
	case "wipe":
		_ = h.q.SetDeviceStatus(c.Request.Context(), store.SetDeviceStatusParams{ID: a.DeviceID, Status: "wiped"})
	}
	response.OK(c, a)
}

func (h *Handler) GetMDM(c *gin.Context) {
	conn, err := h.q.GetMDMConnection(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.OK(c, nil)
		return
	}
	has := conn.SecretEnc != ""
	conn.SecretEnc = ""
	response.OK(c, gin.H{"connection": conn, "has_secret": has})
}

func (h *Handler) SaveMDM(c *gin.Context) {
	var r struct {
		Provider string `json:"provider"`
		URL      string `json:"url" binding:"required"`
		Secret   string `json:"secret"`
		AutoLock *bool  `json:"auto_lock_on_termination"`
	}
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	auto := r.AutoLock == nil || *r.AutoLock
	conn, err := h.svc.SaveConnection(c.Request.Context(), auth.GetCompanyID(c), r.Provider, r.URL, r.Secret, auto)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			response.BadRequest(c, strings.TrimPrefix(err.Error(), "invalid: "))
			return
		}
		response.InternalError(c, "Failed to save MDM connection")
		return
	}
	response.OK(c, conn)
}
