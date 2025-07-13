// Package ml provides error history tracking for pattern recognition.
package ml

import (
	"strings"
	"sync"
	"time"
)

// ErrorHistoryEntry represents a historical error occurrence
type ErrorHistoryEntry struct {
	Timestamp      time.Time            `json:"timestamp"`
	Error          string               `json:"error"`
	WorkflowID     string               `json:"workflow_id"`
	StepName       string               `json:"step_name"`
	RepoURL        string               `json:"repo_url"`
	Language       string               `json:"language,omitempty"`
	Framework      string               `json:"framework,omitempty"`
	Classification *ErrorClassification `json:"classification"`
	Resolved       bool                 `json:"resolved"`
	ResolutionTime *time.Time           `json:"resolution_time,omitempty"`
}

// ErrorHistoryStore manages historical error data for pattern learning
type ErrorHistoryStore struct {
	entries []ErrorHistoryEntry
	mutex   sync.RWMutex
	maxSize int
}

// NewErrorHistoryStore creates a new error history store
func NewErrorHistoryStore() *ErrorHistoryStore {
	return &ErrorHistoryStore{
		entries: make([]ErrorHistoryEntry, 0),
		maxSize: 1000, // Keep last 1000 errors
	}
}

// RecordError stores an error and its classification in history
func (s *ErrorHistoryStore) RecordError(err error, context WorkflowContext, classification *ErrorClassification) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	entry := ErrorHistoryEntry{
		Timestamp:      time.Now(),
		Error:          err.Error(),
		WorkflowID:     context.WorkflowID,
		StepName:       context.StepName,
		RepoURL:        context.RepoURL,
		Language:       context.Language,
		Framework:      context.Framework,
		Classification: classification,
		Resolved:       false,
	}

	s.entries = append(s.entries, entry)

	// Maintain max size by removing oldest entries
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[len(s.entries)-s.maxSize:]
	}
}

// MarkResolved marks an error as resolved (useful for learning successful resolutions)
func (s *ErrorHistoryStore) MarkResolved(workflowID string, stepName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := &s.entries[i]
		if entry.WorkflowID == workflowID && entry.StepName == stepName && !entry.Resolved {
			entry.Resolved = true
			entry.ResolutionTime = &now
			break
		}
	}
}

// FindSimilarErrors finds errors similar to the given error and context
func (s *ErrorHistoryStore) FindSimilarErrors(err error, context WorkflowContext) []ErrorHistoryEntry {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var similar []ErrorHistoryEntry
	errorMsg := strings.ToLower(err.Error())

	for _, entry := range s.entries {
		similarity := s.calculateSimilarity(errorMsg, entry, context)
		if similarity > 0.7 { // 70% similarity threshold
			similar = append(similar, entry)
		}
	}

	return similar
}

// GetErrorStatistics returns statistics about error patterns
func (s *ErrorHistoryStore) GetErrorStatistics() ErrorStatistics {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := ErrorStatistics{
		TotalErrors:     len(s.entries),
		ResolvedErrors:  0,
		CategoryCounts:  make(map[ErrorCategory]int),
		StepCounts:      make(map[string]int),
		LanguageCounts:  make(map[string]int),
		ResolutionTimes: make(map[string]time.Duration),
	}

	for _, entry := range s.entries {
		if entry.Resolved {
			stats.ResolvedErrors++
			if entry.ResolutionTime != nil {
				duration := entry.ResolutionTime.Sub(entry.Timestamp)
				stats.ResolutionTimes[entry.StepName] = duration
			}
		}

		if entry.Classification != nil {
			stats.CategoryCounts[entry.Classification.Category]++
		}

		stats.StepCounts[entry.StepName]++

		if entry.Language != "" {
			stats.LanguageCounts[entry.Language]++
		}
	}

	if stats.TotalErrors > 0 {
		stats.ResolutionRate = float64(stats.ResolvedErrors) / float64(stats.TotalErrors)
	}

	return stats
}

// GetTopPatterns returns the most common error patterns
func (s *ErrorHistoryStore) GetTopPatterns(limit int) []PatternFrequency {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	patternCounts := make(map[string]int)

	for _, entry := range s.entries {
		if entry.Classification != nil {
			for _, pattern := range entry.Classification.Patterns {
				patternCounts[pattern]++
			}
		}
	}

	// Convert to sorted slice
	patterns := make([]PatternFrequency, 0, len(patternCounts))
	for pattern, count := range patternCounts {
		patterns = append(patterns, PatternFrequency{
			Pattern: pattern,
			Count:   count,
		})
	}

	// Sort by frequency (simple bubble sort for small datasets)
	for i := 0; i < len(patterns); i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[j].Count > patterns[i].Count {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}

	// Return top N patterns
	if limit > 0 && limit < len(patterns) {
		patterns = patterns[:limit]
	}

	return patterns
}

// calculateSimilarity calculates similarity between errors (0.0 to 1.0)
func (s *ErrorHistoryStore) calculateSimilarity(errorMsg string, entry ErrorHistoryEntry, context WorkflowContext) float64 {
	score := 0.0
	maxScore := 4.0 // Total possible score

	// Error message similarity (most important)
	if s.messagesAreSimilar(errorMsg, strings.ToLower(entry.Error)) {
		score += 2.0
	}

	// Step name match
	if entry.StepName == context.StepName {
		score += 1.0
	}

	// Language match
	if entry.Language != "" && entry.Language == context.Language {
		score += 0.5
	}

	// Framework match
	if entry.Framework != "" && entry.Framework == context.Framework {
		score += 0.5
	}

	return score / maxScore
}

// messagesAreSimilar checks if two error messages are similar using simple heuristics
func (s *ErrorHistoryStore) messagesAreSimilar(msg1, msg2 string) bool {
	// Simple similarity check - can be enhanced with more sophisticated algorithms

	// Exact match
	if msg1 == msg2 {
		return true
	}

	// Check for common substrings
	words1 := strings.Fields(msg1)
	words2 := strings.Fields(msg2)

	if len(words1) == 0 || len(words2) == 0 {
		return false
	}

	commonWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 3 { // Only count words longer than 3 chars
				commonWords++
				break
			}
		}
	}

	// Consider similar if >50% of words match
	similarity := float64(commonWords) / float64(min(len(words1), len(words2)))
	return similarity > 0.5
}

// Supporting types

// ErrorStatistics provides insights into error patterns
type ErrorStatistics struct {
	TotalErrors     int                      `json:"total_errors"`
	ResolvedErrors  int                      `json:"resolved_errors"`
	ResolutionRate  float64                  `json:"resolution_rate"`
	CategoryCounts  map[ErrorCategory]int    `json:"category_counts"`
	StepCounts      map[string]int           `json:"step_counts"`
	LanguageCounts  map[string]int           `json:"language_counts"`
	ResolutionTimes map[string]time.Duration `json:"resolution_times"`
}

// PatternFrequency represents how often a pattern occurs
type PatternFrequency struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

// Helper function for minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
