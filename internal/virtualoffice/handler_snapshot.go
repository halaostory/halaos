package virtualoffice

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

func snapshotCacheKey(companyID int64) string {
	return fmt.Sprintf("vo:snapshot:%d", companyID)
}

func (h *Handler) invalidateSnapshot(ctx context.Context, companyID int64) {
	if h.rdb != nil {
		_ = h.rdb.Del(ctx, snapshotCacheKey(companyID)).Err()
	}
}

func (h *Handler) GetSnapshot(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
