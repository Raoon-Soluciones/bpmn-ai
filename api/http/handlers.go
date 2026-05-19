package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/organization/bpmn-engine/internal/process"
	"github.com/organization/bpmn-engine/pkg/bpmn"
	"github.com/organization/bpmn-engine/pkg/store"
)

type createProcessRequest struct {
	Name    string `json:"name"`
	BPMNXML string `json:"bpmn_xml"`
}

type startCaseRequest struct {
	Title     string         `json:"title"`
	Variables map[string]any `json:"variables"`
}

type claimTaskRequest struct {
	UserID string `json:"user_id"`
}

type completeTaskRequest struct {
	Variables map[string]any `json:"variables"`
}

func (s *Server) createProcess(w http.ResponseWriter, r *http.Request) {
	var req createProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.BPMNXML == "" {
		writeError(w, http.StatusBadRequest, "bpmn_xml is required")
		return
	}

	parser := bpmn.NewParser()
	proc, err := parser.Parse([]byte(req.BPMNXML))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid BPMN XML: "+err.Error())
		return
	}

	proc.Name = req.Name

	if err := s.store.SaveProcess(r.Context(), proc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save process")
		return
	}

	s.logger.Info("process created", "process_id", proc.ID, "name", proc.Name)
	writeCreated(w, map[string]any{
		"id":   proc.ID,
		"name": proc.Name,
	})
}

func (s *Server) listProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := s.store.ListProcesses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list processes")
		return
	}

	result := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		result = append(result, map[string]any{
			"id":   p.ID,
			"name": p.Name,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	proc, err := s.store.GetProcess(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":   proc.ID,
		"name": proc.Name,
	})
}

func (s *Server) startCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "process id is required")
		return
	}

	var req startCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	proc, err := s.store.GetProcess(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	instance := process.NewInstance(proc, req.Variables)
	if req.Title != "" {
		instance.Title = req.Title
	}

	if err := s.store.CreateInstance(r.Context(), instance.ToRecord()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create case")
		return
	}

	s.logger.Info("case started", "case_id", instance.ID, "process_id", id)
	writeCreated(w, map[string]any{
		"id":     instance.ID,
		"status": instance.State,
	})
}

func (s *Server) listCases(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	cases, err := s.store.ListInstances(r.Context(), store.InstanceStatus(status))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cases")
		return
	}

	result := make([]map[string]any, 0, len(cases))
	for _, c := range cases {
		result = append(result, map[string]any{
			"id":         c.ID,
			"process_id": c.ProcessID,
			"title":      c.Title,
			"status":     string(c.Status),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	c, err := s.store.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "case not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         c.ID,
		"process_id": c.ProcessID,
		"title":      c.Title,
		"status":     string(c.Status),
		"variables":  c.Variables,
	})
}

func (s *Server) getCaseTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	flows, err := s.store.GetFlowsByInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get tasks")
		return
	}

	tasks := make([]map[string]any, 0)
	for _, f := range flows {
		if f.ElementType == bpmn.ElementTypeUserTask && f.Status == store.FlowStatusActive {
			tasks = append(tasks, map[string]any{
				"flow_id":      f.ID,
				"element_id":   f.ElementID,
				"status":       string(f.Status),
				"thread_id":    f.ThreadID,
				"started_at":   f.StartedAt,
				"duration_ms":  f.DurationMs,
			})
		}
	}

	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) getCaseHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	_, err := s.store.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "case not found")
		return
	}

	log, err := s.store.GetExecutionLog(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get history")
		return
	}

	writeJSON(w, http.StatusOK, log)
}

func (s *Server) getCaseDiagram(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	c, err := s.store.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "case not found")
		return
	}

	proc, err := s.store.GetProcess(r.Context(), c.ProcessID)
	if err != nil {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"case_id":    c.ID,
		"process_id": proc.ID,
		"elements":   len(proc.Elements),
		"flows":      len(proc.Flows),
	})
}

func (s *Server) claimTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req claimTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	s.logger.Info("task claimed", "task_id", id, "user_id", req.UserID)
	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": id,
		"claimed_by": req.UserID,
	})
}

func (s *Server) completeTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req completeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.logger.Info("task completed", "task_id", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": id,
		"status":  "completed",
	})
}
