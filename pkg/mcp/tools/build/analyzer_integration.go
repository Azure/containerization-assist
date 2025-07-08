package build

import (
	"context"
	"fmt"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	coretypes "github.com/Azure/container-kit/pkg/mcp/core"
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
	fixer        *DefaultIterativeFixer
	analyzer     core.AIAnalyzer
	contextShare *DefaultContextSharer
	logger       *slog.Logger
}

// NewAnalyzerIntegratedFixer creates a fixer that integrates with CallerAnalyzer
func NewAnalyzerIntegratedFixer(analyzer core.AIAnalyzer, logger *slog.Logger) *AnalyzerIntegratedFixer {
	// Use real DefaultIterativeFixer implementation instead of mock
	fixer := NewDefaultIterativeFixer(analyzer, logger)
	contextSharer := NewDefaultContextSharer(logger)
	return &AnalyzerIntegratedFixer{
		fixer:        fixer,
		analyzer:     analyzer,
		contextShare: contextSharer,
		logger:       logger.With("component", "analyzer_integrated_fixer"),
	}
}

// FixWithAnalyzer performs AI-driven fixing using CallerAnalyzer
func (a *AnalyzerIntegratedFixer) FixWithAnalyzer(ctx context.Context, request types.FixRequest) (*coretypes.FixingResult, error) {
	// Create fixing context
	fixingCtx := &FixingContext{
		SessionID:       request.SessionID,
		ToolName:        request.ToolName,
		OperationType:   request.OperationType,
		OriginalError:   request.Error,
		MaxAttempts:     request.MaxAttempts,
		BaseDir:         request.BaseDir,
		ErrorDetails:    make(map[string]interface{}),
		AttemptHistory:  []interface{}{},
		EnvironmentInfo: make(map[string]interface{}),
		SessionMetadata: make(map[string]interface{}),
	}
	// Use baseDir as workspace directory (simplified)
	fixingCtx.WorkspaceDir = request.BaseDir
	// Enhance error with rich details if possible
	if request.Error != nil {
		fixingCtx.ErrorDetails["code"] = request.Error.Error()
		fixingCtx.ErrorDetails["type"] = "unknown"
		fixingCtx.ErrorDetails["severity"] = "medium"
		fixingCtx.ErrorDetails["message"] = request.Error.Error()
	} else {
		// Convert simple error to rich error for better analysis
		fixingCtx.ErrorDetails["code"] = "UNKNOWN_ERROR"
		fixingCtx.ErrorDetails["type"] = "operation_failure"
		fixingCtx.ErrorDetails["severity"] = "High"
		fixingCtx.ErrorDetails["message"] = "Unknown error occurred"
	}
	// Share initial context for cross-tool coordination
	if a.contextShare != nil {
		shareErr := a.contextShare.ShareContext(ctx, request.SessionID, "failure_context", map[string]interface{}{
			"tool":      request.ToolName,
			"operation": request.OperationType,
			"error": func() string {
				if request.Error != nil {
					return request.Error.Error()
				}
				return "unknown error"
			}(),
			"base_dir":      request.BaseDir,
			"workspace_dir": fixingCtx.WorkspaceDir,
		})
		if shareErr != nil {
			a.logger.Warn("Failed to share failure context", "error", shareErr)
		}
	}
	// Attempt the fix
	var result *coretypes.FixingResult
	var fixErr error
	if a.fixer != nil {
		result, fixErr = a.fixer.AttemptFix(ctx, request)
	} else {
		result = &coretypes.FixingResult{
			Success: false,
		}
	}
	// Handle the fix result
	if result != nil && result.Success {
		a.logger.Info("Fix applied successfully")
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
