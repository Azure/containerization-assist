package runtime

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
)

type ToolRegistryUpdates struct {
	logger logging.Standards
}

func NewToolRegistryUpdates(logger logging.Standards) *ToolRegistryUpdates {
	return &ToolRegistryUpdates{
		logger: logger.WithComponent("tool_registry_updates"),
	}
}
func (t *ToolRegistryUpdates) GetUpdatedToolMap() map[string]string {
	return map[string]string{

		"analyze_repository":  "analyze_repository_atomic",
		"build_image":         "docker_build",
		"generate_manifests":  "deploy_kubernetes_atomic",
		"validate_deployment": "deploy_kubernetes_atomic",
		"generate_dockerfile": "generate_dockerfile",
		"push_image":          "docker_operations",
		"pull_image":          "docker_operations",
		"tag_image":           "docker_operations",
		"get_job_status":      "get_job_status",
		"list_sessions":       "list_sessions",
		"delete_session":      "delete_session",
		"check_health":        "check_health_atomic",
	}
}
