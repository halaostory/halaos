package attendance

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/store"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}
