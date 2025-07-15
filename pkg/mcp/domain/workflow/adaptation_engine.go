// Package workflow provides adaptation engine functionality for adaptive workflows
package workflow

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// findMatchingStrategy finds a successful adaptation strategy for a given error pattern
func (e *AdaptationEngine) findMatchingStrategy(category, errorMessage string) *AdaptationStrategy {
	// Calculate similarity scores for all strategies
	var candidates []strategyCandidate

	for _, strategy := range e.successfulAdaptations {
		similarity := e.calculateSimilarity(category, errorMessage, strategy)
		if similarity > 0.7 { // Minimum similarity threshold
			candidates = append(candidates, strategyCandidate{
				strategy:   strategy,
				similarity: similarity,
			})
		}
	}

	// Sort by similarity and success rate
	sort.Slice(candidates, func(i, j int) bool {
		// Primary sort by similarity
		if candidates[i].similarity != candidates[j].similarity {
			return candidates[i].similarity > candidates[j].similarity
		}
		// Secondary sort by success rate
		return candidates[i].strategy.SuccessRate > candidates[j].strategy.SuccessRate
	})

	// Return the best matching strategy
	if len(candidates) > 0 {
		strategy := candidates[0].strategy
		e.logger.Info("Found matching adaptation strategy",
			"strategy_id", strategy.PatternID,
			"similarity", candidates[0].similarity,
			"success_rate", strategy.SuccessRate,
			"usage_count", strategy.UsageCount)
		return strategy
	}

	return nil
}

// calculateSimilarity calculates similarity between an error and a strategy
func (e *AdaptationEngine) calculateSimilarity(category, errorMessage string, strategy *AdaptationStrategy) float64 {
	// Category match is important
	categoryMatch := 0.0
	if strings.Contains(strategy.PatternID, category) {
		categoryMatch = 0.4
	}

	// String similarity for error message
	errorSimilarity := e.calculateStringSimilarity(errorMessage, strategy.ErrorPattern)

	// Recency bonus (more recent strategies are preferred)
	recencyBonus := 0.0
	if time.Since(strategy.LastUsed) < 24*time.Hour {
		recencyBonus = 0.1
	}

	// Success rate bonus
	successBonus := strategy.SuccessRate * 0.2

	// Usage count bonus (strategies that have been used successfully multiple times)
	usageBonus := 0.0
	if strategy.UsageCount > 1 {
		usageBonus = 0.1
	}

	return categoryMatch + (errorSimilarity * 0.4) + recencyBonus + successBonus + usageBonus
}

// calculateStringSimilarity calculates similarity between two strings using simple metrics
func (e *AdaptationEngine) calculateStringSimilarity(str1, str2 string) float64 {
	// Convert to lowercase for comparison
	s1 := strings.ToLower(str1)
	s2 := strings.ToLower(str2)

	if s1 == s2 {
		return 1.0
	}

	// Calculate word overlap
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Find common words
	commonWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 2 { // Only count words longer than 2 chars
				commonWords++
				break
			}
		}
	}

	// Calculate Jaccard similarity
	totalWords := len(words1) + len(words2) - commonWords
	if totalWords == 0 {
		return 1.0
	}

	return float64(commonWords) / float64(totalWords)
}

// storeSuccessfulStrategy stores a successful adaptation strategy for future use
func (e *AdaptationEngine) storeSuccessfulStrategy(category, errorPattern string, adaptations []AdaptationEvent) {
	// Create a unique pattern ID
	patternID := fmt.Sprintf("%s_%s_%d", category, e.hashString(errorPattern), time.Now().Unix())

	// Determine the primary step name from adaptations
	stepName := "unknown"
	if len(adaptations) > 0 {
		stepName = adaptations[0].StepName
	}

	strategy := &AdaptationStrategy{
		PatternID:    patternID,
		StepName:     stepName,
		ErrorPattern: errorPattern,
		Adaptations:  adaptations,
		SuccessRate:  1.0, // Initial success rate
		UsageCount:   1,
		LastUsed:     time.Now(),
		Confidence:   0.8, // Initial confidence
		Metadata: map[string]interface{}{
			"category":    category,
			"created_at":  time.Now(),
			"adaptations": len(adaptations),
		},
	}

	e.successfulAdaptations[patternID] = strategy

	e.logger.Info("Stored successful adaptation strategy",
		"pattern_id", patternID,
		"step_name", stepName,
		"category", category,
		"adaptations", len(adaptations))
}

// learnFromExecution learns from a completed workflow execution
func (e *AdaptationEngine) learnFromExecution(record *AdaptationRecord) {
	e.logger.Info("Learning from workflow execution",
		"workflow_id", record.WorkflowID,
		"success_rate", record.SuccessRate,
		"adaptations", len(record.Adaptations))

	// Update success rates for any strategies that were used
	for i, adaptation := range record.Adaptations {
		if adaptation.Success {
			// Find and update the strategy if it exists
			for _, strategy := range e.successfulAdaptations {
				if e.adaptationMatches(adaptation, strategy) {
					e.updateStrategySuccessRate(strategy, true)
					break
				}
			}
		} else {
			// Update failure statistics
			for _, strategy := range e.successfulAdaptations {
				if e.adaptationMatches(adaptation, strategy) {
					e.updateStrategySuccessRate(strategy, false)
					break
				}
			}
		}

		// Log adaptation result
		e.logger.Info("Adaptation result",
			"workflow_id", record.WorkflowID,
			"adaptation_index", i,
			"type", adaptation.AdaptationType,
			"step", adaptation.StepName,
			"success", adaptation.Success,
			"execution_time", adaptation.ExecutionTime)
	}

	// Learn patterns from failed adaptations
	if record.SuccessRate < 1.0 {
		e.learnFromFailures(record)
	}

	// Clean up old strategies periodically
	e.cleanupOldStrategies()
}

// adaptationMatches checks if an adaptation event matches a strategy
func (e *AdaptationEngine) adaptationMatches(adaptation AdaptationEvent, strategy *AdaptationStrategy) bool {
	// Check if the adaptation is similar to any in the strategy
	for _, strategyAdaptation := range strategy.Adaptations {
		if adaptation.AdaptationType == strategyAdaptation.AdaptationType &&
			adaptation.StepName == strategyAdaptation.StepName {
			return true
		}
	}
	return false
}

// updateStrategySuccessRate updates the success rate of a strategy
func (e *AdaptationEngine) updateStrategySuccessRate(strategy *AdaptationStrategy, success bool) {
	oldCount := strategy.UsageCount
	strategy.UsageCount++
	strategy.LastUsed = time.Now()

	if success {
		// Update success rate using moving average
		strategy.SuccessRate = ((strategy.SuccessRate * float64(oldCount)) + 1.0) / float64(strategy.UsageCount)
	} else {
		// Decrease success rate
		strategy.SuccessRate = (strategy.SuccessRate * float64(oldCount)) / float64(strategy.UsageCount)
	}

	// Update confidence based on success rate and usage count
	strategy.Confidence = strategy.SuccessRate * (1.0 - (1.0 / float64(strategy.UsageCount+1)))

	e.logger.Info("Updated strategy statistics",
		"pattern_id", strategy.PatternID,
		"success_rate", strategy.SuccessRate,
		"usage_count", strategy.UsageCount,
		"confidence", strategy.Confidence)
}

// learnFromFailures analyzes failed adaptations to improve future strategies
func (e *AdaptationEngine) learnFromFailures(record *AdaptationRecord) {
	e.logger.Info("Analyzing failed adaptations for learning",
		"workflow_id", record.WorkflowID,
		"failed_adaptations", len(record.Adaptations))

	// Analyze what went wrong
	for _, adaptation := range record.Adaptations {
		if !adaptation.Success {
			e.logger.Info("Failed adaptation analysis",
				"workflow_id", record.WorkflowID,
				"type", adaptation.AdaptationType,
				"step", adaptation.StepName,
				"reason", adaptation.Reason,
				"confidence", adaptation.Confidence)

			// Lower confidence for similar strategies
			e.adjustConfidenceForSimilarStrategies(adaptation)
		}
	}
}

// adjustConfidenceForSimilarStrategies adjusts confidence for strategies similar to failed adaptations
func (e *AdaptationEngine) adjustConfidenceForSimilarStrategies(failedAdaptation AdaptationEvent) {
	for _, strategy := range e.successfulAdaptations {
		for _, strategyAdaptation := range strategy.Adaptations {
			if strategyAdaptation.AdaptationType == failedAdaptation.AdaptationType &&
				strategyAdaptation.StepName == failedAdaptation.StepName {

				// Reduce confidence slightly
				strategy.Confidence = strategy.Confidence * 0.95

				e.logger.Info("Reduced strategy confidence due to similar failure",
					"pattern_id", strategy.PatternID,
					"new_confidence", strategy.Confidence)
			}
		}
	}
}

// cleanupOldStrategies removes old, unused strategies
func (e *AdaptationEngine) cleanupOldStrategies() {
	cutoffTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago

	for patternID, strategy := range e.successfulAdaptations {
		// Remove strategies that haven't been used in 30 days and have low confidence
		if strategy.LastUsed.Before(cutoffTime) && strategy.Confidence < 0.3 {
			delete(e.successfulAdaptations, patternID)
			e.logger.Info("Cleaned up old strategy",
				"pattern_id", patternID,
				"last_used", strategy.LastUsed,
				"confidence", strategy.Confidence)
		}
	}
}

// hashString creates a simple hash of a string for pattern IDs
func (e *AdaptationEngine) hashString(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)[:8]
}

// GetAdaptationStatistics returns statistics about the adaptation engine
func (e *AdaptationEngine) GetAdaptationStatistics() *AdaptationStatistics {
	totalStrategies := len(e.successfulAdaptations)
	totalExecutions := len(e.adaptationHistory)

	// Calculate average success rate
	totalSuccessRate := 0.0
	for _, strategy := range e.successfulAdaptations {
		totalSuccessRate += strategy.SuccessRate
	}

	avgSuccessRate := 0.0
	if totalStrategies > 0 {
		avgSuccessRate = totalSuccessRate / float64(totalStrategies)
	}

	// Calculate strategy distribution by type
	strategyDistribution := make(map[AdaptationType]int)
	for _, strategy := range e.successfulAdaptations {
		for _, adaptation := range strategy.Adaptations {
			strategyDistribution[adaptation.AdaptationType]++
		}
	}

	return &AdaptationStatistics{
		TotalStrategies:      totalStrategies,
		TotalExecutions:      totalExecutions,
		AverageSuccessRate:   avgSuccessRate,
		StrategyDistribution: strategyDistribution,
		LastUpdated:          time.Now(),
	}
}

// Supporting types

// strategyCandidate represents a strategy candidate with similarity score
type strategyCandidate struct {
	strategy   *AdaptationStrategy
	similarity float64
}

// AdaptationStatistics provides statistics about the adaptation engine
type AdaptationStatistics struct {
	TotalStrategies      int                    `json:"total_strategies"`
	TotalExecutions      int                    `json:"total_executions"`
	AverageSuccessRate   float64                `json:"average_success_rate"`
	StrategyDistribution map[AdaptationType]int `json:"strategy_distribution"`
	LastUpdated          time.Time              `json:"last_updated"`
}
