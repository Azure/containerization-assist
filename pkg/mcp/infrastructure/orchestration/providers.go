// Package orchestration provides unified dependency injection for container orchestration services
package orchestration

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/container"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/orchestration/steps"
	"github.com/google/wire"
)

// OrchestrationProviders provides all orchestration domain dependencies
var OrchestrationProviders = wire.NewSet(
	// Container management - using existing constructor
	container.NewDockerContainerManager,

	// Kubernetes deployment - using existing constructor
	kubernetes.NewKubernetesDeploymentManager,

	// Workflow steps - using existing constructor
	steps.NewRegistryStepProvider,

	// Interface bindings would go here if needed
)
