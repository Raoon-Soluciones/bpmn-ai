package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Raoon-Soluciones/bpmn-ai/api/http"
	"github.com/Raoon-Soluciones/bpmn-ai/config"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/activities"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/events"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/flows"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/gateways"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/engine"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store/memory"
)

func main() {
	// Load .env file if it exists (ignores error if missing)
	_ = godotenv.Load()

	cfg := config.Default()

	logger, err := observability.NewFromConfig(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	metrics := observability.DefaultMetrics()

	store := memory.NewStore()

	dispatcher := observability.NewDispatcher()
	auditWriter, err := observability.NewFileAuditWriter(cfg.Audit.Dir, cfg.Audit.Enabled, logger)
	if err != nil {
		logger.Error("failed to create audit writer", "error", err)
		os.Exit(1)
	}
	_ = observability.NewAuditor(dispatcher, auditWriter)

	retry := queue.DefaultRetryPolicy()
	dlq := queue.NewDeadLetterQueue(store, store).WithDispatcher(dispatcher)
	q := queue.NewWorkerPool(store, nil, retry, dlq, queue.WorkerPoolConfig{
		Concurrency:  cfg.Engine.WorkerCount,
		PollInterval: cfg.Engine.QueuePollInterval,
	}).WithDispatcher(dispatcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := engine.NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeTerminateEvent, events.NewTerminateEvent)
	registry.Register(bpmn.ElementTypeTimerEvent, events.NewTimerEvent)
	registry.Register(bpmn.ElementTypeMessageThrow, events.NewMessageThrowEvent)
	registry.Register(bpmn.ElementTypeMessageCatch, events.NewMessageCatchEvent)
	registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)
	registry.Register(bpmn.ElementTypeParallelGateway, gateways.NewParallelGateway)
	registry.Register(bpmn.ElementTypeInclusiveGateway, gateways.NewInclusiveGateway)
	registry.Register(bpmn.ElementTypeEventBasedGateway, gateways.NewEventBasedGateway)
	registry.Register(bpmn.ElementTypeUserTask, activities.NewUserTask)
	registry.Register(bpmn.ElementTypeScriptTask, activities.NewScriptTask)
	registry.Register(bpmn.ElementTypeServiceTask, activities.NewServiceTask)
	registry.Register(bpmn.ElementTypeSequenceFlow, flows.NewSequenceFlow)

	eng := engine.New(engine.Config{
		WorkerCount:      cfg.Engine.WorkerCount,
		MaxLoops:         cfg.Engine.MaxLoops,
		ExecutionTimeout: cfg.Engine.ExecutionTimeout,
	}, registry, store, logger, q)
	eng.WithDispatcher(dispatcher)

	q.WithHandler(eng.JobHandler())
	q.Start(ctx)

	srv := http.NewServer(http.ServerConfig{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
		TLSCertFile:  cfg.Server.TLSCertFile,
		TLSKeyFile:   cfg.Server.TLSKeyFile,
	}, store, eng, q, logger, metrics)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	q.Stop()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	if err := auditWriter.Close(); err != nil {
		logger.Error("audit writer close error", "error", err)
	}

	logger.Info("server stopped")
}
