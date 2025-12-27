package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSlidingWindowAllows(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewRateLimiter(rdb, 3, time.Second)

	ctx := context.Background()
	key := "limit:127.0.0.1"

	for i := 0; i < 3; i++ {
		if !limiter.IsRequestAllowed(ctx, key) {
			t.Errorf("request %d should have been allowed", i+1)
		}
	}
}

func TestSlidingWindowBlocks(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewRateLimiter(rdb, 0, time.Second)

	ctx := context.Background()
	key := "limit:127.0.0.1"

	if limiter.IsRequestAllowed(ctx, key) {
		t.Error("request should have been blocked")
	}
}

func TestSlidingWindowAllowsAfterWindow(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewRateLimiter(rdb, 3, time.Second)

	ctx := context.Background()
	key := "limit:127.0.0.1"

	for i := 0; i < 3; i++ {
		if !limiter.IsRequestAllowed(ctx, key) {
			t.Errorf("request %d should have been allowed", i+1)
		}
	}

	if limiter.IsRequestAllowed(ctx, key) {
		t.Error("4th request should have been blocked")
	}

	mr.FastForward(2 * time.Second)
	if !limiter.IsRequestAllowed(ctx, key) {
		t.Error("request after time advance should be allowed")
	}
}
