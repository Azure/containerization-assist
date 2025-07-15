// Package errors provides error aggregation and reporting utilities
package errors

import (
	"sync"
	"time"
)

// ErrorAggregator collects and analyzes errors for reporting and monitoring
type ErrorAggregator struct {
	mu     sync.RWMutex
	errors []*StructuredError

	// Metrics
	totalCount     int64
	categoryCount  map[ErrorCategory]int64
	severityCount  map[ErrorSeverity]int64
	componentCount map[string]int64

	// Time-based analysis
	startTime time.Time
	window    time.Duration
}

// NewErrorAggregator creates a new error aggregator
func NewErrorAggregator(window time.Duration) *ErrorAggregator {
	return &ErrorAggregator{
		errors:         make([]*StructuredError, 0),
		categoryCount:  make(map[ErrorCategory]int64),
		severityCount:  make(map[ErrorSeverity]int64),
		componentCount: make(map[string]int64),
		startTime:      time.Now(),
		window:         window,
	}
}

// Add adds an error to the aggregator
func (ea *ErrorAggregator) Add(err *StructuredError) {
	if err == nil {
		return
	}

	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Clean up old errors outside the window
	ea.cleanupOldErrors()

	// Add the new error
	ea.errors = append(ea.errors, err)
	ea.totalCount++
	ea.categoryCount[err.Category]++
	ea.severityCount[err.Severity]++
	ea.componentCount[err.Component]++
}

// AddFromError converts and adds any error type
func (ea *ErrorAggregator) AddFromError(err error, operation, component string) {
	if err == nil {
		return
	}

	// If it's already structured, add directly
	if structErr, ok := err.(*StructuredError); ok {
		ea.Add(structErr)
		return
	}

	// Otherwise wrap it
	structErr := Wrap(err, operation, component, err.Error())
	ea.Add(structErr)
}

// GetReport generates a comprehensive error report
func (ea *ErrorAggregator) GetReport() *ErrorReport {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	ea.cleanupOldErrors()

	report := &ErrorReport{
		TotalErrors: ea.totalCount,
		WindowStart: ea.startTime,
		WindowEnd:   time.Now(),
		Duration:    time.Since(ea.startTime),
		Categories:  make(map[ErrorCategory]CategoryStats),
		Severities:  make(map[ErrorSeverity]SeverityStats),
		Components:  make(map[string]ComponentStats),
		TopErrors:   ea.getTopErrors(10),
		Patterns:    ea.analyzePatterns(),
		Trends:      ea.analyzeTrends(),
	}

	// Calculate category statistics
	for category, count := range ea.categoryCount {
		errorsList := ea.getErrorsByCategory(category)
		report.Categories[category] = CategoryStats{
			Count:           count,
			RecoverableRate: ea.calculateRecoverableRate(errorsList),
			AvgSeverity:     ea.calculateAvgSeverity(errorsList),
			LastOccurrence:  ea.getLastOccurrence(errorsList),
		}
	}

	// Calculate severity statistics
	for severity, count := range ea.severityCount {
		errorsList := ea.getErrorsBySeverity(severity)
		report.Severities[severity] = SeverityStats{
			Count:          count,
			Percentage:     float64(count) / float64(ea.totalCount) * 100,
			LastOccurrence: ea.getLastOccurrence(errorsList),
		}
	}

	// Calculate component statistics
	for component, count := range ea.componentCount {
		errorsList := ea.getErrorsByComponent(component)
		report.Components[component] = ComponentStats{
			Count:              count,
			MostCommonCategory: ea.getMostCommonCategory(errorsList),
			ErrorRate:          float64(count) / float64(ea.totalCount) * 100,
			LastOccurrence:     ea.getLastOccurrence(errorsList),
		}
	}

	return report
}

// ErrorReport represents a comprehensive error analysis report
type ErrorReport struct {
	TotalErrors int64                           `json:"total_errors"`
	WindowStart time.Time                       `json:"window_start"`
	WindowEnd   time.Time                       `json:"window_end"`
	Duration    time.Duration                   `json:"duration"`
	Categories  map[ErrorCategory]CategoryStats `json:"categories"`
	Severities  map[ErrorSeverity]SeverityStats `json:"severities"`
	Components  map[string]ComponentStats       `json:"components"`
	TopErrors   []*StructuredError              `json:"top_errors"`
	Patterns    []PatternAnalysis               `json:"patterns"`
	Trends      TrendAnalysis                   `json:"trends"`
}

// CategoryStats provides statistics for an error category
type CategoryStats struct {
	Count           int64         `json:"count"`
	RecoverableRate float64       `json:"recoverable_rate"`
	AvgSeverity     ErrorSeverity `json:"avg_severity"`
	LastOccurrence  time.Time     `json:"last_occurrence"`
}

// SeverityStats provides statistics for an error severity
type SeverityStats struct {
	Count          int64     `json:"count"`
	Percentage     float64   `json:"percentage"`
	LastOccurrence time.Time `json:"last_occurrence"`
}

// ComponentStats provides statistics for a component
type ComponentStats struct {
	Count              int64         `json:"count"`
	MostCommonCategory ErrorCategory `json:"most_common_category"`
	ErrorRate          float64       `json:"error_rate"`
	LastOccurrence     time.Time     `json:"last_occurrence"`
}

// PatternAnalysis identifies common error patterns
type PatternAnalysis struct {
	Pattern    string   `json:"pattern"`
	Count      int64    `json:"count"`
	Percentage float64  `json:"percentage"`
	Examples   []string `json:"examples"`
}

// TrendAnalysis provides trend information
type TrendAnalysis struct {
	ErrorsPerHour  float64                  `json:"errors_per_hour"`
	PeakHour       time.Time                `json:"peak_hour"`
	TrendDirection string                   `json:"trend_direction"` // "increasing", "decreasing", "stable"
	SeverityTrend  map[ErrorSeverity]string `json:"severity_trend"`
	CategoryTrend  map[ErrorCategory]string `json:"category_trend"`
}

// GetCriticalErrors returns all critical errors
func (ea *ErrorAggregator) GetCriticalErrors() []*StructuredError {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	var critical []*StructuredError
	for _, err := range ea.errors {
		if err.Severity == SeverityCritical {
			critical = append(critical, err)
		}
	}
	return critical
}

// GetRecoverableErrors returns all recoverable errors
func (ea *ErrorAggregator) GetRecoverableErrors() []*StructuredError {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	var recoverable []*StructuredError
	for _, err := range ea.errors {
		if err.Recoverable {
			recoverable = append(recoverable, err)
		}
	}
	return recoverable
}

// GetErrorsByCategory returns all errors of a specific category
func (ea *ErrorAggregator) getErrorsByCategory(category ErrorCategory) []*StructuredError {
	var errors []*StructuredError
	for _, err := range ea.errors {
		if err.Category == category {
			errors = append(errors, err)
		}
	}
	return errors
}

// GetErrorsBySeverity returns all errors of a specific severity
func (ea *ErrorAggregator) getErrorsBySeverity(severity ErrorSeverity) []*StructuredError {
	var errors []*StructuredError
	for _, err := range ea.errors {
		if err.Severity == severity {
			errors = append(errors, err)
		}
	}
	return errors
}

// GetErrorsByComponent returns all errors from a specific component
func (ea *ErrorAggregator) getErrorsByComponent(component string) []*StructuredError {
	var errors []*StructuredError
	for _, err := range ea.errors {
		if err.Component == component {
			errors = append(errors, err)
		}
	}
	return errors
}

// Private helper methods

func (ea *ErrorAggregator) cleanupOldErrors() {
	if ea.window == 0 {
		return // No time window limit
	}

	cutoff := time.Now().Add(-ea.window)
	var filtered []*StructuredError

	for _, err := range ea.errors {
		if err.Timestamp.After(cutoff) {
			filtered = append(filtered, err)
		}
	}

	ea.errors = filtered
}

func (ea *ErrorAggregator) getTopErrors(limit int) []*StructuredError {
	if len(ea.errors) <= limit {
		return ea.errors
	}

	// Sort by severity (critical first) and timestamp (recent first)
	sorted := make([]*StructuredError, len(ea.errors))
	copy(sorted, ea.errors)

	// Simple sort by severity priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if severityPriority(sorted[i].Severity) > severityPriority(sorted[j].Severity) ||
				(sorted[i].Severity == sorted[j].Severity && sorted[i].Timestamp.Before(sorted[j].Timestamp)) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted[:limit]
}

func (ea *ErrorAggregator) analyzePatterns() []PatternAnalysis {
	patterns := make(map[string]int64)
	examples := make(map[string][]string)

	for _, err := range ea.errors {
		// Extract patterns from error messages
		summary := SummarizeError(err, 1)
		for _, pattern := range summary.Patterns {
			patterns[pattern]++
			if len(examples[pattern]) < 3 {
				examples[pattern] = append(examples[pattern], err.Message)
			}
		}
	}

	var result []PatternAnalysis
	for pattern, count := range patterns {
		result = append(result, PatternAnalysis{
			Pattern:    pattern,
			Count:      count,
			Percentage: float64(count) / float64(ea.totalCount) * 100,
			Examples:   examples[pattern],
		})
	}

	return result
}

func (ea *ErrorAggregator) analyzeTrends() TrendAnalysis {
	if len(ea.errors) == 0 {
		return TrendAnalysis{}
	}

	duration := time.Since(ea.startTime)
	hoursElapsed := duration.Hours()
	if hoursElapsed == 0 {
		hoursElapsed = 1 // Avoid division by zero
	}

	return TrendAnalysis{
		ErrorsPerHour:  float64(ea.totalCount) / hoursElapsed,
		PeakHour:       ea.findPeakHour(),
		TrendDirection: "stable", // Simplified - would need historical data for real trend analysis
		SeverityTrend:  make(map[ErrorSeverity]string),
		CategoryTrend:  make(map[ErrorCategory]string),
	}
}

func (ea *ErrorAggregator) calculateRecoverableRate(errors []*StructuredError) float64 {
	if len(errors) == 0 {
		return 0
	}

	recoverable := 0
	for _, err := range errors {
		if err.Recoverable {
			recoverable++
		}
	}

	return float64(recoverable) / float64(len(errors)) * 100
}

func (ea *ErrorAggregator) calculateAvgSeverity(errors []*StructuredError) ErrorSeverity {
	if len(errors) == 0 {
		return SeverityInfo
	}

	severitySum := 0
	for _, err := range errors {
		severitySum += severityPriority(err.Severity)
	}

	avgPriority := severitySum / len(errors)
	return priorityToSeverity(avgPriority)
}

func (ea *ErrorAggregator) getLastOccurrence(errors []*StructuredError) time.Time {
	if len(errors) == 0 {
		return time.Time{}
	}

	latest := errors[0].Timestamp
	for _, err := range errors {
		if err.Timestamp.After(latest) {
			latest = err.Timestamp
		}
	}

	return latest
}

func (ea *ErrorAggregator) getMostCommonCategory(errors []*StructuredError) ErrorCategory {
	if len(errors) == 0 {
		return CategoryInfrastructure
	}

	categoryCount := make(map[ErrorCategory]int)
	for _, err := range errors {
		categoryCount[err.Category]++
	}

	var mostCommon ErrorCategory
	maxCount := 0
	for category, count := range categoryCount {
		if count > maxCount {
			maxCount = count
			mostCommon = category
		}
	}

	return mostCommon
}

func (ea *ErrorAggregator) findPeakHour() time.Time {
	// Simplified - just return the hour with the most recent error
	if len(ea.errors) == 0 {
		return time.Time{}
	}

	return ea.errors[len(ea.errors)-1].Timestamp
}

// Utility functions

func severityPriority(severity ErrorSeverity) int {
	switch severity {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

func priorityToSeverity(priority int) ErrorSeverity {
	switch priority {
	case 5:
		return SeverityCritical
	case 4:
		return SeverityHigh
	case 3:
		return SeverityMedium
	case 2:
		return SeverityLow
	case 1:
		return SeverityInfo
	default:
		return SeverityMedium
	}
}

// Reset clears all accumulated errors and resets metrics
func (ea *ErrorAggregator) Reset() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.errors = make([]*StructuredError, 0)
	ea.totalCount = 0
	ea.categoryCount = make(map[ErrorCategory]int64)
	ea.severityCount = make(map[ErrorSeverity]int64)
	ea.componentCount = make(map[string]int64)
	ea.startTime = time.Now()
}
