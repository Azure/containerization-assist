package orchestration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// BenchmarkSequentialExecution tests performance of sequential stage execution
func BenchmarkSequentialExecution(b *testing.B) {
	executor := createBenchmarkExecutor()

	// Create workflow with 5 sequential stages
	workflowSpec := createBenchmarkWorkflow(5, false)
	session := createBenchmarkSession()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteStageGroup(
			context.Background(),
			workflowSpec.Spec.Stages,
			session,
			workflowSpec,
			false, // Sequential execution
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelExecution tests performance of parallel stage execution
func BenchmarkParallelExecution(b *testing.B) {
	executor := createBenchmarkExecutor()

	// Create workflow with 5 parallel stages
	workflowSpec := createBenchmarkWorkflow(5, true)
	session := createBenchmarkSession()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteStageGroup(
			context.Background(),
			workflowSpec.Spec.Stages,
			session,
			workflowSpec,
			true, // Parallel execution
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelExecutionScaling tests performance scaling with different numbers of stages
func BenchmarkParallelExecutionScaling(b *testing.B) {
	stageCounts := []int{1, 2, 4, 8, 16, 32}

	for _, stageCount := range stageCounts {
		b.Run(fmt.Sprintf("stages-%d", stageCount), func(b *testing.B) {
			executor := createBenchmarkExecutor()
			workflowSpec := createBenchmarkWorkflow(stageCount, true)
			session := createBenchmarkSession()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := executor.ExecuteStageGroup(
					context.Background(),
					workflowSpec.Spec.Stages,
					session,
					workflowSpec,
					true,
				)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkConcurrencyLimits tests performance with different concurrency limits
func BenchmarkConcurrencyLimits(b *testing.B) {
	concurrencyLimits := []int{1, 2, 4, 8, 16}

	for _, limit := range concurrencyLimits {
		b.Run(fmt.Sprintf("limit-%d", limit), func(b *testing.B) {
			executor := createBenchmarkExecutor()
			workflowSpec := createBenchmarkWorkflow(16, true)
			workflowSpec.Spec.ConcurrencyConfig = &ConcurrencyConfig{
				MaxParallelStages: limit,
				StageTimeout:      5 * time.Minute,
				QueueSize:         100,
				WorkerPoolSize:    limit,
			}
			session := createBenchmarkSession()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := executor.ExecuteStageGroup(
					context.Background(),
					workflowSpec.Spec.Stages,
					session,
					workflowSpec,
					true,
				)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDependencyResolution tests performance of dependency resolution
func BenchmarkDependencyResolution(b *testing.B) {
	resolver := NewDependencyResolver()
	stages := createComplexDependencyGraph(20) // 20 stages with dependencies

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveDependencies(stages)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVariableExpansion tests performance of variable expansion
func BenchmarkVariableExpansion(b *testing.B) {
	resolver := NewVariableResolver(zerolog.Nop())

	// Create complex variable context
	context := &VariableContext{
		WorkflowVars: map[string]string{
			"registry":    "myregistry.azurecr.io",
			"namespace":   "production",
			"version":     "v1.2.3",
			"environment": "prod",
		},
		StageVars: map[string]string{
			"build_args": "--no-cache --progress=plain",
			"scan_mode":  "comprehensive",
		},
		SessionContext: map[string]interface{}{
			"session_id":   "test-session-123",
			"workflow_id":  "workflow-456",
			"current_time": time.Now(),
		},
		EnvironmentVars: map[string]string{
			"CI":                 "true",
			"BUILD_NUMBER":       "123",
			"CONTAINER_REGISTRY": "registry.example.com",
		},
	}

	// Complex template with multiple variable references
	template := "${registry}/${namespace}/app:${version}-${environment} --build-arg VERSION=${version} --build-arg BUILD=${BUILD_NUMBER} --scan=${scan_mode} --session=${session_id}"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveVariables(template, context)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCheckpointCreation tests performance of checkpoint creation
func BenchmarkCheckpointCreation(b *testing.B) {
	logger := zerolog.Nop()
	sessionManager := &MockSessionManager{}
	checkpointManager := NewCheckpointManager(logger, sessionManager)

	session := createBenchmarkSession()
	workflowSpec := createBenchmarkWorkflow(5, true)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := checkpointManager.CreateCheckpoint(
			session,
			"benchmark-stage",
			"Benchmark checkpoint",
			workflowSpec,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions for benchmarks

func createBenchmarkExecutor() *Executor {
	logger := zerolog.Nop()
	stageExecutor := &MockStageExecutor{}
	errorRouter := &MockErrorRouter{}
	sessionManager := &MockSessionManager{}
	stateMachine := NewStateMachine(logger, sessionManager)

	return NewExecutor(logger, stageExecutor, errorRouter, stateMachine)
}

func createBenchmarkWorkflow(stageCount int, parallel bool) *WorkflowSpec {
	stages := make([]WorkflowStage, stageCount)

	for i := 0; i < stageCount; i++ {
		stages[i] = WorkflowStage{
			Name:     fmt.Sprintf("stage-%d", i),
			Type:     "benchmark",
			Tools:    []string{"mock-tool"},
			Parallel: parallel,
			Variables: map[string]string{
				"stage_index": fmt.Sprintf("%d", i),
			},
		}
	}

	return &WorkflowSpec{
		APIVersion: "v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name: "benchmark-workflow",
		},
		Spec: WorkflowDefinition{
			Stages: stages,
			Variables: map[string]string{
				"benchmark": "true",
			},
			ConcurrencyConfig: &ConcurrencyConfig{
				MaxParallelStages: 10,
				StageTimeout:      5 * time.Minute,
				QueueSize:         100,
				WorkerPoolSize:    10,
			},
		},
	}
}

func createBenchmarkSession() *WorkflowSession {
	return &WorkflowSession{
		ID:               "benchmark-session",
		WorkflowID:       "benchmark-workflow",
		WorkflowName:     "benchmark-workflow",
		WorkflowVersion:  "v1",
		Status:           WorkflowStatusRunning,
		CompletedStages:  []string{},
		FailedStages:     []string{},
		SkippedStages:    []string{},
		StageResults:     make(map[string]interface{}),
		SharedContext:    make(map[string]interface{}),
		Checkpoints:      []WorkflowCheckpoint{},
		ResourceBindings: make(map[string]string),
		StartTime:        time.Now(),
		LastActivity:     time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func createComplexDependencyGraph(stageCount int) []WorkflowStage {
	stages := make([]WorkflowStage, stageCount)

	// Create a complex dependency graph with multiple levels
	for i := 0; i < stageCount; i++ {
		stage := WorkflowStage{
			Name:  fmt.Sprintf("stage-%d", i),
			Type:  "benchmark",
			Tools: []string{"mock-tool"},
		}

		// Add dependencies based on position
		if i > 0 {
			// Each stage depends on the previous one
			stage.DependsOn = append(stage.DependsOn, fmt.Sprintf("stage-%d", i-1))
		}

		if i > 2 {
			// Every 3rd stage also depends on stage-0
			stage.DependsOn = append(stage.DependsOn, "stage-0")
		}

		if i > 5 && i%2 == 0 {
			// Even stages after 5 depend on stage-2
			stage.DependsOn = append(stage.DependsOn, "stage-2")
		}

		stages[i] = stage
	}

	return stages
}

// Mock implementations for benchmarking

type MockStageExecutor struct{}

func (m *MockStageExecutor) ExecuteStage(ctx context.Context, stage *WorkflowStage, session *WorkflowSession) (*StageResult, error) {
	// Simulate work with a small delay
	time.Sleep(1 * time.Millisecond)

	return &StageResult{
		StageName: stage.Name,
		Success:   true,
		Duration:  1 * time.Millisecond,
		Results:   map[string]interface{}{"status": "completed"},
	}, nil
}

func (m *MockStageExecutor) ValidateStage(stage *WorkflowStage) error {
	return nil
}

type MockErrorRouter struct{}

func (m *MockErrorRouter) RouteError(ctx context.Context, err *WorkflowError, session *WorkflowSession) (*ErrorAction, error) {
	return nil, nil
}

func (m *MockErrorRouter) CanRecover(err *WorkflowError) bool {
	return false
}

func (m *MockErrorRouter) GetRecoveryOptions(err *WorkflowError) []RecoveryOption {
	return []RecoveryOption{}
}

type MockSessionManager struct{}

func (m *MockSessionManager) CreateSession(workflowSpec *WorkflowSpec) (*WorkflowSession, error) {
	return createBenchmarkSession(), nil
}

func (m *MockSessionManager) GetSession(sessionID string) (*WorkflowSession, error) {
	return createBenchmarkSession(), nil
}

func (m *MockSessionManager) UpdateSession(session *WorkflowSession) error {
	return nil
}

func (m *MockSessionManager) DeleteSession(sessionID string) error {
	return nil
}

func (m *MockSessionManager) ListSessions(filter SessionFilter) ([]*WorkflowSession, error) {
	return []*WorkflowSession{createBenchmarkSession()}, nil
}
