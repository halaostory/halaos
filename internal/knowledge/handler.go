package knowledge

import (
	"log/slog"
	"strconv"

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

func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.BadRequest(c, "Search query is required")
		return
	}

	companyID := auth.GetCompanyID(c)
	articles, err := h.queries.SearchKnowledgeArticles(c.Request.Context(), store.SearchKnowledgeArticlesParams{
		CompanyID:      &companyID,
		PlaintoTsquery: query,
		Limit:          20,
	})
	if err != nil {
		response.InternalError(c, "Search failed")
		return
	}
	response.OK(c, articles)
}

func (h *Handler) List(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	category := c.DefaultQuery("category", "")

	articles, err := h.queries.ListKnowledgeArticles(c.Request.Context(), store.ListKnowledgeArticlesParams{
		CompanyID: &companyID,
		Column2:   category,
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		response.InternalError(c, "Failed to list articles")
		return
	}
	response.OK(c, articles)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid article ID")
		return
	}

	article, err := h.queries.GetKnowledgeArticle(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Article not found")
		return
	}
	response.OK(c, article)
}

func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Category string   `json:"category" binding:"required"`
		Topic    string   `json:"topic" binding:"required"`
		Title    string   `json:"title" binding:"required"`
		Content  string   `json:"content" binding:"required"`
		Tags     []string `json:"tags"`
		Source   *string  `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	companyID := auth.GetCompanyID(c)
	if req.Tags == nil {
		req.Tags = []string{}
	}

	article, err := h.queries.CreateKnowledgeArticle(c.Request.Context(), store.CreateKnowledgeArticleParams{
		CompanyID: &companyID,
		Category:  req.Category,
		Topic:     req.Topic,
		Title:     req.Title,
		Content:   req.Content,
		Tags:      req.Tags,
		Source:    req.Source,
	})
	if err != nil {
		response.InternalError(c, "Failed to create article")
		return
	}
	response.Created(c, article)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid article ID")
		return
	}

	var req struct {
		Category string   `json:"category"`
		Topic    string   `json:"topic"`
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Source   *string  `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	article, err := h.queries.UpdateKnowledgeArticle(c.Request.Context(), store.UpdateKnowledgeArticleParams{
		ID:      id,
		Column2: req.Category,
		Column3: req.Topic,
		Column4: req.Title,
		Column5: req.Content,
		Tags:    req.Tags,
		Source:  req.Source,
	})
	if err != nil {
		response.InternalError(c, "Failed to update article")
		return
	}
	response.OK(c, article)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid article ID")
		return
	}

	if err := h.queries.DeleteKnowledgeArticle(c.Request.Context(), id); err != nil {
		response.InternalError(c, "Failed to delete article")
		return
	}
	response.OK(c, gin.H{"message": "Deleted"})
}

func (h *Handler) ListCategories(c *gin.Context) {
	companyID := auth.GetCompanyID(c)
	categories, err := h.queries.ListKnowledgeCategories(c.Request.Context(), &companyID)
	if err != nil {
		response.InternalError(c, "Failed to list categories")
		return
	}
	response.OK(c, categories)
}
