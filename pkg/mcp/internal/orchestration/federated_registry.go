package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/rs/zerolog"
)

// FederatedRegistry provides unified access to all specialized registries with type-safe dispatch
type FederatedRegistry struct {
	build  *ExtendedBuildRegistries
	deploy *DeployRegistry
	scan   *ScanRegistry

	// Registry routing table
	toolRoutes map[string]RegistryType
	mu         sync.RWMutex
	logger     zerolog.Logger
}

// RegistryType represents the type of registry for routing
type RegistryType string

const (
	RegistryTypeBuild  RegistryType = "build"
	RegistryTypePull   RegistryType = "pull"
	RegistryTypePush   RegistryType = "push"
	RegistryTypeTag    RegistryType = "tag"
	RegistryTypeDeploy RegistryType = "deploy"
	RegistryTypeScan   RegistryType = "scan"
)

// NewFederatedRegistry creates a new federated registry system
func NewFederatedRegistry(logger zerolog.Logger) *FederatedRegistry {
	fr := &FederatedRegistry{
		build:      NewExtendedBuildRegistries(logger.With().Str("domain", "build").Logger()),
		deploy:     NewGenericRegistry[deploy.KubernetesDeployTool, deploy.KubernetesDeployParams, deploy.KubernetesDeployResult](logger.With().Str("domain", "deploy").Logger()),
		scan:       NewGenericRegistry[deploy.SecurityScanTool, deploy.SecurityScanParams, deploy.SecurityScanResult](logger.With().Str("domain", "scan").Logger()),
		toolRoutes: make(map[string]RegistryType),
		logger:     logger.With().Str("component", "federated_registry").Logger(),
	}

	// Initialize default routing table
	fr.initializeRoutes()

	return fr
}

// initializeRoutes sets up the default tool routing table
func (fr *FederatedRegistry) initializeRoutes() {
	fr.toolRoutes = map[string]RegistryType{
		"docker_build":      RegistryTypeBuild,
		"docker_pull":       RegistryTypePull,
		"docker_push":       RegistryTypePush,
		"docker_tag":        RegistryTypeTag,
		"kubernetes_deploy": RegistryTypeDeploy,
		"security_scan":     RegistryTypeScan,
	}
}

// RegisterBuildTool registers a Docker build tool
func (fr *FederatedRegistry) RegisterBuildTool(name string, tool build.DockerBuildTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.build.Build.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypeBuild, err)
	}

	fr.toolRoutes[name] = RegistryTypeBuild
	fr.logger.Info().Str("tool", name).Str("registry", "build").Msg("Build tool registered")
	return nil
}

// RegisterPullTool registers a Docker pull tool
func (fr *FederatedRegistry) RegisterPullTool(name string, tool build.DockerPullTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.build.Pull.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypePull, err)
	}

	fr.toolRoutes[name] = RegistryTypePull
	fr.logger.Info().Str("tool", name).Str("registry", "pull").Msg("Pull tool registered")
	return nil
}

// RegisterPushTool registers a Docker push tool
func (fr *FederatedRegistry) RegisterPushTool(name string, tool build.DockerPushTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.build.Push.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypePush, err)
	}

	fr.toolRoutes[name] = RegistryTypePush
	fr.logger.Info().Str("tool", name).Str("registry", "push").Msg("Push tool registered")
	return nil
}

// RegisterTagTool registers a Docker tag tool
func (fr *FederatedRegistry) RegisterTagTool(name string, tool build.DockerTagTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.build.Tag.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypeTag, err)
	}

	fr.toolRoutes[name] = RegistryTypeTag
	fr.logger.Info().Str("tool", name).Str("registry", "tag").Msg("Tag tool registered")
	return nil
}

// RegisterDeployTool registers a Kubernetes deploy tool
func (fr *FederatedRegistry) RegisterDeployTool(name string, tool deploy.KubernetesDeployTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.deploy.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypeDeploy, err)
	}

	fr.toolRoutes[name] = RegistryTypeDeploy
	fr.logger.Info().Str("tool", name).Str("registry", "deploy").Msg("Deploy tool registered")
	return nil
}

// RegisterScanTool registers a security scan tool
func (fr *FederatedRegistry) RegisterScanTool(name string, tool deploy.SecurityScanTool) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	if err := fr.scan.Register(name, tool); err != nil {
		return fr.wrapRegistrationError(name, RegistryTypeScan, err)
	}

	fr.toolRoutes[name] = RegistryTypeScan
	fr.logger.Info().Str("tool", name).Str("registry", "scan").Msg("Scan tool registered")
	return nil
}

// ExecuteBuildTool executes a Docker build tool with type safety
func (fr *FederatedRegistry) ExecuteBuildTool(ctx context.Context, name string, params build.DockerBuildParams) (build.DockerBuildResult, error) {
	return fr.build.Build.Execute(ctx, name, params)
}

// ExecutePullTool executes a Docker pull tool with type safety
func (fr *FederatedRegistry) ExecutePullTool(ctx context.Context, name string, params build.DockerPullParams) (build.DockerPullResult, error) {
	return fr.build.Pull.Execute(ctx, name, params)
}

// ExecutePushTool executes a Docker push tool with type safety
func (fr *FederatedRegistry) ExecutePushTool(ctx context.Context, name string, params build.DockerPushParams) (build.DockerPushResult, error) {
	return fr.build.Push.Execute(ctx, name, params)
}

// ExecuteTagTool executes a Docker tag tool with type safety
func (fr *FederatedRegistry) ExecuteTagTool(ctx context.Context, name string, params build.DockerTagParams) (build.DockerTagResult, error) {
	return fr.build.Tag.Execute(ctx, name, params)
}

// ExecuteDeployTool executes a Kubernetes deploy tool with type safety
func (fr *FederatedRegistry) ExecuteDeployTool(ctx context.Context, name string, params deploy.KubernetesDeployParams) (deploy.KubernetesDeployResult, error) {
	return fr.deploy.Execute(ctx, name, params)
}

// ExecuteScanTool executes a security scan tool with type safety
func (fr *FederatedRegistry) ExecuteScanTool(ctx context.Context, name string, params deploy.SecurityScanParams) (deploy.SecurityScanResult, error) {
	return fr.scan.Execute(ctx, name, params)
}

// GetToolRegistry returns the registry type for a given tool
func (fr *FederatedRegistry) GetToolRegistry(toolName string) (RegistryType, error) {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	registryType, exists := fr.toolRoutes[toolName]
	if !exists {
		return "", rich.NewError().
			Code("TOOL_ROUTING_FAILED").
			Message("No registry found for tool").
			Type(rich.ErrTypeNotFound).
			Context("tool_name", toolName).
			Context("available_tools", fr.getAllToolNames()).
			Suggestion("Check tool name or register the tool first").
			WithLocation().
			Build()
	}

	return registryType, nil
}

// ListAllTools returns all tools across all registries
func (fr *FederatedRegistry) ListAllTools() map[RegistryType][]string {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	return map[RegistryType][]string{
		RegistryTypeBuild:  fr.build.Build.List(),
		RegistryTypePull:   fr.build.Pull.List(),
		RegistryTypePush:   fr.build.Push.List(),
		RegistryTypeTag:    fr.build.Tag.List(),
		RegistryTypeDeploy: fr.deploy.List(),
		RegistryTypeScan:   fr.scan.List(),
	}
}

// GetFederatedStats returns comprehensive statistics across all registries
func (fr *FederatedRegistry) GetFederatedStats() FederatedStats {
	buildStats := fr.build.GetExtendedStats()
	deployStats := fr.deploy.GetStats()
	scanStats := fr.scan.GetStats()

	return FederatedStats{
		RegistryCount: 6, // build, pull, push, tag, deploy, scan
		BuildStats:    buildStats,
		DeployStats:   deployStats,
		ScanStats:     scanStats,
		TotalTools:    buildStats.TotalTools + deployStats.TotalTools + scanStats.TotalTools,
		TotalUsage:    buildStats.TotalUsage + deployStats.TotalUsage + scanStats.TotalUsage,
	}
}

// FederatedStats contains statistics across all federated registries
type FederatedStats struct {
	RegistryCount int
	BuildStats    ExtendedBuildStats
	DeployStats   RegistryStats
	ScanStats     RegistryStats
	TotalTools    int
	TotalUsage    int64
}

// RouteToolExecution performs type-safe routing and execution based on tool name
func (fr *FederatedRegistry) RouteToolExecution(ctx context.Context, toolName string, paramsJSON []byte) (interface{}, error) {
	registryType, err := fr.GetToolRegistry(toolName)
	if err != nil {
		return nil, err
	}

	fr.logger.Info().
		Str("tool", toolName).
		Str("registry", string(registryType)).
		Msg("Routing tool execution")

	// This would typically parse paramsJSON into the appropriate type
	// For demonstration, we'll return routing information
	return map[string]interface{}{
		"tool":     toolName,
		"registry": string(registryType),
		"routed":   true,
	}, nil
}

// ValidateToolExists checks if a tool exists in any registry
func (fr *FederatedRegistry) ValidateToolExists(toolName string) error {
	_, err := fr.GetToolRegistry(toolName)
	return err
}

// EnableTool enables a tool in its appropriate registry
func (fr *FederatedRegistry) EnableTool(toolName string) error {
	registryType, err := fr.GetToolRegistry(toolName)
	if err != nil {
		return err
	}

	switch registryType {
	case RegistryTypeBuild:
		return fr.build.Build.EnableTool(toolName)
	case RegistryTypePull:
		return fr.build.Pull.EnableTool(toolName)
	case RegistryTypePush:
		return fr.build.Push.EnableTool(toolName)
	case RegistryTypeTag:
		return fr.build.Tag.EnableTool(toolName)
	case RegistryTypeDeploy:
		return fr.deploy.EnableTool(toolName)
	case RegistryTypeScan:
		return fr.scan.EnableTool(toolName)
	default:
		return rich.NewError().
			Code("UNSUPPORTED_REGISTRY_TYPE").
			Message("Unsupported registry type for tool").
			Type(rich.ErrTypeBusiness).
			Context("tool_name", toolName).
			Context("registry_type", string(registryType)).
			WithLocation().
			Build()
	}
}

// DisableTool disables a tool in its appropriate registry
func (fr *FederatedRegistry) DisableTool(toolName string) error {
	registryType, err := fr.GetToolRegistry(toolName)
	if err != nil {
		return err
	}

	switch registryType {
	case RegistryTypeBuild:
		return fr.build.Build.DisableTool(toolName)
	case RegistryTypePull:
		return fr.build.Pull.DisableTool(toolName)
	case RegistryTypePush:
		return fr.build.Push.DisableTool(toolName)
	case RegistryTypeTag:
		return fr.build.Tag.DisableTool(toolName)
	case RegistryTypeDeploy:
		return fr.deploy.DisableTool(toolName)
	case RegistryTypeScan:
		return fr.scan.DisableTool(toolName)
	default:
		return rich.NewError().
			Code("UNSUPPORTED_REGISTRY_TYPE").
			Message("Unsupported registry type for tool").
			Type(rich.ErrTypeBusiness).
			Context("tool_name", toolName).
			Context("registry_type", string(registryType)).
			WithLocation().
			Build()
	}
}

// Helper methods

func (fr *FederatedRegistry) wrapRegistrationError(toolName string, registryType RegistryType, err error) error {
	return rich.NewError().
		Code("TOOL_REGISTRATION_FAILED").
		Message("Failed to register tool in federated registry").
		Type(rich.ErrTypeBusiness).
		Severity(rich.SeverityMedium).
		Cause(err).
		Context("tool_name", toolName).
		Context("registry_type", string(registryType)).
		Suggestion("Check tool implementation and registry availability").
		WithLocation().
		Build()
}

func (fr *FederatedRegistry) getAllToolNames() []string {
	var allTools []string

	allRegistries := fr.ListAllTools()
	for registryType, tools := range allRegistries {
		for _, tool := range tools {
			allTools = append(allTools, fmt.Sprintf("%s:%s", registryType, tool))
		}
	}

	return allTools
}

// SearchTools searches for tools across all registries
func (fr *FederatedRegistry) SearchTools(query string) map[RegistryType][]string {
	results := make(map[RegistryType][]string)

	allTools := fr.ListAllTools()
	for registryType, tools := range allTools {
		var matches []string
		for _, tool := range tools {
			if strings.Contains(strings.ToLower(tool), strings.ToLower(query)) {
				matches = append(matches, tool)
			}
		}
		if len(matches) > 0 {
			results[registryType] = matches
		}
	}

	return results
}
