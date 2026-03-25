package virtualoffice

import (
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

// RegisterRoutes registers virtual office routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	vo := protected.Group("/virtual-office")

	// Admin endpoints
	vo.GET("/config", auth.ManagerOrAbove(), h.GetConfig)
	vo.PUT("/config", auth.AdminOnly(), h.UpdateConfig)
	vo.GET("/seats", auth.ManagerOrAbove(), h.ListSeats)
	vo.POST("/seats/assign", auth.AdminOnly(), h.AssignSeat)
	vo.POST("/seats/auto", auth.AdminOnly(), h.AutoAssign)
	vo.DELETE("/seats/:employee_id", auth.AdminOnly(), h.RemoveSeat)

	// Employee endpoints
	vo.GET("/snapshot", h.GetSnapshot)
	vo.PUT("/my-status", h.UpdateMyStatus)
	vo.PUT("/my-avatar", h.UpdateMyAvatar)
}
