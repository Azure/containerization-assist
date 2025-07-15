package retry

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ProvideRetryServices provides all retry-related services
func ProvideRetryServices() RetryServices {
	return RetryServices{
		DefaultPolicy:      ProvideDefaultRetryPolicy(),
		AggressivePolicy:   ProvideAggressiveRetryPolicy(),
		ConservativePolicy: ProvideConservativeRetryPolicy(),
	}
}

// RetryServices bundles all retry services
type RetryServices struct {
	DefaultPolicy      *RetryPolicy
	AggressivePolicy   *RetryPolicy
	ConservativePolicy *RetryPolicy
}

// ProvideDefaultRetryPolicy provides the default retry policy
func ProvideDefaultRetryPolicy() *RetryPolicy {
	return DefaultRetryPolicy()
}

// ProvideAggressiveRetryPolicy provides an aggressive retry policy
func ProvideAggressiveRetryPolicy() *RetryPolicy {
	return AggressiveRetryPolicy()
}

// ProvideConservativeRetryPolicy provides a conservative retry policy
func ProvideConservativeRetryPolicy() *RetryPolicy {
	return ConservativeRetryPolicy()
}

// ProvideCircuitBreaker provides a circuit breaker with retry
func ProvideCircuitBreaker(policy *RetryPolicy) *CircuitBreaker {
	return NewCircuitBreaker(
		policy,
		5,              // Open after 5 failures
		30*time.Second, // Reset after 30 seconds
	)
}

// ProvideAdaptiveRetryPolicy provides an adaptive retry policy
func ProvideAdaptiveRetryPolicy(basePolicy *RetryPolicy) *AdaptiveRetryPolicy {
	return NewAdaptiveRetryPolicy(basePolicy)
}

// ProvideRetryOrchestrator provides a workflow orchestrator with retry
func ProvideRetryOrchestrator(base workflow.WorkflowOrchestrator, policy *RetryPolicy) workflow.WorkflowOrchestrator {
	return NewRetryDecorator(base, policy)
}
