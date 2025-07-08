package knowledge

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"log/slog"
)

// CrossToolKnowledgeBase stores and manages insights from all tools
type CrossToolKnowledgeBase struct {
	insights  map[string][]*ToolInsights   // domain -> insights
	patterns  map[string][]*FailurePattern // failure_type -> patterns
	knowledge map[string]*SharedKnowledge  // domain -> accumulated knowledge
	mutex     sync.RWMutex
	logger    *slog.Logger
}

// NewCrossToolKnowledgeBase creates a new knowledge base
func NewCrossToolKnowledgeBase(logger *slog.Logger) *CrossToolKnowledgeBase {
	return &CrossToolKnowledgeBase{
		insights:  make(map[string][]*ToolInsights),
		patterns:  make(map[string][]*FailurePattern),
		knowledge: make(map[string]*SharedKnowledge),
		logger:    logger.With("component", "cross_tool_knowledge_base"),
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

	kb.logger.Debug("Stored tool insights",
		"domain", domain,
		"tool", insights.ToolName)

	return nil
}

// GetRelatedFailures finds failures similar to the given request
func (kb *CrossToolKnowledgeBase) GetRelatedFailures(ctx context.Context, request *AnalysisRequest) ([]*RelatedFailure, error) {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()

	relatedFailures := []*RelatedFailure{}

	if request.Error == nil {
		return relatedFailures, nil
	}

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

// GetSharedKnowledge returns accumulated knowledge for a domain
func (kb *CrossToolKnowledgeBase) GetSharedKnowledge(domain string) *SharedKnowledge {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()

	return kb.knowledge[domain]
}

// GetFailurePatterns returns all known failure patterns
func (kb *CrossToolKnowledgeBase) GetFailurePatterns(failureType string) []*FailurePattern {
	kb.mutex.RLock()
	defer kb.mutex.RUnlock()

	if failureType == "" {
		// Return all patterns
		allPatterns := []*FailurePattern{}
		for _, patterns := range kb.patterns {
			allPatterns = append(allPatterns, patterns...)
		}
		return allPatterns
	}

	return kb.patterns[failureType]
}

// updateKnowledge updates accumulated knowledge from new insights
func (kb *CrossToolKnowledgeBase) updateKnowledge(domain string, insights *ToolInsights) {
	if kb.knowledge[domain] == nil {
		kb.knowledge[domain] = &SharedKnowledge{
			Domain:           domain,
			CommonPatterns:   []interface{}{},
			BestPractices:    []interface{}{},
			OptimizationTips: []interface{}{},
			SuccessMetrics:   make(map[string]interface{}),
			SourceTools:      []string{},
			Data:             make(map[string]interface{}),
		}
	}

	knowledge := kb.knowledge[domain]

	// Add optimization tips
	for _, tip := range insights.OptimizationTips {
		knowledge.OptimizationTips = append(knowledge.OptimizationTips, tip)
	}

	// Update source tools
	toolFound := false
	for _, tool := range knowledge.SourceTools {
		if tool == insights.ToolName {
			toolFound = true
			break
		}
	}
	if !toolFound {
		knowledge.SourceTools = append(knowledge.SourceTools, insights.ToolName)
	}

	// Update metrics
	if insights.PerformanceMetrics != nil {
		knowledge.SuccessMetrics["success_rate"] = insights.PerformanceMetrics.SuccessRate
		knowledge.SuccessMetrics["avg_duration"] = insights.PerformanceMetrics.AvgDuration
	}

	knowledge.LastUpdated = time.Now()
}

// calculateSimilarity calculates similarity between error and pattern
func (kb *CrossToolKnowledgeBase) calculateSimilarity(errorStr string, pattern *FailurePattern) float64 {
	// Simple similarity calculation based on string matching
	errorLower := strings.ToLower(errorStr)
	patternLower := strings.ToLower(pattern.Pattern)

	// Check if pattern is contained in error
	if strings.Contains(errorLower, patternLower) {
		return 0.8
	}

	// Check for common words
	errorWords := strings.Fields(errorLower)
	patternWords := strings.Fields(patternLower)

	commonWords := 0
	for _, ew := range errorWords {
		for _, pw := range patternWords {
			if ew == pw && len(ew) > 3 { // Ignore short words
				commonWords++
			}
		}
	}

	if len(patternWords) == 0 {
		return 0.0
	}

	return float64(commonWords) / float64(len(patternWords))
}

// extractResolution extracts resolution from failure pattern
func (kb *CrossToolKnowledgeBase) extractResolution(pattern *FailurePattern) string {
	if len(pattern.TypicalSolutions) > 0 {
		return strings.Join(pattern.TypicalSolutions, "; ")
	}
	return "No known resolution"
}
