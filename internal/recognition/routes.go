package recognition

import "github.com/gin-gonic/gin"

// RegisterRoutes registers recognition routes.
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	protected.POST("/recognitions", h.SendRecognition)
	protected.GET("/recognitions", h.ListRecognitions)
	protected.GET("/recognitions/my", h.ListMyRecognitions)
	protected.GET("/recognitions/stats", h.GetStats)
}
