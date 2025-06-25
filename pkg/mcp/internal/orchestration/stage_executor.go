package orchestration

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration/execution"
	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// DefaultStageExecutor implements StageExecutor for executing workflow stages
type DefaultStageExecutor struct {
	logger           zerolog.Logger
	toolRegistry     InternalToolRegistry
	toolOrchestrator InternalToolOrchestrator
	secretRedactor   *SecretRedactor

	// Execution strategies
	sequentialExecutor  execution.Executor
	parallelExecutor    execution.Executor
	conditionalExecutor map[string]execution.Executor // keyed by base executor type
}

// NewDefaultStageExecutor creates a new stage executor with modular execution strategies
func NewDefaultStageExecutor(
	logger zerolog.Logger,
	toolRegistry InternalToolRegistry,
	toolOrchestrator InternalToolOrchestrator,
) *DefaultStageExecutor {
	// Create base executors
	seqExec := execution.NewSequentialExecutor(logger)
	parExec := execution.NewParallelExecutor(logger, 10)

	// Create conditional wrappers
	condExecs := map[string]execution.Executor{
		"sequential": execution.NewConditionalExecutor(logger, seqExec),
		"parallel":   execution.NewConditionalExecutor(logger, parExec),
	}

	return &DefaultStageExecutor{
		logger:              logger.With().Str("component", "stage_executor").Logger(),
		toolRegistry:        toolRegistry,
		toolOrchestrator:    toolOrchestrator,
		secretRedactor:      NewSecretRedactor(),
		sequentialExecutor:  seqExec,
		parallelExecutor:    parExec,
		conditionalExecutor: condExecs,
	}
}

// ExecuteStage executes a workflow stage with its tools
func (se *DefaultStageExecutor) ExecuteStage(
	ctx context.Context,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
) (*workflow.StageResult, error) {
	se.logger.Info().
		Str("stage_name", stage.Name).
		Str("session_id", session.ID).
		Int("tool_count", len(stage.Tools)).
		Bool("parallel", stage.Parallel).
		Int("conditions", len(stage.Conditions)).
		Msg("Executing workflow stage")

	startTime := time.Now()

	// Apply stage timeout if specified
	stageCtx := ctx
	if stage.Timeout != nil {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, *stage.Timeout)
		defer cancel()
	}

	// Create tool execution function
	executeToolFunc := func(ctx context.Context, toolName string, stage *workflow.WorkflowStage, session *workflow.WorkflowSession) (interface{}, error) {
		return se.executeTool(ctx, toolName, stage, session)
	}

	// Select appropriate executor
	var executor execution.Executor

	if len(stage.Conditions) > 0 {
		// Use conditional executor
		if stage.Parallel && len(stage.Tools) > 1 {
			executor = se.conditionalExecutor["parallel"]
		} else {
			executor = se.conditionalExecutor["sequential"]
		}
	} else {
		// Use direct executor
		if stage.Parallel && len(stage.Tools) > 1 {
			executor = se.parallelExecutor
		} else {
			executor = se.sequentialExecutor
		}
	}

	// Execute using selected strategy
	execResult, err := executor.Execute(stageCtx, stage, session, stage.Tools, executeToolFunc)

	// Convert execution result to stage result
	stageResult := &workflow.StageResult{
		StageName: stage.Name,
		Success:   execResult.Success,
		Duration:  execResult.Duration,
		Results:   execResult.Results,
		Artifacts: execResult.Artifacts,
		Metrics:   execResult.Metrics,
	}

	if err != nil {
		stageResult.Error = &workflow.WorkflowError{
			ID:        fmt.Sprintf("%s_%s_%d", session.ID, stage.Name, time.Now().Unix()),
			StageName: stage.Name,
			ErrorType: "stage_execution_error",
			Message:   err.Error(),
			Timestamp: time.Now(),
			Severity:  "high",
			Retryable: true,
		}
	}

	// Add stage-level metrics
	if stageResult.Metrics == nil {
		stageResult.Metrics = make(map[string]interface{})
	}
	stageResult.Metrics["total_duration"] = time.Since(startTime).String()

	se.logger.Info().
		Str("stage_name", stage.Name).
		Str("session_id", session.ID).
		Bool("success", stageResult.Success).
		Dur("duration", stageResult.Duration).
		Msg("Stage execution completed")

	return stageResult, err
}

// ValidateStage validates a workflow stage configuration
func (se *DefaultStageExecutor) ValidateStage(stage *workflow.WorkflowStage) error {
	validator := NewStageValidator(se.toolRegistry)
	return validator.Validate(stage)
}

// executeTool executes a single tool (internal method)
func (se *DefaultStageExecutor) executeTool(
	ctx context.Context,
	toolName string,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
) (interface{}, error) {
	// Prepare tool arguments
	args := se.prepareToolArgs(toolName, stage, session)

	// Redact secrets from args before logging
	redactedArgs := se.secretRedactor.RedactMap(args)
	se.logger.Debug().
		Str("tool_name", toolName).
		Interface("args", redactedArgs).
		Msg("Executing tool with arguments")

	// Execute tool through orchestrator
	result, err := se.toolOrchestrator.ExecuteTool(ctx, toolName, args, session)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Update session with tool results
	if session.StageResults == nil {
		session.StageResults = make(map[string]interface{})
	}
	session.StageResults[toolName] = result

	return result, nil
}

// prepareToolArgs prepares arguments for tool execution
func (se *DefaultStageExecutor) prepareToolArgs(
	toolName string,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
) map[string]interface{} {
	args := make(map[string]interface{})

	// Add stage variables with enhanced expansion
	for k, v := range stage.Variables {
		args[k] = se.expandVariableEnhanced(v, session, stage)
	}

	// Add session context
	args["session_id"] = session.ID
	args["workflow_id"] = session.WorkflowID
	args["stage_name"] = stage.Name

	// Add shared context values
	for k, v := range session.SharedContext {
		// Prefix with context_ to avoid conflicts
		args["context_"+k] = v
	}

	return args
}

// expandVariableEnhanced expands variables with enhanced ${var} syntax support
func (se *DefaultStageExecutor) expandVariableEnhanced(value string, session *workflow.WorkflowSession, stage *workflow.WorkflowStage) string {
	resolver := workflow.NewVariableResolver(se.logger)

	// Build variable context (without workflow vars since we don't have access to workflowSpec here)
	context := &workflow.VariableContext{
		WorkflowVars:    make(map[string]string), // Will be empty, could be populated from session if needed
		StageVars:       stage.Variables,
		SessionContext:  session.SharedContext,
		EnvironmentVars: make(map[string]string),
		Secrets:         make(map[string]string),
	}

	// Populate environment variables (with common container/k8s prefixes)
	for _, prefix := range []string{"CONTAINER_", "K8S_", "KUBERNETES_", "DOCKER_", "CI_", "BUILD_"} {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, prefix) {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					context.EnvironmentVars[parts[0]] = parts[1]
				}
			}
		}
	}

	// Check if workflow variables are stored in session context
	if workflowVars, exists := session.SharedContext["_workflow_variables"]; exists {
		if varsMap, ok := workflowVars.(map[string]string); ok {
			context.WorkflowVars = varsMap
		}
	}

	// Expand variables
	expanded, err := resolver.ResolveVariables(value, context)
	if err != nil {
		se.logger.Warn().Err(err).Str("value", value).Msg("Failed to expand variables")
		return value // Return original on error
	}

	return expanded
}

// expandVariable expands variables with session context (legacy method - kept for compatibility)
func (se *DefaultStageExecutor) expandVariable(value string, session *workflow.WorkflowSession) string {
	// Simple variable expansion - replace ${var} with session context values
	expanded := value
	for k, v := range session.SharedContext {
		placeholder := fmt.Sprintf("${%s}", k)
		expanded = strings.ReplaceAll(expanded, placeholder, fmt.Sprintf("%v", v))
	}
	return expanded
}

// SecretRedactor handles secret redaction from logs
type SecretRedactor struct {
	patterns []*regexp.Regexp
}

// NewSecretRedactor creates a new secret redactor
func NewSecretRedactor() *SecretRedactor {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pwd|secret|key|token|auth|credential)["\s]*[:=]["\s]*([^"\s,}]+)`),
		regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-\._~\+\/]+=*`),
		regexp.MustCompile(`[A-Za-z0-9]{20,}`), // Long random strings
	}

	// Add environment variable patterns
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && strings.Contains(strings.ToLower(parts[0]), "secret") {
			patterns = append(patterns, regexp.MustCompile(regexp.QuoteMeta(parts[1])))
		}
	}

	return &SecretRedactor{patterns: patterns}
}

// RedactMap redacts secrets from a map
func (sr *SecretRedactor) RedactMap(data map[string]interface{}) map[string]interface{} {
	redacted := make(map[string]interface{})
	for k, v := range data {
		if sr.isSecretKey(k) {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = sr.redactValue(v)
		}
	}
	return redacted
}

// isSecretKey checks if a key name suggests it contains a secret
func (sr *SecretRedactor) isSecretKey(key string) bool {
	lowerKey := strings.ToLower(key)
	secretKeywords := []string{"password", "secret", "token", "key", "auth", "credential", "passwd", "pwd"}
	for _, keyword := range secretKeywords {
		if strings.Contains(lowerKey, keyword) {
			return true
		}
	}
	return false
}

// redactValue redacts secrets from a value
func (sr *SecretRedactor) redactValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return sr.redactString(v)
	case map[string]interface{}:
		return sr.RedactMap(v)
	default:
		return value
	}
}

// redactString redacts secrets from a string
func (sr *SecretRedactor) redactString(s string) string {
	for _, pattern := range sr.patterns {
		s = pattern.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
