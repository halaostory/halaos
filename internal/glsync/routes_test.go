package glsync

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := New(nil, nil, "", nil, map[string]*Provider{})
	s.RegisterRoutes(r.Group("/api"), r.Group("/pub"))
}
