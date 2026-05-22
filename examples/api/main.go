package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/activities"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/events"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/gateways"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/engine"
	observability "github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/process"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store/memory"
)

var (
	storeInstance  store.Store
	engineInstance *engine.Engine
	auditWriter    *observability.FileAuditWriter
	dispatcher     *observability.Dispatcher
	logger         *observability.Logger
	workerPool     *queue.WorkerPool
)

func main() {
	logger = createLogger()
	storeInstance = memory.NewStore()
	dispatcher = observability.NewDispatcher()

	// Audit log — escribe un archivo .log por instancia dentro del directorio
	var err error
	auditWriter, err = observability.NewFileAuditWriter("./data/audit_logs", true, logger)
	if err != nil {
		logger.Error("error creando audit writer", "error", err)
		os.Exit(1)
	}
	defer auditWriter.Close()
	_ = observability.NewAuditor(dispatcher, auditWriter)

	// Worker pool for ServiceTask
	retry := queue.DefaultRetryPolicy()
	dlq := queue.NewDeadLetterQueue(storeInstance).WithDispatcher(dispatcher)
	workerPool = queue.NewWorkerPool(storeInstance, nil, retry, dlq, queue.WorkerPoolConfig{
		Concurrency:  2,
		PollInterval: 2 * time.Second,
	}).WithDispatcher(dispatcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerPool.Start(ctx)

	// Engine
	registry := engine.NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeUserTask, activities.NewUserTask)
	registry.Register(bpmn.ElementTypeServiceTask, activities.NewServiceTask)
	registry.Register(bpmn.ElementTypeScriptTask, activities.NewScriptTask)
	registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)
	registry.Register(bpmn.ElementTypeParallelGateway, gateways.NewParallelGateway)
	registry.Register(bpmn.ElementTypeInclusiveGateway, gateways.NewInclusiveGateway)

	engineInstance = engine.New(engine.Config{
		WorkerCount:      4,
		MaxLoops:         100,
		ExecutionTimeout: 30 * time.Second,
	}, registry, storeInstance, logger, workerPool).WithDispatcher(dispatcher)

	// HTTP server with chi
	r := chi.NewRouter()
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/processes", createProcess)
		r.Get("/processes", listProcesses)
		r.Get("/processes/{id}", getProcess)
		r.Post("/processes/{id}/start", startCase)

		r.Get("/cases", listCases)
		r.Get("/cases/{id}", getCase)
		r.Get("/cases/{id}/tasks", getCaseTasks)
		r.Get("/cases/{id}/history", getCaseHistory)

		r.Post("/tasks/{id}/complete", completeTask)

		r.Get("/audit", getAuditLog)
	})

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		logger.Info("API server escuchando", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down...")
	workerPool.Stop()
	auditWriter.Close()
	logger.Info("server stopped")
}

func createLogger() *observability.Logger {
	l, err := observability.NewFromConfig("debug", "text")
	if err != nil {
		log.Fatalf("error creando logger: %v", err)
	}
	return l
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func createProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		BPMNXML string `json:"bpmn_xml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "cuerpo invalido")
		return
	}
	if req.Name == "" || req.BPMNXML == "" {
		writeError(w, http.StatusBadRequest, "name y bpmn_xml son requeridos")
		return
	}

	parser := bpmn.NewParser()
	proc, err := parser.Parse([]byte(req.BPMNXML))
	if err != nil {
		writeError(w, http.StatusBadRequest, "XML BPMN invalido: "+err.Error())
		return
	}
	proc.Name = req.Name

	if err := storeInstance.SaveProcess(r.Context(), proc); err != nil {
		writeError(w, http.StatusInternalServerError, "error guardando proceso")
		return
	}

	logger.Info("proceso creado", "id", proc.ID, "name", proc.Name)
	writeJSON(w, http.StatusCreated, map[string]any{"id": proc.ID, "name": proc.Name})
}

func listProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := storeInstance.ListProcesses(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error listando procesos")
		return
	}
	result := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		result = append(result, map[string]any{"id": p.ID, "name": p.Name})
	}
	writeJSON(w, http.StatusOK, result)
}

func getProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	proc, err := storeInstance.GetProcess(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "proceso no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": proc.ID, "name": proc.Name})
}

func startCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Title     string         `json:"title"`
		Variables map[string]any `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	proc, err := storeInstance.GetProcess(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "proceso no encontrado")
		return
	}

	instance := process.NewInstance(proc, req.Variables)
	if req.Title != "" {
		instance.Title = req.Title
	}

	if err := storeInstance.CreateInstance(r.Context(), instance.ToRecord()); err != nil {
		writeError(w, http.StatusInternalServerError, "error creando caso")
		return
	}

	// Ejecutar el engine
	execCtx, execCancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer execCancel()

	if err := engineInstance.Run(execCtx, instance); err != nil {
		// Si el error es porque termino en WAITING no es realmente un error
		if instance.State != process.StateWaiting {
			logger.Error("error ejecutando proceso", "error", err, "case_id", instance.ID)
		}
	}

	// Actualizar en store
	storeInstance.UpdateInstance(r.Context(), instance.ToRecord())

	logger.Info("caso iniciado", "case_id", instance.ID, "status", instance.State)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     instance.ID,
		"status": instance.State,
	})
}

func listCases(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	cases, err := storeInstance.ListInstances(r.Context(), store.InstanceStatus(status))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error listando casos")
		return
	}
	result := make([]map[string]any, 0, len(cases))
	for _, c := range cases {
		result = append(result, map[string]any{
			"id":     c.ID,
			"process_id": c.ProcessID,
			"title":  c.Title,
			"status": string(c.Status),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func getCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := storeInstance.GetInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "caso no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        c.ID,
		"process_id": c.ProcessID,
		"title":     c.Title,
		"status":    string(c.Status),
		"variables": c.Variables,
	})
}

func getCaseTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	flows, err := storeInstance.GetFlowsByInstance(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error obteniendo tareas")
		return
	}
	tasks := make([]map[string]any, 0)
	for _, f := range flows {
		if f.ElementType == bpmn.ElementTypeUserTask && f.Status == store.FlowStatusActive {
			tasks = append(tasks, map[string]any{
				"flow_id":    f.ID,
				"element_id": f.ElementID,
				"status":     string(f.Status),
				"thread_id":  f.ThreadID,
				"started_at": f.StartedAt,
			})
		}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func getCaseHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	logs, err := storeInstance.GetExecutionLog(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error obteniendo history")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func completeTask(w http.ResponseWriter, r *http.Request) {
	flowID := chi.URLParam(r, "id")

	var req struct {
		Variables map[string]any `json:"variables"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	flow, err := storeInstance.GetFlow(r.Context(), flowID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tarea no encontrada")
		return
	}
	if flow.Status != store.FlowStatusActive {
		writeError(w, http.StatusConflict, "la tarea no esta activa")
		return
	}

	// Marcar flow como completado
	now := time.Now()
	flow.Status = store.FlowStatusCompleted
	flow.FinishedAt = &now
	d := int(time.Since(*flow.StartedAt).Milliseconds())
	flow.DurationMs = &d
	storeInstance.UpdateFlow(r.Context(), flow)

	// Actualizar variables en la instancia
	c, _ := storeInstance.GetInstance(r.Context(), flow.InstanceID)
	if c.Variables == nil {
		c.Variables = make(map[string]any)
	}
	for k, v := range req.Variables {
		c.Variables[k] = v
	}

	// Reconstruir instancia para el engine
	proc, _ := storeInstance.GetProcess(r.Context(), c.ProcessID)
	instance := process.NewInstance(proc, c.Variables)
	instance.ID = c.ID
	instance.State = process.State(c.Status)

	// Continuar ejecucion desde el elemento completado
	continueExecution(context.Background(), instance, flow)

	// Guardar estado actualizado
	storeInstance.UpdateInstance(context.Background(), instance.ToRecord())

	writeJSON(w, http.StatusOK, map[string]any{
		"task_id": flowID,
		"status":  "COMPLETED",
		"case_status": instance.State,
	})
}

// continueExecution ejecuta los elementos siguientes despues de completar un UserTask.
func continueExecution(ctx context.Context, instance *process.Instance, completedFlow *store.FlowRecord) {
	instance.Transition(process.StateInProgress)

	router := engine.NewFlowRouter()
	execResult := element.ExecutionResult{
		Action:   element.ActionRoute,
		FlowData: completedFlow,
	}
	nextFlows := router.Route(execResult, instance.Process, completedFlow.ThreadID)

	for executeNextFlow(ctx, instance, nextFlows) {
	}
}

// executeNextFlow ejecuta un conjunto de flows y retorna true si hay mas por procesar.
func executeNextFlow(ctx context.Context, instance *process.Instance, flows []engine.NextFlow) bool {
	var nextFlows []engine.NextFlow

	for _, nf := range flows {
		elemDef, ok := instance.Process.Elements[nf.ElementID]
		if !ok {
			continue
		}

		flowRecord := engine.CreateFlowRecord(
			instance.ID, nf.ElementID, nf.ElementType,
			nf.ThreadID, "",
		)
		storeInstance.CreateFlow(ctx, flowRecord)

		execElement, err := engineInstance.Registry().Get(elemDef)
		if err != nil {
			logger.Error("error obteniendo elemento", "error", err)
			continue
		}

		execCtx := engine.NewExecutionContext(ctx, instance, flowRecord, storeInstance, logger)
		result := execElement.Execute(ctx, execCtx)

		now := time.Now()
		flowRecord.FinishedAt = &now
		d := result.DurationMs
		flowRecord.DurationMs = &d
		storeInstance.UpdateFlow(ctx, flowRecord)

		dispatcher.Dispatch(observability.Event{
			Type:      observability.EventElementExecuted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"instance_id":  instance.ID,
				"process_id":   instance.ProcessID,
				"element_id":   nf.ElementID,
				"element_type": string(nf.ElementType),
				"action":       string(result.Action),
				"thread_id":    nf.ThreadID,
				"duration_ms":  result.DurationMs,
			},
		})

		storeInstance.LogExecution(ctx, &store.ExecutionLogEntry{
			InstanceID:  instance.ID,
			ElementID:   nf.ElementID,
			ElementType: nf.ElementType,
			Action:      string(result.Action),
			DurationMs:  result.DurationMs,
		})

		switch result.Action {
		case element.ActionRoute:
			next := router.Route(result, instance.Process, nf.ThreadID)
			nextFlows = append(nextFlows, next...)
		case element.ActionComplete:
			if isEndEvent(nf.ElementID, instance.Process) {
				instance.Transition(process.StateCompleted)
				dispatcher.Dispatch(observability.Event{
					Type:      observability.EventProcessCompleted,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"instance_id": instance.ID,
						"process_id":  instance.ProcessID,
						"from_state":  string(process.StateInProgress),
						"to_state":    string(process.StateCompleted),
					},
				})
				return false
			}
		case element.ActionForm, element.ActionWait:
			instance.Transition(process.StateWaiting)
			dispatcher.Dispatch(observability.Event{
				Type:      observability.EventProcessCompleted,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"instance_id": instance.ID,
					"process_id":  instance.ProcessID,
					"from_state":  string(process.StateInProgress),
					"to_state":    string(process.StateWaiting),
				},
			})
			return false
		case element.ActionQueue:
			next := router.Route(result, instance.Process, nf.ThreadID)
			nextFlows = append(nextFlows, next...)
		case element.ActionError:
			instance.Transition(process.StateError)
			dispatcher.Dispatch(observability.Event{
				Type:      observability.EventProcessError,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"instance_id": instance.ID,
					"process_id":  instance.ProcessID,
					"from_state":  string(process.StateInProgress),
					"to_state":    string(process.StateError),
					"error":       result.Error.Error(),
				},
			})
			return false
		case element.ActionTerminate:
			instance.Transition(process.StateTerminated)
			dispatcher.Dispatch(observability.Event{
				Type:      observability.EventProcessTerminated,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"instance_id": instance.ID,
					"process_id":  instance.ProcessID,
					"from_state":  string(process.StateInProgress),
					"to_state":    string(process.StateTerminated),
				},
			})
			return false
		}
	}

	if len(nextFlows) > 0 {
		return executeNextFlow(ctx, instance, nextFlows)
	}

	// Si no hay mas flows y no se completo, verificar end event
	instance.Transition(process.StateCompleted)
	return false
}

func isEndEvent(elementID string, proc *bpmn.Process) bool {
	elem, ok := proc.Elements[elementID]
	if !ok {
		return false
	}
	return elem.Type == bpmn.ElementTypeEndEvent || elem.Type == bpmn.ElementTypeTerminateEvent
}

var router = engine.NewFlowRouter()

func getAuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir("./data/audit_logs")
	if err != nil {
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	var logs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "audit_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("./data/audit_logs", entry.Name()))
		if err != nil {
			continue
		}
		logs = append(logs, fmt.Sprintf("=== %s ===\n%s", entry.Name(), string(data)))
	}
	writeJSON(w, http.StatusOK, logs)
}
