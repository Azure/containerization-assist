package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/common/interfaces"
	"github.com/Azure/container-kit/pkg/common/pools"
	"github.com/Azure/container-kit/pkg/common/validation"
	"github.com/Azure/container-kit/pkg/mcp/api"
)

// ============================================================================
// WORKSTREAM DELTA: Performance Benchmarks
// Validates <300μs P95 performance target maintenance
// ============================================================================

// MockTool for benchmarking
type MockTool struct {
	name        string
	description string
	execTime    time.Duration
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return m.description
}

func (m *MockTool) Execute(_ context.Context, _ api.ToolInput) (api.ToolOutput, error) {
	// Simulate work
	if m.execTime > 0 {
		time.Sleep(m.execTime)
	}

	return api.ToolOutput{
		Success: true,
		Data:    map[string]interface{}{"result": "test"},
	}, nil
}

func (m *MockTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        m.name,
		Description: m.description,
		Version:     "1.0.0",
	}
}

// BenchmarkUnifiedValidator tests validation performance
func BenchmarkUnifiedValidator(b *testing.B) {
	validator := validation.NewUnifiedValidator([]string{
		interfaces.CapabilityValidation,
		interfaces.CapabilityBusinessRules,
	})

	input := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			_ = validator.ValidateInput(ctx, "test-tool", input)
		}
	})
}

// BenchmarkOptimizedExecutor tests executor performance
func BenchmarkOptimizedExecutor(b *testing.B) {
	config := DefaultExecutorConfig()
	config.EnableValidation = false // Focus on execution performance
	config.EnableMetrics = false    // Focus on execution performance

	executor := NewOptimizedExecutor(config)

	tool := &MockTool{
		name:        "benchmark-tool",
		description: "Tool for benchmarking",
		execTime:    0, // No artificial delay
	}

	input := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"operation": "benchmark",
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			_, _ = executor.ExecuteTool(ctx, tool, input)
		}
	})
}

// BenchmarkToolExecutionPipeline tests the full execution pipeline
func BenchmarkToolExecutionPipeline(b *testing.B) {
	// Setup with all features enabled
	config := DefaultExecutorConfig()
	executor := NewOptimizedExecutor(config)

	// Setup validator
	validator := validation.NewUnifiedValidator([]string{
		interfaces.CapabilityValidation,
		interfaces.CapabilityBusinessRules,
	})
	executor.SetValidator(validator)

	tool := &MockTool{
		name:        "pipeline-tool",
		description: "Tool for pipeline benchmarking",
		execTime:    0,
	}

	input := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"operation": "pipeline_test",
			"data":      "test data",
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			_, _ = executor.ExecuteTool(ctx, tool, input)
		}
	})
}

// BenchmarkBufferPool tests buffer pool performance
func BenchmarkBufferPool(b *testing.B) {
	b.Run("GetPut", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pools.BufferPool.Get()
				// Simulate some work
				buf = append(buf, []byte("test data")...)
				pools.BufferPool.Put(buf)
			}
		})
	})

	b.Run("WithBuffer", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = pools.WithBuffer(func(buf []byte) error {
					// Simulate some work
					buf = append(buf, []byte("test data")...)
					return nil
				})
			}
		})
	})
}

// BenchmarkJSONOperations tests JSON performance optimizations
func BenchmarkJSONOperations(b *testing.B) {
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": []string{"a", "b", "c"},
		"key4": map[string]interface{}{
			"nested": "value",
		},
	}

	b.Run("StandardJSON", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = json.Marshal(testData)
			}
		})
	})

	b.Run("PooledJSON", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = pools.FastJSONMarshal(testData)
			}
		})
	})
}

// BenchmarkStringOperations tests string operation optimizations
func BenchmarkStringOperations(b *testing.B) {
	parts := []string{"part1", "part2", "part3", "part4", "part5"}

	b.Run("StandardConcat", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result := ""
				for _, part := range parts {
					result += part
				}
				_ = result
			}
		})
	})

	b.Run("PooledConcat", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result := pools.FastStringConcat(parts...)
				_ = result
			}
		})
	})
}

// BenchmarkMapOperations tests map pool performance
func BenchmarkMapOperations(b *testing.B) {
	b.Run("StandardMap", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m := make(map[string]interface{})
				m["key1"] = "value1"
				m["key2"] = "value2"
				m["key3"] = "value3"
				// Map goes out of scope and gets GC'd
			}
		})
	})

	b.Run("PooledMap", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = pools.WithStringInterfaceMap(func(m map[string]interface{}) error {
					m["key1"] = "value1"
					m["key2"] = "value2"
					m["key3"] = "value3"
					return nil
				})
			}
		})
	})
}

// BenchmarkValidationFramework tests validation performance
func BenchmarkValidationFramework(b *testing.B) {
	validator := validation.NewUnifiedValidator([]string{
		interfaces.CapabilityValidation,
		interfaces.CapabilitySchemaValidation,
		interfaces.CapabilityBusinessRules,
	})

	testCases := []struct {
		name string
		data interface{}
	}{
		{
			name: "SimpleInput",
			data: api.ToolInput{
				SessionID: "test",
				Data:      map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "ComplexInput",
			data: api.ToolInput{
				SessionID: "test",
				Data: map[string]interface{}{
					"nested": map[string]interface{}{
						"deep": []string{"a", "b", "c"},
					},
					"array": []interface{}{1, 2, 3, "test"},
				},
			},
		},
		{
			name: "LargeInput",
			data: func() api.ToolInput {
				data := make(map[string]interface{})
				for i := 0; i < 100; i++ {
					data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
				}
				return api.ToolInput{
					SessionID: "test",
					Data:      data,
				}
			}(),
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ctx := context.Background()
					if input, ok := tc.data.(api.ToolInput); ok {
						_ = validator.ValidateInput(ctx, "test-tool", input)
					} else {
						_ = validator.ValidateConfig(ctx, tc.data)
					}
				}
			})
		})
	}
}

// BenchmarkConcurrentToolExecution tests concurrent execution performance
func BenchmarkConcurrentToolExecution(b *testing.B) {
	config := DefaultExecutorConfig()
	config.WorkerPoolSize = 50 // High concurrency
	config.EnableCaching = true

	executor := NewOptimizedExecutor(config)

	tool := &MockTool{
		name:        "concurrent-tool",
		description: "Tool for concurrent benchmarking",
		execTime:    100 * time.Microsecond, // Small delay to simulate work
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			input := api.ToolInput{
				SessionID: "test-session",
				Data: map[string]interface{}{
					"operation": "concurrent_test",
					"id":        b.N,
				},
			}
			_, _ = executor.ExecuteTool(ctx, tool, input)
		}
	})
}

// BenchmarkP95LatencyTarget validates the <300μs P95 target
func BenchmarkP95LatencyTarget(b *testing.B) {
	config := HighPerformanceConfig() // Optimized for speed
	executor := NewOptimizedExecutor(config)

	tool := &MockTool{
		name:        "latency-tool",
		description: "Tool for latency benchmarking",
		execTime:    0, // No artificial delay
	}

	input := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"operation": "latency_test",
		},
	}

	// Track latencies to calculate P95
	latencies := make([]time.Duration, 0, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		ctx := context.Background()
		_, _ = executor.ExecuteTool(ctx, tool, input)
		latency := time.Since(start)
		latencies = append(latencies, latency)
	}

	// Calculate P95
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		p95Index := int(float64(len(latencies)) * 0.95)
		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}

		p95Latency := latencies[p95Index]
		target := 300 * time.Microsecond

		b.Logf("P95 Latency: %v (target: %v)", p95Latency, target)

		if p95Latency > target {
			b.Errorf("P95 latency %v exceeds target %v", p95Latency, target)
		}
	}
}

// TestPerformanceRegression tests for performance regressions
func TestPerformanceRegression(t *testing.T) {
	// This test should be run in CI to catch performance regressions
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	config := DefaultExecutorConfig()
	executor := NewOptimizedExecutor(config)

	tool := &MockTool{
		name:        "regression-tool",
		description: "Tool for regression testing",
		execTime:    0,
	}

	input := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"operation": "regression_test",
		},
	}

	// Run multiple iterations to get stable measurements
	const iterations = 1000
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		ctx := context.Background()
		_, err := executor.ExecuteTool(ctx, tool, input)
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}
		totalDuration += time.Since(start)
	}

	avgLatency := totalDuration / iterations
	maxAcceptableLatency := 100 * time.Microsecond // Conservative target

	t.Logf("Average latency: %v (max acceptable: %v)", avgLatency, maxAcceptableLatency)

	if avgLatency > maxAcceptableLatency {
		t.Errorf("Average latency %v exceeds acceptable threshold %v", avgLatency, maxAcceptableLatency)
	}
}
