package fixing

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/analyzer"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// AnalyzerIntegratedFixer combines IterativeFixer with CallerAnalyzer
type AnalyzerIntegratedFixer struct {
	fixer        IterativeFixer
	analyzer     analyzer.Analyzer
	contextShare ContextSharer
	logger       zerolog.Logger
}

// NewAnalyzerIntegratedFixer creates a fixer that integrates with CallerAnalyzer
func NewAnalyzerIntegratedFixer(analyzer analyzer.Analyzer, logger zerolog.Logger) *AnalyzerIntegratedFixer {
	fixer := NewDefaultIterativeFixer(analyzer, logger)
	contextSharer := NewDefaultContextSharer(logger)

	return &AnalyzerIntegratedFixer{
		fixer:        fixer,
		analyzer:     analyzer,
		contextShare: contextSharer,
		logger:       logger.With().Str("component", "analyzer_integrated_fixer").Logger(),
	}
}

// FixWithAnalyzer performs AI-driven fixing using CallerAnalyzer
func (a *AnalyzerIntegratedFixer) FixWithAnalyzer(ctx context.Context, sessionID string, toolName string, operationType string, err error, maxAttempts int, baseDir string) (*FixingResult, error) {
	// Create fixing context
	fixingCtx := &FixingContext{
		SessionID:       sessionID,
		ToolName:        toolName,
		OperationType:   operationType,
		OriginalError:   err,
		MaxAttempts:     maxAttempts,
		BaseDir:         baseDir,
		AttemptHistory:  []FixAttempt{},
		EnvironmentInfo: make(map[string]interface{}),
		SessionMetadata: make(map[string]interface{}),
	}

	// Get workspace directory from session context
	workspaceDir, err := a.getWorkspaceDir(ctx, sessionID)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Could not get workspace directory, using base dir")
		fixingCtx.WorkspaceDir = baseDir
	} else {
		fixingCtx.WorkspaceDir = workspaceDir
	}

	// Enhance error with rich details if possible
	if richError, ok := err.(*types.RichError); ok {
		fixingCtx.ErrorDetails = richError
	} else {
		// Convert simple error to rich error for better analysis
		fixingCtx.ErrorDetails = &types.RichError{
			Code:     "UNKNOWN_ERROR",
			Type:     "operation_failure",
			Severity: "High",
			Message:  err.Error(),
		}
	}

	// Share initial context for cross-tool coordination
	err = a.contextShare.ShareContext(ctx, sessionID, "failure_context", map[string]interface{}{
		"tool":          toolName,
		"operation":     operationType,
		"error":         err.Error(),
		"base_dir":      baseDir,
		"workspace_dir": fixingCtx.WorkspaceDir,
	})
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to share failure context")
	}

	// Attempt the fix
	result, err := a.fixer.AttemptFix(ctx, fixingCtx)
	if err != nil {
		// Check if we should route this failure to another tool
		targetTool, routingErr := a.contextShare.GetFailureRouting(ctx, sessionID, fixingCtx.ErrorDetails)
		if routingErr == nil && targetTool != toolName {
			a.logger.Info().
				Str("current_tool", toolName).
				Str("target_tool", targetTool).
				Msg("Routing failure to different tool")

			// Share context for the target tool
			routingContext := map[string]interface{}{
				"routed_from":        toolName,
				"original_error":     err.Error(),
				"fix_attempts":       result.AllAttempts,
				"recommended_action": fmt.Sprintf("Continue fixing in %s", targetTool),
			}

			shareErr := a.contextShare.ShareContext(ctx, sessionID, "routing_context", routingContext)
			if shareErr != nil {
				a.logger.Error().Err(shareErr).Msg("Failed to share routing context")
			}

			// Add routing recommendation to result
			result.RecommendedNext = append(result.RecommendedNext,
				fmt.Sprintf("Route to %s tool for specialized fixing", targetTool))
		}

		return result, err
	}

	// Share successful fix context for other tools to learn from
	if result.Success {
		successContext := map[string]interface{}{
			"tool":            toolName,
			"operation":       operationType,
			"fix_strategy":    result.FinalAttempt.FixStrategy.Name,
			"fix_duration":    result.TotalDuration,
			"attempts_needed": result.TotalAttempts,
		}

		err = a.contextShare.ShareContext(ctx, sessionID, "success_context", successContext)
		if err != nil {
			a.logger.Warn().Err(err).Msg("Failed to share success context")
		}
	}

	return result, nil
}

// GetFixingRecommendations provides fixing recommendations without attempting fixes
func (a *AnalyzerIntegratedFixer) GetFixingRecommendations(ctx context.Context, sessionID string, toolName string, err error, baseDir string) ([]FixStrategy, error) {
	fixingCtx := &FixingContext{
		SessionID:     sessionID,
		ToolName:      toolName,
		OriginalError: err,
		BaseDir:       baseDir,
		MaxAttempts:   1, // We're just analyzing, not fixing
	}

	// Enhance error details
	if richError, ok := err.(*types.RichError); ok {
		fixingCtx.ErrorDetails = richError
	} else {
		fixingCtx.ErrorDetails = &types.RichError{
			Code:     "UNKNOWN_ERROR",
			Type:     "operation_failure",
			Severity: "Medium",
			Message:  err.Error(),
		}
	}

	return a.fixer.GetFixStrategies(ctx, fixingCtx)
}

// AnalyzeErrorWithContext provides enhanced error analysis using shared context
func (a *AnalyzerIntegratedFixer) AnalyzeErrorWithContext(ctx context.Context, sessionID string, err error, baseDir string) (string, error) {
	// Get any relevant shared context
	var contextInfo []string

	// Try to get failure context
	if failureCtx, err := a.contextShare.GetSharedContext(ctx, sessionID, "failure_context"); err == nil {
		if failureMap, ok := failureCtx.(map[string]interface{}); ok {
			contextInfo = append(contextInfo, fmt.Sprintf("Previous failure context: %+v", failureMap))
		}
	}

	// Try to get success context for learning
	if successCtx, err := a.contextShare.GetSharedContext(ctx, sessionID, "success_context"); err == nil {
		if successMap, ok := successCtx.(map[string]interface{}); ok {
			contextInfo = append(contextInfo, fmt.Sprintf("Previous success context: %+v", successMap))
		}
	}

	// Build comprehensive analysis prompt
	prompt := fmt.Sprintf(`Analyze this error in the context of a containerization workflow:

Error: %s

Session Context:
%s

Please provide:
1. Root cause analysis
2. Impact assessment
3. Recommended fix approach
4. Alternative strategies if the primary approach fails

Use the file reading tools to examine the workspace at: %s
`, err.Error(), fmt.Sprintf("%v", contextInfo), baseDir)

	return a.analyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
}

// getWorkspaceDir attempts to get workspace directory from session context
func (a *AnalyzerIntegratedFixer) getWorkspaceDir(ctx context.Context, sessionID string) (string, error) {
	// This would integrate with the session manager to get workspace directory
	// Currently returns error to fall back to base directory
	return "", fmt.Errorf("workspace directory lookup not implemented")
}

// EnhancedFixingConfiguration provides tool-specific fixing configuration
type EnhancedFixingConfiguration struct {
	ToolName           string
	MaxAttempts        int
	EnableRouting      bool
	SeverityThreshold  string
	SpecializedPrompts map[string]string
}

// GetEnhancedConfiguration returns enhanced fixing configuration for a tool
func GetEnhancedConfiguration(toolName string) *EnhancedFixingConfiguration {
	configs := map[string]*EnhancedFixingConfiguration{
		"atomic_build_image": {
			ToolName:          "atomic_build_image",
			MaxAttempts:       3,
			EnableRouting:     true,
			SeverityThreshold: "Medium",
			SpecializedPrompts: map[string]string{
				"dockerfile_analysis": "Focus on Dockerfile syntax, base image compatibility, and build optimization",
				"dependency_analysis": "Analyze package dependencies, version conflicts, and installation issues",
			},
		},
		"atomic_deploy_kubernetes": {
			ToolName:          "atomic_deploy_kubernetes",
			MaxAttempts:       2,
			EnableRouting:     true,
			SeverityThreshold: "High",
			SpecializedPrompts: map[string]string{
				"manifest_analysis":   "Focus on Kubernetes manifest syntax, resource requirements, and cluster compatibility",
				"deployment_analysis": "Analyze deployment status, pod health, and service connectivity",
			},
		},
		"generate_manifests_atomic": {
			ToolName:          "generate_manifests_atomic",
			MaxAttempts:       3,
			EnableRouting:     false,
			SeverityThreshold: "Medium",
			SpecializedPrompts: map[string]string{
				"generation_analysis": "Focus on manifest template selection, parameter validation, and Kubernetes best practices",
			},
		},
	}

	if config, exists := configs[toolName]; exists {
		return config
	}

	// Default configuration
	return &EnhancedFixingConfiguration{
		ToolName:          toolName,
		MaxAttempts:       2,
		EnableRouting:     false,
		SeverityThreshold: "Medium",
		SpecializedPrompts: map[string]string{
			"default_analysis": "Analyze the error and provide practical fixing recommendations",
		},
	}
}
