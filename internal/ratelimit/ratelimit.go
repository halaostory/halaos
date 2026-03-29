package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/halaostory/halaos/pkg/response"
)

// Config holds rate limiting configuration.
type Config struct {
	// LoginRate is the max login attempts per window (e.g., 5).
	LoginRate int
	// LoginWindow is the window duration for login rate limiting (e.g., 15m).
	LoginWindow time.Duration
	// APIRate is the max API requests per window (e.g., 100).
	APIRate int
	// APIWindow is the window duration for API rate limiting (e.g., 1m).
	APIWindow time.Duration
	// Enabled controls whether rate limiting is active.
	Enabled bool
}

// Limiter provides Redis-based rate limiting using sliding window counters.
type Limiter struct {
	rdb    *redis.Client
	config Config
}

// New creates a new Limiter.
func New(rdb *redis.Client, cfg Config) *Limiter {
	return &Limiter{rdb: rdb, config: cfg}
}

// check increments the counter for key and returns whether the request is allowed.
func (l *Limiter) check(ctx context.Context, key string, maxRequests int, window time.Duration) (bool, int, error) {
	now := time.Now().UnixMilli()
	windowStart := now - window.Milliseconds()

	pipe := l.rdb.Pipeline()
	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	// Count requests in window
	countCmd := pipe.ZCard(ctx, key)
	// Set expiry on the key
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}

	count := int(countCmd.Val())
	remaining := maxRequests - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= maxRequests, remaining, nil
}

// LoginMiddleware returns a Gin middleware that rate limits login attempts by IP.
func (l *Limiter) LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.config.Enabled {
			c.Next()
			return
		}

		key := fmt.Sprintf("rl:login:%s", c.ClientIP())
		allowed, remaining, err := l.check(c.Request.Context(), key, l.config.LoginRate, l.config.LoginWindow)
		if err != nil {
			// On Redis error, allow the request (fail open)
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", l.config.LoginRate))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			retryAfter := int(l.config.LoginWindow.Seconds())
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			response.Error(c, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many login attempts. Please try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}

// APIMiddleware returns a Gin middleware that rate limits API requests by user ID (or IP if unauthenticated).
func (l *Limiter) APIMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.config.Enabled {
			c.Next()
			return
		}

		// Use user ID from JWT context if available, otherwise fall back to IP
		identifier := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			identifier = fmt.Sprintf("user:%v", userID)
		}

		key := fmt.Sprintf("rl:api:%s", identifier)
		allowed, remaining, err := l.check(c.Request.Context(), key, l.config.APIRate, l.config.APIWindow)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", l.config.APIRate))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			retryAfter := int(l.config.APIWindow.Seconds())
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			response.Error(c, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests. Please try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}
