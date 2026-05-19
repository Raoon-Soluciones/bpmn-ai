package engine

import (
	"fmt"
	"time"
)

// FailSafeManager monitors execution for timeouts and infinite loops.
type FailSafeManager struct {
	startTime      time.Time
	maxTimeout     time.Duration
	maxLoops       int
	elementCounts  map[string]int // elementID -> execution count
	totalExecutions int
}

// NewFailSafeManager creates a new fail-safe manager.
func NewFailSafeManager(maxTimeout time.Duration, maxLoops int) *FailSafeManager {
	return &FailSafeManager{
		startTime:     time.Now(),
		maxTimeout:    maxTimeout,
		maxLoops:      maxLoops,
		elementCounts: make(map[string]int),
	}
}

// Check performs all fail-safe checks. Returns an error if any check fails.
func (f *FailSafeManager) Check(elementID string) error {
	if err := f.checkTimeout(); err != nil {
		return err
	}
	if err := f.checkLoopCount(elementID); err != nil {
		return err
	}
	return nil
}

// checkTimeout verifies that execution hasn't exceeded the maximum time.
func (f *FailSafeManager) checkTimeout() error {
	elapsed := time.Since(f.startTime)
	if elapsed > f.maxTimeout {
		return &ExecutionTimeoutError{
			Elapsed: elapsed,
			Limit:   f.maxTimeout,
		}
	}
	return nil
}

// checkLoopCount verifies that an element hasn't been executed too many times.
func (f *FailSafeManager) checkLoopCount(elementID string) error {
	if elementID == "" {
		return nil
	}

	f.elementCounts[elementID]++
	count := f.elementCounts[elementID]
	f.totalExecutions++

	if count > f.maxLoops {
		return &NestedLoopError{
			ElementID: elementID,
			Count:     count,
			Limit:     f.maxLoops,
		}
	}

	return nil
}

// Elapsed returns the total execution time.
func (f *FailSafeManager) Elapsed() time.Duration {
	return time.Since(f.startTime)
}

// TotalExecutions returns the total number of element executions.
func (f *FailSafeManager) TotalExecutions() int {
	return f.totalExecutions
}

// ExecutionTimeoutError is returned when execution exceeds the time limit.
type ExecutionTimeoutError struct {
	Elapsed time.Duration
	Limit   time.Duration
}

func (e *ExecutionTimeoutError) Error() string {
	return fmt.Sprintf("execution timeout: elapsed %v, limit %v", e.Elapsed, e.Limit)
}

// NestedLoopError is returned when an element is executed too many times.
type NestedLoopError struct {
	ElementID string
	Count     int
	Limit     int
}

func (e *NestedLoopError) Error() string {
	return fmt.Sprintf("nested loop limit reached: element %s executed %d times (limit %d)", e.ElementID, e.Count, e.Limit)
}
