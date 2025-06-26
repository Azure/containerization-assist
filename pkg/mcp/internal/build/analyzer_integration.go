package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// FixingContext holds context for fixing operations
type FixingContext struct {
	SessionID       string
	ToolName        string
	OperationType   string
	OriginalError   error
	MaxAttempts     int
	BaseDir         string
	WorkspaceDir    string
	ErrorDetails    map[string]interface{}
	AttemptHistory  []mcptypes.FixAttempt
	EnvironmentInfo map[string]interface{}
	SessionMetadata map[string]interface{}
}

// AnalyzerIntegratedFixer combines IterativeFixer with CallerAnalyzer
type AnalyzerIntegratedFixer struct {
	fixer        mcptypes.IterativeFixer
	analyzer     mcptypes.AIAnalyzer
	contextShare mcptypes.ContextSharer
	logger       zerolog.Logger
}

// NewAnalyzerIntegratedFixer creates a fixer that integrates with CallerAnalyzer
func NewAnalyzerIntegratedFixer(analyzer mcptypes.AIAnalyzer, logger zerolog.Logger) *AnalyzerIntegratedFixer {
	// Use real DefaultIterativeFixer implementation instead of mock
	fixer := NewDefaultIterativeFixer(analyzer, logger)
	contextSharer := &realContextSharer{context: make(map[string]interface{})}

	return &AnalyzerIntegratedFixer{
		fixer:        fixer,
		analyzer:     analyzer,
		contextShare: contextSharer,
		logger:       logger.With().Str("component", "analyzer_integrated_fixer").Logger(),
	}
}

// FixWithAnalyzer performs AI-driven fixing using CallerAnalyzer
func (a *AnalyzerIntegratedFixer) FixWithAnalyzer(ctx context.Context, sessionID string, toolName string, operationType string, err error, maxAttempts int, baseDir string) (*mcptypes.FixingResult, error) {
	// Create fixing context
	fixingCtx := &FixingContext{
		SessionID:       sessionID,
		ToolName:        toolName,
		OperationType:   operationType,
		OriginalError:   err,
		MaxAttempts:     maxAttempts,
		BaseDir:         baseDir,
		ErrorDetails:    make(map[string]interface{}),
		AttemptHistory:  []mcptypes.FixAttempt{},
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
		fixingCtx.ErrorDetails["code"] = richError.Code
		fixingCtx.ErrorDetails["type"] = richError.Type
		fixingCtx.ErrorDetails["severity"] = richError.Severity
		fixingCtx.ErrorDetails["message"] = richError.Message
	} else {
		// Convert simple error to rich error for better analysis
		fixingCtx.ErrorDetails["code"] = "UNKNOWN_ERROR"
		fixingCtx.ErrorDetails["type"] = "operation_failure"
		fixingCtx.ErrorDetails["severity"] = "High"
		fixingCtx.ErrorDetails["message"] = err.Error()
	}

	// Share initial context for cross-tool coordination
	if a.contextShare != nil {
		err = a.contextShare.ShareContext(ctx, fmt.Sprintf("%s:failure_context", sessionID), map[string]interface{}{
			"tool":          toolName,
			"operation":     operationType,
			"error":         err.Error(),
			"base_dir":      baseDir,
			"workspace_dir": fixingCtx.WorkspaceDir,
		})
	}
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to share failure context")
	}

	// Attempt the fix
	var result *mcptypes.FixingResult
	var fixErr error
	if a.fixer != nil {
		result, fixErr = a.fixer.Fix(ctx, fixingCtx)
	} else {
		result = &mcptypes.FixingResult{
			Success: false,
			Error:   fmt.Errorf("fixer not initialized"),
		}
		fixErr = result.Error
	}
	if fixErr != nil {
		// Check if we should route this failure to another tool
		routing := a.fixer.GetFailureRouting()
		errorType := "unknown_error"
		if richError, ok := fixingCtx.OriginalError.(*types.RichError); ok {
			errorType = richError.Type
		}
		targetTool, hasRouting := routing[errorType]
		var routingErr error
		if !hasRouting {
			routingErr = fmt.Errorf("no routing for error type: %s", errorType)
		}
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

			// TODO: Fix ShareContext signature
			var shareErr error
			if a.contextShare != nil {
				shareErr = a.contextShare.ShareContext(ctx, fmt.Sprintf("%s:routing_context", sessionID), routingContext)
			}
			if shareErr != nil {
				a.logger.Error().Err(shareErr).Msg("Failed to share routing context")
			}

			// Add routing recommendation to result
			result.RecommendedNext = append(result.RecommendedNext,
				fmt.Sprintf("Route to %s tool for specialized fixing", targetTool))
		}

		return result, fixErr
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

		if a.contextShare != nil {
			err = a.contextShare.ShareContext(ctx, fmt.Sprintf("%s:success_context", sessionID), successContext)
		}
		if err != nil {
			a.logger.Warn().Err(err).Msg("Failed to share success context")
		}
	}

	return result, nil
}

// getWorkspaceDir retrieves the workspace directory for a session
func (a *AnalyzerIntegratedFixer) getWorkspaceDir(ctx context.Context, sessionID string) (string, error) {
	// TODO: Implement proper workspace directory retrieval
	return "", fmt.Errorf("not implemented")
}

// GetFixingRecommendations provides fixing recommendations without attempting fixes
func (a *AnalyzerIntegratedFixer) GetFixingRecommendations(ctx context.Context, sessionID string, toolName string, err error, baseDir string) ([]mcptypes.FixStrategy, error) {
	fixingCtx := &FixingContext{
		SessionID:     sessionID,
		ToolName:      toolName,
		OriginalError: err,
		BaseDir:       baseDir,
		ErrorDetails:  make(map[string]interface{}),
		MaxAttempts:   1, // We're just analyzing, not fixing
	}

	// Enhance error details
	if richError, ok := err.(*types.RichError); ok {
		fixingCtx.ErrorDetails["code"] = richError.Code
		fixingCtx.ErrorDetails["type"] = richError.Type
		fixingCtx.ErrorDetails["severity"] = richError.Severity
		fixingCtx.ErrorDetails["message"] = richError.Message
	} else {
		fixingCtx.ErrorDetails["code"] = "UNKNOWN_ERROR"
		fixingCtx.ErrorDetails["type"] = "operation_failure"
		fixingCtx.ErrorDetails["severity"] = "Medium"
		fixingCtx.ErrorDetails["message"] = err.Error()
	}

	// Get available fix strategies from the fixer
	strategyNames := a.fixer.GetFixStrategies()
	strategies := make([]mcptypes.FixStrategy, 0, len(strategyNames))

	// Convert strategy names to FixStrategy objects
	for i, name := range strategyNames {
		strategies = append(strategies, mcptypes.FixStrategy{
			Name:        name,
			Description: fmt.Sprintf("Apply %s strategy", name),
			Type:        getStrategyType(name),
			Priority:    i + 1, // Lower index = higher priority
		})
	}

	return strategies, nil
}

// AnalyzeErrorWithContext provides enhanced error analysis using shared context
func (a *AnalyzerIntegratedFixer) AnalyzeErrorWithContext(ctx context.Context, sessionID string, err error, baseDir string) (string, error) {
	// Get any relevant shared context
	var contextInfo []string

	// Try to get failure context
	if a.contextShare != nil {
		if failureCtx, ok := a.contextShare.GetSharedContext(ctx, fmt.Sprintf("%s:failure_context", sessionID)); ok {
			if failureMap, ok := failureCtx.(map[string]interface{}); ok {
				contextInfo = append(contextInfo, fmt.Sprintf("Previous failure context: %+v", failureMap))
			}
		}
	}

	// Try to get success context for learning
	if a.contextShare != nil {
		if successCtx, ok := a.contextShare.GetSharedContext(ctx, fmt.Sprintf("%s:success_context", sessionID)); ok {
			if successMap, ok := successCtx.(map[string]interface{}); ok {
				contextInfo = append(contextInfo, fmt.Sprintf("Previous success context: %+v", successMap))
			}
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

// mockIterativeFixer provides a minimal implementation for testing
type mockIterativeFixer struct {
	maxAttempts int
	history     []mcptypes.FixAttempt
	analyzer    mcptypes.AIAnalyzer
}

func (m *mockIterativeFixer) Fix(ctx context.Context, issue interface{}) (*mcptypes.FixingResult, error) {
	// Call the analyzer to simulate the real behavior
	if m.analyzer != nil {
		_, err := m.analyzer.AnalyzeWithFileTools(ctx, "Fix this Docker build error", "/tmp")
		if err != nil {
			return &mcptypes.FixingResult{
				Success: false,
				Error:   err,
			}, err
		}
	}

	// For testing, simulate a successful fix with working Dockerfile content
	attempt := mcptypes.FixAttempt{
		AttemptNumber: len(m.history) + 1,
		Success:       true,
		Error:         nil,
		Strategy:      "dockerfile",
		FixStrategy: mcptypes.FixStrategy{
			Name:        "Fix Dockerfile base image",
			Priority:    5,
			Type:        "dockerfile",
			Description: "Update the base image to a valid one",
		},
		FixedContent: `FROM node:18-alpine
WORKDIR /app
COPY . .
CMD ["echo", "hello"]`,
	}
	m.history = append(m.history, attempt)

	return &mcptypes.FixingResult{
		Success:       true,
		Error:         nil,
		FixApplied:    "Fixed Dockerfile base image",
		Attempts:      attempt.AttemptNumber,
		TotalAttempts: attempt.AttemptNumber,
		FixHistory:    []mcptypes.FixAttempt{attempt},
		FinalAttempt:  &attempt,
	}, nil
}

func (m *mockIterativeFixer) SetMaxAttempts(max int) {
	m.maxAttempts = max
}

func (m *mockIterativeFixer) GetFixHistory() []mcptypes.FixAttempt {
	return m.history
}

func (m *mockIterativeFixer) AttemptFix(ctx context.Context, issue interface{}, attempt int) (*mcptypes.FixingResult, error) {
	// For mock, just call Fix with limited attempts
	savedMax := m.maxAttempts
	m.maxAttempts = attempt
	result, err := m.Fix(ctx, issue)
	m.maxAttempts = savedMax
	return result, err
}

func (m *mockIterativeFixer) GetFailureRouting() map[string]string {
	return map[string]string{
		"build_error":  "dockerfile",
		"deploy_error": "kubernetes",
	}
}

func (m *mockIterativeFixer) GetFixStrategies() []string {
	return []string{"dockerfile_fix", "dependency_fix", "config_fix"}
}

// realContextSharer provides proper context sharing implementation
type realContextSharer struct {
	context map[string]interface{}
}

func (r *realContextSharer) ShareContext(ctx context.Context, key string, value interface{}) error {
	if r.context == nil {
		r.context = make(map[string]interface{})
	}
	r.context[key] = value
	return nil
}

func (r *realContextSharer) GetSharedContext(ctx context.Context, key string) (interface{}, bool) {
	if r.context == nil {
		return nil, false
	}
	value, exists := r.context[key]
	return value, exists
}

// getStrategyType infers the strategy type from its name
func getStrategyType(strategyName string) string {
	switch {
	case strings.Contains(strategyName, "dockerfile"):
		return "dockerfile"
	case strings.Contains(strategyName, "dependency"):
		return "dependency"
	case strings.Contains(strategyName, "config"):
		return "config"
	case strings.Contains(strategyName, "manifest"):
		return "manifest"
	case strings.Contains(strategyName, "network"):
		return "network"
	case strings.Contains(strategyName, "permission"):
		return "permission"
	default:
		return "general"
	}
}
