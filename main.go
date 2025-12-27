// Package main implements the core rate limiter structs, vars, funcs, and sliding window logic.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// A CounterVec for tracking the number of requests in Prometheus.
// The "status" will be either "allowed" or "blocked".
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ratelimiter_requests_total",
		Help: "Total number of requests processed by the rate limiter",
	}, []string{"status"})
)

// A Lua script to ensure an atomic implementation of the sliding window algorithim in Redis.
// First, old requests outside the sliding window are removed.
// Second, the number of current requests within the sliding window are counted.
// Finally, if the count is below the limit, the request is added to the user's sorted set,
// and 0 is returned, otherwise, 1 is returned.
var slidingWindowScript = redis.NewScript(`
    local key = KEYS[1]
    local now = tonumber(ARGV[1])
    local window = tonumber(ARGV[2])
    local limit = tonumber(ARGV[3])
    local clearBefore = now - window

    redis.call("ZREMRANGEBYSCORE", key, 0, clearBefore)

    local currentCount = redis.call("ZCARD", key)

    if currentCount < limit then
        redis.call("ZADD", key, now, ARGV[4])
        redis.call("PEXPIRE", key, window)
        return 0
    else
        return 1
    end
`)

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

// The RateLimiter's Middleware checking if a request should be allowed or blocked.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "limit:" + ip

		if !rl.IsRequestAllowed(r.Context(), key) {
			httpRequestsTotal.WithLabelValues("blocked").Inc()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		httpRequestsTotal.WithLabelValues("allowed").Inc()
		next(w, r)
	}
}

// The background context for Redis operations.
var ctx = context.Background()

// A simple API endpoint.
func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, World!")
}

// The main.
// Connects to Redis, falling back to localhost for development.
// Creates a RateLimiter with a limit of 5 requests per minute before creating a basic
// web server and running it as a goroutine.
// Creates a channel to listen for OS shutdown signals in order to enable graceful shutdowns,
// allowing Redis to finish serving operations for 5 seconds before shutting down.
func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	fmt.Printf("Connected to Redis: %s\n", pong)

	limiter := NewRateLimiter(rdb, 5, time.Minute)

	http.HandleFunc("/", limiter.Middleware(helloWorldHandler))
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		fmt.Println("Server starting on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	fmt.Println("\nShutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := rdb.Close(); err != nil {
		log.Printf("Error closing Redis: %v", err)
	}

	fmt.Println("Server stopped.")
}
