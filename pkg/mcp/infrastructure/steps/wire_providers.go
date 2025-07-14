// Package steps provides Wire providers for workflow step implementations.
package steps

import "github.com/Azure/container-kit/pkg/mcp/domain/workflow"

// ProvideStepProvider creates a step provider
func ProvideStepProvider() workflow.StepProvider {
	return NewRegistryStepProvider()
}
