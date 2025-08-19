// Package steps provides a registry-based implementation of StepProvider.
package steps

import (
	"fmt"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

// RegistryStepProvider provides steps from the registry
type RegistryStepProvider struct{}

// NewRegistryStepProvider creates a new registry-based step provider
func NewRegistryStepProvider() workflow.StepProvider {
	return &RegistryStepProvider{}
}

// GetStep retrieves a step by name (consolidated implementation)
func (p *RegistryStepProvider) GetStep(name string) (workflow.Step, error) {
	step, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("step %s not found in registry - ensure it is registered via init()", name)
	}
	return step, nil
}

// ListSteps returns all available step names
func (p *RegistryStepProvider) ListSteps() []string {
	return Names() // Use the existing Names() function from registry
}
