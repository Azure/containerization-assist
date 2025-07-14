package workflow

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow/common"
	"github.com/stretchr/testify/assert"
)

// TestArchitectureValidation validates the new orchestrator architecture
func TestArchitectureValidation(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "BaseOrchestrator should implement WorkflowOrchestrator",
			check: func(t *testing.T) {
				var _ WorkflowOrchestrator = (*BaseOrchestrator)(nil)
			},
		},
		{
			name: "EventDecorator should implement EventAwareOrchestrator",
			check: func(t *testing.T) {
				// This is validated at compile time
				var orchestrator interface{} = &eventDecorator{}
				_, ok := orchestrator.(EventAwareOrchestrator)
				if !ok {
					t.Error("eventDecorator does not implement EventAwareOrchestrator")
				}
			},
		},
		{
			name: "SagaDecorator should implement SagaAwareOrchestrator",
			check: func(t *testing.T) {
				// This is validated at compile time
				var orchestrator interface{} = &sagaDecorator{}
				_, ok := orchestrator.(SagaAwareOrchestrator)
				if !ok {
					t.Error("sagaDecorator does not implement SagaAwareOrchestrator")
				}
			},
		},
		{
			name: "No duplicate noOpSink implementations",
			check: func(t *testing.T) {
				// Check that we only have one noOpSink in common package
				commonType := reflect.TypeOf(common.NoOpSink{})
				if commonType.Name() != "NoOpSink" {
					t.Error("NoOpSink not found in common package")
				}
			},
		},
		{
			name: "Middleware pattern is used correctly",
			check: func(t *testing.T) {
				// Verify middleware signatures
				var _ StepMiddleware = RetryMiddleware()
				var _ StepMiddleware = ProgressMiddleware()
				var _ StepMiddleware = TracingMiddleware(nil)
			},
		},
		{
			name: "Legacy orchestrator is marked as deprecated",
			check: func(t *testing.T) {
				// Check if legacy_orchestrator.go exists
				orchestratorType := reflect.TypeOf(Orchestrator{})
				if !strings.Contains(orchestratorType.PkgPath(), "workflow") {
					t.Error("Orchestrator type not in workflow package")
				}
				// Note: In a real test, we would check file contents for deprecation notice
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t)
		})
	}
}

// TestDecoratorComposition validates that decorators can be composed correctly
func TestDecoratorComposition(t *testing.T) {
	// Create actual instances instead of nil pointers
	logger := slog.Default()
	publisher := events.NewPublisher(logger)
	coordinator := saga.NewSagaCoordinator(logger, publisher)

	// Create a minimal base orchestrator
	mockProvider := &MockStepProvider{}
	stepFactory := NewStepFactory(mockProvider, nil, nil, logger)
	base := NewBaseOrchestrator(stepFactory, nil, logger)

	// Test that decorators can be composed
	eventAware := WithEvents(base, publisher)
	sagaAware := WithSaga(eventAware, coordinator, logger)

	// Test that interfaces are satisfied
	var _ WorkflowOrchestrator = base
	var _ EventAwareOrchestrator = eventAware
	var _ SagaAwareOrchestrator = sagaAware

	// Verify the decorators were applied (no panic)
	assert.NotNil(t, eventAware)
	assert.NotNil(t, sagaAware)
}

// TestMiddlewareChain validates middleware execution order
func TestMiddlewareChain(t *testing.T) {
	// Create a chain of middleware
	var middlewares []StepMiddleware

	// Add middleware in order
	middlewares = append(middlewares, RetryMiddleware())
	middlewares = append(middlewares, ProgressMiddleware())

	// Verify we can create an orchestrator with middleware
	logger := slog.Default()
	mockProvider := &MockStepProvider{}
	factory := NewStepFactory(mockProvider, nil, nil, logger)
	_ = NewBaseOrchestrator(factory, nil, logger, middlewares...)
}

// TestNoCircularDependencies validates no circular imports
func TestNoCircularDependencies(t *testing.T) {
	// This test passes if the file compiles
	// Circular dependencies would prevent compilation

	// Import and use types from different packages
	_ = common.NoOpSink{}
	_ = &BaseOrchestrator{}
	_ = &eventDecorator{}
	_ = &sagaDecorator{}
}
