package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// AIContextEnhancer provides rich context for AI-driven decisions across tools
type AIContextEnhancer struct {
	contextSharer *DefaultContextSharer
	logger        *slog.Logger
}

// NewAIContextEnhancer creates a new AI context enhancer
func NewAIContextEnhancer(contextSharer *DefaultContextSharer, logger *slog.Logger) *AIContextEnhancer {
	return &AIContextEnhancer{
		contextSharer: contextSharer,
		logger:        logger.With("component", "ai_context_enhancer"),
	}
}

// AIToolContext represents comprehensive context for AI decision making
type AIToolContext struct {
	// Tool execution context
	ToolName      string        `json:"tool_name"`
	OperationType string        `json:"operation_type"`
	StartTime     time.Time     `json:"start_time"`
	Duration      time.Duration `json:"duration,omitempty"`
	Success       bool          `json:"success"`
	// Input and output analysis
	InputAnalysis  *InputAnalysis  `json:"input_analysis"`
	OutputAnalysis *OutputAnalysis `json:"output_analysis"`
	// Error context
	ConsolidatedErrorContext *ErrorContextInfo `json:"error_context,omitempty"`
	// Performance insights
	PerformanceMetrics *AIPerformanceMetrics `json:"performance_metrics"`
	// Resource usage
	ResourceUsage *ResourceUsageInfo `json:"resource_usage"`
	// Workflow context
	WorkflowPosition *WorkflowPosition `json:"workflow_position"`
	// Previous attempts
	PreviousAttempts []AttemptSummary `json:"previous_attempts"`
	// AI recommendations
	AIRecommendations *AIRecommendations `json:"ai_recommendations"`
}

// InputAnalysis provides insights about tool inputs
type InputAnalysis struct {
	InputComplexity   string                 `json:"input_complexity"` // low, medium, high
	KeyParameters     []string               `json:"key_parameters"`
	ValidationResults []AIValidationResult   `json:"validation_results"`
	PatternMatches    []AIPatternMatch       `json:"pattern_matches"`
	RiskFactors       []string               `json:"risk_factors"`
	InputMetadata     map[string]interface{} `json:"input_metadata"`
}

// OutputAnalysis provides insights about tool outputs
type OutputAnalysis struct {
	OutputQuality    string                 `json:"output_quality"`    // excellent, good, fair, poor
	CompletionStatus string                 `json:"completion_status"` // complete, partial, failed
	KeyArtifacts     []string               `json:"key_artifacts"`
	QualityMetrics   []AIQualityMetric      `json:"quality_metrics"`
	ImprovementAreas []string               `json:"improvement_areas"`
	OutputMetadata   map[string]interface{} `json:"output_metadata"`
}

// ErrorContextInfo provides rich error context for AI analysis
type ErrorContextInfo struct {
	ErrorType         string                 `json:"error_type"`
	ErrorSeverity     string                 `json:"error_severity"`
	RootCauseAnalysis []string               `json:"root_cause_analysis"`
	SimilarErrors     []AISimilarError       `json:"similar_errors"`
	FixProbability    string                 `json:"fix_probability"` // high, medium, low
	FixComplexity     string                 `json:"fix_complexity"`  // simple, moderate, complex
	EscalationNeeded  bool                   `json:"escalation_needed"`
	ErrorMetadata     map[string]interface{} `json:"error_metadata"`
}

// AIPerformanceMetrics tracks tool performance for AI optimization
type AIPerformanceMetrics struct {
	ExecutionTime     time.Duration        `json:"execution_time"`
	MemoryUsed        int64                `json:"memory_used"`
	CacheHitRatio     float64              `json:"cache_hit_ratio"`
	ThroughputMBps    float64              `json:"throughput_mbps"`
	OptimizationScore float64              `json:"optimization_score"`
	Bottlenecks       []AIPerformanceIssue `json:"bottlenecks"`
}

// ResourceUsageInfo tracks resource consumption patterns
type ResourceUsageInfo struct {
	CPUUsage           float64        `json:"cpu_usage"`
	MemoryUsage        int64          `json:"memory_usage"`
	DiskIOBytes        int64          `json:"disk_io_bytes"`
	NetworkIOBytes     int64          `json:"network_io_bytes"`
	FileSystemChanges  []AIFileChange `json:"filesystem_changes"`
	EnvironmentChanges []AIEnvChange  `json:"environment_changes"`
}

// WorkflowPosition describes where this tool fits in the overall workflow
type WorkflowPosition struct {
	CurrentStage     string   `json:"current_stage"`
	PreviousStages   []string `json:"previous_stages"`
	NextStages       []string `json:"next_stages"`
	CriticalPath     bool     `json:"critical_path"`
	Dependencies     []string `json:"dependencies"`
	DependentTools   []string `json:"dependent_tools"`
	WorkflowProgress float64  `json:"workflow_progress"` // 0.0 to 1.0
}

// AttemptSummary summarizes previous attempts for learning
type AttemptSummary struct {
	AttemptNumber int                    `json:"attempt_number"`
	Strategy      string                 `json:"strategy"`
	Outcome       string                 `json:"outcome"`
	LessonLearned string                 `json:"lesson_learned"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// AIRecommendations provides AI-generated recommendations
type AIRecommendations struct {
	NextBestActions     []AIActionRecommendation `json:"next_best_actions"`
	OptimizationTips    []AIOptimizationTip      `json:"optimization_tips"`
	RiskMitigation      []AIRiskMitigation       `json:"risk_mitigation"`
	QualityImprovements []AIQualityImprovement   `json:"quality_improvements"`
	ContextualInsights  []AIContextualInsight    `json:"contextual_insights"`
}

// Supporting types for AI context (prefixed to avoid conflicts)
type AIValidationResult struct {
	Field   string `json:"field"`
	Status  string `json:"status"` // valid, invalid, warning
	Message string `json:"message"`
}
type AIPatternMatch struct {
	Pattern     string  `json:"pattern"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
}
type AIQualityMetric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type AISimilarError struct {
	ErrorMessage string    `json:"error_message"`
	Resolution   string    `json:"resolution"`
	Success      bool      `json:"success"`
	Timestamp    time.Time `json:"timestamp"`
}
type AIPerformanceIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}
type AIFileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"` // create, modify, delete
	Size      int64  `json:"size"`
}
type AIEnvChange struct {
	Variable string `json:"variable"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}
type AIActionRecommendation struct {
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
	Priority   int     `json:"priority"`
}
type AIOptimizationTip struct {
	Area        string `json:"area"`
	Suggestion  string `json:"suggestion"`
	ImpactLevel string `json:"impact_level"` // high, medium, low
}
type AIRiskMitigation struct {
	Risk       string `json:"risk"`
	Mitigation string `json:"mitigation"`
	Urgency    string `json:"urgency"` // immediate, soon, eventual
}
type AIQualityImprovement struct {
	Aspect       string  `json:"aspect"`
	CurrentScore float64 `json:"current_score"`
	TargetScore  float64 `json:"target_score"`
	Improvement  string  `json:"improvement"`
}
type AIContextualInsight struct {
	Category string `json:"category"`
	Insight  string `json:"insight"`
	Impact   string `json:"impact"`
}

// EnhanceContext enriches tool context with AI-relevant information
func (e *AIContextEnhancer) EnhanceContext(
	ctx context.Context,
	config mcptypes.AIContextEnhanceConfig,
) (*AIToolContext, error) {
	e.logger.Info("Enhancing context for AI analysis",
		"session_id", config.SessionID,
		"tool_name", config.ToolName,
		"operation_type", config.OperationType)
	aiContext := &AIToolContext{
		ToolName:      config.ToolName,
		OperationType: config.OperationType,
		StartTime:     time.Now(),
		Success:       config.ToolError == nil,
	}
	// Analyze inputs if available
	if inputData := e.extractInputData(ctx); inputData != nil {
		aiContext.InputAnalysis = e.analyzeInput(inputData)
	}
	// Analyze outputs if available
	if config.ToolResult != nil {
		aiContext.OutputAnalysis = e.analyzeOutput(config.ToolResult)
	}
	// Analyze errors if present
	if config.ToolError != nil {
		aiContext.ConsolidatedErrorContext = e.analyzeError(config.ToolError, config.SessionID)
	}
	// Add performance metrics
	aiContext.PerformanceMetrics = e.gatherPerformanceMetrics(config.ToolName)
	// Add resource usage
	aiContext.ResourceUsage = e.gatherResourceUsage()
	// Determine workflow position
	aiContext.WorkflowPosition = e.analyzeWorkflowPosition(ctx, config.SessionID, config.ToolName)
	// Get previous attempts
	aiContext.PreviousAttempts = e.getPreviousAttempts(config.SessionID, config.ToolName)
	// Generate AI recommendations
	aiContext.AIRecommendations = e.generateRecommendations(aiContext)
	// Share the enhanced context
	contextType := fmt.Sprintf("ai_context_%s", config.ToolName)
	if err := e.contextSharer.ShareContext(ctx, config.SessionID, contextType, aiContext); err != nil {
		e.logger.Warn("Failed to share enhanced AI context", "error", err)
	}
	return aiContext, nil
}

// analyzeInput provides intelligent analysis of tool inputs
func (e *AIContextEnhancer) analyzeInput(inputData interface{}) *InputAnalysis {
	analysis := &InputAnalysis{
		KeyParameters:     []string{},
		ValidationResults: []AIValidationResult{},
		PatternMatches:    []AIPatternMatch{},
		RiskFactors:       []string{},
		InputMetadata:     make(map[string]interface{}),
	}
	// Analyze input complexity
	if inputMap, ok := inputData.(map[string]interface{}); ok {
		paramCount := len(inputMap)
		if paramCount <= 3 {
			analysis.InputComplexity = "low"
		} else if paramCount <= 8 {
			analysis.InputComplexity = "medium"
		} else {
			analysis.InputComplexity = "high"
		}
		// Extract key parameters
		for key := range inputMap {
			analysis.KeyParameters = append(analysis.KeyParameters, key)
		}
		// Identify common patterns
		if _, hasImage := inputMap["image_ref"]; hasImage {
			analysis.PatternMatches = append(analysis.PatternMatches, AIPatternMatch{
				Pattern:     "container_deployment",
				Confidence:  0.9,
				Description: "Container image deployment pattern detected",
			})
		}
		if _, hasNamespace := inputMap["namespace"]; hasNamespace {
			analysis.PatternMatches = append(analysis.PatternMatches, AIPatternMatch{
				Pattern:     "kubernetes_deployment",
				Confidence:  0.85,
				Description: "Kubernetes deployment pattern detected",
			})
		}
	}
	return analysis
}

// analyzeOutput provides intelligent analysis of tool outputs
func (e *AIContextEnhancer) analyzeOutput(outputData interface{}) *OutputAnalysis {
	analysis := &OutputAnalysis{
		KeyArtifacts:     []string{},
		QualityMetrics:   []AIQualityMetric{},
		ImprovementAreas: []string{},
		OutputMetadata:   make(map[string]interface{}),
	}
	// Analyze output structure and quality
	if outputMap, ok := outputData.(map[string]interface{}); ok {
		// Check for success indicators
		if success, ok := outputMap["success"].(bool); ok && success {
			analysis.OutputQuality = "good"
			analysis.CompletionStatus = "complete"
		} else {
			analysis.OutputQuality = "poor"
			analysis.CompletionStatus = "failed"
		}
		// Extract artifacts
		if _, ok := outputMap["manifests"]; ok {
			analysis.KeyArtifacts = append(analysis.KeyArtifacts, "kubernetes_manifests")
		}
		if imageRef, ok := outputMap["image_ref"].(string); ok && imageRef != "" {
			analysis.KeyArtifacts = append(analysis.KeyArtifacts, "docker_image")
		}
		// Quality metrics
		if duration, ok := outputMap["duration"].(time.Duration); ok {
			analysis.QualityMetrics = append(analysis.QualityMetrics, AIQualityMetric{
				Name:  "execution_time",
				Value: duration.Seconds(),
				Unit:  "seconds",
			})
		}
	}
	return analysis
}

// analyzeError provides comprehensive error analysis
func (e *AIContextEnhancer) analyzeError(err error, _ string) *ErrorContextInfo {
	errorInfo := &ErrorContextInfo{
		RootCauseAnalysis: []string{},
		SimilarErrors:     []AISimilarError{},
		ErrorMetadata:     make(map[string]interface{}),
	}
	// Analyze error type and severity
	errorInfo.ErrorType = "build_error"
	errorInfo.ErrorSeverity = "medium"
	// Determine fix probability based on error type
	switch errorInfo.ErrorType {
	case "network_error", "timeout_error":
		errorInfo.FixProbability = "high"
		errorInfo.FixComplexity = "simple"
	case "authentication_error", "permission_error":
		errorInfo.FixProbability = "medium"
		errorInfo.FixComplexity = "moderate"
	case "compilation_error", "syntax_error":
		errorInfo.FixProbability = "high"
		errorInfo.FixComplexity = "moderate"
	default:
		errorInfo.FixProbability = "medium"
		errorInfo.FixComplexity = "moderate"
	}
	// Root cause analysis
	errorMsg := err.Error()
	errorInfo.RootCauseAnalysis = e.performRootCauseAnalysis(errorMsg)
	return errorInfo
}

// performRootCauseAnalysis analyzes error messages for root causes
func (e *AIContextEnhancer) performRootCauseAnalysis(errorMsg string) []string {
	causes := []string{}
	// Common patterns
	patterns := map[string]string{
		"connection refused": "network_connectivity_issue",
		"timeout":            "operation_timeout",
		"permission denied":  "insufficient_permissions",
		"not found":          "missing_resource",
		"syntax error":       "configuration_syntax_error",
		"image pull":         "image_accessibility_issue",
		"resource quota":     "resource_limits_exceeded",
	}
	for pattern, cause := range patterns {
		if strings.Contains(errorMsg, pattern) {
			causes = append(causes, cause)
		}
	}
	if len(causes) == 0 {
		causes = append(causes, "unknown_root_cause")
	}
	return causes
}

// gatherPerformanceMetrics collects performance data
func (e *AIContextEnhancer) gatherPerformanceMetrics(_ string) *AIPerformanceMetrics {
	return &AIPerformanceMetrics{
		CacheHitRatio:     0.85, // Placeholder - would be real metrics
		ThroughputMBps:    10.5,
		OptimizationScore: 0.75,
		Bottlenecks:       []AIPerformanceIssue{},
	}
}

// gatherResourceUsage collects resource usage information
func (e *AIContextEnhancer) gatherResourceUsage() *ResourceUsageInfo {
	return &ResourceUsageInfo{
		CPUUsage:           25.5,
		MemoryUsage:        1024 * 1024 * 512, // 512MB
		FileSystemChanges:  []AIFileChange{},
		EnvironmentChanges: []AIEnvChange{},
	}
}

// analyzeWorkflowPosition determines the tool's position in the workflow
func (e *AIContextEnhancer) analyzeWorkflowPosition(_ context.Context, _ string, toolName string) *WorkflowPosition {
	position := &WorkflowPosition{
		PreviousStages: []string{},
		NextStages:     []string{},
		Dependencies:   []string{},
		DependentTools: []string{},
	}
	// Workflow mapping
	switch toolName {
	case "analyze_repository":
		position.CurrentStage = "analysis"
		position.NextStages = []string{"dockerfile_generation", "build"}
		position.WorkflowProgress = 0.1
	case "generate_dockerfile", "atomic_generate_dockerfile":
		position.CurrentStage = "dockerfile_generation"
		position.PreviousStages = []string{"analysis"}
		position.NextStages = []string{"build"}
		position.Dependencies = []string{"analyze_repository"}
		position.WorkflowProgress = 0.25
	case "build_image", "atomic_build_image":
		position.CurrentStage = "build"
		position.PreviousStages = []string{"dockerfile_generation"}
		position.NextStages = []string{"push", "manifest_generation"}
		position.Dependencies = []string{"generate_dockerfile"}
		position.WorkflowProgress = 0.5
	case "push_image", "atomic_push_image":
		position.CurrentStage = "push"
		position.PreviousStages = []string{"build"}
		position.NextStages = []string{"manifest_generation", "deployment"}
		position.Dependencies = []string{"build_image"}
		position.WorkflowProgress = 0.65
	case "generate_manifests":
		position.CurrentStage = "manifest_generation"
		position.PreviousStages = []string{"build", "push"}
		position.NextStages = []string{"deployment"}
		position.Dependencies = []string{"build_image"}
		position.WorkflowProgress = 0.8
	case "deploy_kubernetes", "atomic_deploy_kubernetes":
		position.CurrentStage = "deployment"
		position.PreviousStages = []string{"manifest_generation"}
		position.NextStages = []string{"validation", "completion"}
		position.Dependencies = []string{"generate_manifests"}
		position.WorkflowProgress = 0.95
	}
	return position
}

// getPreviousAttempts retrieves previous attempts for learning
func (e *AIContextEnhancer) getPreviousAttempts(_, _ string) []AttemptSummary {
	// This would integrate with session history
	return []AttemptSummary{}
}

// generateRecommendations creates AI-driven recommendations
func (e *AIContextEnhancer) generateRecommendations(aiContext *AIToolContext) *AIRecommendations {
	recommendations := &AIRecommendations{
		NextBestActions:     []AIActionRecommendation{},
		OptimizationTips:    []AIOptimizationTip{},
		RiskMitigation:      []AIRiskMitigation{},
		QualityImprovements: []AIQualityImprovement{},
		ContextualInsights:  []AIContextualInsight{},
	}
	// Generate recommendations based on context
	if aiContext.Success {
		recommendations.NextBestActions = append(recommendations.NextBestActions, AIActionRecommendation{
			Action:     "proceed_to_next_stage",
			Confidence: 0.9,
			Reasoning:  "Operation completed successfully",
			Priority:   1,
		})
	} else if aiContext.ConsolidatedErrorContext != nil {
		if aiContext.ConsolidatedErrorContext.FixProbability == "high" {
			recommendations.NextBestActions = append(recommendations.NextBestActions, AIActionRecommendation{
				Action:     "attempt_automatic_fix",
				Confidence: 0.8,
				Reasoning:  "Error has high fix probability",
				Priority:   1,
			})
		} else {
			recommendations.NextBestActions = append(recommendations.NextBestActions, AIActionRecommendation{
				Action:     "escalate_to_human",
				Confidence: 0.7,
				Reasoning:  "Error requires manual intervention",
				Priority:   2,
			})
		}
	}
	return recommendations
}

// extractInputData extracts input data from context
func (e *AIContextEnhancer) extractInputData(_ context.Context) interface{} {
	// This would extract input parameters from the context
	return nil
}

// GetEnhancedContext retrieves enhanced AI context for a tool
func (e *AIContextEnhancer) GetEnhancedContext(ctx context.Context, sessionID, toolName string) (*AIToolContext, error) {
	contextType := fmt.Sprintf("ai_context_%s", toolName)
	data, err := e.contextSharer.GetSharedContext(ctx, sessionID, contextType)
	if err != nil {
		return nil, err
	}
	if aiContext, ok := data.(*AIToolContext); ok {
		return aiContext, nil
	}
	return nil, errors.NewError().Messagef("invalid context type").Build()
}
