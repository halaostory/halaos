package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSecurityHeaders_SetsAllHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=(self)",
	}

	for header, expected := range expectedHeaders {
		actual := w.Header().Get(header)
		if actual != expected {
			t.Errorf("header %q: expected %q, got %q", header, expected, actual)
		}
	}
}

func TestSecurityHeaders_NoHSTS_WithoutTLS(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	// No TLS, no X-Forwarded-Proto header
	router.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Fatalf("expected no HSTS header for non-TLS request, got %q", hsts)
	}
}

func TestSecurityHeaders_HSTS_WithTLS(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{} // Simulate TLS connection
	router.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	expected := "max-age=31536000; includeSubDomains"
	if hsts != expected {
		t.Fatalf("expected HSTS %q, got %q", expected, hsts)
	}
}

func TestSecurityHeaders_HSTS_WithXForwardedProto(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	router.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	expected := "max-age=31536000; includeSubDomains"
	if hsts != expected {
		t.Fatalf("expected HSTS %q, got %q", expected, hsts)
	}
}

func TestSecurityHeaders_NoHSTS_WithHTTPForwardedProto(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	router.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Fatalf("expected no HSTS for HTTP forwarded proto, got %q", hsts)
	}
}

func TestSecurityHeaders_CallsNext(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeaders())

	called := false
	router.GET("/test", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
}

func TestSecurityHeaders_AllHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			router := gin.New()
			router.Use(SecurityHeaders())
			router.Handle(method, "/test", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(method, "/test", nil)
			router.ServeHTTP(w, req)

			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("expected X-Content-Type-Options nosniff for %s", method)
			}
			if w.Header().Get("X-Frame-Options") != "DENY" {
				t.Errorf("expected X-Frame-Options DENY for %s", method)
			}
		})
	}
}
