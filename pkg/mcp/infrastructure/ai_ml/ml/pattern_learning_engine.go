// Package ml provides machine learning capabilities for error pattern recognition
package ml

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// PatternLearningEngine learns from error patterns and provides intelligent recommendations
type PatternLearningEngine struct {
	patterns         map[string]*PatternData
	stepPatterns     map[string][]StepPattern
	trends           map[string]*TrendData
	recommendations  map[string][]LearningRecommendation
	mu               sync.RWMutex
	totalPredictions int
	successfulFixes  int
}

// PatternData represents learned data about an error pattern
type PatternData struct {
	Pattern        string                 `json:"pattern"`
	Frequency      int                    `json:"frequency"`
	SuccessRate    float64                `json:"success_rate"`
	AverageResTime time.Duration          `json:"average_resolution_time"`
	Resolutions    []string               `json:"resolutions"`
	Contexts       []WorkflowContext      `json:"contexts"`
	LastSeen       time.Time              `json:"last_seen"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// StepPattern represents a pattern specific to a workflow step
type StepPattern struct {
	Pattern    string    `json:"pattern"`
	Frequency  int       `json:"frequency"`
	StepName   string    `json:"step_name"`
	LastSeen   time.Time `json:"last_seen"`
	Success    bool      `json:"success"`
	Resolution string    `json:"resolution"`
}

// TrendData represents trend information for error patterns
type TrendData struct {
	Pattern               string        `json:"pattern"`
	RecentOccurrences     []time.Time   `json:"recent_occurrences"`
	MovingAverage         float64       `json:"moving_average"`
	Trend                 string        `json:"trend"` // "increasing", "decreasing", "stable"
	SeasonalPattern       bool          `json:"seasonal_pattern"`
	AverageResolutionTime time.Duration `json:"average_resolution_time"`
	SuccessRate           float64       `json:"success_rate"`
}

// LearningRecommendation represents a machine learning-generated recommendation
type LearningRecommendation struct {
	Action      string  `json:"action"`
	Priority    string  `json:"priority"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Command     string  `json:"command"`
	BasedOn     string  `json:"based_on"`
}

// TrendAnalysisResult represents the result of trend analysis
type TrendAnalysisResult struct {
	Frequency             float64       `json:"frequency"`
	RecentIncrease        bool          `json:"recent_increase"`
	SeasonalPattern       bool          `json:"seasonal_pattern"`
	AverageResolutionTime time.Duration `json:"average_resolution_time"`
	SuccessRate           float64       `json:"success_rate"`
}

// NewPatternLearningEngine creates a new pattern learning engine
func NewPatternLearningEngine() *PatternLearningEngine {
	return &PatternLearningEngine{
		patterns:        make(map[string]*PatternData),
		stepPatterns:    make(map[string][]StepPattern),
		trends:          make(map[string]*TrendData),
		recommendations: make(map[string][]LearningRecommendation),
	}
}

// Learn processes new error data and updates the learning models
func (e *PatternLearningEngine) Learn(err error, context WorkflowContext, classification *EnhancedErrorClassification) {
	e.mu.Lock()
	defer e.mu.Unlock()

	errorPattern := err.Error()

	// Update pattern data
	e.updatePatternData(errorPattern, context, classification)

	// Update step-specific patterns
	e.updateStepPatterns(errorPattern, context, classification)

	// Update trend data
	e.updateTrendData(errorPattern, context, classification)

	// Generate new recommendations based on learning
	e.generateLearningRecommendations(errorPattern, context, classification)
}

// updatePatternData updates the pattern data with new information
func (e *PatternLearningEngine) updatePatternData(errorPattern string, context WorkflowContext, classification *EnhancedErrorClassification) {
	pattern, exists := e.patterns[errorPattern]
	if !exists {
		pattern = &PatternData{
			Pattern:     errorPattern,
			Frequency:   0,
			SuccessRate: 0.0,
			Resolutions: []string{},
			Contexts:    []WorkflowContext{},
			Metadata:    make(map[string]interface{}),
		}
		e.patterns[errorPattern] = pattern
	}

	pattern.Frequency++
	pattern.LastSeen = time.Now()
	pattern.Contexts = append(pattern.Contexts, context)

	// Add resolution if suggested
	if classification.SuggestedFix != "" {
		pattern.Resolutions = append(pattern.Resolutions, classification.SuggestedFix)
	}

	// Update metadata
	pattern.Metadata["category"] = string(classification.Category)
	pattern.Metadata["severity"] = string(classification.Severity)
	pattern.Metadata["auto_fixable"] = classification.AutoFixable
}

// updateStepPatterns updates step-specific pattern data
func (e *PatternLearningEngine) updateStepPatterns(errorPattern string, context WorkflowContext, classification *EnhancedErrorClassification) {
	stepName := context.StepName

	// Find existing step pattern or create new one
	stepPatterns := e.stepPatterns[stepName]
	found := false

	for i, sp := range stepPatterns {
		if sp.Pattern == errorPattern {
			stepPatterns[i].Frequency++
			stepPatterns[i].LastSeen = time.Now()
			if classification.SuggestedFix != "" {
				stepPatterns[i].Resolution = classification.SuggestedFix
			}
			found = true
			break
		}
	}

	if !found {
		newPattern := StepPattern{
			Pattern:    errorPattern,
			Frequency:  1,
			StepName:   stepName,
			LastSeen:   time.Now(),
			Success:    false,
			Resolution: classification.SuggestedFix,
		}
		stepPatterns = append(stepPatterns, newPattern)
	}

	e.stepPatterns[stepName] = stepPatterns
}

// updateTrendData updates trend analysis data
func (e *PatternLearningEngine) updateTrendData(errorPattern string, context WorkflowContext, classification *EnhancedErrorClassification) {
	trend, exists := e.trends[errorPattern]
	if !exists {
		trend = &TrendData{
			Pattern:           errorPattern,
			RecentOccurrences: []time.Time{},
			MovingAverage:     0.0,
			Trend:             "stable",
			SeasonalPattern:   false,
		}
		e.trends[errorPattern] = trend
	}

	// Add current occurrence
	now := time.Now()
	trend.RecentOccurrences = append(trend.RecentOccurrences, now)

	// Keep only last 30 days
	cutoff := now.Add(-30 * 24 * time.Hour)
	filtered := []time.Time{}
	for _, occurrence := range trend.RecentOccurrences {
		if occurrence.After(cutoff) {
			filtered = append(filtered, occurrence)
		}
	}
	trend.RecentOccurrences = filtered

	// Calculate moving average and trend
	e.calculateTrend(trend)
}

// calculateTrend calculates trend information for a pattern
func (e *PatternLearningEngine) calculateTrend(trend *TrendData) {
	if len(trend.RecentOccurrences) < 2 {
		// Set a basic average for single occurrences
		if len(trend.RecentOccurrences) == 1 {
			trend.MovingAverage = 1.0
		}
		return
	}

	// Calculate daily averages for the last 30 days
	dailyCounts := make(map[string]int)
	for _, occurrence := range trend.RecentOccurrences {
		day := occurrence.Format("2006-01-02")
		dailyCounts[day]++
	}

	// Calculate moving average
	totalDays := len(dailyCounts)
	if totalDays > 0 {
		totalOccurrences := len(trend.RecentOccurrences)
		trend.MovingAverage = float64(totalOccurrences) / float64(totalDays)
	}

	// Determine trend direction
	if len(trend.RecentOccurrences) >= 7 {
		recent := trend.RecentOccurrences[len(trend.RecentOccurrences)-7:]
		older := trend.RecentOccurrences[:len(trend.RecentOccurrences)-7]

		recentRate := float64(len(recent)) / 7.0
		olderRate := float64(len(older)) / float64(len(older))

		if recentRate > olderRate*1.2 {
			trend.Trend = "increasing"
		} else if recentRate < olderRate*0.8 {
			trend.Trend = "decreasing"
		} else {
			trend.Trend = "stable"
		}
	}
}

// generateLearningRecommendations generates recommendations based on learned patterns
func (e *PatternLearningEngine) generateLearningRecommendations(errorPattern string, context WorkflowContext, classification *EnhancedErrorClassification) {
	stepName := context.StepName
	recommendations := []LearningRecommendation{}

	// Get historical success data for this pattern
	if patternData, exists := e.patterns[errorPattern]; exists {
		// If we have successful resolutions, recommend them
		if len(patternData.Resolutions) > 0 {
			// Get most common resolution
			resolutionCounts := make(map[string]int)
			for _, resolution := range patternData.Resolutions {
				resolutionCounts[resolution]++
			}

			// Find most common resolution
			maxCount := 0
			mostCommon := ""
			for resolution, count := range resolutionCounts {
				if count > maxCount {
					maxCount = count
					mostCommon = resolution
				}
			}

			if mostCommon != "" {
				confidence := float64(maxCount) / float64(len(patternData.Resolutions))
				recommendations = append(recommendations, LearningRecommendation{
					Action:      "apply_learned_fix",
					Priority:    "high",
					Confidence:  confidence,
					Description: mostCommon,
					Command:     "",
					BasedOn:     "historical_success",
				})
			}
		}
	}

	// Get step-specific recommendations
	if stepPatterns, exists := e.stepPatterns[stepName]; exists {
		for _, pattern := range stepPatterns {
			if pattern.Success && pattern.Resolution != "" {
				// Calculate confidence based on frequency and recency
				confidence := 0.5 + (float64(pattern.Frequency) / 10.0) // Base confidence + frequency bonus
				if time.Since(pattern.LastSeen) < 7*24*time.Hour {
					confidence += 0.2 // Recent success bonus
				}
				if confidence > 1.0 {
					confidence = 1.0
				}

				recommendations = append(recommendations, LearningRecommendation{
					Action:      "apply_step_specific_fix",
					Priority:    "medium",
					Confidence:  confidence,
					Description: pattern.Resolution,
					Command:     "",
					BasedOn:     "step_pattern_success",
				})
			}
		}
	}

	// Trend-based recommendations
	if trendData, exists := e.trends[errorPattern]; exists {
		if trendData.Trend == "increasing" {
			recommendations = append(recommendations, LearningRecommendation{
				Action:      "escalate_priority",
				Priority:    "high",
				Confidence:  0.8,
				Description: "This error is increasing in frequency - consider priority escalation",
				Command:     "",
				BasedOn:     "trend_analysis",
			})
		}
	}

	// Sort recommendations by priority and confidence
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Priority != recommendations[j].Priority {
			priorityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
			return priorityOrder[recommendations[i].Priority] > priorityOrder[recommendations[j].Priority]
		}
		return recommendations[i].Confidence > recommendations[j].Confidence
	})

	e.recommendations[stepName] = recommendations
}

// GetStepPatterns returns patterns for a specific step
func (e *PatternLearningEngine) GetStepPatterns(stepName string) []StepPattern {
	e.mu.RLock()
	defer e.mu.RUnlock()

	patterns, exists := e.stepPatterns[stepName]
	if !exists {
		return []StepPattern{}
	}

	return patterns
}

// GetPatternFrequency returns the frequency of a pattern for a specific step
func (e *PatternLearningEngine) GetPatternFrequency(stepName string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	patterns, exists := e.stepPatterns[stepName]
	if !exists {
		return 0.0
	}

	totalFrequency := 0
	for _, pattern := range patterns {
		totalFrequency += pattern.Frequency
	}

	if totalFrequency == 0 {
		return 0.0
	}

	// Return average frequency
	return float64(totalFrequency) / float64(len(patterns))
}

// AnalyzeTrends analyzes trends for a specific step and error
func (e *PatternLearningEngine) AnalyzeTrends(stepName string, errorPattern string) TrendAnalysisResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := TrendAnalysisResult{
		Frequency:             0.0,
		RecentIncrease:        false,
		SeasonalPattern:       false,
		AverageResolutionTime: 0,
		SuccessRate:           0.0,
	}

	// Get trend data
	if trendData, exists := e.trends[errorPattern]; exists {
		result.Frequency = trendData.MovingAverage
		result.RecentIncrease = trendData.Trend == "increasing"
		result.SeasonalPattern = trendData.SeasonalPattern
		result.AverageResolutionTime = trendData.AverageResolutionTime
		result.SuccessRate = trendData.SuccessRate
	}

	// Get step-specific data
	if stepPatterns, exists := e.stepPatterns[stepName]; exists {
		for _, pattern := range stepPatterns {
			if strings.Contains(pattern.Pattern, errorPattern) {
				result.Frequency = float64(pattern.Frequency)
				break
			}
		}
	}

	return result
}

// GetRecommendations returns recommendations for a specific step and error
func (e *PatternLearningEngine) GetRecommendations(stepName string, errorPattern string) []LearningRecommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	recommendations, exists := e.recommendations[stepName]
	if !exists {
		return []LearningRecommendation{}
	}

	// Filter recommendations relevant to this error pattern
	relevant := []LearningRecommendation{}
	for _, rec := range recommendations {
		if strings.Contains(strings.ToLower(rec.Description), strings.ToLower(errorPattern)) ||
			strings.Contains(strings.ToLower(errorPattern), strings.ToLower(rec.Description)) {
			relevant = append(relevant, rec)
		}
	}

	return relevant
}

// UpdatePattern updates a pattern with new training data
func (e *PatternLearningEngine) UpdatePattern(data PatternTrainingData) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.totalPredictions++
	if data.Success {
		e.successfulFixes++
	}

	// Update pattern data
	pattern, exists := e.patterns[data.ErrorPattern]
	if !exists {
		pattern = &PatternData{
			Pattern:     data.ErrorPattern,
			Frequency:   0,
			SuccessRate: 0.0,
			Resolutions: []string{},
			Contexts:    []WorkflowContext{},
			Metadata:    make(map[string]interface{}),
		}
		e.patterns[data.ErrorPattern] = pattern
	}

	pattern.Frequency++
	pattern.LastSeen = time.Now()
	pattern.Contexts = append(pattern.Contexts, data.Context)

	if data.Success && data.Resolution != "" {
		pattern.Resolutions = append(pattern.Resolutions, data.Resolution)
	}

	// Update success rate
	successCount := 0
	for _, resolution := range pattern.Resolutions {
		if resolution != "" {
			successCount++
		}
	}
	pattern.SuccessRate = float64(successCount) / float64(pattern.Frequency)
}

// GetMetrics returns learning metrics
func (e *PatternLearningEngine) GetMetrics() LearningMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	accuracyRate := 0.0
	if e.totalPredictions > 0 {
		accuracyRate = float64(e.successfulFixes) / float64(e.totalPredictions)
	}

	return LearningMetrics{
		TotalPatterns:   len(e.patterns),
		AccuracyRate:    accuracyRate,
		LearningRate:    0.1, // Could be dynamic based on recent performance
		PatternsCovered: len(e.stepPatterns),
		PredictionsMade: e.totalPredictions,
		SuccessfulFixes: e.successfulFixes,
	}
}
