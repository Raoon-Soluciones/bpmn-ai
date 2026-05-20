package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	cors "github.com/go-chi/cors"
	"golang.org/x/time/rate"

	"github.com/Raoon-Soluciones/bpmn-ai/api/middleware"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host           string
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxBodySize    int64
	AllowedOrigins []string
	DisableCSRF    bool
}

// Server is the HTTP API server.
type Server struct {
	config  ServerConfig
	router  *chi.Mux
	store   store.Store
	queue   *queue.WorkerPool
	logger  *observability.Logger
	metrics *observability.Metrics
	srv     *http.Server
}

// NewServer creates a new HTTP server.
func NewServer(cfg ServerConfig, s store.Store, q *queue.WorkerPool, logger *observability.Logger, metrics *observability.Metrics) *Server {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestLogger(logger, metrics))

	limiter := middleware.NewIPRateLimiter(rate.Every(time.Second/10), 20)
	r.Use(middleware.RateLimiter(limiter))

	srv := &Server{
		config:  cfg,
		router:  r,
		store:   s,
		queue:   q,
		logger:  logger,
		metrics: metrics,
	}

	srv.routes()

	return srv
}

// Router returns the chi router for testing.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Start begins serving HTTP requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	s.logger.Info("http server starting", "addr", addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("http server shutting down")
	return s.srv.Shutdown(ctx)
}
