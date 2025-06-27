package state

import (
	"context"
	"fmt"
)

// StateOperation represents a single state operation in a transaction
type StateOperation struct {
	Type      OperationType
	StateType StateType
	StateID   string
	State     interface{}
	Validator func(interface{}) error
}

// OperationType represents the type of state operation
type OperationType string

const (
	OperationSet    OperationType = "set"
	OperationDelete OperationType = "delete"
)

// StateTransaction provides atomic state updates across multiple state types
type StateTransaction struct {
	manager    *UnifiedStateManager
	operations []StateOperation
	ctx        context.Context
	committed  bool
}

// Set adds a set operation to the transaction
func (t *StateTransaction) Set(stateType StateType, id string, state interface{}) *StateTransaction {
	t.operations = append(t.operations, StateOperation{
		Type:      OperationSet,
		StateType: stateType,
		StateID:   id,
		State:     state,
	})
	return t
}

// SetWithValidation adds a set operation with custom validation
func (t *StateTransaction) SetWithValidation(stateType StateType, id string, state interface{}, validator func(interface{}) error) *StateTransaction {
	t.operations = append(t.operations, StateOperation{
		Type:      OperationSet,
		StateType: stateType,
		StateID:   id,
		State:     state,
		Validator: validator,
	})
	return t
}

// Delete adds a delete operation to the transaction
func (t *StateTransaction) Delete(stateType StateType, id string) *StateTransaction {
	t.operations = append(t.operations, StateOperation{
		Type:      OperationDelete,
		StateType: stateType,
		StateID:   id,
	})
	return t
}

// Commit executes all operations in the transaction atomically
func (t *StateTransaction) Commit() error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	// Validate all operations first
	for _, op := range t.operations {
		if op.Validator != nil {
			if err := op.Validator(op.State); err != nil {
				return fmt.Errorf("validation failed for %s/%s: %w", op.StateType, op.StateID, err)
			}
		}
	}

	// Create rollback information
	rollback := make([]func() error, 0, len(t.operations))
	completed := 0

	// Execute operations
	for i, op := range t.operations {
		switch op.Type {
		case OperationSet:
			// Get current state for rollback
			oldState, _ := t.manager.GetState(t.ctx, op.StateType, op.StateID)

			// Execute operation
			if err := t.manager.SetState(t.ctx, op.StateType, op.StateID, op.State); err != nil {
				// Rollback completed operations
				t.rollback(rollback[:completed])
				return fmt.Errorf("operation %d failed: %w", i, err)
			}

			// Add rollback function
			rollback = append(rollback, func() error {
				if oldState != nil {
					return t.manager.SetState(t.ctx, op.StateType, op.StateID, oldState)
				}
				return t.manager.DeleteState(t.ctx, op.StateType, op.StateID)
			})

		case OperationDelete:
			// Get current state for rollback
			oldState, _ := t.manager.GetState(t.ctx, op.StateType, op.StateID)

			// Execute operation
			if err := t.manager.DeleteState(t.ctx, op.StateType, op.StateID); err != nil {
				// Rollback completed operations
				t.rollback(rollback[:completed])
				return fmt.Errorf("operation %d failed: %w", i, err)
			}

			// Add rollback function if state existed
			if oldState != nil {
				rollback = append(rollback, func() error {
					return t.manager.SetState(t.ctx, op.StateType, op.StateID, oldState)
				})
			}
		}

		completed++
	}

	t.committed = true
	return nil
}

// rollback executes rollback functions in reverse order
func (t *StateTransaction) rollback(rollbackFuncs []func() error) {
	for i := len(rollbackFuncs) - 1; i >= 0; i-- {
		if err := rollbackFuncs[i](); err != nil {
			t.manager.logger.Error().Err(err).Int("operation", i).Msg("Rollback operation failed")
		}
	}
}
