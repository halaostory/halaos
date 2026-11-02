package app

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// publicUploadDirs are the only sub-directories of the upload root that may be
// served without authentication.
var publicUploadDirs = []string{"logos", "avatars"}

// registerPublicUploads exposes /uploads/<dir>/... for public assets only.
// Everything else under the upload root (employee documents, 201 files) must
// go through authenticated download endpoints.
func registerPublicUploads(r gin.IRoutes, root string) {
	for _, d := range publicUploadDirs {
		r.Static("/uploads/"+d, filepath.Join(root, d))
	}
}
