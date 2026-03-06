package notification

import (
	"context"
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

func (h *Handler) ListNotifications(c *gin.Context) {
	userID := auth.GetUserID(c)

	notifications, err := h.queries.ListNotifications(c.Request.Context(), store.ListNotificationsParams{
		UserID: userID,
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list notifications")
		return
	}
	response.OK(c, notifications)
}

func (h *Handler) CountUnread(c *gin.Context) {
	userID := auth.GetUserID(c)

	count, err := h.queries.CountUnreadNotifications(c.Request.Context(), userID)
	if err != nil {
		response.OK(c, gin.H{"count": 0})
		return
	}
	response.OK(c, gin.H{"count": count})
}

func (h *Handler) MarkRead(c *gin.Context) {
	notifID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	userID := auth.GetUserID(c)

	if err := h.queries.MarkNotificationRead(c.Request.Context(), store.MarkNotificationReadParams{
		ID:     notifID,
		UserID: userID,
	}); err != nil {
		response.InternalError(c, "Failed to mark notification as read")
		return
	}
	response.OK(c, gin.H{"message": "Marked as read"})
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	userID := auth.GetUserID(c)

	if err := h.queries.MarkAllNotificationsRead(c.Request.Context(), userID); err != nil {
		response.InternalError(c, "Failed to mark all as read")
		return
	}
	response.OK(c, gin.H{"message": "All marked as read"})
}

func (h *Handler) Delete(c *gin.Context) {
	notifID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	userID := auth.GetUserID(c)

	if err := h.queries.DeleteNotification(c.Request.Context(), store.DeleteNotificationParams{
		ID:     notifID,
		UserID: userID,
	}); err != nil {
		response.InternalError(c, "Failed to delete notification")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

// NotifyOpts holds optional parameters for sending notifications.
type NotifyOpts struct {
	EmailTo   string // if set, also send an email
	EmailSubj string
	EmailBody string
}

// Notify creates a notification for a user. Can be called from any service.
// Pass emailSender as nil to skip email.
func Notify(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID, userID int64, title, msg, category string, entityType *string, entityID *int64) {
	_, err := queries.CreateNotification(ctx, store.CreateNotificationParams{
		CompanyID:  companyID,
		UserID:     userID,
		Title:      title,
		Message:    msg,
		Category:   category,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		logger.Error("failed to create notification", "error", err, "user_id", userID)
	}
}

// NotifyWithEmail creates a notification and optionally sends an email.
func NotifyWithEmail(ctx context.Context, queries *store.Queries, logger *slog.Logger, emailSender EmailSender,
	companyID, userID int64, title, msg, category string, entityType *string, entityID *int64,
	emailTo, emailSubj, emailBody string) {

	Notify(ctx, queries, logger, companyID, userID, title, msg, category, entityType, entityID)

	if emailSender != nil && emailTo != "" && emailSubj != "" {
		emailSender.SendAsync(emailTo, emailSubj, emailBody)
	}
}

// EmailSender is the interface for sending emails asynchronously.
type EmailSender interface {
	SendAsync(to, subject, htmlBody string)
}
