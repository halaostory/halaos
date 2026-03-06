package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

func (h *Handler) ListActivityLogs(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	action := c.DefaultQuery("action", "")
	entityType := c.DefaultQuery("entity_type", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	logs, err := h.queries.ListActivityLogs(c.Request.Context(), store.ListActivityLogsParams{
		CompanyID: companyID,
		Column2:   action,
		Column3:   entityType,
		Limit:     int32(pageSize),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list activity logs")
		return
	}

	total, err := h.queries.CountActivityLogs(c.Request.Context(), store.CountActivityLogsParams{
		CompanyID: companyID,
		Column2:   action,
		Column3:   entityType,
	})
	if err != nil {
		total = 0
	}

	response.OK(c, gin.H{
		"data":  logs,
		"total": total,
		"page":  page,
		"limit": pageSize,
	})
}

// LogActivity is a helper function callable from other services to record activity logs.
func LogActivity(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID, userID int64, action, entityType string, entityID *string, description, ipAddress, userAgent string, metadata map[string]any) {
	metaBytes := []byte("{}")
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metaBytes = b
		}
	}

	var ipPtr, uaPtr *string
	if ipAddress != "" {
		ipPtr = &ipAddress
	}
	if userAgent != "" {
		uaPtr = &userAgent
	}

	_, err := queries.CreateActivityLog(ctx, store.CreateActivityLogParams{
		CompanyID:   companyID,
		UserID:      userID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Description: description,
		IpAddress:   ipPtr,
		UserAgent:   uaPtr,
		Metadata:    metaBytes,
	})
	if err != nil {
		logger.Error("failed to create activity log", "error", err, "action", action, "entity_type", entityType)
	}
}
