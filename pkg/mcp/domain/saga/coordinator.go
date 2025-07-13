// Package saga provides the saga coordinator for managing distributed transactions.
package saga

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
)

// SagaCoordinator manages multiple saga transactions
type SagaCoordinator struct {
	executions map[string]*SagaExecution
	mutex      sync.RWMutex
	logger     *slog.Logger
	publisher  *events.Publisher
}

// NewSagaCoordinator creates a new saga coordinator
func NewSagaCoordinator(logger *slog.Logger, publisher *events.Publisher) *SagaCoordinator {
	return &SagaCoordinator{
		executions: make(map[string]*SagaExecution),
		logger:     logger.With("component", "saga_coordinator"),
		publisher:  publisher,
	}
}

// StartSaga creates and starts a new saga transaction
func (c *SagaCoordinator) StartSaga(ctx context.Context, sagaID, workflowID string, steps []SagaStep) (*SagaExecution, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if saga already exists
	if _, exists := c.executions[sagaID]; exists {
		return nil, errors.New(errors.CodeAlreadyExists, "saga", "saga already exists", nil).
			With("saga_id", sagaID).
			With("workflow_id", workflowID)
	}

	execution := NewSagaExecution(sagaID, workflowID, steps, c.logger)
	c.executions[sagaID] = execution

	c.logger.Info("Starting new saga transaction",
		"saga_id", sagaID,
		"workflow_id", workflowID,
		"steps_count", len(steps))

	// Publish saga started event
	if c.publisher != nil {
		event := events.SagaStartedEvent{
			ID:         generateEventID(),
			Timestamp:  time.Now(),
			Workflow:   workflowID,
			SagaID:     sagaID,
			StepsCount: len(steps),
		}
		c.publisher.PublishAsync(ctx, event)
	}

	// Execute saga in a goroutine to avoid blocking
	go c.executeSaga(ctx, execution)

	return execution, nil
}

// executeSaga executes a saga and publishes completion events
func (c *SagaCoordinator) executeSaga(ctx context.Context, execution *SagaExecution) {
	err := execution.Execute(ctx)

	// Publish saga completion event
	if c.publisher != nil {
		var event events.DomainEvent

		if err == nil {
			event = events.SagaCompletedEvent{
				ID:            generateEventID(),
				Timestamp:     time.Now(),
				Workflow:      execution.WorkflowID,
				SagaID:        execution.ID,
				Success:       true,
				Duration:      execution.GetDuration(),
				StepsExecuted: len(execution.ExecutedSteps),
			}
		} else {
			event = events.SagaCompletedEvent{
				ID:               generateEventID(),
				Timestamp:        time.Now(),
				Workflow:         execution.WorkflowID,
				SagaID:           execution.ID,
				Success:          false,
				Duration:         execution.GetDuration(),
				StepsExecuted:    len(execution.ExecutedSteps),
				StepsCompensated: len(execution.CompensatedSteps),
				ErrorMsg:         err.Error(),
			}
		}

		c.publisher.PublishAsync(ctx, event)
	}

	c.logger.Info("Saga execution finished",
		"saga_id", execution.ID,
		"workflow_id", execution.WorkflowID,
		"success", err == nil,
		"duration", execution.GetDuration())
}

// GetSaga retrieves a saga execution by ID
func (c *SagaCoordinator) GetSaga(sagaID string) (*SagaExecution, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	execution, exists := c.executions[sagaID]
	if !exists {
		return nil, errors.New(errors.CodeNotFound, "saga", "saga not found", nil).
			With("saga_id", sagaID)
	}

	return execution, nil
}

// ListSagas returns all saga executions for a workflow
func (c *SagaCoordinator) ListSagas(workflowID string) []*SagaExecution {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var sagas []*SagaExecution
	for _, execution := range c.executions {
		if workflowID == "" || execution.WorkflowID == workflowID {
			sagas = append(sagas, execution)
		}
	}

	return sagas
}

// CancelSaga attempts to cancel a running saga
func (c *SagaCoordinator) CancelSaga(ctx context.Context, sagaID string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	execution, exists := c.executions[sagaID]
	if !exists {
		return errors.New(errors.CodeNotFound, "saga", "saga not found for cancellation", nil).
			With("saga_id", sagaID)
	}

	// Only cancel if saga is still in progress
	if execution.State != SagaStateInProgress {
		return errors.New(errors.CodeInvalidState, "saga", "saga cannot be cancelled in current state", nil).
			With("saga_id", sagaID).
			With("current_state", string(execution.State))
	}

	c.logger.Info("Cancelling saga", "saga_id", sagaID, "workflow_id", execution.WorkflowID)

	// Start compensation immediately
	if err := execution.compensate(ctx); err != nil {
		execution.State = SagaStateAborted
		execution.Error = fmt.Sprintf("cancellation compensation failed: %v", err)

		return errors.New(errors.CodeOperationFailed, "saga", "saga cancellation failed", err).
			With("saga_id", sagaID)
	}

	execution.State = SagaStateCompensated
	endTime := time.Now()
	execution.EndTime = &endTime

	// Publish saga cancelled event
	if c.publisher != nil {
		event := events.SagaCancelledEvent{
			ID:               generateEventID(),
			Timestamp:        time.Now(),
			Workflow:         execution.WorkflowID,
			SagaID:           sagaID,
			StepsCompensated: len(execution.CompensatedSteps),
			Duration:         execution.GetDuration(),
		}
		c.publisher.PublishAsync(ctx, event)
	}

	c.logger.Info("Saga cancelled successfully", "saga_id", sagaID)
	return nil
}

// CleanupCompletedSagas removes completed sagas older than the specified duration
func (c *SagaCoordinator) CleanupCompletedSagas(maxAge time.Duration) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for sagaID, execution := range c.executions {
		// Only clean up completed, failed, or compensated sagas
		if (execution.State == SagaStateCompleted || execution.State == SagaStateFailed || execution.State == SagaStateCompensated) &&
			execution.StartTime.Before(cutoff) {
			delete(c.executions, sagaID)
			cleaned++
		}
	}

	if cleaned > 0 {
		c.logger.Info("Cleaned up completed sagas", "count", cleaned, "max_age", maxAge)
	}

	return cleaned
}

// GetStats returns statistics about saga executions
func (c *SagaCoordinator) GetStats() SagaStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := SagaStats{}

	for _, execution := range c.executions {
		stats.Total++

		switch execution.State {
		case SagaStateStarted:
			stats.Started++
		case SagaStateInProgress:
			stats.InProgress++
		case SagaStateCompleted:
			stats.Completed++
		case SagaStateFailed:
			stats.Failed++
		case SagaStateCompensated:
			stats.Compensated++
		case SagaStateAborted:
			stats.Aborted++
		}
	}

	return stats
}

// SagaStats represents statistics about saga executions
type SagaStats struct {
	Total       int `json:"total"`
	Started     int `json:"started"`
	InProgress  int `json:"in_progress"`
	Completed   int `json:"completed"`
	Failed      int `json:"failed"`
	Compensated int `json:"compensated"`
	Aborted     int `json:"aborted"`
}

// generateEventID creates a simple event ID for saga events
func generateEventID() string {
	return fmt.Sprintf("saga-%d", time.Now().UnixNano())
}
