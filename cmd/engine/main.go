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
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
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
	auditWriter, err := observability.NewFileAuditWriter(cfg.Audit.FilePath, cfg.Audit.Enabled, logger)
	if err != nil {
		logger.Error("failed to create audit writer", "error", err)
		os.Exit(1)
	}
	_ = observability.NewAuditor(dispatcher, auditWriter)

	retry := queue.DefaultRetryPolicy()
	dlq := queue.NewDeadLetterQueue(store).WithDispatcher(dispatcher)
	q := queue.NewWorkerPool(store, nil, retry, dlq, queue.WorkerPoolConfig{
		Concurrency:  cfg.Engine.WorkerCount,
		PollInterval: cfg.Engine.QueuePollInterval,
	}).WithDispatcher(dispatcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)

	srv := http.NewServer(http.ServerConfig{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}, store, q, logger, metrics)

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
