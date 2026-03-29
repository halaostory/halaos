package notification

import (
	"context"
	"encoding/json"
	"fmt"
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

// backendActions lists actions that are executed server-side rather than via frontend navigation.
var backendActions = map[string]bool{
	"quick_sick_leave":     true,
	"quick_vacation_leave": true,
	"quick_approve":        true,
}

// ExecuteAction validates the requested action exists in the notification's actions,
// marks the notification as read, and either executes the action server-side
// or returns the action details for the frontend to handle.
func (h *Handler) ExecuteAction(c *gin.Context) {
	notifID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid notification ID")
		return
	}

	userID := auth.GetUserID(c)
	companyID := auth.GetCompanyID(c)

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

	// Check if this is a backend-executed action
	if backendActions[matchedAction.Action] {
		params := matchedAction.Params
		if params == nil {
			params = req.Params
		}
		msg, err := h.executeBackendAction(c.Request.Context(), companyID, userID, matchedAction.Action, params)
		if err != nil {
			h.logger.Error("backend action failed",
				"action", matchedAction.Action,
				"notification_id", notifID,
				"error", err,
			)
			response.InternalError(c, fmt.Sprintf("Action failed: %s", err.Error()))
			return
		}
		response.OK(c, gin.H{
			"notification_id":  notifID,
			"action":           matchedAction.Action,
			"label":            matchedAction.Label,
			"executed":         true,
			"backend_executed": true,
			"message":          msg,
		})
		return
	}

	// Return the action details - the frontend will handle execution
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

// executeBackendAction dispatches a backend-executable action and returns a success message.
func (h *Handler) executeBackendAction(ctx context.Context, companyID, userID int64, action string, params map[string]any) (string, error) {
	switch action {
	case "quick_sick_leave":
		return h.executeQuickLeave(ctx, companyID, userID, "SL", params)
	case "quick_vacation_leave":
		return h.executeQuickLeave(ctx, companyID, userID, "VL", params)
	case "quick_approve":
		return h.executeQuickApprove(ctx, companyID, userID, params)
	default:
		return "", fmt.Errorf("unknown backend action: %s", action)
	}
}

// executeQuickLeave creates a 1-day leave request for the employee.
func (h *Handler) executeQuickLeave(ctx context.Context, companyID, userID int64, leaveCode string, params map[string]any) (string, error) {
	// Find employee from user
	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID:    &userID,
		CompanyID: companyID,
	})
	if err != nil {
		return "", fmt.Errorf("employee not found for user")
	}

	// If employee_id is in params, use that (manager filing for someone else)
	employeeID := emp.ID
	if eid, ok := params["employee_id"]; ok {
		if eidFloat, ok := eid.(float64); ok {
			employeeID = int64(eidFloat)
		}
	}

	// Find leave type by code
	lt, err := h.queries.GetLeaveTypeByCode(ctx, store.GetLeaveTypeByCodeParams{
		CompanyID: companyID,
		Code:      leaveCode,
	})
	if err != nil {
		return "", fmt.Errorf("leave type %s not found", leaveCode)
	}

	// Determine date
	today := time.Now().Truncate(24 * time.Hour)
	if dateStr, ok := params["date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			today = parsed
		}
	}

	// Create a 1-day numeric
	var oneDayNum pgtype.Numeric
	_ = oneDayNum.Scan("1")

	reason := fmt.Sprintf("Filed via notification (quick %s)", leaveCode)
	lr, err := h.queries.CreateLeaveRequest(ctx, store.CreateLeaveRequestParams{
		CompanyID:   companyID,
		EmployeeID:  employeeID,
		LeaveTypeID: lt.ID,
		StartDate:   today,
		EndDate:     today,
		Days:        oneDayNum,
		Reason:      &reason,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create leave request: %w", err)
	}

	return fmt.Sprintf("Leave request #%d created (%s for %s)", lr.ID, lt.Name, today.Format("Jan 2")), nil
}

// executeQuickApprove approves a pending request (leave or overtime).
func (h *Handler) executeQuickApprove(ctx context.Context, companyID, userID int64, params map[string]any) (string, error) {
	entityType, _ := params["entity_type"].(string)
	entityIDFloat, _ := params["entity_id"].(float64)
	entityID := int64(entityIDFloat)

	if entityType == "" || entityID == 0 {
		return "", fmt.Errorf("missing entity_type or entity_id")
	}

	// Verify user has manager/admin role by checking the users table
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	if user.Role != "admin" && user.Role != "manager" && user.Role != "super_admin" {
		return "", fmt.Errorf("insufficient permissions to approve")
	}

	var result string
	switch entityType {
	case "leave_request":
		lr, err := h.queries.ApproveLeaveRequest(ctx, store.ApproveLeaveRequestParams{
			ID:         entityID,
			CompanyID:  companyID,
			ApproverID: &userID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to approve leave request: %w", err)
		}
		result = fmt.Sprintf("Leave request #%d approved", lr.ID)

	case "overtime_request":
		ot, err := h.queries.ApproveOvertimeRequest(ctx, store.ApproveOvertimeRequestParams{
			ID:         entityID,
			CompanyID:  companyID,
			ApproverID: &userID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to approve overtime request: %w", err)
		}
		result = fmt.Sprintf("Overtime request #%d approved", ot.ID)

	default:
		return "", fmt.Errorf("unsupported entity type: %s", entityType)
	}

	// Record AI decision override if decision_id present
	if decIDFloat, ok := params["decision_id"].(float64); ok && decIDFloat > 0 {
		decID := int64(decIDFloat)
		action := "approved"
		_ = h.queries.RecordDecisionOverride(ctx, store.RecordDecisionOverrideParams{
			ID:             decID,
			OverriddenBy:   &userID,
			OverrideAction: &action,
		})
	}

	return result, nil
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
