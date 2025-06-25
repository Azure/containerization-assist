package deploy

import (
	"context"
	"fmt"
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	corek8s "github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/fixing"
	"github.com/Azure/container-copilot/pkg/mcp/internal/mcperror"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicGenerateManifestsTool implements atomic Kubernetes manifest generation with secret handling
type AtomicGenerateManifestsTool struct {
	pipelineAdapter     mcptypes.PipelineOperations
	sessionManager      mcptypes.ToolSessionManager
	secretScanner       *utils.SecretScanner
	secretGenerator     *corek8s.SecretGenerator
	fixingMixin         *fixing.AtomicToolFixingMixin
	templateIntegration *TemplateIntegration
	// manifestsAdapter removed - functionality integrated directly
	logger zerolog.Logger
}

// NewAtomicGenerateManifestsTool creates a new atomic generate manifests tool
func NewAtomicGenerateManifestsTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicGenerateManifestsTool {
	toolLogger := logger.With().Str("tool", "atomic_generate_manifests").Logger()
	return &AtomicGenerateManifestsTool{
		pipelineAdapter:     adapter,
		sessionManager:      sessionManager,
		secretScanner:       utils.NewSecretScanner(),
		secretGenerator:     corek8s.NewSecretGenerator(toolLogger),
		fixingMixin:         nil, // Will be set via SetAnalyzer if fixing is enabled
		templateIntegration: NewTemplateIntegration(toolLogger),
		// manifestsAdapter removed - functionality integrated directly
		logger: toolLogger,
	}
}

// SetAnalyzer enables AI-driven fixing capabilities by providing an analyzer
func (t *AtomicGenerateManifestsTool) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	if analyzer != nil {
		t.fixingMixin = fixing.NewAtomicToolFixingMixin(analyzer, "generate_manifests_atomic", t.logger)
	}
}

// ExecuteWithFixes runs the atomic manifest generation with AI-driven fixing capabilities
func (t *AtomicGenerateManifestsTool) ExecuteWithFixes(ctx context.Context, args AtomicGenerateManifestsArgs) (*AtomicGenerateManifestsResult, error) {
	// Check if fixing is enabled
	if t.fixingMixin == nil {
		t.logger.Warn().Msg("AI-driven fixing not enabled, falling back to regular execution")
		return t.ExecuteManifestGeneration(ctx, args)
	}

	// First validate basic requirements
	if args.SessionID == "" {
		return nil, types.NewValidationErrorBuilder("Session ID is required", "session_id", args.SessionID).
			WithOperation("generate_manifests").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide session ID", "Specify a valid session ID for the manifest generation operation").
			Build()
	}
	if args.AppName == "" {
		return nil, types.NewValidationErrorBuilder("Application name is required", "app_name", args.AppName).
			WithOperation("generate_manifests").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide app name", "Specify a valid application name for the Kubernetes manifests").
			Build()
	}
	if args.ImageRef == "" {
		return nil, types.NewValidationErrorBuilder("Image reference is required", "image_ref", args.ImageRef).
			WithOperation("generate_manifests").
			WithStage("input_validation").
			WithImmediateStep(1, "Provide image reference", "Specify a valid Docker image reference for the deployment").
			Build()
	}

	// Get session and workspace info
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("session not found: %s", args.SessionID), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)
	workspaceDir := t.pipelineAdapter.GetSessionWorkspace(session.SessionID)

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("app_name", args.AppName).
		Str("image_ref", args.ImageRef).
		Msg("Starting manifest generation with AI-driven fixing")
	// Create fixable operation wrapper
	operation := &ManifestGenerationOperation{
		tool:         t,
		args:         args,
		session:      session,
		workspaceDir: workspaceDir,
		logger:       t.logger,
	}
	// Execute with retry and fixing
	err = t.fixingMixin.ExecuteWithRetry(ctx, args.SessionID, workspaceDir, operation)
	if err != nil {
		return &AtomicGenerateManifestsResult{
			BaseToolResponse:    types.NewBaseResponse("generate_manifests_atomic", args.SessionID, args.DryRun),
			BaseAIContextResult: internal.NewBaseAIContextResult("generate_manifests", false, time.Since(time.Now())),
			Success:             false,
			SessionID:           args.SessionID,
			AppName:             args.AppName,
			ImageRef:            args.ImageRef,
			// Error details are logged, not stored in result
		}, err
	}
	// If we get here, the generation succeeded - call the regular ExecuteManifestGeneration to get full results
	return t.ExecuteManifestGeneration(ctx, args)
}

// ExecuteManifestGeneration runs the atomic manifest generation with secret handling (legacy method)
func (t *AtomicGenerateManifestsTool) ExecuteManifestGeneration(ctx context.Context, args AtomicGenerateManifestsArgs) (*AtomicGenerateManifestsResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic manifest generation with GoMCP progress tracking
func (t *AtomicGenerateManifestsTool) ExecuteWithContext(serverCtx *server.Context, args AtomicGenerateManifestsArgs) (*AtomicGenerateManifestsResult, error) {
	// Create progress adapter for GoMCP using centralized generate stages
	_ = internal.NewGoMCPProgressAdapter(serverCtx, []mcptypes.ProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Generate", Weight: 0.80, Description: "Generating"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performManifestGeneration(ctx, args, nil)

	// Complete progress tracking
	if err != nil {
		// Progress tracking removed
		t.logger.Info().Msg("Manifest generation failed")
		if result != nil {
			result.Success = false
		}
		return result, nil // Return result with error info, not the error itself
	} else {
		// Progress tracking removed
		t.logger.Info().Msg("Manifest generation completed successfully")
	}

	return result, nil
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicGenerateManifestsTool) executeWithoutProgress(ctx context.Context, args AtomicGenerateManifestsArgs) (*AtomicGenerateManifestsResult, error) {
	return t.performManifestGeneration(ctx, args, nil)
}

// performManifestGeneration performs the actual manifest generation
func (t *AtomicGenerateManifestsTool) performManifestGeneration(ctx context.Context, args AtomicGenerateManifestsArgs, reporter mcptypes.ProgressReporter) (*AtomicGenerateManifestsResult, error) {
	startTime := time.Now()

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicGenerateManifestsResult{
			BaseToolResponse:          types.NewBaseResponse("atomic_generate_manifests", args.SessionID, args.DryRun),
			BaseAIContextResult:       internal.NewBaseAIContextResult("generate_manifests", false, time.Since(startTime)),
			SessionID:                 args.SessionID,
			TotalDuration:             time.Since(startTime),
			ManifestContext:           &ManifestContext{},
			DeploymentStrategyContext: &DeploymentStrategyContext{},
		}
		result.Success = false

		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, types.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("session not found: %s", args.SessionID), types.ErrTypeSession)
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("image_ref", args.ImageRef).
		Bool("gitops_ready", args.GitOpsReady).
		Msg("Starting atomic manifest generation")

	// Use refactored modules if enabled
	useRefactoredModules := os.Getenv("USE_REFACTORED_MANIFESTS") == "true"
	if useRefactoredModules {
		t.logger.Info().Msg("Using refactored manifest generation modules")
		_ = t.pipelineAdapter.GetSessionWorkspace(session.SessionID) // workspaceDir would be used here
		// ManifestsAdapter removed - return error for now
		return nil, types.NewRichError("FEATURE_NOT_IMPLEMENTED", "refactored manifest generation not implemented without adapter", types.ErrTypeSystem)
	}

	// Create base response
	result := &AtomicGenerateManifestsResult{
		BaseToolResponse:          types.NewBaseResponse("atomic_generate_manifests", session.SessionID, args.DryRun),
		BaseAIContextResult:       internal.NewBaseAIContextResult("generate_manifests", false, 0), // Will update success and duration later
		SessionID:                 session.SessionID,
		WorkspaceDir:              t.pipelineAdapter.GetSessionWorkspace(session.SessionID),
		ImageRef:                  args.ImageRef,
		AppName:                   t.getAppName(args.AppName, args.ImageRef),
		Namespace:                 t.getNamespace(args.Namespace),
		ManifestContext:           &ManifestContext{},
		DeploymentStrategyContext: &DeploymentStrategyContext{},
	}

	// Step 1: Scan for secrets in environment variables
	if len(args.Environment) > 0 {
		t.logger.Info().
			Int("env_vars", len(args.Environment)).
			Msg("Scanning environment variables for secrets")

		detectedSecrets := t.secretScanner.ScanEnvironment(args.Environment)
		result.SecretsDetected = t.convertDetectedSecrets(detectedSecrets)
		result.ManifestContext.SecretsDetected = len(detectedSecrets)

		if len(detectedSecrets) > 0 {
			t.logger.Warn().
				Int("secrets_found", len(detectedSecrets)).
				Msg("Sensitive environment variables detected")

			// Progress reporting removed
			t.logger.Info().Msg("Creating secrets externalization plan")

			// Create externalization plan based on mode
			plan := t.createSecretsPlan(args, detectedSecrets)
			result.SecretsPlan = plan

			// Update args to use externalized secrets
			if args.SecretHandling == types.ResourceModeAuto || args.SecretHandling == "prompt" {
				args = t.applySecretsPlan(args, plan)
				result.ManifestContext.SecretsExternalized = len(plan.SecretReferences)
			}
		}
	}

	// Progress reporting removed
	t.logger.Info().Msg("Requirements analysis complete")

	// Step 3: Select template for manifest generation
	t.logger.Info().Msg("Selecting template for manifest generation")

	// Extract repository info from session for template selection
	repoInfo := make(map[string]interface{})
	if session.ScanSummary != nil {
		repoInfo = sessiontypes.ConvertScanSummaryToRepositoryInfo(session.ScanSummary)
	}

	// Use template integration to select the best template
	templateContext, err := t.templateIntegration.SelectManifestTemplate(args, repoInfo)
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to select template, using default")
		// Continue with default template selection
	}

	// Log template selection
	if templateContext != nil {
		if contextMap, ok := templateContext.(map[string]interface{}); ok {
			if template, exists := contextMap["template"]; exists {
				t.logger.Info().
					Interface("template", template).
					Msg("Selected manifest template")
			}
		}

		// Store template context in result for rich AI context
		if result.DeploymentStrategyContext == nil {
			result.DeploymentStrategyContext = &DeploymentStrategyContext{}
		}
		// For now, skip template context assignment as it's a stub implementation
		// result.DeploymentStrategyContext.TemplateContext = templateContext
	}

	// Step 4: Generate Kubernetes manifests using core operations
	generationStartTime := time.Now()

	// Prepare parameters for core manifest generation
	manifestResult, err := t.pipelineAdapter.GenerateKubernetesManifests(
		session.SessionID,
		result.ImageRef,
		result.AppName,
		t.getPort(args.Port),
		args.CPURequest,
		args.MemoryRequest,
		args.CPULimit,
		args.MemoryLimit,
	)

	result.GenerationDuration = time.Since(generationStartTime)

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
		t.logger.Error().Err(err).
			Str("session_id", session.SessionID).
			Str("image_ref", args.ImageRef).
			Msg("Manifest generation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("MANIFEST_GENERATION_FAILED", "Manifest generation failed", types.ErrTypeBuild)
	}

	if !manifestResult.Success {
		t.logger.Error().
			Str("session_id", session.SessionID).
			Str("image_ref", args.ImageRef).
			Str("error_message", manifestResult.Error.Message).
			Msg("Manifest generation failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, types.NewRichError("MANIFEST_GENERATION_FAILED", fmt.Sprintf("Manifest generation failed: %s", manifestResult.Error.Message), types.ErrTypeBuild)
	}

	// Step 5: Generate additional secret manifests if needed
	if result.SecretsPlan != nil && len(result.SecretsPlan.SecretReferences) > 0 {
		secretManifests := t.generateSecretManifests(session.SessionID, result)
		result.SecretManifests = secretManifests
	}

	// Step 6: Enhance manifests based on configuration
	if err := t.enhanceManifests(session.SessionID, result.ManifestResult, args, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to enhance manifests with additional configuration")
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)

	// Update internal.BaseAIContextResult with final values
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration

	// Step 7: Generate rich context
	t.generateManifestContext(result, args)

	// Step 8: Generate deployment strategy context for AI decision-making
	t.generateDeploymentStrategyContext(result, args, session)

	// Step 9: Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Int("manifests", result.ManifestContext.ManifestsGenerated).
		Int("secrets_externalized", result.ManifestContext.SecretsExternalized).
		Dur("duration", result.TotalDuration).
		Msg("Atomic manifest generation completed")

	// Progress reporting removed
	t.logger.Info().Msg("Manifest generation complete")

	return result, nil
}

// Helper methods
func (t *AtomicGenerateManifestsTool) enhanceManifests(sessionID string, manifestResult *corek8s.ManifestGenerationResult, args AtomicGenerateManifestsArgs, result *AtomicGenerateManifestsResult) error {
	t.logger.Info().
		Bool("has_resource_limits", args.CPURequest != "" || args.MemoryRequest != "").
		Bool("has_ingress", args.IncludeIngress).
		Int("secret_refs", len(args.Environment)).
		Msg("Enhancing manifests with additional configuration")

	// Generate ConfigMap for non-sensitive environment variables
	if err := t.generateConfigMapManifest(manifestResult, args); err != nil {
		t.logger.Error().Err(err).Msg("Failed to generate ConfigMap manifest")
		return types.NewRichError("CONFIGMAP_GENERATION_FAILED", fmt.Sprintf("failed to generate ConfigMap: %v", err), types.ErrTypeBuild)
	}

	// Generate Ingress if requested
	if args.IncludeIngress {
		if err := t.generateIngressManifest(manifestResult, args); err != nil {
			t.logger.Error().Err(err).Msg("Failed to generate Ingress manifest")
			return types.NewRichError("INGRESS_GENERATION_FAILED", fmt.Sprintf("failed to generate Ingress: %v", err), types.ErrTypeBuild)
		}
	}

	// Update context to reflect resource limits have been applied if specified
	if args.CPURequest != "" || args.MemoryRequest != "" || args.CPULimit != "" || args.MemoryLimit != "" {
		result.ManifestContext.ResourceLimitsSet = true
	}

	return nil
}

func (t *AtomicGenerateManifestsTool) generateManifestContext(result *AtomicGenerateManifestsResult, args AtomicGenerateManifestsArgs) {
	ctx := result.ManifestContext

	// Count resources
	if result.ManifestResult != nil {
		ctx.ManifestsGenerated = len(result.ManifestResult.Manifests)
		for _, m := range result.ManifestResult.Manifests {
			ctx.ResourceTypes = append(ctx.ResourceTypes, m.Kind)
		}
		ctx.TotalResources = len(result.ManifestResult.Manifests) + len(result.SecretManifests)
	}

	// Security assessment
	if ctx.SecretsDetected > 0 && ctx.SecretsExternalized == ctx.SecretsDetected {
		ctx.SecurityLevel = "high"
		ctx.BestPractices = append(ctx.BestPractices, "All secrets externalized")
	} else if ctx.SecretsDetected > 0 && ctx.SecretsExternalized > 0 {
		ctx.SecurityLevel = "medium"
		ctx.BestPractices = append(ctx.BestPractices, "Some secrets externalized")
		ctx.SecurityIssues = append(ctx.SecurityIssues,
			fmt.Sprintf("%d secrets remain inline", ctx.SecretsDetected-ctx.SecretsExternalized))
	} else if ctx.SecretsDetected > 0 {
		ctx.SecurityLevel = "low"
		ctx.SecurityIssues = append(ctx.SecurityIssues,
			fmt.Sprintf("%d secrets detected but not externalized", ctx.SecretsDetected))
	} else {
		ctx.SecurityLevel = "high"
		ctx.BestPractices = append(ctx.BestPractices, "No hardcoded secrets detected")
	}

	// Best practices
	if args.CPURequest != "" && args.MemoryRequest != "" {
		ctx.BestPractices = append(ctx.BestPractices, "Resource requests defined")
	}
	if args.CPULimit != "" && args.MemoryLimit != "" {
		ctx.BestPractices = append(ctx.BestPractices, "Resource limits defined")
	}
	if args.GitOpsReady {
		ctx.BestPractices = append(ctx.BestPractices, "GitOps-ready configuration")
	}

	// Configuration summary
	ctx.DeploymentConfig = map[string]interface{}{
		"namespace":       result.Namespace,
		"replicas":        args.Replicas,
		"service_type":    args.ServiceType,
		"has_ingress":     args.IncludeIngress,
		"secret_strategy": ctx.SecretStrategy,
	}

	// Next steps
	ctx.NextSteps = append(ctx.NextSteps, "Review generated manifests")

	if len(result.SecretManifests) > 0 {
		ctx.NextSteps = append(ctx.NextSteps,
			fmt.Sprintf("Create %d secret(s) using provided templates", len(result.SecretManifests)))
	}

	if result.SecretsPlan != nil && len(result.SecretsPlan.Instructions) > 0 {
		ctx.NextSteps = append(ctx.NextSteps, "Follow secret creation instructions")
	}

	ctx.NextSteps = append(ctx.NextSteps, "Deploy to Kubernetes cluster")

	// Deployment tips
	if ctx.SecretsDetected > 0 {
		ctx.DeploymentTips = append(ctx.DeploymentTips,
			"Ensure all secrets are created before deployment")
	}

	if args.GitOpsReady {
		ctx.DeploymentTips = append(ctx.DeploymentTips,
			"Manifests are safe to commit to Git (no inline secrets)")
	}

	ctx.DeploymentTips = append(ctx.DeploymentTips,
		fmt.Sprintf("Deploy with: kubectl apply -f %s", result.ManifestResult.OutputDir))
}

func (t *AtomicGenerateManifestsTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicGenerateManifestsResult) error {
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}

	session.Metadata["last_manifest_generation"] = time.Now()
	session.Metadata["manifests_generated"] = result.ManifestContext.ManifestsGenerated
	session.Metadata["secrets_detected"] = result.ManifestContext.SecretsDetected
	session.Metadata["secrets_externalized"] = result.ManifestContext.SecretsExternalized

	if result.ManifestResult != nil && result.ManifestResult.Success {
		session.Metadata["manifest_path"] = result.ManifestResult.OutputDir

		// Add to StageHistory for stage tracking
		now := time.Now()
		startTime := now.Add(-result.TotalDuration) // Calculate start time from duration
		execution := sessiontypes.ToolExecution{
			Tool:       "generate_manifests",
			StartTime:  startTime,
			EndTime:    &now,
			Duration:   &result.TotalDuration,
			Success:    true,
			DryRun:     false,
			TokensUsed: 0, // Could be tracked if needed
		}
		session.AddToolExecution(execution)
	}

	session.UpdateLastAccessed()

	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if state, ok := s.(*sessiontypes.SessionState); ok {
			*state = *session
		}
	})
}

func (t *AtomicGenerateManifestsTool) getAppName(appName, imageRef string) string {
	if appName != "" {
		return appName
	}

	// Extract from image reference
	parts := strings.Split(imageRef, "/")
	if len(parts) > 0 {
		imageName := parts[len(parts)-1]
		if tagIndex := strings.Index(imageName, ":"); tagIndex > 0 {
			imageName = imageName[:tagIndex]
		}
		return imageName
	}

	return "app"
}

func (t *AtomicGenerateManifestsTool) getNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func (t *AtomicGenerateManifestsTool) getPort(port int) int {
	if port <= 0 {
		return 8080
	}
	return port
}

// ManifestGenerationOperation implements fixing.FixableOperation for manifest generation
type ManifestGenerationOperation struct {
	tool         *AtomicGenerateManifestsTool
	args         AtomicGenerateManifestsArgs
	session      *sessiontypes.SessionState
	workspaceDir string
	logger       zerolog.Logger
}

// ExecuteOnce performs a single manifest generation attempt
func (op *ManifestGenerationOperation) ExecuteOnce(ctx context.Context) error {
	op.logger.Info().
		Str("session_id", op.args.SessionID).
		Str("app_name", op.args.AppName).
		Str("image_ref", op.args.ImageRef).
		Msg("Executing manifest generation attempt")

	// Execute the regular manifest generation
	resultInterface, err := op.tool.Execute(ctx, op.args)
	if err != nil {
		return types.NewRichError("MANIFEST_GENERATION_FAILED", fmt.Sprintf("manifest generation failed: %v", err), types.ErrTypeBuild)
	}

	// Type assert to get the actual result
	result, ok := resultInterface.(*AtomicGenerateManifestsResult)
	if !ok {
		return types.NewRichError("UNEXPECTED_RESULT_TYPE", fmt.Sprintf("unexpected result type: %T", resultInterface), types.ErrTypeSystem)
	}

	if !result.Success {
		return types.NewRichError("MANIFEST_GENERATION_FAILED", "manifest generation failed with unknown error", types.ErrTypeBuild)
	}

	return nil
}

// GetFailureAnalysis analyzes why the manifest generation failed
func (op *ManifestGenerationOperation) GetFailureAnalysis(ctx context.Context, err error) (*mcptypes.RichError, error) {
	op.logger.Error().
		Err(err).
		Str("session_id", op.args.SessionID).
		Msg("Analyzing manifest generation failure")

	// Create a rich error with context
	// Log the error instead of creating a RichError
	op.logger.Error().Err(err).Msg("Manifest generation failed")
	// Create a rich error for mcptypes
	richError := &mcptypes.RichError{
		Code:     "MANIFEST_GENERATION_FAILED",
		Type:     "deployment",
		Severity: "high",
		Message:  fmt.Sprintf("Manifest generation failed: %v", err),
	}

	// Analyze error patterns
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "invalid image"):
		richError.Type = "invalid_image"
		richError.Severity = "high"

	case strings.Contains(errStr, "resource limits"):
		richError.Type = "resource_configuration"
		richError.Severity = "medium"

	case strings.Contains(errStr, "secret") || strings.Contains(errStr, "sensitive"):
		richError.Type = "secret_handling"
		richError.Severity = "high"

	case strings.Contains(errStr, "template") || strings.Contains(errStr, "render"):
		richError.Type = "template_error"
		richError.Severity = "medium"

	default:
		richError.Type = "general_manifest_error"
		richError.Severity = "medium"
	}

	return richError, nil
}

// PrepareForRetry prepares the manifest generation for another attempt
func (op *ManifestGenerationOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcptypes.FixAttempt) error {
	op.logger.Info().
		Str("session_id", op.args.SessionID).
		Int("attempt", fixAttempt.AttemptNumber).
		Msg("Preparing manifest generation for retry")

	// Apply any file changes from the fix attempt
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		switch change.Operation {
		case "update":
			// Update manifest templates or configuration files
			if err := os.WriteFile(change.FilePath, []byte(change.NewContent), 0644); err != nil {
				return types.NewRichError("FILE_UPDATE_FAILED", fmt.Sprintf("failed to update file %s: %v", change.FilePath, err), types.ErrTypeSystem)
			}
			op.logger.Info().
				Str("file", change.FilePath).
				Str("reason", change.Reason).
				Msg("Updated file for retry")

		case "create":
			// Create new configuration files if needed
			dir := filepath.Dir(change.FilePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return types.NewRichError("DIRECTORY_CREATION_FAILED", fmt.Sprintf("failed to create directory %s: %v", dir, err), types.ErrTypeSystem)
			}
			if err := os.WriteFile(change.FilePath, []byte(change.NewContent), 0644); err != nil {
				return types.NewRichError("FILE_CREATION_FAILED", fmt.Sprintf("failed to create file %s: %v", change.FilePath, err), types.ErrTypeSystem)
			}
			op.logger.Info().
				Str("file", change.FilePath).
				Str("reason", change.Reason).
				Msg("Created file for retry")
		}
	}

	// Apply any argument changes suggested by the fix
	if fixAttempt.FixStrategy.Name == "adjust_resources" {
		// AI might suggest adjusting resource limits through file changes
		for _, change := range fixAttempt.FixStrategy.FileChanges {
			if strings.Contains(change.Reason, "cpu_request") || strings.Contains(change.Reason, "memory_request") {
				op.logger.Info().
					Str("reason", change.Reason).
					Msg("Adjusting resources based on fix strategy")
			}
		}
	}

	return nil
}

// CanRetry determines if the operation can be retried
func (op *ManifestGenerationOperation) CanRetry() bool {
	return true // Manifest generation can always be retried
}

// GetLastError returns the last error encountered
func (op *ManifestGenerationOperation) GetLastError() error {
	// For now, return nil as we don't track errors in the operation
	return nil
}

// Execute runs the operation
func (op *ManifestGenerationOperation) Execute(ctx context.Context) error {
	return op.ExecuteOnce(ctx)
}

// SimpleTool interface implementation
// GetName returns the tool name
func (t *AtomicGenerateManifestsTool) GetName() string {
	return "atomic_generate_manifests"
}

// GetDescription returns the tool description
func (t *AtomicGenerateManifestsTool) GetDescription() string {
	return "Generates Kubernetes deployment manifests including deployments, services, ingress, and secrets with GitOps support"
}

// GetVersion returns the tool version
func (t *AtomicGenerateManifestsTool) GetVersion() string {
	return "1.0.0"
}

// GetCapabilities returns the tool capabilities
func (t *AtomicGenerateManifestsTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

// GetMetadata returns comprehensive metadata about the tool
func (t *AtomicGenerateManifestsTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "atomic_generate_manifests",
		Description: "Generates Kubernetes deployment manifests including deployments, services, ingress, and secrets with GitOps support and automatic secret detection",
		Version:     "1.0.0",
		Category:    "kubernetes",
		Dependencies: []string{
			"session_manager",
			"kubernetes_access",
			"docker_image",
		},
		Capabilities: []string{
			"manifest_generation",
			"secret_detection",
			"secret_externalization",
			"gitops_ready_output",
			"template_selection",
			"resource_optimization",
			"ingress_generation",
			"configmap_generation",
		},
		Requirements: []string{
			"valid_session_id",
			"docker_image_reference",
		},
		Parameters: map[string]string{
			"session_id":      "string - Session ID for session context",
			"image_ref":       "string - Docker image reference (required)",
			"app_name":        "string - Application name (optional, derived from image if not provided)",
			"namespace":       "string - Kubernetes namespace (default: default)",
			"replicas":        "int - Number of replicas (default: 1)",
			"port":            "int - Application port (default: 8080)",
			"service_type":    "string - Kubernetes service type (ClusterIP, NodePort, LoadBalancer)",
			"include_ingress": "bool - Generate Ingress manifest",
			"secret_handling": "string - Secret handling strategy (auto, prompt, inline)",
			"secret_manager":  "string - Secret manager type (kubernetes, vault, azure)",
			"cpu_request":     "string - CPU resource request (e.g., '100m')",
			"memory_request":  "string - Memory resource request (e.g., '128Mi')",
			"cpu_limit":       "string - CPU resource limit (e.g., '500m')",
			"memory_limit":    "string - Memory resource limit (e.g., '512Mi')",
			"generate_helm":   "bool - Generate Helm chart templates",
			"gitops_ready":    "bool - Ensure manifests are GitOps ready (no inline secrets)",
			"environment":     "map[string]string - Environment variables",
			"dry_run":         "bool - Validate without generating files",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Basic Manifest Generation",
				Description: "Generate basic Kubernetes manifests for a containerized application",
				Input: map[string]interface{}{
					"session_id": "session-123",
					"image_ref":  "myapp:latest",
					"app_name":   "myapp",
					"namespace":  "default",
				},
				Output: map[string]interface{}{
					"success":             true,
					"manifests_generated": 2,
					"manifest_types":      []string{"Deployment", "Service"},
					"output_directory":    "/workspace/manifests",
				},
			},
			{
				Name:        "Production-Ready with Resources",
				Description: "Generate manifests with resource limits and ingress for production",
				Input: map[string]interface{}{
					"session_id":      "session-456",
					"image_ref":       "myregistry.io/myapp:v1.2.3",
					"app_name":        "myapp",
					"namespace":       "production",
					"replicas":        3,
					"include_ingress": true,
					"cpu_request":     "100m",
					"memory_request":  "256Mi",
					"cpu_limit":       "500m",
					"memory_limit":    "512Mi",
					"gitops_ready":    true,
				},
				Output: map[string]interface{}{
					"success":              true,
					"manifests_generated":  3,
					"manifest_types":       []string{"Deployment", "Service", "Ingress"},
					"secrets_externalized": 0,
					"gitops_ready":         true,
				},
			},
			{
				Name:        "With Secret Management",
				Description: "Generate manifests with automatic secret detection and externalization",
				Input: map[string]interface{}{
					"session_id":      "session-789",
					"image_ref":       "myapp:latest",
					"secret_handling": "auto",
					"environment": map[string]interface{}{
						"DATABASE_URL": "postgresql://user:secret@host:5432/db",
						"API_KEY":      "sk-1234567890abcdef",
						"DEBUG":        "false",
					},
				},
				Output: map[string]interface{}{
					"success":              true,
					"secrets_detected":     2,
					"secrets_externalized": 2,
					"secret_manifests":     []string{"app-secrets.yaml"},
					"security_level":       "high",
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicGenerateManifestsTool) Validate(ctx context.Context, args interface{}) error {
	manifestArgs, ok := args.(AtomicGenerateManifestsArgs)
	if !ok {
		// Try to convert from map if it's not already typed
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			manifestArgs, err = convertToAtomicGenerateManifestsArgs(mapArgs)
			if err != nil {
				return types.NewRichError("CONVERSION_ERROR", fmt.Sprintf("failed to convert arguments: %v", err), types.ErrTypeValidation)
			}
		} else {
			return mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_generate_manifests", map[string]interface{}{
				"expected": "AtomicGenerateManifestsArgs or map[string]interface{}",
				"received": fmt.Sprintf("%T", args),
			})
		}
	}

	if manifestArgs.ImageRef == "" {
		return mcperror.NewWithData("missing_required_field", "ImageRef is required", map[string]interface{}{
			"field": "image_ref",
		})
	}

	if manifestArgs.SessionID == "" {
		return mcperror.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicGenerateManifestsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Handle both typed and untyped arguments
	var manifestArgs AtomicGenerateManifestsArgs
	var err error

	switch a := args.(type) {
	case AtomicGenerateManifestsArgs:
		manifestArgs = a
	case map[string]interface{}:
		manifestArgs, err = convertToAtomicGenerateManifestsArgs(a)
		if err != nil {
			return nil, mcperror.NewWithData("conversion_error", fmt.Sprintf("Failed to convert arguments: %v", err), map[string]interface{}{
				"error": err.Error(),
			})
		}
	default:
		return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_generate_manifests", map[string]interface{}{
			"expected": "AtomicGenerateManifestsArgs or map[string]interface{}",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, manifestArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicGenerateManifestsTool) ExecuteTyped(ctx context.Context, args AtomicGenerateManifestsArgs) (*AtomicGenerateManifestsResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

// AtomicGenerateManifests is a Copilot-compatible wrapper that accepts untyped arguments
func AtomicGenerateManifests(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Create mock adapter and session manager for standalone usage
	adapter := &mockPipelineAdapter{}
	sessionManager := &mockSessionManager{}

	tool := NewAtomicGenerateManifestsTool(adapter, sessionManager, logger)

	// Convert untyped map to typed args
	typedArgs, err := convertToAtomicGenerateManifestsArgs(args)
	if err != nil {
		return nil, err
	}

	// Execute with typed args
	result, err := tool.ExecuteTyped(ctx, typedArgs)
	if err != nil {
		return nil, err
	}

	// Convert result to untyped map
	return convertAtomicGenerateManifestsResultToMap(result), nil
}

// convertToAtomicGenerateManifestsArgs converts untyped map to typed AtomicGenerateManifestsArgs
func convertToAtomicGenerateManifestsArgs(args map[string]interface{}) (AtomicGenerateManifestsArgs, error) {
	result := AtomicGenerateManifestsArgs{}

	// Base fields
	if sessionID, ok := args["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if dryRun, ok := args["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}

	// Required fields
	if imageRef, ok := args["image_ref"].(string); ok {
		result.ImageRef = imageRef
	}

	// Optional fields
	if appName, ok := args["app_name"].(string); ok {
		result.AppName = appName
	}
	if namespace, ok := args["namespace"].(string); ok {
		result.Namespace = namespace
	}
	if replicas, ok := args["replicas"].(float64); ok {
		result.Replicas = int(replicas)
	}
	if port, ok := args["port"].(float64); ok {
		result.Port = int(port)
	}
	if serviceType, ok := args["service_type"].(string); ok {
		result.ServiceType = serviceType
	}
	if includeIngress, ok := args["include_ingress"].(bool); ok {
		result.IncludeIngress = includeIngress
	}
	if secretHandling, ok := args["secret_handling"].(string); ok {
		result.SecretHandling = secretHandling
	}
	if secretManager, ok := args["secret_manager"].(string); ok {
		result.SecretManager = secretManager
	}
	if cpuRequest, ok := args["cpu_request"].(string); ok {
		result.CPURequest = cpuRequest
	}
	if memoryRequest, ok := args["memory_request"].(string); ok {
		result.MemoryRequest = memoryRequest
	}
	if cpuLimit, ok := args["cpu_limit"].(string); ok {
		result.CPULimit = cpuLimit
	}
	if memoryLimit, ok := args["memory_limit"].(string); ok {
		result.MemoryLimit = memoryLimit
	}
	if generateHelm, ok := args["generate_helm"].(bool); ok {
		result.GenerateHelm = generateHelm
	}
	if gitOpsReady, ok := args["gitops_ready"].(bool); ok {
		result.GitOpsReady = gitOpsReady
	}

	// Environment variables
	if env, ok := args["environment"].(map[string]interface{}); ok {
		result.Environment = make(map[string]string)
		for k, v := range env {
			if str, ok := v.(string); ok {
				result.Environment[k] = str
			}
		}
	}

	return result, nil
}

// convertAtomicGenerateManifestsResultToMap converts typed result to untyped map
func convertAtomicGenerateManifestsResultToMap(result *AtomicGenerateManifestsResult) map[string]interface{} {
	output := map[string]interface{}{
		"session_id":    result.SessionID,
		"success":       result.Success,
		"workspace_dir": result.WorkspaceDir,
		"image_ref":     result.ImageRef,
		"app_name":      result.AppName,
		"namespace":     result.Namespace,
	}

	// Add manifest result if present
	if result.ManifestResult != nil {
		manifestMap := map[string]interface{}{
			"success":    result.ManifestResult.Success,
			"output_dir": result.ManifestResult.OutputDir,
		}
		if len(result.ManifestResult.Manifests) > 0 {
			manifests := make([]map[string]interface{}, len(result.ManifestResult.Manifests))
			for i, m := range result.ManifestResult.Manifests {
				manifests[i] = map[string]interface{}{
					"name": m.Name,
					"kind": m.Kind,
					"path": m.Path,
				}
			}
			manifestMap["manifests"] = manifests
		}
		if result.ManifestResult.Error != nil {
			manifestMap["error"] = map[string]interface{}{
				"message": result.ManifestResult.Error.Message,
			}
		}
		output["manifest_result"] = manifestMap
	}

	// Add secrets information
	if len(result.SecretsDetected) > 0 {
		secrets := make([]map[string]interface{}, len(result.SecretsDetected))
		for i, s := range result.SecretsDetected {
			secrets[i] = map[string]interface{}{
				"name":           s.Name,
				"redacted_value": s.RedactedValue,
				"suggested_ref":  s.SuggestedRef,
				"pattern":        s.Pattern,
			}
		}
		output["secrets_detected"] = secrets
	}

	if result.SecretsPlan != nil {
		planMap := map[string]interface{}{
			"strategy":       result.SecretsPlan.Strategy,
			"secret_manager": result.SecretsPlan.SecretManager,
			"instructions":   result.SecretsPlan.Instructions,
		}
		if len(result.SecretsPlan.SecretReferences) > 0 {
			refs := make(map[string]interface{})
			for k, v := range result.SecretsPlan.SecretReferences {
				refs[k] = map[string]interface{}{
					"name": v.Name,
					"key":  v.Key,
				}
			}
			planMap["secret_references"] = refs
		}
		if len(result.SecretsPlan.ConfigMapEntries) > 0 {
			planMap["configmap_entries"] = result.SecretsPlan.ConfigMapEntries
		}
		output["secrets_plan"] = planMap
	}

	if len(result.SecretManifests) > 0 {
		manifests := make([]map[string]interface{}, len(result.SecretManifests))
		for i, m := range result.SecretManifests {
			manifests[i] = map[string]interface{}{
				"name":    m.Name,
				"kind":    m.Kind,
				"path":    m.Path,
				"purpose": m.Purpose,
			}
		}
		output["secret_manifests"] = manifests
	}

	// Add timing information
	output["generation_duration"] = result.GenerationDuration.String()
	output["total_duration"] = result.TotalDuration.String()

	// Add rich context if present
	if result.ManifestContext != nil {
		output["manifest_context"] = result.ManifestContext
	}

	if result.DeploymentStrategyContext != nil {
		output["deployment_strategy_context"] = result.DeploymentStrategyContext
	}

	return output
}

// Mock implementations for standalone usage
type mockPipelineAdapter struct{}

// Remove duplicate - this is implemented below with correct signature

func (m *mockPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	return fmt.Sprintf("/tmp/container-copilot/%s", sessionID)
}

// Implement remaining mcptypes.PipelineOperations methods
func (m *mockPipelineAdapter) AnalyzeRepository(sessionID, repoPath string) (*analysis.AnalysisResult, error) {
	return &analysis.AnalysisResult{}, nil
}

func (m *mockPipelineAdapter) CloneRepository(sessionID, repoURL, branch string) (*git.CloneResult, error) {
	return &git.CloneResult{}, nil
}

func (m *mockPipelineAdapter) GenerateDockerfile(sessionID, language, framework string) (string, error) {
	return "Dockerfile", nil
}

func (m *mockPipelineAdapter) BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*mcptypes.BuildResult, error) {
	return &mcptypes.BuildResult{
		Success:  true,
		ImageRef: imageRef,
		ImageID:  "mock-image-id",
	}, nil
}

// Remove duplicates - these are implemented below with correct signatures

func (m *mockPipelineAdapter) PreviewDeployment(sessionID, manifestPath, namespace string) (string, error) {
	return "preview", nil
}

func (m *mockPipelineAdapter) SaveAnalysisCache(sessionID string, result *analysis.AnalysisResult) error {
	return nil
}

func (m *mockPipelineAdapter) SetContext(sessionID string, ctx context.Context) {}
func (m *mockPipelineAdapter) GetContext(sessionID string) context.Context {
	return context.Background()
}
func (m *mockPipelineAdapter) ClearContext(sessionID string) {}

// Add missing PipelineOperations methods
func (m *mockPipelineAdapter) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	return nil
}

func (m *mockPipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	return nil
}

func (m *mockPipelineAdapter) PushDockerImage(sessionID, imageRef string) error {
	return nil
}

func (m *mockPipelineAdapter) TagDockerImage(sessionID, sourceRef, targetRef string) error {
	return nil
}

func (m *mockPipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	return &mcptypes.DockerState{}, nil
}

func (m *mockPipelineAdapter) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	return &mcptypes.KubernetesManifestResult{
		Success: true,
		Manifests: []mcptypes.GeneratedManifest{
			{Kind: "Deployment", Name: appName + "-deployment", Path: "deployment.yaml", Content: ""},
			{Kind: "Service", Name: appName + "-service", Path: "service.yaml", Content: ""},
		},
	}, nil
}

func (m *mockPipelineAdapter) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	return &mcptypes.KubernetesDeploymentResult{
		Success: true,
	}, nil
}

func (m *mockPipelineAdapter) CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	return &mcptypes.HealthCheckResult{
		Healthy: true,
	}, nil
}

func (m *mockPipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	return nil
}

func (m *mockPipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	return nil
}

type mockSessionManager struct{}

func (m *mockSessionManager) GetSession(sessionID string) (interface{}, error) {
	// Mock implementation
	return &sessiontypes.SessionState{
		SessionID: sessionID,
		Metadata:  make(map[string]interface{}),
	}, nil
}

func (m *mockSessionManager) GetOrCreateSession(repoURL string) (interface{}, error) {
	// Mock implementation
	return &sessiontypes.SessionState{
		SessionID: "mock-session-id",
		Metadata:  make(map[string]interface{}),
	}, nil
}

func (m *mockSessionManager) UpdateSession(sessionID string, updateFunc func(interface{})) error {
	// Mock implementation
	return nil
}

func (m *mockSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockSessionManager) ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (m *mockSessionManager) FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error) {
	return &sessiontypes.SessionState{
		SessionID: "mock-session-id",
		Metadata:  make(map[string]interface{}),
	}, nil
}

func (m *mockSessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	return m.GetSession(sessionID)
}

func (m *mockSessionManager) GetOrCreateSessionFromRepo(repoURL string) (interface{}, error) {
	return &sessiontypes.SessionState{
		SessionID: "mock-session-from-repo",
		RepoURL:   repoURL,
		Metadata:  make(map[string]interface{}),
	}, nil
}
