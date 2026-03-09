package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter provides per-user rate limiting for bot messages.
type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

// NewRateLimiter creates a bot rate limiter.
func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

// Allow checks if the user is within their rate limit. Returns true if allowed.
func (r *RateLimiter) Allow(ctx context.Context, platform, userID string) bool {
	if r.rdb == nil {
		return true // No Redis = no rate limiting
	}

	key := fmt.Sprintf("bot:rl:%s:%s", platform, userID)
	now := time.Now().UnixMilli()
	windowStart := now - r.window.Milliseconds()

	pipe := r.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, r.window+time.Second)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return true // Fail open
	}

	count := results[2].(*redis.IntCmd).Val()
	return count <= int64(r.limit)
}
