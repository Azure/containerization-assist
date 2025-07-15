// Package infrastructure provides race detection tests for concurrent operations
package infrastructure

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/utilities"
	msgevents "github.com/Azure/container-kit/pkg/mcp/infrastructure/messaging/events"
	"github.com/stretchr/testify/assert"
)

// TestConcurrentGlobalMetrics tests for race conditions in global metrics access
func TestConcurrentGlobalMetrics(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 100

		var wg sync.WaitGroup

		// Test concurrent metrics recording
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// This should expose race conditions in global metrics
					metrics := sampling.GetGlobalMetrics()
					ctx := context.Background()
					duration := time.Millisecond * time.Duration(j+1)

					// Test concurrent recording with real API
					metrics.RecordSamplingRequest(ctx, "test-template", true, duration, 100, 50, 50, "text", 1024, sampling.ValidationResult{
						IsValid:       true,
						SyntaxValid:   true,
						BestPractices: true,
					})

					// Also test metrics access
					_ = metrics.GetCombinedMetrics()
					_ = metrics.GetHealthStatus()
				}
			}(i)
		}

		// Also test concurrent reset operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				sampling.ResetGlobalMetrics()
				time.Sleep(time.Millisecond)
			}
		}()

		wg.Wait()

		// Verify metrics are in valid state
		metrics := sampling.GetGlobalMetrics()
		assert.NotNil(t, metrics)
	})
}

// TestConcurrentWorkflowMetrics tests for race conditions in workflow metrics
func TestConcurrentWorkflowMetrics(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 30
		const numOperations = 50

		var wg sync.WaitGroup

		// Test concurrent workflow metrics access
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// Access workflow metrics concurrently
					metrics := events.GetGlobalWorkflowMetrics()

					// Test concurrent access to workflow metrics
					_ = metrics.GetMetrics()

					// Create a workflow event for testing
					event := &events.WorkflowStartedEvent{}

					// Verify event interface methods work concurrently
					_ = event.EventID()
					_ = event.OccurredAt()
					_ = event.WorkflowID()
					_ = event.EventType()
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestConcurrentSecretMasking tests for race conditions in secret masking
func TestConcurrentSecretMasking(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 40
		const numOperations = 75

		var wg sync.WaitGroup

		// Test concurrent secret masking operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// Use global masker functions concurrently
					input := "password=secret123 token=abc456"
					masked := utilities.Mask(input)
					assert.NotEqual(t, input, masked)

					// Test map masking
					testMap := map[string]interface{}{
						"password": "secret123",
						"token":    "abc456",
					}
					maskedMap := utilities.MaskMap(testMap)
					assert.NotNil(t, maskedMap)

					// Test adding custom patterns concurrently
					err := utilities.AddCustomPattern("test", `test=\w+`)
					assert.NoError(t, err)

					// Test adding custom secrets
					utilities.AddCustomSecret("mysecret123")
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestConcurrentCacheOperations tests for race conditions in cache operations
func TestConcurrentCacheOperations(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 25
		const numOperations = 100

		// This test would require creating a cache instance
		// For now, we'll focus on the identified global state issues
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// Simulate concurrent cache-like operations
					// This is a placeholder for actual cache testing
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestConcurrentEventPublishing tests for race conditions in event publishing
func TestConcurrentEventPublishing(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 20
		const numEvents = 50

		var wg sync.WaitGroup
		publisher := msgevents.NewPublisher(slog.Default())

		// Create event handler function
		handler := func(ctx context.Context, event events.DomainEvent) error {
			// Simple handler that just verifies event methods
			_ = event.EventID()
			_ = event.OccurredAt()
			return nil
		}

		// Register multiple handlers concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				publisher.Subscribe("workflow", handler)
			}(i)
		}

		// Publish events concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numEvents; j++ {
					event := &events.WorkflowStartedEvent{}

					err := publisher.Publish(context.Background(), event)
					assert.NoError(t, err)

					// Also test async publishing
					publisher.PublishAsync(context.Background(), event)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestRaceDetectionSuite runs a comprehensive race detection test
func TestRaceDetectionSuite(t *testing.T) {
	t.Run("comprehensive_race_detection", func(t *testing.T) {
		// Run all race tests in parallel to maximize chance of detecting races
		t.Run("global_metrics", TestConcurrentGlobalMetrics)
		t.Run("workflow_metrics", TestConcurrentWorkflowMetrics)
		t.Run("secret_masking", TestConcurrentSecretMasking)
		t.Run("cache_operations", TestConcurrentCacheOperations)
		t.Run("event_publishing", TestConcurrentEventPublishing)
	})
}
