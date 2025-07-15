// Package infrastructure provides simple race detection tests
package infrastructure

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
	"github.com/stretchr/testify/assert"
)

// TestSimpleGlobalMetrics tests global metrics access without reset
func TestSimpleGlobalMetrics(t *testing.T) {
	t.Run("race", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 20

		var wg sync.WaitGroup

		// Test concurrent metrics recording without reset
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					// Get global metrics and use them
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
				}
			}(i)
		}

		wg.Wait()

		// Verify metrics are in valid state
		metrics := sampling.GetGlobalMetrics()
		assert.NotNil(t, metrics)
	})
}
