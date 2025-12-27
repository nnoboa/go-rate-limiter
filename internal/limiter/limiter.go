// Package limiter implements the RateLimiter struct, constructer, and sliding window rate
// limiting logic.
package limiter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// A RateLimiter holds data on a Redis client, a rate limit, and the sliding window duration.
type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

// Creates a new RateLimiter.
func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		limit:  limit,
		window: window,
	}
}

// Reports whether the request is allowed. If Redis is down, the RateLimiter fails open.
func (rl *RateLimiter) IsRequestAllowed(ctx context.Context, key string) bool {
	now := time.Now().UnixMilli()
	window := rl.window.Milliseconds()
	requestID := fmt.Sprintf("%d-%d", now, time.Now().UnixNano())

	result, err := slidingWindowScript.Run(
		ctx, rl.rdb, []string{key}, now, window, rl.limit, requestID).Int()

	if err != nil {
		log.Printf("Redis Script Error: %v", err)
		return true
	}

	return result == 0
}
