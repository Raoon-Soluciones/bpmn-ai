package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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
