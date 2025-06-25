package runtime

import (
	"github.com/rs/zerolog"
)

// ToolRegistryUpdates provides a centralized place to update tool registrations
// to use atomic tools instead of AI-powered ones
type ToolRegistryUpdates struct {
	logger zerolog.Logger
}

// NewToolRegistryUpdates creates a new tool registry updater
func NewToolRegistryUpdates(logger zerolog.Logger) *ToolRegistryUpdates {
	return &ToolRegistryUpdates{
		logger: logger.With().Str("component", "tool_registry_updates").Logger(),
	}
}

// GetUpdatedToolMap returns the updated tool mappings that redirect to atomic tools
func (t *ToolRegistryUpdates) GetUpdatedToolMap() map[string]string {
	return map[string]string{
		// Core containerization tools now use atomic implementations
		"analyze_repository":  "analyze_repository_atomic",
		"build_image":         "build_image_atomic",
		"generate_manifests":  "deploy_kubernetes_atomic", // Combined into deploy
		"validate_deployment": "deploy_kubernetes_atomic", // Combined into deploy

		// These tools remain unchanged as they don't use AI
		"generate_dockerfile": "generate_dockerfile", // Template-based, no AI
		"push_image":          "push_image",          // Simple registry operation
		"get_job_status":      "get_job_status",      // Job tracking
		"list_sessions":       "list_sessions",       // Session management
		"delete_session":      "delete_session",      // Session cleanup
		"get_server_health":   "get_server_health",   // Health check

		// New atomic tool
		"check_health": "check_health_atomic", // Health checking
	}
}
