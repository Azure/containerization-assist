// Package steps provides a registry-based implementation of StepProvider.
package steps

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// RegistryStepProvider provides steps from the registry
type RegistryStepProvider struct{}

// NewRegistryStepProvider creates a new registry-based step provider
func NewRegistryStepProvider() workflow.StepProvider {
	return &RegistryStepProvider{}
}

// getStep is a helper that retrieves a step from registry and panics if not found
func (p *RegistryStepProvider) getStep(name string) workflow.Step {
	step, ok := Get(name)
	if !ok {
		panic(fmt.Sprintf("step %s not found in registry - ensure it is registered via init()", name))
	}
	return step
}

// GetAnalyzeStep returns the analyze step
func (p *RegistryStepProvider) GetAnalyzeStep() workflow.Step {
	return p.getStep("analyze_repository")
}

// GetDockerfileStep returns the dockerfile step
func (p *RegistryStepProvider) GetDockerfileStep() workflow.Step {
	return p.getStep("generate_dockerfile")
}

// GetBuildStep returns the build step
func (p *RegistryStepProvider) GetBuildStep() workflow.Step {
	return p.getStep("build_image")
}

// GetScanStep returns the scan step
func (p *RegistryStepProvider) GetScanStep() workflow.Step {
	return p.getStep("security_scan")
}

// GetTagStep returns the tag step
func (p *RegistryStepProvider) GetTagStep() workflow.Step {
	return p.getStep("tag_image")
}

// GetPushStep returns the push step
func (p *RegistryStepProvider) GetPushStep() workflow.Step {
	return p.getStep("push_image")
}

// GetManifestStep returns the manifest step
func (p *RegistryStepProvider) GetManifestStep() workflow.Step {
	return p.getStep("generate_manifests")
}

// GetClusterStep returns the cluster step
func (p *RegistryStepProvider) GetClusterStep() workflow.Step {
	return p.getStep("setup_cluster")
}

// GetDeployStep returns the deploy step
func (p *RegistryStepProvider) GetDeployStep() workflow.Step {
	return p.getStep("deploy_application")
}

// GetVerifyStep returns the verify step
func (p *RegistryStepProvider) GetVerifyStep() workflow.Step {
	return p.getStep("verify_deployment")
}
