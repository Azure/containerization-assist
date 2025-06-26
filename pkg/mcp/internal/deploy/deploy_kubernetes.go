package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/internal"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicDeployKubernetesArgs defines arguments for atomic Kubernetes deployment
type AtomicDeployKubernetesArgs struct {
	types.BaseToolArgs

	// Deployment target
	ImageRef  string `json:"image_ref" jsonschema:"required,pattern=^[a-zA-Z0-9][a-zA-Z0-9._/-]*:[a-zA-Z0-9][a-zA-Z0-9._-]*$" description:"Container image reference (e.g., myregistry.azurecr.io/myapp:latest)"`
	AppName   string `json:"app_name,omitempty" jsonschema:"pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$" description:"Application name (default: from image name)"`
	Namespace string `json:"namespace,omitempty" jsonschema:"pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$" description:"Kubernetes namespace (default: default)"`

	// Deployment configuration
	Replicas       int               `json:"replicas,omitempty" jsonschema:"minimum=1,maximum=100" description:"Number of replicas (default: 1)"`
	Port           int               `json:"port,omitempty" jsonschema:"minimum=1,maximum=65535" description:"Application port (default: 80)"`
	ServiceType    string            `json:"service_type,omitempty" jsonschema:"enum=ClusterIP,enum=NodePort,enum=LoadBalancer" description:"Service type: ClusterIP, NodePort, LoadBalancer (default: ClusterIP)"`
	IncludeIngress bool              `json:"include_ingress,omitempty" description:"Generate and deploy Ingress resource"`
	Environment    map[string]string `json:"environment,omitempty" description:"Environment variables"`

	// Resource requirements
	CPURequest    string `json:"cpu_request,omitempty" jsonschema:"pattern=^[0-9]+(m|[kMGT])?$" description:"CPU request (e.g., 100m)"`
	MemoryRequest string `json:"memory_request,omitempty" jsonschema:"pattern=^[0-9]+([kMGT]i?)?$" description:"Memory request (e.g., 128Mi)"`
	CPULimit      string `json:"cpu_limit,omitempty" jsonschema:"pattern=^[0-9]+(m|[kMGT])?$" description:"CPU limit (e.g., 500m)"`
	MemoryLimit   string `json:"memory_limit,omitempty" jsonschema:"pattern=^[0-9]+([kMGT]i?)?$" description:"Memory limit (e.g., 512Mi)"`

	// Deployment behavior
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

	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	// Deployment configuration
	ImageRef    string `json:"image_ref"`
	AppName     string `json:"app_name"`
	Namespace   string `json:"namespace"`
	Replicas    int    `json:"replicas"`
	Port        int    `json:"port"`
	ServiceType string `json:"service_type"`

	// Generation results from core operations
	ManifestResult *kubernetes.ManifestGenerationResult `json:"manifest_result"`

	// Deployment results from core operations (if deployed)
	DeploymentResult *kubernetes.DeploymentResult `json:"deployment_result,omitempty"`

	// Health check results (if deployed and waited)
	HealthResult *kubernetes.HealthCheckResult `json:"health_result,omitempty"`

	// Timing information
	GenerationDuration  time.Duration `json:"generation_duration"`
	DeploymentDuration  time.Duration `json:"deployment_duration,omitempty"`
	HealthCheckDuration time.Duration `json:"health_check_duration,omitempty"`
	TotalDuration       time.Duration `json:"total_duration"`

	// Rich context for Claude reasoning
	DeploymentContext *DeploymentContext `json:"deployment_context"`

	// Failure analysis for AI reasoning when deployment fails
	FailureAnalysis *DeploymentFailureAnalysis `json:"failure_analysis,omitempty"`

	// Dry-run preview output (when dry_run=true)
	DryRunPreview string `json:"dry_run_preview,omitempty"`
}

// Unified AI Context Interface Implementations
// All AI context methods are now provided by embedded internal.BaseAIContextResult

// DeploymentFailureAnalysis provides rich failure analysis for AI reasoning
type DeploymentFailureAnalysis struct {
	// Failure classification
	FailureType    string   `json:"failure_type"`    // network, authentication, resources, configuration, image, timeout
	FailureStage   string   `json:"failure_stage"`   // manifest_generation, deployment, health_check, rollback
	RootCauses     []string `json:"root_causes"`     // Identified root causes
	ImpactSeverity string   `json:"impact_severity"` // low, medium, high, critical

	// Remediation strategies
	ImmediateActions      []DeploymentRemediationAction `json:"immediate_actions"`
	AlternativeApproaches []DeploymentAlternative       `json:"alternative_approaches"`

	// Monitoring and observability guidance
	DiagnosticCommands []DiagnosticCommand      `json:"diagnostic_commands"`
	MonitoringSetup    MonitoringRecommendation `json:"monitoring_setup"`

	// Rollback guidance
	RollbackStrategy RollbackGuidance `json:"rollback_strategy"`

	// Performance optimization suggestions
	PerformanceTuning PerformanceOptimization `json:"performance_tuning"`
}

// Supporting types for failure analysis...
type DeploymentRemediationAction struct {
	Priority    int    `json:"priority"`    // 1 (highest) to 5 (lowest)
	Action      string `json:"action"`      // Brief action description
	Command     string `json:"command"`     // Executable command
	Description string `json:"description"` // Detailed explanation
	Expected    string `json:"expected"`    // Expected outcome
	RiskLevel   string `json:"risk_level"`  // low, medium, high
}

type DeploymentAlternative struct {
	Strategy     string   `json:"strategy"`      // rolling, blue-green, canary, recreate
	Pros         []string `json:"pros"`          // Benefits of this approach
	Cons         []string `json:"cons"`          // Drawbacks of this approach
	Complexity   string   `json:"complexity"`    // low, medium, high
	TimeToValue  string   `json:"time_to_value"` // immediate, short, medium, long
	ResourceReqs string   `json:"resource_reqs"` // Description of additional resources needed
}

type DiagnosticCommand struct {
	Purpose     string `json:"purpose"`     // What this command diagnoses
	Command     string `json:"command"`     // The kubectl/docker command
	Explanation string `json:"explanation"` // How to interpret results
}

type MonitoringRecommendation struct {
	HealthChecks    []HealthCheckSetup     `json:"health_checks"`
	MetricsToTrack  []MetricRecommendation `json:"metrics_to_track"`
	AlertingRules   []AlertingRule         `json:"alerting_rules"`
	LoggingStrategy LoggingSetup           `json:"logging_strategy"`
}

type HealthCheckSetup struct {
	Type         string `json:"type"`          // readiness, liveness, startup
	Endpoint     string `json:"endpoint"`      // HTTP endpoint path
	Port         int    `json:"port"`          // Port number
	InitialDelay int    `json:"initial_delay"` // Initial delay in seconds
	Period       int    `json:"period"`        // Check period in seconds
	Timeout      int    `json:"timeout"`       // Timeout in seconds
}

type MetricRecommendation struct {
	Name        string `json:"name"`        // Metric name
	Type        string `json:"type"`        // counter, gauge, histogram
	Description string `json:"description"` // What this metric measures
	Threshold   string `json:"threshold"`   // Alert threshold
}

type AlertingRule struct {
	Name        string `json:"name"`        // Alert rule name
	Condition   string `json:"condition"`   // Alert condition
	Severity    string `json:"severity"`    // info, warning, critical
	Description string `json:"description"` // What this alert means
}

type LoggingSetup struct {
	LogLevel       string   `json:"log_level"`       // debug, info, warn, error
	StructuredLogs bool     `json:"structured_logs"` // Whether to use structured logging
	LogFields      []string `json:"log_fields"`      // Important fields to log
	Aggregation    string   `json:"aggregation"`     // How to aggregate logs
}

type RollbackGuidance struct {
	AutoRollbackTriggers []string `json:"auto_rollback_triggers"` // Conditions for automatic rollback
	ManualRollbackSteps  []string `json:"manual_rollback_steps"`  // Manual rollback procedure
	RollbackRisk         string   `json:"rollback_risk"`          // low, medium, high
	DataIntegrity        string   `json:"data_integrity"`         // Impact on data consistency
	DowntimeEstimate     string   `json:"downtime_estimate"`      // Expected downtime duration
}

type PerformanceOptimization struct {
	ResourceAdjustments    []ResourceAdjustment    `json:"resource_adjustments"`
	ScalingRecommendations []ScalingOption         `json:"scaling_recommendations"`
	BottleneckAnalysis     []PerformanceBottleneck `json:"bottleneck_analysis"`
}

type ResourceAdjustment struct {
	Resource    string `json:"resource"`    // cpu, memory, storage
	Current     string `json:"current"`     // Current setting
	Recommended string `json:"recommended"` // Recommended setting
	Rationale   string `json:"rationale"`   // Why this change is needed
}

type ScalingOption struct {
	Type        string `json:"type"`         // horizontal, vertical, cluster
	Trigger     string `json:"trigger"`      // CPU, memory, custom metric
	MinReplicas int    `json:"min_replicas"` // Minimum replicas
	MaxReplicas int    `json:"max_replicas"` // Maximum replicas
	TargetValue string `json:"target_value"` // Target metric value
}

type PerformanceBottleneck struct {
	Component  string `json:"component"`  // pod, service, ingress, storage
	Issue      string `json:"issue"`      // Description of the bottleneck
	Impact     string `json:"impact"`     // Performance impact description
	Resolution string `json:"resolution"` // How to resolve this bottleneck
}

type DeploymentContext struct {
	// Manifest analysis
	ManifestsGenerated int      `json:"manifests_generated"`
	ManifestPaths      []string `json:"manifest_paths"`
	ResourceTypes      []string `json:"resource_types"`
	ManifestValidation []string `json:"manifest_validation"`

	// Deployment analysis
	DeploymentStatus string   `json:"deployment_status"`
	ResourcesCreated []string `json:"resources_created"`
	ResourcesUpdated []string `json:"resources_updated"`
	DeploymentErrors []string `json:"deployment_errors,omitempty"`

	// Health analysis
	PodsReady       int      `json:"pods_ready"`
	PodsTotal       int      `json:"pods_total"`
	ServicesHealthy int      `json:"services_healthy"`
	HealthIssues    []string `json:"health_issues,omitempty"`

	// Kubernetes insights
	ClusterVersion  string   `json:"cluster_version,omitempty"`
	NamespaceExists bool     `json:"namespace_exists"`
	ResourceQuotas  []string `json:"resource_quotas,omitempty"`

	// Next step suggestions and guidance
	NextStepSuggestions       []string `json:"next_step_suggestions"`
	TroubleshootingTips       []string `json:"troubleshooting_tips,omitempty"`
	MonitoringRecommendations []string `json:"monitoring_recommendations"`

	// Enhanced monitoring and observability guidance
	ObservabilitySetup   MonitoringRecommendation `json:"observability_setup"`
	RollbackInstructions RollbackGuidance         `json:"rollback_instructions"`
	PerformanceGuidance  PerformanceOptimization  `json:"performance_guidance"`
}

// AtomicDeployKubernetesTool implements atomic Kubernetes deployment using core operations
type AtomicDeployKubernetesTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	fixingMixin     *build.AtomicToolFixingMixin
	logger          zerolog.Logger
}

// NewAtomicDeployKubernetesTool creates a new atomic deploy Kubernetes tool
func NewAtomicDeployKubernetesTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicDeployKubernetesTool {
	return &AtomicDeployKubernetesTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		fixingMixin:     nil, // Will be set via SetAnalyzer
		logger:          logger.With().Str("tool", "atomic_deploy_kubernetes").Logger(),
	}
}

// SetAnalyzer sets the analyzer for the tool
func (t *AtomicDeployKubernetesTool) SetAnalyzer(analyzer interface{}) {
	// Note: This method is required for factory compatibility
	// The deploy tool doesn't currently use the analyzer directly
}

// ExecuteDeployment runs the atomic Kubernetes deployment (legacy method)
func (t *AtomicDeployKubernetesTool) ExecuteDeployment(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
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

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result.Success = false
		return result, fmt.Errorf("failed to get session: %w", err)
	}
	session := sessionInterface.(*sessiontypes.SessionState)
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	// Set defaults
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

	// Step 1: Generate manifests
	if err := t.performManifestGeneration(ctx, session, args, result, nil); err != nil {
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil // Return result with error info
	}

	// Step 2: Deploy (unless generate-only)
	if !args.GenerateOnly {
		if err := t.performDeployment(ctx, session, args, result, nil); err != nil {
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil // Return result with error info
		}

		// Step 3: Health check (if deployed and wait requested)
		if args.WaitForReady {
			if err := t.performHealthCheck(ctx, session, args, result, nil); err != nil {
				// Health check failure doesn't fail the deployment
				t.logger.Warn().Err(err).Msg("Health check failed, but deployment succeeded")
			}
		}

		// Update session state
		if err := t.updateSessionState(session, result); err != nil {
			t.logger.Warn().Err(err).Msg("Failed to update session state")
		}
	}

	// Mark success and finalize
	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.TotalDuration = time.Since(startTime)

	return result, nil
}

// ExecuteWithContext executes the tool with GoMCP server context for native progress tracking
func (t *AtomicDeployKubernetesTool) ExecuteWithContext(serverCtx *server.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	// Delegate to main execution method
	return t.ExecuteDeployment(context.Background(), args)
}

func extractAppNameFromImage(imageRef string) string {
	// Simple extraction from image reference
	// Example: "myregistry.com/myapp:v1.0" -> "myapp"
	parts := strings.Split(imageRef, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if idx := strings.Index(lastPart, ":"); idx > 0 {
			return lastPart[:idx]
		}
		return lastPart
	}
	return "app" // fallback
}

// GetMetadata returns comprehensive tool metadata
func (t *AtomicDeployKubernetesTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
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

// Execute implements unified Tool interface
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

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicDeployKubernetesTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicDeployKubernetesTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicDeployKubernetesTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicDeployKubernetesTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}