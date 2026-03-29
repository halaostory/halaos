package virtualoffice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/pkg/response"
)

func snapshotCacheKey(companyID int64) string {
	return fmt.Sprintf("vo:snapshot:%d", companyID)
}

func (h *Handler) invalidateSnapshot(ctx context.Context, companyID int64) {
	if h.rdb != nil {
		_ = h.rdb.Del(ctx, snapshotCacheKey(companyID)).Err()
	}
}

type SeatStatus struct {
	SeatID          int64   `json:"seat_id"`
	EmployeeID      int64   `json:"employee_id"`
	Name            string  `json:"name"`
	Position        string  `json:"position"`
	Department      string  `json:"department"`
	Floor           int32   `json:"floor"`
	Zone            string  `json:"zone"`
	SeatX           int32   `json:"seat_x"`
	SeatY           int32   `json:"seat_y"`
	AvatarType      string  `json:"avatar_type"`
	AvatarColor     string  `json:"avatar_color"`
	Status          string  `json:"status"`
	IsLate          bool    `json:"is_late"`
	CustomStatus    *string `json:"custom_status"`
	CustomEmoji     *string `json:"custom_emoji"`
	ClockInAt       *string `json:"clock_in_at"`
	LeaveType       *string `json:"leave_type"`
	MeetingRoomZone *string `json:"meeting_room_zone"`
}

type MeetingRoom struct {
	ZoneID      string  `json:"zone_id"`
	Label       string  `json:"label"`
	OccupantIDs []int64 `json:"occupant_ids"`
}

type SnapshotResponse struct {
	Template     string         `json:"template"`
	Stats        map[string]int `json:"stats"`
	Seats        []SeatStatus   `json:"seats"`
	MeetingRooms []MeetingRoom  `json:"meeting_rooms"`
}

func (h *Handler) GetSnapshot(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	ctx := c.Request.Context()

	// Try Redis cache first
	if h.rdb != nil {
		cached, err := h.rdb.Get(ctx, snapshotCacheKey(companyID)).Bytes()
		if err == nil {
			var snap SnapshotResponse
			if json.Unmarshal(cached, &snap) == nil {
				response.OK(c, snap)
				return
			}
		}
	}

	cfg, err := h.queries.GetVirtualOfficeConfig(ctx, companyID)
	if err != nil {
		response.NotFound(c, "Virtual office not configured")
		return
	}

	rows, err := h.queries.GetSnapshotSeats(ctx, companyID)
	if err != nil {
		h.logger.Error("failed to get snapshot", "error", err)
		response.InternalError(c, "Failed to load office data")
		return
	}

	seats := make([]SeatStatus, 0, len(rows))
	stats := map[string]int{
		"total_assigned": len(rows),
		"online":         0,
		"on_leave":       0,
		"in_meeting":     0,
		"offline":        0,
	}
	meetingOccupants := map[string][]int64{}

	for _, row := range rows {
		ss := SeatStatus{
			SeatID:       row.SeatID,
			EmployeeID:   row.EmployeeID,
			Name:         row.Name,
			Position:     row.Position,
			Department:   row.Department,
			Floor:        row.Floor,
			Zone:         row.Zone,
			SeatX:        row.SeatX,
			SeatY:        row.SeatY,
			AvatarType:   row.AvatarType,
			AvatarColor:  row.AvatarColor,
			CustomStatus: row.CustomStatus,
			CustomEmoji:  row.CustomEmoji,
		}

		// Status derivation priority: on_leave > manual_status > attendance > offline
		switch {
		case row.LeaveType != nil:
			ss.Status = "on_leave"
			ss.LeaveType = row.LeaveType
			stats["on_leave"]++

		case row.ManualStatus != nil && *row.ManualStatus != "":
			ss.Status = *row.ManualStatus
			if *row.ManualStatus == "in_meeting" {
				ss.MeetingRoomZone = row.MeetingRoomZone
				stats["in_meeting"]++
				if row.MeetingRoomZone != nil {
					meetingOccupants[*row.MeetingRoomZone] = append(
						meetingOccupants[*row.MeetingRoomZone], row.EmployeeID)
				}
			} else {
				stats["online"]++
			}

		case row.ClockInAt.Valid && !row.ClockOutAt.Valid:
			ss.Status = "working"
			ss.IsLate = row.LateMinutes > 0
			clockIn := row.ClockInAt.Time.Format(time.RFC3339)
			ss.ClockInAt = &clockIn
			stats["online"]++

		default:
			ss.Status = "offline"
			stats["offline"]++
		}

		seats = append(seats, ss)
	}

	// Build meeting rooms from template + occupants
	meetingRooms := make([]MeetingRoom, 0)
	tmpl, ok := Templates[cfg.Template]
	if ok {
		for _, zone := range tmpl.Zones {
			if zone.Type == "meeting_room" {
				ids := meetingOccupants[zone.ID]
				if ids == nil {
					ids = []int64{}
				}
				meetingRooms = append(meetingRooms, MeetingRoom{
					ZoneID:      zone.ID,
					Label:       zone.Label,
					OccupantIDs: ids,
				})
			}
		}
	}

	snap := SnapshotResponse{
		Template:     cfg.Template,
		Stats:        stats,
		Seats:        seats,
		MeetingRooms: meetingRooms,
	}

	// Cache in Redis for 30 seconds
	if h.rdb != nil {
		data, err := json.Marshal(snap)
		if err == nil {
			_ = h.rdb.Set(ctx, snapshotCacheKey(companyID), data, 30*time.Second).Err()
		}
	}

	response.OK(c, snap)
}
