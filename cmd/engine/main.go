package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/organization/bpmn-engine/api/http"
	"github.com/organization/bpmn-engine/config"
	"github.com/organization/bpmn-engine/internal/observability"
	"github.com/organization/bpmn-engine/internal/queue"
	"github.com/organization/bpmn-engine/pkg/store/memory"
)

func main() {
	cfg := config.Default()

	logger, err := observability.NewFromConfig(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	metrics := observability.DefaultMetrics()

	store := memory.NewStore()

	retry := queue.DefaultRetryPolicy()
	dlq := queue.NewDeadLetterQueue(store)
	q := queue.NewWorkerPool(store, nil, retry, dlq, queue.WorkerPoolConfig{
		Concurrency:  cfg.Engine.WorkerCount,
		PollInterval: cfg.Engine.QueuePollInterval,
	})

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

	logger.Info("server stopped")
}
