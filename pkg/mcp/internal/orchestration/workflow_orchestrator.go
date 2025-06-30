package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// WorkflowOrchestrator manages workflow execution and coordination
type WorkflowOrchestrator struct {
	logger            zerolog.Logger
	sessionManager    interface{}              // Session manager
	toolRegistry      interface{}              // Tool registry
	contextSharer     interface{}              // Context sharer
	executionSessions map[string]*ExecutionSession // Active execution sessions
	mutex             sync.RWMutex
	engine            *Engine                  // Workflow engine
	workflowSpecs     map[string]*WorkflowSpec // Registered workflow specs
}

// NewWorkflowOrchestrator creates a new workflow orchestrator
// Accepts db, registryAdapter, toolOrchestrator, logger as parameters
func NewWorkflowOrchestrator(deps ...interface{}) *WorkflowOrchestrator {
	var logger zerolog.Logger
	// Extract logger from the last parameter (expected to be logger)
	if len(deps) > 0 {
		if l, ok := deps[len(deps)-1].(zerolog.Logger); ok {
			logger = l
		} else {
			logger = zerolog.Nop()
		}
	} else {
		logger = zerolog.Nop()
	}

	return &WorkflowOrchestrator{
		logger:            logger.With().Str("component", "workflow_orchestrator").Logger(),
		executionSessions: make(map[string]*ExecutionSession),
		engine:            NewEngine(),
		workflowSpecs:     make(map[string]*WorkflowSpec),
	}
}

// ExecuteWorkflow executes a workflow with variadic options
func (wo *WorkflowOrchestrator) ExecuteWorkflow(ctx context.Context, workflowID string, options ...ExecutionOption) (interface{}, error) {
	startTime := time.Now()
	
	// Get or create workflow specification
	spec, exists := wo.workflowSpecs[workflowID]
	if !exists {
		spec = wo.createDefaultWorkflowSpec(workflowID)
		wo.workflowSpecs[workflowID] = spec
	}
	
	// Merge execution options
	mergedOptions := wo.mergeExecutionOptions(options...)
	
	// Create execution session
	session := &ExecutionSession{
		SessionID:        wo.generateSessionID(),
		ID:               wo.generateSessionID(), // Legacy compatibility
		WorkflowID:       workflowID,
		WorkflowName:     spec.Name,
		Variables:        mergedOptions.Variables,
		Context:          make(map[string]interface{}),
		StartTime:        startTime,
		Status:           WorkflowStatusRunning,
		CurrentStage:     "",
		CompletedStages:  []string{},
		FailedStages:     []string{},
		SkippedStages:    []string{},
		SharedContext:    make(map[string]interface{}),
		ResourceBindings: make(map[string]interface{}),
		LastActivity:     startTime,
		StageResults:     make(map[string]interface{}),
		CreatedAt:        startTime,
		UpdatedAt:        startTime,
		Checkpoints:      []WorkflowCheckpoint{},
		ErrorContext:     make(map[string]interface{}),
		WorkflowVersion:  spec.Version,
		Labels:           make(map[string]string),
	}
	
	// Store session for tracking
	wo.mutex.Lock()
	wo.executionSessions[session.SessionID] = session
	wo.mutex.Unlock()
	
	wo.logger.Info().
		Str("session_id", session.SessionID).
		Str("workflow_id", workflowID).
		Str("workflow_name", spec.Name).
		Msg("Starting workflow execution")
	
	// Execute workflow using engine
	_, err := wo.executeWorkflowStages(ctx, spec, session, mergedOptions)
	
	// Update session status
	endTime := time.Now()
	session.EndTime = &endTime
	session.UpdatedAt = endTime
	
	if err != nil {
		session.Status = WorkflowStatusFailed
		session.ErrorContext["execution_error"] = err.Error()
		wo.logger.Error().
			Err(err).
			Str("session_id", session.SessionID).
			Str("workflow_id", workflowID).
			Msg("Workflow execution failed")
	} else {
		session.Status = WorkflowStatusCompleted
		wo.logger.Info().
			Str("session_id", session.SessionID).
			Str("workflow_id", workflowID).
			Dur("duration", endTime.Sub(startTime)).
			Msg("Workflow execution completed successfully")
	}
	
	// Return comprehensive result
	return &WorkflowResult{
		Success:         err == nil,
		Results:         session.StageResults,
		Error:           wo.formatWorkflowError(err),
		Duration:        endTime.Sub(startTime),
		Artifacts:       wo.extractArtifactsFromSession(session),
		SessionID:       session.SessionID,
		StagesCompleted: len(session.CompletedStages),
	}, err
}

// ExecuteCustomWorkflow executes a custom workflow specification
func (wo *WorkflowOrchestrator) ExecuteCustomWorkflow(ctx context.Context, spec *WorkflowSpec) (interface{}, error) {
	if spec == nil {
		return nil, fmt.Errorf("workflow specification cannot be nil")
	}
	
	// Validate workflow specification
	if err := wo.validateWorkflowSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid workflow specification: %w", err)
	}
	
	// Register the custom workflow spec temporarily
	wo.mutex.Lock()
	wo.workflowSpecs[spec.ID] = spec
	wo.mutex.Unlock()
	
	// Execute workflow using standard execution path
	options := []ExecutionOption{}
	if spec.Variables != nil {
		options = append(options, WithVariables(spec.Variables))
	}
	
	result, err := wo.ExecuteWorkflow(ctx, spec.ID, options...)
	if err != nil {
		wo.logger.Error().
			Err(err).
			Str("workflow_id", spec.ID).
			Str("workflow_name", spec.Name).
			Msg("Custom workflow execution failed")
		return nil, err
	}
	
	wo.logger.Info().
		Str("workflow_id", spec.ID).
		Str("workflow_name", spec.Name).
		Msg("Custom workflow execution completed successfully")
	
	return result, nil
}

// GetWorkflowStatus gets the status of a workflow
func (wo *WorkflowOrchestrator) GetWorkflowStatus(workflowID string) (string, error) {
	wo.mutex.RLock()
	defer wo.mutex.RUnlock()
	
	// Find the session associated with this workflow
	for _, session := range wo.executionSessions {
		if session.WorkflowID == workflowID {
			return session.Status, nil
		}
	}
	
	return "not_found", fmt.Errorf("no active session found for workflow ID: %s", workflowID)
}

// ListAvailableWorkflows returns available workflows
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

// Helper methods for WorkflowOrchestrator

// generateSessionID generates a unique session ID
func (wo *WorkflowOrchestrator) generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// mergeExecutionOptions merges multiple execution options
func (wo *WorkflowOrchestrator) mergeExecutionOptions(options ...ExecutionOption) ExecutionOption {
	merged := ExecutionOption{
		Variables: make(map[string]interface{}),
	}
	
	for _, opt := range options {
		if opt.Parallel {
			merged.Parallel = true
		}
		if opt.MaxRetries > merged.MaxRetries {
			merged.MaxRetries = opt.MaxRetries
		}
		if opt.Timeout > merged.Timeout {
			merged.Timeout = opt.Timeout
		}
		// Merge variables
		for k, v := range opt.Variables {
			merged.Variables[k] = v
		}
	}
	
	return merged
}

// createDefaultWorkflowSpec creates a default workflow specification
func (wo *WorkflowOrchestrator) createDefaultWorkflowSpec(workflowID string) *WorkflowSpec {
	switch workflowID {
	case "analyze_and_build":
		return &WorkflowSpec{
			ID:      workflowID,
			Name:    "Analyze and Build Application",
			Version: "1.0.0",
			Stages: []ExecutionStage{
				{
					ID:       "analyze",
					Name:     "Analyze Repository",
					Type:     "analysis",
					Tools:    []string{"analyze_repository"},
					DependsOn: []string{},
					Parallel: false,
				},
				{
					ID:       "build",
					Name:     "Build Container Image",
					Type:     "build",
					Tools:    []string{"build_image"},
					DependsOn: []string{"analyze"},
					Parallel: false,
				},
			},
			Variables: make(map[string]interface{}),
		}
	case "deploy_application":
		return &WorkflowSpec{
			ID:      workflowID,
			Name:    "Deploy Application",
			Version: "1.0.0",
			Stages: []ExecutionStage{
				{
					ID:       "generate_manifests",
					Name:     "Generate Kubernetes Manifests", 
					Type:     "deployment",
					Tools:    []string{"generate_manifests"},
					DependsOn: []string{},
					Parallel: false,
				},
				{
					ID:       "deploy",
					Name:     "Deploy to Kubernetes",
					Type:     "deployment",
					Tools:    []string{"deploy_kubernetes"},
					DependsOn: []string{"generate_manifests"},
					Parallel: false,
				},
			},
			Variables: make(map[string]interface{}),
		}
	case "scan_and_fix":
		return &WorkflowSpec{
			ID:      workflowID,
			Name:    "Security Scan and Fix",
			Version: "1.0.0", 
			Stages: []ExecutionStage{
				{
					ID:       "scan",
					Name:     "Security Scan",
					Type:     "security",
					Tools:    []string{"scan_security"},
					DependsOn: []string{},
					Parallel: false,
				},
			},
			Variables: make(map[string]interface{}),
		}
	default:
		return &WorkflowSpec{
			ID:        workflowID,
			Name:      fmt.Sprintf("Custom Workflow: %s", workflowID),
			Version:   "1.0.0",
			Stages:    []ExecutionStage{},
			Variables: make(map[string]interface{}),
		}
	}
}

// validateWorkflowSpec validates a workflow specification
func (wo *WorkflowOrchestrator) validateWorkflowSpec(spec *WorkflowSpec) error {
	if spec.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}
	if spec.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(spec.Stages) == 0 {
		return fmt.Errorf("workflow must have at least one stage")
	}
	
	// Validate stage dependencies
	stageMap := make(map[string]bool)
	for _, stage := range spec.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage ID is required")
		}
		stageMap[stage.ID] = true
	}
	
	for _, stage := range spec.Stages {
		for _, dep := range stage.DependsOn {
			if !stageMap[dep] {
				return fmt.Errorf("stage %s depends on non-existent stage %s", stage.ID, dep)
			}
		}
	}
	
	// Check for circular dependencies
	if wo.hasCircularDependencies(spec.Stages) {
		return fmt.Errorf("workflow has circular dependencies")
	}
	
	return nil
}

// hasCircularDependencies checks for circular dependencies in workflow stages
func (wo *WorkflowOrchestrator) hasCircularDependencies(stages []ExecutionStage) bool {
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	
	// Create adjacency map
	dependencies := make(map[string][]string)
	for _, stage := range stages {
		dependencies[stage.ID] = stage.DependsOn
	}
	
	// Check each stage for cycles
	for _, stage := range stages {
		if !visited[stage.ID] {
			if wo.hasCycleDFS(stage.ID, dependencies, visited, recursionStack) {
				return true
			}
		}
	}
	
	return false
}

// hasCycleDFS performs depth-first search to detect cycles
func (wo *WorkflowOrchestrator) hasCycleDFS(stageID string, dependencies map[string][]string, visited map[string]bool, recursionStack map[string]bool) bool {
	visited[stageID] = true
	recursionStack[stageID] = true
	
	for _, dep := range dependencies[stageID] {
		if !visited[dep] {
			if wo.hasCycleDFS(dep, dependencies, visited, recursionStack) {
				return true
			}
		} else if recursionStack[dep] {
			return true
		}
	}
	
	recursionStack[stageID] = false
	return false
}

// executeWorkflowStages executes the stages of a workflow
func (wo *WorkflowOrchestrator) executeWorkflowStages(ctx context.Context, spec *WorkflowSpec, session *ExecutionSession, options ExecutionOption) (interface{}, error) {
	// Create stage execution order based on dependencies
	executionOrder, err := wo.createExecutionOrder(spec.Stages)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution order: %w", err)
	}
	
	wo.logger.Info().
		Str("session_id", session.SessionID).
		Int("total_stages", len(executionOrder)).
		Msg("Starting stage execution")
	
	// Execute stages in order
	for i, stageID := range executionOrder {
		stage := wo.findStageByID(spec.Stages, stageID)
		if stage == nil {
			return nil, fmt.Errorf("stage not found: %s", stageID)
		}
		
		session.CurrentStage = stageID
		session.LastActivity = time.Now()
		
		wo.logger.Info().
			Str("session_id", session.SessionID).
			Str("stage_id", stageID).
			Str("stage_name", stage.Name).
			Int("stage_index", i+1).
			Int("total_stages", len(executionOrder)).
			Msg("Executing stage")
		
		// Execute stage
		stageResult, err := wo.executeStage(ctx, stage, session, options)
		if err != nil {
			session.FailedStages = append(session.FailedStages, stageID)
			session.ErrorContext[fmt.Sprintf("stage_%s_error", stageID)] = err.Error()
			return nil, fmt.Errorf("stage %s failed: %w", stageID, err)
		}
		
		// Store stage result
		session.StageResults[stageID] = stageResult
		session.CompletedStages = append(session.CompletedStages, stageID)
		
		wo.logger.Info().
			Str("session_id", session.SessionID).
			Str("stage_id", stageID).
			Msg("Stage completed successfully")
	}
	
	return session.StageResults, nil
}

// createExecutionOrder creates the execution order based on stage dependencies
func (wo *WorkflowOrchestrator) createExecutionOrder(stages []ExecutionStage) ([]string, error) {
	// Simple topological sort implementation
	inDegree := make(map[string]int)
	dependencies := make(map[string][]string)
	stageMap := make(map[string]*ExecutionStage)
	
	// Initialize
	for _, stage := range stages {
		stageMap[stage.ID] = &stage
		inDegree[stage.ID] = 0
		dependencies[stage.ID] = []string{}
	}
	
	// Build dependency graph
	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			dependencies[dep] = append(dependencies[dep], stage.ID)
			inDegree[stage.ID]++
		}
	}
	
	// Topological sort
	var queue []string
	for stageID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, stageID)
		}
	}
	
	var result []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		for _, dependent := range dependencies[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	
	if len(result) != len(stages) {
		return nil, fmt.Errorf("circular dependency detected in workflow stages")
	}
	
	return result, nil
}

// findStageByID finds a stage by its ID
func (wo *WorkflowOrchestrator) findStageByID(stages []ExecutionStage, stageID string) *ExecutionStage {
	for _, stage := range stages {
		if stage.ID == stageID {
			return &stage
		}
	}
	return nil
}

// executeStage executes a single workflow stage
func (wo *WorkflowOrchestrator) executeStage(ctx context.Context, stage *ExecutionStage, session *ExecutionSession, options ExecutionOption) (interface{}, error) {
	// Create stage timeout context
	stageCtx := ctx
	if stage.Timeout != nil {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, *stage.Timeout)
		defer cancel()
	}
	
	// Execute tools in stage
	stageResults := make(map[string]interface{})
	
	if stage.Parallel && len(stage.Tools) > 1 {
		// Execute tools in parallel
		type toolResult struct {
			tool   string
			result interface{}
			err    error
		}
		
		resultChan := make(chan toolResult, len(stage.Tools))
		
		for _, toolName := range stage.Tools {
			go func(tool string) {
				result, err := wo.executeTool(stageCtx, tool, stage, session)
				resultChan <- toolResult{tool: tool, result: result, err: err}
			}(toolName)
		}
		
		// Collect results
		for i := 0; i < len(stage.Tools); i++ {
			result := <-resultChan
			if result.err != nil {
				return nil, fmt.Errorf("tool %s failed: %w", result.tool, result.err)
			}
			stageResults[result.tool] = result.result
		}
	} else {
		// Execute tools sequentially
		for _, toolName := range stage.Tools {
			result, err := wo.executeTool(stageCtx, toolName, stage, session)
			if err != nil {
				return nil, fmt.Errorf("tool %s failed: %w", toolName, err)
			}
			stageResults[toolName] = result
		}
	}
	
	return stageResults, nil
}

// executeTool executes a single tool (placeholder implementation)
func (wo *WorkflowOrchestrator) executeTool(ctx context.Context, toolName string, stage *ExecutionStage, session *ExecutionSession) (interface{}, error) {
	wo.logger.Debug().
		Str("session_id", session.SessionID).
		Str("stage_id", stage.ID).
		Str("tool_name", toolName).
		Msg("Executing tool")
	
	// Simulate tool execution
	time.Sleep(100 * time.Millisecond)
	
	// Return mock result
	return map[string]interface{}{
		"tool":       toolName,
		"stage":      stage.ID,
		"session":    session.SessionID,
		"success":    true,
		"timestamp":  time.Now(),
	}, nil
}

// formatWorkflowError formats an error into a WorkflowError structure
func (wo *WorkflowOrchestrator) formatWorkflowError(err error) *WorkflowError {
	if err == nil {
		return nil
	}
	
	return &WorkflowError{
		ID:        fmt.Sprintf("error_%d", time.Now().UnixNano()),
		Message:   err.Error(),
		Code:      "EXECUTION_FAILED",
		Type:      "workflow_execution_error",
		ErrorType: "execution",
		Severity:  "high",
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// extractArtifactsFromSession extracts artifacts from execution session
func (wo *WorkflowOrchestrator) extractArtifactsFromSession(session *ExecutionSession) []ExecutionArtifact {
	var artifacts []ExecutionArtifact
	
	// Extract artifacts from stage results
	for stageID, result := range session.StageResults {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if artifactsData, exists := resultMap["artifacts"]; exists {
				if artifactList, ok := artifactsData.([]ExecutionArtifact); ok {
					artifacts = append(artifacts, artifactList...)
				}
			}
		}
		
		// Create default artifact for each stage
		artifacts = append(artifacts, ExecutionArtifact{
			ID:        fmt.Sprintf("%s_%s_result", session.SessionID, stageID),
			Name:      fmt.Sprintf("Stage %s Result", stageID),
			Type:      "stage_result",
			Path:      fmt.Sprintf("/tmp/%s/%s.json", session.SessionID, stageID),
			Size:      0,
			Metadata:  map[string]interface{}{"stage_id": stageID},
			CreatedAt: time.Now(),
		})
	}
	
	return artifacts
}
