package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterPublicUploads_OnlyLogosAndAvatars(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	files := map[string]string{
		"logos/1/logo.png":           "logo",
		"avatars/7/me.png":           "avatar",
		"documents/1/2/contract.pdf": "secret",
		"201file/1/2/passport.jpg":   "secret",
	}
	for p, body := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	registerPublicUploads(r, root)

	cases := map[string]int{
		"/uploads/logos/1/logo.png":                          200,
		"/uploads/avatars/7/me.png":                          200,
		"/uploads/documents/1/2/contract.pdf":                404,
		"/uploads/201file/1/2/passport.jpg":                  404,
		"/uploads/logos/../documents/1/2/contract.pdf":       404,
		"/uploads/logos/..%2Fdocuments%2F1%2F2/contract.pdf": 404,
	}
	for path, want := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want && !(want == 404 && w.Code >= 300 && w.Code < 500 && w.Body.String() != "secret") {
			t.Errorf("%s: got %d, want %d", path, w.Code, want)
		}
		if want == 404 && w.Body.String() == "secret" {
			t.Errorf("%s leaked a private file", path)
		}
	}
}
