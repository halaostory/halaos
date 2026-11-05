package selfonboard

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler exposes admin and public endpoints.
type Handler struct {
	svc *Service
	q   *store.Queries
	log *slog.Logger
}

// NewHandler creates the handler.
func NewHandler(svc *Service, q *store.Queries, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, q: q, log: logger}
}

// Service returns the underlying service.
func (h *Handler) Service() *Service { return h.svc }

// RegisterRoutes registers authenticated admin routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/lifecycle/employees/:id/onboarding-invite", auth.AdminOnly(), h.CreateInvite)
	protected.GET("/lifecycle/employees/:id/onboarding-invite", auth.ManagerOrAbove(), h.GetInvite)
	protected.GET("/documents/templates", auth.ManagerOrAbove(), h.ListTemplates)
	protected.POST("/documents/templates", auth.AdminOnly(), h.CreateTemplate)
	protected.PUT("/documents/templates/:id", auth.AdminOnly(), h.UpdateTemplate)
	protected.POST("/documents/templates/ai-draft", auth.AdminOnly(), h.AIDraft)
	protected.GET("/documents/merge-fields", auth.ManagerOrAbove(), func(c *gin.Context) { response.OK(c, MergeFields) })
	protected.GET("/documents/signature-requests", auth.ManagerOrAbove(), h.ListRequests)
	protected.POST("/documents/signature-requests", auth.AdminOnly(), h.CreateRequest)
	protected.POST("/documents/signature-requests/:id/void", auth.AdminOnly(), h.VoidRequest)
	protected.GET("/documents/signature-requests/:id/pdf", auth.ManagerOrAbove(), h.DownloadPDF)
}

// RegisterPublic registers token-authenticated routes for new hires/signers.
// read: per-IP general limiter; write: stricter limiter.
func (h *Handler) RegisterPublic(api *gin.RouterGroup, read, write gin.HandlerFunc) {
	p := api.Group("/public")
	p.GET("/onboarding/:token", read, h.PublicPortal)
	p.POST("/onboarding/:token/profile", write, h.PublicProfile)
	p.GET("/onboarding/:token/documents/:docId", read, h.PublicInviteDoc)
	p.POST("/onboarding/:token/documents/:docId/sign", write, h.PublicInviteSign)
	p.GET("/sign/:token", read, h.PublicDoc)
	p.POST("/sign/:token", write, h.PublicSign)
}

func (h *Handler) fail(c *gin.Context, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		if len(ve.Fields) > 0 {
			c.JSON(400, gin.H{"success": false, "error": gin.H{"code": "validation_error", "message": ve.Msg, "fields": ve.Fields}})
			return
		}
		response.BadRequest(c, ve.Msg)
	case errors.Is(err, ErrNotFound):
		response.NotFound(c, "Not found")
	case errors.Is(err, ErrGone):
		response.Error(c, 410, "gone", "This link has expired or is no longer valid. Please contact HR for a new one.")
	default:
		h.log.Error("selfonboard", "error", err)
		response.InternalError(c, "Request failed")
	}
}

func pathID(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || v <= 0 {
		response.BadRequest(c, "Invalid "+name)
		return 0, false
	}
	return v, true
}

// --- admin -----------------------------------------------------------------

func (h *Handler) CreateInvite(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Email     string `json:"email"`
		SendEmail bool   `json:"send_email"`
	}
	_ = c.ShouldBindJSON(&req)
	l, err := h.svc.CreateInvite(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), id, req.Email, req.SendEmail)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Created(c, l)
}

func (h *Handler) GetInvite(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	inv, err := h.q.GetLatestOnboardingInvite(c.Request.Context(), store.GetLatestOnboardingInviteParams{EmployeeID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.OK(c, nil)
		return
	}
	inv.TokenHash = ""
	response.OK(c, inv)
}

var templateKinds = map[string]bool{"offer_letter": true, "employment_contract": true, "nda": true, "policy_ack": true, "other": true}

type templateReq struct {
	Kind     string `json:"kind"`
	Name     string `json:"name" binding:"required"`
	Body     string `json:"body" binding:"required"`
	IsActive *bool  `json:"is_active"`
}

func (r *templateReq) check() error {
	if r.Kind == "" {
		r.Kind = "other"
	}
	if !templateKinds[r.Kind] {
		return invalid("invalid kind")
	}
	if _, missing := Render(r.Body, map[string]string{}); len(missing) > 0 {
		known := map[string]bool{}
		for _, f := range MergeFields {
			known[f] = true
		}
		var bad []string
		for _, m := range missing {
			if !known[m] {
				bad = append(bad, m)
			}
		}
		if len(bad) > 0 {
			return invalid("unknown merge fields: %s", strings.Join(bad, ", "))
		}
	}
	return nil
}

func (h *Handler) ListTemplates(c *gin.Context) {
	rows, err := h.q.ListDocumentTemplates(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := req.check(); err != nil {
		h.fail(c, err)
		return
	}
	uid := auth.GetUserID(c)
	t, err := h.q.CreateDocumentTemplate(c.Request.Context(), store.CreateDocumentTemplateParams{
		CompanyID: auth.GetCompanyID(c), Kind: req.Kind, Name: req.Name, Body: req.Body, CreatedBy: &uid,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Created(c, t)
}

func (h *Handler) UpdateTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := req.check(); err != nil {
		h.fail(c, err)
		return
	}
	active := req.IsActive == nil || *req.IsActive
	t, err := h.q.UpdateDocumentTemplate(c.Request.Context(), store.UpdateDocumentTemplateParams{
		ID: id, CompanyID: auth.GetCompanyID(c), Kind: req.Kind, Name: req.Name, Body: req.Body, IsActive: active,
	})
	if err != nil {
		response.NotFound(c, "Template not found")
		return
	}
	response.OK(c, t)
}

func (h *Handler) AIDraft(c *gin.Context) {
	var req struct {
		Kind         string `json:"kind" binding:"required"`
		Instructions string `json:"instructions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	body, err := h.svc.DraftTemplate(c.Request.Context(), auth.GetCompanyID(c), req.Kind, req.Instructions)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"body": body})
}

func (h *Handler) ListRequests(c *gin.Context) {
	emp, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
	rows, err := h.q.ListSignatureRequests(c.Request.Context(), store.ListSignatureRequestsParams{CompanyID: auth.GetCompanyID(c), EmployeeID: emp})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *Handler) CreateRequest(c *gin.Context) {
	var req RequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	out, err := h.svc.CreateRequest(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	out.Request.TokenHash = ""
	response.Created(c, out)
}

func (h *Handler) VoidRequest(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	r, err := h.q.VoidSignatureRequest(c.Request.Context(), store.VoidSignatureRequestParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.Conflict(c, "Only pending requests can be voided")
		return
	}
	r.TokenHash = ""
	response.OK(c, r)
}

func (h *Handler) DownloadPDF(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	r, err := h.q.GetSignatureRequest(c.Request.Context(), store.GetSignatureRequestParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		response.NotFound(c, "Not found")
		return
	}
	p, err := h.svc.SignedPDFPath(&r)
	if err != nil {
		response.NotFound(c, "Not signed yet")
		return
	}
	c.FileAttachment(p, strings.ReplaceAll(r.Title, " ", "_")+"_signed.pdf")
}

// --- public ----------------------------------------------------------------

func (h *Handler) PublicPortal(c *gin.Context) {
	v, err := h.svc.Portal(c.Request.Context(), c.Param("token"))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *Handler) PublicProfile(c *gin.Context) {
	var in ProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.SubmitProfile(c.Request.Context(), c.Param("token"), in); err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"saved": true, "completed": in.Complete})
}

func (h *Handler) PublicInviteDoc(c *gin.Context) {
	id, ok := pathID(c, "docId")
	if !ok {
		return
	}
	v, err := h.svc.ViewViaInvite(c.Request.Context(), c.Param("token"), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *Handler) PublicInviteSign(c *gin.Context) {
	id, ok := pathID(c, "docId")
	if !ok {
		return
	}
	var in SignInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	r, err := h.svc.SignViaInvite(c.Request.Context(), c.Param("token"), id, in, SignContext{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"status": r.Status})
}

func (h *Handler) PublicDoc(c *gin.Context) {
	v, err := h.svc.ViewByToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, v)
}

func (h *Handler) PublicSign(c *gin.Context) {
	var in SignInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	r, err := h.svc.SignByToken(c.Request.Context(), c.Param("token"), in, SignContext{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"status": r.Status})
}
