package virtualoffice

// TemplateZone defines a zone within an office template.
type TemplateZone struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"` // desk_area, meeting_room, cafe, lounge, phone_booth
	Label    string    `json:"label"`
	X, Y     int       `json:"x"`
	W, H     int       `json:"w"`
	Capacity int       `json:"capacity,omitempty"`
	Seats    []SeatPos `json:"seats,omitempty"`
}

// SeatPos is a grid coordinate for a seat.
type SeatPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// TemplateInfo defines an office layout template.
type TemplateInfo struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	MaxSeats int            `json:"max_seats"`
	Zones    []TemplateZone `json:"zones"`
}

// Templates is the registry of all available templates.
var Templates = map[string]TemplateInfo{
	"small": {
		ID: "small", Name: "Small Office", Width: 20, Height: 16, MaxSeats: 10,
		Zones: []TemplateZone{
			{
				ID: "desk-a", Type: "desk_area", Label: "Workstations",
				X: 1, Y: 1, W: 12, H: 6,
				Seats: []SeatPos{
					{2, 2}, {5, 2}, {8, 2}, {11, 2},
					{2, 5}, {5, 5}, {8, 5}, {11, 5},
					{2, 3}, {5, 3},
				},
			},
			{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room",
				X: 1, Y: 9, W: 6, H: 5, Capacity: 6,
				Seats: []SeatPos{{2, 10}, {4, 10}, {2, 12}, {4, 12}},
			},
			{
				ID: "cafe", Type: "cafe", Label: "Tea Room",
				X: 9, Y: 9, W: 5, H: 5,
			},
		},
	},
	"medium": {
		ID: "medium", Name: "Medium Office", Width: 32, Height: 24, MaxSeats: 30,
		Zones: []TemplateZone{
			{
				ID: "desk-a", Type: "desk_area", Label: "Workstations A",
				X: 1, Y: 1, W: 14, H: 10,
				Seats: func() []SeatPos {
					var seats []SeatPos
					for y := 2; y <= 8; y += 3 {
						for x := 2; x <= 12; x += 3 {
							seats = append(seats, SeatPos{x, y})
						}
					}
					return seats // 15 seats
				}(),
			},
			{
				ID: "desk-b", Type: "desk_area", Label: "Workstations B",
				X: 17, Y: 1, W: 14, H: 10,
				Seats: func() []SeatPos {
					var seats []SeatPos
					for y := 2; y <= 8; y += 3 {
						for x := 18; x <= 28; x += 3 {
							seats = append(seats, SeatPos{x, y})
						}
					}
					return seats // 15 seats
				}(),
			},
			{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room A",
				X: 1, Y: 13, W: 8, H: 6, Capacity: 8,
				Seats: []SeatPos{{2, 14}, {4, 14}, {6, 14}, {2, 16}, {4, 16}, {6, 16}},
			},
			{
				ID: "meeting-b", Type: "meeting_room", Label: "Meeting Room B",
				X: 11, Y: 13, W: 8, H: 6, Capacity: 8,
				Seats: []SeatPos{{12, 14}, {14, 14}, {16, 14}, {12, 16}, {14, 16}, {16, 16}},
			},
			{
				ID: "cafe", Type: "cafe", Label: "Tea Room",
				X: 21, Y: 13, W: 10, H: 6,
			},
			{
				ID: "lounge", Type: "lounge", Label: "Lounge",
				X: 1, Y: 20, W: 30, H: 3,
			},
		},
	},
	"large": {
		ID: "large", Name: "Large Office", Width: 48, Height: 36, MaxSeats: 100,
		Zones: func() []TemplateZone {
			zones := []TemplateZone{}
			// 4 desk areas with 25 seats each
			deskConfigs := []struct{ id, label string; x, y int }{
				{"desk-a", "Workstations A", 1, 1},
				{"desk-b", "Workstations B", 25, 1},
				{"desk-c", "Workstations C", 1, 13},
				{"desk-d", "Workstations D", 25, 13},
			}
			for _, dc := range deskConfigs {
				var seats []SeatPos
				for dy := 0; dy < 5; dy++ {
					for dx := 0; dx < 5; dx++ {
						seats = append(seats, SeatPos{dc.x + 1 + dx*4, dc.y + 1 + dy*2})
					}
				}
				zones = append(zones, TemplateZone{
					ID: dc.id, Type: "desk_area", Label: dc.label,
					X: dc.x, Y: dc.y, W: 22, H: 10,
					Seats: seats,
				})
			}
			// 3 meeting rooms
			zones = append(zones, TemplateZone{
				ID: "meeting-a", Type: "meeting_room", Label: "Meeting Room A",
				X: 1, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{2, 26}, {4, 26}, {6, 26}, {8, 26}, {2, 28}, {4, 28}, {6, 28}, {8, 28}},
			})
			zones = append(zones, TemplateZone{
				ID: "meeting-b", Type: "meeting_room", Label: "Meeting Room B",
				X: 13, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{14, 26}, {16, 26}, {18, 26}, {20, 26}, {14, 28}, {16, 28}, {18, 28}, {20, 28}},
			})
			zones = append(zones, TemplateZone{
				ID: "meeting-c", Type: "meeting_room", Label: "Meeting Room C",
				X: 25, Y: 25, W: 10, H: 6, Capacity: 10,
				Seats: []SeatPos{{26, 26}, {28, 26}, {30, 26}, {32, 26}, {26, 28}, {28, 28}, {30, 28}, {32, 28}},
			})
			// Other zones
			zones = append(zones, TemplateZone{
				ID: "cafe", Type: "cafe", Label: "Cafeteria",
				X: 37, Y: 25, W: 10, H: 6,
			})
			zones = append(zones, TemplateZone{
				ID: "lounge", Type: "lounge", Label: "Lounge",
				X: 1, Y: 33, W: 46, H: 3,
			})
			zones = append(zones, TemplateZone{
				ID: "phone-a", Type: "phone_booth", Label: "Phone Booth A",
				X: 37, Y: 33, W: 3, H: 3,
			})
			zones = append(zones, TemplateZone{
				ID: "phone-b", Type: "phone_booth", Label: "Phone Booth B",
				X: 42, Y: 33, W: 3, H: 3,
			})
			return zones
		}(),
	},
}

// ValidTemplate returns true if the template ID is valid.
func ValidTemplate(id string) bool {
	_, ok := Templates[id]
	return ok
}

// ValidTemplateNames returns the list of valid template IDs.
func ValidTemplateNames() []string {
	return []string{"small", "medium", "large"}
}

// GetMeetingRoomZones returns all meeting room zone IDs for a template.
func GetMeetingRoomZones(templateID string) []string {
	t, ok := Templates[templateID]
	if !ok {
		return nil
	}
	var zones []string
	for _, z := range t.Zones {
		if z.Type == "meeting_room" {
			zones = append(zones, z.ID)
		}
	}
	return zones
}

// IsValidMeetingRoom checks if a zone ID is a meeting room in the given template.
func IsValidMeetingRoom(templateID, zoneID string) bool {
	for _, id := range GetMeetingRoomZones(templateID) {
		if id == zoneID {
			return true
		}
	}
	return false
}

// InBounds checks if coordinates are within the template grid.
func InBounds(templateID string, x, y int) bool {
	t, ok := Templates[templateID]
	if !ok {
		return false
	}
	return x >= 0 && x < t.Width && y >= 0 && y < t.Height
}

// GetDeskSeats returns all desk seat positions for a template (for auto-assign).
func GetDeskSeats(templateID string) []struct{ Zone string; Seats []SeatPos } {
	t, ok := Templates[templateID]
	if !ok {
		return nil
	}
	var result []struct{ Zone string; Seats []SeatPos }
	for _, z := range t.Zones {
		if z.Type == "desk_area" && len(z.Seats) > 0 {
			result = append(result, struct{ Zone string; Seats []SeatPos }{z.ID, z.Seats})
		}
	}
	return result
}
