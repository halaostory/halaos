package breaks

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/halaostory/halaos/internal/store"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	rdb     *redis.Client
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger, rdb *redis.Client) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger, rdb: rdb}
}
