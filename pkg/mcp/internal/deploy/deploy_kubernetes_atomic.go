package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/core"

	// mcp import removed - using mcptypes
	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicDeployKubernetesArgs defines arguments for atomic Kubernetes deployment
type AtomicDeployKubernetesArgs struct {
	types.BaseToolArgs

	ImageRef  string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Container image reference (e.g., myregistry.azurecr.io/myapp:latest)"`
	AppName   string `json:"app_name,omitempty" jsonschema:"pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$" description:"Application name (default: from image name)"`
	Namespace string `json:"namespace,omitempty" jsonschema:"pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$" description:"Kubernetes namespace (default: default)"`

	Replicas       int               `json:"replicas,omitempty" jsonschema:"minimum=1,maximum=100" description:"Number of replicas (default: 1)"`
	Port           int               `json:"port,omitempty" jsonschema:"minimum=1,maximum=65535" description:"Application port (default: 80)"`
	ServiceType    string            `json:"service_type,omitempty" jsonschema:"enum=ClusterIP,enum=NodePort,enum=LoadBalancer" description:"Service type: ClusterIP, NodePort, LoadBalancer (default: ClusterIP)"`
	IncludeIngress bool              `json:"include_ingress,omitempty" description:"Generate and deploy Ingress resource"`
	Environment    map[string]string `json:"environment,omitempty" description:"Environment variables"`

	CPURequest    string `json:"cpu_request,omitempty" jsonschema:"pattern=^[0-9]+(m|[kMGT])?$" description:"CPU request (e.g., 100m)"`
	MemoryRequest string `json:"memory_request,omitempty" jsonschema:"pattern=^[0-9]+([kMGT]i?)?$" description:"Memory request (e.g., 128Mi)"`
	CPULimit      string `json:"cpu_limit,omitempty" jsonschema:"pattern=^[0-9]+(m|[kMGT])?$" description:"CPU limit (e.g., 500m)"`
	MemoryLimit   string `json:"memory_limit,omitempty" jsonschema:"pattern=^[0-9]+([kMGT]i?)?$" description:"Memory limit (e.g., 512Mi)"`

	GenerateOnly bool `json:"generate_only,omitempty" description:"Only generate manifests, don't deploy"`
	WaitForReady bool `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready (default: true)"`
	WaitTimeout  int  `json:"wait_timeout,omitempty" jsonschema:"minimum=30,maximum=3600" description:"Wait timeout in seconds (default: 300)"`
	DryRun       bool `json:"dry_run,omitempty" description:"Preview changes without applying (shows kubectl diff output)"`
}

// AtomicDeployKubernetesResult defines the response from atomic Kubernetes deployment
type AtomicDeployKubernetesResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult      // Embed AI context methods
	Success                      bool `json:"success"`

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

	FailureAnalysis *DeploymentFailureAnalysis `json:"failure_analysis,omitempty"`

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

	ObservabilitySetup   MonitoringRecommendation `json:"observability_setup"`
	RollbackInstructions RollbackGuidance         `json:"rollback_instructions"`
	PerformanceGuidance  PerformanceOptimization  `json:"performance_guidance"`
}

// AtomicDeployKubernetesTool implements atomic Kubernetes deployment using core operations
type AtomicDeployKubernetesTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  core.ToolSessionManager
	fixingMixin     *build.AtomicToolFixingMixin
	analyzer        core.AIAnalyzer
	contextSharer   *build.DefaultContextSharer
	contextEnhancer *build.AIContextEnhancer
	logger          zerolog.Logger
}

// NewAtomicDeployKubernetesTool creates a new atomic deploy Kubernetes tool
func NewAtomicDeployKubernetesTool(adapter mcptypes.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicDeployKubernetesTool {
	toolLogger := logger.With().Str("tool", "atomic_deploy_kubernetes").Logger()

	contextSharer := build.NewDefaultContextSharer(toolLogger)
	contextEnhancer := build.NewAIContextEnhancer(contextSharer, toolLogger)

	return &AtomicDeployKubernetesTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		fixingMixin:     nil,
		analyzer:        nil,
		contextSharer:   contextSharer,
		contextEnhancer: contextEnhancer,
		logger:          toolLogger,
	}
}

func (t *AtomicDeployKubernetesTool) SetAnalyzer(analyzer interface{}) {
	if aiAnalyzer, ok := analyzer.(core.AIAnalyzer); ok {
		t.analyzer = aiAnalyzer
		t.fixingMixin = build.NewAtomicToolFixingMixin(aiAnalyzer, "atomic_deploy_kubernetes", t.logger)
		t.logger.Info().Msg("Deploy tool analyzer and fixing mixin initialized")
	}
}

func (t *AtomicDeployKubernetesTool) ExecuteDeploymentWithFixes(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	if t.fixingMixin != nil && !args.DryRun {
		t.logger.Info().
			Str("session_id", args.SessionID).
			Bool("fixing_enabled", true).
			Msg("Executing deployment with automatic fixing enabled")

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
				if result.FailureAnalysis != nil {
					return fmt.Errorf("deployment failed: %s", result.FailureAnalysis.FailureType)
				}
				return fmt.Errorf("deployment failed")
			}
			return nil
		}

		operation.AnalyzeFunc = func(_ context.Context, err error) (error, error) {
			if result != nil && result.FailureAnalysis != nil {
				return fmt.Errorf("deployment failed at stage: %s", result.FailureAnalysis.FailureStage), nil
			}
			return err, nil
		}

		operation.PrepareFunc = func(_ context.Context, fixAttempt interface{}) error {
			t.logger.Info().
				Str("session_id", args.SessionID).
				Msg("Applying deployment fix")

			return nil
		}

		err := t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, t.pipelineAdapter.GetSessionWorkspace(args.SessionID), operation)
		if err != nil {
			if result != nil {
				result.Success = false
				if t.contextEnhancer != nil {
					_, contextErr := t.contextEnhancer.EnhanceContext(ctx, args.SessionID, "atomic_deploy_kubernetes", "deployment", result, err)
					if contextErr != nil {
						t.logger.Warn().Err(contextErr).Msg("Failed to enhance context after deployment failure")
					}
				}
				return result, nil
			}
			return nil, err
		}

		if t.contextEnhancer != nil && result != nil {
			_, contextErr := t.contextEnhancer.EnhanceContext(ctx, args.SessionID, "atomic_deploy_kubernetes", "deployment", result, nil)
			if contextErr != nil {
				t.logger.Warn().Err(contextErr).Msg("Failed to enhance context after deployment success")
			}
		}

		return result, nil
	}

	return t.executeDeploymentCore(ctx, args)
}

func (t *AtomicDeployKubernetesTool) executeDeploymentCore(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	startTime := time.Now()

	result := &AtomicDeployKubernetesResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_deploy_kubernetes", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("deploy", false, 0), // Duration and success will be updated later
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

	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result.Success = false
		return result, fmt.Errorf("failed to get session: %w", err)
	}
	session := sessionInterface.(*core.SessionState)
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
				t.logger.Warn().Err(err).Msg("Health check failed, but deployment succeeded")
			}
		}

		if err := t.updateSessionState(session, result); err != nil {
			t.logger.Warn().Err(err).Msg("Failed to update session state")
		}
	}

	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.TotalDuration = time.Since(startTime)

	if t.contextEnhancer != nil {
		_, contextErr := t.contextEnhancer.EnhanceContext(ctx, args.SessionID, "atomic_deploy_kubernetes", "deployment_complete", result, nil)
		if contextErr != nil {
			t.logger.Warn().Err(contextErr).Msg("Failed to enhance context after deployment completion")
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
	zerolog.Ctx(context.Background()).Warn().Str("imageRef", imageRef).Msg("Failed to extract app name from image reference")
	return "unknown-app"
}

func (t *AtomicDeployKubernetesTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:         "atomic_deploy_kubernetes",
		Description:  "Deploys containerized applications to Kubernetes with manifest generation, health checks, and rollback support",
		Version:      "1.0.0",
		Category:     "kubernetes",
		Dependencies: []string{"kubernetes", "kubectl"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
			"long_running",
		},
		Requirements: []string{"kubernetes_cluster", "kubectl_config"},
		Parameters: map[string]string{
			"image_ref":       "required - Container image reference",
			"app_name":        "optional - Application name (default: from image)",
			"namespace":       "optional - Kubernetes namespace (default: default)",
			"replicas":        "optional - Number of replicas (default: 1)",
			"port":            "optional - Application port (default: 80)",
			"service_type":    "optional - Service type (default: ClusterIP)",
			"include_ingress": "optional - Generate Ingress resource",
			"environment":     "optional - Environment variables",
			"cpu_request":     "optional - CPU request",
			"memory_request":  "optional - Memory request",
			"cpu_limit":       "optional - CPU limit",
			"memory_limit":    "optional - Memory limit",
			"generate_only":   "optional - Only generate manifests",
			"wait_for_ready":  "optional - Wait for pods to be ready",
			"wait_timeout":    "optional - Wait timeout in seconds",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "basic_deployment",
				Description: "Deploy a basic application to Kubernetes",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"image_ref":  "nginx:latest",
					"app_name":   "my-nginx",
					"namespace":  "default",
				},
				Output: map[string]interface{}{
					"success":          true,
					"deployment_ready": true,
					"pod_count":        3,
				},
			},
		},
	}
}

func (t *AtomicDeployKubernetesTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		return nil, utils.NewWithData("invalid_arguments", "Invalid argument type for atomic_deploy_kubernetes", map[string]interface{}{
			"expected": "AtomicDeployKubernetesArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	return t.ExecuteDeployment(ctx, deployArgs)
}

func (t *AtomicDeployKubernetesTool) GetName() string {
	return t.GetMetadata().Name
}

func (t *AtomicDeployKubernetesTool) GetDescription() string {
	return t.GetMetadata().Description
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
