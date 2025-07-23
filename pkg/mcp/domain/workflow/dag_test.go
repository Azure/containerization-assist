package workflow

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGBuilder(t *testing.T) {
	t.Run("creates valid DAG with sequential steps", func(t *testing.T) {
		builder := NewDAGBuilder()

		builder.
			AddStep(&DAGStep{Name: "step1"}).
			AddStep(&DAGStep{Name: "step2"}).
			AddStep(&DAGStep{Name: "step3"}).
			AddDependency("step1", "step2").
			AddDependency("step2", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)
		assert.Len(t, dag.steps, 3)
		assert.Len(t, dag.edges["step1"], 1)
		assert.Len(t, dag.edges["step2"], 1)
		assert.Equal(t, 0, dag.inDegree["step1"])
		assert.Equal(t, 1, dag.inDegree["step2"])
		assert.Equal(t, 1, dag.inDegree["step3"])
	})

	t.Run("creates valid DAG with parallel steps", func(t *testing.T) {
		builder := NewDAGBuilder()

		builder.
			AddStep(&DAGStep{Name: "step1"}).
			AddStep(&DAGStep{Name: "step2a"}).
			AddStep(&DAGStep{Name: "step2b"}).
			AddStep(&DAGStep{Name: "step3"}).
			AddDependency("step1", "step2a").
			AddDependency("step1", "step2b").
			AddDependency("step2a", "step3").
			AddDependency("step2b", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)
		assert.Len(t, dag.steps, 4)
		assert.Equal(t, 0, dag.inDegree["step1"])
		assert.Equal(t, 1, dag.inDegree["step2a"])
		assert.Equal(t, 1, dag.inDegree["step2b"])
		assert.Equal(t, 2, dag.inDegree["step3"])
	})

	t.Run("detects cycles in DAG", func(t *testing.T) {
		builder := NewDAGBuilder()

		builder.
			AddStep(&DAGStep{Name: "step1"}).
			AddStep(&DAGStep{Name: "step2"}).
			AddStep(&DAGStep{Name: "step3"}).
			AddDependency("step1", "step2").
			AddDependency("step2", "step3").
			AddDependency("step3", "step1") // Creates cycle

		_, err := builder.Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DAG contains cycles")
	})

	t.Run("handles self-cycles", func(t *testing.T) {
		builder := NewDAGBuilder()

		builder.
			AddStep(&DAGStep{Name: "step1"}).
			AddDependency("step1", "step1") // Self cycle

		_, err := builder.Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DAG contains cycles")
	})
}

func TestTopologicalSort(t *testing.T) {
	t.Run("sorts sequential workflow correctly", func(t *testing.T) {
		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{Name: "analyze"}).
			AddStep(&DAGStep{Name: "build"}).
			AddStep(&DAGStep{Name: "deploy"}).
			AddDependency("analyze", "build").
			AddDependency("build", "deploy")

		dag, err := builder.Build()
		require.NoError(t, err)

		sorted, err := dag.TopologicalSort()
		require.NoError(t, err)
		assert.Equal(t, []string{"analyze", "build", "deploy"}, sorted)
	})

	t.Run("sorts parallel workflow correctly", func(t *testing.T) {
		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{Name: "start"}).
			AddStep(&DAGStep{Name: "parallel1"}).
			AddStep(&DAGStep{Name: "parallel2"}).
			AddStep(&DAGStep{Name: "end"}).
			AddDependency("start", "parallel1").
			AddDependency("start", "parallel2").
			AddDependency("parallel1", "end").
			AddDependency("parallel2", "end")

		dag, err := builder.Build()
		require.NoError(t, err)

		sorted, err := dag.TopologicalSort()
		require.NoError(t, err)

		// Check that start comes first and end comes last
		assert.Equal(t, "start", sorted[0])
		assert.Equal(t, "end", sorted[3])

		// parallel1 and parallel2 can be in any order between start and end
		middle := sorted[1:3]
		assert.Contains(t, middle, "parallel1")
		assert.Contains(t, middle, "parallel2")
	})
}

func TestGetParallelizableSteps(t *testing.T) {
	t.Run("identifies parallel levels correctly", func(t *testing.T) {
		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{Name: "step1"}).
			AddStep(&DAGStep{Name: "step2a"}).
			AddStep(&DAGStep{Name: "step2b"}).
			AddStep(&DAGStep{Name: "step3"}).
			AddDependency("step1", "step2a").
			AddDependency("step1", "step2b").
			AddDependency("step2a", "step3").
			AddDependency("step2b", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)

		levels, err := dag.GetParallelizableSteps()
		require.NoError(t, err)
		require.Len(t, levels, 3)

		// First level: step1
		assert.Equal(t, []string{"step1"}, levels[0])

		// Second level: step2a and step2b (order doesn't matter)
		assert.Len(t, levels[1], 2)
		assert.Contains(t, levels[1], "step2a")
		assert.Contains(t, levels[1], "step2b")

		// Third level: step3
		assert.Equal(t, []string{"step3"}, levels[2])
	})

	t.Run("handles independent steps", func(t *testing.T) {
		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{Name: "independent1"}).
			AddStep(&DAGStep{Name: "independent2"}).
			AddStep(&DAGStep{Name: "independent3"})

		dag, err := builder.Build()
		require.NoError(t, err)

		levels, err := dag.GetParallelizableSteps()
		require.NoError(t, err)
		require.Len(t, levels, 1)

		// All steps should be in the same level
		assert.Len(t, levels[0], 3)
		assert.Contains(t, levels[0], "independent1")
		assert.Contains(t, levels[0], "independent2")
		assert.Contains(t, levels[0], "independent3")
	})
}

func TestDAGExecution(t *testing.T) {
	t.Run("executes sequential workflow", func(t *testing.T) {
		executionOrder := make([]string, 0)
		var mu sync.Mutex

		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{
				Name: "step1",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step1")
					mu.Unlock()
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step2",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step2")
					mu.Unlock()
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step3",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step3")
					mu.Unlock()
					return nil
				},
			}).
			AddDependency("step1", "step2").
			AddDependency("step2", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.NoError(t, err)

		assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder)
	})

	t.Run("executes parallel steps concurrently", func(t *testing.T) {
		var step2aStarted, step2bStarted atomic.Bool
		var wg sync.WaitGroup
		wg.Add(2)

		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{
				Name: "step1",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step2a",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					step2aStarted.Store(true)
					wg.Done()
					wg.Wait() // Wait for both parallel steps to start
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step2b",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					step2bStarted.Store(true)
					wg.Done()
					wg.Wait() // Wait for both parallel steps to start
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step3",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					// Verify both parallel steps completed
					assert.True(t, step2aStarted.Load())
					assert.True(t, step2bStarted.Load())
					return nil
				},
			}).
			AddDependency("step1", "step2a").
			AddDependency("step1", "step2b").
			AddDependency("step2a", "step3").
			AddDependency("step2b", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.NoError(t, err)
	})

	t.Run("handles step failures", func(t *testing.T) {
		executionOrder := make([]string, 0)
		var mu sync.Mutex

		builder := NewDAGBuilder()
		builder.
			AddStep(&DAGStep{
				Name: "step1",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step1")
					mu.Unlock()
					return nil
				},
			}).
			AddStep(&DAGStep{
				Name: "step2",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step2")
					mu.Unlock()
					return errors.New("step2 failed")
				},
			}).
			AddStep(&DAGStep{
				Name: "step3",
				Execute: func(ctx context.Context, state *WorkflowState) error {
					mu.Lock()
					executionOrder = append(executionOrder, "step3")
					mu.Unlock()
					return nil
				},
			}).
			AddDependency("step1", "step2").
			AddDependency("step2", "step3")

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Step 'step2' failed")

		// step3 should not have executed
		assert.Equal(t, []string{"step1", "step2"}, executionOrder)
	})

	t.Run("respects timeout", func(t *testing.T) {
		builder := NewDAGBuilder()
		builder.AddStep(&DAGStep{
			Name: "slow-step",
			Execute: func(ctx context.Context, state *WorkflowState) error {
				select {
				case <-time.After(1 * time.Second):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			Timeout: 100 * time.Millisecond,
		})

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Step 'slow-step' failed")
	})

	t.Run("applies retry logic", func(t *testing.T) {
		attemptCount := 0
		var mu sync.Mutex

		builder := NewDAGBuilder()
		builder.AddStep(&DAGStep{
			Name: "flaky-step",
			Execute: func(ctx context.Context, state *WorkflowState) error {
				mu.Lock()
				attemptCount++
				count := attemptCount
				mu.Unlock()

				if count < 3 {
					return errors.New("temporary failure")
				}
				return nil
			},
			Retry: DAGRetryPolicy{
				MaxAttempts: 3,
				BackoffBase: 10 * time.Millisecond,
			},
		})

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, 3, attemptCount)
	})

	t.Run("fails after max retries", func(t *testing.T) {
		attemptCount := 0
		var mu sync.Mutex

		builder := NewDAGBuilder()
		builder.AddStep(&DAGStep{
			Name: "always-fails",
			Execute: func(ctx context.Context, state *WorkflowState) error {
				mu.Lock()
				attemptCount++
				mu.Unlock()
				return errors.New("persistent failure")
			},
			Retry: DAGRetryPolicy{
				MaxAttempts: 2,
				BackoffBase: 10 * time.Millisecond,
			},
		})

		dag, err := builder.Build()
		require.NoError(t, err)

		state := &WorkflowState{}
		err = dag.Execute(context.Background(), state)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Step 'always-fails' failed")
		assert.Equal(t, 2, attemptCount)
	})
}
