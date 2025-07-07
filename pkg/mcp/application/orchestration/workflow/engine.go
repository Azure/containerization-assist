package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/registry"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// SimpleWorkflowExecutor replaces the over-engineered WorkflowOrchestrator
// with a simple tool execution system
type SimpleWorkflowExecutor struct {
	logger         zerolog.Logger
	sessionManager session.UnifiedSessionManager
	toolRegistry   *registry.ToolRegistry
}

// NewSimpleWorkflowExecutor creates a simple workflow executor
func NewSimpleWorkflowExecutor(sessionManager session.UnifiedSessionManager, toolRegistry *registry.ToolRegistry, logger zerolog.Logger) *SimpleWorkflowExecutor {
	return createSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}

// NewSimpleWorkflowExecutorUnified creates a simple workflow executor using unified session manager
// Deprecated: Use NewSimpleWorkflowExecutor directly. This function will be removed in v2.0.0
func NewSimpleWorkflowExecutorUnified(sessionManager session.UnifiedSessionManager, toolRegistry *registry.ToolRegistry, logger zerolog.Logger) *SimpleWorkflowExecutor {
	return createSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}

// createSimpleWorkflowExecutor is the common creation logic
func createSimpleWorkflowExecutor(sessionManager session.UnifiedSessionManager, toolRegistry *registry.ToolRegistry, logger zerolog.Logger) *SimpleWorkflowExecutor {
	return &SimpleWorkflowExecutor{
		logger:         logger.With().Str("component", "simple_workflow").Logger(),
		sessionManager: sessionManager,
		toolRegistry:   toolRegistry,
	}
}

// ExecuteWorkflow executes a predefined workflow by calling tools sequentially
func (swe *SimpleWorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflowID string, options ...execution.ExecutionOption) (interface{}, error) {
	swe.logger.Info().Str("workflow_id", workflowID).Msg("Starting simple workflow execution")

	// Parse options for variables
	variables := make(map[string]interface{})

	for _, opt := range options {
		for k, v := range opt.Variables {
			variables[k] = v
		}
	}

	// Generate session ID
	sessionID := fmt.Sprintf("workflow_%d", time.Now().UnixNano())

	// Execute predefined workflow
	switch workflowID {
	case "analyze_and_build":
		return swe.executeAnalyzeAndBuild(ctx, sessionID, variables)
	case "deploy_application":
		return swe.executeDeployApplication(ctx, sessionID, variables)
	case "scan_and_fix":
		return swe.executeScanAndFix(ctx, sessionID, variables)
	case "containerize_app":
		return swe.executeContainerizeApp(ctx, sessionID, variables)
	case "full_deployment_pipeline":
		return swe.executeFullPipeline(ctx, sessionID, variables)
	case "security_audit":
		return swe.executeSecurityAudit(ctx, sessionID, variables)
	default:
		return nil, errors.NewError().Messagef("unknown workflow: %s", workflowID).WithLocation(

		// ExecuteCustomWorkflow executes a custom workflow specification
		).Build()
	}
}

func (swe *SimpleWorkflowExecutor) ExecuteCustomWorkflow(ctx context.Context, spec *execution.WorkflowSpec) (interface{}, error) {
	swe.logger.Info().Str("workflow_name", spec.Name).Msg("Starting custom workflow execution")

	sessionID := fmt.Sprintf("custom_%d", time.Now().UnixNano())
	results := make(map[string]interface{})

	// Execute stages sequentially
	for _, stage := range spec.Stages {
		swe.logger.Info().Str("stage", stage.ID).Str("stage_name", stage.Name).Msg("Executing stage")

		for _, toolName := range stage.Tools {
			result, err := swe.executeTool(ctx, sessionID, toolName, spec.Variables)
			if err != nil {
				swe.logger.Error().Err(err).Str("tool", toolName).Str("stage", stage.ID).Msg("Tool execution failed")
				return nil, errors.Wrapf(err, "orchestration", "stage %s failed", stage.ID)
			}
			results[fmt.Sprintf("%s_%s", stage.ID, toolName)] = result
		}
	}

	return map[string]interface{}{
		"status":     "completed",
		"session_id": sessionID,
		"results":    results,
	}, nil
}

// GetWorkflowStatus returns workflow status (simplified)
func (swe *SimpleWorkflowExecutor) GetWorkflowStatus(sessionID string) (string, error) {
	// Simple implementation - check if session exists
	_, err := swe.sessionManager.GetSession(context.Background(), sessionID)
	if err != nil {
		return "not_found", err
	}
	return "running", nil
}

// Predefined workflow implementations
func (swe *SimpleWorkflowExecutor) executeAnalyzeAndBuild(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	results := make(map[string]interface{})

	// Step 1: Analyze repository
	analyzeResult, err := swe.executeTool(ctx, sessionID, "analyze_repository", variables)
	if err != nil {
		return nil, errors.NewError().Message("analysis failed").Cause(err).Build()
	}
	results["analyze"] = analyzeResult

	// Step 2: Detect databases
	detectResult, err := swe.executeTool(ctx, sessionID, "detect_databases", variables)
	if err != nil {
		// Database detection is optional, log warning but continue
		swe.logger.Warn().Err(err).Msg("Database detection failed, continuing without database configuration")
		results["detect_databases"] = map[string]interface{}{"skipped": true, "reason": err.Error()}
	} else {
		results["detect_databases"] = detectResult
	}

	// Step 3: Build image
	buildResult, err := swe.executeTool(ctx, sessionID, "build_image", variables)
	if err != nil {
		return nil, errors.NewError().Message("build failed").Cause(err).Build()
	}
	results["build"] = buildResult

	return map[string]interface{}{
		"status":     "completed",
		"session_id": sessionID,
		"results":    results,
	}, nil
}

func (swe *SimpleWorkflowExecutor) executeDeployApplication(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	results := make(map[string]interface{})

	// Step 1: Detect databases (optional, for enhanced deployment configuration)
	detectResult, err := swe.executeTool(ctx, sessionID, "detect_databases", variables)
	if err != nil {
		// Database detection is optional, log warning but continue
		swe.logger.Warn().Err(err).Msg("Database detection failed, continuing without automatic database configuration")
		results["detect_databases"] = map[string]interface{}{"skipped": true, "reason": err.Error()}
	} else {
		results["detect_databases"] = detectResult
	}

	// Step 2: Generate manifests (will use detected databases if available)
	manifestResult, err := swe.executeTool(ctx, sessionID, "generate_manifests", variables)
	if err != nil {
		return nil, errors.NewError().Message("manifest generation failed").Cause(err).Build()
	}
	results["generate_manifests"] = manifestResult

	// Step 3: Deploy to Kubernetes
	deployResult, err := swe.executeTool(ctx, sessionID, "deploy_kubernetes", variables)
	if err != nil {
		return nil, errors.NewError().Message("deployment failed").Cause(err).Build()
	}
	results["deploy"] = deployResult

	return map[string]interface{}{
		"status":     "completed",
		"session_id": sessionID,
		"results":    results,
	}, nil
}

func (swe *SimpleWorkflowExecutor) executeScanAndFix(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	results := make(map[string]interface{})

	// Simple security scan workflow
	scanResult, err := swe.executeTool(ctx, sessionID, "scan_security", variables)
	if err != nil {
		return nil, errors.NewError().Message("security scan failed").Cause(err).Build()
	}
	results["scan"] = scanResult

	return map[string]interface{}{
		"status":     "completed",
		"session_id": sessionID,
		"results":    results,
	}, nil
}

func (swe *SimpleWorkflowExecutor) executeContainerizeApp(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	// Simplified containerization: analyze + build
	return swe.executeAnalyzeAndBuild(ctx, sessionID, variables)
}

func (swe *SimpleWorkflowExecutor) executeFullPipeline(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	results := make(map[string]interface{})

	// Full pipeline: analyze + detect databases + build + deploy
	analyzeResult, _ := swe.executeTool(ctx, sessionID, "analyze_repository", variables)
	results["analyze"] = analyzeResult

	// Detect databases for automatic configuration
	detectResult, _ := swe.executeTool(ctx, sessionID, "detect_databases", variables)
	results["detect_databases"] = detectResult

	buildResult, _ := swe.executeTool(ctx, sessionID, "build_image", variables)
	results["build"] = buildResult

	manifestResult, _ := swe.executeTool(ctx, sessionID, "generate_manifests", variables)
	results["manifests"] = manifestResult

	deployResult, _ := swe.executeTool(ctx, sessionID, "deploy_kubernetes", variables)
	results["deploy"] = deployResult

	return map[string]interface{}{
		"status":     "completed",
		"session_id": sessionID,
		"results":    results,
	}, nil
}

func (swe *SimpleWorkflowExecutor) executeSecurityAudit(ctx context.Context, sessionID string, variables map[string]interface{}) (interface{}, error) {
	return swe.executeScanAndFix(ctx, sessionID, variables)
}

// Helper to execute individual tools
func (swe *SimpleWorkflowExecutor) executeTool(ctx context.Context, sessionID, toolName string, variables map[string]interface{}) (interface{}, error) {
	swe.logger.Debug().Str("tool", toolName).Str("session_id", sessionID).Msg("Executing tool")

	// For now, return a simple success result
	// Real implementation would call actual tools through the registry
	return map[string]interface{}{
		"tool":       toolName,
		"status":     "completed",
		"timestamp":  time.Now(),
		"session_id": sessionID,
	}, nil
}

// ListAvailableWorkflows returns the list of available predefined workflows
func ListAvailableWorkflows() []string {
	return []string{
		"analyze_and_build",
		"deploy_application",
		"scan_and_fix",
		"containerize_app",
		"full_deployment_pipeline",
		"security_audit",
	}
}

// Compatibility aliases for existing code
type WorkflowOrchestrator = SimpleWorkflowExecutor

// NewWorkflowOrchestrator creates a new workflow orchestrator (compatibility wrapper)
func NewWorkflowOrchestrator(deps ...interface{}) *WorkflowOrchestrator {
	// Extract session manager, tool registry, and logger from deps
	var sessionManager session.UnifiedSessionManager
	var toolRegistry *registry.ToolRegistry
	var logger zerolog.Logger

	for _, dep := range deps {
		switch d := dep.(type) {
		case session.UnifiedSessionManager:
			sessionManager = d
		case *registry.ToolRegistry:
			toolRegistry = d
		case zerolog.Logger:
			logger = d
		}
	}

	// Provide defaults if needed
	if logger.GetLevel() == zerolog.Disabled {
		logger = zerolog.Nop()
	}

	return NewSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}
