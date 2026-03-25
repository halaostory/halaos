package virtualoffice

import "github.com/gin-gonic/gin"

func (h *Handler) GetConfig(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) UpdateConfig(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) ListSeats(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) AssignSeat(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) AutoAssign(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (h *Handler) RemoveSeat(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
