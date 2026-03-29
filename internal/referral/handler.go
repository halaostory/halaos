package referral

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Handler manages referral endpoints.
type Handler struct {
	queries *store.Queries
	logger  *slog.Logger
}

// NewHandler creates a new referral handler.
func NewHandler(queries *store.Queries, logger *slog.Logger) *Handler {
	return &Handler{queries: queries, logger: logger}
}

// RegisterRoutes registers referral routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	r := protected.Group("/referrals")
	r.GET("/code", h.GetCode)
	r.GET("/stats", h.GetStats)
	r.GET("/list", h.ListReferrals)
}

// GetCode returns the company's referral code, generating one if needed.
func (h *Handler) GetCode(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	code, err := h.queries.GetCompanyReferralCode(c.Request.Context(), companyID)
	if err != nil || code == nil || *code == "" {
		// Generate a new referral code
		newCode := generateReferralCode()
		if err := h.queries.SetCompanyReferralCode(c.Request.Context(), store.SetCompanyReferralCodeParams{
			ID:           companyID,
			ReferralCode: &newCode,
		}); err != nil {
			h.logger.Error("failed to set referral code", "error", err)
			response.InternalError(c, "Failed to generate referral code")
			return
		}
		code = &newCode
	}

	response.OK(c, gin.H{
		"referral_code": *code,
		"referral_link": "https://halaos.com/register?ref=" + *code,
	})
}

// GetStats returns referral statistics for the company.
func (h *Handler) GetStats(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	total, _ := h.queries.CountReferralsByCompany(c.Request.Context(), companyID)
	active, _ := h.queries.CountActiveReferralsByCompany(c.Request.Context(), companyID)

	response.OK(c, gin.H{
		"total_referrals":  total,
		"active_referrals": active,
	})
}

// ListReferrals lists referral events for the company.
func (h *Handler) ListReferrals(c *gin.Context) {
	companyID := auth.GetCompanyID(c)

	referrals, err := h.queries.ListReferralsByCompany(c.Request.Context(), store.ListReferralsByCompanyParams{
		ReferrerCompanyID: companyID,
		Limit:             50,
		Offset:            0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list referrals")
		return
	}

	response.OK(c, referrals)
}

func generateReferralCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return strings.ToUpper("HL" + hex.EncodeToString(b))
}
