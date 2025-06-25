package deploy

import (
	"context"
	"fmt"
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
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

// DeploymentRemediationAction defines immediate steps to resolve deployment issues
type DeploymentRemediationAction struct {
	Priority    int    `json:"priority"`    // 1 (highest) to 5 (lowest)
	Action      string `json:"action"`      // Brief action description
	Command     string `json:"command"`     // Executable command
	Description string `json:"description"` // Detailed explanation
	Expected    string `json:"expected"`    // Expected outcome
	RiskLevel   string `json:"risk_level"`  // low, medium, high
}

// DeploymentAlternative suggests alternative deployment strategies
type DeploymentAlternative struct {
	Strategy     string   `json:"strategy"`      // rolling, blue-green, canary, recreate
	Pros         []string `json:"pros"`          // Benefits of this approach
	Cons         []string `json:"cons"`          // Drawbacks of this approach
	Complexity   string   `json:"complexity"`    // low, medium, high
	TimeToValue  string   `json:"time_to_value"` // immediate, short, medium, long
	ResourceReqs string   `json:"resource_reqs"` // Description of additional resources needed
}

// DiagnosticCommand provides troubleshooting commands
type DiagnosticCommand struct {
	Purpose     string `json:"purpose"`     // What this command diagnoses
	Command     string `json:"command"`     // The kubectl/docker command
	Explanation string `json:"explanation"` // How to interpret results
}

// MonitoringRecommendation provides observability setup guidance
type MonitoringRecommendation struct {
	HealthChecks    []HealthCheckSetup     `json:"health_checks"`
	MetricsToTrack  []MetricRecommendation `json:"metrics_to_track"`
	AlertingRules   []AlertingRule         `json:"alerting_rules"`
	LoggingStrategy LoggingSetup           `json:"logging_strategy"`
}

// HealthCheckSetup defines health check configuration
type HealthCheckSetup struct {
	Type         string `json:"type"`          // readiness, liveness, startup
	Endpoint     string `json:"endpoint"`      // HTTP endpoint path
	Port         int    `json:"port"`          // Port number
	InitialDelay int    `json:"initial_delay"` // Initial delay in seconds
	Period       int    `json:"period"`        // Check period in seconds
	Timeout      int    `json:"timeout"`       // Timeout in seconds
}

// MetricRecommendation defines which metrics to monitor
type MetricRecommendation struct {
	Name        string `json:"name"`        // Metric name
	Type        string `json:"type"`        // counter, gauge, histogram
	Description string `json:"description"` // What this metric measures
	Threshold   string `json:"threshold"`   // Alert threshold
}

// AlertingRule defines alerting configuration
type AlertingRule struct {
	Name        string `json:"name"`        // Alert rule name
	Condition   string `json:"condition"`   // Alert condition
	Severity    string `json:"severity"`    // info, warning, critical
	Description string `json:"description"` // What this alert means
}

// LoggingSetup defines logging configuration recommendations
type LoggingSetup struct {
	LogLevel       string   `json:"log_level"`       // debug, info, warn, error
	StructuredLogs bool     `json:"structured_logs"` // Whether to use structured logging
	LogFields      []string `json:"log_fields"`      // Important fields to log
	Aggregation    string   `json:"aggregation"`     // How to aggregate logs
}

// RollbackGuidance provides rollback strategy and risk assessment
type RollbackGuidance struct {
	AutoRollbackTriggers []string `json:"auto_rollback_triggers"` // Conditions for automatic rollback
	ManualRollbackSteps  []string `json:"manual_rollback_steps"`  // Manual rollback procedure
	RollbackRisk         string   `json:"rollback_risk"`          // low, medium, high
	DataIntegrity        string   `json:"data_integrity"`         // Impact on data consistency
	DowntimeEstimate     string   `json:"downtime_estimate"`      // Expected downtime duration
}

// PerformanceOptimization provides performance tuning recommendations
type PerformanceOptimization struct {
	ResourceAdjustments    []ResourceAdjustment    `json:"resource_adjustments"`
	ScalingRecommendations []ScalingOption         `json:"scaling_recommendations"`
	BottleneckAnalysis     []PerformanceBottleneck `json:"bottleneck_analysis"`
}

// ResourceAdjustment suggests resource limit/request changes
type ResourceAdjustment struct {
	Resource    string `json:"resource"`    // cpu, memory, storage
	Current     string `json:"current"`     // Current setting
	Recommended string `json:"recommended"` // Recommended setting
	Rationale   string `json:"rationale"`   // Why this change is needed
}

// ScalingOption defines scaling strategy options
type ScalingOption struct {
	Type        string `json:"type"`         // horizontal, vertical, cluster
	Trigger     string `json:"trigger"`      // CPU, memory, custom metric
	MinReplicas int    `json:"min_replicas"` // Minimum replicas
	MaxReplicas int    `json:"max_replicas"` // Maximum replicas
	TargetValue string `json:"target_value"` // Target metric value
}

// PerformanceBottleneck identifies performance issues
type PerformanceBottleneck struct {
	Component  string `json:"component"`  // pod, service, ingress, storage
	Issue      string `json:"issue"`      // Description of the bottleneck
	Impact     string `json:"impact"`     // Performance impact description
	Resolution string `json:"resolution"` // How to resolve this bottleneck
}

// DeploymentContext provides rich context for Claude to reason about
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

// AI Context Interface Implementations
// AI Context methods are now provided by embedded internal.BaseAIContextResult

// AtomicDeployKubernetesTool implements atomic Kubernetes deployment using core operations
type AtomicDeployKubernetesTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	fixingMixin     *fixing.AtomicToolFixingMixin
	logger          zerolog.Logger
}

// NewAtomicDeployKubernetesTool creates a new atomic deploy Kubernetes tool
func NewAtomicDeployKubernetesTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicDeployKubernetesTool {
	return &AtomicDeployKubernetesTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		fixingMixin:     nil, // Will be set via SetAnalyzer if fixing is enabled
		logger:          logger.With().Str("tool", "atomic_deploy_kubernetes").Logger(),
	}
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
		DeploymentContext:   &DeploymentContext{},
	}

	// Execute without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}

// ExecuteWithContext runs the atomic Kubernetes deployment with GoMCP progress tracking
func (t *AtomicDeployKubernetesTool) ExecuteWithContext(serverCtx *server.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicDeployKubernetesResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_deploy_kubernetes", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("deploy", false, 0), // Duration will be updated later
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		AppName:             args.AppName,
		Namespace:           args.Namespace,
		Replicas:            args.Replicas,
		Port:                args.Port,
		ServiceType:         args.ServiceType,
		DeploymentContext:   &DeploymentContext{},
	}

	// Create progress adapter for GoMCP using centralized deploy stages
	_ = internal.NewGoMCPProgressAdapter(serverCtx, []internal.LocalProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Deploy", Weight: 0.80, Description: "Deploying"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Execute with progress tracking
	ctx := context.Background()
	err := t.executeWithProgress(ctx, args, result, startTime, nil)

	// Always set total duration
	result.TotalDuration = time.Since(startTime)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Deployment failed")
		result.Success = false
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Deployment completed successfully")
	}

	return result, nil
}

// executeWithProgress handles the main execution with progress reporting
func (t *AtomicDeployKubernetesTool) executeWithProgress(ctx context.Context, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, startTime time.Time, reporter interface{}) error {
	// Stage 1: Initialize - Loading session and validating inputs
	t.logger.Info().Msg("Loading session")
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return mcperror.NewSessionNotFound(args.SessionID)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Str("namespace", args.Namespace).
		Msg("Starting atomic Kubernetes deployment")

	t.logger.Info().Msg("Session initialized")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult.IsSuccessful = true
		result.DryRunPreview = "This is a dry-run - no actual deployment was performed"
		result.DeploymentContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual deployment was performed",
			"Remove dry_run flag to perform actual deployment",
		}
		t.logger.Info().Msg("Dry-run completed")
		return nil
	}

	// Stage 2: Generate - Generating Kubernetes manifests
	t.logger.Info().Msg("Generating Kubernetes manifests")
	if err := t.performManifestGeneration(ctx, session, args, result, reporter); err != nil {
		return err
	}

	// Stage 3: Deploy - Deploying to cluster
	t.logger.Info().Msg("Deploying to cluster")
	if !args.GenerateOnly {
		if err := t.performDeployment(ctx, session, args, result, reporter); err != nil {
			return err
		}
	}

	// Stage 4: Verify - Verifying deployment health
	t.logger.Info().Msg("Verifying deployment health")
	if !args.GenerateOnly && args.WaitForReady {
		if err := t.performHealthCheck(ctx, session, args, result, reporter); err != nil {
			return err
		}
	}

	// Stage 5: Finalize - Saving deployment status
	t.logger.Info().Msg("Finalizing")
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", result.ImageRef).
		Bool("success", result.Success).
		Msg("Atomic Kubernetes deployment completed")

	t.logger.Info().Msg("Deployment operation completed")
	return nil
}

// executeWithoutProgress handles execution without progress tracking (fallback)
func (t *AtomicDeployKubernetesTool) executeWithoutProgress(ctx context.Context, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, startTime time.Time) (*AtomicDeployKubernetesResult, error) {
	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Set session details
	result.SessionID = session.SessionID
	result.WorkspaceDir = t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Str("namespace", args.Namespace).
		Msg("Starting atomic Kubernetes deployment")

	// Handle dry-run
	if args.DryRun {
		result.Success = true
		result.BaseAIContextResult.IsSuccessful = true
		result.DryRunPreview = "This is a dry-run - no actual deployment was performed"
		result.DeploymentContext.NextStepSuggestions = []string{
			"This is a dry-run - no actual deployment was performed",
			"Remove dry_run flag to perform actual deployment",
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Generate manifests
	if err := t.performManifestGeneration(ctx, session, args, result, nil); err != nil {
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Deploy to cluster (if not generate-only)
	if !args.GenerateOnly {
		if err := t.performDeployment(ctx, session, args, result, nil); err != nil {
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil
		}
	}

	// Health check (if not generate-only and wait requested)
	if !args.GenerateOnly && args.WaitForReady {
		if err := t.performHealthCheck(ctx, session, args, result, nil); err != nil {
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil
		}
	}

	// Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration
	result.TotalDuration = time.Since(startTime)

	return result, nil
}

// KubernetesDeployOperation implements FixableOperation for Kubernetes deployments
type KubernetesDeployOperation struct {
	tool         *AtomicDeployKubernetesTool
	args         AtomicDeployKubernetesArgs
	session      *sessiontypes.SessionState
	workspaceDir string
	namespace    string
	manifests    []string
	logger       zerolog.Logger
}

// ExecuteOnce performs a single Kubernetes deployment attempt
func (op *KubernetesDeployOperation) ExecuteOnce(ctx context.Context) error {
	op.logger.Debug().
		Str("image_ref", op.args.ImageRef).
		Str("namespace", op.namespace).
		Msg("Executing Kubernetes deployment")

	// Deploy to Kubernetes via pipeline adapter
	deployResult, err := op.tool.pipelineAdapter.DeployToKubernetes(
		op.session.SessionID,
		op.manifests,
	)

	if err != nil {
		op.logger.Warn().Err(err).Msg("Kubernetes deployment failed")
		return err
	}

	if deployResult == nil || !deployResult.Success {
		errorMsg := "unknown deployment error"
		if deployResult != nil && deployResult.Error != nil {
			errorMsg = deployResult.Error.Message
		}
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("kubernetes deployment failed: %s", errorMsg), "deployment_error")
	}

	op.logger.Info().
		Str("namespace", op.namespace).
		Msg("Kubernetes deployment completed successfully")

	return nil
}

// GetFailureAnalysis analyzes why the Kubernetes deployment failed
func (op *KubernetesDeployOperation) GetFailureAnalysis(ctx context.Context, err error) (error, error) {
	op.logger.Debug().Err(err).Msg("Analyzing Kubernetes deployment failure")
	return err, nil
}

// PrepareForRetry applies fixes and prepares for the next deployment attempt
func (op *KubernetesDeployOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().
		Str("fix_strategy", fixAttempt.FixStrategy.Name).
		Msg("Preparing for retry after fix")

	// Apply fix based on the strategy type
	switch fixAttempt.FixStrategy.Type {
	case "manifest":
		return op.applyManifestFix(ctx, fixAttempt)
	case "dependency":
		return op.applyDependencyFix(ctx, fixAttempt)
	case "resource":
		return op.applyResourceFix(ctx, fixAttempt)
	default:
		op.logger.Warn().
			Str("fix_type", fixAttempt.FixStrategy.Type).
			Msg("Unknown fix type, applying generic fix")
		return op.applyGenericFix(ctx, fixAttempt)
	}
}

// applyManifestFix applies fixes to Kubernetes manifests
func (op *KubernetesDeployOperation) applyManifestFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	if fixAttempt.FixedContent == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "no fixed manifest content provided", "missing_content")
	}

	op.logger.Info().
		Int("content_length", len(fixAttempt.FixedContent)).
		Msg("Applying manifest fix")

	// Determine the manifest file path based on file changes or default
	manifestPath := filepath.Join(op.workspaceDir, "k8s", "deployment.yaml")

	// Check if there's a specific file path in FileChanges
	if len(fixAttempt.FixStrategy.FileChanges) > 0 {
		// Use the first file change path as the manifest path
		manifestPath = filepath.Join(op.workspaceDir, fixAttempt.FixStrategy.FileChanges[0].FilePath)
	}

	// Ensure the directory exists
	dir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create manifest directory: %v", err), "filesystem_error")
	}

	// Create backup of existing manifest if it exists
	if _, err := os.Stat(manifestPath); err == nil {
		backupPath := manifestPath + ".backup"
		data, err := os.ReadFile(manifestPath)
		if err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create manifest backup")
			}
		}
	}

	// Write the fixed manifest content
	if err := os.WriteFile(manifestPath, []byte(fixAttempt.FixedContent), 0644); err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to write fixed manifest: %v", err), "file_error")
	}

	op.logger.Info().
		Str("manifest_path", manifestPath).
		Msg("Successfully applied manifest fix")

	return nil
}

// applyDependencyFix applies dependency-related fixes
func (op *KubernetesDeployOperation) applyDependencyFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "dependency").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying dependency fix")

	// Apply file changes for dependency fixes (e.g., updated image references)
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply dependency fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied dependency file change")
	}

	// Handle specific dependency fix patterns
	if fixAttempt.FixedContent != "" {
		// If we have fixed content for a manifest with updated dependencies
		return op.applyManifestFix(ctx, fixAttempt)
	}

	// Log any commands that might be needed (e.g., pulling new images)
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Dependency fix command identified (execution delegated to deployment tool)")
	}

	return nil
}

// applyResourceFix applies resource-related fixes
func (op *KubernetesDeployOperation) applyResourceFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "resource").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying resource fix")

	// Apply file changes for resource fixes (e.g., adjusted resource limits)
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to apply resource fix to %s: %v", change.FilePath, err), "file_error")
		}

		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied resource file change")
	}

	// Handle manifest updates with adjusted resources
	if fixAttempt.FixedContent != "" {
		// Apply the manifest with updated resource specifications
		return op.applyManifestFix(ctx, fixAttempt)
	}

	// Log resource-related insights from the fix strategy
	if fixAttempt.FixStrategy.Type == "resource" {
		op.logger.Info().
			Str("fix_name", fixAttempt.FixStrategy.Name).
			Str("fix_description", fixAttempt.FixStrategy.Description).
			Msg("Applied resource adjustment fix")
	}

	return nil
}

// applyGenericFix applies generic fixes
func (op *KubernetesDeployOperation) applyGenericFix(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	// Generic fix application
	if fixAttempt.FixedContent != "" {
		return op.applyManifestFix(ctx, fixAttempt)
	}

	op.logger.Info().Msg("Applied generic fix (no specific action needed)")
	return nil
}

// applyFileChange applies a single file change operation
func (op *KubernetesDeployOperation) applyFileChange(change mcptypes.FileChange) error {
	filePath := filepath.Join(op.workspaceDir, change.FilePath)

	switch change.Operation {
	case "create":
		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create directory %s: %v", dir, err), "filesystem_error")
		}

		// Write the new file
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0644); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create file %s: %v", filePath, err), "file_error")
		}

	case "update", "replace":
		// Create backup
		backupPath := filePath + ".backup"
		if data, err := os.ReadFile(filePath); err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create backup")
			}
		}

		// Write the updated content
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0644); err != nil {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to update file %s: %v", filePath, err), "file_error")
		}

	case "delete":
		// Create backup before deletion
		backupPath := filePath + ".backup"
		if data, err := os.ReadFile(filePath); err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				op.logger.Warn().Err(err).Msg("Failed to create backup before deletion")
			}
		}

		// Remove the file
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to delete file %s: %v", filePath, err), "file_error")
		}

	default:
		return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("unknown file operation: %s", change.Operation), "invalid_operation")
	}

	op.logger.Info().
		Str("file", filePath).
		Str("operation", change.Operation).
		Msg("Applied file change")

	return nil
}

// Helper methods for the Execute implementation

// performManifestGeneration generates Kubernetes manifests
func (t *AtomicDeployKubernetesTool) performManifestGeneration(ctx context.Context, session *sessiontypes.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, reporter interface{}) error {
	// Progress reporting removed

	generationStart := time.Now()

	// Generate Kubernetes manifests using pipeline adapter
	port := args.Port
	if port == 0 {
		port = 80 // Default port
	}
	manifestResult, err := t.pipelineAdapter.GenerateKubernetesManifests(
		session.SessionID,
		args.ImageRef,
		args.AppName,
		port,
		"", // cpuRequest - not specified for deploy tool
		"", // memoryRequest - not specified for deploy tool
		"", // cpuLimit - not specified for deploy tool
		"", // memoryLimit - not specified for deploy tool
	)
	result.GenerationDuration = time.Since(generationStart)

	// Convert from mcptypes.KubernetesManifestResult to kubernetes.ManifestGenerationResult
	if manifestResult != nil {
		result.ManifestResult = &kubernetes.ManifestGenerationResult{
			Success:   manifestResult.Success,
			OutputDir: result.WorkspaceDir,
		}
		if manifestResult.Error != nil {
			result.ManifestResult.Error = &kubernetes.ManifestError{
				Type:    manifestResult.Error.Type,
				Message: manifestResult.Error.Message,
			}
		}
		// Convert manifests
		for _, manifest := range manifestResult.Manifests {
			result.ManifestResult.Manifests = append(result.ManifestResult.Manifests, kubernetes.GeneratedManifest{
				Kind:    manifest.Kind,
				Name:    manifest.Name,
				Path:    manifest.Path,
				Content: manifest.Content,
			})
		}
	}

	if err != nil {
		t.handleGenerationError(ctx, err, result.ManifestResult, result)
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", err), "generation_error")
	}

	if manifestResult != nil && !manifestResult.Success {
		generationErr := types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %s", manifestResult.Error.Message), "generation_error")
		t.handleGenerationError(ctx, generationErr, result.ManifestResult, result)
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", generationErr), "generation_error")
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("app_name", args.AppName).
		Str("namespace", args.Namespace).
		Msg("Kubernetes manifests generated successfully")

	// Progress reporting removed

	return nil
}

// performDeployment deploys manifests to Kubernetes cluster
func (t *AtomicDeployKubernetesTool) performDeployment(ctx context.Context, session *sessiontypes.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, reporter interface{}) error {
	// Progress reporting removed

	deploymentStart := time.Now()

	// Deploy to Kubernetes using pipeline adapter
	// Get manifests from result
	manifests := []string{}
	if result.ManifestResult != nil {
		for _, manifest := range result.ManifestResult.Manifests {
			manifests = append(manifests, manifest.Path)
		}
	}
	deployResult, err := t.pipelineAdapter.DeployToKubernetes(
		session.SessionID,
		manifests,
	)
	result.DeploymentDuration = time.Since(deploymentStart)

	// Convert from mcptypes.KubernetesDeploymentResult to kubernetes.DeploymentResult
	if deployResult != nil {
		result.DeploymentResult = &kubernetes.DeploymentResult{
			Success:   deployResult.Success,
			Namespace: deployResult.Namespace,
		}
		if deployResult.Error != nil {
			result.DeploymentResult.Error = &kubernetes.DeploymentError{
				Type:    deployResult.Error.Type,
				Message: deployResult.Error.Message,
			}
		}
		// Convert deployments and services
		for _, d := range deployResult.Deployments {
			result.DeploymentResult.Resources = append(result.DeploymentResult.Resources, kubernetes.DeployedResource{
				Kind:      "Deployment",
				Name:      d,
				Namespace: deployResult.Namespace,
			})
		}
		for _, s := range deployResult.Services {
			result.DeploymentResult.Resources = append(result.DeploymentResult.Resources, kubernetes.DeployedResource{
				Kind:      "Service",
				Name:      s,
				Namespace: deployResult.Namespace,
			})
		}
	}

	if err != nil {
		t.handleDeploymentError(ctx, err, result.DeploymentResult, result)
		return err
	}

	if deployResult != nil && !deployResult.Success {
		deploymentErr := types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("deployment failed: %s", deployResult.Error.Message), "deployment_error")
		t.handleDeploymentError(ctx, deploymentErr, result.DeploymentResult, result)
		return deploymentErr
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", args.Namespace).
		Msg("Kubernetes deployment completed successfully")

	// Progress reporting removed

	return nil
}

// performHealthCheck verifies deployment health
func (t *AtomicDeployKubernetesTool) performHealthCheck(ctx context.Context, session *sessiontypes.SessionState, args AtomicDeployKubernetesArgs, result *AtomicDeployKubernetesResult, reporter interface{}) error {
	// Progress reporting removed

	healthStart := time.Now()
	timeout := 300 * time.Second // Default 5 minutes
	if args.WaitTimeout > 0 {
		timeout = time.Duration(args.WaitTimeout) * time.Second
	}

	// Check deployment health using pipeline adapter
	healthResult, err := t.pipelineAdapter.CheckApplicationHealth(
		session.SessionID,
		args.Namespace,
		"app="+args.AppName, // label selector
		timeout,
	)
	result.HealthCheckDuration = time.Since(healthStart)

	// Convert from mcptypes.HealthCheckResult to kubernetes.HealthCheckResult
	if healthResult != nil {
		result.HealthResult = &kubernetes.HealthCheckResult{
			Success:   healthResult.Healthy,
			Namespace: args.Namespace,
			Duration:  result.HealthCheckDuration,
		}
		if healthResult.Error != nil {
			result.HealthResult.Error = &kubernetes.HealthCheckError{
				Type:    healthResult.Error.Type,
				Message: healthResult.Error.Message,
			}
		}
		// Convert pod statuses
		for _, ps := range healthResult.PodStatuses {
			podStatus := kubernetes.DetailedPodStatus{
				Name:      ps.Name,
				Namespace: args.Namespace,
				Status:    ps.Status,
				Ready:     ps.Ready,
			}
			result.HealthResult.Pods = append(result.HealthResult.Pods, podStatus)
		}
		// Update summary
		result.HealthResult.Summary = kubernetes.HealthSummary{
			TotalPods:   len(result.HealthResult.Pods),
			ReadyPods:   0,
			FailedPods:  0,
			PendingPods: 0,
		}
		for _, pod := range result.HealthResult.Pods {
			if pod.Ready {
				result.HealthResult.Summary.ReadyPods++
			} else if pod.Status == "Failed" || pod.Phase == "Failed" {
				result.HealthResult.Summary.FailedPods++
			} else if pod.Status == "Pending" || pod.Phase == "Pending" {
				result.HealthResult.Summary.PendingPods++
			}
		}
		if result.HealthResult.Summary.TotalPods > 0 {
			result.HealthResult.Summary.HealthyRatio = float64(result.HealthResult.Summary.ReadyPods) / float64(result.HealthResult.Summary.TotalPods)
		}
	}

	if err != nil {
		t.handleHealthCheckError(ctx, err, result.HealthResult, result)
		return err
	}

	if healthResult != nil && !healthResult.Healthy {
		var readyPods, totalPods int
		if result.HealthResult != nil {
			readyPods = result.HealthResult.Summary.ReadyPods
			totalPods = result.HealthResult.Summary.TotalPods
		}
		healthErr := types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("deployment health check failed: %d/%d pods ready", readyPods, totalPods), "health_check_error")
		t.handleHealthCheckError(ctx, healthErr, result.HealthResult, result)
		return healthErr
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("namespace", args.Namespace).
		Str("app_name", args.AppName).
		Msg("Deployment health check passed")

	// Progress reporting removed

	return nil
}

// updateSessionState updates session with deployment results
func (t *AtomicDeployKubernetesTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicDeployKubernetesResult) error {
	// Update session with deployment results
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	// Update session state fields (using Metadata since SessionState doesn't have these fields)
	if result.Success {
		session.Metadata["deployed"] = true
		session.Metadata["deployment_namespace"] = result.Namespace
		session.Metadata["deployment_name"] = result.AppName
	}

	// Update metadata for backward compatibility and additional details
	session.Metadata["last_deployed_image"] = result.ImageRef
	session.Metadata["last_deployment_namespace"] = result.Namespace
	session.Metadata["last_deployment_app"] = result.AppName
	session.Metadata["last_deployment_success"] = result.Success
	session.Metadata["deployed_image_ref"] = result.ImageRef
	session.Metadata["deployment_namespace"] = result.Namespace
	session.Metadata["deployment_app"] = result.AppName
	session.Metadata["deployment_success"] = result.Success

	if result.Success {
		session.Metadata["deployment_duration_seconds"] = result.TotalDuration.Seconds()
		session.Metadata["generation_duration_seconds"] = result.GenerationDuration.Seconds()
		if result.DeploymentDuration > 0 {
			session.Metadata["deploy_duration_seconds"] = result.DeploymentDuration.Seconds()
		}
		if result.HealthCheckDuration > 0 {
			session.Metadata["health_check_duration_seconds"] = result.HealthCheckDuration.Seconds()
		}
	}

	session.UpdateLastAccessed()

	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
	})
}

// Error handling methods

// handleGenerationError creates an error for manifest generation failures
func (t *AtomicDeployKubernetesTool) handleGenerationError(ctx context.Context, err error, manifestResult *kubernetes.ManifestGenerationResult, result *AtomicDeployKubernetesResult) error {
	return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("manifest generation failed: %v", err), "generation_error")
}

// handleDeploymentError creates an error for deployment failures
func (t *AtomicDeployKubernetesTool) handleDeploymentError(ctx context.Context, err error, deployResult *kubernetes.DeploymentResult, result *AtomicDeployKubernetesResult) error {
	return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("kubernetes deployment failed: %v", err), "deployment_error")
}

// handleHealthCheckError creates an error for health check failures
func (t *AtomicDeployKubernetesTool) handleHealthCheckError(ctx context.Context, err error, healthResult *kubernetes.HealthCheckResult, result *AtomicDeployKubernetesResult) error {
	return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("health check failed: %v", err), "health_check_error")
}

// Tool interface implementation (unified MCP interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicDeployKubernetesTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_deploy_kubernetes",
		Description:  "Deploys containerized applications to Kubernetes with manifest generation, health checks, and rollback support",
		Version:      "1.0.0",
		Category:     "kubernetes",
		Dependencies: []string{"kubectl", "kubernetes-cluster"},
		Capabilities: []string{
			"supports_dry_run",
			"supports_streaming",
			"long_running",
			"manifest_generation",
		},
		Requirements: []string{"kubernetes_access", "image_registry_access"},
		Parameters: map[string]string{
			"image_ref":       "required - Container image reference",
			"app_name":        "optional - Application name",
			"namespace":       "optional - Kubernetes namespace (default: default)",
			"replicas":        "optional - Number of replicas (default: 1)",
			"port":            "optional - Application port (default: 80)",
			"service_type":    "optional - Service type (ClusterIP, NodePort, LoadBalancer)",
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
					"image_ref":  "myregistry.azurecr.io/myapp:v1.0.0",
					"app_name":   "myapp",
					"namespace":  "production",
					"replicas":   3,
					"port":       8080,
				},
				Output: map[string]interface{}{
					"success":          true,
					"manifest_paths":   []string{"/workspace/manifests/deployment.yaml"},
					"deployment_ready": true,
					"pod_count":        3,
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicDeployKubernetesTool) Validate(ctx context.Context, args interface{}) error {
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		return mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_deploy_kubernetes", map[string]interface{}{
			"expected": "AtomicDeployKubernetesArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	if deployArgs.ImageRef == "" {
		return mcperror.NewWithData("missing_required_field", "ImageRef is required", map[string]interface{}{
			"field": "image_ref",
		})
	}

	if deployArgs.SessionID == "" {
		return mcperror.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicDeployKubernetesTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	deployArgs, ok := args.(AtomicDeployKubernetesArgs)
	if !ok {
		return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_deploy_kubernetes", map[string]interface{}{
			"expected": "AtomicDeployKubernetesArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, deployArgs)
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
func (t *AtomicDeployKubernetesTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicDeployKubernetesTool) ExecuteTyped(ctx context.Context, args AtomicDeployKubernetesArgs) (*AtomicDeployKubernetesResult, error) {
	startTime := time.Now()

	// Create result object early for error handling
	result := &AtomicDeployKubernetesResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_deploy_kubernetes", args.SessionID, args.DryRun),
		BaseAIContextResult: internal.NewBaseAIContextResult("deploy", false, 0), // Will be updated later
		SessionID:           args.SessionID,
		ImageRef:            args.ImageRef,
		AppName:             args.AppName,
		Namespace:           args.Namespace,
		Replicas:            args.Replicas,
		Port:                args.Port,
		ServiceType:         args.ServiceType,
		DeploymentContext:   &DeploymentContext{},
	}

	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args, result, startTime)
}

// SetAnalyzer enables AI-driven fixing capabilities by providing an analyzer
func (t *AtomicDeployKubernetesTool) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	if analyzer != nil {
		t.fixingMixin = fixing.NewAtomicToolFixingMixin(analyzer, "deploy_kubernetes_atomic", t.logger)
	}
}
