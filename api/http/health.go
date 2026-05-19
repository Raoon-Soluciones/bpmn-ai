package http

import (
	"encoding/json"
	"net/http"
	"time"
)

var version = "dev"

type healthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   version,
	})
}

func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{
			Status:    "not ready",
			Timestamp: time.Now(),
			Version:   version,
		})
		return
	}

	ctx := r.Context()
	if _, err := s.store.ListProcesses(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{
			Status:    "not ready",
			Timestamp: time.Now(),
			Version:   version,
		})
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Version:   version,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}

func writeCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}
