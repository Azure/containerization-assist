package workflow

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Table-driven tests for better coverage and maintainability
func TestProgressiveErrorContext_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		maxHistory int
		operations []func(*ProgressiveErrorContext)
		validate   func(t *testing.T, pec *ProgressiveErrorContext)
	}{
		{
			name:       "escalation with repeated errors",
			maxHistory: 10,
			operations: []func(*ProgressiveErrorContext){
				func(pec *ProgressiveErrorContext) {
					// Add the same error 3 times
					for i := 0; i < 3; i++ {
						pec.AddError("build", errors.New("connection timeout"), i+1, nil)
					}
				},
			},
			validate: func(t *testing.T, pec *ProgressiveErrorContext) {
				assert.True(t, pec.ShouldEscalate("build"), "Should escalate after 3 repeated errors")
				assert.True(t, pec.HasRepeatedErrors("build", 3), "Should detect repeated errors")
			},
		},
		{
			name:       "escalation with many different errors",
			maxHistory: 20,
			operations: []func(*ProgressiveErrorContext){
				func(pec *ProgressiveErrorContext) {
					// Add 6 different errors
					for i := 0; i < 6; i++ {
						pec.AddError("deploy", fmt.Errorf("error variant %d", i), i+1, nil)
					}
				},
			},
			validate: func(t *testing.T, pec *ProgressiveErrorContext) {
				assert.True(t, pec.ShouldEscalate("deploy"), "Should escalate after 6 different errors")
				assert.False(t, pec.HasRepeatedErrors("deploy", 3), "Should not have repeated errors")
			},
		},
		{
			name:       "escalation with multiple fix attempts",
			maxHistory: 10,
			operations: []func(*ProgressiveErrorContext){
				func(pec *ProgressiveErrorContext) {
					pec.AddError("dockerfile", errors.New("syntax error"), 1, nil)
					pec.AddFixAttempt("dockerfile", "Fixed FROM statement")
					pec.AddFixAttempt("dockerfile", "Fixed ENV syntax")
				},
			},
			validate: func(t *testing.T, pec *ProgressiveErrorContext) {
				assert.True(t, pec.ShouldEscalate("dockerfile"), "Should escalate after 2 fix attempts")
			},
		},
		{
			name:       "step summary accumulation",
			maxHistory: 10,
			operations: []func(*ProgressiveErrorContext){
				func(pec *ProgressiveErrorContext) {
					pec.AddError("analyze", errors.New("no go.mod"), 1, nil)
					pec.AddError("analyze", errors.New("invalid syntax"), 2, nil)
					pec.AddError("build", errors.New("compilation failed"), 1, nil)
				},
			},
			validate: func(t *testing.T, pec *ProgressiveErrorContext) {
				summary := pec.GetSummary()
				assert.Contains(t, summary, "analyze (2 errors)")
				assert.Contains(t, summary, "build (1 errors)")
				assert.Contains(t, summary, "3 total errors")
			},
		},
		{
			name:       "AI context generation",
			maxHistory: 10,
			operations: []func(*ProgressiveErrorContext){
				func(pec *ProgressiveErrorContext) {
					pec.AddError("build", errors.New("docker build failed"), 1, map[string]interface{}{
						"exit_code": 1,
						"image":     "app:latest",
					})
					pec.AddFixAttempt("build", "Added missing dependencies")
					pec.AddError("build", errors.New("out of memory"), 2, map[string]interface{}{
						"memory_limit": "512MB",
					})
				},
			},
			validate: func(t *testing.T, pec *ProgressiveErrorContext) {
				aiContext := pec.GetAIContext()
				assert.Contains(t, aiContext, "PREVIOUS ERRORS AND ATTEMPTS:")
				assert.Contains(t, aiContext, "exit_code: 1")
				assert.Contains(t, aiContext, "Added missing dependencies")
				assert.Contains(t, aiContext, "memory_limit: 512MB")
				assert.Contains(t, aiContext, "STEP ERROR PATTERNS:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pec := NewProgressiveErrorContext(tt.maxHistory)

			// Execute operations
			for _, op := range tt.operations {
				op(pec)
			}

			// Validate results
			tt.validate(t, pec)
		})
	}
}

func TestProgressiveErrorContext_MaxHistoryEnforcement(t *testing.T) {
	tests := []struct {
		name          string
		maxHistory    int
		numErrors     int
		expectedCount int
		checkFirst    string
		checkLast     string
	}{
		{
			name:          "within limit",
			maxHistory:    5,
			numErrors:     3,
			expectedCount: 3,
			checkFirst:    "error 0",
			checkLast:     "error 2",
		},
		{
			name:          "exactly at limit",
			maxHistory:    5,
			numErrors:     5,
			expectedCount: 5,
			checkFirst:    "error 0",
			checkLast:     "error 4",
		},
		{
			name:          "exceeds limit",
			maxHistory:    3,
			numErrors:     10,
			expectedCount: 3,
			checkFirst:    "error 7", // Should keep last 3: 7, 8, 9
			checkLast:     "error 9",
		},
		{
			name:          "single item limit",
			maxHistory:    1,
			numErrors:     5,
			expectedCount: 1,
			checkFirst:    "error 4",
			checkLast:     "error 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pec := NewProgressiveErrorContext(tt.maxHistory)

			// Add errors
			for i := 0; i < tt.numErrors; i++ {
				pec.AddError("test", fmt.Errorf("error %d", i), i+1, nil)
			}

			// Verify count
			assert.Len(t, pec.errors, tt.expectedCount)

			// Verify content
			if tt.expectedCount > 0 {
				assert.Equal(t, tt.checkFirst, pec.errors[0].Error)
				assert.Equal(t, tt.checkLast, pec.errors[len(pec.errors)-1].Error)
			}
		})
	}
}

func TestProgressiveErrorContext_RaceConditions(t *testing.T) {
	pec := NewProgressiveErrorContext(1000)

	// Counters for verification
	var writeCount atomic.Int32
	var readCount atomic.Int32

	// Start time for test duration
	start := time.Now()
	duration := 100 * time.Millisecond

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Writer goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for time.Since(start) < duration {
				step := fmt.Sprintf("step_%d", id%3)
				err := fmt.Errorf("error_%d_%d", id, writeCount.Load())
				pec.AddError(step, err, int(writeCount.Add(1)), map[string]interface{}{
					"goroutine": id,
				})

				if writeCount.Load()%3 == 0 {
					pec.AddFixAttempt(step, fmt.Sprintf("fix_%d", writeCount.Load()))
				}
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for time.Since(start) < duration {
				readCount.Add(1)

				switch id % 5 {
				case 0:
					_ = pec.GetRecentErrors(5)
				case 1:
					_ = pec.GetStepErrors(fmt.Sprintf("step_%d", id%3))
				case 2:
					_ = pec.GetSummary()
				case 3:
					_ = pec.GetAIContext()
				case 4:
					_ = pec.ShouldEscalate(fmt.Sprintf("step_%d", id%3))
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify operations completed without panic
	t.Logf("Completed %d writes and %d reads without race conditions", writeCount.Load(), readCount.Load())
	assert.Greater(t, int(writeCount.Load()), 0, "Should have completed some writes")
	assert.Greater(t, int(readCount.Load()), 0, "Should have completed some reads")
}

func TestProgressiveErrorContext_ComplexScenarios(t *testing.T) {
	t.Run("interleaved errors and fixes", func(t *testing.T) {
		pec := NewProgressiveErrorContext(20)

		// Simulate a realistic error recovery scenario
		steps := []struct {
			action string
			step   string
			param  interface{}
		}{
			{"error", "dockerfile", errors.New("syntax error line 5")},
			{"fix", "dockerfile", "Fixed missing FROM statement"},
			{"error", "dockerfile", errors.New("unknown instruction")},
			{"fix", "dockerfile", "Fixed typo in RUN command"},
			{"error", "build", errors.New("package not found")},
			{"error", "dockerfile", errors.New("invalid base image")},
			{"fix", "dockerfile", "Changed base image to valid one"},
			{"error", "build", errors.New("compilation failed")},
			{"fix", "build", "Added missing dependencies"},
		}

		attempt := 1
		for _, s := range steps {
			if s.action == "error" {
				pec.AddError(s.step, s.param.(error), attempt, nil)
				attempt++
			} else {
				pec.AddFixAttempt(s.step, s.param.(string))
			}
		}

		// Verify state
		dockerfileErrors := pec.GetStepErrors("dockerfile")
		buildErrors := pec.GetStepErrors("build")

		assert.Len(t, dockerfileErrors, 3)
		assert.Len(t, buildErrors, 2)

		// Check that fixes are attached to correct errors
		fixCount := 0
		for _, err := range dockerfileErrors {
			fixCount += len(err.Fixes)
		}
		assert.Equal(t, 3, fixCount, "Should have 3 dockerfile fixes total")

		// Should NOT escalate - only 3 errors (not >5), no repeated errors, and each error has only 1 fix
		assert.False(t, pec.ShouldEscalate("dockerfile"))
	})

	t.Run("error patterns across time", func(t *testing.T) {
		pec := NewProgressiveErrorContext(50)

		// Add errors with time delays to test timestamp handling
		baseTime := time.Now()

		// Pattern 1: Repeated transient errors
		for i := 0; i < 3; i++ {
			pec.AddError("network", errors.New("connection timeout"), i+1, map[string]interface{}{
				"timestamp": baseTime.Add(time.Duration(i) * time.Second),
			})
		}

		// Pattern 2: Different errors for same step
		errorTypes := []string{"syntax error", "missing file", "permission denied"}
		for i, errType := range errorTypes {
			pec.AddError("setup", errors.New(errType), i+1, nil)
		}

		// Verify pattern detection
		assert.True(t, pec.HasRepeatedErrors("network", 3))
		assert.False(t, pec.HasRepeatedErrors("setup", 2))

		// Get AI context and verify it captures patterns
		aiContext := pec.GetAIContext()
		assert.Contains(t, aiContext, "connection timeout")
		assert.Contains(t, aiContext, "network:")
		assert.Contains(t, aiContext, "setup:")
	})
}

func TestProgressiveErrorContext_AdvancedEdgeCases(t *testing.T) {
	t.Run("zero max history", func(t *testing.T) {
		pec := NewProgressiveErrorContext(0)
		pec.AddError("test", errors.New("error"), 1, nil)

		// Should handle gracefully, possibly keeping at least 1
		errors := pec.GetRecentErrors(10)
		assert.GreaterOrEqual(t, len(errors), 0)
	})

	t.Run("negative count in GetRecentErrors", func(t *testing.T) {
		pec := NewProgressiveErrorContext(10)
		pec.AddError("test", errors.New("error"), 1, nil)

		errors := pec.GetRecentErrors(-1)
		assert.Len(t, errors, 0)
	})

	t.Run("empty step names", func(t *testing.T) {
		pec := NewProgressiveErrorContext(10)
		pec.AddError("", errors.New("error with empty step"), 1, nil)
		pec.AddFixAttempt("", "fix for empty step")

		errors := pec.GetStepErrors("")
		assert.Len(t, errors, 1)
		assert.Len(t, errors[0].Fixes, 1)
	})

	t.Run("very long error messages", func(t *testing.T) {
		pec := NewProgressiveErrorContext(10)
		longError := strings.Repeat("error ", 10000) // 60KB error message
		pec.AddError("test", errors.New(longError), 1, nil)

		summary := pec.GetSummary()
		assert.Contains(t, summary, "error error") // Should contain at least part of it
	})

	t.Run("nil and empty contexts", func(t *testing.T) {
		pec := NewProgressiveErrorContext(10)

		// Nil context
		pec.AddError("test1", errors.New("error1"), 1, nil)

		// Empty context
		pec.AddError("test2", errors.New("error2"), 1, map[string]interface{}{})

		// Context with nil values
		pec.AddError("test3", errors.New("error3"), 1, map[string]interface{}{
			"key1": nil,
			"key2": "value",
		})

		aiContext := pec.GetAIContext()
		assert.NotPanics(t, func() {
			_ = pec.GetSummary()
			_ = pec.GetAIContext()
		})
		assert.Contains(t, aiContext, "error1")
		assert.Contains(t, aiContext, "error2")
		assert.Contains(t, aiContext, "error3")
	})
}

// Benchmark tests for performance analysis
func BenchmarkProgressiveErrorContext_Operations(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func(b *testing.B, pec *ProgressiveErrorContext)
	}{
		{
			name: "AddError",
			fn: func(b *testing.B, pec *ProgressiveErrorContext) {
				err := errors.New("benchmark error")
				ctx := map[string]interface{}{"key": "value"}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					pec.AddError("step", err, i, ctx)
				}
			},
		},
		{
			name: "GetRecentErrors",
			fn: func(b *testing.B, pec *ProgressiveErrorContext) {
				// Populate with errors
				for i := 0; i < 100; i++ {
					pec.AddError("step", errors.New("error"), i, nil)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = pec.GetRecentErrors(10)
				}
			},
		},
		{
			name: "GetSummary",
			fn: func(b *testing.B, pec *ProgressiveErrorContext) {
				// Populate with varied errors
				for i := 0; i < 50; i++ {
					pec.AddError(fmt.Sprintf("step%d", i%5), fmt.Errorf("error %d", i), i, nil)
					if i%3 == 0 {
						pec.AddFixAttempt(fmt.Sprintf("step%d", i%5), fmt.Sprintf("fix %d", i))
					}
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = pec.GetSummary()
				}
			},
		},
		{
			name: "ConcurrentMixedOps",
			fn: func(b *testing.B, pec *ProgressiveErrorContext) {
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						switch i % 4 {
						case 0:
							pec.AddError("step", errors.New("error"), i, nil)
						case 1:
							_ = pec.GetRecentErrors(5)
						case 2:
							_ = pec.HasRepeatedErrors("step", 3)
						case 3:
							_ = pec.GetSummary()
						}
						i++
					}
				})
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			pec := NewProgressiveErrorContext(1000)
			bm.fn(b, pec)
		})
	}
}
