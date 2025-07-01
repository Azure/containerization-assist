package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
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
	AttemptHistory  []interface{}
	EnvironmentInfo map[string]interface{}
	SessionMetadata map[string]interface{}
}

// AnalyzerIntegratedFixer combines IterativeFixer with CallerAnalyzer
type AnalyzerIntegratedFixer struct {
	fixer        mcptypes.IterativeFixer
	analyzer     core.AIAnalyzer
	contextShare mcptypes.ContextSharer
	logger       zerolog.Logger
}

// NewAnalyzerIntegratedFixer creates a fixer that integrates with CallerAnalyzer
func NewAnalyzerIntegratedFixer(analyzer core.AIAnalyzer, logger zerolog.Logger) *AnalyzerIntegratedFixer {
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
		AttemptHistory:  []interface{}{},
		EnvironmentInfo: make(map[string]interface{}),
		SessionMetadata: make(map[string]interface{}),
	}
	// Use baseDir as workspace directory (simplified)
	fixingCtx.WorkspaceDir = baseDir
	// Enhance error with rich details if possible
	if err != nil {
		fixingCtx.ErrorDetails["code"] = err.Error()
		fixingCtx.ErrorDetails["type"] = "unknown"
		fixingCtx.ErrorDetails["severity"] = "medium"
		fixingCtx.ErrorDetails["message"] = err.Error()
	} else {
		// Convert simple error to rich error for better analysis
		fixingCtx.ErrorDetails["code"] = "UNKNOWN_ERROR"
		fixingCtx.ErrorDetails["type"] = "operation_failure"
		fixingCtx.ErrorDetails["severity"] = "High"
		fixingCtx.ErrorDetails["message"] = err.Error()
	}
	// Share initial context for cross-tool coordination
	if a.contextShare != nil {
		shareErr := a.contextShare.ShareContext(ctx, sessionID, "failure_context", map[string]interface{}{
			"tool":          toolName,
			"operation":     operationType,
			"error":         err.Error(),
			"base_dir":      baseDir,
			"workspace_dir": fixingCtx.WorkspaceDir,
		})
		if shareErr != nil {
			a.logger.Warn().Err(shareErr).Msg("Failed to share failure context")
		}
	}
	// Attempt the fix
	var result *mcptypes.FixingResult
	var fixErr error
	if a.fixer != nil {
		result, fixErr = a.fixer.AttemptFix(ctx, sessionID, toolName, operationType, err, maxAttempts, baseDir)
	} else {
		result = &mcptypes.FixingResult{
			Success: false,
		}
	}
	// Handle the fix result
	if result != nil && result.Success {
		a.logger.Info().Msg("Fix applied successfully")
	}
	return result, fixErr
}

// AnalyzeErrorWithContext provides enhanced error analysis using shared context
func (a *AnalyzerIntegratedFixer) AnalyzeErrorWithContext(ctx context.Context, sessionID string, err error, baseDir string) (string, error) {
	// Get any relevant shared context
	var contextInfo []string
	// Try to get failure context
	if a.contextShare != nil {
		if failureCtx, err := a.contextShare.GetSharedContext(ctx, sessionID, "failure_context"); err == nil {
			if failureMap, ok := failureCtx.(map[string]interface{}); ok {
				contextInfo = append(contextInfo, fmt.Sprintf("Previous failure context: %+v", failureMap))
			}
		}
	}
	// Try to get success context for learning
	if a.contextShare != nil {
		if successCtx, err := a.contextShare.GetSharedContext(ctx, sessionID, "success_context"); err == nil {
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
	history     []interface{}
	analyzer    core.AIAnalyzer
}

func (m *mockIterativeFixer) Fix(ctx context.Context, issue interface{}) (*mcptypes.FixingResult, error) {
	// Call the analyzer to simulate the real behavior
	if m.analyzer != nil {
		_, err := m.analyzer.AnalyzeWithFileTools(ctx, "Fix this Docker build error", "/tmp")
		if err != nil {
			return &mcptypes.FixingResult{
				Success:    false,
				FinalError: err,
			}, err
		}
	}
	// For testing, simulate a successful fix with working Dockerfile content
	attempt := map[string]interface{}{
		"AttemptNumber": len(m.history) + 1,
		"Success":       true,
		"Error":         nil,
		"Strategy":      "dockerfile",
		"FixStrategy": map[string]interface{}{
			"Name":        "Fix Dockerfile base image",
			"Priority":    5,
			"Type":        "dockerfile",
			"Description": "Update the base image to a valid one",
		},
		"FixedContent": `FROM node:18-alpine
WORKDIR /app
COPY . .
CMD ["echo", "hello"]`,
	}
	m.history = append(m.history, attempt)
	attemptNum := len(m.history)
	return &mcptypes.FixingResult{
		Success:        true,
		FixApplied:     true,
		FixDescription: "Fixed Dockerfile base image",
		AttemptsUsed:   attemptNum,
		TotalAttempts:  attemptNum,
		AllAttempts:    []interface{}{attempt},
	}, nil
}
func (m *mockIterativeFixer) SetMaxAttempts(max int) {
	m.maxAttempts = max
}
func (m *mockIterativeFixer) GetFixHistory() []interface{} {
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

func (r *realContextSharer) ShareContext(ctx context.Context, sessionID string, contextType string, data interface{}) error {
	if r.context == nil {
		r.context = make(map[string]interface{})
	}
	key := sessionID + ":" + contextType
	r.context[key] = data
	return nil
}

func (r *realContextSharer) GetSharedContext(ctx context.Context, sessionID string, contextType string) (interface{}, error) {
	if r.context == nil {
		return nil, fmt.Errorf("no context found")
	}
	key := sessionID + ":" + contextType
	value, exists := r.context[key]
	if !exists {
		return nil, fmt.Errorf("context not found for %s", key)
	}
	return value, nil
}

func (r *realContextSharer) ClearContext(ctx context.Context, sessionID string) error {
	if r.context == nil {
		return nil
	}
	// Clear all contexts for the session
	for key := range r.context {
		if strings.HasPrefix(key, sessionID+":") {
			delete(r.context, key)
		}
	}
	return nil
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
