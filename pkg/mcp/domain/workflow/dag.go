// Package workflow provides a DAG-based workflow execution engine
package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// DAGStep represents a single step in the DAG workflow
type DAGStep struct {
	Name    string
	Execute func(context.Context, *WorkflowState) error
	Retry   DAGRetryPolicy
	Timeout time.Duration
}

// DAGRetryPolicy defines retry behavior for a step
type DAGRetryPolicy struct {
	MaxAttempts int
	BackoffBase time.Duration
	BackoffMax  time.Duration
}

// DAGWorkflow represents a workflow as a directed acyclic graph
type DAGWorkflow struct {
	steps    map[string]*DAGStep
	edges    map[string][]string // from -> []to
	inDegree map[string]int      // tracks dependencies
	mu       sync.RWMutex
}

// DAGBuilder provides a fluent interface for building DAG workflows
type DAGBuilder struct {
	workflow *DAGWorkflow
}

// NewDAGBuilder creates a new DAG workflow builder
func NewDAGBuilder() *DAGBuilder {
	return &DAGBuilder{
		workflow: &DAGWorkflow{
			steps:    make(map[string]*DAGStep),
			edges:    make(map[string][]string),
			inDegree: make(map[string]int),
		},
	}
}

// AddStep adds a workflow step to the DAG
func (b *DAGBuilder) AddStep(step *DAGStep) *DAGBuilder {
	b.workflow.steps[step.Name] = step
	// Initialize in-degree if not already present
	if _, exists := b.workflow.inDegree[step.Name]; !exists {
		b.workflow.inDegree[step.Name] = 0
	}
	return b
}

// AddDependency creates a dependency edge from one step to another
func (b *DAGBuilder) AddDependency(from, to string) *DAGBuilder {
	b.workflow.edges[from] = append(b.workflow.edges[from], to)
	b.workflow.inDegree[to]++
	// Ensure both steps are in the inDegree map
	if _, exists := b.workflow.inDegree[from]; !exists {
		b.workflow.inDegree[from] = 0
	}
	return b
}

// Build finalizes and returns the DAG workflow
func (b *DAGBuilder) Build() (*DAGWorkflow, error) {
	// Validate DAG has no cycles
	if err := b.workflow.validateNoCycles(); err != nil {
		return nil, err
	}
	return b.workflow, nil
}

// validateNoCycles ensures the graph is acyclic
func (dag *DAGWorkflow) validateNoCycles() error {
	// Create a copy of inDegree for manipulation
	inDegreeCopy := make(map[string]int)
	for k, v := range dag.inDegree {
		inDegreeCopy[k] = v
	}

	// Queue for steps with no dependencies
	queue := make([]string, 0)
	for step, degree := range inDegreeCopy {
		if degree == 0 {
			queue = append(queue, step)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++

		// Process all dependent steps
		for _, dependent := range dag.edges[current] {
			inDegreeCopy[dependent]--
			if inDegreeCopy[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if processed != len(dag.steps) {
		return errors.NewWorkflowError(
			errors.CodeInvalidParameter,
			"workflow",
			"dag_validation",
			"DAG contains cycles",
			nil,
		)
	}

	return nil
}

// TopologicalSort returns steps in execution order
func (dag *DAGWorkflow) TopologicalSort() ([]string, error) {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	// Create a copy of inDegree for manipulation
	inDegreeCopy := make(map[string]int)
	for k, v := range dag.inDegree {
		inDegreeCopy[k] = v
	}

	// Queue for steps with no dependencies
	queue := make([]string, 0)
	for step, degree := range inDegreeCopy {
		if degree == 0 {
			queue = append(queue, step)
		}
	}

	result := make([]string, 0, len(dag.steps))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Process all dependent steps
		for _, dependent := range dag.edges[current] {
			inDegreeCopy[dependent]--
			if inDegreeCopy[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(dag.steps) {
		return nil, errors.NewWorkflowError(
			errors.CodeInvalidState,
			"workflow",
			"topological_sort",
			"Unable to sort DAG - possible cycle detected",
			nil,
		)
	}

	return result, nil
}

// GetParallelizableSteps returns groups of steps that can be executed in parallel
func (dag *DAGWorkflow) GetParallelizableSteps() ([][]string, error) {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	// Create a copy of inDegree for manipulation
	inDegreeCopy := make(map[string]int)
	for k, v := range dag.inDegree {
		inDegreeCopy[k] = v
	}

	levels := make([][]string, 0)
	remaining := len(dag.steps)

	for remaining > 0 {
		// Find all steps with in-degree 0
		currentLevel := make([]string, 0)
		for step, degree := range inDegreeCopy {
			if degree == 0 {
				currentLevel = append(currentLevel, step)
			}
		}

		if len(currentLevel) == 0 {
			return nil, errors.NewWorkflowError(
				errors.CodeInvalidState,
				"workflow",
				"parallel_analysis",
				"No steps available for execution - possible cycle",
				nil,
			)
		}

		// Remove processed steps and update dependencies
		for _, step := range currentLevel {
			delete(inDegreeCopy, step)
			remaining--

			// Update in-degree for dependent steps
			for _, dependent := range dag.edges[step] {
				if _, exists := inDegreeCopy[dependent]; exists {
					inDegreeCopy[dependent]--
				}
			}
		}

		levels = append(levels, currentLevel)
	}

	return levels, nil
}

// Execute runs the DAG workflow with the given state
func (dag *DAGWorkflow) Execute(ctx context.Context, state *WorkflowState) error {
	// Get parallel execution levels
	levels, err := dag.GetParallelizableSteps()
	if err != nil {
		return err
	}

	// Execute each level
	for levelIdx, level := range levels {
		// Execute all steps in this level in parallel
		if err := dag.executeLevel(ctx, level, state, levelIdx); err != nil {
			return err
		}
	}

	return nil
}

// executeLevel executes all steps in a level in parallel
func (dag *DAGWorkflow) executeLevel(ctx context.Context, stepNames []string, state *WorkflowState, levelIdx int) error {
	if len(stepNames) == 1 {
		// Single step, execute directly
		return dag.executeStep(ctx, stepNames[0], state)
	}

	// Multiple steps, execute in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(stepNames))

	for _, stepName := range stepNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if err := dag.executeStep(ctx, name, state); err != nil {
				errChan <- fmt.Errorf("step %s failed: %w", name, err)
			}
		}(stepName)
	}

	// Wait for all steps to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.NewWorkflowError(
			errors.CodeInternal,
			"workflow",
			"parallel_execution",
			fmt.Sprintf("Level %d execution failed with %d errors", levelIdx, len(errs)),
			errs[0], // Use first error as cause
		).
			WithStepContext("level", levelIdx).
			WithStepContext("errors", errs)
	}

	return nil
}

// executeStep executes a single step with retry logic
func (dag *DAGWorkflow) executeStep(ctx context.Context, stepName string, state *WorkflowState) error {
	step, exists := dag.steps[stepName]
	if !exists {
		return errors.NewWorkflowError(
			errors.CodeNotFound,
			"workflow",
			"step_execution",
			fmt.Sprintf("Step '%s' not found in DAG", stepName),
			nil,
		)
	}

	// Apply timeout if specified
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Execute with retry logic
	var lastErr error
	maxAttempts := 1
	if step.Retry.MaxAttempts > 0 {
		maxAttempts = step.Retry.MaxAttempts
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := step.Execute(ctx, state)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on final attempt or context cancellation
		if attempt == maxAttempts || ctx.Err() != nil {
			break
		}

		// Calculate backoff
		backoff := step.Retry.BackoffBase * time.Duration(attempt)
		if step.Retry.BackoffMax > 0 && backoff > step.Retry.BackoffMax {
			backoff = step.Retry.BackoffMax
		}

		// Wait before retry
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errors.NewWorkflowError(
		errors.CodeInternal,
		"workflow",
		"step_execution",
		fmt.Sprintf("Step '%s' failed after %d attempts", stepName, maxAttempts),
		lastErr,
	).WithStepContext("step", stepName).
		WithStepContext("attempts", maxAttempts)
}
