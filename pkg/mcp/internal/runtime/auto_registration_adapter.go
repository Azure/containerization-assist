package runtime

import (
	"fmt"

	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ToolDependencies contains all dependencies needed for tool instantiation
type ToolDependencies struct {
	PipelineOperations mcptypes.PipelineOperations
	SessionManager     mcptypes.ToolSessionManager
	ToolRegistry       interface {
		RegisterTool(name string, tool interface{}) error
	}
	Logger zerolog.Logger
}

// AutoRegistrationAdapter provides a bridge between generated registry and current tool implementations
type AutoRegistrationAdapter struct {
	registry map[string]interface{}
}

// NewAutoRegistrationAdapter creates an adapter for current tool implementations
func NewAutoRegistrationAdapter() *AutoRegistrationAdapter {
	return &AutoRegistrationAdapter{
		registry: make(map[string]interface{}),
	}
}

// OrchestratorRegistryAdapter adapts the orchestrator's registry to the unified interface
type OrchestratorRegistryAdapter struct {
	orchestratorRegistry interface {
		RegisterTool(name string, tool interface{}) error
	}
}

// NewOrchestratorRegistryAdapter creates an adapter for the orchestrator registry
func NewOrchestratorRegistryAdapter(orchestratorRegistry interface {
	RegisterTool(name string, tool interface{}) error
}) *OrchestratorRegistryAdapter {
	return &OrchestratorRegistryAdapter{orchestratorRegistry: orchestratorRegistry}
}

// Register implements mcptypes.ToolRegistry by delegating to the orchestrator registry
func (ora *OrchestratorRegistryAdapter) Register(name string, factory mcptypes.ToolFactory) error {
	tool := factory()
	return ora.orchestratorRegistry.RegisterTool(name, tool)
}

// Unregister is not implemented in the orchestrator registry
func (ora *OrchestratorRegistryAdapter) Unregister(name string) error {
	return fmt.Errorf("unregister not supported by orchestrator registry")
}

// Get is not implemented in the orchestrator registry
func (ora *OrchestratorRegistryAdapter) Get(name string) (mcptypes.ToolFactory, error) {
	return nil, fmt.Errorf("get not supported by orchestrator registry")
}

// List is not implemented in the orchestrator registry
func (ora *OrchestratorRegistryAdapter) List() []string {
	return []string{}
}

// GetMetadata is not implemented in the orchestrator registry
func (ora *OrchestratorRegistryAdapter) GetMetadata() map[string]mcptypes.ToolMetadata {
	return map[string]mcptypes.ToolMetadata{}
}

// RegisterAtomicTools registers all atomic tools that are ready for auto-registration
func (ara *AutoRegistrationAdapter) RegisterAtomicTools(deps ToolDependencies) error {
	// Create atomic tools with proper dependency injection
	atomicTools := ara.createAtomicTools(deps)

	// Register tools with the provided registry
	var registrationErrors []error
	for name, tool := range atomicTools {
		if err := deps.ToolRegistry.RegisterTool(name, tool); err != nil {
			deps.Logger.Error().Err(err).Str("tool", name).Msg("Failed to register atomic tool via auto-registration")
			registrationErrors = append(registrationErrors, fmt.Errorf("failed to register %s: %w", name, err))
		} else {
			deps.Logger.Info().Str("tool", name).Msg("Auto-registered atomic tool successfully")
		}
	}

	if len(registrationErrors) > 0 {
		return fmt.Errorf("auto-registration completed with %d errors: %v", len(registrationErrors), registrationErrors)
	}

	deps.Logger.Info().Int("tools_registered", len(atomicTools)).Msg("Auto-registration completed successfully")
	return nil
}

// GetReadyToolNames returns tools that are ready for auto-registration
func (ara *AutoRegistrationAdapter) GetReadyToolNames() []string {
	return []string{
		"atomic_analyze_repository",
		"atomic_build_image",
		"atomic_check_health",
		"atomic_deploy_kubernetes",
		"atomic_generate_manifests",
		"atomic_pull_image",
		"atomic_push_image",
		"atomic_scan_image_security",
		"atomic_scan_secrets",
		"atomic_tag_image",
		"atomic_validate_dockerfile",
	}
}

// GetPendingToolNames returns tools that need interface migration
func (ara *AutoRegistrationAdapter) GetPendingToolNames() []string {
	return []string{
		// All atomic tools now implement the unified mcptypes.Tool interface
	}
}

// createAtomicTools instantiates all atomic tools with proper dependencies
func (ara *AutoRegistrationAdapter) createAtomicTools(deps ToolDependencies) map[string]interface{} {
	return map[string]interface{}{
		"atomic_analyze_repository": analyze.NewAtomicAnalyzeRepositoryTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_build_image": build.NewAtomicBuildImageTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_generate_manifests": deploy.NewAtomicGenerateManifestsTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_deploy_kubernetes": deploy.NewAtomicDeployKubernetesTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_scan_image_security": scan.NewAtomicScanImageSecurityTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_scan_secrets": scan.NewAtomicScanSecretsTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_pull_image": build.NewAtomicPullImageTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_push_image": build.NewAtomicPushImageTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
		"atomic_tag_image": build.NewAtomicTagImageTool(
			deps.PipelineOperations,
			deps.SessionManager,
			deps.Logger,
		),
	}
}
