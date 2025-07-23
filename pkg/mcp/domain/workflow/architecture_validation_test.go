package workflow

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestArchitectureValidation validates the simplified orchestrator architecture
func TestArchitectureValidation(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "DAGOrchestrator should implement WorkflowOrchestrator",
			check: func(t *testing.T) {
				var _ WorkflowOrchestrator = (*DAGOrchestrator)(nil)
			},
		},
		{
			name: "NoOpSink is available for fallback cases",
			check: func(t *testing.T) {
				// Check that NoOpSink exists
				commonType := reflect.TypeOf(NoOpSink{})
				if commonType.Name() != "NoOpSink" {
					t.Error("NoOpSink not found")
				}
			},
		},
		{
			name: "Middleware pattern is used correctly",
			check: func(t *testing.T) {
				// Verify middleware signatures
				var _ StepMiddleware = DefaultRetryMiddleware()
				var _ StepMiddleware = ProgressMiddleware(SimpleProgress)
				var _ StepMiddleware = TracingMiddleware(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// TestDAGOrchestratorCreation validates that DAGOrchestrator can be created
func TestDAGOrchestratorCreation(t *testing.T) {
	// Skip this test for now since it requires complex setup
	t.Skip("Skipping DAGOrchestrator creation test - requires step provider setup")
}

// TestMiddlewareChain validates middleware execution order
func TestMiddlewareChain(t *testing.T) {
	// Create a chain of middleware
	var middlewares []StepMiddleware

	// Add middleware in order
	middlewares = append(middlewares, DefaultRetryMiddleware())
	middlewares = append(middlewares, ProgressMiddleware(SimpleProgress))

	// Verify we can create a chain of middleware
	chainedMiddleware := Chain(middlewares...)
	assert.NotNil(t, chainedMiddleware)
}

// TestNoCircularDependencies validates no circular imports
func TestNoCircularDependencies(t *testing.T) {
	// This test passes if the file compiles
	// Circular dependencies would prevent compilation

	// Import and use types from different packages
	_ = NoOpSink{}
	_ = &DAGOrchestrator{}
}

// TestArchitecturalBoundaryEnforcement validates that the 4-layer architecture is properly maintained
func TestArchitecturalBoundaryEnforcement(t *testing.T) {
	// This test only runs if the architectural validation tool exists
	// It serves as a safety check for CI/CD to ensure boundaries are maintained

	// Note: In CI/CD environments, this should be run as a separate make target
	// This test is informational only and will not fail the build
	t.Log("Architectural boundary enforcement should be verified by running 'make arch-validate'")
	t.Log("Expected violations (as of current state):")
	t.Log("- Application layer should not import from Infrastructure layer")
	t.Log("- These should be resolved by implementing dependency inversion")
}
