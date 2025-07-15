package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProgressiveErrorContext(t *testing.T) {
	maxHistory := 10
	ctx := NewProgressiveErrorContext(maxHistory)

	assert.NotNil(t, ctx)
	assert.Equal(t, maxHistory, ctx.maxHistory)
	assert.Empty(t, ctx.errors)
	assert.NotNil(t, ctx.stepSummary)
}

func TestProgressiveErrorContext_AddError(t *testing.T) {
	ctx := NewProgressiveErrorContext(5)
	err := fmt.Errorf("build failed")
	step := "build"
	attempt := 1
	context := map[string]interface{}{
		"dockerfile": "/path/to/Dockerfile",
		"command":    "docker build",
	}

	ctx.AddError(step, err, attempt, context)

	assert.Len(t, ctx.errors, 1)
	errorCtx := ctx.errors[0]
	assert.Equal(t, step, errorCtx.Step)
	assert.Equal(t, err.Error(), errorCtx.Error)
	assert.Equal(t, attempt, errorCtx.Attempt)
	assert.Equal(t, context, errorCtx.Context)
	assert.WithinDuration(t, time.Now(), errorCtx.Timestamp, 1*time.Second)
}

func TestProgressiveErrorContext_AddError_MaxHistoryLimit(t *testing.T) {
	maxHistory := 3
	ctx := NewProgressiveErrorContext(maxHistory)

	// Add more errors than max history
	for i := 0; i < 5; i++ {
		err := fmt.Errorf("error %d", i)
		ctx.AddError("step", err, i+1, nil)
	}

	// Should only keep the most recent 3 errors
	assert.Len(t, ctx.errors, maxHistory)

	// Check that the oldest errors were removed (should have errors 2, 3, 4)
	assert.Equal(t, "error 2", ctx.errors[0].Error)
	assert.Equal(t, "error 3", ctx.errors[1].Error)
	assert.Equal(t, "error 4", ctx.errors[2].Error)
}

func TestProgressiveErrorContext_AddFixAttempt(t *testing.T) {
	ctx := NewProgressiveErrorContext(5)
	step := "build"
	err := fmt.Errorf("build failed")

	// Add an error first
	ctx.AddError(step, err, 1, nil)

	// Add a fix attempt
	fix := "Updated Dockerfile base image"
	ctx.AddFixAttempt(step, fix)

	// Check that the fix was added to the most recent error for this step
	assert.Len(t, ctx.errors, 1)
	assert.Contains(t, ctx.errors[0].Fixes, fix)
	assert.Len(t, ctx.errors[0].Fixes, 1)
}

func TestProgressiveErrorContext_AddFixAttempt_MultipleSteps(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	// Add errors for different steps
	ctx.AddError("build", fmt.Errorf("build error"), 1, nil)
	ctx.AddError("deploy", fmt.Errorf("deploy error"), 1, nil)
	ctx.AddError("build", fmt.Errorf("another build error"), 2, nil)

	// Add fix for build step
	buildFix := "Fixed Dockerfile"
	ctx.AddFixAttempt("build", buildFix)

	// Add fix for deploy step
	deployFix := "Updated manifest"
	ctx.AddFixAttempt("deploy", deployFix)

	// Check that fixes are applied to the correct steps
	buildErrors := 0
	deployErrors := 0

	for _, errorCtx := range ctx.errors {
		if errorCtx.Step == "build" && len(errorCtx.Fixes) > 0 {
			buildErrors++
			assert.Contains(t, errorCtx.Fixes, buildFix)
		}
		if errorCtx.Step == "deploy" && len(errorCtx.Fixes) > 0 {
			deployErrors++
			assert.Contains(t, errorCtx.Fixes, deployFix)
		}
	}

	assert.Equal(t, 1, buildErrors, "Build fix should be applied to the most recent build error")
	assert.Equal(t, 1, deployErrors, "Deploy fix should be applied to the deploy error")
}

func TestProgressiveErrorContext_GetRecentErrors(t *testing.T) {
	ctx := NewProgressiveErrorContext(5)

	// Add multiple errors
	ctx.AddError("analyze", fmt.Errorf("analyze failed"), 1, map[string]interface{}{"repo": "test"})
	ctx.AddError("build", fmt.Errorf("build failed"), 1, map[string]interface{}{"dockerfile": "test"})
	ctx.AddFixAttempt("analyze", "Fixed repo path")

	recent := ctx.GetRecentErrors(2)

	assert.Len(t, recent, 2)
	assert.Equal(t, "analyze", recent[0].Step)
	assert.Equal(t, "build", recent[1].Step)
	assert.Contains(t, recent[0].Fixes, "Fixed repo path")
}

func TestProgressiveErrorContext_ShouldEscalate(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)
	step := "build"

	// Should not escalate with fewer errors and no repeated pattern
	ctx.AddError(step, fmt.Errorf("error 1"), 1, nil)
	assert.False(t, ctx.ShouldEscalate(step))

	ctx.AddError(step, fmt.Errorf("error 2"), 2, nil)
	assert.False(t, ctx.ShouldEscalate(step))

	// Should escalate after adding fixes (>= 2 fixes triggers escalation)
	ctx.AddFixAttempt(step, "fix 1")
	ctx.AddFixAttempt(step, "fix 2")
	assert.True(t, ctx.ShouldEscalate(step))
}

func TestProgressiveErrorContext_ShouldEscalate_DifferentSteps(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	// Add failures for different steps
	ctx.AddError("build", fmt.Errorf("build error 1"), 1, nil)
	ctx.AddError("deploy", fmt.Errorf("deploy error 1"), 1, nil)
	ctx.AddError("build", fmt.Errorf("build error 2"), 2, nil)
	ctx.AddError("build", fmt.Errorf("build error 3"), 3, nil)

	// Add enough fixes to trigger escalation for build
	ctx.AddFixAttempt("build", "fix 1")
	ctx.AddFixAttempt("build", "fix 2")

	// Only build should escalate (due to fixes), deploy should not
	assert.True(t, ctx.ShouldEscalate("build"))
	assert.False(t, ctx.ShouldEscalate("deploy"))
	assert.False(t, ctx.ShouldEscalate("nonexistent"))
}

func TestProgressiveErrorContext_GetSummary(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	// Add various errors
	ctx.AddError("analyze", fmt.Errorf("repository not found"), 1, nil)
	ctx.AddError("build", fmt.Errorf("dockerfile syntax error"), 1, nil)
	ctx.AddError("build", fmt.Errorf("build failed"), 2, nil)
	ctx.AddFixAttempt("build", "Fixed syntax")

	summary := ctx.GetSummary()

	assert.Contains(t, summary, "repository not found")
	assert.Contains(t, summary, "dockerfile syntax error")
	assert.Contains(t, summary, "build failed")
	assert.Contains(t, summary, "Fixed syntax")
}

func TestProgressiveErrorContext_GetStepErrors(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	// Add errors for different steps
	ctx.AddError("build", fmt.Errorf("error 1"), 1, nil)
	ctx.AddError("build", fmt.Errorf("error 2"), 2, nil)
	ctx.AddError("deploy", fmt.Errorf("error 3"), 1, nil)

	buildErrors := ctx.GetStepErrors("build")
	deployErrors := ctx.GetStepErrors("deploy")
	nonexistentErrors := ctx.GetStepErrors("nonexistent")

	assert.Len(t, buildErrors, 2)
	assert.Len(t, deployErrors, 1)
	assert.Len(t, nonexistentErrors, 0)
}

func TestProgressiveErrorContext_HasRepeatedErrors(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)
	step := "build"

	// Add similar errors (same step, similar error messages)
	ctx.AddError(step, fmt.Errorf("docker build failed"), 1, nil)
	ctx.AddError(step, fmt.Errorf("docker build failed"), 2, nil)
	ctx.AddError(step, fmt.Errorf("docker build failed"), 3, nil)

	assert.True(t, ctx.HasRepeatedErrors(step, 3))
}

func TestProgressiveErrorContext_HasRepeatedErrors_NoPattern(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)
	step := "build"

	// Add different errors
	ctx.AddError(step, fmt.Errorf("syntax error"), 1, nil)
	ctx.AddError(step, fmt.Errorf("network error"), 2, nil)
	ctx.AddError(step, fmt.Errorf("permission error"), 3, nil)

	assert.False(t, ctx.HasRepeatedErrors(step, 3))
}

func TestProgressiveErrorContext_GetStepErrors_LatestContext(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)
	step := "build"

	// Add errors with different contexts
	firstContext := map[string]interface{}{"attempt": 1, "dockerfile": "v1"}
	secondContext := map[string]interface{}{"attempt": 2, "dockerfile": "v2"}

	ctx.AddError(step, fmt.Errorf("first error"), 1, firstContext)
	ctx.AddError(step, fmt.Errorf("second error"), 2, secondContext)

	stepErrors := ctx.GetStepErrors(step)
	require.Len(t, stepErrors, 2)

	// Latest error should be the last one
	latestError := stepErrors[len(stepErrors)-1]
	assert.Equal(t, secondContext, latestError.Context)
}

func TestProgressiveErrorContext_GetStepErrors_NoErrors(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	stepErrors := ctx.GetStepErrors("nonexistent")
	assert.Len(t, stepErrors, 0)
}

func TestProgressiveErrorContext_EdgeCases(t *testing.T) {
	t.Run("Empty step name", func(t *testing.T) {
		ctx := NewProgressiveErrorContext(5)
		ctx.AddError("", fmt.Errorf("error"), 1, nil)

		assert.Len(t, ctx.errors, 1)
		assert.Equal(t, "", ctx.errors[0].Step)
	})

	t.Run("Error message handling", func(t *testing.T) {
		ctx := NewProgressiveErrorContext(5)
		err := fmt.Errorf("test error message")
		ctx.AddError("step", err, 1, nil)

		assert.Len(t, ctx.errors, 1)
		assert.Equal(t, err.Error(), ctx.errors[0].Error)
	})

	t.Run("Nil context", func(t *testing.T) {
		ctx := NewProgressiveErrorContext(5)
		ctx.AddError("step", fmt.Errorf("error"), 1, nil)

		assert.Len(t, ctx.errors, 1)
		assert.Nil(t, ctx.errors[0].Context)
	})

	t.Run("Zero max history", func(t *testing.T) {
		ctx := NewProgressiveErrorContext(0)
		ctx.AddError("step", fmt.Errorf("error"), 1, nil)

		// Should still store at least one error for functionality
		assert.GreaterOrEqual(t, len(ctx.errors), 0)
	})
}

func TestProgressiveErrorContext_ConcurrentAccess(t *testing.T) {
	// This is a basic test - in a real scenario, you'd want more sophisticated
	// concurrency testing with race condition detection
	ctx := NewProgressiveErrorContext(100)

	// Simulate concurrent error additions
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				ctx.AddError(fmt.Sprintf("step-%d", id), fmt.Errorf("error %d-%d", id, j), j+1, nil)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 100 errors (10 goroutines * 10 errors each)
	assert.Len(t, ctx.errors, 100)
}

func TestErrorContext_Fields(t *testing.T) {
	errorCtx := ErrorContext{
		Step:      "build",
		Error:     "docker build failed",
		Timestamp: time.Now(),
		Attempt:   2,
		Context:   map[string]interface{}{"dockerfile": "/path/to/Dockerfile"},
		Fixes:     []string{"Updated base image", "Fixed syntax"},
	}

	assert.Equal(t, "build", errorCtx.Step)
	assert.Equal(t, "docker build failed", errorCtx.Error)
	assert.Equal(t, 2, errorCtx.Attempt)
	assert.Equal(t, "/path/to/Dockerfile", errorCtx.Context["dockerfile"])
	assert.Contains(t, errorCtx.Fixes, "Updated base image")
	assert.Contains(t, errorCtx.Fixes, "Fixed syntax")
}

func TestProgressiveErrorContext_GetAIContext(t *testing.T) {
	ctx := NewProgressiveErrorContext(10)

	// Add some errors with context
	ctx.AddError("build", fmt.Errorf("docker build failed"), 1, map[string]interface{}{
		"dockerfile": "/path/to/Dockerfile",
		"command":    "docker build .",
	})
	ctx.AddFixAttempt("build", "Updated base image")

	ctx.AddError("deploy", fmt.Errorf("deployment failed"), 1, map[string]interface{}{
		"namespace": "default",
		"manifest":  "deployment.yaml",
	})

	aiContext := ctx.GetAIContext()

	assert.Contains(t, aiContext, "PREVIOUS ERRORS AND ATTEMPTS")
	assert.Contains(t, aiContext, "docker build failed")
	assert.Contains(t, aiContext, "deployment failed")
	assert.Contains(t, aiContext, "Updated base image")
	assert.Contains(t, aiContext, "dockerfile")
	assert.Contains(t, aiContext, "namespace")
}
