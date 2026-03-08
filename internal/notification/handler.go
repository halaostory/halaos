package notification

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

// executeActionRequest is the request body for executing a notification action.
type executeActionRequest struct {
	Action string         `json:"action" binding:"required"`
	Params map[string]any `json:"params"`
}

// NotificationAction represents a single action attached to a notification.
type NotificationAction struct {
	Label  string         `json:"label"`
	Action string         `json:"action,omitempty"`
	Route  string         `json:"route,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// ExecuteAction validates the requested action exists in the notification's actions,
// marks the notification as read, and returns the action details for the frontend.
// For route-only actions, the frontend handles navigation.
// For tool-based actions, the frontend can call the AI agent with the action.
func (h *Handler) ExecuteAction(c *gin.Context) {
	notifID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	userID := auth.GetUserID(c)

	var req executeActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: action is required")
		return
	}

	// Get the notification
	notif, err := h.queries.GetNotificationByID(c.Request.Context(), store.GetNotificationByIDParams{
		ID:     notifID,
		UserID: userID,
	})
	if err != nil {
		response.NotFound(c, "Notification not found")
		return
	}

	// Parse and validate the action exists in the notification's actions
	if len(notif.Actions) == 0 {
		response.BadRequest(c, "This notification has no actions")
		return
	}

	var actions []NotificationAction
	if err := json.Unmarshal(notif.Actions, &actions); err != nil {
		response.InternalError(c, "Failed to parse notification actions")
		return
	}

	// Find the matching action
	var matchedAction *NotificationAction
	for i := range actions {
		if actions[i].Action == req.Action {
			matchedAction = &actions[i]
			break
		}
	}
	if matchedAction == nil {
		response.BadRequest(c, "Action not found in this notification")
		return
	}

	// Mark notification as read
	_ = h.queries.MarkNotificationRead(c.Request.Context(), store.MarkNotificationReadParams{
		ID:     notifID,
		UserID: userID,
	})

	// Return the action details - the frontend will handle execution
	// (either navigate to a route or call the AI agent with the tool action)
	result := gin.H{
		"notification_id": notifID,
		"action":          matchedAction.Action,
		"label":           matchedAction.Label,
		"params":          matchedAction.Params,
		"executed":        true,
	}
	if matchedAction.Route != "" {
		result["route"] = matchedAction.Route
	}

	response.OK(c, result)
}

// NotifyOpts holds optional parameters for sending notifications.
type NotifyOpts struct {
	EmailTo   string // if set, also send an email
	EmailSubj string
	EmailBody string
}

// Notify creates a notification for a user. Can be called from any service.
// The optional actions parameter accepts a single []NotificationAction value.
// Pass nil or omit to create a notification without actions.
func Notify(ctx context.Context, queries *store.Queries, logger *slog.Logger, companyID, userID int64, title, msg, category string, entityType *string, entityID *int64, actions ...[]NotificationAction) {
	var actionsJSON []byte
	if len(actions) > 0 && actions[0] != nil {
		var err error
		actionsJSON, err = json.Marshal(actions[0])
		if err != nil {
			logger.Error("failed to marshal notification actions", "error", err)
		}
	}

	_, err := queries.CreateNotification(ctx, store.CreateNotificationParams{
		CompanyID:  companyID,
		UserID:     userID,
		Title:      title,
		Message:    msg,
		Category:   category,
		EntityType: entityType,
		EntityID:   entityID,
		Actions:    actionsJSON,
	})
	if err != nil {
		logger.Error("failed to create notification", "error", err, "user_id", userID)
	}
}

// NotifyWithEmail creates a notification and optionally sends an email.
func NotifyWithEmail(ctx context.Context, queries *store.Queries, logger *slog.Logger, emailSender EmailSender,
	companyID, userID int64, title, msg, category string, entityType *string, entityID *int64,
	emailTo, emailSubj, emailBody string, actions ...[]NotificationAction) {

	Notify(ctx, queries, logger, companyID, userID, title, msg, category, entityType, entityID, actions...)

	if emailSender != nil && emailTo != "" && emailSubj != "" {
		emailSender.SendAsync(emailTo, emailSubj, emailBody)
	}
}

// EmailSender is the interface for sending emails asynchronously.
type EmailSender interface {
	SendAsync(to, subject, htmlBody string)
}
