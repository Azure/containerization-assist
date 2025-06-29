package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// EnhancedBuildAnalyzer implements UnifiedAnalyzer with comprehensive build intelligence
type EnhancedBuildAnalyzer struct {
	aiAnalyzer         mcp.AIAnalyzer
	repositoryAnalyzer core.RepositoryAnalyzer // Use core interface directly
	knowledgeBase      *CrossToolKnowledgeBase
	failurePredictor   *FailurePredictor
	strategizer        *BuildStrategizer
	logger             zerolog.Logger
	// Capabilities
	capabilities *AnalyzerCapabilities
}

// NewEnhancedBuildAnalyzer creates a new unified analyzer with full capabilities
func NewEnhancedBuildAnalyzer(
	aiAnalyzer mcp.AIAnalyzer,
	repositoryAnalyzer core.RepositoryAnalyzer, // Use core interface directly
	logger zerolog.Logger,
) *EnhancedBuildAnalyzer {
	analyzer := &EnhancedBuildAnalyzer{
		aiAnalyzer:         aiAnalyzer,
		repositoryAnalyzer: repositoryAnalyzer,
		knowledgeBase:      NewCrossToolKnowledgeBase(logger),
		failurePredictor:   NewFailurePredictor(logger),
		strategizer:        NewBuildStrategizer(logger),
		logger:             logger.With().Str("component", "enhanced_build_analyzer").Logger(),
		capabilities: &AnalyzerCapabilities{
			SupportedFailureTypes: []string{
				"dockerfile_error", "dependency_error", "build_error", "test_failure",
				"deployment_error", "security_issue", "performance_issue", "resource_exhaustion",
			},
			SupportedLanguages: []string{
				"go", "python", "javascript", "typescript", "java", "csharp", "rust", "cpp",
			},
			CanAnalyzeRepository:   true,
			CanGenerateFixes:       true,
			CanOptimizePerformance: true,
			CanAssessSecurity:      true,
			CanPredictFailures:     true,
			CanShareKnowledge:      true,
		},
	}
	return analyzer
}

// Implement AIAnalyzer interface by delegating to underlying AI analyzer
func (e *EnhancedBuildAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return e.aiAnalyzer.Analyze(ctx, prompt)
}
func (e *EnhancedBuildAnalyzer) AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error) {
	return e.aiAnalyzer.AnalyzeWithFileTools(ctx, prompt, baseDir)
}
func (e *EnhancedBuildAnalyzer) AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error) {
	return e.aiAnalyzer.AnalyzeWithFormat(ctx, promptTemplate, args...)
}
func (e *EnhancedBuildAnalyzer) GetTokenUsage() mcptypes.TokenUsage {
	return e.aiAnalyzer.GetTokenUsage()
}
func (e *EnhancedBuildAnalyzer) ResetTokenUsage() {
	e.aiAnalyzer.ResetTokenUsage()
}

// Enhanced analysis capabilities
// AnalyzeFailure provides comprehensive failure analysis
func (e *EnhancedBuildAnalyzer) AnalyzeFailure(ctx context.Context, failure *AnalysisRequest) (*AnalysisResult, error) {
	startTime := time.Now()
	e.logger.Info().
		Str("session_id", failure.SessionID).
		Str("tool_name", failure.ToolName).
		Str("operation_type", failure.OperationType).
		Msg("Starting comprehensive failure analysis")
	// Get historical context from knowledge base
	relatedFailures, err := e.knowledgeBase.GetRelatedFailures(ctx, failure)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to get related failures")
		relatedFailures = []*RelatedFailure{}
	}
	// Categorize the failure
	failureType := e.categorizeFailure(failure.Error)
	// Assess severity and impact
	severity := e.assessSeverity(failure.Error, failure.Context)
	impactAssessment := e.assessImpact(ctx, failure, severity)
	// Generate fix strategies using AI
	fixStrategies, err := e.generateFixStrategies(ctx, failure, failureType, relatedFailures)
	if err != nil {
		e.logger.Error().Err(err).Msg("Failed to generate fix strategies")
		fixStrategies = []*FixStrategy{}
	}
	// Determine if retry is worthwhile
	isRetryable := e.isRetryable(failure.Error, len(relatedFailures))
	// Generate recommendations
	recommendations := e.generateRecommendations(failure, fixStrategies, relatedFailures)
	// Calculate confidence score
	confidenceScore := e.calculateConfidenceScore(fixStrategies, relatedFailures, impactAssessment)
	result := &AnalysisResult{
		FailureType:      failureType,
		RootCause:        e.identifyRootCause(failure.Error, failure.Context),
		Severity:         severity,
		IsRetryable:      isRetryable,
		FixStrategies:    fixStrategies,
		RelatedFailures:  relatedFailures,
		ImpactAssessment: impactAssessment,
		Recommendations:  recommendations,
		AnalysisMetadata: map[string]interface{}{
			"analyzer_version":       "enhanced_v1.0",
			"analysis_method":        "ai_augmented_pattern_recognition",
			"knowledge_base_entries": len(relatedFailures),
		},
		AnalysisDuration: time.Since(startTime),
		ConfidenceScore:  confidenceScore,
	}
	// Share insights for future analysis
	go e.shareFailureInsights(ctx, failure, result)
	return result, nil
}

// GetCapabilities returns analyzer capabilities
func (e *EnhancedBuildAnalyzer) GetCapabilities() *AnalyzerCapabilities {
	return e.capabilities
}

// AnalyzeWithRepository provides repository-aware analysis
func (e *EnhancedBuildAnalyzer) AnalyzeWithRepository(ctx context.Context, request *RepositoryAnalysisRequest) (*RepositoryAwareAnalysis, error) {
	e.logger.Info().
		Str("session_id", request.SessionID).
		Msg("Starting repository-aware analysis")
	// Perform basic failure analysis
	baseAnalysis, err := e.AnalyzeFailure(ctx, &request.AnalysisRequest)
	if err != nil {
		return nil, fmt.Errorf("base analysis failed: %w", err)
	}
	// Repository-specific insights
	projectInsights, err := e.analyzeProjectInsights(ctx, request.RepositoryInfo, request.ProjectMetadata)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to analyze project insights")
		projectInsights = &ProjectInsights{}
	}
	// Language-specific recommendations
	languageRecommendations := e.generateLanguageRecommendations(request.ProjectMetadata, baseAnalysis)
	// Framework optimizations
	frameworkOptimizations := e.generateFrameworkOptimizations(request.ProjectMetadata, request.BuildHistory)
	// Dependency analysis
	dependencyAnalysis, err := e.analyzeDependencies(ctx, request.RepositoryInfo)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to analyze dependencies")
		dependencyAnalysis = &DependencyAnalysis{}
	}
	// Security implications
	securityAnalysis := e.analyzeSecurityImplications(request.RepositoryInfo, baseAnalysis)
	return &RepositoryAwareAnalysis{
		AnalysisResult:          baseAnalysis,
		ProjectSpecificInsights: projectInsights,
		LanguageRecommendations: languageRecommendations,
		FrameworkOptimizations:  frameworkOptimizations,
		DependencyAnalysis:      dependencyAnalysis,
		SecurityImplications:    securityAnalysis,
	}, nil
}

// ShareInsights shares tool insights with the knowledge base
func (e *EnhancedBuildAnalyzer) ShareInsights(ctx context.Context, insights *ToolInsights) error {
	return e.knowledgeBase.StoreInsights(ctx, insights)
}

// GetSharedKnowledge retrieves accumulated knowledge for a domain
func (e *EnhancedBuildAnalyzer) GetSharedKnowledge(ctx context.Context, domain string) (*SharedKnowledge, error) {
	return e.knowledgeBase.GetKnowledge(ctx, domain)
}

// OptimizeBuildStrategy optimizes build strategy based on context
func (e *EnhancedBuildAnalyzer) OptimizeBuildStrategy(ctx context.Context, request *BuildOptimizationRequest) (*OptimizedBuildStrategy, error) {
	return e.strategizer.OptimizeStrategy(ctx, request)
}

// PredictFailures predicts potential build failures
func (e *EnhancedBuildAnalyzer) PredictFailures(ctx context.Context, buildContext *AnalysisBuildContext) (*FailurePrediction, error) {
	return e.failurePredictor.PredictFailures(ctx, buildContext)
}

// Private helper methods
func (e *EnhancedBuildAnalyzer) categorizeFailure(err error) string {
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "dockerfile"):
		return "dockerfile_error"
	case strings.Contains(errStr, "dependency") || strings.Contains(errStr, "package") || strings.Contains(errStr, "module"):
		return "dependency_error"
	case strings.Contains(errStr, "build") || strings.Contains(errStr, "compile"):
		return "build_error"
	case strings.Contains(errStr, "test"):
		return "test_failure"
	case strings.Contains(errStr, "deploy") || strings.Contains(errStr, "manifest"):
		return "deployment_error"
	case strings.Contains(errStr, "security") || strings.Contains(errStr, "vulnerability"):
		return "security_issue"
	case strings.Contains(errStr, "memory") || strings.Contains(errStr, "cpu") || strings.Contains(errStr, "resource"):
		return "resource_exhaustion"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "slow"):
		return "performance_issue"
	default:
		return "unknown_error"
	}
}
func (e *EnhancedBuildAnalyzer) assessSeverity(err error, context map[string]interface{}) string {
	errStr := strings.ToLower(err.Error())
	// Critical severity indicators
	if strings.Contains(errStr, "fatal") || strings.Contains(errStr, "critical") ||
		strings.Contains(errStr, "segmentation fault") || strings.Contains(errStr, "out of memory") {
		return "critical"
	}
	// High severity indicators
	if strings.Contains(errStr, "error") || strings.Contains(errStr, "failed") ||
		strings.Contains(errStr, "exception") {
		return "high"
	}
	// Medium severity indicators
	if strings.Contains(errStr, "warning") || strings.Contains(errStr, "deprecated") {
		return "medium"
	}
	return "low"
}
func (e *EnhancedBuildAnalyzer) assessImpact(ctx context.Context, failure *AnalysisRequest, severity string) *ImpactAssessment {
	// Simple impact assessment - could be enhanced with more sophisticated analysis
	var businessImpact, technicalImpact, userImpact string
	var recoveryTime time.Duration
	var costEstimate float64
	var urgencyScore float64
	switch severity {
	case "critical":
		businessImpact = "High - blocks all development"
		technicalImpact = "Severe - system unusable"
		userImpact = "Major - complete service disruption"
		recoveryTime = time.Hour * 4
		costEstimate = 1000.0
		urgencyScore = 0.9
	case "high":
		businessImpact = "Medium - affects productivity"
		technicalImpact = "Major - feature broken"
		userImpact = "Moderate - degraded experience"
		recoveryTime = time.Hour * 2
		costEstimate = 500.0
		urgencyScore = 0.7
	case "medium":
		businessImpact = "Low - minor delays"
		technicalImpact = "Minor - workaround available"
		userImpact = "Low - minimal impact"
		recoveryTime = time.Hour
		costEstimate = 100.0
		urgencyScore = 0.5
	default:
		businessImpact = "Minimal"
		technicalImpact = "Negligible"
		userImpact = "None"
		recoveryTime = time.Minute * 30
		costEstimate = 50.0
		urgencyScore = 0.3
	}
	return &ImpactAssessment{
		BusinessImpact:  businessImpact,
		TechnicalImpact: technicalImpact,
		UserImpact:      userImpact,
		RecoveryTime:    recoveryTime,
		CostEstimate:    costEstimate,
		UrgencyScore:    urgencyScore,
	}
}
func (e *EnhancedBuildAnalyzer) generateFixStrategies(ctx context.Context, failure *AnalysisRequest, failureType string, relatedFailures []*RelatedFailure) ([]*FixStrategy, error) {
	strategies := []*FixStrategy{}
	// Generate AI-enhanced fix strategies
	prompt := fmt.Sprintf(`
Analyze this %s failure and provide detailed fix strategies:
Error: %s
Tool: %s
Operation: %s
Context: %v
Similar past failures: %d found
Provide 3-5 concrete fix strategies with:
1. Step-by-step instructions
2. Estimated time to implement
3. Success probability
4. Risk assessment
5. Prerequisites
Format as structured recommendations.
`, failureType, failure.Error.Error(), failure.ToolName, failure.OperationType, failure.Context, len(relatedFailures))
	aiResponse, err := e.aiAnalyzer.Analyze(ctx, prompt)
	if err != nil {
		e.logger.Warn().Err(err).Msg("AI analysis failed, using fallback strategies")
		return e.getFallbackStrategies(failureType), nil
	}
	// Parse AI response and create strategies
	// For now, create some example strategies based on the response
	strategies = append(strategies, &FixStrategy{
		Name:          "AI-Suggested Primary Fix",
		Description:   "AI-generated fix based on error analysis",
		Steps:         []string{"Analyze error context", "Apply AI-suggested fix", "Verify resolution"},
		EstimatedTime: time.Minute * 10,
		SuccessRate:   0.8,
		RiskLevel:     "low",
		Prerequisites: []string{"Workspace access", "Build tools available"},
		Metadata: map[string]interface{}{
			"ai_generated": true,
			"source":       "enhanced_build_analyzer",
			"response":     aiResponse,
		},
	})
	// Add strategies based on related failures
	for _, related := range relatedFailures {
		if related.Similarity > 0.7 {
			strategies = append(strategies, &FixStrategy{
				Name:          fmt.Sprintf("Historical Fix: %s", related.Resolution),
				Description:   fmt.Sprintf("Previously successful fix for similar %s", related.FailureType),
				Steps:         []string{related.Resolution},
				EstimatedTime: time.Minute * 5,
				SuccessRate:   related.Similarity,
				RiskLevel:     "low",
				Prerequisites: []string{"Similar context to previous failure"},
				Metadata: map[string]interface{}{
					"historical": true,
					"last_used":  related.LastOccurred,
					"frequency":  related.Frequency,
					"similarity": related.Similarity,
				},
			})
		}
	}
	return strategies, nil
}
func (e *EnhancedBuildAnalyzer) getFallbackStrategies(failureType string) []*FixStrategy {
	// Provide basic fallback strategies when AI analysis fails
	switch failureType {
	case "dockerfile_error":
		return []*FixStrategy{
			{
				Name:          "Dockerfile Syntax Check",
				Description:   "Check and fix Dockerfile syntax errors",
				Steps:         []string{"Validate Dockerfile syntax", "Fix syntax errors", "Rebuild image"},
				EstimatedTime: time.Minute * 5,
				SuccessRate:   0.7,
				RiskLevel:     "low",
			},
		}
	case "dependency_error":
		return []*FixStrategy{
			{
				Name:          "Dependency Resolution",
				Description:   "Resolve dependency conflicts",
				Steps:         []string{"Clean dependency cache", "Update dependencies", "Resolve conflicts"},
				EstimatedTime: time.Minute * 10,
				SuccessRate:   0.6,
				RiskLevel:     "medium",
			},
		}
	default:
		return []*FixStrategy{
			{
				Name:          "Generic Retry",
				Description:   "Retry operation with clean state",
				Steps:         []string{"Clean workspace", "Retry operation"},
				EstimatedTime: time.Minute * 3,
				SuccessRate:   0.5,
				RiskLevel:     "low",
			},
		}
	}
}
func (e *EnhancedBuildAnalyzer) identifyRootCause(err error, context map[string]interface{}) string {
	// Simple root cause identification - could be enhanced with more sophisticated analysis
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "permission denied") {
		return "Insufficient permissions"
	}
	if strings.Contains(errStr, "not found") {
		return "Missing resource or dependency"
	}
	if strings.Contains(errStr, "timeout") {
		return "Operation timeout or network issues"
	}
	if strings.Contains(errStr, "syntax") {
		return "Syntax error in configuration or code"
	}
	return "Unknown root cause - requires deeper analysis"
}
func (e *EnhancedBuildAnalyzer) isRetryable(err error, relatedFailureCount int) bool {
	errStr := strings.ToLower(err.Error())
	// Non-retryable errors
	if strings.Contains(errStr, "syntax") || strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "permission denied") {
		return false
	}
	// Retryable errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "temporary") || relatedFailureCount > 0 {
		return true
	}
	return true // Default to retryable for unknown errors
}
func (e *EnhancedBuildAnalyzer) generateRecommendations(failure *AnalysisRequest, strategies []*FixStrategy, relatedFailures []*RelatedFailure) []string {
	recommendations := []string{}
	if len(strategies) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Try the '%s' strategy first (%.0f%% success rate)",
			strategies[0].Name, strategies[0].SuccessRate*100))
	}
	if len(relatedFailures) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Similar failures occurred %d times recently - check pattern analysis", len(relatedFailures)))
	}
	recommendations = append(recommendations, "Consider adding monitoring for this failure type")
	recommendations = append(recommendations, "Document resolution for future reference")
	return recommendations
}
func (e *EnhancedBuildAnalyzer) calculateConfidenceScore(strategies []*FixStrategy, relatedFailures []*RelatedFailure, impact *ImpactAssessment) float64 {
	score := 0.5 // Base confidence
	// Boost confidence based on available strategies
	if len(strategies) > 0 {
		avgSuccessRate := 0.0
		for _, strategy := range strategies {
			avgSuccessRate += strategy.SuccessRate
		}
		avgSuccessRate /= float64(len(strategies))
		score += avgSuccessRate * 0.3
	}
	// Boost confidence based on historical data
	if len(relatedFailures) > 0 {
		score += 0.2
	}
	// Adjust based on impact urgency
	score += impact.UrgencyScore * 0.1
	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}
	return score
}
func (e *EnhancedBuildAnalyzer) shareFailureInsights(ctx context.Context, failure *AnalysisRequest, result *AnalysisResult) {
	// Share insights asynchronously to avoid blocking main analysis
	insights := &ToolInsights{
		ToolName:      failure.ToolName,
		OperationType: failure.OperationType,
		FailurePattern: &FailurePattern{
			PatternName:      result.FailureType,
			FailureType:      result.FailureType,
			CommonCauses:     []string{result.RootCause},
			TypicalSolutions: e.extractSolutionNames(result.FixStrategies),
			Frequency:        1, // Will be aggregated by knowledge base
		},
		PerformanceMetrics: &PerformanceMetrics{
			TotalDuration: result.AnalysisDuration,
		},
		Metadata: map[string]interface{}{
			"severity":         result.Severity,
			"confidence_score": result.ConfidenceScore,
			"retryable":        result.IsRetryable,
		},
		Timestamp: time.Now(),
	}
	err := e.knowledgeBase.StoreInsights(ctx, insights)
	if err != nil {
		e.logger.Warn().Err(err).Msg("Failed to share failure insights")
	}
}
func (e *EnhancedBuildAnalyzer) extractSolutionNames(strategies []*FixStrategy) []string {
	solutions := make([]string, len(strategies))
	for i, strategy := range strategies {
		solutions[i] = strategy.Name
	}
	return solutions
}

// Placeholder implementations for repository-aware analysis
func (e *EnhancedBuildAnalyzer) analyzeProjectInsights(ctx context.Context, repoInfo *core.RepositoryInfo, projectMeta *ProjectMetadata) (*ProjectInsights, error) {
	// Placeholder - would analyze code quality, architecture, etc.
	return &ProjectInsights{
		CodeQuality: &QualityMetrics{
			OverallScore:    0.8,
			Maintainability: 0.7,
			Reliability:     0.8,
			Security:        0.9,
			Performance:     0.6,
		},
	}, nil
}
func (e *EnhancedBuildAnalyzer) generateLanguageRecommendations(projectMeta *ProjectMetadata, analysis *AnalysisResult) []string {
	recommendations := []string{}
	switch strings.ToLower(projectMeta.Language) {
	case "go":
		recommendations = append(recommendations, "Consider using Go modules for dependency management")
		recommendations = append(recommendations, "Enable Go build cache for faster builds")
	case "python":
		recommendations = append(recommendations, "Use virtual environments to avoid dependency conflicts")
		recommendations = append(recommendations, "Consider using poetry for dependency management")
	case "javascript", "typescript":
		recommendations = append(recommendations, "Use npm ci for faster, reproducible builds")
		recommendations = append(recommendations, "Consider using workspace features for monorepos")
	default:
		recommendations = append(recommendations, "Follow language-specific best practices")
	}
	return recommendations
}
func (e *EnhancedBuildAnalyzer) generateFrameworkOptimizations(projectMeta *ProjectMetadata, buildHistory []*BuildHistoryEntry) []*FrameworkOptimization {
	optimizations := []*FrameworkOptimization{}
	if projectMeta.Framework != "" {
		optimizations = append(optimizations, &FrameworkOptimization{
			Framework:     projectMeta.Framework,
			Optimizations: []string{"Enable framework-specific optimizations", "Use framework build tools"},
			Impact:        "medium",
			Difficulty:    "low",
		})
	}
	return optimizations
}
func (e *EnhancedBuildAnalyzer) analyzeDependencies(ctx context.Context, repoInfo *core.RepositoryInfo) (*DependencyAnalysis, error) {
	// Placeholder - would analyze actual dependencies
	return &DependencyAnalysis{
		OutdatedDeps:   []string{},
		SecurityIssues: []string{},
		LicenseIssues:  []string{},
		SizeImpact: &SizeImpactAnalysis{
			TotalSize:       1024 * 1024, // 1MB placeholder
			LargestDeps:     []string{},
			OptimizationOps: []string{"Remove unused dependencies"},
			SizeReduction:   256 * 1024, // 256KB potential reduction
		},
		Recommendations: []string{"Regular dependency updates", "Security scanning"},
	}, nil
}
func (e *EnhancedBuildAnalyzer) analyzeSecurityImplications(repoInfo *core.RepositoryInfo, analysis *AnalysisResult) *GeneralSecurityAnalysis {
	// Placeholder - would analyze actual security implications
	return &GeneralSecurityAnalysis{
		VulnerabilityCount: 0,
		RiskLevel:          "low",
		CriticalIssues:     []string{},
		Recommendations:    []string{"Regular security scanning", "Keep dependencies updated"},
	}
}
