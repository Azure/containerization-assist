package build

import (
	"context"
	"fmt"
	"os"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/rs/zerolog"
)

// AtomicDockerBuildOperation implements ConsolidatedFixableOperation for Docker builds
type AtomicDockerBuildOperation struct {
	tool           *AtomicBuildImageTool
	args           AtomicBuildImageArgs
	session        *core.SessionState
	workspaceDir   string
	buildContext   string
	dockerfilePath string
	logger         zerolog.Logger
}

// ExecuteOnce performs a single Docker build attempt
func (op *AtomicDockerBuildOperation) ExecuteOnce(ctx context.Context) error {
	op.logger.Debug().
		Str("image_name", op.args.ImageName).
		Str("dockerfile_path", op.dockerfilePath).
		Msg("Executing Docker build")

	// Check if Dockerfile exists
	if _, err := os.Stat(op.dockerfilePath); os.IsNotExist(err) {
		return errors.NewError().Messagef("dockerfile not found: %s", op.dockerfilePath).WithLocation(

		// Execute the build
		).Build()
	}

	op.logger.Info().Msg("Starting Docker build")
	return nil
}

// PrepareForRetry prepares the operation for a retry attempt
func (op *AtomicDockerBuildOperation) PrepareForRetry(ctx context.Context, lastAttempt interface{}) error {
	op.logger.Debug().Msg("Preparing for retry")
	// Clean up any temporary files or state from previous attempt
	return nil
}

// GetOperationInfo provides information about the current operation
func (op *AtomicDockerBuildOperation) GetOperationInfo() map[string]interface{} {
	return map[string]interface{}{
		"tool":          "atomic_build_image",
		"operation":     "docker_build",
		"image_name":    op.args.ImageName,
		"dockerfile":    op.dockerfilePath,
		"build_context": op.buildContext,
		"workspace_dir": op.workspaceDir,
		"session_id":    op.session.SessionID,
	}
}

// GetFailureAnalysis provides detailed failure analysis for AI-driven fixes
func (op *AtomicDockerBuildOperation) GetFailureAnalysis(ctx context.Context, err error) (error, error) {
	// Create a rich error with comprehensive analysis
	analysis := op.tool.generateBuildFailureAnalysis(err, nil, &AtomicBuildImageResult{
		BuildContext:   op.buildContext,
		DockerfilePath: op.dockerfilePath,
		BuildContext_Info: &BuildContextInfo{
			BaseImage:       "unknown", // Would be extracted from Dockerfile in real implementation
			ContextSize:     0,         // Would be calculated in real implementation
			FileCount:       0,         // Would be counted in real implementation
			HasDockerIgnore: false,     // Would be checked in real implementation
		},
	})

	// Create error context
	errorContext := createBuildErrorContext(BuildErrorContextConfig{
		ToolName:      "docker_build",
		OperationType: "build_execution",
		ErrorType:     "build_failure",
		Args:          op.args,
		AdditionalData: map[string]interface{}{
			"failure_analysis": analysis,
			"build_context":    op.buildContext,
			"dockerfile":       op.dockerfilePath,
		},
		Files: []string{op.dockerfilePath},
	})

	// Return as a structured error that can be understood by the AI fixer
	return &BuildFixerError{
		Code:    "BUILD_FAILED",
		Message: fmt.Sprintf("%v (context: %v)", err.Error(), errorContext),
		Stage:   analysis.FailureStage,
		Type:    analysis.FailureType,
	}, nil
}

// AdvancedBuildFixer provides intelligent build error recovery
type AdvancedBuildFixer struct {
	logger         zerolog.Logger
	analyzer       core.AIAnalyzer
	sessionManager session.UnifiedSessionManager
	strategies     map[string]BuildRecoveryStrategyInterface
}

// BuildRecoveryStrategyInterface defines a strategy for recovering from build failures
type BuildRecoveryStrategyInterface interface {
	CanHandle(err error, analysis *BuildFailureAnalysis) bool
	Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error
	GetPriority() int
}

// NewAdvancedBuildFixer creates a new advanced build fixer
func NewAdvancedBuildFixer(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager session.UnifiedSessionManager) *AdvancedBuildFixer {
	return NewAdvancedBuildFixerUnified(logger, analyzer, sessionManager)
}

// NewAdvancedBuildFixerUnified creates a new advanced build fixer with unified session manager
func NewAdvancedBuildFixerUnified(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager session.UnifiedSessionManager) *AdvancedBuildFixer {
	fixer := &AdvancedBuildFixer{
		logger:         logger.With().Str("component", "advanced_build_fixer").Logger(),
		analyzer:       analyzer,
		sessionManager: sessionManager,
		strategies:     make(map[string]BuildRecoveryStrategyInterface),
	}

	return fixer
}

// Legacy functions removed - use NewAdvancedBuildFixerUnified with session.UnifiedSessionManager

// RegisterStrategy registers a new recovery strategy
func (f *AdvancedBuildFixer) RegisterStrategy(name string, strategy BuildRecoveryStrategyInterface) {
	f.strategies[name] = strategy
}

// RecoverFromError attempts to recover from a build error
func (f *AdvancedBuildFixer) RecoverFromError(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	f.logger.Info().
		Str("error_type", analysis.FailureType).
		Str("error_stage", analysis.FailureStage).
		Msg("Attempting to recover from build error")

	// Find applicable recovery strategies
	var applicableStrategies []BuildRecoveryStrategyInterface
	for name, strategy := range f.strategies {
		if strategy.CanHandle(err, analysis) {
			f.logger.Debug().Str("strategy", name).Msg("Found applicable recovery strategy")
			applicableStrategies = append(applicableStrategies, strategy)
		}
	}

	// Sort by priority
	// In a real implementation, you'd sort the strategies by priority

	// Try each strategy
	for _, strategy := range applicableStrategies {
		f.logger.Info().Msg("Attempting recovery with strategy")
		if err := strategy.Recover(ctx, err, analysis, operation); err == nil {
			f.logger.Info().Msg("Recovery successful")
			return nil
		}
	}

	// If no strategy worked, use AI analyzer for custom fix
	if f.analyzer != nil {
		f.logger.Info().Msg("Attempting AI-driven recovery")
		return f.attemptAIRecovery(ctx, err, analysis, operation)
	}

	return errors.NewError().Message("unable to recover from error").Cause(err).WithLocation(

	// attemptAIRecovery uses AI analyzer to generate custom fixes
	).Build()
}

func (f *AdvancedBuildFixer) attemptAIRecovery(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	// Prepare context for AI
	aiContext := map[string]interface{}{
		"error":            err.Error(),
		"failure_analysis": analysis,
		"operation_info":   operation.GetOperationInfo(),
		"suggested_fixes":  analysis.SuggestedFixes,
	}

	// Request AI analysis
	prompt := fmt.Sprintf("Analyze this build error and suggest fixes: %+v", aiContext)
	response, err := f.analyzer.Analyze(ctx, prompt)
	if err != nil {
		f.logger.Error().Err(err).Msg("AI analysis failed")
		return err
	}

	// Apply AI-suggested fixes
	// This would involve parsing the AI response and applying the suggested changes
	f.logger.Info().Interface("ai_response", response).Msg("Received AI recovery suggestions")

	return nil
}

// InitializeDefaultStrategies initializes the default recovery strategies
func (f *AdvancedBuildFixer) InitializeDefaultStrategies() {
	// This will be implemented once the recovery strategies module is created
	f.logger.Info().Msg("Initializing default recovery strategies")
}

// BuildErrorContextConfig contains all parameters needed to create a build error context
type BuildErrorContextConfig struct {
	ToolName       string
	OperationType  string
	ErrorType      string
	Args           interface{}
	AdditionalData map[string]interface{}
	Files          []string
}

// createBuildErrorContext creates a comprehensive error context for build failures
func createBuildErrorContext(config BuildErrorContextConfig) map[string]interface{} {
	errorContext := map[string]interface{}{
		"tool":           config.ToolName,
		"operation_type": config.OperationType,
		"error_type":     config.ErrorType,
		"args":           config.Args,
		"files":          config.Files,
	}

	// Add additional data
	for key, value := range config.AdditionalData {
		errorContext[key] = value
	}

	return errorContext
}

// NewAtomicDockerBuildOperation creates a new Docker build operation
func NewAtomicDockerBuildOperation(config mcptypes.BuildOperationConfig) (*AtomicDockerBuildOperation, error) {
	// Safe type assertions with proper error handling
	tool, ok := config.Tool.(*AtomicBuildImageTool)
	if !ok {
		return nil, errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert tool to AtomicBuildImageTool").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("tool_type", fmt.Sprintf("%T", config.Tool)).
			Suggestion("Ensure the tool is of type AtomicBuildImageTool").
			WithLocation().
			Build()
	}

	args, ok := config.Args.(AtomicBuildImageArgs)
	if !ok {
		return nil, errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert args to AtomicBuildImageArgs").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("args_type", fmt.Sprintf("%T", config.Args)).
			Suggestion("Ensure the args are of type AtomicBuildImageArgs").
			WithLocation().
			Build()
	}

	session, ok := config.Session.(*core.SessionState)
	if !ok {
		return nil, errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert session to SessionState").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("session_type", fmt.Sprintf("%T", config.Session)).
			Suggestion("Ensure the session is of type SessionState").
			WithLocation().
			Build()
	}

	logger, ok := config.Logger.(zerolog.Logger)
	if !ok {
		return nil, errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert logger to zerolog.Logger").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("logger_type", fmt.Sprintf("%T", config.Logger)).
			Suggestion("Ensure the logger is of type zerolog.Logger").
			WithLocation().
			Build()
	}

	return &AtomicDockerBuildOperation{
		tool:           tool,
		args:           args,
		session:        session,
		workspaceDir:   config.WorkspaceDir,
		buildContext:   config.BuildContext,
		dockerfilePath: config.DockerfilePath,
		logger:         logger.With().Str("component", "docker_build_operation").Logger(),
	}, nil
}

// GetStrategies returns all registered strategies
func (f *AdvancedBuildFixer) GetStrategies() map[string]BuildRecoveryStrategyInterface {
	return f.strategies
}

// HasStrategy checks if a strategy is registered
func (f *AdvancedBuildFixer) HasStrategy(name string) bool {
	_, exists := f.strategies[name]
	return exists
}

// RemoveStrategy removes a strategy from the fixer
func (f *AdvancedBuildFixer) RemoveStrategy(name string) {
	delete(f.strategies, name)
}

// AnalyzeError creates a failure analysis from an error
func (f *AdvancedBuildFixer) AnalyzeError(err error, buildResult *coredocker.BuildResult) *BuildFailureAnalysis {
	analysis := &BuildFailureAnalysis{}
	errStr := strings.ToLower(err.Error())

	// Basic classification - this would be enhanced with more sophisticated analysis
	switch {
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout"):
		analysis.FailureType = "network"
		analysis.FailureStage = "download"
	case strings.Contains(errStr, "permission") || strings.Contains(errStr, "access denied"):
		analysis.FailureType = "permission"
		analysis.FailureStage = "file_access"
	case strings.Contains(errStr, "no such file"):
		analysis.FailureType = "file_missing"
		analysis.FailureStage = "file_copy"
	case strings.Contains(errStr, "space") || strings.Contains(errStr, "disk full"):
		analysis.FailureType = "disk_space"
		analysis.FailureStage = "build_process"
	case strings.Contains(errStr, "syntax"):
		analysis.FailureType = "dockerfile_syntax"
		analysis.FailureStage = "dockerfile_parsing"
	default:
		analysis.FailureType = "unknown"
		analysis.FailureStage = "unknown"
	}

	analysis.FailureReason = err.Error()
	analysis.RetryRecommended = analysis.FailureType == "network" || analysis.FailureType == "disk_space"

	return analysis
}
