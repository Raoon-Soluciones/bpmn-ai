package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// JobHandler processes a single job. Returns nil on success, error on failure.
type JobHandler func(ctx context.Context, job *store.JobRecord) error

// WorkerPool polls for pending jobs and processes them concurrently.
type WorkerPool struct {
	store      store.Store
	handler    JobHandler
	retry      *RetryPolicy
	dlq        *DeadLetterQueue
	concurrency int
	pollInterval time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// WorkerPoolConfig holds worker pool configuration.
type WorkerPoolConfig struct {
	Concurrency  int
	PollInterval time.Duration
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(s store.Store, handler JobHandler, retry *RetryPolicy, dlq *DeadLetterQueue, cfg WorkerPoolConfig) *WorkerPool {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.PollInterval < 1 {
		cfg.PollInterval = 5 * time.Second
	}
	if retry == nil {
		retry = DefaultRetryPolicy()
	}
	return &WorkerPool{
		store:        s,
		handler:      handler,
		retry:        retry,
		dlq:          dlq,
		concurrency:  cfg.Concurrency,
		pollInterval: cfg.PollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins polling for and processing jobs.
func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.concurrency; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
}

// Stop signals all workers to shut down and waits for them to finish.
func (wp *WorkerPool) Stop() {
	close(wp.stopCh)
	wp.wg.Wait()
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	ticker := time.NewTicker(wp.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			wp.processBatch(ctx)
		}
	}
}

func (wp *WorkerPool) processBatch(ctx context.Context) {
	jobs, err := wp.store.GetPendingJobs(ctx, wp.concurrency)
	if err != nil {
		return
	}

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		default:
			wp.processJob(ctx, job)
		}
	}
}

func (wp *WorkerPool) processJob(ctx context.Context, job *store.JobRecord) {
	// Copy to avoid data race: job is a shared pointer from GetPendingJobs;
	// modifications below must not race with concurrent readers of the store's map.
	j := *job
	j.Status = store.JobStatusRunning
	now := time.Now()
	j.ExecutedAt = &now
	if err := wp.store.UpdateJob(ctx, &j); err != nil {
		return
	}

	if wp.handler == nil {
		wp.completeJob(ctx, &j)
		return
	}

	if err := wp.handler(ctx, &j); err != nil {
		wp.failJob(ctx, &j, err)
	} else {
		wp.completeJob(ctx, &j)
	}
}

func (wp *WorkerPool) completeJob(ctx context.Context, job *store.JobRecord) {
	job.Status = store.JobStatusCompleted
	now := time.Now()
	job.ExecutedAt = &now
	_ = wp.store.UpdateJob(ctx, job)
}

func (wp *WorkerPool) failJob(ctx context.Context, job *store.JobRecord, err error) {
	if wp.retry.ShouldRetry(job) {
		wp.retry.ApplyRetry(job, err.Error())
		_ = wp.store.UpdateJob(ctx, job)
		return
	}

	if wp.dlq != nil {
		_ = wp.dlq.Add(ctx, job, err.Error())
	} else {
		job.Status = store.JobStatusDead
		job.ErrorMessage = err.Error()
		_ = wp.store.UpdateJob(ctx, job)
	}
}

// Enqueue creates a new job and adds it to the queue.
func (wp *WorkerPool) Enqueue(ctx context.Context, job *store.JobRecord) error {
	if job.ID == "" {
		job.ID = fmt.Sprintf("job-%s", job.InstanceID)
	}
	if job.Status == "" {
		job.Status = store.JobStatusPending
	}
	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = time.Now()
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = wp.retry.MaxRetries
	}
	return wp.store.CreateJob(ctx, job)
}
