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

// RegisterAtomicTools registers all atomic tools that are ready for auto-registration
func (ara *AutoRegistrationAdapter) RegisterAtomicTools(toolRegistry mcptypes.ToolRegistry) error {
	// Tools that implement the unified interface properly
	readyTools := map[string]func() interface{}{
		"atomic_analyze_repository":  func() interface{} { return &AtomicAnalyzeRepositoryTool{} },
		"atomic_build_image":         func() interface{} { return &AtomicBuildImageTool{} },
		"atomic_deploy_kubernetes":   func() interface{} { return &AtomicDeployKubernetesTool{} },
		"atomic_pull_image":          func() interface{} { return &AtomicPullImageTool{} },
		"atomic_push_image":          func() interface{} { return &AtomicPushImageTool{} },
		"atomic_scan_image_security": func() interface{} { return &AtomicScanImageSecurityTool{} },
		"atomic_tag_image":           func() interface{} { return &AtomicTagImageTool{} },
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
		"atomic_deploy_kubernetes",
		"atomic_pull_image",
		"atomic_push_image",
		"atomic_scan_image_security",
		"atomic_tag_image",
	}
}

// GetPendingToolNames returns tools that need interface migration
func (ara *AutoRegistrationAdapter) GetPendingToolNames() []string {
	return []string{
		"atomic_check_health",         // Missing GetMetadata
		"atomic_generate_manifests",   // Missing GetMetadata  
		"atomic_scan_secrets",         // Missing GetMetadata
		"atomic_validate_dockerfile",  // Missing GetMetadata
	}
}