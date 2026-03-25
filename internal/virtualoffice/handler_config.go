package virtualoffice

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

func (h *Handler) GetConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	cfg, err := h.queries.GetVirtualOfficeConfig(c.Request.Context(), companyID)
	if err != nil {
		response.NotFound(c, "Virtual office not configured")
		return
	}
	response.OK(c, cfg)
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		Template string `json:"template" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !ValidTemplate(req.Template) {
		response.BadRequest(c, fmt.Sprintf("Invalid template. Must be one of: %v", ValidTemplateNames()))
		return
	}
	cfg, err := h.queries.UpsertVirtualOfficeConfig(c.Request.Context(), store.UpsertVirtualOfficeConfigParams{
		CompanyID: companyID,
		Template:  req.Template,
	})
	if err != nil {
		h.logger.Error("failed to upsert virtual office config", "error", err)
		response.InternalError(c, "Failed to save configuration")
		return
	}
	response.OK(c, cfg)
}

func (h *Handler) ListSeats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	seats, err := h.queries.ListSeats(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("failed to list seats", "error", err)
		response.InternalError(c, "Failed to list seats")
		return
	}
	response.OK(c, seats)
}

func (h *Handler) AssignSeat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	var req struct {
		EmployeeID int64  `json:"employee_id" binding:"required"`
		Floor      int32  `json:"floor"`
		Zone       string `json:"zone" binding:"required"`
		SeatX      int32  `json:"seat_x" binding:"required"`
		SeatY      int32  `json:"seat_y" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Floor == 0 {
		req.Floor = 1
	}
	ctx := c.Request.Context()

	emp, err := h.queries.GetEmployeeByID(ctx, store.GetEmployeeByIDParams{
		ID: req.EmployeeID, CompanyID: companyID,
	})
	if err != nil {
		response.BadRequest(c, "Employee not found")
		return
	}
	if emp.Status != "active" && emp.Status != "probationary" {
		response.BadRequest(c, "Only active or probationary employees can be assigned seats")
		return
	}

	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.BadRequest(c, "Virtual office not configured. Set up a template first.")
		return
	}
	if !InBounds(cfg.Template, int(req.SeatX), int(req.SeatY)) {
		response.BadRequest(c, "Seat coordinates out of template bounds")
		return
	}

	seat, err := h.queries.AssignSeat(ctx, store.AssignSeatParams{
		CompanyID:  companyID,
		EmployeeID: req.EmployeeID,
		Floor:      req.Floor,
		Zone:       req.Zone,
		SeatX:      req.SeatX,
		SeatY:      req.SeatY,
	})
	if err != nil {
		h.logger.Error("failed to assign seat", "error", err)
		response.BadRequest(c, "Seat already occupied or employee already assigned")
		return
	}
	h.invalidateSnapshot(c.Request.Context(), companyID)
	response.Created(c, seat)
}

func (h *Handler) RemoveSeat(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	employeeID, err := strconv.ParseInt(c.Param("employee_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid employee ID")
		return
	}
	if err := h.queries.RemoveSeat(c.Request.Context(), store.RemoveSeatParams{
		CompanyID: companyID, EmployeeID: employeeID,
	}); err != nil {
		h.logger.Error("failed to remove seat", "error", err)
		response.InternalError(c, "Failed to remove seat")
		return
	}
	h.invalidateSnapshot(c.Request.Context(), companyID)
	response.OK(c, gin.H{"message": "Seat removed"})
}

func (h *Handler) AutoAssign(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.BadRequest(c, "Virtual office not configured")
		return
	}

	unassigned, err := h.queries.ListUnassignedActiveEmployees(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list unassigned employees", "error", err)
		response.InternalError(c, "Failed to list employees")
		return
	}
	if len(unassigned) == 0 {
		response.OK(c, gin.H{"assigned": 0, "skipped": 0, "no_seats": 0})
		return
	}

	occupied, err := h.queries.ListOccupiedPositions(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to list occupied positions", "error", err)
		response.InternalError(c, "Failed to list seats")
		return
	}

	type pos struct{ floor, x, y int }
	occupiedSet := make(map[pos]bool)
	for _, o := range occupied {
		occupiedSet[pos{int(o.Floor), int(o.SeatX), int(o.SeatY)}] = true
	}

	deskGroups := GetDeskSeats(cfg.Template)
	var availableSeats []struct {
		Zone string
		X, Y int
	}
	for _, group := range deskGroups {
		for _, s := range group.Seats {
			if !occupiedSet[pos{1, s.X, s.Y}] {
				availableSeats = append(availableSeats, struct {
					Zone string
					X, Y int
				}{group.Zone, s.X, s.Y})
			}
		}
	}

	assigned := 0
	skipped := 0
	seatIdx := 0
	for _, emp := range unassigned {
		if seatIdx >= len(availableSeats) {
			break
		}
		seat := availableSeats[seatIdx]
		seatIdx++
		_, err := h.queries.AssignSeat(ctx, store.AssignSeatParams{
			CompanyID:  companyID,
			EmployeeID: emp.ID,
			Floor:      1,
			Zone:       seat.Zone,
			SeatX:      int32(seat.X),
			SeatY:      int32(seat.Y),
		})
		if err != nil {
			skipped++
			h.logger.Error("auto-assign failed for employee", "employee_id", emp.ID, "error", err)
			continue
		}
		assigned++
	}

	noSeats := 0
	if seatIdx >= len(availableSeats) && assigned+skipped < len(unassigned) {
		noSeats = len(unassigned) - assigned - skipped
	}

	h.invalidateSnapshot(ctx, companyID)
	response.OK(c, gin.H{
		"assigned": assigned,
		"skipped":  skipped,
		"no_seats": noSeats,
	})
}
