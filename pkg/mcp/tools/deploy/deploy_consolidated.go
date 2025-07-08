package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// Register consolidated deployment tools
func init() {
	core.RegisterTool("kubernetes_deploy_consolidated", func() api.Tool {
		return &ConsolidatedKubernetesDeployTool{}
	})

	core.RegisterTool("manifests_consolidated", func() api.Tool {
		return &ConsolidatedManifestsTool{}
	})
}

// ConsolidatedKubernetesDeployInput represents unified input for all deployment variants
type ConsolidatedKubernetesDeployInput struct {
	// Core parameters (with backward compatibility aliases)
	SessionID string `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	ImageRef  string `json:"image_ref" validate:"required,docker_image" description:"Container image reference"`
	ImageName string `json:"image_name,omitempty" description:"Alias for image_ref for backward compatibility"`
	Image     string `json:"image,omitempty" description:"Alias for image_ref for backward compatibility"`

	// Application configuration
	AppName   string `json:"app_name,omitempty" validate:"omitempty,k8s_name" description:"Application name (default: from image name)"`
	Namespace string `json:"namespace,omitempty" validate:"omitempty,namespace" description:"Kubernetes namespace (default: default)"`

	// Deployment configuration
	Replicas       int               `json:"replicas,omitempty" validate:"omitempty,min=1,max=100" description:"Number of replicas (default: 1)"`
	Port           int               `json:"port,omitempty" validate:"omitempty,port" description:"Application port (default: 80)"`
	ServiceType    string            `json:"service_type,omitempty" validate:"omitempty,service_type" description:"Service type: ClusterIP, NodePort, LoadBalancer"`
	IncludeIngress bool              `json:"include_ingress,omitempty" description:"Generate and deploy Ingress resource"`
	Environment    map[string]string `json:"environment,omitempty" validate:"omitempty,dive,keys,required,endkeys,no_sensitive" description:"Environment variables"`

	// Resource configuration
	CPURequest    string `json:"cpu_request,omitempty" validate:"omitempty,resource_spec" description:"CPU request (e.g., 100m)"`
	MemoryRequest string `json:"memory_request,omitempty" validate:"omitempty,resource_spec" description:"Memory request (e.g., 128Mi)"`
	CPULimit      string `json:"cpu_limit,omitempty" validate:"omitempty,resource_spec" description:"CPU limit (e.g., 500m)"`
	MemoryLimit   string `json:"memory_limit,omitempty" validate:"omitempty,resource_spec" description:"Memory limit (e.g., 512Mi)"`

	// Deployment modes
	DeployMode string `json:"deploy_mode,omitempty" validate:"omitempty,oneof=apply generate validate health" description:"Deployment mode: apply, generate, validate, or health"`
	DryRun     bool   `json:"dry_run,omitempty" description:"Preview deployment without applying"`

	// Optional features
	GenerateOnly      bool `json:"generate_only,omitempty" description:"Only generate manifests, don't deploy"`
	WaitForReady      bool `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready"`
	WaitTimeout       int  `json:"wait_timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Wait timeout in seconds"`
	SkipHealthCheck   bool `json:"skip_health_check,omitempty" description:"Skip health check validation"`
	IncludeValidation bool `json:"include_validation,omitempty" description:"Include manifest validation"`

	// Performance options
	UseCache        bool `json:"use_cache,omitempty" description:"Use cached results if available"`
	Timeout         int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Deployment timeout in seconds"`
	RollbackOnError bool `json:"rollback_on_error,omitempty" description:"Rollback on deployment failure"`

	// Advanced options
	DeploymentStrategy string                 `json:"deployment_strategy,omitempty" validate:"omitempty,oneof=rolling recreate blue-green" description:"Deployment strategy"`
	Metadata           map[string]interface{} `json:"metadata,omitempty" description:"Additional metadata for deployment context"`
}

// Validate implements validation using tag-based validation
func (c ConsolidatedKubernetesDeployInput) Validate() error {
	imageRef := c.getImageRef()
	if imageRef == "" {
		return errors.NewError().Message("image reference is required").Build()
	}
	return validation.ValidateTaggedStruct(c)
}

// getImageRef returns the image reference, handling backward compatibility aliases
func (c ConsolidatedKubernetesDeployInput) getImageRef() string {
	if c.ImageRef != "" {
		return c.ImageRef
	}
	if c.ImageName != "" {
		return c.ImageName
	}
	return c.Image
}

// getDeployMode returns the deployment mode, defaulting to apply
func (c ConsolidatedKubernetesDeployInput) getDeployMode() string {
	if c.DeployMode != "" {
		return c.DeployMode
	}
	return "apply"
}

// ConsolidatedKubernetesDeployOutput represents unified output for all deployment variants
type ConsolidatedKubernetesDeployOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`

	// Core deployment results
	ImageRef       string        `json:"image_ref"`
	AppName        string        `json:"app_name"`
	Namespace      string        `json:"namespace"`
	DeployMode     string        `json:"deploy_mode"`
	DeployTime     time.Time     `json:"deploy_time"`
	DeployDuration time.Duration `json:"deploy_duration"`

	// Deployment results
	ManifestsGenerated []string           `json:"manifests_generated,omitempty"`
	ResourcesCreated   []string           `json:"resources_created,omitempty"`
	ResourcesUpdated   []string           `json:"resources_updated,omitempty"`
	DeploymentStatus   *DeploymentStatus  `json:"deployment_status,omitempty"`
	HealthCheck        *HealthCheckResult `json:"health_check,omitempty"`
	ValidationResult   *ValidationResult  `json:"validation_result,omitempty"`

	// Generated manifests
	GeneratedManifests map[string]string `json:"generated_manifests,omitempty"`
	ManifestPaths      []string          `json:"manifest_paths,omitempty"`

	// Deployment metadata
	DeploymentStrategy string                      `json:"deployment_strategy"`
	Rollback           *RollbackInfo               `json:"rollback,omitempty"`
	ServiceEndpoints   []ServiceEndpoint           `json:"service_endpoints,omitempty"`
	PodStatus          map[string]PodStatusSummary `json:"pod_status,omitempty"`

	// Performance metrics
	GenerationDuration  time.Duration `json:"generation_duration"`
	ApplyDuration       time.Duration `json:"apply_duration"`
	HealthCheckDuration time.Duration `json:"health_check_duration"`
	TotalDuration       time.Duration `json:"total_duration"`
	CacheHit            bool          `json:"cache_hit,omitempty"`

	// Metadata
	ToolVersion string                 `json:"tool_version"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
}

// Supporting types (consolidated from all deployment variants)
type DeploymentStatus struct {
	Phase             string    `json:"phase"`
	ReadyReplicas     int       `json:"ready_replicas"`
	UpdatedReplicas   int       `json:"updated_replicas"`
	AvailableReplicas int       `json:"available_replicas"`
	Conditions        []string  `json:"conditions"`
	LastUpdateTime    time.Time `json:"last_update_time"`
}

type HealthCheckResult struct {
	Passed       bool              `json:"passed"`
	Checks       []HealthCheck     `json:"checks"`
	Endpoints    []ServiceEndpoint `json:"endpoints"`
	ReadyPods    int               `json:"ready_pods"`
	TotalPods    int               `json:"total_pods"`
	ResponseTime time.Duration     `json:"response_time"`
}

type HealthCheck struct {
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type ValidationResult struct {
	Valid       bool                `json:"valid"`
	Errors      []ValidationError   `json:"errors,omitempty"`
	Warnings    []ValidationWarning `json:"warnings,omitempty"`
	Suggestions []string            `json:"suggestions,omitempty"`
	Score       int                 `json:"score"` // 0-100
}

type ValidationError struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Resource   string `json:"resource,omitempty"`
	Field      string `json:"field,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type ValidationWarning struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Resource   string `json:"resource,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

type RollbackInfo struct {
	Triggered     bool      `json:"triggered"`
	Reason        string    `json:"reason,omitempty"`
	PreviousImage string    `json:"previous_image,omitempty"`
	RollbackTime  time.Time `json:"rollback_time,omitempty"`
}

type ServiceEndpoint struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Ready    bool   `json:"ready"`
}

type PodStatusSummary struct {
	Name      string    `json:"name"`
	Phase     string    `json:"phase"`
	Ready     bool      `json:"ready"`
	Restarts  int       `json:"restarts"`
	CreatedAt time.Time `json:"created_at"`
}

// ConsolidatedKubernetesDeployTool - Unified Kubernetes deployment tool
type ConsolidatedKubernetesDeployTool struct {
	// Service dependencies
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	workflowExecutor services.WorkflowExecutor
	k8sClient        services.K8sClient
	configValidator  services.ConfigValidator
	logger           *slog.Logger

	// Core deployment components
	manifestGenerator *ManifestGenerator
	healthChecker     *HealthChecker
	validator         *DeploymentValidator
	deployer          *KubernetesDeployer
	cacheManager      *DeploymentCacheManager

	// State management
	workspaceDir string
}

// NewConsolidatedKubernetesDeployTool creates a new consolidated Kubernetes deployment tool
func NewConsolidatedKubernetesDeployTool(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *ConsolidatedKubernetesDeployTool {
	toolLogger := logger.With("tool", "kubernetes_deploy_consolidated")

	return &ConsolidatedKubernetesDeployTool{
		sessionStore:      serviceContainer.SessionStore(),
		sessionState:      serviceContainer.SessionState(),
		workflowExecutor:  serviceContainer.WorkflowExecutor(),
		k8sClient:         serviceContainer.K8sClient(),
		configValidator:   serviceContainer.ConfigValidator(),
		logger:            toolLogger,
		manifestGenerator: NewManifestGenerator(toolLogger),
		healthChecker:     NewHealthChecker(toolLogger),
		validator:         NewDeploymentValidator(toolLogger),
		deployer:          NewKubernetesDeployer(toolLogger),
		cacheManager:      NewDeploymentCacheManager(toolLogger),
	}
}

// Execute implements api.Tool interface
func (t *ConsolidatedKubernetesDeployTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	deployInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := deployInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Generate session ID if not provided
	sessionID := deployInput.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("deploy_%d", time.Now().Unix())
	}

	// Execute deployment based on mode
	result, err := t.executeDeployment(ctx, deployInput, sessionID, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Deployment failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeDeployment performs the deployment based on the specified mode
func (t *ConsolidatedKubernetesDeployTool) executeDeployment(
	ctx context.Context,
	input *ConsolidatedKubernetesDeployInput,
	sessionID string,
	startTime time.Time,
) (*ConsolidatedKubernetesDeployOutput, error) {
	result := &ConsolidatedKubernetesDeployOutput{
		Success:            false,
		SessionID:          sessionID,
		ImageRef:           input.getImageRef(),
		AppName:            input.AppName,
		Namespace:          input.Namespace,
		DeployMode:         input.getDeployMode(),
		DeploymentStrategy: input.DeploymentStrategy,
		ToolVersion:        "2.0.0",
		Timestamp:          startTime,
		DeployTime:         startTime,
		Metadata:           make(map[string]interface{}),
	}

	// Initialize session
	if err := t.initializeSession(ctx, sessionID, input); err != nil {
		t.logger.Warn("Failed to initialize session", "error", err)
	}

	// Check cache if enabled
	if input.UseCache {
		if cachedResult := t.checkCache(input); cachedResult != nil {
			cachedResult.CacheHit = true
			return cachedResult, nil
		}
	}

	// Execute based on deployment mode
	switch input.getDeployMode() {
	case "generate":
		return t.executeGenerateManifests(ctx, input, result)
	case "validate":
		return t.executeValidateDeployment(ctx, input, result)
	case "health":
		return t.executeHealthCheck(ctx, input, result)
	default: // apply
		return t.executeFullDeployment(ctx, input, result)
	}
}

// executeGenerateManifests performs manifest generation only
func (t *ConsolidatedKubernetesDeployTool) executeGenerateManifests(
	ctx context.Context,
	input *ConsolidatedKubernetesDeployInput,
	result *ConsolidatedKubernetesDeployOutput,
) (*ConsolidatedKubernetesDeployOutput, error) {
	t.logger.Info("Executing manifest generation",
		"image_ref", result.ImageRef,
		"session_id", result.SessionID)

	generationStart := time.Now()

	// Generate manifests
	manifests, err := t.generateManifests(ctx, input, result)
	if err != nil {
		return result, err
	}

	result.GeneratedManifests = manifests
	result.ManifestsGenerated = make([]string, 0, len(manifests))
	for name := range manifests {
		result.ManifestsGenerated = append(result.ManifestsGenerated, name)
	}

	result.Success = true
	result.GenerationDuration = time.Since(generationStart)
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Manifest generation completed",
		"manifests_count", len(manifests),
		"duration", result.TotalDuration)

	return result, nil
}

// executeValidateDeployment performs deployment validation
func (t *ConsolidatedKubernetesDeployTool) executeValidateDeployment(
	ctx context.Context,
	input *ConsolidatedKubernetesDeployInput,
	result *ConsolidatedKubernetesDeployOutput,
) (*ConsolidatedKubernetesDeployOutput, error) {
	t.logger.Info("Executing deployment validation",
		"image_ref", result.ImageRef,
		"session_id", result.SessionID)

	validationStart := time.Now()

	// Generate manifests first
	manifests, err := t.generateManifests(ctx, input, result)
	if err != nil {
		return result, err
	}

	// Validate manifests
	validationResult, err := t.validateManifests(ctx, manifests, input)
	if err != nil {
		return result, err
	}

	result.GeneratedManifests = manifests
	result.ValidationResult = validationResult
	result.Success = validationResult.Valid
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Deployment validation completed",
		"valid", validationResult.Valid,
		"errors", len(validationResult.Errors),
		"warnings", len(validationResult.Warnings),
		"duration", time.Since(validationStart))

	return result, nil
}

// executeHealthCheck performs health check only
func (t *ConsolidatedKubernetesDeployTool) executeHealthCheck(
	ctx context.Context,
	input *ConsolidatedKubernetesDeployInput,
	result *ConsolidatedKubernetesDeployOutput,
) (*ConsolidatedKubernetesDeployOutput, error) {
	t.logger.Info("Executing health check",
		"app_name", result.AppName,
		"namespace", result.Namespace,
		"session_id", result.SessionID)

	healthStart := time.Now()

	// Perform health check
	healthResult, err := t.performHealthCheck(ctx, input, result)
	if err != nil {
		return result, err
	}

	result.HealthCheck = healthResult
	result.Success = healthResult.Passed
	result.HealthCheckDuration = time.Since(healthStart)
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Health check completed",
		"passed", healthResult.Passed,
		"ready_pods", healthResult.ReadyPods,
		"total_pods", healthResult.TotalPods,
		"duration", result.HealthCheckDuration)

	return result, nil
}

// executeFullDeployment performs complete deployment pipeline
func (t *ConsolidatedKubernetesDeployTool) executeFullDeployment(
	ctx context.Context,
	input *ConsolidatedKubernetesDeployInput,
	result *ConsolidatedKubernetesDeployOutput,
) (*ConsolidatedKubernetesDeployOutput, error) {
	t.logger.Info("Executing full deployment",
		"image_ref", result.ImageRef,
		"app_name", result.AppName,
		"namespace", result.Namespace,
		"session_id", result.SessionID)

	deployStart := time.Now()

	// Step 1: Generate manifests
	generationStart := time.Now()
	manifests, err := t.generateManifests(ctx, input, result)
	if err != nil {
		return result, err
	}
	result.GeneratedManifests = manifests
	result.GenerationDuration = time.Since(generationStart)

	// Step 2: Validate manifests (if requested)
	if input.IncludeValidation {
		validationResult, err := t.validateManifests(ctx, manifests, input)
		if err != nil {
			t.logger.Warn("Validation failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Validation warning: %v", err))
		} else {
			result.ValidationResult = validationResult
		}
	}

	// Step 3: Apply manifests (unless dry run or generate only)
	if !input.DryRun && !input.GenerateOnly {
		applyStart := time.Now()
		deploymentStatus, err := t.applyManifests(ctx, manifests, input, result)
		if err != nil {
			if input.RollbackOnError {
				if rollbackErr := t.performRollback(ctx, input, result); rollbackErr != nil {
					t.logger.Error("Rollback failed", "error", rollbackErr)
					result.Warnings = append(result.Warnings, fmt.Sprintf("Rollback failed: %v", rollbackErr))
				}
			}
			return result, err
		}
		result.DeploymentStatus = deploymentStatus
		result.ApplyDuration = time.Since(applyStart)
	}

	// Step 4: Health check (if not skipped)
	if !input.SkipHealthCheck && !input.DryRun && !input.GenerateOnly {
		healthStart := time.Now()
		healthResult, err := t.performHealthCheck(ctx, input, result)
		if err != nil {
			t.logger.Warn("Health check failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Health check warning: %v", err))
		} else {
			result.HealthCheck = healthResult
		}
		result.HealthCheckDuration = time.Since(healthStart)
	}

	result.Success = true
	result.DeployDuration = time.Since(deployStart)
	result.TotalDuration = time.Since(result.Timestamp)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Full deployment completed",
		"manifests_count", len(manifests),
		"deployment_duration", result.DeployDuration,
		"total_duration", result.TotalDuration)

	return result, nil
}

// Implement api.Tool interface methods

func (t *ConsolidatedKubernetesDeployTool) Name() string {
	return "kubernetes_deploy_consolidated"
}

func (t *ConsolidatedKubernetesDeployTool) Description() string {
	return "Comprehensive Kubernetes deployment tool with unified interface supporting generate, validate, health, and apply modes"
}

func (t *ConsolidatedKubernetesDeployTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "kubernetes_deploy_consolidated",
		Description: "Comprehensive Kubernetes deployment tool with unified interface supporting generate, validate, health, and apply modes",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Container image reference",
				},
				"deploy_mode": map[string]interface{}{
					"type":        "string",
					"description": "Deployment mode: apply, generate, validate, or health",
					"enum":        []string{"apply", "generate", "validate", "health"},
				},
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Application name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
				},
				"replicas": map[string]interface{}{
					"type":        "integer",
					"description": "Number of replicas",
				},
				"port": map[string]interface{}{
					"type":        "integer",
					"description": "Application port",
				},
			},
			"required": []string{"image_ref"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether deployment was successful",
				},
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Deployed image reference",
				},
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Application name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Deployment namespace",
				},
				"generated_manifests": map[string]interface{}{
					"type":        "object",
					"description": "Generated Kubernetes manifests",
				},
				"deployment_status": map[string]interface{}{
					"type":        "object",
					"description": "Deployment status information",
				},
				"health_check": map[string]interface{}{
					"type":        "object",
					"description": "Health check results",
				},
			},
		},
	}
}

// Helper methods for tool implementation

func (t *ConsolidatedKubernetesDeployTool) parseInput(input api.ToolInput) (*ConsolidatedKubernetesDeployInput, error) {
	result := &ConsolidatedKubernetesDeployInput{}

	// Handle map[string]interface{} data
	v := input.Data
	// Extract parameters from map
	if imageRef, ok := v["image_ref"].(string); ok {
		result.ImageRef = imageRef
	}
	if imageName, ok := v["image_name"].(string); ok {
		result.ImageName = imageName
	}
	if image, ok := v["image"].(string); ok {
		result.Image = image
	}
	if sessionID, ok := v["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if appName, ok := v["app_name"].(string); ok {
		result.AppName = appName
	}
	if namespace, ok := v["namespace"].(string); ok {
		result.Namespace = namespace
	}
	if deployMode, ok := v["deploy_mode"].(string); ok {
		result.DeployMode = deployMode
	}
	if replicas, ok := v["replicas"].(float64); ok {
		result.Replicas = int(replicas)
	}
	if port, ok := v["port"].(float64); ok {
		result.Port = int(port)
	}
	if serviceType, ok := v["service_type"].(string); ok {
		result.ServiceType = serviceType
	}
	if includeIngress, ok := v["include_ingress"].(bool); ok {
		result.IncludeIngress = includeIngress
	}
	if environment, ok := v["environment"].(map[string]interface{}); ok {
		result.Environment = make(map[string]string)
		for k, v := range environment {
			if str, ok := v.(string); ok {
				result.Environment[k] = str
			}
		}
	}
	if cpuRequest, ok := v["cpu_request"].(string); ok {
		result.CPURequest = cpuRequest
	}
	if memoryRequest, ok := v["memory_request"].(string); ok {
		result.MemoryRequest = memoryRequest
	}
	if cpuLimit, ok := v["cpu_limit"].(string); ok {
		result.CPULimit = cpuLimit
	}
	if memoryLimit, ok := v["memory_limit"].(string); ok {
		result.MemoryLimit = memoryLimit
	}
	if dryRun, ok := v["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}
	if generateOnly, ok := v["generate_only"].(bool); ok {
		result.GenerateOnly = generateOnly
	}
	if waitForReady, ok := v["wait_for_ready"].(bool); ok {
		result.WaitForReady = waitForReady
	}
	if waitTimeout, ok := v["wait_timeout"].(float64); ok {
		result.WaitTimeout = int(waitTimeout)
	}
	if skipHealthCheck, ok := v["skip_health_check"].(bool); ok {
		result.SkipHealthCheck = skipHealthCheck
	}
	if includeValidation, ok := v["include_validation"].(bool); ok {
		result.IncludeValidation = includeValidation
	}
	if useCache, ok := v["use_cache"].(bool); ok {
		result.UseCache = useCache
	}
	if timeout, ok := v["timeout"].(float64); ok {
		result.Timeout = int(timeout)
	}
	if rollbackOnError, ok := v["rollback_on_error"].(bool); ok {
		result.RollbackOnError = rollbackOnError
	}
	if deploymentStrategy, ok := v["deployment_strategy"].(string); ok {
		result.DeploymentStrategy = deploymentStrategy
	}
	// ... (more field extractions)

	return result, nil
}

// initializeSession initializes session state for deployment
func (t *ConsolidatedKubernetesDeployTool) initializeSession(ctx context.Context, sessionID string, input *ConsolidatedKubernetesDeployInput) error {
	if t.sessionStore == nil {
		return nil // Session management not available
	}

	sessionData := map[string]interface{}{
		"image_ref":   input.getImageRef(),
		"app_name":    input.AppName,
		"namespace":   input.Namespace,
		"deploy_mode": input.getDeployMode(),
		"started_at":  time.Now(),
	}

	session := &api.Session{
		ID:       sessionID,
		Metadata: sessionData,
	}
	return t.sessionStore.Create(ctx, session)
}

// checkCache checks for cached deployment results
func (t *ConsolidatedKubernetesDeployTool) checkCache(input *ConsolidatedKubernetesDeployInput) *ConsolidatedKubernetesDeployOutput {
	if t.cacheManager == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("%s_%s_%s_%s", input.getImageRef(), input.AppName, input.Namespace, input.getDeployMode())
	return t.cacheManager.Get(cacheKey)
}

// cacheResult caches the deployment result
func (t *ConsolidatedKubernetesDeployTool) cacheResult(input *ConsolidatedKubernetesDeployInput, result *ConsolidatedKubernetesDeployOutput) {
	if t.cacheManager == nil {
		return
	}

	cacheKey := fmt.Sprintf("%s_%s_%s_%s", input.getImageRef(), input.AppName, input.Namespace, input.getDeployMode())
	t.cacheManager.Set(cacheKey, result)
}

// Placeholder methods for the helper components - these would be implemented based on existing code

func (t *ConsolidatedKubernetesDeployTool) generateManifests(ctx context.Context, input *ConsolidatedKubernetesDeployInput, result *ConsolidatedKubernetesDeployOutput) (map[string]string, error) {
	// This would use the existing manifest generation logic
	return map[string]string{
		"deployment.yaml": "# Generated deployment manifest",
		"service.yaml":    "# Generated service manifest",
	}, nil
}

func (t *ConsolidatedKubernetesDeployTool) validateManifests(ctx context.Context, manifests map[string]string, input *ConsolidatedKubernetesDeployInput) (*ValidationResult, error) {
	// This would use the existing validation logic
	return &ValidationResult{
		Valid:       true,
		Errors:      []ValidationError{},
		Warnings:    []ValidationWarning{},
		Suggestions: []string{},
		Score:       90,
	}, nil
}

func (t *ConsolidatedKubernetesDeployTool) applyManifests(ctx context.Context, manifests map[string]string, input *ConsolidatedKubernetesDeployInput, result *ConsolidatedKubernetesDeployOutput) (*DeploymentStatus, error) {
	// This would use the existing apply logic
	return &DeploymentStatus{
		Phase:             "Progressing",
		ReadyReplicas:     input.Replicas,
		UpdatedReplicas:   input.Replicas,
		AvailableReplicas: input.Replicas,
		Conditions:        []string{"Available"},
		LastUpdateTime:    time.Now(),
	}, nil
}

func (t *ConsolidatedKubernetesDeployTool) performHealthCheck(ctx context.Context, input *ConsolidatedKubernetesDeployInput, result *ConsolidatedKubernetesDeployOutput) (*HealthCheckResult, error) {
	// This would use the existing health check logic
	return &HealthCheckResult{
		Passed:       true,
		Checks:       []HealthCheck{},
		Endpoints:    []ServiceEndpoint{},
		ReadyPods:    input.Replicas,
		TotalPods:    input.Replicas,
		ResponseTime: 100 * time.Millisecond,
	}, nil
}

func (t *ConsolidatedKubernetesDeployTool) performRollback(ctx context.Context, input *ConsolidatedKubernetesDeployInput, result *ConsolidatedKubernetesDeployOutput) error {
	// This would implement rollback logic
	result.Rollback = &RollbackInfo{
		Triggered:    true,
		Reason:       "Deployment failed",
		RollbackTime: time.Now(),
	}
	return nil
}

// Supporting components that would be implemented based on existing code

type ManifestGenerator struct {
	logger *slog.Logger
}

func NewManifestGenerator(logger *slog.Logger) *ManifestGenerator {
	return &ManifestGenerator{logger: logger}
}

type HealthChecker struct {
	logger *slog.Logger
}

func NewHealthChecker(logger *slog.Logger) *HealthChecker {
	return &HealthChecker{logger: logger}
}

type DeploymentValidator struct {
	logger *slog.Logger
}

func NewDeploymentValidator(logger *slog.Logger) *DeploymentValidator {
	return &DeploymentValidator{logger: logger}
}

type KubernetesDeployer struct {
	logger *slog.Logger
}

func NewKubernetesDeployer(logger *slog.Logger) *KubernetesDeployer {
	return &KubernetesDeployer{logger: logger}
}

type DeploymentCacheManager struct {
	logger *slog.Logger
	cache  map[string]*ConsolidatedKubernetesDeployOutput
}

func NewDeploymentCacheManager(logger *slog.Logger) *DeploymentCacheManager {
	return &DeploymentCacheManager{
		logger: logger,
		cache:  make(map[string]*ConsolidatedKubernetesDeployOutput),
	}
}

func (d *DeploymentCacheManager) Get(key string) *ConsolidatedKubernetesDeployOutput {
	if result, exists := d.cache[key]; exists {
		d.logger.Info("Deployment cache hit", "key", key)
		return result
	}
	return nil
}

func (d *DeploymentCacheManager) Set(key string, result *ConsolidatedKubernetesDeployOutput) {
	d.cache[key] = result
	d.logger.Info("Deployment cache set", "key", key)
}
