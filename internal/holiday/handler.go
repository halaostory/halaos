package holiday

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

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

// List returns holidays for a given year.
func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
	year, _ := strconv.ParseInt(yearStr, 10, 32)
	holidays, err := h.queries.ListHolidays(c.Request.Context(), store.ListHolidaysParams{
		CompanyID: companyID,
		Year:      int32(year),
	})
	if err != nil {
		response.InternalError(c, "Failed to list holidays")
		return
	}
	response.OK(c, holidays)
}

// Create creates a new holiday.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		HolidayDate  string `json:"holiday_date" binding:"required"`
		HolidayType  string `json:"holiday_type" binding:"required"`
		IsNationwide *bool  `json:"is_nationwide"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	companyID := auth.GetCompanyID(c)
	date, err := time.Parse("2006-01-02", req.HolidayDate)
	if err != nil {
		response.BadRequest(c, "Invalid date format")
		return
	}
	isNationwide := true
	if req.IsNationwide != nil {
		isNationwide = *req.IsNationwide
	}
	holiday, err := h.queries.CreateHoliday(c.Request.Context(), store.CreateHolidayParams{
		CompanyID:    companyID,
		Name:         req.Name,
		HolidayDate:  date,
		HolidayType:  req.HolidayType,
		Year:         int32(date.Year()),
		IsNationwide: isNationwide,
	})
	if err != nil {
		response.InternalError(c, "Failed to create holiday")
		return
	}
	response.Created(c, holiday)
}

// Delete removes a holiday.
func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	companyID := auth.GetCompanyID(c)
	if err := h.queries.DeleteHoliday(c.Request.Context(), store.DeleteHolidayParams{
		ID: id, CompanyID: companyID,
	}); err != nil {
		response.NotFound(c, "Holiday not found")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}
