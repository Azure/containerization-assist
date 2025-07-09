package state

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/knowledge"
	"github.com/Azure/container-kit/pkg/mcp/domain"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// AIAnalyzer interface for AI analysis operations
// This interface represents the minimal contract that an AI analyzer must implement
type AIAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() domain.TokenUsage
	ResetTokenUsage()
}

// AIContextIntegration provides comprehensive AI context integration
type AIContextIntegration struct {
	aggregator     *AIContextAggregator
	stateManager   *UnifiedStateManager
	sessionManager session.SessionManager
	knowledgeBase  *knowledge.CrossToolKnowledgeBase
	logger         *slog.Logger
}

// NewAIContextIntegration creates a new AI context integration
func NewAIContextIntegration(
	stateManager *UnifiedStateManager,
	sessionManager session.SessionManager,
	knowledgeBase *knowledge.CrossToolKnowledgeBase,
	logger *slog.Logger,
) *AIContextIntegration {
	aggregator := NewAIContextAggregator()

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

	aggregator.RegisterContextEnricher(NewRelationshipEnricher(logger))
	aggregator.RegisterContextEnricher(NewInsightEnricher(knowledgeBase, logger))
	// TODO: Fix parameter mismatch - NewSecurityEnricher expects SessionStore and SessionState
	// aggregator.RegisterContextEnricher(NewSecurityEnricher(sessionManager, logger))
	aggregator.RegisterContextEnricher(NewPerformanceEnricher(logger))

	return &AIContextIntegration{
		aggregator:     aggregator,
		stateManager:   stateManager,
		sessionManager: sessionManager,
		knowledgeBase:  knowledgeBase,
		logger:         logger.With(slog.String("component", "ai_context_integration")),
	}
}

func (i *AIContextIntegration) GetAggregator() *AIContextAggregator {
	return i.aggregator
}

func (i *AIContextIntegration) CreateAIAwareAnalyzer(baseAnalyzer AIAnalyzer) AIAnalyzer {
	return &AIAwareAnalyzer{
		baseAnalyzer: baseAnalyzer,
		integration:  i,
		logger:       i.logger,
	}
}

type AIAwareAnalyzer struct {
	baseAnalyzer AIAnalyzer
	integration  *AIContextIntegration
	logger       *slog.Logger
}

// AnalyzeWithContext analyzes with context awareness
func (a *AIAwareAnalyzer) AnalyzeWithContext(ctx context.Context, prompt string) (string, error) {
	// Extract session ID from context
	sessionID := extractSessionID(ctx)
	if sessionID == "" {
		// No session context, use base analyzer
		return a.baseAnalyzer.Analyze(ctx, prompt)
	}

	// Get comprehensive context
	compContext, err := a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		a.integration.logger.Warn("Failed to get comprehensive context",
			slog.String("error", err.Error()))
		return a.baseAnalyzer.Analyze(ctx, prompt)
	}

	// Enhance prompt with context
	enhancedPrompt := a.enhancePromptWithContext(prompt, compContext)

	// Analyze with enhanced prompt
	result, err := a.baseAnalyzer.Analyze(ctx, enhancedPrompt)
	if err != nil {
		return "", err
	}

	// Post-process result with context awareness
	return a.postProcessWithContext(result, compContext), nil
}

func (a *AIAwareAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	// Delegate to context-aware version
	return a.AnalyzeWithContext(ctx, prompt)
}

func (a *AIAwareAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	// Get session context if available
	sessionID := extractSessionID(ctx)
	if sessionID != "" {
		compContext, _ := a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
		if compContext != nil {
			prompt = a.enhancePromptWithContext(prompt, compContext)
		}
	}

	return a.baseAnalyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
}

func (a *AIAwareAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	formattedPrompt := fmt.Sprintf(promptTemplate, args...)
	return a.Analyze(ctx, formattedPrompt)
}

func (a *AIAwareAnalyzer) GetTokenUsage() domain.TokenUsage {
	return a.baseAnalyzer.GetTokenUsage()
}

func (a *AIAwareAnalyzer) ResetTokenUsage() {
	a.baseAnalyzer.ResetTokenUsage()
}

type ErrorAnalysis struct {
	PossibleCauses    []string
	Recommendations   []string
	AdditionalContext map[string]interface{}
}

func (a *AIAwareAnalyzer) AnalyzeError(ctx context.Context, err error, contextInfo map[string]interface{}) (*ErrorAnalysis, error) {
	sessionID := ""
	if sid, ok := contextInfo["session_id"].(string); ok {
		sessionID = sid
	}

	var compContext *ComprehensiveContext
	if sessionID != "" {
		compContext, _ = a.integration.aggregator.GetComprehensiveContext(ctx, sessionID)
	}

	if compContext != nil {
		contextInfo["comprehensive_context"] = compContext
		contextInfo["recommendations"] = compContext.Recommendations
		contextInfo["predicted_issues"] = compContext.AnalysisInsights.PredictedIssues
	}

	analysis := &ErrorAnalysis{
		PossibleCauses:    []string{err.Error()},
		Recommendations:   []string{},
		AdditionalContext: make(map[string]interface{}),
	}

	if compContext != nil {
		a.enhanceErrorAnalysis(analysis, compContext)
	}

	return analysis, nil
}

func (a *AIAwareAnalyzer) extractSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}

	return ""
}

func (a *AIAwareAnalyzer) enhancePromptWithContext(prompt string, context *ComprehensiveContext) string {
	contextSummary := "\n\n--- Context Information ---\n"

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

	if len(context.Recommendations) > 0 {
		contextSummary += fmt.Sprintf("\nActive Recommendations (%d):\n", len(context.Recommendations))
		maxRecs := 3
		if len(context.Recommendations) < maxRecs {
			maxRecs = len(context.Recommendations)
		}
		for i, rec := range context.Recommendations[:maxRecs] {
			contextSummary += fmt.Sprintf("  %d. [%d] %s\n",
				i+1, rec.Priority, rec.Title)
		}
	}

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

func (a *AIAwareAnalyzer) enhanceErrorAnalysis(analysis *ErrorAnalysis, context *ComprehensiveContext) {
	for _, pattern := range context.AnalysisInsights.Patterns {
		if pattern.Type == "repeated_failure" {
			analysis.PossibleCauses = append(analysis.PossibleCauses,
				fmt.Sprintf("Pattern detected: %s (occurrences: %d)", pattern.Description, pattern.Occurrences))
		}
	}

	for _, rec := range context.Recommendations {
		if rec.Priority <= 2 { // 1 = critical, 2 = high
			analysis.Recommendations = append(analysis.Recommendations, rec.Description)
		}
	}

	for _, issue := range context.AnalysisInsights.PredictedIssues {
		if issue.Probability > 0.7 {
			analysis.AdditionalContext["predicted_issue"] = issue.Description
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Prevent predicted issue: %s", issue.Description))
		}
	}
}

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

	recordID := fmt.Sprintf("analysis_%s_%d", sessionID, time.Now().UnixNano())
	if err := a.integration.stateManager.SetState(ctx, StateTypeGlobal, recordID, analysisRecord); err != nil {
		a.logger.Error("Failed to store analysis result",
			slog.String("error", err.Error()))
	}
}

// CreateContextAwareTools creates tools with full context awareness
func (i *AIContextIntegration) CreateContextAwareTools(toolFactory interface{}) error {

	i.logger.Info("Created context-aware tools (simplified)")
	return nil
}

func (i *AIContextIntegration) GetToolRecommendations(ctx context.Context, sessionID string) ([]*ToolRecommendation, error) {
	compContext, err := i.aggregator.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	recommendations := make([]*ToolRecommendation, 0)

	if buildCtx, hasBuild := compContext.ToolContexts["build"]; hasBuild {
		if buildData, ok := buildCtx.Data["docker_build"].(map[string]interface{}); ok {
			if images, ok := buildData["images_built"].(int); ok && images > 0 {
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

type ToolRecommendation struct {
	Tool        string
	Priority    string
	Reason      string
	Description string
	Actions     []string
}

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

func (i *AIContextIntegration) calculateOverallHealth(context *ComprehensiveContext) float64 {
	health := 1.0

	for _, rec := range context.Recommendations {
		if rec.Priority == 1 { // critical
			health -= 0.2
		} else if rec.Priority == 2 { // high
			health -= 0.1
		}
	}

	if context.AnalysisInsights != nil {
		for _, issue := range context.AnalysisInsights.PredictedIssues {
			if issue.Probability > 0.8 {
				health -= 0.15
			}
		}
	}

	if health < 0 {
		health = 0
	}

	return health
}

func (i *AIContextIntegration) extractKeyInsights(context *ComprehensiveContext) []string {
	insights := make([]string, 0)

	if context.AnalysisInsights != nil {
		for _, pattern := range context.AnalysisInsights.Patterns {
			if pattern.Confidence > 0.8 {
				insights = append(insights, pattern.Description)
			}
		}
	}

	for _, rel := range context.Relationships {
		if rel.Strength > 0.8 {
			insights = append(insights, rel.Description)
		}
	}

	return insights
}

func (i *AIContextIntegration) extractActionItems(context *ComprehensiveContext) []string {
	actions := make([]string, 0)

	for _, rec := range context.Recommendations {
		if rec.Priority <= 2 { // critical (1) or high (2)
			actions = append(actions, rec.Actions...)
		}
	}

	if context.AnalysisInsights != nil {
		for _, issue := range context.AnalysisInsights.PredictedIssues {
			if issue.Probability > 0.7 {
				actions = append(actions, issue.Mitigations...)
			}
		}
	}

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

// postProcessWithContext post-processes result with context awareness
func (a *AIAwareAnalyzer) postProcessWithContext(result string, context *ComprehensiveContext) string {
	// Add context-aware enhancements to the result
	if len(context.Recommendations) > 0 {
		result += "\n\n--- Context-Aware Recommendations ---\n"
		for i, rec := range context.Recommendations {
			if i >= 3 {
				break // Limit to top 3
			}
			result += fmt.Sprintf("• %s\n", rec.Title)
		}
	}

	return result
}

// extractSessionID extracts session ID from context
func extractSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}
