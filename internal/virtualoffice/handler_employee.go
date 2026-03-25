package virtualoffice

import (
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

var validAvatarTypes = map[string]bool{
	"person_1": true, "person_2": true, "person_3": true,
	"person_4": true, "person_5": true, "person_6": true,
	"cat": true, "dog": true, "rabbit": true,
	"bear": true, "penguin": true, "shiba": true,
}

var validManualStatuses = map[string]bool{
	"focused": true, "in_meeting": true, "on_break": true,
	"away": true, "overtime": true,
}

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (h *Handler) UpdateMyStatus(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		ManualStatus    *string `json:"manual_status"`
		MeetingRoomZone *string `json:"meeting_room_zone"`
		CustomStatus    *string `json:"custom_status"`
		CustomEmoji     *string `json:"custom_emoji"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Validate manual_status against allowlist
	if req.ManualStatus != nil && *req.ManualStatus != "" {
		if !validManualStatuses[*req.ManualStatus] {
			response.BadRequest(c, "Invalid status. Must be one of: focused, in_meeting, on_break, away, overtime")
			return
		}
	}

	// Validate length limits
	if req.CustomStatus != nil && len(*req.CustomStatus) > 100 {
		response.BadRequest(c, "Custom status must be 100 characters or less")
		return
	}
	if req.CustomEmoji != nil && len(*req.CustomEmoji) > 10 {
		response.BadRequest(c, "Custom emoji must be 10 characters or less")
		return
	}

	// Resolve employee from user ID
	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID: &userID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	// Check employee has a seat
	_, err = h.queries.GetSeatByEmployee(ctx, store.GetSeatByEmployeeParams{
		CompanyID: companyID, EmployeeID: emp.ID,
	})
	if err != nil {
		response.NotFound(c, "No seat assigned. Ask your admin to assign you a seat.")
		return
	}

	// Validate meeting room zone if manual_status is "in_meeting"
	if req.ManualStatus != nil && *req.ManualStatus == "in_meeting" && req.MeetingRoomZone != nil {
		cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
		if err == nil {
			if !IsValidMeetingRoom(cfg.Template, *req.MeetingRoomZone) {
				response.BadRequest(c, "Invalid meeting room zone")
				return
			}
		}
	}

	// Clear meeting_room_zone if status is not in_meeting
	if req.ManualStatus != nil && *req.ManualStatus != "in_meeting" {
		req.MeetingRoomZone = nil
	}

	if err := h.queries.UpdateSeatStatus(ctx, store.UpdateSeatStatusParams{
		CompanyID:       companyID,
		EmployeeID:      emp.ID,
		CustomStatus:    req.CustomStatus,
		CustomEmoji:     req.CustomEmoji,
		ManualStatus:    req.ManualStatus,
		MeetingRoomZone: req.MeetingRoomZone,
	}); err != nil {
		h.logger.Error("failed to update seat status", "error", err)
		response.InternalError(c, "Failed to update status")
		return
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{"message": "Status updated"})
}

func (h *Handler) UpdateMyAvatar(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		AvatarType  string `json:"avatar_type" binding:"required"`
		AvatarColor string `json:"avatar_color" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !validAvatarTypes[req.AvatarType] {
		response.BadRequest(c, "Invalid avatar type")
		return
	}
	if !hexColorRe.MatchString(req.AvatarColor) {
		response.BadRequest(c, "Invalid color. Must be hex format: #RRGGBB")
		return
	}

	emp, err := h.queries.GetEmployeeByUserID(ctx, store.GetEmployeeByUserIDParams{
		UserID: &userID, CompanyID: companyID,
	})
	if err != nil {
		response.NotFound(c, "Employee not found")
		return
	}

	if err := h.queries.UpdateSeatAvatar(ctx, store.UpdateSeatAvatarParams{
		CompanyID:   companyID,
		EmployeeID:  emp.ID,
		AvatarType:  req.AvatarType,
		AvatarColor: req.AvatarColor,
	}); err != nil {
		h.logger.Error("failed to update avatar", "error", err)
		response.InternalError(c, "Failed to update avatar")
		return
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{"message": "Avatar updated"})
}
