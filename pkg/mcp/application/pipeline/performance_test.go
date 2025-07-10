package pipeline

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/commands"
)

func BenchmarkAtomicPipelineExecution(b *testing.B) {
	pipeline := NewAtomicPipeline(
		&TestStage{name: "stage1", delay: 10 * time.Microsecond},
		&TestStage{name: "stage2", delay: 10 * time.Microsecond},
		&TestStage{name: "stage3", delay: 10 * time.Microsecond},
	)

	ctx := context.Background()
	request := &api.PipelineRequest{Input: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Execute(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWorkflowPipelineExecution(b *testing.B) {
	pipeline := NewWorkflowPipeline(false,
		&TestStage{name: "stage1", delay: 10 * time.Microsecond},
		&TestStage{name: "stage2", delay: 10 * time.Microsecond},
		&TestStage{name: "stage3", delay: 10 * time.Microsecond},
	)

	ctx := context.Background()
	request := &api.PipelineRequest{Input: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Execute(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOrchestrationPipelineExecution(b *testing.B) {
	pipeline := NewOrchestrationPipeline(
		&TestStage{name: "stage1", delay: 10 * time.Microsecond},
		&TestStage{name: "stage2", delay: 10 * time.Microsecond},
		&TestStage{name: "stage3", delay: 10 * time.Microsecond},
	)

	ctx := context.Background()
	request := &api.PipelineRequest{Input: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Execute(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCommandRouterExecution(b *testing.B) {
	router := commands.NewRouter()
	_ = router.Register("test", &TestCommandHandler{})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := router.Route(ctx, "test", "args")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test helpers
type TestStage struct {
	name  string
	delay time.Duration
}

func (s *TestStage) Name() string { return s.name }

func (s *TestStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return input, nil
}

func (s *TestStage) Validate(_ interface{}) error { return nil }

type TestCommandHandler struct{}

func (h *TestCommandHandler) Execute(_ context.Context, args interface{}) (interface{}, error) {
	return "result", nil
}

// P95 validation test
func TestP95PerformanceTarget(t *testing.T) {
	const targetP95 = 300 * time.Microsecond
	const iterations = 1000

	pipeline := NewOrchestrationPipeline(
		&TestStage{name: "stage1", delay: 0},
		&TestStage{name: "stage2", delay: 0},
	)

	ctx := context.Background()
	request := &api.PipelineRequest{Input: "test"}

	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := pipeline.Execute(ctx, request)
		durations[i] = time.Since(start)

		if err != nil {
			t.Fatal(err)
		}
	}

	// Calculate P95
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	p95Index := int(float64(iterations) * 0.95)
	p95Duration := durations[p95Index]

	t.Logf("P95 latency: %v (target: %v)", p95Duration, targetP95)

	if p95Duration > targetP95 {
		t.Errorf("P95 latency %v exceeds target %v", p95Duration, targetP95)
	}
}

// Concurrency performance test
func BenchmarkConcurrentPipelineExecution(b *testing.B) {
	pipeline := NewOrchestrationPipeline(
		&TestStage{name: "stage1", delay: 5 * time.Microsecond},
		&TestStage{name: "stage2", delay: 5 * time.Microsecond},
	)

	ctx := context.Background()
	request := &api.PipelineRequest{Input: "test"}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pipeline.Execute(ctx, request)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
