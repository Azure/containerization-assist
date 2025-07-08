package deploy

import (
	"context"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	// mcp import removed - using mcptypes

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/core/tools"
	"github.com/Azure/container-kit/pkg/mcp/tools/build"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/localrivet/gomcp/server"
)

// AtomicDeployKubernetesArgs defines arguments for atomic Kubernetes deployment
type AtomicDeployKubernetesArgs struct {
	types.BaseToolArgs

	ImageRef  string `json:"image_ref" validate:"required,docker_image" description:"Container image reference (e.g., myregistry.azurecr.io/myapp:latest)"`
	AppName   string `json:"app_name,omitempty" validate:"omitempty,k8s_name" description:"Application name (default: from image name)"`
	Namespace string `json:"namespace,omitempty" validate:"omitempty,namespace" description:"Kubernetes namespace (default: default)"`

	Replicas       int               `json:"replicas,omitempty" validate:"omitempty,min=1,max=100" description:"Number of replicas (default: 1)"`
	Port           int               `json:"port,omitempty" validate:"omitempty,port" description:"Application port (default: 80)"`
	ServiceType    string            `json:"service_type,omitempty" validate:"omitempty,service_type" description:"Service type: ClusterIP, NodePort, LoadBalancer (default: ClusterIP)"`
	IncludeIngress bool              `json:"include_ingress,omitempty" description:"Generate and deploy Ingress resource"`
	Environment    map[string]string `json:"environment,omitempty" validate:"omitempty,dive,keys,required,endkeys,no_sensitive" description:"Environment variables"`

	CPURequest    string `json:"cpu_request,omitempty" validate:"omitempty,resource_spec" description:"CPU request (e.g., 100m)"`
	MemoryRequest string `json:"memory_request,omitempty" validate:"omitempty,resource_spec" description:"Memory request (e.g., 128Mi)"`
	CPULimit      string `json:"cpu_limit,omitempty" validate:"omitempty,resource_spec" description:"CPU limit (e.g., 500m)"`
	MemoryLimit   string `json:"memory_limit,omitempty" validate:"omitempty,resource_spec" description:"Memory limit (e.g., 512Mi)"`

	GenerateOnly    bool   `json:"generate_only,omitempty" description:"Only generate manifests, don't deploy"`
	WaitForReady    bool   `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready (default: true)"`
	WaitTimeout     int    `json:"wait_timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Wait timeout in seconds (default: 300)"`
	SkipHealthCheck bool   `json:"skip_health_check,omitempty" description:"Skip health check validation after deployment"`
	ManifestPath    string `json:"manifest_path,omitempty" validate:"omitempty,secure_path" description:"Custom path for generated manifests"`
	Force           bool   `json:"force,omitempty" description:"Force deployment even if validation fails"`
	DryRun          bool   `json:"dry_run,omitempty" description:"Preview changes without applying (shows kubectl diff output)"`
}

// AtomicDeployKubernetesResult defines the response from atomic Kubernetes deployment
type AtomicDeployKubernetesResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embed AI context methods
	Success                  bool `json:"success"`

	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	ImageRef    string `json:"image_ref"`
	AppName     string `json:"app_name"`
	Namespace   string `json:"namespace"`
	Replicas    int    `json:"replicas"`
	Port        int    `json:"port"`
	ServiceType string `json:"service_type"`

	ManifestResult *kubernetes.ManifestGenerationResult `json:"manifest_result"`

	DeploymentResult *kubernetes.DeploymentResult `json:"deployment_result,omitempty"`

	HealthResult *kubernetes.HealthCheckResult `json:"health_result,omitempty"`

	GenerationDuration  time.Duration `json:"generation_duration"`
	DeploymentDuration  time.Duration `json:"deployment_duration,omitempty"`
	HealthCheckDuration time.Duration `json:"health_check_duration,omitempty"`
	TotalDuration       time.Duration `json:"total_duration"`

	DeploymentContext *DeploymentContext `json:"deployment_context"`

	ConsolidatedFailureAnalysis *DeploymentFailureAnalysis `json:"failure_analysis,omitempty"`

	DryRunPreview string `json:"dry_run_preview,omitempty"`
}

// DeploymentFailureAnalysis provides failure analysis for AI reasoning
type DeploymentFailureAnalysis struct {
	FailureType    string   `json:"failure_type"`
	FailureStage   string   `json:"failure_stage"`
	RootCauses     []string `json:"root_causes"`
	ImpactSeverity string   `json:"impact_severity"`

	ImmediateActions      []DeploymentRemediationAction `json:"immediate_actions"`
	AlternativeApproaches []DeploymentAlternative       `json:"alternative_approaches"`

	DiagnosticCommands []DiagnosticCommand      `json:"diagnostic_commands"`
	MonitoringSetup    MonitoringRecommendation `json:"monitoring_setup"`

	RollbackStrategy RollbackGuidance `json:"rollback_strategy"`

	PerformanceTuning PerformanceOptimization `json:"performance_tuning"`
}

type DeploymentRemediationAction struct {
	Priority    int    `json:"priority"`
	Action      string `json:"action"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Expected    string `json:"expected"`
	RiskLevel   string `json:"risk_level"`
}

type DeploymentAlternative struct {
	Strategy     string   `json:"strategy"`
	Pros         []string `json:"pros"`
	Cons         []string `json:"cons"`
	Complexity   string   `json:"complexity"`
	TimeToValue  string   `json:"time_to_value"`
	ResourceReqs string   `json:"resource_reqs"`
}

type DiagnosticCommand struct {
	Purpose     string `json:"purpose"`
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

type MonitoringRecommendation struct {
	HealthChecks    []HealthCheckSetup     `json:"health_checks"`
	MetricsToTrack  []MetricRecommendation `json:"metrics_to_track"`
	AlertingRules   []AlertingRule         `json:"alerting_rules"`
	LoggingStrategy LoggingSetup           `json:"logging_strategy"`
}

type HealthCheckSetup struct {
	Type         string `json:"type"`
	Endpoint     string `json:"endpoint"`
	Port         int    `json:"port"`
	InitialDelay int    `json:"initial_delay"`
	Period       int    `json:"period"`
	Timeout      int    `json:"timeout"`
}

type MetricRecommendation struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Threshold   string `json:"threshold"`
}

type AlertingRule struct {
	Name        string `json:"name"`
	Condition   string `json:"condition"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type LoggingSetup struct {
	LogLevel       string   `json:"log_level"`
	StructuredLogs bool     `json:"structured_logs"`
	LogFields      []string `json:"log_fields"`
	Aggregation    string   `json:"aggregation"`
}

type RollbackGuidance struct {
	AutoRollbackTriggers []string `json:"auto_rollback_triggers"`
	ManualRollbackSteps  []string `json:"manual_rollback_steps"`
	RollbackRisk         string   `json:"rollback_risk"`
	DataIntegrity        string   `json:"data_integrity"`
	DowntimeEstimate     string   `json:"downtime_estimate"`
}

type PerformanceOptimization struct {
	ResourceAdjustments    []ResourceAdjustment    `json:"resource_adjustments"`
	ScalingRecommendations []ScalingOption         `json:"scaling_recommendations"`
	BottleneckAnalysis     []PerformanceBottleneck `json:"bottleneck_analysis"`
}

type ResourceAdjustment struct {
	Resource    string `json:"resource"`
	Current     string `json:"current"`
	Recommended string `json:"recommended"`
	Rationale   string `json:"rationale"`
}

type ScalingOption struct {
	Type        string `json:"type"`
	Trigger     string `json:"trigger"`
	MinReplicas int    `json:"min_replicas"`
	MaxReplicas int    `json:"max_replicas"`
	TargetValue string `json:"target_value"`
}

type PerformanceBottleneck struct {
	Component  string `json:"component"`
	Issue      string `json:"issue"`
	Impact     string `json:"impact"`
	Resolution string `json:"resolution"`
}

type DeploymentContext struct {
	ManifestsGenerated int      `json:"manifests_generated"`
	ManifestPaths      []string `json:"manifest_paths"`
	ResourceTypes      []string `json:"resource_types"`
	ManifestValidation []string `json:"manifest_validation"`

	DeploymentStatus string   `json:"deployment_status"`
	ResourcesCreated []string `json:"resources_created"`
	ResourcesUpdated []string `json:"resources_updated"`
	DeploymentErrors []string `json:"deployment_errors,omitempty"`

	PodsReady       int      `json:"pods_ready"`
	PodsTotal       int      `json:"pods_total"`
	ServicesHealthy int      `json:"services_healthy"`
	HealthIssues    []string `json:"health_issues,omitempty"`

	ClusterVersion  string   `json:"cluster_version,omitempty"`
	NamespaceExists bool     `json:"namespace_exists"`
	ResourceQuotas  []string `json:"resource_quotas,omitempty"`

	NextStepSuggestions       []string `json:"next_step_suggestions"`
	TroubleshootingTips       []string `json:"troubleshooting_tips,omitempty"`
	MonitoringRecommendations []string `json:"monitoring_recommendations"`

	MonitoringSetup      MonitoringRecommendation `json:"monitoring_setup"`
	RollbackInstructions RollbackGuidance         `json:"rollback_instructions"`
	PerformanceGuidance  PerformanceOptimization  `json:"performance_guidance"`
}

// AtomicDeployKubernetesTool implements atomic Kubernetes deployment using core operations
type AtomicDeployKubernetesTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionStore    services.SessionStore // Focused service interface
	sessionState    services.SessionState // Focused service interface
	fixingMixin     *build.AtomicToolFixingMixin
	analyzer        core.AIAnalyzer
	contextSharer   *build.DefaultContextSharer
	contextEnhancer *build.AIContextEnhancer
	logger          *slog.Logger
}

// NewAtomicDeployKubernetesTool creates a new atomic deploy Kubernetes tool using focused service interfaces

// NewAtomicDeployKubernetesToolWithServices creates a new atomic deploy Kubernetes tool using service container
func NewAtomicDeployKubernetesToolWithServices(adapter core.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicDeployKubernetesTool {
	toolLogger := logger.With("tool", "atomic_deploy_kubernetes")

	// Use focused services directly - no wrapper needed!
	return createAtomicDeployKubernetesTool(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), toolLogger)
}

// createAtomicDeployKubernetesTool is the common creation logic
func createAtomicDeployKubernetesTool(adapter core.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicDeployKubernetesTool {
	contextSharer := build.NewDefaultContextSharer(logger)
	contextEnhancer := build.NewAIContextEnhancer(contextSharer, logger)

	return &AtomicDeployKubernetesTool{
		pipelineAdapter: adapter,
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		fixingMixin:     nil,
		analyzer:        nil,
		contextSharer:   contextSharer,
		contextEnhancer: contextEnhancer,
		logger:          logger,
	}
}

// Validate validates the tool arguments using tag-based validation
func (t *AtomicDeployKubernetesTool) Validate(_ context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

func (t *AtomicDeployKubernetesTool) SetAnalyzer(analyzer interface{}) {
	if aiAnalyzer, ok := analyzer.(core.AIAnalyzer); ok {
		t.analyzer = aiAnalyzer
		t.fixingMixin = build.NewAtomicToolFixingMixin(aiAnalyzer, "atomic_deploy_kubernetes", t.logger)
		t.logger.Info("Deploy tool analyzer and fixing mixin initialized")
	}
}

func (t *AtomicDeployKubernetesTool) ExecuteDeploymentWithFixes(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		t.logger.Info("Executing deployment with automatic fixing enabled",
			"session_id", args.SessionID,
			"fixing_enabled", true)

		var result *AtomicDeployKubernetesResult
		operation := NewOperation(OperationConfig{
			Type:          OperationDeploy,
			Name:          "deploy_kubernetes",
			RetryAttempts: 3,
			Timeout:       5 * time.Minute,
			Logger:        t.logger,
		})

		operation.ExecuteFunc = func(ctx context.Context) error {
			var err error
			result, err = t.executeDeploymentCore(ctx, args)
			if err != nil {
				return err
			}
			if !result.Success {
				if result.ConsolidatedFailureAnalysis != nil {
					return errors.NewError().Messagef("deployment failed: %s", result.ConsolidatedFailureAnalysis.FailureType).Build()
				}
				return errors.NewError().Messagef("deployment failed").Build()
			}
			return nil
		}

		operation.AnalyzeFunc = func(_ context.Context, err error) (error, error) {
			if result != nil && result.ConsolidatedFailureAnalysis != nil {
				return errors.NewError().Messagef("deployment failed at stage: %s", result.ConsolidatedFailureAnalysis.FailureStage).Build(), nil
			}
			return err, nil
		}

		operation.PrepareFunc = func(_ context.Context, fixAttempt interface{}) error {
			t.logger.Info("Applying deployment fix",
				"session_id", args.SessionID)

			return nil
		}

		err := t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, t.pipelineAdapter.GetSessionWorkspace(args.SessionID), operation)
		if err != nil {
			if result != nil {
				result.Success = false
				if t.contextEnhancer != nil {
					_, contextErr := t.contextEnhancer.EnhanceContext(ctx, mcptypes.AIContextEnhanceConfig{
						SessionID:     args.SessionID,
						ToolName:      "atomic_deploy_kubernetes",
						OperationType: "deployment",
						ToolResult:    result,
						ToolError:     err,
					})
					if contextErr != nil {
						t.logger.Warn("Failed to enhance context after deployment failure", "error", contextErr)
					}
				}
				return result, nil
			}
			return nil, err
		}

		if t.contextEnhancer != nil && result != nil {
			_, contextErr := t.contextEnhancer.EnhanceContext(ctx, mcptypes.AIContextEnhanceConfig{
				SessionID:     args.SessionID,
				ToolName:      "atomic_deploy_kubernetes",
				OperationType: "deployment",
				ToolResult:    result,
				ToolError:     nil,
			})
			if contextErr != nil {
				t.logger.Warn("Failed to enhance context after deployment success", "error", contextErr)
			}
		}

		return result, nil
	}

	return t.executeDeploymentCore(ctx, args)
}

func (t *AtomicDeployKubernetesTool) executeDeploymentCore(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	startTime := time.Now()

	result := &AtomicDeployKubernetesResult{
		BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
		BaseAIContextResult: core.NewBaseAIContextResult("deploy", false, 0), // Duration and success will be updated later
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		AppName:             args.AppName,
		Namespace:           args.Namespace,
		Replicas:            args.Replicas,
		Port:                args.Port,
		ServiceType:         args.ServiceType,
		WorkspaceDir:        "",
		DeploymentContext:   &DeploymentContext{},
	}

	// Get session using focused service interface
	sessionData, err := t.sessionStore.Get(ctx, args.SessionID)
	if err != nil {
		result.Success = false
		return result, errors.NewError().Message("failed to get session").Cause(err).WithLocation().Build()
	}

	// Convert to core.SessionState for compatibility
	session := &core.SessionState{
		SessionID: sessionData.ID,
		Metadata:  sessionData.Metadata,
	}
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	if result.AppName == "" {
		result.AppName = extractAppNameFromImage(result.ImageRef)
	}
	if result.Namespace == "" {
		result.Namespace = "default"
	}
	if result.Replicas == 0 {
		result.Replicas = 1
	}
	if result.Port == 0 {
		result.Port = 80
	}
	if result.ServiceType == "" {
		result.ServiceType = "ClusterIP"
	}

	if err := t.performManifestGeneration(ctx, session, args, result, nil); err != nil {
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil // Return result with error info
	}

	if !args.GenerateOnly {
		if err := t.performDeployment(ctx, session, args, result, nil); err != nil {
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil // Return result with error info
		}

		if args.WaitForReady {
			if err := t.performHealthCheck(ctx, session, args, result, nil); err != nil {
				t.logger.Warn("Health check failed, but deployment succeeded", "error", err)
			}
		}

		if err := t.updateSessionState(session, result); err != nil {
			t.logger.Warn("Failed to update session state", "error", err)
		}
	}

	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.TotalDuration = time.Since(startTime)

	if t.contextEnhancer != nil {
		_, contextErr := t.contextEnhancer.EnhanceContext(ctx, mcptypes.AIContextEnhanceConfig{
			SessionID:     args.SessionID,
			ToolName:      "atomic_deploy_kubernetes",
			OperationType: "deployment_complete",
			ToolResult:    result,
			ToolError:     nil,
		})
		if contextErr != nil {
			t.logger.Warn("Failed to enhance context after deployment completion", "error", contextErr)
		}
	}

	return result, nil
}

func (t *AtomicDeployKubernetesTool) ExecuteDeployment(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	return t.ExecuteDeploymentWithFixes(ctx, args)
}

func (t *AtomicDeployKubernetesTool) ExecuteWithContext(_ *server.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	return t.ExecuteDeployment(context.Background(), args)
}

func extractAppNameFromImage(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if idx := strings.Index(lastPart, ":"); idx > 0 {
			return lastPart[:idx]
		}
		return lastPart
	}
	// Note: This would typically use the logger from context, but for now we'll handle the fallback gracefully
	return "unknown-app"
}

func (t *AtomicDeployKubernetesTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_deploy_kubernetes",
		Description:  "Deploys containerized applications to Kubernetes with manifest generation, health checks, and rollback support",
		Version:      "1.0.0",
		Category:     api.ToolCategory("kubernetes"),
		Dependencies: []string{"kubernetes", "kubectl"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
			"long_running",
		},
		Requirements: []string{"kubernetes_cluster", "kubectl_config"},
		Tags:         []string{"kubernetes", "deployment", "atomic"},
		Status:       api.ToolStatus("active"),
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Schema returns the JSON schema for this tool
func (t *AtomicDeployKubernetesTool) Schema() interface{} {
	return AtomicDeployKubernetesArgsSchema
}

func (t *AtomicDeployKubernetesTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	var deployArgs AtomicDeployKubernetesArgs

	switch v := args.(type) {
	case AtomicDeployKubernetesArgs:
		deployArgs = v
	case *AtomicDeployKubernetesArgs:
		deployArgs = *v
	case toolstypes.AtomicDeployKubernetesParams:
		// Convert from typed parameters package to internal args structure
		deployArgs = AtomicDeployKubernetesArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: v.SessionID,
				DryRun:    v.DryRun,
			},
			ImageRef:       v.ImageRef,
			Namespace:      v.Namespace,
			AppName:        v.AppName,
			Replicas:       v.Replicas,
			Port:           v.Port,
			ServiceType:    v.ServiceType,
			IncludeIngress: v.IncludeIngress,
			Environment:    v.Environment,
			CPURequest:     v.CPURequest,
			MemoryRequest:  v.MemoryRequest,
			CPULimit:       v.CPULimit,
			MemoryLimit:    v.MemoryLimit,
		}
	case *toolstypes.AtomicDeployKubernetesParams:
		// Convert from pointer to typed parameters
		deployArgs = AtomicDeployKubernetesArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: v.SessionID,
				DryRun:    v.DryRun,
			},
			ImageRef:       v.ImageRef,
			Namespace:      v.Namespace,
			AppName:        v.AppName,
			Replicas:       v.Replicas,
			Port:           v.Port,
			ServiceType:    v.ServiceType,
			IncludeIngress: v.IncludeIngress,
			Environment:    v.Environment,
			CPURequest:     v.CPURequest,
			MemoryRequest:  v.MemoryRequest,
			CPULimit:       v.CPULimit,
			MemoryLimit:    v.MemoryLimit,
		}
	default:
		return nil, errors.NewError().Messagef("invalid argument type for atomic_deploy_kubernetes: expected AtomicDeployKubernetesArgs or AtomicDeployKubernetesParams, received %T", args).Build()
	}

	return t.ExecuteDeployment(ctx, deployArgs)
}

func (t *AtomicDeployKubernetesTool) GetName() string {
	return t.GetMetadata().Name
}

func (t *AtomicDeployKubernetesTool) GetDescription() string {
	return t.GetMetadata().Description
}

// Name implements the api.Tool interface
func (t *AtomicDeployKubernetesTool) Name() string {
	return "atomic_deploy_kubernetes"
}

// Description implements the api.Tool interface
func (t *AtomicDeployKubernetesTool) Description() string {
	return "Deploys applications to Kubernetes with atomic session management and AI-driven error fixing"
}

func (t *AtomicDeployKubernetesTool) GetVersion() string {
	return t.GetMetadata().Version
}

func (t *AtomicDeployKubernetesTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}
