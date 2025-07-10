package commands

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/deploy"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ConsolidatedDeployCommand consolidates all deploy tool functionality into a single command
// This replaces the 44 files in pkg/mcp/tools/deploy/ with a unified implementation
type ConsolidatedDeployCommand struct {
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	kubernetesClient kubernetes.Client
	logger           *slog.Logger
}

// NewConsolidatedDeployCommand creates a new consolidated deploy command
func NewConsolidatedDeployCommand(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	kubernetesClient kubernetes.Client,
	logger *slog.Logger,
) *ConsolidatedDeployCommand {
	return &ConsolidatedDeployCommand{
		sessionStore:     sessionStore,
		sessionState:     sessionState,
		kubernetesClient: kubernetesClient,
		logger:           logger,
	}
}

// Execute performs deploy operations with full functionality from original tools
func (cmd *ConsolidatedDeployCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Extract and validate input parameters
	deployRequest, err := cmd.parseDeployInput(input)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("failed to parse deploy input").
			Cause(err).
			Build()
	}

	// Validate using domain rules
	if validationErrors := cmd.validateDeployRequest(deployRequest); len(validationErrors) > 0 {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeValidationFailed).
			Message("deploy request validation failed").
			Context("validation_errors", validationErrors).
			Build()
	}

	// Get workspace directory for the session
	workspaceDir, err := cmd.getSessionWorkspace(ctx, deployRequest.SessionID)
	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to get session workspace").
			Cause(err).
			Build()
	}

	// Execute deploy operation based on operation type
	var deployResult *deploy.DeploymentResult
	switch deployRequest.Operation {
	case "deploy":
		deployResult, err = cmd.executeDeployment(ctx, deployRequest, workspaceDir)
	case "generate_manifests":
		deployResult, err = cmd.executeGenerateManifests(ctx, deployRequest, workspaceDir)
	case "rollback":
		deployResult, err = cmd.executeRollback(ctx, deployRequest, workspaceDir)
	case "health_check":
		deployResult, err = cmd.executeHealthCheck(ctx, deployRequest, workspaceDir)
	default:
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message(fmt.Sprintf("unsupported operation: %s", deployRequest.Operation)).
			Build()
	}

	if err != nil {
		return api.ToolOutput{}, errors.NewError().
			Code(errors.CodeInternalError).
			Message("deploy operation failed").
			Cause(err).
			Build()
	}

	// Update session state with deploy results
	if err := cmd.updateSessionState(ctx, deployRequest.SessionID, deployResult); err != nil {
		cmd.logger.Warn("failed to update session state", "error", err)
	}

	// Create consolidated response
	response := cmd.createDeployResponse(deployResult, time.Since(startTime))

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"deploy_result": response,
		},
	}, nil
}

// parseDeployInput extracts and validates deploy parameters from tool input
func (cmd *ConsolidatedDeployCommand) parseDeployInput(input api.ToolInput) (*DeployRequest, error) {
	// Extract operation type
	operation := getStringParam(input.Data, "operation", "deploy")

	// Extract common parameters
	request := &DeployRequest{
		SessionID:   input.SessionID,
		Operation:   operation,
		Name:        getStringParam(input.Data, "name", ""),
		Namespace:   getStringParam(input.Data, "namespace", "default"),
		Image:       getStringParam(input.Data, "image", ""),
		Tag:         getStringParam(input.Data, "tag", "latest"),
		Replicas:    getIntParam(input.Data, "replicas", 1),
		Environment: deploy.Environment(getStringParam(input.Data, "environment", "development")),
		Strategy:    deploy.DeploymentStrategy(getStringParam(input.Data, "strategy", "rolling")),
		DeployOptions: DeployOptions{
			ManifestPath:      getStringParam(input.Data, "manifest_path", ""),
			DryRun:            getBoolParam(input.Data, "dry_run", false),
			WaitForReady:      getBoolParam(input.Data, "wait_for_ready", true),
			Timeout:           getDurationParam(input.Data, "timeout", 5*time.Minute),
			Force:             getBoolParam(input.Data, "force", false),
			IncludeIngress:    getBoolParam(input.Data, "include_ingress", false),
			IngressHost:       getStringParam(input.Data, "ingress_host", ""),
			CustomLabels:      getStringMapParam(input.Data, "labels"),
			CustomAnnotations: getStringMapParam(input.Data, "annotations"),
		},
		ResourceRequirements: ResourceRequirements{
			CPU:         getStringParam(input.Data, "cpu_request", "100m"),
			Memory:      getStringParam(input.Data, "memory_request", "128Mi"),
			CPULimit:    getStringParam(input.Data, "cpu_limit", "500m"),
			MemoryLimit: getStringParam(input.Data, "memory_limit", "512Mi"),
		},
		Ports:     cmd.parsePortsFromInput(input.Data),
		CreatedAt: time.Now(),
	}

	// Validate required fields based on operation
	if err := cmd.validateOperationParams(request); err != nil {
		return nil, err
	}

	return request, nil
}

// validateOperationParams validates operation-specific parameters
func (cmd *ConsolidatedDeployCommand) validateOperationParams(request *DeployRequest) error {
	switch request.Operation {
	case "deploy":
		if request.Name == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("name is required for deploy operation").
				WithLocation().
				Build()
		}
		if request.Image == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("image is required for deploy operation").
				WithLocation().
				Build()
		}
	case "generate_manifests":
		if request.Name == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("name is required for generate_manifests operation").
				WithLocation().
				Build()
		}
		if request.Image == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("image is required for generate_manifests operation").
				WithLocation().
				Build()
		}
	case "rollback":
		if request.Name == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("name is required for rollback operation").
				WithLocation().
				Build()
		}
	case "health_check":
		if request.Name == "" {
			return errors.NewError().
				Code(errors.CodeValidationFailed).
				Type(errors.ErrTypeValidation).
				Message("name is required for health_check operation").
				WithLocation().
				Build()
		}
	}
	return nil
}

// validateDeployRequest validates deploy request using domain rules
func (cmd *ConsolidatedDeployCommand) validateDeployRequest(request *DeployRequest) []ValidationError {
	var errors []ValidationError

	// Session ID validation
	if request.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Operation validation
	validOperations := []string{"deploy", "generate_manifests", "rollback", "health_check"}
	if !slices.Contains(validOperations, request.Operation) {
		errors = append(errors, ValidationError{
			Field:   "operation",
			Message: fmt.Sprintf("operation must be one of: %s", strings.Join(validOperations, ", ")),
			Code:    "INVALID_OPERATION",
		})
	}

	// Name validation
	if request.Name != "" {
		if !isValidKubernetesName(request.Name) {
			errors = append(errors, ValidationError{
				Field:   "name",
				Message: "invalid Kubernetes name format",
				Code:    "INVALID_NAME",
			})
		}
	}

	// Namespace validation
	if request.Namespace != "" {
		if !isValidKubernetesName(request.Namespace) {
			errors = append(errors, ValidationError{
				Field:   "namespace",
				Message: "invalid Kubernetes namespace format",
				Code:    "INVALID_NAMESPACE",
			})
		}
	}

	// Image validation
	if request.Image != "" {
		if !isValidImageName(request.Image) {
			errors = append(errors, ValidationError{
				Field:   "image",
				Message: "invalid image name format",
				Code:    "INVALID_IMAGE",
			})
		}
	}

	// Replicas validation
	if request.Replicas < 0 || request.Replicas > 100 {
		errors = append(errors, ValidationError{
			Field:   "replicas",
			Message: "replicas must be between 0 and 100",
			Code:    "INVALID_REPLICAS",
		})
	}

	return errors
}

// getSessionWorkspace retrieves the workspace directory for a session
func (cmd *ConsolidatedDeployCommand) getSessionWorkspace(ctx context.Context, sessionID string) (string, error) {
	sessionMetadata, err := cmd.sessionState.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSession).
			Messagef("failed to get session metadata: %w", err).
			WithLocation().
			Build()
	}

	workspaceDir, ok := sessionMetadata["workspace_dir"].(string)
	if !ok || workspaceDir == "" {
		return "", errors.NewError().
			Code(errors.CodeNotFound).
			Type(errors.ErrTypeNotFound).
			Messagef("workspace directory not found for session %s", sessionID).
			WithLocation().
			Build()
	}

	return workspaceDir, nil
}

// executeDeployment performs Kubernetes deployment operation
func (cmd *ConsolidatedDeployCommand) executeDeployment(ctx context.Context, request *DeployRequest, workspaceDir string) (*deploy.DeploymentResult, error) {
	// Create deployment request from domain
	deploymentRequest := &deploy.DeploymentRequest{
		ID:          fmt.Sprintf("deploy-%d", time.Now().Unix()),
		SessionID:   request.SessionID,
		Name:        request.Name,
		Namespace:   request.Namespace,
		Environment: request.Environment,
		Strategy:    request.Strategy,
		Image:       request.Image,
		Tag:         request.Tag,
		Replicas:    request.Replicas,
		Resources: deploy.ResourceRequirements{
			CPU: deploy.ResourceSpec{
				Request: request.ResourceRequirements.CPU,
				Limit:   request.ResourceRequirements.CPULimit,
			},
			Memory: deploy.ResourceSpec{
				Request: request.ResourceRequirements.Memory,
				Limit:   request.ResourceRequirements.MemoryLimit,
			},
		},
		Configuration: deploy.DeploymentConfiguration{
			Environment: request.DeployOptions.CustomLabels,
			Ports:       cmd.convertToDomainPorts(request.Ports),
		},
		Options: deploy.DeploymentOptions{
			DryRun:       request.DeployOptions.DryRun,
			Timeout:      request.DeployOptions.Timeout,
			WaitForReady: request.DeployOptions.WaitForReady,
			Labels:       request.DeployOptions.CustomLabels,
			Annotations:  request.DeployOptions.CustomAnnotations,
		},
		CreatedAt: time.Now(),
	}

	// Execute deployment using Kubernetes client
	result, err := cmd.performKubernetesDeployment(ctx, deploymentRequest, workspaceDir)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeKubernetesAPIError).
			Type(errors.ErrTypeKubernetes).
			Messagef("deployment execution failed: %w", err).
			WithLocation().
			Build()
	}

	return result, nil
}

// executeGenerateManifests performs Kubernetes manifest generation operation
func (cmd *ConsolidatedDeployCommand) executeGenerateManifests(ctx context.Context, request *DeployRequest, workspaceDir string) (*deploy.DeploymentResult, error) {
	// Create manifest generation request from domain
	manifestRequest := &deploy.ManifestGenerationRequest{
		ID:           fmt.Sprintf("manifest-%d", time.Now().Unix()),
		SessionID:    request.SessionID,
		TemplateType: deploy.TemplateTypeDeployment,
		Configuration: deploy.DeploymentConfiguration{
			Environment: request.DeployOptions.CustomLabels,
			Ports:       cmd.convertToDomainPorts(request.Ports),
		},
		ResourceReqs: deploy.ResourceRequirements{
			CPU: deploy.ResourceSpec{
				Request: request.ResourceRequirements.CPU,
				Limit:   request.ResourceRequirements.CPULimit,
			},
			Memory: deploy.ResourceSpec{
				Request: request.ResourceRequirements.Memory,
				Limit:   request.ResourceRequirements.MemoryLimit,
			},
		},
		Options: deploy.ManifestOptions{
			Namespace:   request.Namespace,
			Labels:      request.DeployOptions.CustomLabels,
			Annotations: request.DeployOptions.CustomAnnotations,
			Validate:    true,
		},
		CreatedAt: time.Now(),
	}

	// Execute manifest generation
	result, err := cmd.performManifestGeneration(ctx, manifestRequest, workspaceDir)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeKubernetesAPIError).
			Type(errors.ErrTypeKubernetes).
			Messagef("manifest generation failed: %w", err).
			WithLocation().
			Build()
	}

	return result, nil
}

// executeRollback performs deployment rollback operation
func (cmd *ConsolidatedDeployCommand) executeRollback(ctx context.Context, request *DeployRequest, workspaceDir string) (*deploy.DeploymentResult, error) {
	// Create rollback request from domain
	rollbackRequest := &deploy.RollbackRequest{
		ID:           fmt.Sprintf("rollback-%d", time.Now().Unix()),
		SessionID:    request.SessionID,
		DeploymentID: request.Name, // Use name as deployment ID
		Reason:       "Manual rollback triggered",
		CreatedAt:    time.Now(),
	}

	// Execute rollback using Kubernetes client
	result, err := cmd.performKubernetesRollback(ctx, rollbackRequest, workspaceDir)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeKubernetesAPIError).
			Type(errors.ErrTypeKubernetes).
			Messagef("rollback execution failed: %w", err).
			WithLocation().
			Build()
	}

	return result, nil
}

// executeHealthCheck performs deployment health check operation
func (cmd *ConsolidatedDeployCommand) executeHealthCheck(ctx context.Context, request *DeployRequest, workspaceDir string) (*deploy.DeploymentResult, error) {
	// Execute health check using Kubernetes client
	result, err := cmd.performHealthCheck(ctx, request.Name, request.Namespace)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeKubernetesAPIError).
			Type(errors.ErrTypeKubernetes).
			Messagef("health check failed: %w", err).
			WithLocation().
			Build()
	}

	return result, nil
}

// updateSessionState updates session state with deploy results
func (cmd *ConsolidatedDeployCommand) updateSessionState(ctx context.Context, sessionID string, result *deploy.DeploymentResult) error {
	// Update session state with deploy results
	stateUpdate := map[string]interface{}{
		"last_deployment":      result,
		"deployment_time":      time.Now(),
		"deployment_success":   result.Status == deploy.StatusCompleted,
		"deployment_name":      result.Name,
		"deployment_namespace": result.Namespace,
		"deployment_duration":  result.Duration,
	}

	return cmd.sessionState.UpdateSessionData(ctx, sessionID, stateUpdate)
}

// createDeployResponse creates the final deploy response
func (cmd *ConsolidatedDeployCommand) createDeployResponse(result *deploy.DeploymentResult, duration time.Duration) *ConsolidatedDeployResponse {
	return &ConsolidatedDeployResponse{
		Success:       result.Status == deploy.StatusCompleted,
		DeploymentID:  result.DeploymentID,
		Name:          result.Name,
		Namespace:     result.Namespace,
		Status:        string(result.Status),
		Resources:     convertDeployedResources(result.Resources),
		Endpoints:     convertEndpoints(result.Endpoints),
		Events:        convertEvents(result.Events),
		Duration:      result.Duration,
		Error:         result.Error,
		TotalDuration: duration,
		Metadata:      convertDeploymentMetadata(result.Metadata),
	}
}

// Tool registration for consolidated deploy command
func (cmd *ConsolidatedDeployCommand) Name() string {
	return "deploy_kubernetes"
}

func (cmd *ConsolidatedDeployCommand) Description() string {
	return "Comprehensive Kubernetes deployment tool that consolidates all deployment capabilities"
}

func (cmd *ConsolidatedDeployCommand) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        cmd.Name(),
		Description: cmd.Description(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Deployment operation type",
					"enum":        []string{"deploy", "generate_manifests", "rollback", "health_check"},
					"default":     "deploy",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Application name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
					"default":     "default",
				},
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Container image",
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Image tag",
					"default":     "latest",
				},
				"replicas": map[string]interface{}{
					"type":        "integer",
					"description": "Number of replicas",
					"default":     1,
					"minimum":     0,
					"maximum":     100,
				},
				"environment": map[string]interface{}{
					"type":        "string",
					"description": "Deployment environment",
					"enum":        []string{"development", "staging", "production", "test"},
					"default":     "development",
				},
				"strategy": map[string]interface{}{
					"type":        "string",
					"description": "Deployment strategy",
					"enum":        []string{"rolling", "recreate", "blue_green", "canary"},
					"default":     "rolling",
				},
				"manifest_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to Kubernetes manifest file",
				},
				"dry_run": map[string]interface{}{
					"type":        "boolean",
					"description": "Perform dry run only",
					"default":     false,
				},
				"wait_for_ready": map[string]interface{}{
					"type":        "boolean",
					"description": "Wait for deployment to be ready",
					"default":     true,
				},
				"timeout": map[string]interface{}{
					"type":        "string",
					"description": "Timeout duration (e.g., '5m', '300s')",
					"default":     "5m",
				},
				"force": map[string]interface{}{
					"type":        "boolean",
					"description": "Force deployment",
					"default":     false,
				},
				"include_ingress": map[string]interface{}{
					"type":        "boolean",
					"description": "Include ingress configuration",
					"default":     false,
				},
				"ingress_host": map[string]interface{}{
					"type":        "string",
					"description": "Ingress host",
				},
				"cpu_request": map[string]interface{}{
					"type":        "string",
					"description": "CPU request",
					"default":     "100m",
				},
				"memory_request": map[string]interface{}{
					"type":        "string",
					"description": "Memory request",
					"default":     "128Mi",
				},
				"cpu_limit": map[string]interface{}{
					"type":        "string",
					"description": "CPU limit",
					"default":     "500m",
				},
				"memory_limit": map[string]interface{}{
					"type":        "string",
					"description": "Memory limit",
					"default":     "512Mi",
				},
				"ports": map[string]interface{}{
					"type":        "array",
					"description": "Port configurations",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type": "string",
							},
							"port": map[string]interface{}{
								"type": "integer",
							},
							"target_port": map[string]interface{}{
								"type": "integer",
							},
							"protocol": map[string]interface{}{
								"type":    "string",
								"enum":    []string{"TCP", "UDP"},
								"default": "TCP",
							},
						},
					},
				},
				"labels": map[string]interface{}{
					"type":        "object",
					"description": "Custom labels",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"annotations": map[string]interface{}{
					"type":        "object",
					"description": "Custom annotations",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
			},
			"required": []string{"name", "image"},
		},
		Tags:     []string{"deploy", "kubernetes", "containerization"},
		Category: api.CategoryDeploy,
	}
}

// Helper types for consolidated deploy functionality

// DeployRequest represents a consolidated deploy request
type DeployRequest struct {
	SessionID            string                    `json:"session_id"`
	Operation            string                    `json:"operation"`
	Name                 string                    `json:"name"`
	Namespace            string                    `json:"namespace"`
	Image                string                    `json:"image"`
	Tag                  string                    `json:"tag"`
	Replicas             int                       `json:"replicas"`
	Environment          deploy.Environment        `json:"environment"`
	Strategy             deploy.DeploymentStrategy `json:"strategy"`
	DeployOptions        DeployOptions             `json:"deploy_options"`
	ResourceRequirements ResourceRequirements      `json:"resource_requirements"`
	Ports                []PortConfig              `json:"ports"`
	CreatedAt            time.Time                 `json:"created_at"`
}

// DeployOptions contains deployment configuration options
type DeployOptions struct {
	ManifestPath      string            `json:"manifest_path"`
	DryRun            bool              `json:"dry_run"`
	WaitForReady      bool              `json:"wait_for_ready"`
	Timeout           time.Duration     `json:"timeout"`
	Force             bool              `json:"force"`
	IncludeIngress    bool              `json:"include_ingress"`
	IngressHost       string            `json:"ingress_host"`
	CustomLabels      map[string]string `json:"custom_labels"`
	CustomAnnotations map[string]string `json:"custom_annotations"`
}

// ResourceRequirements contains resource configuration
type ResourceRequirements struct {
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	CPULimit    string `json:"cpu_limit"`
	MemoryLimit string `json:"memory_limit"`
}

// PortConfig represents port configuration
type PortConfig struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`
}

// ConsolidatedDeployResponse represents the consolidated deploy response
type ConsolidatedDeployResponse struct {
	Success       bool                   `json:"success"`
	DeploymentID  string                 `json:"deployment_id"`
	Name          string                 `json:"name"`
	Namespace     string                 `json:"namespace"`
	Status        string                 `json:"status"`
	Resources     DeployedResources      `json:"resources"`
	Endpoints     []EndpointInfo         `json:"endpoints"`
	Events        []EventInfo            `json:"events"`
	Duration      time.Duration          `json:"duration"`
	Error         string                 `json:"error,omitempty"`
	TotalDuration time.Duration          `json:"total_duration"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// DeployedResources represents deployed resources
type DeployedResources struct {
	Deployment string   `json:"deployment"`
	Service    string   `json:"service"`
	Ingress    string   `json:"ingress"`
	ConfigMaps []string `json:"config_maps"`
	Secrets    []string `json:"secrets"`
}

// EndpointInfo represents endpoint information
type EndpointInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Type     string `json:"type"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Ready    bool   `json:"ready"`
}

// EventInfo represents event information
type EventInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Component string    `json:"component"`
}

// Note: ValidationError is defined in common.go

// Helper functions for deploy operations

// isValidKubernetesName validates Kubernetes resource names
func isValidKubernetesName(name string) bool {
	// Basic validation - can be enhanced with full Kubernetes naming rules
	if name == "" || len(name) > 63 {
		return false
	}

	// Check for invalid characters
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}

	// Must start and end with alphanumeric
	if len(name) > 0 {
		first := name[0]
		last := name[len(name)-1]
		if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
			return false
		}
		if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
			return false
		}
	}

	return true
}

// Note: isValidImageName is defined in common.go

// parsePortsFromInput parses port configurations from input data
func (cmd *ConsolidatedDeployCommand) parsePortsFromInput(data map[string]interface{}) []PortConfig {
	var ports []PortConfig

	if portsData, ok := data["ports"].([]interface{}); ok {
		for _, portData := range portsData {
			if portMap, ok := portData.(map[string]interface{}); ok {
				port := PortConfig{
					Name:       getStringFromMap(portMap, "name", "http"),
					Port:       getIntFromMap(portMap, "port", 80),
					TargetPort: getIntFromMap(portMap, "target_port", 8080),
					Protocol:   getStringFromMap(portMap, "protocol", "TCP"),
				}
				ports = append(ports, port)
			}
		}
	}

	// Default port if none specified
	if len(ports) == 0 {
		ports = append(ports, PortConfig{
			Name:       "http",
			Port:       80,
			TargetPort: 8080,
			Protocol:   "TCP",
		})
	}

	return ports
}

// convertToDomainPorts converts PortConfig to domain ServicePort
func (cmd *ConsolidatedDeployCommand) convertToDomainPorts(ports []PortConfig) []deploy.ServicePort {
	domainPorts := make([]deploy.ServicePort, len(ports))
	for i, port := range ports {
		domainPorts[i] = deploy.ServicePort{
			Name:        port.Name,
			Port:        port.Port,
			TargetPort:  port.TargetPort,
			Protocol:    deploy.Protocol(port.Protocol),
			ServiceType: deploy.ServiceTypeClusterIP,
		}
	}
	return domainPorts
}

// convertDeployedResources converts domain DeployedResources to response format
func convertDeployedResources(resources deploy.DeployedResources) DeployedResources {
	return DeployedResources{
		Deployment: resources.Deployment,
		Service:    resources.Service,
		Ingress:    resources.Ingress,
		ConfigMaps: resources.ConfigMaps,
		Secrets:    resources.Secrets,
	}
}

// convertEndpoints converts domain Endpoints to response format
func convertEndpoints(endpoints []deploy.Endpoint) []EndpointInfo {
	result := make([]EndpointInfo, len(endpoints))
	for i, endpoint := range endpoints {
		result[i] = EndpointInfo{
			Name:     endpoint.Name,
			URL:      endpoint.URL,
			Type:     string(endpoint.Type),
			Port:     endpoint.Port,
			Protocol: string(endpoint.Protocol),
			Ready:    endpoint.Ready,
		}
	}
	return result
}

// convertEvents converts domain Events to response format
func convertEvents(events []deploy.DeploymentEvent) []EventInfo {
	result := make([]EventInfo, len(events))
	for i, event := range events {
		result[i] = EventInfo{
			Timestamp: event.Timestamp,
			Type:      string(event.Type),
			Reason:    event.Reason,
			Message:   event.Message,
			Component: event.Component,
		}
	}
	return result
}

// convertDeploymentMetadata converts domain metadata to response format
func convertDeploymentMetadata(metadata deploy.DeploymentMetadata) map[string]interface{} {
	return map[string]interface{}{
		"strategy":         metadata.Strategy,
		"environment":      metadata.Environment,
		"image_digest":     metadata.ImageDigest,
		"previous_version": metadata.PreviousVersion,
		"resource_usage":   metadata.ResourceUsage,
		"scaling_info":     metadata.ScalingInfo,
		"network_info":     metadata.NetworkInfo,
		"security_scan":    metadata.SecurityScan,
	}
}

// contains checks if a string slice contains a specific value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// Note: Parameter extraction functions (getStringParam, getBoolParam, getIntParam) are defined in common.go

// Note: getDurationParam is defined in commands.go

// Note: getStringMapParam can be added to common.go if needed

// getStringFromMap extracts a string value from a map
func getStringFromMap(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return defaultValue
}

// getIntFromMap extracts an integer value from a map
func getIntFromMap(data map[string]interface{}, key string, defaultValue int) int {
	if value, ok := data[key].(int); ok {
		return value
	}
	if value, ok := data[key].(float64); ok {
		return int(value)
	}
	return defaultValue
}
