package govfiling

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/govfiling/browser"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler exposes the filing agent.
type Handler struct {
	svc    *Service
	q      *store.Queries
	logger *slog.Logger
}

// NewHandler creates the handler.
func NewHandler(svc *Service, q *store.Queries, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, q: q, logger: logger}
}

// RegisterRoutes registers /gov routes (admin only).
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/gov", auth.AdminOnly())
	g.GET("/forms", h.ListForms)
	g.GET("/accounts", h.ListAccounts)
	g.POST("/accounts", h.SaveAccount)
	g.DELETE("/accounts/:id", h.DeleteAccount)
	g.GET("/submissions", h.ListSubmissions)
	g.POST("/submissions", h.Prepare)
	g.GET("/submissions/:id", h.GetSubmission)
	g.POST("/submissions/:id/approve", h.Approve)
	g.POST("/submissions/:id/dry-run", h.DryRun)
	g.POST("/submissions/:id/cancel", h.Cancel)
	g.POST("/submissions/:id/record", h.Record)
	g.GET("/submissions/:id/file", h.DownloadFile)
	g.GET("/submissions/:id/evidence", h.Evidence)
	g.GET("/human-requests", h.ListHumanRequests)
	g.POST("/human-requests/:id/answer", h.AnswerHuman)
	g.GET("/human-requests/:id/screenshot", h.HumanScreenshot)
	g.GET("/playbooks", h.ListPlaybooks)
	g.POST("/playbooks", h.SavePlaybook)
	g.POST("/playbooks/:id/verify", h.VerifyPlaybook)
	g.DELETE("/playbooks/:id", h.DeletePlaybook)
}

func (h *Handler) fail(c *gin.Context, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		response.BadRequest(c, ve.Msg)
	case errors.Is(err, ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		response.NotFound(c, "Not found")
	case errors.Is(err, ErrConflict):
		response.Conflict(c, strings.TrimPrefix(err.Error(), ErrConflict.Error()+": "))
	default:
		h.logger.Error("govfiling", "error", err)
		response.InternalError(c, "Government filing request failed")
	}
}

func id(c *gin.Context) (int64, bool) {
	v, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || v <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return v, true
}

// ListForms lists supported returns, filtered by the company's country unless ?country= is given.
func (h *Handler) ListForms(c *gin.Context) {
	country := c.Query("country")
	if country == "" {
		if comp, err := h.q.GetCompanyByID(c.Request.Context(), auth.GetCompanyID(c)); err == nil {
			country = comp.Country
		}
	}
	if country == "all" {
		country = ""
	}
	response.OK(c, Forms(country))
}

func (h *Handler) ListAccounts(c *gin.Context) {
	rows, err := h.q.ListGovPortalAccounts(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *Handler) SaveAccount(c *gin.Context) {
	var in AccountInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	a, err := h.svc.SaveAccount(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	a.CredentialsEnc = "" // never echo secrets
	response.OK(c, a)
}

func (h *Handler) DeleteAccount(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	if err := h.q.DeleteGovPortalAccount(c.Request.Context(), store.DeleteGovPortalAccountParams{ID: v, CompanyID: auth.GetCompanyID(c)}); err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": v})
}

func (h *Handler) ListSubmissions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	companyID := auth.GetCompanyID(c)
	rows, err := h.q.ListGovSubmissions(c.Request.Context(), store.ListGovSubmissionsParams{
		CompanyID: companyID, Status: c.Query("status"), Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	counts, _ := h.q.CountGovSubmissionsByStatus(c.Request.Context(), companyID)
	cm := map[string]int64{}
	for _, r := range counts {
		cm[r.Status] = r.N
	}
	response.OK(c, gin.H{"items": rows, "counts": cm})
}

func (h *Handler) Prepare(c *gin.Context) {
	var in PrepareInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := auth.GetUserID(c)
	p, err := h.svc.Prepare(c.Request.Context(), auth.GetCompanyID(c), &uid, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Created(c, p)
}

func (h *Handler) GetSubmission(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	sub, err := h.svc.get(c.Request.Context(), auth.GetCompanyID(c), v)
	if err != nil {
		h.fail(c, err)
		return
	}
	events, _ := h.q.ListGovSubmissionEvents(c.Request.Context(), v)
	spec, _ := Lookup(sub.FormCode)
	var pf Preflight
	_ = json.Unmarshal(sub.Preflight, &pf)
	response.OK(c, gin.H{"submission": sub, "events": events, "form": spec, "preflight": pf})
}

func (h *Handler) Approve(c *gin.Context) { h.approve(c, false) }
func (h *Handler) DryRun(c *gin.Context)  { h.approve(c, true) }

func (h *Handler) approve(c *gin.Context, dry bool) {
	v, ok := id(c)
	if !ok {
		return
	}
	sub, err := h.svc.Approve(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), v, dry)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, sub)
}

func (h *Handler) Cancel(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	sub, err := h.svc.Cancel(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), v)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, sub)
}

func (h *Handler) Record(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	var req struct {
		ReferenceNo string `json:"reference_no" binding:"required"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	sub, err := h.svc.RecordManual(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), v, req.ReferenceNo, req.Note)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, sub)
}

func (h *Handler) DownloadFile(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	companyID := auth.GetCompanyID(c)
	sub, err := h.svc.get(c.Request.Context(), companyID, v)
	if err != nil || sub.FilePath == nil {
		response.NotFound(c, "File not found")
		return
	}
	full, err := h.svc.FilePath(companyID, *sub.FilePath)
	if err != nil {
		response.NotFound(c, "File not found")
		return
	}
	c.FileAttachment(full, deref(sub.FileName))
}

// Evidence serves a screenshot of a submission (?file=<relative path from events>).
func (h *Handler) Evidence(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	companyID := auth.GetCompanyID(c)
	if _, err := h.svc.get(c.Request.Context(), companyID, v); err != nil {
		h.fail(c, err)
		return
	}
	rel := c.Query("file")
	want := filepath.Join(strconv.FormatInt(companyID, 10), "evidence", strconv.FormatInt(v, 10)) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(rel), want) {
		response.NotFound(c, "File not found")
		return
	}
	full, err := h.svc.FilePath(companyID, rel)
	if err != nil {
		response.NotFound(c, "File not found")
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.File(full)
}

func (h *Handler) ListHumanRequests(c *gin.Context) {
	rows, err := h.q.ListOpenGovHumanRequests(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *Handler) AnswerHuman(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	var req struct {
		Answer string `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.Answer(c.Request.Context(), auth.GetCompanyID(c), auth.GetUserID(c), v, req.Answer); err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, gin.H{"answered": v})
}

func (h *Handler) HumanScreenshot(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	companyID := auth.GetCompanyID(c)
	req, err := h.q.GetGovHumanRequest(c.Request.Context(), v)
	if err != nil || req.CompanyID != companyID || req.ScreenshotPath == nil {
		response.NotFound(c, "Screenshot not found")
		return
	}
	full, err := h.svc.FilePath(companyID, *req.ScreenshotPath)
	if err != nil {
		response.NotFound(c, "Screenshot not found")
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.File(full)
}

func (h *Handler) ListPlaybooks(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	rows, err := h.q.ListGovPlaybooks(c.Request.Context(), &companyID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, rows)
}

// SavePlaybook stores a company playbook. Saving always resets verification.
func (h *Handler) SavePlaybook(c *gin.Context) {
	var req struct {
		FormCode   string           `json:"form_code" binding:"required"`
		Definition browser.Playbook `json:"definition" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if _, ok := Lookup(req.FormCode); !ok {
		response.BadRequest(c, "Unknown form_code")
		return
	}
	if err := req.Definition.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !strings.HasPrefix(req.Definition.StartURL, "https://") && !strings.HasPrefix(req.Definition.StartURL, "http://localhost") && !strings.HasPrefix(req.Definition.StartURL, "http://127.0.0.1") {
		response.BadRequest(c, "start_url must use https")
		return
	}
	def, _ := json.Marshal(req.Definition)
	companyID, uid := auth.GetCompanyID(c), auth.GetUserID(c)
	row, err := h.q.UpsertGovPlaybook(c.Request.Context(), store.UpsertGovPlaybookParams{
		CompanyID: &companyID, PlaybookKey: req.Definition.ID, FormCode: req.FormCode, Definition: def, CreatedBy: &uid,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) VerifyPlaybook(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	var req struct {
		Verified bool `json:"verified"`
	}
	_ = c.ShouldBindJSON(&req)
	companyID, uid := auth.GetCompanyID(c), auth.GetUserID(c)
	row, err := h.q.VerifyGovPlaybook(c.Request.Context(), store.VerifyGovPlaybookParams{ID: v, CompanyID: &companyID, Verified: req.Verified, VerifiedBy: &uid})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) DeletePlaybook(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	companyID := auth.GetCompanyID(c)
	if err := h.q.DeleteGovPlaybook(c.Request.Context(), store.DeleteGovPlaybookParams{ID: v, CompanyID: &companyID}); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
