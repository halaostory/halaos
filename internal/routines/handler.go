package routines

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler serves the routines API.
type Handler struct {
	engine *Engine
}

// NewHandler builds the handler.
func NewHandler(e *Engine) *Handler { return &Handler{engine: e} }

// RegisterRoutes registers /routines routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.GET("/routines", auth.AdminOnly(), h.List)
	protected.PUT("/routines/:key", auth.AdminOnly(), h.Update)
	protected.GET("/routines/runs", auth.AdminOnly(), h.ListRuns)
	protected.GET("/routines/runs/:id", auth.AdminOnly(), h.GetRun)
}

// RoutineView is a routine with the company's settings and recent activity.
type RoutineView struct {
	Definition
	Setting
	LastRun *store.AgentRoutineRun `json:"last_run"`
	Runs30d int                    `json:"runs_30d"`
	Open30d int                    `json:"needs_attention_30d"`
}

func (h *Handler) visible() []Definition {
	var out []Definition
	for _, d := range Catalog {
		if h.engine.module(d.Module) {
			out = append(out, d)
		}
	}
	return out
}

// List handles GET /routines.
func (h *Handler) List(c *gin.Context) {
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	latest := map[string]*store.AgentRoutineRun{}
	if rows, err := h.engine.q.LatestAgentRoutineRuns(ctx, companyID); err == nil {
		for i := range rows {
			latest[rows[i].RoutineKey] = &rows[i]
		}
	}
	runs, open := map[string]int{}, map[string]int{}
	if rows, err := h.engine.q.CountAgentRoutineRunsSince(ctx, store.CountAgentRoutineRunsSinceParams{
		CompanyID: companyID, CreatedAt: time.Now().AddDate(0, 0, -30),
	}); err == nil {
		for _, r := range rows {
			runs[r.RoutineKey] += int(r.N)
			if r.Status == "needs_attention" {
				open[r.RoutineKey] += int(r.N)
			}
		}
	}
	out := []RoutineView{}
	for _, d := range h.visible() {
		out = append(out, RoutineView{
			Definition: d, Setting: h.engine.Setting(ctx, companyID, d.Key),
			LastRun: latest[d.Key], Runs30d: runs[d.Key], Open30d: open[d.Key],
		})
	}
	response.OK(c, out)
}

type updateRequest struct {
	Enabled      *bool   `json:"enabled"`
	Autonomy     *string `json:"autonomy"`
	Instructions *string `json:"instructions"`
}

// Update handles PUT /routines/:key.
func (h *Handler) Update(c *gin.Context) {
	def, ok := Lookup(c.Param("key"))
	if !ok || !h.engine.module(def.Module) {
		response.NotFound(c, "Unknown routine")
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	ctx := c.Request.Context()
	companyID := auth.GetCompanyID(c)
	cur := h.engine.Setting(ctx, companyID, def.Key)
	if req.Enabled != nil {
		cur.Enabled = *req.Enabled
	}
	if req.Autonomy != nil {
		if !ValidAutonomy(*req.Autonomy) {
			response.BadRequest(c, "autonomy must be recommend, approve or autonomous")
			return
		}
		if clampAutonomy(def, *req.Autonomy) != *req.Autonomy {
			response.BadRequest(c, "This routine can at most "+map[string]string{Approve: "ask first", Recommend: "recommend"}[def.MaxAutonomy])
			return
		}
		cur.Autonomy = *req.Autonomy
	}
	if req.Instructions != nil {
		ins := strings.TrimSpace(*req.Instructions)
		if len(ins) > 4000 {
			response.BadRequest(c, "Instructions are limited to 4000 characters")
			return
		}
		cur.Instructions = ins
	}
	uid := auth.GetUserID(c)
	if _, err := h.engine.q.UpsertAgentRoutineSetting(ctx, store.UpsertAgentRoutineSettingParams{
		CompanyID: companyID, RoutineKey: def.Key, Enabled: cur.Enabled, Autonomy: cur.Autonomy,
		Instructions: cur.Instructions, UpdatedBy: &uid,
	}); err != nil {
		h.engine.logger.Error("routines: save setting", "error", err)
		response.InternalError(c, "Failed to save routine")
		return
	}
	response.OK(c, RoutineView{Definition: def, Setting: cur})
}

// ListRuns handles GET /routines/runs?routine=&limit=.
func (h *Handler) ListRuns(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := h.engine.q.ListAgentRoutineRuns(c.Request.Context(), store.ListAgentRoutineRunsParams{
		CompanyID: auth.GetCompanyID(c), RoutineKey: c.Query("routine"), Lim: int32(limit),
	})
	if err != nil {
		h.engine.logger.Error("routines: list runs", "error", err)
		response.InternalError(c, "Failed to load runs")
		return
	}
	if rows == nil {
		rows = []store.AgentRoutineRun{}
	}
	response.OK(c, rows)
}

// GetRun handles GET /routines/runs/:id.
func (h *Handler) GetRun(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	row, err := h.engine.q.GetAgentRoutineRun(c.Request.Context(), store.GetAgentRoutineRunParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "Run not found")
			return
		}
		response.InternalError(c, "Failed to load run")
		return
	}
	response.OK(c, row)
}
