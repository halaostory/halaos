package attendance

import (
	"github.com/gin-gonic/gin"
	"github.com/halaostory/halaos/internal/auth"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/attendance/clock-in", h.ClockIn)
	protected.POST("/attendance/clock-out", h.ClockOut)
	protected.GET("/attendance/records", h.ListRecords)
	protected.GET("/attendance/summary", h.GetSummary)
	protected.GET("/attendance/shifts", h.ListShifts)
	protected.POST("/attendance/shifts", auth.AdminOnly(), h.CreateShift)
	protected.PUT("/attendance/shifts/:id", auth.AdminOnly(), h.UpdateShift)
	protected.POST("/attendance/schedules", auth.AdminOnly(), h.AssignSchedule)
	protected.GET("/attendance/schedules", auth.ManagerOrAbove(), h.ListAllSchedules)
	protected.POST("/attendance/schedules/bulk", auth.AdminOnly(), h.BulkAssignSchedule)
	protected.DELETE("/attendance/schedules/:schedule_id", auth.AdminOnly(), h.DeleteSchedule)

	// Geofencing
	protected.GET("/attendance/geofences", auth.AdminOnly(), h.ListGeofences)
	protected.POST("/attendance/geofences", auth.AdminOnly(), h.CreateGeofence)
	protected.PUT("/attendance/geofences/:id", auth.AdminOnly(), h.UpdateGeofence)
	protected.DELETE("/attendance/geofences/:id", auth.AdminOnly(), h.DeleteGeofence)
	protected.GET("/attendance/geofence-settings", h.GetGeofenceSettings)
	protected.PUT("/attendance/geofence-settings", auth.AdminOnly(), h.SetGeofenceSettings)

	// Schedule Templates
	protected.GET("/attendance/schedule-templates", auth.ManagerOrAbove(), h.ListScheduleTemplates)
	protected.POST("/attendance/schedule-templates", auth.AdminOnly(), h.CreateScheduleTemplate)
	protected.GET("/attendance/schedule-templates/:id", auth.ManagerOrAbove(), h.GetScheduleTemplate)
	protected.PUT("/attendance/schedule-templates/:id", auth.AdminOnly(), h.UpdateScheduleTemplate)
	protected.DELETE("/attendance/schedule-templates/:id", auth.AdminOnly(), h.DeleteScheduleTemplate)
	protected.POST("/attendance/schedule-templates/:id/assign", auth.AdminOnly(), h.AssignTemplate)
	protected.GET("/attendance/schedule-assignments", auth.ManagerOrAbove(), h.ListScheduleAssignments)

	// Corrections
	protected.POST("/attendance/corrections", h.CreateCorrection)
	protected.GET("/attendance/corrections", auth.ManagerOrAbove(), h.ListCorrections)
	protected.GET("/attendance/corrections/pending", auth.ManagerOrAbove(), h.ListPendingCorrections)
	protected.GET("/attendance/corrections/my", h.ListMyCorrections)
	protected.POST("/attendance/corrections/:id/approve", auth.ManagerOrAbove(), h.ApproveCorrection)
	protected.POST("/attendance/corrections/:id/reject", auth.ManagerOrAbove(), h.RejectCorrection)

	// Report
	protected.GET("/attendance/report", auth.ManagerOrAbove(), h.GetReport)
}
