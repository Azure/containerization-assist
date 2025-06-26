package context

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/state"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// AIContextIntegration provides comprehensive AI context integration
type AIContextIntegration struct {
	aggregator     *AIContextAggregator
	stateManager   *state.UnifiedStateManager
	sessionManager *session.SessionManager
	knowledgeBase  *build.CrossToolKnowledgeBase
	toolFactory    *orchestration.EnhancedToolFactory
	logger         zerolog.Logger
}

// NewAIContextIntegration creates a new AI context integration
func NewAIContextIntegration(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	knowledgeBase *build.CrossToolKnowledgeBase,
	logger zerolog.Logger,
) *AIContextIntegration {
	// Create context aggregator
	aggregator := NewAIContextAggregator(stateManager, sessionManager, logger)

	// Register context providers
	aggregator.RegisterContextProvider("build", NewBuildContextProvider(
		stateManager, sessionManager, knowledgeBase, logger))
	aggregator.RegisterContextProvider("deployment", NewDeploymentContextProvider(
		stateManager, sessionManager, logger))
	aggregator.RegisterContextProvider("security", NewSecurityContextProvider(
		stateManager, sessionManager, logger))
	aggregator.RegisterContextProvider("performance", NewPerformanceContextProvider(
		stateManager, sessionManager, logger))
	aggregator.RegisterContextProvider("state", NewStateContextProvider(
		stateManager, logger))

	// Register context enrichers
	aggregator.RegisterContextEnricher(NewRelationshipEnricher(logger))
	aggregator.RegisterContextEnricher(NewInsightEnricher(knowledgeBase, logger))
	aggregator.RegisterContextEnricher(NewSecurityEnricher(sessionManager, logger))
	aggregator.RegisterContextEnricher(NewPerformanceEnricher(logger))

	return &AIContextIntegration{
		aggregator:     aggregator,
		stateManager:   stateManager,
		sessionManager: sessionManager,
		knowledgeBase:  knowledgeBase,
		logger:         logger.With().Str("component", "ai_context_integration").Logger(),
	}
}

// GetAggregator returns the context aggregator
func (i *AIContextIntegration) GetAggregator() *AIContextAggregator {
	return i.aggregator
}

// CreateAIAwareAnalyzer creates an analyzer with comprehensive AI context
func (i *AIContextIntegration) CreateAIAwareAnalyzer(baseAnalyzer mcptypes.AIAnalyzer) mcptypes.AIAnalyzer {
	return &AIAwareAnalyzer{
		baseAnalyzer: baseAnalyzer,
		integration:  i,
		logger:       i.logger,
	}
}

// AIAwareAnalyzer wraps an analyzer with comprehensive context
type AIAwareAnalyzer struct {
	baseAnalyzer mcptypes.AIAnalyzer
	integration  *AIContextIntegration
	logger       zerolog.Logger
}

// Analyze performs analysis with comprehensive context
func (a *AIAwareAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	// Get session ID from context
	sessionID := a.extractSessionID(ctx)

	// Get comprehensive context
	compContext, err := a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to get comprehensive context")
		// Continue with base analysis even if context fails
		return a.baseAnalyzer.Analyze(ctx, prompt)
	}

	// Enhance prompt with context
	enhancedPrompt := a.enhancePromptWithContext(prompt, compContext)

	// Perform analysis with enhanced context
	result, err := a.baseAnalyzer.Analyze(ctx, enhancedPrompt)
	if err != nil {
		return "", err
	}

	// Store analysis result in context for future reference
	a.storeAnalysisResult(ctx, sessionID, prompt, result, compContext)

	return result, nil
}

// AnalyzeWithFileTools performs analysis with file system access
func (a *AIAwareAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	// Get session ID from context
	sessionID := a.extractSessionID(ctx)

	// Get comprehensive context
	compContext, err := a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to get comprehensive context")
		// Continue with base analysis even if context fails
		return a.baseAnalyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
	}

	// Enhance prompt with context
	enhancedPrompt := a.enhancePromptWithContext(prompt, compContext)

	// Perform analysis with enhanced context
	result, err := a.baseAnalyzer.AnalyzeWithFileTools(ctx, enhancedPrompt, baseDir)
	if err != nil {
		return "", err
	}

	// Store analysis result in context for future reference
	a.storeAnalysisResult(ctx, sessionID, prompt, result, compContext)

	return result, nil
}

// AnalyzeWithFormat performs analysis with formatted prompts
func (a *AIAwareAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	formattedPrompt := fmt.Sprintf(promptTemplate, args...)
	return a.Analyze(ctx, formattedPrompt)
}

// GetTokenUsage returns usage statistics
func (a *AIAwareAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return a.baseAnalyzer.GetTokenUsage()
}

// ResetTokenUsage resets usage statistics
func (a *AIAwareAnalyzer) ResetTokenUsage() {
	a.baseAnalyzer.ResetTokenUsage()
}

// ErrorAnalysis represents an error analysis result
type ErrorAnalysis struct {
	PossibleCauses    []string
	Recommendations   []string
	AdditionalContext map[string]interface{}
}

// AnalyzeError analyzes an error with comprehensive context
func (a *AIAwareAnalyzer) AnalyzeError(ctx context.Context, err error, contextInfo map[string]interface{}) (*ErrorAnalysis, error) {
	// Extract session ID
	sessionID := ""
	if sid, ok := contextInfo["session_id"].(string); ok {
		sessionID = sid
	}

	// Get comprehensive context
	var compContext *ComprehensiveContext
	if sessionID != "" {
		compContext, _ = a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
	}

	// Add comprehensive context to contextInfo
	if compContext != nil {
		contextInfo["comprehensive_context"] = compContext
		contextInfo["recommendations"] = compContext.Recommendations
		contextInfo["predicted_issues"] = compContext.AnalysisInsights.PredictedIssues
	}

	// Create basic error analysis
	analysis := &ErrorAnalysis{
		PossibleCauses:    []string{err.Error()},
		Recommendations:   []string{},
		AdditionalContext: make(map[string]interface{}),
	}

	// Enhance analysis with context-based insights
	if compContext != nil {
		a.enhanceErrorAnalysis(analysis, compContext)
	}

	return analysis, nil
}

// extractSessionID extracts session ID from context
func (a *AIAwareAnalyzer) extractSessionID(ctx context.Context) string {
	// Try to get from context
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}

	return ""
}

// enhancePromptWithContext enhances the prompt with relevant context
func (a *AIAwareAnalyzer) enhancePromptWithContext(prompt string, context *ComprehensiveContext) string {
	// Build context summary
	contextSummary := fmt.Sprintf("\n\n--- Context Information ---\n")

	// Add recent events
	if len(context.RecentEvents) > 0 {
		contextSummary += fmt.Sprintf("Recent Events (%d):\n", len(context.RecentEvents))
		maxEvents := 3
		if len(context.RecentEvents) < maxEvents {
			maxEvents = len(context.RecentEvents)
		}
		for i, event := range context.RecentEvents[:maxEvents] {
			contextSummary += fmt.Sprintf("  %d. %s - %s (severity: %s)\n",
				i+1, event.Type, event.Source, event.Severity)
		}
	}

	// Add active recommendations
	if len(context.Recommendations) > 0 {
		contextSummary += fmt.Sprintf("\nActive Recommendations (%d):\n", len(context.Recommendations))
		maxRecs := 3
		if len(context.Recommendations) < maxRecs {
			maxRecs = len(context.Recommendations)
		}
		for i, rec := range context.Recommendations[:maxRecs] {
			contextSummary += fmt.Sprintf("  %d. [%s] %s\n",
				i+1, rec.Priority, rec.Title)
		}
	}

	// Add predicted issues
	if context.AnalysisInsights != nil && len(context.AnalysisInsights.PredictedIssues) > 0 {
		contextSummary += "\nPredicted Issues:\n"
		maxIssues := 2
		if len(context.AnalysisInsights.PredictedIssues) < maxIssues {
			maxIssues = len(context.AnalysisInsights.PredictedIssues)
		}
		for i, issue := range context.AnalysisInsights.PredictedIssues[:maxIssues] {
			contextSummary += fmt.Sprintf("  %d. %s (probability: %.2f)\n",
				i+1, issue.Description, issue.Probability)
		}
	}

	contextSummary += "--- End Context ---\n"

	return prompt + contextSummary
}

// enhanceErrorAnalysis enhances error analysis with context insights
func (a *AIAwareAnalyzer) enhanceErrorAnalysis(analysis *ErrorAnalysis, context *ComprehensiveContext) {
	// Add context-based root causes
	for _, pattern := range context.AnalysisInsights.Patterns {
		if pattern.Type == "repeated_failure" {
			analysis.PossibleCauses = append(analysis.PossibleCauses,
				fmt.Sprintf("Pattern detected: %s (occurrences: %d)", pattern.Description, pattern.Occurrences))
		}
	}

	// Add recommendations from context
	for _, rec := range context.Recommendations {
		if rec.Priority == "high" || rec.Priority == "critical" {
			analysis.Recommendations = append(analysis.Recommendations, rec.Description)
		}
	}

	// Add predicted issues as warnings
	for _, issue := range context.AnalysisInsights.PredictedIssues {
		if issue.Probability > 0.7 {
			analysis.AdditionalContext["predicted_issue"] = issue.Description
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Prevent predicted issue: %s", issue.Description))
		}
	}
}

// storeAnalysisResult stores analysis results for future reference
func (a *AIAwareAnalyzer) storeAnalysisResult(ctx context.Context, sessionID, prompt, result string, context *ComprehensiveContext) {
	analysisRecord := map[string]interface{}{
		"session_id":   sessionID,
		"timestamp":    time.Now(),
		"prompt":       prompt,
		"result":       result,
		"context_used": context.RequestID,
		"metadata": map[string]interface{}{
			"tool_contexts_count": len(context.ToolContexts),
			"recommendations":     len(context.Recommendations),
			"events_count":        len(context.RecentEvents),
		},
	}

	// Store in global state
	recordID := fmt.Sprintf("analysis_%s_%d", sessionID, time.Now().UnixNano())
	if err := a.integration.stateManager.SetState(ctx, state.StateTypeGlobal, recordID, analysisRecord); err != nil {
		a.logger.Error().Err(err).Msg("Failed to store analysis result")
	}
}

// CreateContextAwareTools creates tools with full context awareness
func (i *AIContextIntegration) CreateContextAwareTools(toolFactory *orchestration.ToolFactory) error {
	// Note: AI analyzer integration would be completed here
	// For now, we skip this as toolFactory doesn't have GetAIAnalyzer method

	// Create enhanced tool factory with state management
	enhancedFactory := orchestration.NewEnhancedToolFactory(toolFactory, i.stateManager)

	// Store for later use
	i.toolFactory = enhancedFactory

	i.logger.Info().Msg("Created context-aware tools")
	return nil
}

// GetToolRecommendations gets tool recommendations based on current context
func (i *AIContextIntegration) GetToolRecommendations(ctx context.Context, sessionID string) ([]*ToolRecommendation, error) {
	// Get comprehensive context
	compContext, err := i.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	recommendations := make([]*ToolRecommendation, 0)

	// Analyze context to recommend tools
	// Check if build succeeded but no deployment
	if buildCtx, hasBuild := compContext.ToolContexts["build"]; hasBuild {
		if buildData, ok := buildCtx.Data["docker_build"].(map[string]interface{}); ok {
			if images, ok := buildData["images_built"].(int); ok && images > 0 {
				// Check if deployment exists
				if _, hasDeployment := compContext.ToolContexts["deployment"]; !hasDeployment {
					recommendations = append(recommendations, &ToolRecommendation{
						Tool:        "k8s_deploy",
						Priority:    "high",
						Reason:      "Images built but not deployed",
						Description: "Deploy the built images to Kubernetes",
						Actions: []string{
							"Review deployment manifests",
							"Configure deployment parameters",
							"Execute deployment",
						},
					})
				}
			}
		}
	}

	// Check for security scanning needs
	if secCtx, hasSec := compContext.ToolContexts["security"]; !hasSec || secCtx.Timestamp.Before(time.Now().Add(-24*time.Hour)) {
		recommendations = append(recommendations, &ToolRecommendation{
			Tool:        "security_scan",
			Priority:    "medium",
			Reason:      "Security scan outdated or missing",
			Description: "Run security scan on images",
			Actions: []string{
				"Scan for vulnerabilities",
				"Review security policies",
				"Update base images if needed",
			},
		})
	}

	// Check for performance testing needs
	if _, hasPerf := compContext.ToolContexts["performance"]; !hasPerf {
		if _, hasDeployment := compContext.ToolContexts["deployment"]; hasDeployment {
			recommendations = append(recommendations, &ToolRecommendation{
				Tool:        "performance_test",
				Priority:    "low",
				Reason:      "No performance testing done",
				Description: "Run performance tests on deployed application",
				Actions: []string{
					"Define performance criteria",
					"Execute load tests",
					"Analyze results",
				},
			})
		}
	}

	return recommendations, nil
}

// ToolRecommendation represents a tool usage recommendation
type ToolRecommendation struct {
	Tool        string
	Priority    string
	Reason      string
	Description string
	Actions     []string
}

// GetContextSummary gets a summary of the current context
func (i *AIContextIntegration) GetContextSummary(ctx context.Context, sessionID string) (*ContextSummary, error) {
	compContext, err := i.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	summary := &ContextSummary{
		SessionID:           sessionID,
		Timestamp:           compContext.Timestamp,
		ToolsActive:         len(compContext.ToolContexts),
		EventCount:          len(compContext.RecentEvents),
		RecommendationCount: len(compContext.Recommendations),
		OverallHealth:       i.calculateOverallHealth(compContext),
		KeyInsights:         i.extractKeyInsights(compContext),
		ActionItems:         i.extractActionItems(compContext),
	}

	return summary, nil
}

// ContextSummary provides a high-level summary of context
type ContextSummary struct {
	SessionID           string
	Timestamp           time.Time
	ToolsActive         int
	EventCount          int
	RecommendationCount int
	OverallHealth       float64
	KeyInsights         []string
	ActionItems         []string
}

// calculateOverallHealth calculates overall system health
func (i *AIContextIntegration) calculateOverallHealth(context *ComprehensiveContext) float64 {
	health := 1.0

	// Deduct for critical recommendations
	for _, rec := range context.Recommendations {
		if rec.Priority == "critical" {
			health -= 0.2
		} else if rec.Priority == "high" {
			health -= 0.1
		}
	}

	// Deduct for predicted issues
	if context.AnalysisInsights != nil {
		for _, issue := range context.AnalysisInsights.PredictedIssues {
			if issue.Probability > 0.8 {
				health -= 0.15
			}
		}
	}

	// Ensure health doesn't go below 0
	if health < 0 {
		health = 0
	}

	return health
}

// extractKeyInsights extracts key insights from context
func (i *AIContextIntegration) extractKeyInsights(context *ComprehensiveContext) []string {
	insights := make([]string, 0)

	// Add pattern insights
	if context.AnalysisInsights != nil {
		for _, pattern := range context.AnalysisInsights.Patterns {
			if pattern.Confidence > 0.8 {
				insights = append(insights, pattern.Description)
			}
		}
	}

	// Add relationship insights
	for _, rel := range context.Relationships {
		if rel.Strength > 0.8 {
			insights = append(insights, rel.Description)
		}
	}

	return insights
}

// extractActionItems extracts action items from context
func (i *AIContextIntegration) extractActionItems(context *ComprehensiveContext) []string {
	actions := make([]string, 0)

	// Extract from recommendations
	for _, rec := range context.Recommendations {
		if rec.Priority == "critical" || rec.Priority == "high" {
			actions = append(actions, rec.Actions...)
		}
	}

	// Extract from predicted issues
	if context.AnalysisInsights != nil {
		for _, issue := range context.AnalysisInsights.PredictedIssues {
			if issue.Probability > 0.7 {
				actions = append(actions, issue.Mitigations...)
			}
		}
	}

	// Deduplicate actions
	uniqueActions := make(map[string]bool)
	result := make([]string, 0)
	for _, action := range actions {
		if !uniqueActions[action] {
			uniqueActions[action] = true
			result = append(result, action)
		}
	}

	return result
}
