// Package ml provides advanced error pattern recognition capabilities
package ml

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	domainml "github.com/Azure/container-kit/pkg/mcp/domain/ml"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// AdvancedPatternRecognizer extends the basic pattern recognizer with machine learning capabilities
type AdvancedPatternRecognizer struct {
	*ErrorPatternRecognizer
	patternCache     *PatternCache
	learningEngine   *PatternLearningEngine
	similarityEngine *SimilarityEngine
	logger           *slog.Logger
	mu               sync.RWMutex
}

// NewAdvancedPatternRecognizer creates a new advanced pattern recognizer
func NewAdvancedPatternRecognizer(samplingClient domainsampling.UnifiedSampler, logger *slog.Logger) *AdvancedPatternRecognizer {
	baseRecognizer := NewErrorPatternRecognizer(samplingClient, logger)

	return &AdvancedPatternRecognizer{
		ErrorPatternRecognizer: baseRecognizer,
		patternCache:           NewPatternCache(),
		learningEngine:         NewPatternLearningEngine(),
		similarityEngine:       NewSimilarityEngine(),
		logger:                 logger.With("component", "advanced_pattern_recognizer"),
	}
}

// AnalyzeErrorPatterns performs deep analysis of error patterns
func (r *AdvancedPatternRecognizer) AnalyzeErrorPatterns(ctx context.Context, err error, context WorkflowContext) (*EnhancedErrorClassification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()

	// Check pattern cache first
	if cached := r.patternCache.Get(err.Error()); cached != nil {
		r.logger.Debug("Using cached pattern analysis", "cache_hit", true)
		// Still update learning engine even with cached results
		r.learningEngine.Learn(err, context, cached)

		// Recalculate trend analysis for cached results
		trends := r.learningEngine.AnalyzeTrends(context.StepName, err.Error())
		cached.TrendAnalysis = TrendAnalysis{
			Frequency:             trends.Frequency,
			RecentIncrease:        trends.RecentIncrease,
			SeasonalPattern:       trends.SeasonalPattern,
			AverageResolutionTime: trends.AverageResolutionTime,
			SuccessRate:           trends.SuccessRate,
		}

		return cached, nil
	}

	// Perform comprehensive analysis
	classification, analyzeErr := r.comprehensiveAnalysis(ctx, err, context)
	if analyzeErr != nil {
		return nil, analyzeErr
	}

	// Cache the result
	r.patternCache.Set(err.Error(), classification)

	// Update learning engine
	r.learningEngine.Learn(err, context, classification)

	duration := time.Since(start)
	r.logger.Info("Advanced pattern analysis completed",
		"duration", duration,
		"confidence", classification.Confidence,
		"patterns_detected", len(classification.Patterns),
		"similar_errors", len(classification.SimilarErrors))

	return classification, nil
}

// comprehensiveAnalysis performs in-depth error pattern analysis
func (r *AdvancedPatternRecognizer) comprehensiveAnalysis(ctx context.Context, err error, context WorkflowContext) (*EnhancedErrorClassification, error) {
	// Start with basic classification
	basicClassification, classifyErr := r.ClassifyError(ctx, err, context)
	if classifyErr != nil {
		return nil, classifyErr
	}

	// Create enhanced classification
	enhanced := &EnhancedErrorClassification{
		ErrorClassification: *basicClassification,
		PatternScore:        0.0,
		SimilarErrors:       []SimilarError{},
		TrendAnalysis:       TrendAnalysis{},
		RecommendedActions:  []RecommendedAction{},
		LearningInsights:    []LearningInsight{},
	}

	// Analyze error patterns
	r.analyzePatterns(enhanced, err, context)

	// Find similar errors with advanced matching
	r.findSimilarErrors(enhanced, err, context)

	// Perform trend analysis
	r.analyzeTrends(enhanced, err, context)

	// Generate recommendations
	r.generateRecommendations(enhanced, err, context)

	// Extract learning insights
	r.extractLearningInsights(enhanced, err, context)

	return enhanced, nil
}

// analyzePatterns performs advanced pattern analysis
func (r *AdvancedPatternRecognizer) analyzePatterns(classification *EnhancedErrorClassification, err error, context WorkflowContext) {
	if err == nil {
		r.logger.Warn("analyzePatterns called with nil error")
		return
	}

	errorMsg := strings.ToLower(err.Error())

	// Extract technical patterns
	technicalPatterns := r.extractTechnicalPatterns(errorMsg)
	classification.Patterns = append(classification.Patterns, technicalPatterns...)

	// Calculate pattern confidence score
	patternScore := r.calculatePatternScore(errorMsg, context)
	classification.PatternScore = patternScore

	// Adjust confidence based on pattern matching
	if patternScore > 0.8 {
		classification.Confidence = math.Min(classification.Confidence+0.2, 1.0)
	}

	r.logger.Debug("Pattern analysis completed",
		"technical_patterns", len(technicalPatterns),
		"pattern_score", patternScore,
		"adjusted_confidence", classification.Confidence)
}

// extractTechnicalPatterns extracts technical error patterns
func (r *AdvancedPatternRecognizer) extractTechnicalPatterns(errorMsg string) []string {
	patterns := []string{}

	// Network patterns
	networkPatterns := []string{
		"connection refused", "timeout", "network unreachable", "dns resolution failed",
		"tls handshake failed", "certificate invalid", "proxy error",
	}

	// Docker patterns
	dockerPatterns := []string{
		"no such file or directory", "permission denied", "invalid instruction",
		"failed to build", "image not found", "registry authentication failed",
	}

	// Kubernetes patterns
	k8sPatterns := []string{
		"pod failed", "deployment failed", "service unavailable", "cluster unreachable",
		"rbac error", "resource quota exceeded", "node not ready",
	}

	// Build patterns
	buildPatterns := []string{
		"compilation error", "dependency not found", "version conflict",
		"missing package", "syntax error", "import error",
	}

	allPatterns := map[string][]string{
		"network": networkPatterns,
		"docker":  dockerPatterns,
		"k8s":     k8sPatterns,
		"build":   buildPatterns,
	}

	for category, categoryPatterns := range allPatterns {
		for _, pattern := range categoryPatterns {
			if strings.Contains(errorMsg, pattern) {
				patterns = append(patterns, fmt.Sprintf("%s:%s", category, pattern))
			}
		}
	}

	return patterns
}

// calculatePatternScore calculates how well the error matches known patterns
func (r *AdvancedPatternRecognizer) calculatePatternScore(errorMsg string, context WorkflowContext) float64 {
	// Get historical patterns for this step
	stepPatterns := r.learningEngine.GetStepPatterns(context.StepName)

	if len(stepPatterns) == 0 {
		return 0.5 // Default score when no historical data
	}

	maxScore := 0.0
	for _, pattern := range stepPatterns {
		score := r.similarityEngine.CalculateStringSimilarity(errorMsg, pattern.Pattern)
		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// findSimilarErrors finds similar errors with advanced matching
func (r *AdvancedPatternRecognizer) findSimilarErrors(classification *EnhancedErrorClassification, err error, context WorkflowContext) {
	if err == nil {
		return
	}

	// Get similar errors from history
	historicalErrors := r.errorHistory.FindSimilarErrors(err, context)

	// Convert to enhanced format with similarity scores
	for _, historical := range historicalErrors {
		similarity := r.similarityEngine.CalculateErrorSimilarity(err.Error(), historical.Error)

		if similarity > 0.6 { // Higher threshold for enhanced matching
			similarError := SimilarError{
				Error:      historical.Error,
				Similarity: similarity,
				Context:    historical.StepName,
				Timestamp:  historical.Timestamp,
				Resolved:   historical.Resolved,
			}

			if historical.Classification != nil {
				similarError.Category = string(historical.Classification.Category)
				similarError.Solution = historical.Classification.SuggestedFix
			}

			classification.SimilarErrors = append(classification.SimilarErrors, similarError)
		}
	}

	// Sort by similarity score
	sort.Slice(classification.SimilarErrors, func(i, j int) bool {
		return classification.SimilarErrors[i].Similarity > classification.SimilarErrors[j].Similarity
	})

	// Keep only top 5 most similar
	if len(classification.SimilarErrors) > 5 {
		classification.SimilarErrors = classification.SimilarErrors[:5]
	}
}

// analyzeTrends analyzes error trends for this workflow step
func (r *AdvancedPatternRecognizer) analyzeTrends(classification *EnhancedErrorClassification, err error, context WorkflowContext) {
	if err == nil {
		return
	}

	trends := r.learningEngine.AnalyzeTrends(context.StepName, err.Error())

	classification.TrendAnalysis = TrendAnalysis{
		Frequency:             trends.Frequency,
		RecentIncrease:        trends.RecentIncrease,
		SeasonalPattern:       trends.SeasonalPattern,
		AverageResolutionTime: trends.AverageResolutionTime,
		SuccessRate:           trends.SuccessRate,
	}
}

// generateRecommendations generates AI-powered recommendations
func (r *AdvancedPatternRecognizer) generateRecommendations(classification *EnhancedErrorClassification, err error, context WorkflowContext) {
	if err == nil {
		return
	}

	recommendations := []RecommendedAction{}

	// Generate recommendations based on classification
	switch classification.Category {
	case CategoryNetwork:
		recommendations = append(recommendations, RecommendedAction{
			Action:      "retry_with_backoff",
			Priority:    "high",
			Confidence:  0.9,
			Description: "Retry with exponential backoff for network issues",
			Command:     "sleep 30 && retry",
		})

	case CategoryDockerfile:
		recommendations = append(recommendations, RecommendedAction{
			Action:      "validate_dockerfile",
			Priority:    "high",
			Confidence:  0.8,
			Description: "Validate Dockerfile syntax and dependencies",
			Command:     "docker build --dry-run .",
		})

	case CategoryKubernetes:
		recommendations = append(recommendations, RecommendedAction{
			Action:      "check_cluster_status",
			Priority:    "medium",
			Confidence:  0.7,
			Description: "Verify cluster connectivity and permissions",
			Command:     "kubectl cluster-info",
		})

	case CategoryRegistry:
		recommendations = append(recommendations, RecommendedAction{
			Action:      "check_registry_auth",
			Priority:    "high",
			Confidence:  0.9,
			Description: "Verify registry authentication and permissions",
			Command:     "docker login",
		})

	case CategoryBuild:
		recommendations = append(recommendations, RecommendedAction{
			Action:      "retry_build",
			Priority:    "medium",
			Confidence:  0.8,
			Description: "Retry build process with clean state",
			Command:     "docker build --no-cache",
		})

	default:
		// Generic recommendation for unknown categories
		recommendations = append(recommendations, RecommendedAction{
			Action:      "investigate_logs",
			Priority:    "low",
			Confidence:  0.5,
			Description: "Review logs and check system status",
			Command:     "",
		})
	}

	// Add recommendations based on similar errors
	for _, similar := range classification.SimilarErrors {
		if similar.Resolved && similar.Solution != "" {
			recommendations = append(recommendations, RecommendedAction{
				Action:      "apply_historical_fix",
				Priority:    "medium",
				Confidence:  similar.Similarity,
				Description: fmt.Sprintf("Apply solution from similar error: %s", similar.Solution),
				Command:     "",
			})
		}
	}

	// Add learning-based recommendations
	learningRecommendations := r.learningEngine.GetRecommendations(context.StepName, err.Error())
	for _, rec := range learningRecommendations {
		recommendations = append(recommendations, RecommendedAction{
			Action:      rec.Action,
			Priority:    rec.Priority,
			Confidence:  rec.Confidence,
			Description: rec.Description,
			Command:     rec.Command,
		})
	}

	// Sort by priority and confidence
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Priority != recommendations[j].Priority {
			// High > Medium > Low
			priorityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
			return priorityOrder[recommendations[i].Priority] > priorityOrder[recommendations[j].Priority]
		}
		return recommendations[i].Confidence > recommendations[j].Confidence
	})

	classification.RecommendedActions = recommendations
}

// extractLearningInsights extracts insights for continuous learning
func (r *AdvancedPatternRecognizer) extractLearningInsights(classification *EnhancedErrorClassification, err error, context WorkflowContext) {
	if err == nil {
		return
	}

	insights := []LearningInsight{}

	// Analyze pattern frequency
	patternFreq := r.learningEngine.GetPatternFrequency(context.StepName)
	if patternFreq > 0.1 { // If this pattern occurs in >10% of cases
		insights = append(insights, LearningInsight{
			Type:        "pattern_frequency",
			Description: fmt.Sprintf("This error pattern occurs in %.1f%% of %s step failures", patternFreq*100, context.StepName),
			Confidence:  0.8,
			Actionable:  true,
		})
	}

	// Analyze resolution patterns
	if len(classification.SimilarErrors) > 0 {
		resolvedCount := 0
		for _, similar := range classification.SimilarErrors {
			if similar.Resolved {
				resolvedCount++
			}
		}

		if resolvedCount > 0 {
			resolutionRate := float64(resolvedCount) / float64(len(classification.SimilarErrors))
			insights = append(insights, LearningInsight{
				Type:        "resolution_rate",
				Description: fmt.Sprintf("Similar errors have %.1f%% resolution rate", resolutionRate*100),
				Confidence:  0.9,
				Actionable:  true,
			})
		}
	}

	// Time-based insights
	if classification.TrendAnalysis.RecentIncrease {
		insights = append(insights, LearningInsight{
			Type:        "trend_alert",
			Description: "This error type has increased in frequency recently",
			Confidence:  0.7,
			Actionable:  true,
		})
	}

	classification.LearningInsights = insights
}

// GetPatternStatistics returns comprehensive pattern statistics
func (r *AdvancedPatternRecognizer) GetPatternStatistics(ctx context.Context) (*PatternStatistics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	baseStats := r.errorHistory.GetErrorStatistics()

	stats := &PatternStatistics{
		ErrorStatistics: baseStats,
		CacheHitRate:    r.patternCache.GetHitRate(),
		TopPatterns:     r.errorHistory.GetTopPatterns(10),
		LearningMetrics: r.learningEngine.GetMetrics(),
	}

	return stats, nil
}

// UpdatePatternDatabase updates the pattern database with new training data
func (r *AdvancedPatternRecognizer) UpdatePatternDatabase(ctx context.Context, trainingData []PatternTrainingData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, data := range trainingData {
		// Update learning engine with new data
		r.learningEngine.UpdatePattern(data)

		// Clear cache for affected patterns
		r.patternCache.ClearPattern(data.ErrorPattern)
	}

	r.logger.Info("Pattern database updated",
		"training_records", len(trainingData))

	return nil
}

// Supporting types for advanced pattern recognition

// EnhancedErrorClassification extends basic classification with advanced features
type EnhancedErrorClassification struct {
	ErrorClassification
	PatternScore       float64             `json:"pattern_score"`
	SimilarErrors      []SimilarError      `json:"similar_errors"`
	TrendAnalysis      TrendAnalysis       `json:"trend_analysis"`
	RecommendedActions []RecommendedAction `json:"recommended_actions"`
	LearningInsights   []LearningInsight   `json:"learning_insights"`
}

// SimilarError represents a similar error with enhanced metadata
type SimilarError struct {
	Error      string    `json:"error"`
	Similarity float64   `json:"similarity"`
	Context    string    `json:"context"`
	Category   string    `json:"category"`
	Solution   string    `json:"solution"`
	Timestamp  time.Time `json:"timestamp"`
	Resolved   bool      `json:"resolved"`
}

// TrendAnalysis provides trend analysis for error patterns
type TrendAnalysis struct {
	Frequency             float64       `json:"frequency"`
	RecentIncrease        bool          `json:"recent_increase"`
	SeasonalPattern       bool          `json:"seasonal_pattern"`
	AverageResolutionTime time.Duration `json:"average_resolution_time"`
	SuccessRate           float64       `json:"success_rate"`
}

// RecommendedAction represents an AI-generated recommendation
type RecommendedAction struct {
	Action      string  `json:"action"`
	Priority    string  `json:"priority"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Command     string  `json:"command,omitempty"`
}

// LearningInsight represents insights from pattern learning
type LearningInsight struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	Actionable  bool    `json:"actionable"`
}

// PatternStatistics provides comprehensive pattern statistics
type PatternStatistics struct {
	ErrorStatistics
	CacheHitRate    float64            `json:"cache_hit_rate"`
	TopPatterns     []PatternFrequency `json:"top_patterns"`
	LearningMetrics LearningMetrics    `json:"learning_metrics"`
}

// PatternTrainingData represents training data for pattern learning
type PatternTrainingData struct {
	ErrorPattern string                 `json:"error_pattern"`
	Context      WorkflowContext        `json:"context"`
	Resolution   string                 `json:"resolution"`
	Success      bool                   `json:"success"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// LearningMetrics provides metrics about the learning system
type LearningMetrics struct {
	TotalPatterns   int     `json:"total_patterns"`
	AccuracyRate    float64 `json:"accuracy_rate"`
	LearningRate    float64 `json:"learning_rate"`
	PatternsCovered int     `json:"patterns_covered"`
	PredictionsMade int     `json:"predictions_made"`
	SuccessfulFixes int     `json:"successful_fixes"`
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Domain interface implementation methods

// RecognizePattern implements domainml.ErrorPatternRecognizer interface
func (r *AdvancedPatternRecognizer) RecognizePattern(ctx context.Context, err error, stepContext *workflow.WorkflowState) (*domainml.ErrorClassification, error) {
	// Convert WorkflowState to our internal WorkflowContext
	stepName := "unknown"
	repoURL := ""
	branch := ""

	if stepContext.Args != nil {
		repoURL = stepContext.Args.RepoURL
		branch = stepContext.Args.Branch
	}

	// Try to determine step name from current step number
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	if stepContext.CurrentStep > 0 && stepContext.CurrentStep <= len(stepNames) {
		stepName = stepNames[stepContext.CurrentStep-1]
	}

	context := WorkflowContext{
		WorkflowID: stepContext.WorkflowID,
		StepName:   stepName,
		StepNumber: stepContext.CurrentStep,
		TotalSteps: stepContext.TotalSteps,
		RepoURL:    repoURL,
		Branch:     branch,
	}

	// Use our advanced analysis
	classification, classifyErr := r.AnalyzeErrorPatterns(ctx, err, context)
	if classifyErr != nil {
		return nil, classifyErr
	}

	// Convert to domain ErrorClassification
	suggestions := []string{}
	if classification.SuggestedFix != "" {
		suggestions = append(suggestions, classification.SuggestedFix)
	}

	// Add recommended actions as suggestions
	for _, action := range classification.RecommendedActions {
		if action.Description != "" {
			suggestions = append(suggestions, action.Description)
		}
	}

	return &domainml.ErrorClassification{
		Category:    classification.ErrorType,
		Confidence:  classification.Confidence,
		Patterns:    classification.Patterns,
		Suggestions: suggestions,
		Metadata: map[string]interface{}{
			"severity":          string(classification.Severity),
			"auto_fixable":      classification.AutoFixable,
			"retry_strategy":    string(classification.RetryRecommendation),
			"category":          string(classification.Category),
			"pattern_score":     classification.PatternScore,
			"similar_errors":    len(classification.SimilarErrors),
			"learning_insights": len(classification.LearningInsights),
			"trend_analysis":    classification.TrendAnalysis,
		},
	}, nil
}

// GetSimilarErrors implements domainml.ErrorPatternRecognizer interface
func (r *AdvancedPatternRecognizer) GetSimilarErrors(ctx context.Context, err error) ([]domainml.HistoricalError, error) {
	// Get similar errors from our error history
	similarErrors := r.errorHistory.FindSimilarErrors(err, WorkflowContext{})

	// Convert to domain HistoricalError format
	var result []domainml.HistoricalError
	for _, similar := range similarErrors {
		solutions := []string{}
		if similar.Classification != nil && similar.Classification.SuggestedFix != "" {
			solutions = append(solutions, similar.Classification.SuggestedFix)
		}

		// Use similarity engine to calculate proper similarity score
		similarity := r.similarityEngine.CalculateErrorSimilarity(err.Error(), similar.Error)

		result = append(result, domainml.HistoricalError{
			Error:      similar.Error,
			Context:    similar.StepName,
			Solutions:  solutions,
			Similarity: similarity,
			Timestamp:  similar.Timestamp.Format("2006-01-02T15:04:05Z"),
		})
	}

	return result, nil
}
