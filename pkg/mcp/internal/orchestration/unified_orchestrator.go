package orchestration

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/rs/zerolog"
)

// UnifiedOrchestrator provides type-safe tool orchestration using the federated registry system
// This replaces the interface{}-based orchestrator with strongly-typed generics
type UnifiedOrchestrator struct {
	federatedRegistry  *FederatedRegistry
	sessionManager     core.ToolSessionManager
	pipelineOperations core.PipelineOperations
	logger             zerolog.Logger
}

// NewUnifiedOrchestrator creates a new type-safe orchestrator
func NewUnifiedOrchestrator(
	pipelineOperations core.PipelineOperations,
	sessionManager core.ToolSessionManager,
	logger zerolog.Logger,
) *UnifiedOrchestrator {
	return &UnifiedOrchestrator{
		federatedRegistry:  NewFederatedRegistry(logger.With().Str("component", "federated_registry").Logger()),
		sessionManager:     sessionManager,
		pipelineOperations: pipelineOperations,
		logger:             logger.With().Str("component", "unified_orchestrator").Logger(),
	}
}

// RegisterBuildTools registers strongly-typed build tools
func (uo *UnifiedOrchestrator) RegisterBuildTools(
	buildTool build.DockerBuildTool,
	pullTool build.DockerPullTool,
	pushTool build.DockerPushTool,
	tagTool build.DockerTagTool,
) error {
	if err := uo.federatedRegistry.RegisterBuildTool("docker_build", buildTool); err != nil {
		return uo.wrapRegistrationError("docker_build", err)
	}

	if err := uo.federatedRegistry.RegisterPullTool("docker_pull", pullTool); err != nil {
		return uo.wrapRegistrationError("docker_pull", err)
	}

	if err := uo.federatedRegistry.RegisterPushTool("docker_push", pushTool); err != nil {
		return uo.wrapRegistrationError("docker_push", err)
	}

	if err := uo.federatedRegistry.RegisterTagTool("docker_tag", tagTool); err != nil {
		return uo.wrapRegistrationError("docker_tag", err)
	}

	uo.logger.Info().Msg("All build tools registered successfully")
	return nil
}

// RegisterDeploymentTools registers strongly-typed deployment tools
func (uo *UnifiedOrchestrator) RegisterDeploymentTools(
	deployTool deploy.KubernetesDeployTool,
	scanTool deploy.SecurityScanTool,
) error {
	if err := uo.federatedRegistry.RegisterDeployTool("kubernetes_deploy", deployTool); err != nil {
		return uo.wrapRegistrationError("kubernetes_deploy", err)
	}

	if err := uo.federatedRegistry.RegisterScanTool("security_scan", scanTool); err != nil {
		return uo.wrapRegistrationError("security_scan", err)
	}

	uo.logger.Info().Msg("All deployment tools registered successfully")
	return nil
}

// ExecuteBuildTool executes a Docker build tool with type safety
func (uo *UnifiedOrchestrator) ExecuteBuildTool(ctx context.Context, params build.DockerBuildParams) (build.DockerBuildResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecuteBuildTool(ctx, "docker_build", params)

	uo.logExecution("docker_build", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecutePullTool executes a Docker pull tool with type safety
func (uo *UnifiedOrchestrator) ExecutePullTool(ctx context.Context, params build.DockerPullParams) (build.DockerPullResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecutePullTool(ctx, "docker_pull", params)

	uo.logExecution("docker_pull", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecutePushTool executes a Docker push tool with type safety
func (uo *UnifiedOrchestrator) ExecutePushTool(ctx context.Context, params build.DockerPushParams) (build.DockerPushResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecutePushTool(ctx, "docker_push", params)

	uo.logExecution("docker_push", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecuteTagTool executes a Docker tag tool with type safety
func (uo *UnifiedOrchestrator) ExecuteTagTool(ctx context.Context, params build.DockerTagParams) (build.DockerTagResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecuteTagTool(ctx, "docker_tag", params)

	uo.logExecution("docker_tag", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecuteDeployTool executes a Kubernetes deploy tool with type safety
func (uo *UnifiedOrchestrator) ExecuteDeployTool(ctx context.Context, params deploy.KubernetesDeployParams) (deploy.KubernetesDeployResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecuteDeployTool(ctx, "kubernetes_deploy", params)

	uo.logExecution("kubernetes_deploy", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecuteScanTool executes a security scan tool with type safety
func (uo *UnifiedOrchestrator) ExecuteScanTool(ctx context.Context, params deploy.SecurityScanParams) (deploy.SecurityScanResult, error) {
	startTime := time.Now()

	result, err := uo.federatedRegistry.ExecuteScanTool(ctx, "security_scan", params)

	uo.logExecution("security_scan", params.SessionID, time.Since(startTime), err)
	return result, err
}

// ExecuteToolByName provides generic tool execution with routing
func (uo *UnifiedOrchestrator) ExecuteToolByName(ctx context.Context, toolName string, paramsJSON []byte) (interface{}, error) {
	// Validate tool exists
	if err := uo.federatedRegistry.ValidateToolExists(toolName); err != nil {
		return nil, err
	}

	// Route execution through federated registry
	return uo.federatedRegistry.RouteToolExecution(ctx, toolName, paramsJSON)
}

// GetToolMetadata returns metadata for a specific tool
func (uo *UnifiedOrchestrator) GetToolMetadata(toolName string) (interface{}, error) {
	registryType, err := uo.federatedRegistry.GetToolRegistry(toolName)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_name":     toolName,
		"registry_type": string(registryType),
		"available":     true,
	}, nil
}

// ListAllTools returns all available tools across all registries
func (uo *UnifiedOrchestrator) ListAllTools() map[string][]string {
	allTools := uo.federatedRegistry.ListAllTools()

	result := make(map[string][]string)
	for registryType, tools := range allTools {
		result[string(registryType)] = tools
	}

	return result
}

// GetOrchestrationStats returns comprehensive orchestration statistics
func (uo *UnifiedOrchestrator) GetOrchestrationStats() OrchestrationStats {
	federatedStats := uo.federatedRegistry.GetFederatedStats()

	return OrchestrationStats{
		TotalRegistries:   federatedStats.RegistryCount,
		TotalTools:        federatedStats.TotalTools,
		TotalExecutions:   federatedStats.TotalUsage,
		BuildToolCount:    federatedStats.BuildStats.TotalTools,
		DeployToolCount:   federatedStats.DeployStats.TotalTools,
		ScanToolCount:     federatedStats.ScanStats.TotalTools,
		FederatedRegistry: true,
		TypeSafe:          true,
	}
}

// OrchestrationStats contains orchestration-wide statistics
type OrchestrationStats struct {
	TotalRegistries   int
	TotalTools        int
	TotalExecutions   int64
	BuildToolCount    int
	DeployToolCount   int
	ScanToolCount     int
	FederatedRegistry bool
	TypeSafe          bool
}

// EnableTool enables a tool across all registries
func (uo *UnifiedOrchestrator) EnableTool(toolName string) error {
	return uo.federatedRegistry.EnableTool(toolName)
}

// DisableTool disables a tool across all registries
func (uo *UnifiedOrchestrator) DisableTool(toolName string) error {
	return uo.federatedRegistry.DisableTool(toolName)
}

// SearchTools searches for tools across all registries
func (uo *UnifiedOrchestrator) SearchTools(query string) map[string][]string {
	results := uo.federatedRegistry.SearchTools(query)

	searchResults := make(map[string][]string)
	for registryType, tools := range results {
		searchResults[string(registryType)] = tools
	}

	return searchResults
}

// ValidateConfiguration validates the orchestrator configuration
func (uo *UnifiedOrchestrator) ValidateConfiguration() error {
	if uo.federatedRegistry == nil {
		return rich.NewError().
			Code("ORCHESTRATOR_NOT_INITIALIZED").
			Message("Federated registry not initialized").
			Type(rich.ErrTypeSystem).
			Severity(rich.SeverityHigh).
			Suggestion("Initialize the orchestrator properly").
			WithLocation().
			Build()
	}

	if uo.sessionManager == nil {
		return rich.NewError().
			Code("SESSION_MANAGER_NOT_SET").
			Message("Session manager not configured").
			Type(rich.ErrTypeSystem).
			Severity(rich.SeverityHigh).
			Suggestion("Set a session manager before using the orchestrator").
			WithLocation().
			Build()
	}

	if uo.pipelineOperations == nil {
		return rich.NewError().
			Code("PIPELINE_OPERATIONS_NOT_SET").
			Message("Pipeline operations not configured").
			Type(rich.ErrTypeSystem).
			Severity(rich.SeverityHigh).
			Suggestion("Set pipeline operations before using the orchestrator").
			WithLocation().
			Build()
	}

	uo.logger.Info().Msg("Orchestrator configuration validated successfully")
	return nil
}

// Helper methods

func (uo *UnifiedOrchestrator) wrapRegistrationError(toolName string, err error) error {
	return rich.NewError().
		Code("TOOL_REGISTRATION_FAILED").
		Message("Failed to register tool in unified orchestrator").
		Type(rich.ErrTypeBusiness).
		Severity(rich.SeverityMedium).
		Cause(err).
		Context("tool_name", toolName).
		Suggestion("Check tool implementation and registry availability").
		WithLocation().
		Build()
}

func (uo *UnifiedOrchestrator) logExecution(toolName, sessionID string, duration time.Duration, err error) {
	if err != nil {
		uo.logger.Error().
			Err(err).
			Str("tool", toolName).
			Str("session_id", sessionID).
			Dur("duration", duration).
			Msg("Tool execution failed")
	} else {
		uo.logger.Info().
			Str("tool", toolName).
			Str("session_id", sessionID).
			Dur("duration", duration).
			Msg("Tool execution completed successfully")
	}
}
