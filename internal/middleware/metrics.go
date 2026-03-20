package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	httpActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_requests",
			Help: "Number of active HTTP requests",
		},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)
)

// normalizePath groups dynamic route segments to avoid high-cardinality labels.
func normalizePath(path string) string {
	if len(path) == 0 {
		return "/"
	}
	// Keep first two segments, replace UUIDs/IDs with :id
	segments := make([]byte, 0, len(path))
	depth := 0
	i := 0
	for i < len(path) {
		if path[i] == '/' {
			segments = append(segments, '/')
			i++
			depth++
			if depth > 3 {
				segments = append(segments, '*')
				break
			}
			// Check if next segment looks like an ID (UUID or numeric)
			j := i
			for j < len(path) && path[j] != '/' {
				j++
			}
			seg := path[i:j]
			if isIDSegment(seg) {
				segments = append(segments, ":id"...)
				i = j
			}
		} else {
			segments = append(segments, path[i])
			i++
		}
	}
	return string(segments)
}

func isIDSegment(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Numeric IDs
	allDigits := true
	for _, c := range s {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(s) > 0 {
		return true
	}
	// UUID pattern (36 chars with hyphens)
	if len(s) == 36 {
		return true
	}
	return false
}

// PrometheusMetrics returns a Gin middleware that records HTTP metrics.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := normalizePath(c.Request.URL.Path)

		httpActiveRequests.Inc()
		c.Next()
		httpActiveRequests.Dec()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
		httpResponseSize.WithLabelValues(c.Request.Method, path).Observe(float64(c.Writer.Size()))
	}
}
