package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
)

// Recovery recovers from panics and returns a 500 error.
func Recovery(logger *observability.Logger) func(http.Handler) http.Handler {
	return middleware.Recoverer
}
