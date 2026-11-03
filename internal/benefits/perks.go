package benefits

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Perks (Warp: partner perks): offers the company curates for employees.
// Employees see the offer; the code and link are revealed when they claim
// it, which is counted so HR can see what people use.

var perkCategories = map[string]bool{"health": true, "wellness": true, "food": true, "learning": true, "travel": true, "finance": true, "software": true, "other": true}

type perkInput struct {
	Title       string `json:"title"`
	Partner     string `json:"partner"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Offer       string `json:"offer"`
	Code        string `json:"code"`
	URL         string `json:"url"`
	ExpiresOn   string `json:"expires_on"`
	Active      *bool  `json:"active"`
}

func (in *perkInput) clean() (pgtype.Date, error) {
	in.Title, in.Partner, in.Offer, in.Code, in.URL = strings.TrimSpace(in.Title), strings.TrimSpace(in.Partner), strings.TrimSpace(in.Offer), strings.TrimSpace(in.Code), strings.TrimSpace(in.URL)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" || len(in.Title) > 160 || len(in.Partner) > 120 || len(in.Offer) > 160 || len(in.Code) > 80 {
		return pgtype.Date{}, errors.New("a title is required (max 160 characters)")
	}
	if in.Category == "" {
		in.Category = "other"
	}
	if !perkCategories[in.Category] {
		return pgtype.Date{}, errors.New("unknown category")
	}
	if in.URL != "" {
		u, err := url.Parse(in.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return pgtype.Date{}, errors.New("the link must be an http(s) URL")
		}
	}
	if in.ExpiresOn == "" {
		return pgtype.Date{}, nil
	}
	d, err := time.Parse("2006-01-02", in.ExpiresOn)
	if err != nil {
		return pgtype.Date{}, errors.New("expires_on must be YYYY-MM-DD")
	}
	return pgtype.Date{Time: d, Valid: true}, nil
}

func isAdmin(c *gin.Context) bool {
	r := auth.GetRole(c)
	return r == auth.RoleAdmin || r == auth.RoleSuperAdmin
}

// ListPerks handles GET /benefits/perks.
func (h *Handler) ListPerks(c *gin.Context) {
	admin := isAdmin(c)
	rows, err := h.queries.ListPerks(c.Request.Context(), store.ListPerksParams{CompanyID: auth.GetCompanyID(c), AllRows: admin})
	if err != nil {
		response.InternalError(c, "Failed to list perks")
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, p := range rows {
		item := gin.H{"id": p.ID, "title": p.Title, "partner": p.Partner, "category": p.Category, "description": p.Description,
			"offer": p.Offer, "expires_on": p.ExpiresOn, "active": p.Active, "has_code": p.Code != "", "has_link": p.Url != ""}
		if admin {
			item["code"], item["url"], item["claims"] = p.Code, p.Url, p.Claims
		}
		out = append(out, item)
	}
	response.OK(c, out)
}

// CreatePerk handles POST /benefits/perks.
func (h *Handler) CreatePerk(c *gin.Context) {
	var in perkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid perk")
		return
	}
	exp, err := in.clean()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	uid := auth.GetUserID(c)
	var by *int64
	if uid > 0 {
		by = &uid
	}
	p, err := h.queries.CreatePerk(c.Request.Context(), store.CreatePerkParams{CompanyID: auth.GetCompanyID(c), Title: in.Title, Partner: in.Partner,
		Category: in.Category, Description: in.Description, Offer: in.Offer, Code: in.Code, Url: in.URL, ExpiresOn: exp, Active: in.Active == nil || *in.Active, CreatedBy: by})
	if err != nil {
		response.InternalError(c, "Failed to create perk")
		return
	}
	response.Created(c, p)
}

// UpdatePerk handles PUT /benefits/perks/:id.
func (h *Handler) UpdatePerk(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var in perkInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid perk")
		return
	}
	exp, err := in.clean()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p, err := h.queries.UpdatePerk(c.Request.Context(), store.UpdatePerkParams{ID: id, CompanyID: auth.GetCompanyID(c), Title: in.Title, Partner: in.Partner,
		Category: in.Category, Description: in.Description, Offer: in.Offer, Code: in.Code, Url: in.URL, ExpiresOn: exp, Active: in.Active == nil || *in.Active})
	if errors.Is(err, pgx.ErrNoRows) {
		response.NotFound(c, "Perk not found")
		return
	} else if err != nil {
		response.InternalError(c, "Failed to update perk")
		return
	}
	response.OK(c, p)
}

// DeletePerk handles DELETE /benefits/perks/:id.
func (h *Handler) DeletePerk(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.queries.DeletePerk(c.Request.Context(), store.DeletePerkParams{ID: id, CompanyID: auth.GetCompanyID(c)}); err != nil {
		response.InternalError(c, "Failed to delete perk")
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ClaimPerk handles POST /benefits/perks/:id/claim: reveals the code/link.
func (h *Handler) ClaimPerk(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	r, err := h.queries.ClaimPerk(c.Request.Context(), store.ClaimPerkParams{ID: id, CompanyID: auth.GetCompanyID(c)})
	if errors.Is(err, pgx.ErrNoRows) {
		response.NotFound(c, "This perk is no longer available")
		return
	} else if err != nil {
		response.InternalError(c, "Failed to claim perk")
		return
	}
	response.OK(c, gin.H{"code": r.Code, "url": r.Url})
}
