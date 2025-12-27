// Package main connects to the Redis server, starts a simple web server, and awaits for an OS
// shutdown signal.
package main

import (
	"context"
	"fmt"
	"go-rate-limiter/internal/limiter"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

// Helper function to get the requester's IP address.
func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// Middleware to check if a request should be allowed or blocked.
func RateLimitMiddleware(rl *limiter.RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "limit:" + getIP(r)
		if !rl.IsRequestAllowed(r.Context(), key) {
			httpRequestsTotal.WithLabelValues("blocked").Inc()
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		httpRequestsTotal.WithLabelValues("allowed").Inc()
		next(w, r)
	}
}

// The background context for Redis operations.
var ctx = context.Background()

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

	limiter := limiter.NewRateLimiter(rdb, 5, time.Minute)

	http.HandleFunc("/", RateLimitMiddleware(limiter, HelloWorldHandler))
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

	sig := <-stop
	fmt.Printf("\nShutting down gracefully (Signal: %v)...\n", sig)

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
