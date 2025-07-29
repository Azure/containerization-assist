package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// BenchmarkCoreSamplingPerformance focuses on core performance without middleware
func BenchmarkCoreSamplingPerformance(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Minimal logging for benchmarking
	}))

	ctx := context.Background()
	client := NewClient(logger)

	tests := []struct {
		name           string
		request        sampling.Request
		expectedTimeNs int64 // 300μs = 300,000ns
	}{
		{
			name: "SmallPrompt",
			request: sampling.Request{
				Prompt:      "Hello",
				MaxTokens:   10,
				Temperature: 0.7,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "MediumPrompt",
			request: sampling.Request{
				Prompt:      "Generate a simple Dockerfile for a Node.js application with Express framework",
				MaxTokens:   50,
				Temperature: 0.7,
			},
			expectedTimeNs: 300000, // 300μs
		},
		{
			name: "LargePrompt",
			request: sampling.Request{
				Prompt: `Analyze the following repository structure and provide detailed containerization recommendations:
				The application is a microservices architecture with multiple services including API gateway,
				user service, product service, and notification service. Each service has its own database
				and should be containerized separately. Consider security best practices, multi-stage builds,
				resource optimization, and deployment strategies for Kubernetes environments.`,
				MaxTokens:   100,
				Temperature: 0.8,
			},
			expectedTimeNs: 300000, // 300μs
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Warm up
			for i := 0; i < 5; i++ {
				client.Sample(ctx, tt.request)
			}

			b.ResetTimer()

			// Measure execution times for P95 calculation
			times := make([]time.Duration, b.N)

			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, _ = client.Sample(ctx, tt.request) // Ignore expected errors
				duration := time.Since(start)
				times[i] = duration
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
				targetMet := p95Duration.Nanoseconds() <= tt.expectedTimeNs
				if targetMet {
					b.Logf("✓ P95 latency %v meets target of <%v", p95Duration, time.Duration(tt.expectedTimeNs))
				} else {
					b.Logf("✗ P95 latency %v exceeds target of <%v", p95Duration, time.Duration(tt.expectedTimeNs))
				}
			}
		})
	}
}

// BenchmarkSamplingThroughputOptimized tests throughput without retries
func BenchmarkSamplingThroughputOptimized(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	ctx := context.Background()
	client := NewClient(logger)
	request := sampling.Request{
		Prompt:      "Quick throughput test",
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

// BenchmarkSamplingMemoryOptimized tests memory allocation patterns
func BenchmarkSamplingMemoryOptimized(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	ctx := context.Background()
	client := NewClient(logger)
	request := sampling.Request{
		Prompt:      "Memory test prompt for allocation patterns",
		MaxTokens:   25,
		Temperature: 0.7,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = client.Sample(ctx, request)
	}
}

// BenchmarkSamplingConcurrency tests concurrent performance
func BenchmarkSamplingConcurrency(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	ctx := context.Background()
	client := NewClient(logger)
	request := sampling.Request{
		Prompt:      "Concurrent test",
		MaxTokens:   15,
		Temperature: 0.7,
	}

	// Test different concurrency levels
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, _ = client.Sample(ctx, request)
				}
			})
		})
	}
}

// BenchmarkRequestSizes tests performance across different request sizes
func BenchmarkRequestSizes(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	ctx := context.Background()
	client := NewClient(logger)

	// Different prompt sizes to test scalability
	prompts := map[string]string{
		"Tiny":   "Hi",
		"Small":  "Generate a Dockerfile",
		"Medium": "Generate a comprehensive Dockerfile for a Node.js application with best practices",
		"Large": `Create a production-ready Dockerfile for a complex Node.js microservice that includes:
- Multi-stage build process
- Security best practices
- Optimized layer caching
- Health checks
- Non-root user configuration
- Minimal attack surface
- Efficient dependency management`,
		"XLarge": `Analyze and containerize a complex enterprise application with the following requirements:
- Multi-service architecture with API gateway, authentication service, business logic services, and data services
- Each service needs its own optimized container with specific runtime requirements
- Implement comprehensive security scanning and vulnerability assessment
- Design for high availability and scalability in Kubernetes environments
- Include monitoring, logging, and observability configurations
- Optimize for fast startup times and minimal resource consumption
- Implement proper secret management and configuration injection
- Design CI/CD pipeline integration with automated testing and deployment
- Consider compliance requirements and audit logging
- Implement proper error handling and graceful degradation strategies`,
	}

	for name, prompt := range prompts {
		b.Run(name, func(b *testing.B) {
			request := sampling.Request{
				Prompt:      prompt,
				MaxTokens:   50,
				Temperature: 0.7,
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = client.Sample(ctx, request)
			}
		})
	}
}
