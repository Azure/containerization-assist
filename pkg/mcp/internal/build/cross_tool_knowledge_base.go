package build

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// CrossToolKnowledgeBase stores and manages insights from all tools
type CrossToolKnowledgeBase struct {
	insights  map[string][]*ToolInsights   // domain -> insights
	patterns  map[string][]*FailurePattern // failure_type -> patterns
	knowledge map[string]*SharedKnowledge  // domain -> accumulated knowledge
	mutex     sync.RWMutex
	logger    zerolog.Logger
}

// NewCrossToolKnowledgeBase creates a new knowledge base
func NewCrossToolKnowledgeBase(logger zerolog.Logger) *CrossToolKnowledgeBase {
	return &CrossToolKnowledgeBase{
		insights:  make(map[string][]*ToolInsights),
		patterns:  make(map[string][]*FailurePattern),
		knowledge: make(map[string]*SharedKnowledge),
		logger:    logger.With().Str("component", "cross_tool_knowledge_base").Logger(),
	}
}

// StoreInsights stores tool insights for future analysis
func (kb *CrossToolKnowledgeBase) StoreInsights(ctx context.Context, insights *ToolInsights) error {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()
	domain := insights.ToolName
	// Add to insights collection
	if kb.insights[domain] == nil {
		kb.insights[domain] = []*ToolInsights{}
	}
	kb.insights[domain] = append(kb.insights[domain], insights)
	// Update failure patterns if this is a failure insight
	if insights.FailurePattern != nil {
		failureType := insights.FailurePattern.FailureType
		if kb.patterns[failureType] == nil {
			kb.patterns[failureType] = []*FailurePattern{}
		}
		// Check if pattern already exists and update frequency
		updated := false
		for _, pattern := range kb.patterns[failureType] {
			if pattern.PatternName == insights.FailurePattern.PatternName {
				pattern.Frequency += insights.FailurePattern.Frequency
				updated = true
				break
			}
		}
		if !updated {
			kb.patterns[failureType] = append(kb.patterns[failureType], insights.FailurePattern)
		}
	}
	// Update accumulated knowledge
	kb.updateKnowledge(domain, insights)
	kb.logger.Debug().
		Str("domain", domain).
		Str("tool", insights.ToolName).
		Msg("Stored tool insights")
	return nil
}

// GetRelatedFailures finds failures similar to the given request
func (kb *CrossToolKnowledgeBase) GetRelatedFailures(ctx context.Context, request *AnalysisRequest) ([]*RelatedFailure, error) {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	relatedFailures := []*RelatedFailure{}
	requestErrorStr := request.Error.Error()
	// Search through all stored insights for similar failures
	for domain, insights := range kb.insights {
		for _, insight := range insights {
			if insight.FailurePattern != nil {
				similarity := kb.calculateSimilarity(requestErrorStr, insight.FailurePattern)
				if similarity > 0.3 { // Minimum similarity threshold
					relatedFailure := &RelatedFailure{
						FailureType:  insight.FailurePattern.FailureType,
						Similarity:   similarity,
						Resolution:   kb.extractResolution(insight.FailurePattern),
						LastOccurred: insight.Timestamp,
						Frequency:    insight.FailurePattern.Frequency,
					}
					relatedFailures = append(relatedFailures, relatedFailure)
				}
			}
		}
		_ = domain // Avoid unused variable warning
	}
	// Sort by similarity (highest first)
	sort.Slice(relatedFailures, func(i, j int) bool {
		return relatedFailures[i].Similarity > relatedFailures[j].Similarity
	})
	// Limit to top 10 most similar failures
	if len(relatedFailures) > 10 {
		relatedFailures = relatedFailures[:10]
	}
	return relatedFailures, nil
}

// GetKnowledge retrieves accumulated knowledge for a domain
func (kb *CrossToolKnowledgeBase) GetKnowledge(ctx context.Context, domain string) (*SharedKnowledge, error) {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	knowledge, exists := kb.knowledge[domain]
	if !exists {
		// Return empty knowledge if domain not found
		return &SharedKnowledge{
			Domain:           domain,
			CommonPatterns:   []*FailurePattern{},
			BestPractices:    []string{},
			OptimizationTips: []*GeneralOptimizationTip{},
			SuccessMetrics:   &AggregatedMetrics{},
			LastUpdated:      time.Now(),
			SourceTools:      []string{},
		}, nil
	}
	// Return a copy to avoid mutation
	return &SharedKnowledge{
		Domain:           knowledge.Domain,
		CommonPatterns:   knowledge.CommonPatterns,
		BestPractices:    knowledge.BestPractices,
		OptimizationTips: knowledge.OptimizationTips,
		SuccessMetrics:   knowledge.SuccessMetrics,
		LastUpdated:      knowledge.LastUpdated,
		SourceTools:      knowledge.SourceTools,
	}, nil
}

// GetDomainInsights retrieves all insights for a specific domain
func (kb *CrossToolKnowledgeBase) GetDomainInsights(domain string) []*ToolInsights {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	insights, exists := kb.insights[domain]
	if !exists {
		return []*ToolInsights{}
	}
	// Return a copy to avoid mutation
	result := make([]*ToolInsights, len(insights))
	copy(result, insights)
	return result
}

// GetFailurePatterns retrieves failure patterns for a specific failure type
func (kb *CrossToolKnowledgeBase) GetFailurePatterns(failureType string) []*FailurePattern {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	patterns, exists := kb.patterns[failureType]
	if !exists {
		return []*FailurePattern{}
	}
	// Return a copy to avoid mutation
	result := make([]*FailurePattern, len(patterns))
	copy(result, patterns)
	return result
}

// Private helper methods
func (kb *CrossToolKnowledgeBase) updateKnowledge(domain string, insights *ToolInsights) {
	if kb.knowledge[domain] == nil {
		kb.knowledge[domain] = &SharedKnowledge{
			Domain:           domain,
			CommonPatterns:   []*FailurePattern{},
			BestPractices:    []string{},
			OptimizationTips: []*GeneralOptimizationTip{},
			SuccessMetrics: &AggregatedMetrics{
				TotalOperations:  0,
				SuccessRate:      0.0,
				AverageTime:      0,
				ImprovementTrend: 0.0,
				CommonIssueTypes: []string{},
			},
			LastUpdated: time.Now(),
			SourceTools: []string{},
		}
	}
	knowledge := kb.knowledge[domain]
	// Update source tools
	if !kb.contains(knowledge.SourceTools, insights.ToolName) {
		knowledge.SourceTools = append(knowledge.SourceTools, insights.ToolName)
	}
	// Update common patterns
	if insights.FailurePattern != nil {
		knowledge.CommonPatterns = kb.updatePatterns(knowledge.CommonPatterns, insights.FailurePattern)
	}
	// Update optimization tips
	for _, tip := range insights.OptimizationTips {
		knowledge.OptimizationTips = append(knowledge.OptimizationTips, &GeneralOptimizationTip{
			Category:      "tool_insight",
			Tip:           tip,
			Impact:        "medium",
			Difficulty:    "low",
			Applicability: 0.7,
		})
	}
	// Update success metrics
	if insights.PerformanceMetrics != nil {
		kb.updateSuccessMetrics(knowledge.SuccessMetrics, insights)
	}
	knowledge.LastUpdated = time.Now()
}
func (kb *CrossToolKnowledgeBase) updatePatterns(patterns []*FailurePattern, newPattern *FailurePattern) []*FailurePattern {
	// Check if pattern already exists
	for _, pattern := range patterns {
		if pattern.PatternName == newPattern.PatternName {
			pattern.Frequency += newPattern.Frequency
			return patterns
		}
	}
	// Add new pattern
	return append(patterns, newPattern)
}
func (kb *CrossToolKnowledgeBase) updateSuccessMetrics(metrics *AggregatedMetrics, insights *ToolInsights) {
	metrics.TotalOperations++
	// Update average time
	if insights.PerformanceMetrics != nil {
		currentTotal := metrics.AverageTime * time.Duration(metrics.TotalOperations-1)
		newTotal := currentTotal + insights.PerformanceMetrics.TotalDuration
		metrics.AverageTime = newTotal / time.Duration(metrics.TotalOperations)
	}
	// Update success rate (placeholder logic)
	if insights.SuccessPattern != nil {
		successOps := int(metrics.SuccessRate * float64(metrics.TotalOperations-1))
		successOps++ // This operation was successful
		metrics.SuccessRate = float64(successOps) / float64(metrics.TotalOperations)
	}
	// Update common issue types
	if insights.FailurePattern != nil {
		if !kb.contains(metrics.CommonIssueTypes, insights.FailurePattern.FailureType) {
			metrics.CommonIssueTypes = append(metrics.CommonIssueTypes, insights.FailurePattern.FailureType)
		}
	}
}
func (kb *CrossToolKnowledgeBase) calculateSimilarity(errorStr string, pattern *FailurePattern) float64 {
	similarity := 0.0
	// Simple similarity calculation based on string matching
	// In a real implementation, this could use more sophisticated NLP techniques
	// Check failure type match
	if pattern.FailureType != "" {
		if containsIgnoreCase(errorStr, pattern.FailureType) {
			similarity += 0.4
		}
	}
	// Check common causes
	for _, cause := range pattern.CommonCauses {
		if containsIgnoreCase(errorStr, cause) {
			similarity += 0.2 / float64(len(pattern.CommonCauses))
		}
	}
	// Check pattern name
	if containsIgnoreCase(errorStr, pattern.PatternName) {
		similarity += 0.3
	}
	// Boost similarity for frequent patterns
	if pattern.Frequency > 5 {
		similarity += 0.1
	}
	return similarity
}
func (kb *CrossToolKnowledgeBase) extractResolution(pattern *FailurePattern) string {
	if len(pattern.TypicalSolutions) > 0 {
		return pattern.TypicalSolutions[0]
	}
	return "No specific resolution available"
}
func (kb *CrossToolKnowledgeBase) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
func containsIgnoreCase(str, substr string) bool {
	return len(str) >= len(substr) &&
		strings.ToLower(str)[0:len(substr)] == strings.ToLower(substr) ||
		strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// Advanced methods for knowledge base management
// GetInsightStatistics returns statistics about stored insights
func (kb *CrossToolKnowledgeBase) GetInsightStatistics() map[string]interface{} {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	stats := map[string]interface{}{
		"total_domains":          len(kb.insights),
		"total_insights":         0,
		"total_patterns":         0,
		"domains_with_knowledge": len(kb.knowledge),
	}
	for _, insights := range kb.insights {
		stats["total_insights"] = stats["total_insights"].(int) + len(insights)
	}
	for _, patterns := range kb.patterns {
		stats["total_patterns"] = stats["total_patterns"].(int) + len(patterns)
	}
	return stats
}

// CleanupOldInsights removes insights older than the specified duration
func (kb *CrossToolKnowledgeBase) CleanupOldInsights(maxAge time.Duration) int {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for domain, insights := range kb.insights {
		filtered := []*ToolInsights{}
		for _, insight := range insights {
			if insight.Timestamp.After(cutoff) {
				filtered = append(filtered, insight)
			} else {
				removed++
			}
		}
		kb.insights[domain] = filtered
	}
	kb.logger.Info().
		Int("removed", removed).
		Dur("max_age", maxAge).
		Msg("Cleaned up old insights")
	return removed
}

// Export returns all knowledge base data for backup/analysis
func (kb *CrossToolKnowledgeBase) Export() map[string]interface{} {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()
	return map[string]interface{}{
		"insights":    kb.insights,
		"patterns":    kb.patterns,
		"knowledge":   kb.knowledge,
		"exported_at": time.Now(),
	}
}
