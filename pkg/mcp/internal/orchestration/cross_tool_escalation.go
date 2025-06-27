package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// CrossToolEscalationHandler manages cross-tool error escalation with enhanced context sharing
type CrossToolEscalationHandler struct {
	errorRouter    *DefaultErrorRouter
	contextSharer  *build.DefaultContextSharer
	sessionManager mcptypes.ToolSessionManager
	toolExecutor   ToolExecutor
	logger         zerolog.Logger
}

// NewCrossToolEscalationHandler creates a new cross-tool escalation handler
func NewCrossToolEscalationHandler(
	errorRouter *DefaultErrorRouter,
	contextSharer *build.DefaultContextSharer,
	sessionManager mcptypes.ToolSessionManager,
	toolExecutor ToolExecutor,
	logger zerolog.Logger,
) *CrossToolEscalationHandler {
	return &CrossToolEscalationHandler{
		errorRouter:    errorRouter,
		contextSharer:  contextSharer,
		sessionManager: sessionManager,
		toolExecutor:   toolExecutor,
		logger:         logger.With().Str("component", "cross_tool_escalation").Logger(),
	}
}

// EscalationContext contains rich context for cross-tool escalation
type EscalationContext struct {
	SourceTool      string                 `json:"source_tool"`
	TargetTool      string                 `json:"target_tool"`
	ErrorType       string                 `json:"error_type"`
	ErrorMessage    string                 `json:"error_message"`
	RootCauses      []string               `json:"root_causes"`
	FixAttempted    bool                   `json:"fix_attempted"`
	FixResult       string                 `json:"fix_result,omitempty"`
	SharedData      map[string]interface{} `json:"shared_data"`
	Recommendations []string               `json:"recommendations"`
}

// HandleToolFailure processes a tool failure and determines if cross-tool escalation is needed
func (h *CrossToolEscalationHandler) HandleToolFailure(
	ctx context.Context,
	sessionID string,
	sourceTool string,
	toolError error,
	toolResult interface{},
) (*ErrorAction, error) {
	h.logger.Info().
		Str("session_id", sessionID).
		Str("source_tool", sourceTool).
		Err(toolError).
		Msg("Handling tool failure for potential cross-tool escalation")

	// Create workflow error from tool error
	workflowError := h.createWorkflowError(sourceTool, toolError, toolResult)

	// Get session for routing
	sess, err := h.getWorkflowSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Route the error to determine action
	action, err := h.errorRouter.RouteError(ctx, workflowError, sess)
	if err != nil {
		return nil, fmt.Errorf("failed to route error: %w", err)
	}

	// If action is redirect, handle cross-tool escalation
	if action.Action == "redirect" && action.RedirectTo != "" {
		h.logger.Info().
			Str("source_tool", sourceTool).
			Str("target_tool", action.RedirectTo).
			Msg("Performing cross-tool escalation")

		// Share escalation context
		escalationCtx := h.buildEscalationContext(sourceTool, action.RedirectTo, workflowError, toolResult)
		if err := h.shareEscalationContext(ctx, sessionID, escalationCtx); err != nil {
			h.logger.Warn().Err(err).Msg("Failed to share escalation context")
		}

		// Execute the target tool with escalation parameters
		if h.toolExecutor != nil {
			targetResult, err := h.executeTargetTool(ctx, sessionID, action, escalationCtx)
			if err != nil {
				h.logger.Error().
					Err(err).
					Str("target_tool", action.RedirectTo).
					Msg("Target tool execution failed during escalation")
				action.Success = false
				action.Error = err.Error()
			} else {
				action.Success = true
				action.Result = targetResult
			}
		}
	}

	return action, nil
}

// createWorkflowError converts a tool error into a workflow error
func (h *CrossToolEscalationHandler) createWorkflowError(sourceTool string, toolError error, toolResult interface{}) *WorkflowError {
	errorType := "tool_error"
	severity := "high"
	retryable := true

	// Extract error details from rich error if available
	if richErr, ok := toolError.(*mcptypes.RichError); ok {
		errorType = richErr.Type
		severity = strings.ToLower(richErr.Severity)
	}

	// Check if tool result contains failure analysis
	rootCauses := []string{}
	if result, ok := toolResult.(interface{ GetFailureAnalysis() *FailureAnalysis }); ok {
		if fa := result.GetFailureAnalysis(); fa != nil {
			rootCauses = fa.RootCauses
			if fa.ImpactSeverity != "" {
				severity = strings.ToLower(fa.ImpactSeverity)
			}
		}
	}

	return &WorkflowError{
		ID:         fmt.Sprintf("%s_%d", sourceTool, time.Now().Unix()),
		StageName:  sourceTool,
		ToolName:   sourceTool,
		ErrorType:  errorType,
		Message:    toolError.Error(),
		Timestamp:  time.Now(),
		Severity:   severity,
		Retryable:  retryable,
		RootCauses: rootCauses,
	}
}

// getWorkflowSession retrieves or creates a workflow session
func (h *CrossToolEscalationHandler) getWorkflowSession(sessionID string) (*WorkflowSession, error) {
	// Try to get existing session
	sessInterface, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// Convert to session state
	sessState, ok := sessInterface.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("invalid session type")
	}

	// Create workflow session wrapper
	return &WorkflowSession{
		ID:            sessState.SessionID,
		StartTime:     sessState.CreatedAt,
		ErrorContext:  make(map[string]interface{}),
		SharedContext: make(map[string]interface{}),
	}, nil
}

// buildEscalationContext creates rich context for cross-tool escalation
func (h *CrossToolEscalationHandler) buildEscalationContext(
	sourceTool string,
	targetTool string,
	workflowError *WorkflowError,
	toolResult interface{},
) *EscalationContext {
	ctx := &EscalationContext{
		SourceTool:   sourceTool,
		TargetTool:   targetTool,
		ErrorType:    workflowError.ErrorType,
		ErrorMessage: workflowError.Message,
		RootCauses:   workflowError.RootCauses,
		SharedData:   make(map[string]interface{}),
	}

	// Extract recommendations from tool result
	if result, ok := toolResult.(interface{ GetRecommendations() []string }); ok {
		ctx.Recommendations = result.GetRecommendations()
	}

	// Add tool-specific context
	switch sourceTool {
	case "deploy_kubernetes", "atomic_deploy_kubernetes":
		ctx.SharedData["deployment_stage"] = "failed"
		ctx.SharedData["requires_rebuild"] = strings.Contains(workflowError.Message, "image")
		ctx.SharedData["requires_manifest_fix"] = strings.Contains(workflowError.Message, "manifest")

	case "build_image", "atomic_build_image":
		ctx.SharedData["build_stage"] = "failed"
		ctx.SharedData["requires_dockerfile_fix"] = strings.Contains(workflowError.Message, "dockerfile")
		ctx.SharedData["requires_dependency_fix"] = strings.Contains(workflowError.Message, "dependency")

	case "generate_manifests":
		ctx.SharedData["manifest_stage"] = "failed"
		ctx.SharedData["requires_port_fix"] = strings.Contains(workflowError.Message, "port")
		ctx.SharedData["requires_resource_fix"] = strings.Contains(workflowError.Message, "resource")
	}

	return ctx
}

// shareEscalationContext shares escalation context between tools
func (h *CrossToolEscalationHandler) shareEscalationContext(ctx context.Context, sessionID string, escalationCtx *EscalationContext) error {
	// Share general escalation context
	contextData := map[string]interface{}{
		"escalation_context": escalationCtx,
		"timestamp":          time.Now(),
	}

	if err := h.contextSharer.ShareContext(ctx, sessionID, "escalation_context", contextData); err != nil {
		return fmt.Errorf("failed to share escalation context: %w", err)
	}

	// Share tool-specific failure context
	failureContext := map[string]interface{}{
		"source_tool":     escalationCtx.SourceTool,
		"error_type":      escalationCtx.ErrorType,
		"root_causes":     escalationCtx.RootCauses,
		"shared_data":     escalationCtx.SharedData,
		"recommendations": escalationCtx.Recommendations,
	}

	if err := h.contextSharer.ShareContext(ctx, sessionID, "failure_context", failureContext); err != nil {
		return fmt.Errorf("failed to share failure context: %w", err)
	}

	h.logger.Info().
		Str("session_id", sessionID).
		Str("source_tool", escalationCtx.SourceTool).
		Str("target_tool", escalationCtx.TargetTool).
		Msg("Successfully shared escalation context")

	return nil
}

// executeTargetTool executes the target tool with escalation parameters
func (h *CrossToolEscalationHandler) executeTargetTool(
	ctx context.Context,
	sessionID string,
	action *ErrorAction,
	escalationCtx *EscalationContext,
) (interface{}, error) {
	h.logger.Info().
		Str("target_tool", action.RedirectTo).
		Str("source_tool", escalationCtx.SourceTool).
		Msg("Executing target tool for escalation")

	// Build tool input with escalation parameters
	toolInput := ToolInput{
		SessionID: sessionID,
		Name:      action.RedirectTo,
		Parameters: map[string]interface{}{
			"session_id":        sessionID,
			"escalation_mode":   "auto",
			"escalation_source": escalationCtx.SourceTool,
			"fix_errors":        true,
		},
	}

	// Add specific parameters based on target tool
	switch action.RedirectTo {
	case "build_image", "atomic_build_image":
		toolInput.Parameters["rebuild_image"] = true
		if escalationCtx.SharedData["requires_dockerfile_fix"] == true {
			toolInput.Parameters["fix_dockerfile"] = true
		}

	case "generate_manifests":
		toolInput.Parameters["fix_manifests"] = true
		if escalationCtx.SharedData["requires_resource_fix"] == true {
			toolInput.Parameters["fix_resources"] = true
		}

	case "generate_dockerfile":
		toolInput.Parameters["fix_dockerfile"] = true
		toolInput.Parameters["deep_analysis"] = true
	}

	// Execute the tool
	result, err := h.toolExecutor.ExecuteTool(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("failed to execute target tool %s: %w", action.RedirectTo, err)
	}

	// Share success context if execution succeeded
	if err == nil {
		successContext := map[string]interface{}{
			"escalation_successful": true,
			"source_tool":           escalationCtx.SourceTool,
			"target_tool":           action.RedirectTo,
			"timestamp":             time.Now(),
		}
		_ = h.contextSharer.ShareContext(ctx, sessionID, "escalation_success", successContext)
	}

	return result, nil
}

// GetEscalationHistory retrieves the escalation history for a session
func (h *CrossToolEscalationHandler) GetEscalationHistory(ctx context.Context, sessionID string) ([]EscalationContext, error) {
	// Retrieve escalation contexts from shared context
	sharedCtx, err := h.contextSharer.GetSharedContext(ctx, sessionID, "escalation_context")
	if err != nil {
		return nil, err
	}

	// Parse escalation history
	history := []EscalationContext{}
	if ctxData, ok := sharedCtx.(map[string]interface{}); ok {
		if escalationCtx, ok := ctxData["escalation_context"].(*EscalationContext); ok {
			history = append(history, *escalationCtx)
		}
	}

	return history, nil
}

// FailureAnalysis represents a failure analysis (interface compatibility)
type FailureAnalysis struct {
	FailureType    string
	FailureStage   string
	RootCauses     []string
	ImpactSeverity string
}
