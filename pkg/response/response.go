package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Meta struct {
	Total  int64 `json:"total"`
	Page   int   `json:"page"`
	Limit  int   `json:"limit"`
	Pages  int   `json:"pages"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    data,
	})
}

func Paginated(c *gin.Context, data interface{}, total int64, page, limit int) {
	pages := int(total) / limit
	if int(total)%limit > 0 {
		pages++
	}
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Total: total,
			Page:  page,
			Limit: limit,
			Pages: pages,
		},
	})
}

func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "forbidden", message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "not_found", message)
}

func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, "conflict", message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "internal_error", message)
}
