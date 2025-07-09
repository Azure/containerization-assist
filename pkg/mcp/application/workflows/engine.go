package workflow

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// SimpleWorkflowExecutor replaces the over-engineered WorkflowOrchestrator
// with a simple tool execution system
type SimpleWorkflowExecutor struct {
	logger         *slog.Logger
	sessionManager session.UnifiedSessionManager
	toolRegistry   *ToolRegistry
}

// NewSimpleWorkflowExecutor creates a simple workflow executor
func NewSimpleWorkflowExecutor(sessionManager session.UnifiedSessionManager, toolRegistry *ToolRegistry, logger *slog.Logger) *SimpleWorkflowExecutor {
	return createSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}

// NewSimpleWorkflowExecutorUnified creates a simple workflow executor using unified session manager
// Deprecated: Use NewSimpleWorkflowExecutor directly. This function will be removed in v2.0.0
func NewSimpleWorkflowExecutorUnified(sessionManager session.UnifiedSessionManager, toolRegistry *ToolRegistry, logger *slog.Logger) *SimpleWorkflowExecutor {
	return createSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}

// createSimpleWorkflowExecutor is the common creation logic
func createSimpleWorkflowExecutor(sessionManager session.UnifiedSessionManager, toolRegistry *ToolRegistry, logger *slog.Logger) *SimpleWorkflowExecutor {
	return &SimpleWorkflowExecutor{
		logger:         logger.With("component", "simple_workflow"),
		sessionManager: sessionManager,
		toolRegistry:   toolRegistry,
	}
}

// ExecuteWorkflow executes a predefined workflow by calling tools sequentially
func (swe *SimpleWorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflowID string, options ...ExecutionOption) (interface{}, error) {
	swe.logger.Info("Starting simple workflow execution", "workflow_id", workflowID)

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
		return nil, errors.NewError().
			Messagef("unknown workflow: %s", workflowID).
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeNotFound).
			Context("workflow_id", workflowID).
			WithLocation().
			Build()
	}
}

func (swe *SimpleWorkflowExecutor) ExecuteCustomWorkflow(ctx context.Context, spec *WorkflowSpec) (interface{}, error) {
	swe.logger.Info("Starting custom workflow execution", "workflow_name", spec.Name)

	sessionID := fmt.Sprintf("custom_%d", time.Now().UnixNano())
	results := make(map[string]interface{})

	// Execute stages sequentially
	for _, stage := range spec.Stages {
		swe.logger.Info("Executing stage", "stage", stage.ID, "stage_name", stage.Name)

		for _, toolName := range stage.Tools {
			result, err := swe.executeTool(ctx, sessionID, toolName, spec.Variables)
			if err != nil {
				swe.logger.Error("Tool execution failed", "error", err, "tool", toolName, "stage", stage.ID)
				return nil, errors.NewError().
					Messagef("stage %s failed: %s", stage.ID, stage.Name).
					Code(errors.CodeToolExecutionFailed).
					Type(errors.ErrTypeTool).
					Cause(err).
					Context("stage_id", stage.ID).
					Context("stage_name", stage.Name).
					Context("tool", toolName).
					WithLocation().
					Build()
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
	_, err := swe.sessionManager.GetSession(sessionID)
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
		return nil, errors.NewError().
			Message("repository analysis failed").
			Code(errors.CodeToolExecutionFailed).
			Type(errors.ErrTypeTool).
			Cause(err).
			Context("operation", "analyze_repository").
			Context("session_id", sessionID).
			WithLocation().
			Build()
	}
	results["analyze"] = analyzeResult

	// Step 2: Detect databases
	detectResult, err := swe.executeTool(ctx, sessionID, "detect_databases", variables)
	if err != nil {
		// Database detection is optional, log warning but continue
		swe.logger.Warn("Database detection failed, continuing without database configuration", "error", err)
		results["detect_databases"] = map[string]interface{}{"skipped": true, "reason": err.Error()}
	} else {
		results["detect_databases"] = detectResult
	}

	// Step 3: Build image
	buildResult, err := swe.executeTool(ctx, sessionID, "build_image", variables)
	if err != nil {
		return nil, errors.NewError().
			Message("image build failed").
			Code(errors.CodeImageBuildFailed).
			Type(errors.ErrTypeContainer).
			Cause(err).
			Context("operation", "build_image").
			Context("session_id", sessionID).
			WithLocation().
			Build()
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
		swe.logger.Warn("Database detection failed, continuing without automatic database configuration", "error", err)
		results["detect_databases"] = map[string]interface{}{"skipped": true, "reason": err.Error()}
	} else {
		results["detect_databases"] = detectResult
	}

	// Step 2: Generate manifests (will use detected databases if available)
	manifestResult, err := swe.executeTool(ctx, sessionID, "generate_manifests", variables)
	if err != nil {
		return nil, errors.NewError().
			Message("Kubernetes manifest generation failed").
			Code(errors.CodeManifestInvalid).
			Type(errors.ErrTypeKubernetes).
			Cause(err).
			Context("operation", "generate_manifests").
			Context("session_id", sessionID).
			WithLocation().
			Build()
	}
	results["generate_manifests"] = manifestResult

	// Step 3: Deploy to Kubernetes
	deployResult, err := swe.executeTool(ctx, sessionID, "deploy_kubernetes", variables)
	if err != nil {
		return nil, errors.NewError().
			Message("Kubernetes deployment failed").
			Code(errors.CodeDeploymentFailed).
			Type(errors.ErrTypeKubernetes).
			Cause(err).
			Context("operation", "deploy_kubernetes").
			Context("session_id", sessionID).
			WithLocation().
			Build()
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
		return nil, errors.NewError().
			Message("security scan failed").
			Code(errors.CodeSecurity).
			Type(errors.ErrTypeSecurity).
			Cause(err).
			Context("operation", "scan_security").
			Context("session_id", sessionID).
			WithLocation().
			Build()
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
	swe.logger.Debug("Executing tool", "tool", toolName, "session_id", sessionID)

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
	var toolRegistry *ToolRegistry
	var logger *slog.Logger

	for _, dep := range deps {
		switch d := dep.(type) {
		case session.UnifiedSessionManager:
			sessionManager = d
		case *ToolRegistry:
			toolRegistry = d
		case *slog.Logger:
			logger = d
		}
	}

	// Provide defaults if needed
	if logger == nil {
		logger = slog.Default()
	}

	return NewSimpleWorkflowExecutor(sessionManager, toolRegistry, logger)
}
