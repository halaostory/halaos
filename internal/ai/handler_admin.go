package ai

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// feedbackRequest is the JSON body for submitting feedback.
type feedbackRequest struct {
	Rating  string  `json:"rating" binding:"required"`
	Comment *string `json:"comment"`
}

// SubmitFeedback creates or updates feedback for a chat message.
func (h *Handler) SubmitFeedback(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	messageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid message ID")
		return
	}

	var req feedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	if req.Rating != "positive" && req.Rating != "negative" {
		response.BadRequest(c, "Rating must be 'positive' or 'negative'")
		return
	}

	feedback, err := h.queries.InsertChatFeedback(c.Request.Context(), store.InsertChatFeedbackParams{
		MessageID: messageID,
		CompanyID: companyID,
		UserID:    userID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	})
	if err != nil {
		response.InternalError(c, "Failed to save feedback")
		return
	}

	response.OK(c, feedback)
}

// GetFeedbackStats returns aggregated feedback counts by rating (admin only).
func (h *Handler) GetFeedbackStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	stats, err := h.queries.GetFeedbackStats(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to get feedback stats")
		return
	}

	response.OK(c, stats)
}

// ListRecentFeedback returns paginated recent feedback with message content (admin only).
func (h *Handler) ListRecentFeedback(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	feedbackList, err := h.queries.ListRecentFeedback(c.Request.Context(), store.ListRecentFeedbackParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list feedback")
		return
	}

	response.OK(c, feedbackList)
}

// ListAIAuditLog returns paginated AI audit logs (admin only).
func (h *Handler) ListAIAuditLog(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	logs, err := h.queries.ListAIAuditLogs(c.Request.Context(), store.ListAIAuditLogsParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		response.InternalError(c, "Failed to list AI audit logs")
		return
	}

	response.OK(c, logs)
}
