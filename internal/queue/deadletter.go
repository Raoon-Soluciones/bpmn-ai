package queue

import (
	"context"

	"github.com/organization/bpmn-engine/pkg/store"
)

// DeadLetterQueue stores jobs that exceeded all retry attempts.
type DeadLetterQueue struct {
	store store.Store
}

// NewDeadLetterQueue creates a new dead letter queue.
func NewDeadLetterQueue(s store.Store) *DeadLetterQueue {
	return &DeadLetterQueue{store: s}
}

// Add moves a failed job to the dead letter queue.
func (dlq *DeadLetterQueue) Add(ctx context.Context, job *store.JobRecord, errMsg string) error {
	record := &store.DeadLetterRecord{
		JobID:        job.ID,
		InstanceID:   job.InstanceID,
		Type:         job.Type,
		Payload:      job.Payload,
		ErrorMessage: errMsg,
		RetryCount:   job.RetryCount,
	}
	if err := dlq.store.CreateDeadLetter(ctx, record); err != nil {
		return err
	}
	job.Status = store.JobStatusDead
	job.ErrorMessage = errMsg
	return dlq.store.UpdateJob(ctx, job)
}

// List returns all dead letters for an instance.
func (dlq *DeadLetterQueue) List(ctx context.Context, instanceID string) ([]*store.DeadLetterRecord, error) {
	return dlq.store.GetDeadLetters(ctx, instanceID)
}

// Get returns a single dead letter by ID.
func (dlq *DeadLetterQueue) Get(ctx context.Context, id string) (*store.DeadLetterRecord, error) {
	return dlq.store.GetDeadLetter(ctx, id)
}

// ListAll returns all dead letters with a limit.
func (dlq *DeadLetterQueue) ListAll(ctx context.Context, limit int) ([]*store.DeadLetterRecord, error) {
	return dlq.store.ListDeadLetters(ctx, limit)
}

// Count returns the total number of dead letters for an instance.
func (dlq *DeadLetterQueue) Count(ctx context.Context, instanceID string) (int, error) {
	dls, err := dlq.store.GetDeadLetters(ctx, instanceID)
	if err != nil {
		return 0, err
	}
	return len(dls), nil
}
