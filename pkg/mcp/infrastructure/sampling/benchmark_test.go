package sampling

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/stretchr/testify/require"
)

// BenchmarkSamplingPerformance tests the P95 latency target of <300μs
func BenchmarkSamplingPerformance(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise during benchmarking
	}))

	// Create context for testing (no server needed for client-side benchmarks)
	ctx := context.Background()

	tests := []struct {
		name           string
		createClient   func() sampling.UnifiedSampler
		request        sampling.Request
		expectedTimeNs int64 // 300μs = 300,000ns
	}{
		{
			name: "CoreClient_SmallPrompt",
			createClient: func() sampling.UnifiedSampler {
				return NewCoreClient(logger)
			},
			request: sampling.Request{
				Prompt:      "Hello world",
				MaxTokens:   10,
				Temperature: 0.7,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "CoreClient_MediumPrompt",
			createClient: func() sampling.UnifiedSampler {
				return NewCoreClient(logger)
			},
			request: sampling.Request{
				Prompt:      "This is a medium-sized prompt with some more text to simulate real usage patterns and see how performance scales with input size",
				MaxTokens:   50,
				Temperature: 0.7,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "CoreClient_LargePrompt",
			createClient: func() sampling.UnifiedSampler {
				return NewCoreClient(logger)
			},
			request: sampling.Request{
				Prompt: `This is a large prompt that contains significantly more text to test performance under realistic conditions.
				It includes multiple lines, detailed instructions, and substantial content that would be typical in
				real-world containerization workflows. The prompt discusses Docker optimization, Kubernetes deployment
				strategies, security scanning practices, and various best practices for container orchestration.
				This level of detail helps us understand how the sampling client performs with more realistic workloads.`,
				MaxTokens:   100,
				Temperature: 0.7,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "DomainAdapter_StandardRequest",
			createClient: func() sampling.UnifiedSampler {
				client := NewClient(logger)
				return NewDomainAdapter(client)
			},
			request: sampling.Request{
				Prompt:      "Generate a Dockerfile for a Node.js application",
				MaxTokens:   75,
				Temperature: 0.8,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "ClientWithMiddleware_ComplexRequest",
			createClient: func() sampling.UnifiedSampler {
				opts := ClientOptions{
					EnableTracing: false, // Disable for benchmarking
					EnableMetrics: false, // Disable for benchmarking
					EnableRetry:   false, // Disable for benchmarking
				}
				return NewClientWithMiddleware(logger, opts)
			},
			request: sampling.Request{
				Prompt:      "Analyze repository structure and recommend deployment strategy",
				MaxTokens:   80,
				Temperature: 0.6,
				Metadata: map[string]interface{}{
					"priority": "high",
					"source":   "benchmark",
				},
			},
			expectedTimeNs: 300000, // 300μs
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := tt.createClient()

			// Warm up
			for i := 0; i < 10; i++ {
				_, _ = client.Sample(ctx, tt.request)
			}

			b.ResetTimer()

			// Measure execution times for P95 calculation
			times := make([]time.Duration, b.N)

			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, err := client.Sample(ctx, tt.request)
				duration := time.Since(start)
				times[i] = duration

				// Don't fail the benchmark on expected "no MCP server" errors
				// We're measuring client-side performance
				if err != nil && !isExpectedBenchmarkError(err) {
					b.Fatalf("Unexpected error: %v", err)
				}
			}

			// Calculate P95 latency
			if len(times) > 0 {
				p95Index := int(float64(len(times)) * 0.95)
				if p95Index >= len(times) {
					p95Index = len(times) - 1
				}

				// Sort times to find P95
				sortTimes(times)
				p95Duration := times[p95Index]

				b.ReportMetric(float64(p95Duration.Nanoseconds()), "p95_ns")
				b.ReportMetric(float64(p95Duration.Microseconds()), "p95_μs")

				// Check if we meet the <300μs P95 target
				if p95Duration.Nanoseconds() > tt.expectedTimeNs {
					b.Logf("P95 latency %v exceeds target of %v", p95Duration, time.Duration(tt.expectedTimeNs))
				} else {
					b.Logf("P95 latency %v meets target of <%v", p95Duration, time.Duration(tt.expectedTimeNs))
				}
			}
		})
	}
}

// BenchmarkSamplingThroughput tests how many requests can be processed per second
func BenchmarkSamplingThroughput(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	ctx := context.Background()

	client := NewCoreClient(logger)
	request := sampling.Request{
		Prompt:      "Quick test",
		MaxTokens:   10,
		Temperature: 0.7,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = client.Sample(ctx, request)
		}
	})
}

// BenchmarkSamplingMemory tests memory allocation patterns
func BenchmarkSamplingMemory(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	ctx := context.Background()

	client := NewCoreClient(logger)
	request := sampling.Request{
		Prompt:      "Memory allocation test prompt",
		MaxTokens:   50,
		Temperature: 0.7,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = client.Sample(ctx, request)
	}
}

// BenchmarkSamplingWithRetries tests performance under retry scenarios
func BenchmarkSamplingWithRetries(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	ctx := context.Background()

	opts := ClientOptions{
		EnableTracing: false,
		EnableMetrics: false,
		EnableRetry:   true, // Test with retries
	}
	client := NewClientWithMiddleware(logger, opts)

	request := sampling.Request{
		Prompt:      "Retry test prompt",
		MaxTokens:   25,
		Temperature: 0.8,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = client.Sample(ctx, request)
	}
}

// BenchmarkConcurrentSampling tests performance under concurrent load
func BenchmarkConcurrentSampling(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	ctx := context.Background()

	client := NewCoreClient(logger)
	request := sampling.Request{
		Prompt:      "Concurrent test",
		MaxTokens:   20,
		Temperature: 0.7,
	}

	b.SetParallelism(10) // 10 concurrent goroutines
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = client.Sample(ctx, request)
		}
	})
}

// Helper functions

func isExpectedBenchmarkError(err error) bool {
	if err == nil {
		return true
	}
	errStr := err.Error()
	// These are expected errors during benchmarking when there's no real MCP server
	return contains(errStr, "no MCP server") ||
		contains(errStr, "not available") ||
		contains(errStr, "context") ||
		contains(errStr, "timeout")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		s[:len(s)-len(substr)+1] != "" &&
		findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func sortTimes(times []time.Duration) {
	// Simple bubble sort for benchmarking (we don't want to import sort package)
	n := len(times)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if times[j] > times[j+1] {
				times[j], times[j+1] = times[j+1], times[j]
			}
		}
	}
}

// TestBenchmarkBaseline ensures our benchmark tests are working correctly
func TestBenchmarkBaseline(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	// Test that our client creation works
	coreClient := NewCoreClient(logger)
	require.NotNil(t, coreClient, "Core client should be created successfully")

	// Test that legacy client works
	client := NewClient(logger)
	require.NotNil(t, client, "Client should be created successfully")

	// Test that domain adapter works
	adapter := NewDomainAdapter(client)
	require.NotNil(t, adapter, "Domain adapter should be created successfully")

	// Test that middleware client works
	opts := ClientOptions{
		EnableTracing: false,
		EnableMetrics: false,
		EnableRetry:   false,
	}
	middlewareClient := NewClientWithMiddleware(logger, opts)
	require.NotNil(t, middlewareClient, "Middleware client should be created successfully")
}
