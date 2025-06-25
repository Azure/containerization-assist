package tools

import (
	"fmt"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

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
func (ara *AutoRegistrationAdapter) RegisterAtomicTools(toolRegistry mcptypes.ToolRegistry) error {
	// Tools that implement the unified interface properly
	readyTools := map[string]func() interface{}{
		"atomic_analyze_repository":  func() interface{} { return &AtomicAnalyzeRepositoryTool{} },
		"atomic_build_image":         func() interface{} { return &AtomicBuildImageTool{} },
		"atomic_check_health":        func() interface{} { return &AtomicCheckHealthTool{} },
		"atomic_deploy_kubernetes":   func() interface{} { return &AtomicDeployKubernetesTool{} },
		"atomic_generate_manifests":  func() interface{} { return &AtomicGenerateManifestsTool{} },
		"atomic_pull_image":          func() interface{} { return &AtomicPullImageTool{} },
		"atomic_push_image":          func() interface{} { return &AtomicPushImageTool{} },
		"atomic_scan_image_security": func() interface{} { return &AtomicScanImageSecurityTool{} },
		"atomic_scan_secrets":        func() interface{} { return &AtomicScanSecretsTool{} },
		"atomic_tag_image":           func() interface{} { return &AtomicTagImageTool{} },
		"atomic_validate_dockerfile": func() interface{} { return &AtomicValidateDockerfileTool{} },
	}

	registered := 0
	for name, factory := range readyTools {
		tool := factory()

		// Try to register as unified Tool interface
		if unifiedTool, ok := tool.(mcptypes.Tool); ok {
			err := toolRegistry.Register(name, func() mcptypes.Tool { return unifiedTool })
			if err != nil {
				return fmt.Errorf("failed to register unified tool %s: %w", name, err)
			}
			registered++
			fmt.Printf("🔧 Auto-registered unified tool: %s\n", name)
		} else {
			fmt.Printf("⏳ Tool %s not yet migrated to unified interface\n", name)
		}
	}

	fmt.Printf("✅ Auto-registered %d atomic tools with zero boilerplate\n", registered)
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
