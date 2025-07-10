package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

type WorkflowError struct {
	ID        string    `json:"id"`
	StageName string    `json:"stage_name"`
	ToolName  string    `json:"tool_name"`
	ErrorType string    `json:"error_type"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}
type WorkflowSession struct {
	SessionID                string                 `json:"session_id"`
	Context                  map[string]interface{} `json:"context"`
	ConsolidatedErrorContext map[string]interface{} `json:"consolidated_error_context"`
}
type ErrorAction struct {
	Type       string                 `json:"type"`
	Action     string                 `json:"action"`
	RedirectTo string                 `json:"redirect_to"`
	Metadata   map[string]interface{} `json:"metadata"`
	Parameters map[string]interface{} `json:"parameters"`
}
type UserPreferences = shared.UserPreferences

type ConversationHandler struct {
	promptManager    *PromptManager
	sessionManager   session.SessionManager
	toolOrchestrator api.Orchestrator
	preferenceStore  *shared.PreferenceStore
	logger           *slog.Logger
}
type ConversationHandlerConfig struct {
	SessionManager     session.SessionManager
	SessionAdapter     session.SessionManager
	PreferenceStore    *shared.PreferenceStore
	PipelineOperations interface{} // TypedPipelineOperations - not used, keeping for compatibility
	ToolOrchestrator   api.Orchestrator
	Transport          interface{}
	Logger             *slog.Logger
}

func NewConversationHandler(config ConversationHandlerConfig) (*ConversationHandler, error) {

	var toolOrchestrator api.Orchestrator
	if config.ToolOrchestrator != nil {

		toolOrchestrator = config.ToolOrchestrator
		config.Logger.Info("Using provided canonical orchestrator for conversation handler")
	} else {
		return nil, errors.NewError().Messagef("tool orchestrator is required for conversation handler").WithLocation().Build()
	}

	promptManager := NewPromptManager(PromptManagerConfig{
		SessionManager:   config.SessionManager,
		ToolOrchestrator: toolOrchestrator,
		PreferenceStore:  config.PreferenceStore,
		Logger:           config.Logger,
	})

	handler := &ConversationHandler{
		promptManager:    promptManager,
		sessionManager:   config.SessionManager,
		toolOrchestrator: toolOrchestrator,
		preferenceStore:  config.PreferenceStore,
		logger:           config.Logger,
	}
	promptManager.SetConversationHandler(handler)

	return handler, nil
}
func (ch *ConversationHandler) HandleConversation(ctx context.Context, args ChatToolArgs) (*ChatToolResult, error) {
	if args.Message == "" {
		return nil, errors.NewError().Messagef("message parameter is required").WithLocation().Build()
	}

	response, err := ch.promptManager.ProcessPrompt(ctx, args.SessionID, args.Message)
	if err != nil {
		return &ChatToolResult{
			Success: false,
			Message: fmt.Sprintf("Failed to process prompt: %v", err),
		}, nil
	}
	finalResponse, err := ch.handleAutoAdvance(ctx, response)
	if err != nil {
		ch.logger.Error("Auto-advance failed", "error", err)

		finalResponse = response
	}
	result := &ChatToolResult{
		Success:   true,
		SessionID: finalResponse.SessionID,
		Message:   finalResponse.Message,
		Stage:     string(finalResponse.Stage),
		Status:    string(finalResponse.Status),
	}

	if len(finalResponse.Options) > 0 {
		options := make([]map[string]interface{}, len(finalResponse.Options))
		for i, opt := range finalResponse.Options {
			options[i] = map[string]interface{}{
				"id":          opt.ID,
				"label":       opt.Label,
				"description": opt.Description,
				"recommended": opt.Recommended,
			}
		}
		result.Options = options
	}

	if len(finalResponse.NextSteps) > 0 {
		result.NextSteps = finalResponse.NextSteps
	}

	if finalResponse.Progress != nil {
		result.Progress = map[string]interface{}{
			"current_stage": string(finalResponse.Progress.CurrentStage),
			"current_step":  finalResponse.Progress.CurrentStep,
			"total_steps":   finalResponse.Progress.TotalSteps,
			"percentage":    finalResponse.Progress.Percentage,
		}
	}

	return result, nil
}
func (ch *ConversationHandler) handleAutoAdvance(ctx context.Context, response *ConversationResponse) (*ConversationResponse, error) {
	if response == nil {
		return response, nil
	}
	var userPrefs shared.UserPreferences = shared.UserPreferences{
		SkipConfirmations: false,
	}
	if sessionID := response.SessionID; sessionID != "" {
		coreSession, err := ch.sessionManager.GetSessionTyped(sessionID)
		if err == nil && coreSession != nil {
			if coreSession.Metadata != nil {
				if repoAnalysis, ok := coreSession.Metadata["repo_analysis"].(map[string]interface{}); ok {
					if sessionCtx, ok := repoAnalysis["_context"].(map[string]interface{}); ok {
						if autopilotEnabled, exists := sessionCtx["autopilot_enabled"].(bool); exists && autopilotEnabled {

							userPrefs.SkipConfirmations = true
						}
					}
				}
			}
		}
	}

	maxAdvanceSteps := 5
	currentResponse := response

	for i := 0; i < maxAdvanceSteps; i++ {
		if !currentResponse.ShouldAutoAdvance(userPrefs) {
			break
		}

		ch.logger.Debug("Auto-advancing conversation",
			"session_id", currentResponse.SessionID,
			"stage", string(currentResponse.Stage))
		nextMessage := ""
		if currentResponse.AutoAdvance != nil && currentResponse.AutoAdvance.DefaultAction != "" {
			nextMessage = currentResponse.AutoAdvance.DefaultAction
		} else {

			nextMessage = "continue"
		}
		nextResponse, err := ch.promptManager.ProcessPrompt(ctx, currentResponse.SessionID, nextMessage)
		if err != nil {
			ch.logger.Error("Auto-advance processing failed", "error", err)
			return currentResponse, err
		}
		currentResponse = nextResponse
		if !currentResponse.CanAutoAdvance() {
			break
		}
	}

	return currentResponse, nil
}
func (ch *ConversationHandler) attemptAutoFix(ctx context.Context, sessionID string, stage shared.ConversationStage, err error, state *ConversationState) (*AutoFixResult, error) {
	ch.logger.Info("Attempting automatic fix before manual intervention",
		"session_id", sessionID,
		"stage", string(stage),
		"error", err)

	failureAnalysis := make(map[string]interface{})
	if latestTurn := state.GetLatestTurn(); latestTurn != nil && len(latestTurn.ToolCalls) > 0 {
		lastToolCall := latestTurn.ToolCalls[len(latestTurn.ToolCalls)-1]
		if lastToolCall.Result != nil {

			if resultMap, ok := lastToolCall.Result.(map[string]interface{}); ok {
				if fa, exists := resultMap["failure_analysis"]; exists && fa != nil {
					failureAnalysis, _ = fa.(map[string]interface{})
				}
			}
		}
	}
	workflowError := &WorkflowError{
		ID:        fmt.Sprintf("%s_%d", sessionID, time.Now().Unix()),
		StageName: string(stage),
		ToolName:  ch.getToolNameForStage(stage),
		ErrorType: ch.classifyError(err),
		Message:   err.Error(),
		Severity:  ch.getErrorSeverity(err),
		Timestamp: time.Now(),
	}
	workflowSession := &WorkflowSession{
		SessionID: sessionID,
		Context:   make(map[string]interface{}),
		ConsolidatedErrorContext: map[string]interface{}{
			"conversation_stage": string(stage),
			"state":              state,
			"failure_analysis":   failureAnalysis,
		},
	}

	_ = workflowSession
	errorAction := &ErrorAction{
		Type: "retry",
		Metadata: map[string]interface{}{
			"error_type": workflowError.ErrorType,
			"severity":   workflowError.Severity,
			"action":     "retry",
		},
		Parameters: make(map[string]interface{}),
	}

	result := &AutoFixResult{
		Success:        false,
		AttemptedFixes: []string{},
	}
	actionType := errorAction.Type
	if errorAction.Action != "" {
		actionType = errorAction.Action
	}
	switch actionType {
	case "retry":
		result.AttemptedFixes = append(result.AttemptedFixes, "Automatic retry with enhanced parameters")
		success := ch.attemptRetryFix(ctx, sessionID, stage, errorAction)
		result.Success = success

	case "redirect":
		result.AttemptedFixes = append(result.AttemptedFixes, fmt.Sprintf("Cross-tool escalation to %s", errorAction.RedirectTo))
		success := ch.attemptRedirectFix(ctx, sessionID, errorAction.RedirectTo, workflowError)
		result.Success = success

	case "skip":
		result.AttemptedFixes = append(result.AttemptedFixes, "Automatic skip with warning")
		result.Success = true

	case "fail":
		result.AttemptedFixes = append(result.AttemptedFixes, "Analyzed error - manual intervention required")
		result.Success = false
	}
	result.FallbackOptions = ch.generateFallbackOptions(stage, err, errorAction)

	ch.logger.Info("Auto-fix attempt completed",
		"success", result.Success,
		"attempted_fixes", result.AttemptedFixes,
		"fallback_options", len(result.FallbackOptions))

	return result, nil
}

type AutoFixResult struct {
	Success         bool     `json:"success"`
	AttemptedFixes  []string `json:"attempted_fixes"`
	FallbackOptions []Option `json:"fallback_options"`
	Message         string   `json:"message"`
}

func (ch *ConversationHandler) getToolNameForStage(stage shared.ConversationStage) string {
	switch stage {
	case shared.StageDockerfile, shared.StageBuild:
		return "build_image"
	case shared.StagePush:
		return "push_image"
	case shared.StageDeployment:
		return "deploy_kubernetes"
	case shared.StageManifests:
		return "generate_manifests"
	default:
		return "unknown"
	}
}

func (ch *ConversationHandler) classifyError(err error) string {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "build"):
		return "build_error"
	case strings.Contains(errMsg, "push"):
		return "push_error"
	case strings.Contains(errMsg, "deploy"):
		return "deployment_error"
	case strings.Contains(errMsg, "manifest"):
		return "manifest_error"
	case strings.Contains(errMsg, "dockerfile"):
		return "dockerfile_error"
	case strings.Contains(errMsg, "network"):
		return "network_error"
	case strings.Contains(errMsg, "auth") || strings.Contains(errMsg, "authentication"):
		return "authentication_error"
	case strings.Contains(errMsg, "registry"):
		return "registry_error"
	default:
		return "unknown_error"
	}
}

func (ch *ConversationHandler) getErrorSeverity(err error) string {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "fatal") || strings.Contains(errMsg, "critical"):
		return "critical"
	case strings.Contains(errMsg, "error"):
		return "high"
	case strings.Contains(errMsg, "warning"):
		return "medium"
	default:
		return "high"
	}
}

func (ch *ConversationHandler) attemptRetryFix(ctx context.Context, sessionID string, stage shared.ConversationStage, action *ErrorAction) bool {

	convState, err := ch.prepareRetrySession(sessionID)
	if err != nil {
		return false
	}
	lastToolCall := ch.findOrBuildLastToolCall(convState, stage)
	if lastToolCall == nil {
		ch.logger.Error("Could not determine tool call for retry")
		return false
	}
	return ch.executeRetryWithEnhancements(ctx, lastToolCall, action)
}
func (ch *ConversationHandler) prepareRetrySession(sessionID string) (*ConversationState, error) {

	sessionInterface, err := ch.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		ch.logger.Error("Failed to get session for retry", "error", err)
		return nil, err
	}
	internalSession := sessionInterface
	if internalSession == nil {
		ch.logger.Error("Session is nil during retry")
		return nil, fmt.Errorf("session is nil")
	}
	convState := &ConversationState{
		SessionState: internalSession,
		History:      make([]ConversationTurn, 0),
		Context:      make(map[string]interface{}),
	}
	convState = ch.loadConversationHistory(convState, internalSession)
	return convState, nil
}
func (ch *ConversationHandler) loadConversationHistory(convState *ConversationState, internalSession *session.SessionState) *ConversationState {
	if internalSession.Metadata == nil {
		return convState
	}

	history, ok := internalSession.Metadata["conversation_history"].([]interface{})
	if !ok {
		return convState
	}

	for _, turnData := range history {
		turnMap, ok := turnData.(map[string]interface{})
		if !ok {
			continue
		}

		turn := ConversationTurn{
			UserInput: "",
			ToolCalls: make([]ToolCall, 0),
			Assistant: "",
		}

		if userMsg, ok := turnMap["user_message"].(string); ok {
			turn.UserInput = userMsg
		}
		if response, ok := turnMap["response"].(string); ok {
			turn.Assistant = response
		}
		if toolCallsData, ok := turnMap["tool_calls"].([]interface{}); ok {
			for _, tcData := range toolCallsData {
				if tcMap, ok := tcData.(map[string]interface{}); ok {
					toolCall := ToolCall{}
					if tool, ok := tcMap["tool"].(string); ok {
						toolCall.Tool = tool
					}
					if params, ok := tcMap["parameters"].(map[string]interface{}); ok {
						toolCall.Parameters = params
					}
					turn.ToolCalls = append(turn.ToolCalls, toolCall)
				}
			}
		}
		convState.History = append(convState.History, turn)
	}
	return convState
}
func (ch *ConversationHandler) findOrBuildLastToolCall(convState *ConversationState, stage shared.ConversationStage) *ToolCall {

	if lastToolCall := ch.findLastToolCallInHistory(convState, stage); lastToolCall != nil {
		return lastToolCall
	}
	return ch.buildToolCallFromMetadata(convState.SessionState, stage)
}
func (ch *ConversationHandler) findLastToolCallInHistory(convState *ConversationState, stage shared.ConversationStage) *ToolCall {
	if len(convState.History) == 0 {
		return nil
	}

	toolName := ch.getToolNameForStage(stage)
	for i := len(convState.History) - 1; i >= 0; i-- {
		turn := convState.History[i]
		if len(turn.ToolCalls) == 0 {
			continue
		}
		for _, tc := range turn.ToolCalls {
			if tc.Tool == toolName || strings.Contains(tc.Tool, strings.TrimSuffix(toolName, "_atomic")) {
				return &tc
			}
		}
	}
	return nil
}
func (ch *ConversationHandler) buildToolCallFromMetadata(internalSession *session.SessionState, stage shared.ConversationStage) *ToolCall {
	toolName := ch.getToolNameForStage(stage)
	params := make(map[string]interface{})
	params["session_id"] = internalSession.SessionID

	if internalSession.Metadata == nil {
		return &ToolCall{Tool: toolName, Parameters: params}
	}
	switch stage {
	case shared.StageBuild:
		ch.addBuildParameters(params, internalSession.Metadata)
	case shared.StageDeployment:
		ch.addDeploymentParameters(params, internalSession.Metadata)
	}

	return &ToolCall{Tool: toolName, Parameters: params}
}
func (ch *ConversationHandler) addBuildParameters(params map[string]interface{}, metadata map[string]interface{}) {
	if imageRef, ok := metadata["image_ref"].(string); ok {
		params["image_ref"] = imageRef
	}
	if imageName, ok := metadata["image_name"].(string); ok {
		params["image_name"] = imageName
	}
	if dockerfilePath, ok := metadata["dockerfile_path"].(string); ok {
		params["dockerfile_path"] = dockerfilePath
	}
}
func (ch *ConversationHandler) addDeploymentParameters(params map[string]interface{}, metadata map[string]interface{}) {
	if manifestPath, ok := metadata["manifest_path"].(string); ok {
		params["manifest_path"] = manifestPath
	}
	if imageRef, ok := metadata["image_ref"].(string); ok {
		params["image_ref"] = imageRef
	}
	if namespace, ok := metadata["namespace"].(string); ok {
		params["namespace"] = namespace
	}
}
func (ch *ConversationHandler) executeRetryWithEnhancements(ctx context.Context, lastToolCall *ToolCall, action *ErrorAction) bool {

	enhancedParams := ch.buildEnhancedParameters(lastToolCall, action)
	result, err := ch.toolOrchestrator.ExecuteTool(ctx, lastToolCall.Tool, enhancedParams)
	if err != nil {
		ch.logger.Error("Retry execution failed",
			"error", err,
			"tool", lastToolCall.Tool)
		return false
	}

	ch.logger.Info("Retry succeeded",
		"result", result,
		"tool", lastToolCall.Tool)

	return true
}
func (ch *ConversationHandler) buildEnhancedParameters(lastToolCall *ToolCall, action *ErrorAction) map[string]interface{} {
	enhancedParams := make(map[string]interface{})
	for k, v := range lastToolCall.Parameters {
		enhancedParams[k] = v
	}
	if action.Parameters != nil {
		for k, v := range action.Parameters {
			enhancedParams[k] = v
		}
	}
	enhancedParams["is_retry"] = true

	if _, exists := enhancedParams["retry_count"]; !exists {
		enhancedParams["retry_count"] = 1
	}

	return enhancedParams
}

func (ch *ConversationHandler) attemptRedirectFix(ctx context.Context, sessionID string, redirectTo string, workflowError *WorkflowError) bool {

	ch.logger.Info("Attempting redirect fix",
		"session_id", sessionID,
		"redirect_to", redirectTo,
		"from_tool", workflowError.ToolName)
	sessionInterface, err := ch.sessionManager.GetSessionConcrete(sessionID)
	if err != nil {
		ch.logger.Error("Failed to get session for redirect", "error", err)
		return false
	}
	internalSession := sessionInterface
	if internalSession == nil {
		ch.logger.Error("Session is nil during redirect")
		return false
	}
	redirectParams := make(map[string]interface{})
	redirectParams["session_id"] = sessionID
	if internalSession.Metadata != nil {

		if imageRef, ok := internalSession.Metadata["image_ref"].(string); ok {
			redirectParams["image_ref"] = imageRef
		}
		if appName, ok := internalSession.Metadata["app_name"].(string); ok {
			redirectParams["app_name"] = appName
		}
		if namespace, ok := internalSession.Metadata["namespace"].(string); ok {
			redirectParams["namespace"] = namespace
		}
	}
	switch redirectTo {
	case "validate_dockerfile", "validate_dockerfile_atomic":

		if dockerfilePath, ok := internalSession.Metadata["dockerfile_path"].(string); ok {
			redirectParams["dockerfile_path"] = dockerfilePath
		}
		redirectParams["generate_fixes"] = true

	case "generate_dockerfile":

		redirectParams["force_regenerate"] = true
		if optimization, ok := internalSession.Metadata["optimization"].(string); ok {
			redirectParams["optimization"] = optimization
		}

	case "scan_image_security", "scan_image_security_atomic":

		redirectParams["fail_on_critical"] = false

	case "check_health", "check_health_atomic":

		redirectParams["include_logs"] = true
		redirectParams["log_lines"] = 50
	}
	redirectParams["error_context"] = map[string]interface{}{
		"original_tool": workflowError.ToolName,
		"error_message": workflowError.Message,
		"error_type":    workflowError.ErrorType,
		"is_redirect":   true,
	}

	ch.logger.Info("Executing redirect tool",
		"redirect_tool", redirectTo,
		"params", redirectParams)
	result, err := ch.toolOrchestrator.ExecuteTool(ctx, redirectTo, redirectParams)

	if err != nil {
		ch.logger.Error("Redirect fix failed",
			"error", err,
			"redirect_tool", redirectTo)
		return false
	}
	err = ch.sessionManager.UpdateSession(context.Background(), sessionID, func(sess *session.SessionState) error {
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		sess.Metadata["last_redirect_success"] = true
		sess.Metadata["redirect_result"] = result
		sess.Metadata["redirect_from"] = workflowError.ToolName
		sess.Metadata["redirect_to"] = redirectTo
		return nil
	})

	ch.logger.Info("Redirect fix succeeded",
		"session_id", sessionID,
		"redirect_to", redirectTo)

	return true
}

func (ch *ConversationHandler) generateFallbackOptions(stage shared.ConversationStage, _ error, action *ErrorAction) []Option {
	var options []Option
	options = append(options, Option{
		ID:    "retry",
		Label: "Retry operation",
	})
	switch stage {
	case shared.StageBuild:
		options = append(options, Option{
			ID:    "logs",
			Label: "Show build logs",
		})
		options = append(options, Option{
			ID:    "modify",
			Label: "Modify Dockerfile",
		})

	case shared.StageDeployment:
		options = append(options, Option{
			ID:    "manifests",
			Label: "Review manifests",
		})
		options = append(options, Option{
			ID:    "rebuild",
			Label: "Rebuild image",
		})

	case shared.StageManifests:
		options = append(options, Option{
			ID:    "regenerate",
			Label: "Regenerate manifests",
		})
	}
	actionType := ""
	if action != nil {
		actionType = action.Type
		if action.Action != "" {
			actionType = action.Action
		}
	}
	if actionType != "fail" {
		options = append(options, Option{
			ID:    "skip",
			Label: "Skip this stage",
		})
	}

	return options
}
