package attendance

import (
	"fmt"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// haversineDistance calculates the distance between two GPS coordinates in meters.
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

type geofenceResult struct {
	matchedID int64
	status    string // "inside", "outside", "not_checked"
}

// checkGeofence validates GPS coordinates against active geofences.
// Returns the matched geofence (closest within radius) or "outside" if none match.
func (h *Handler) checkGeofence(c *gin.Context, companyID int64, lat, lng float64, enforceClockIn bool) geofenceResult {
	geofences, err := h.queries.ListActiveGeofences(c.Request.Context(), companyID)
	if err != nil || len(geofences) == 0 {
		return geofenceResult{status: "not_checked"}
	}

	var closestID int64
	closestDist := math.MaxFloat64
	enforced := false
	for _, gf := range geofences {
		// Check enforcement direction
		if enforceClockIn && !gf.EnforceOnClockIn {
			continue
		}
		if !enforceClockIn && !gf.EnforceOnClockOut {
			continue
		}
		enforced = true

		gfLat, _ := gf.Latitude.Float64Value()
		gfLng, _ := gf.Longitude.Float64Value()
		if !gfLat.Valid || !gfLng.Valid {
			continue
		}
		dist := haversineDistance(lat, lng, gfLat.Float64, gfLng.Float64)
		if dist <= float64(gf.RadiusMeters) && dist < closestDist {
			closestID = gf.ID
			closestDist = dist
		}
	}

	// No geofences enforce this action — allow it
	if !enforced {
		return geofenceResult{status: "not_checked"}
	}
	if closestID > 0 {
		return geofenceResult{matchedID: closestID, status: "inside"}
	}
	return geofenceResult{status: "outside"}
}

func nilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func (h *Handler) ListGeofences(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	geofences, err := h.queries.ListGeofences(c.Request.Context(), companyID)
	if err != nil {
		response.InternalError(c, "Failed to list geofences")
		return
	}
	response.OK(c, geofences)
}

func (h *Handler) CreateGeofence(c *gin.Context) {
	var req struct {
		Name              string  `json:"name" binding:"required"`
		Address           *string `json:"address"`
		Latitude          float64 `json:"latitude" binding:"required"`
		Longitude         float64 `json:"longitude" binding:"required"`
		RadiusMeters      int32   `json:"radius_meters"`
		EnforceOnClockIn  bool    `json:"enforce_on_clock_in"`
		EnforceOnClockOut bool    `json:"enforce_on_clock_out"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	if req.RadiusMeters <= 0 {
		req.RadiusMeters = 200
	}

	var lat, lng pgtype.Numeric
	_ = lat.Scan(fmt.Sprintf("%.7f", req.Latitude))
	_ = lng.Scan(fmt.Sprintf("%.7f", req.Longitude))

	gf, err := h.queries.CreateGeofence(c.Request.Context(), store.CreateGeofenceParams{
		CompanyID:         companyID,
		Name:              req.Name,
		Address:           req.Address,
		Latitude:          lat,
		Longitude:         lng,
		RadiusMeters:      req.RadiusMeters,
		EnforceOnClockIn:  req.EnforceOnClockIn,
		EnforceOnClockOut: req.EnforceOnClockOut,
		CreatedBy:         &userID,
	})
	if err != nil {
		h.logger.Error("failed to create geofence", "error", err)
		response.InternalError(c, "Failed to create geofence")
		return
	}
	response.Created(c, gf)
}

func (h *Handler) UpdateGeofence(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid geofence ID")
		return
	}

	var req struct {
		Name              string  `json:"name"`
		Address           *string `json:"address"`
		Latitude          float64 `json:"latitude" binding:"required"`
		Longitude         float64 `json:"longitude" binding:"required"`
		RadiusMeters      int32   `json:"radius_meters"`
		IsActive          bool    `json:"is_active"`
		EnforceOnClockIn  bool    `json:"enforce_on_clock_in"`
		EnforceOnClockOut bool    `json:"enforce_on_clock_out"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)

	var lat, lng pgtype.Numeric
	_ = lat.Scan(fmt.Sprintf("%.7f", req.Latitude))
	_ = lng.Scan(fmt.Sprintf("%.7f", req.Longitude))

	gf, err := h.queries.UpdateGeofence(c.Request.Context(), store.UpdateGeofenceParams{
		ID:                id,
		CompanyID:         companyID,
		Name:              req.Name,
		Address:           req.Address,
		Latitude:          lat,
		Longitude:         lng,
		RadiusMeters:      req.RadiusMeters,
		IsActive:          req.IsActive,
		EnforceOnClockIn:  req.EnforceOnClockIn,
		EnforceOnClockOut: req.EnforceOnClockOut,
	})
	if err != nil {
		response.NotFound(c, "Geofence not found")
		return
	}
	response.OK(c, gf)
}

func (h *Handler) DeleteGeofence(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid geofence ID")
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteGeofence(c.Request.Context(), store.DeleteGeofenceParams{
		ID:        id,
		CompanyID: companyID,
	}); err != nil {
		response.InternalError(c, "Failed to delete geofence")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

func (h *Handler) GetGeofenceSettings(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	enabled, _ := h.queries.IsGeofenceEnabled(c.Request.Context(), companyID)
	response.OK(c, gin.H{"geofence_enabled": enabled})
}

func (h *Handler) SetGeofenceSettings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"geofence_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	if err := h.queries.SetGeofenceEnabled(c.Request.Context(), store.SetGeofenceEnabledParams{
		ID:              companyID,
		GeofenceEnabled: req.Enabled,
	}); err != nil {
		response.InternalError(c, "Failed to update geofence settings")
		return
	}
	response.OK(c, gin.H{"geofence_enabled": req.Enabled})
}
