package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

type requestIDKey struct{}

// RequestID generates a unique request ID for each request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey{}, reqID)
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from context.
func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok {
		return reqID
	}
	return ""
}

// ChiRequestID integrates with chi's middleware request ID.
func ChiRequestID(next http.Handler) http.Handler {
	return middleware.RequestID(next)
}

// IPRateLimiter limits requests per IP using token bucket algorithm.
type IPRateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*rate.Limiter
	rate    rate.Limit
	burst   int
}

// NewIPRateLimiter creates a new per-IP rate limiter.
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		clients: make(map[string]*rate.Limiter),
		rate:    r,
		burst:   b,
	}
}

// Allow checks if a request from the given IP is allowed.
func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.clients[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.clients[ip] = limiter
	}

	return limiter.Allow()
}

// Cleanup removes inactive clients to prevent memory leaks.
func (rl *IPRateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, limiter := range rl.clients {
		if limiter.Tokens() == float64(rl.burst) {
			delete(rl.clients, ip)
		}
	}
}

// RateLimiter limits requests per IP based on a token bucket algorithm.
// Uses chi.middleware.RealIP from request context for proxy-aware IP extraction.
func RateLimiter(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
				ip = realIP
			} else if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}
			if !limiter.Allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
