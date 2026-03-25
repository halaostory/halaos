package virtualoffice

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
)

// Handler handles virtual office endpoints.
type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	rdb     *redis.Client
}

// NewHandler creates a new virtual office handler.
func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, rdb *redis.Client) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, rdb: rdb}
}

// ClearManualStatusForEmployee clears manual_status and meeting_room_zone for an
// employee and invalidates the snapshot cache. Intended to be called from other
// packages (e.g. attendance on clock-out).
func (h *Handler) ClearManualStatusForEmployee(ctx context.Context, companyID, employeeID int64) error {
	if err := h.queries.ClearManualStatusByEmployee(ctx, store.ClearManualStatusByEmployeeParams{
		CompanyID: companyID, EmployeeID: employeeID,
	}); err != nil {
		return err
	}
	h.invalidateSnapshot(ctx, companyID)
	return nil
}

// RegisterRoutes registers virtual office routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	vo := protected.Group("/virtual-office")

	// Admin endpoints
	vo.GET("/config", auth.ManagerOrAbove(), h.GetConfig)
	vo.PUT("/config", auth.AdminOnly(), h.UpdateConfig)
	vo.GET("/seats", auth.ManagerOrAbove(), h.ListSeats)
	vo.GET("/unassigned", auth.AdminOnly(), h.ListUnassigned)
	vo.POST("/seats/assign", auth.AdminOnly(), h.AssignSeat)
	vo.POST("/seats/auto", auth.AdminOnly(), h.AutoAssign)
	vo.DELETE("/seats/:employee_id", auth.AdminOnly(), h.RemoveSeat)

	// Employee endpoints
	vo.GET("/snapshot", h.GetSnapshot)
	vo.PUT("/my-status", h.UpdateMyStatus)
	vo.PUT("/my-avatar", h.UpdateMyAvatar)
}
