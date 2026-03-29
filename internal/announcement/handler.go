package announcement

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

type Handler struct {
	queries *store.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(queries *store.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, pool: pool, logger: logger}
}

// ListActive returns currently active announcements.
func (h *Handler) ListActive(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	announcements, err := h.queries.ListAnnouncements(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list announcements")
		return
	}
	response.OK(c, announcements)
}

// ListAll returns all announcements including inactive ones (admin).
func (h *Handler) ListAll(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	announcements, err := h.queries.ListAllAnnouncements(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list announcements")
		return
	}
	response.OK(c, announcements)
}

// Create creates a new announcement.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Title             string   `json:"title" binding:"required"`
		Content           string   `json:"content" binding:"required"`
		Priority          string   `json:"priority"`
		TargetRoles       []string `json:"target_roles"`
		TargetDepartments []int64  `json:"target_departments"`
		PublishedAt       *string  `json:"published_at"`
		ExpiresAt         *string  `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	priority := req.Priority
	if priority == "" {
		priority = "normal"
	}

	var publishedAt interface{}
	if req.PublishedAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.PublishedAt); err == nil {
			publishedAt = t
		}
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
			expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	ann, err := h.queries.CreateAnnouncement(c.Request.Context(), store.CreateAnnouncementParams{
		CompanyID:         companyID,
		Title:             req.Title,
		Content:           req.Content,
		Priority:          priority,
		TargetRoles:       req.TargetRoles,
		TargetDepartments: req.TargetDepartments,
		Column7:           publishedAt,
		ExpiresAt:         expiresAt,
		CreatedBy:         &userID,
	})
	if err != nil {
		h.logger.Error("failed to create announcement", "error", err)
		response.InternalError(c, "Failed to create announcement")
		return
	}
	response.Created(c, ann)
}

// Delete removes an announcement.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}
	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteAnnouncement(c.Request.Context(), store.DeleteAnnouncementParams{
		ID: id, CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Announcement not found")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}
