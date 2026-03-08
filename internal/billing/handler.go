package billing

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/response"
)

// Handler handles billing HTTP endpoints.
type Handler struct {
	service *Service
	queries *store.Queries
	logger  *slog.Logger
}

// NewHandler creates a billing handler.
func NewHandler(service *Service, queries *store.Queries, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		queries: queries,
		logger:  logger,
	}
}

// RegisterRoutes adds billing routes to the router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	billing := rg.Group("/billing")
	{
		billing.GET("/balance", h.GetBalance)
		billing.GET("/transactions", h.ListTransactions)
		billing.GET("/usage/agents", h.UsageByAgent)
		billing.GET("/usage/daily", h.DailyUsage)
		billing.GET("/packages", h.ListPackages)
		billing.POST("/purchase", h.PurchaseTokens)
	}
}

// balanceResponse is the shape returned by GetBalance.
type balanceResponse struct {
	Balance        int64 `json:"balance"`
	TotalPurchased int64 `json:"total_purchased"`
	TotalGranted   int64 `json:"total_granted"`
	TotalConsumed  int64 `json:"total_consumed"`
}

// GetBalance returns the current token balance and lifetime stats.
func (h *Handler) GetBalance(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	tb, err := h.queries.GetTokenBalance(c.Request.Context(), companyID)
	if err != nil {
		// If no balance row exists, return zeros.
		balance, checkErr := h.service.CheckBalance(c.Request.Context(), companyID)
		if checkErr != nil {
			h.logger.Error("GetBalance failed", "company_id", companyID, "error", checkErr)
			response.InternalError(c, "Failed to retrieve token balance")
			return
		}
		response.OK(c, balanceResponse{
			Balance:        balance,
			TotalPurchased: 0,
			TotalGranted:   0,
			TotalConsumed:  0,
		})
		return
	}

	response.OK(c, balanceResponse{
		Balance:        tb.Balance,
		TotalPurchased: tb.TotalPurchased,
		TotalGranted:   tb.TotalGranted,
		TotalConsumed:  tb.TotalConsumed,
	})
}

// ListTransactions returns paginated token transactions.
func (h *Handler) ListTransactions(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		offset = 0
	}

	ctx := c.Request.Context()

	count, err := h.queries.CountTokenTransactions(ctx, companyID)
	if err != nil {
		h.logger.Error("CountTokenTransactions failed", "company_id", companyID, "error", err)
		response.InternalError(c, "Failed to count transactions")
		return
	}

	txns, err := h.queries.ListTokenTransactions(ctx, store.ListTokenTransactionsParams{
		CompanyID: companyID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		h.logger.Error("ListTokenTransactions failed", "company_id", companyID, "error", err)
		response.InternalError(c, "Failed to list transactions")
		return
	}

	// Calculate page from offset + limit for the paginated response.
	page := (offset / limit) + 1

	response.Paginated(c, txns, count, page, limit)
}

// UsageByAgent returns token consumption grouped by agent.
func (h *Handler) UsageByAgent(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	usage, err := h.queries.GetTokenUsageByAgent(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("GetTokenUsageByAgent failed", "company_id", companyID, "error", err)
		response.InternalError(c, "Failed to retrieve agent usage")
		return
	}

	response.OK(c, usage)
}

// DailyUsage returns daily token consumption for the last 30 days.
func (h *Handler) DailyUsage(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	usage, err := h.queries.GetDailyTokenUsage(c.Request.Context(), companyID)
	if err != nil {
		h.logger.Error("GetDailyTokenUsage failed", "company_id", companyID, "error", err)
		response.InternalError(c, "Failed to retrieve daily usage")
		return
	}

	response.OK(c, usage)
}

// ListPackages returns all active token packages.
func (h *Handler) ListPackages(c *gin.Context) {
	packages, err := h.queries.ListTokenPackages(c.Request.Context())
	if err != nil {
		h.logger.Error("ListTokenPackages failed", "error", err)
		response.InternalError(c, "Failed to list token packages")
		return
	}

	response.OK(c, packages)
}

// purchaseRequest is the JSON body for PurchaseTokens.
type purchaseRequest struct {
	PackageID int64 `json:"package_id" binding:"required"`
}

// PurchaseTokens handles token package purchases (MVP: no real payment).
func (h *Handler) PurchaseTokens(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	userID := auth.GetUserID(c)

	var req purchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: package_id is required")
		return
	}

	if req.PackageID <= 0 {
		response.BadRequest(c, "Invalid package ID")
		return
	}

	result, err := h.service.PurchaseTokens(c.Request.Context(), companyID, userID, req.PackageID)
	if err != nil {
		// Distinguish user-facing errors from internal errors.
		if err.Error() == "token package not found or inactive" {
			response.NotFound(c, err.Error())
			return
		}
		h.logger.Error("PurchaseTokens failed",
			"company_id", companyID,
			"package_id", req.PackageID,
			"error", err,
		)
		response.InternalError(c, "Failed to purchase tokens")
		return
	}

	response.OK(c, result)
}
