package build

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// FailurePredictor predicts potential build failures based on context and history
type FailurePredictor struct {
	patternAnalyzer *PatternAnalyzer
	riskCalculator  *RiskCalculator
	logger          zerolog.Logger
}

// NewFailurePredictor creates a new failure predictor
func NewFailurePredictor(logger zerolog.Logger) *FailurePredictor {
	return &FailurePredictor{
		patternAnalyzer: NewPatternAnalyzer(logger),
		riskCalculator:  NewRiskCalculator(logger),
		logger:          logger.With().Str("component", "failure_predictor").Logger(),
	}
}

// PredictFailures predicts potential failures for a build context
func (fp *FailurePredictor) PredictFailures(ctx context.Context, buildContext *AnalysisBuildContext) (*FailurePrediction, error) {
	fp.logger.Info().
		Str("session_id", buildContext.SessionID).
		Str("project_type", buildContext.ProjectInfo.Language).
		Msg("Starting failure prediction analysis")
	// Analyze historical patterns
	patterns, err := fp.patternAnalyzer.AnalyzeHistoricalPatterns(buildContext.BuildHistory)
	if err != nil {
		fp.logger.Warn().Err(err).Msg("Failed to analyze historical patterns")
		patterns = []*HistoricalPattern{}
	}
	// Calculate base risk score
	baseRiskScore := fp.riskCalculator.CalculateBaseRisk(buildContext)
	// Predict specific failures
	potentialFailures := fp.predictSpecificFailures(buildContext, patterns)
	// Generate preventive actions
	preventiveActions := fp.generatePreventiveActions(buildContext, potentialFailures)
	// Identify monitoring points
	monitoringPoints := fp.identifyMonitoringPoints(buildContext, potentialFailures)
	// Calculate overall confidence
	confidence := fp.calculatePredictionConfidence(buildContext, patterns, potentialFailures)
	// Calculate final risk score considering all factors
	finalRiskScore := fp.adjustRiskScore(baseRiskScore, potentialFailures, patterns)
	prediction := &FailurePrediction{
		PotentialFailures: potentialFailures,
		RiskScore:         finalRiskScore,
		PreventiveActions: preventiveActions,
		MonitoringPoints:  monitoringPoints,
		ConfidenceLevel:   confidence,
	}
	fp.logger.Info().
		Float64("risk_score", finalRiskScore).
		Float64("confidence", confidence).
		Int("potential_failures", len(potentialFailures)).
		Msg("Failure prediction completed")
	return prediction, nil
}

// predictSpecificFailures identifies specific potential failures
func (fp *FailurePredictor) predictSpecificFailures(buildContext *AnalysisBuildContext, patterns []*HistoricalPattern) []*PredictedFailure {
	failures := []*PredictedFailure{}
	// Language-specific failure predictions
	failures = append(failures, fp.predictLanguageSpecificFailures(buildContext.ProjectInfo)...)
	// Environment-based failure predictions
	failures = append(failures, fp.predictEnvironmentFailures(buildContext.Environment)...)
	// Resource-based failure predictions
	failures = append(failures, fp.predictResourceFailures(buildContext.CurrentState)...)
	// Historical pattern-based predictions
	failures = append(failures, fp.predictFromHistoricalPatterns(patterns)...)
	// Dependency-based predictions
	failures = append(failures, fp.predictDependencyFailures(buildContext.ProjectInfo)...)
	// Remove duplicates and sort by probability
	failures = fp.deduplicateAndSort(failures)
	return failures
}
func (fp *FailurePredictor) predictLanguageSpecificFailures(projectInfo *ProjectMetadata) []*PredictedFailure {
	failures := []*PredictedFailure{}
	switch strings.ToLower(projectInfo.Language) {
	case "go":
		failures = append(failures, &PredictedFailure{
			FailureType:       "module_download_failure",
			Probability:       0.3,
			TriggerConditions: []string{"Network connectivity issues", "Proxy configuration", "Module not found"},
			PreventiveActions: []string{"Pre-download dependencies", "Configure proxy properly", "Verify module paths"},
			ImpactLevel:       "medium",
		})
		if strings.Contains(strings.ToLower(projectInfo.Framework), "cgo") {
			failures = append(failures, &PredictedFailure{
				FailureType:       "cgo_compilation_failure",
				Probability:       0.4,
				TriggerConditions: []string{"Missing C compiler", "Library path issues", "Cross-compilation problems"},
				PreventiveActions: []string{"Install build-essential", "Set CGO_ENABLED=0 if not needed", "Configure library paths"},
				ImpactLevel:       "high",
			})
		}
	case "python":
		failures = append(failures, &PredictedFailure{
			FailureType:       "dependency_conflict",
			Probability:       0.5,
			TriggerConditions: []string{"Version conflicts", "Missing system packages", "Virtual environment issues"},
			PreventiveActions: []string{"Use dependency lock files", "Create clean virtual environment", "Install system dependencies"},
			ImpactLevel:       "high",
		})
	case "javascript", "typescript":
		failures = append(failures, &PredictedFailure{
			FailureType:       "node_modules_corruption",
			Probability:       0.2,
			TriggerConditions: []string{"Interrupted installation", "Disk space issues", "Network timeouts"},
			PreventiveActions: []string{"Use npm ci instead of npm install", "Clear npm cache", "Ensure sufficient disk space"},
			ImpactLevel:       "medium",
		})
	case "java":
		failures = append(failures, &PredictedFailure{
			FailureType:       "classpath_issues",
			Probability:       0.3,
			TriggerConditions: []string{"Missing dependencies", "Version conflicts", "Circular dependencies"},
			PreventiveActions: []string{"Verify dependency tree", "Use dependency management tools", "Check for conflicts"},
			ImpactLevel:       "high",
		})
	}
	return failures
}
func (fp *FailurePredictor) predictEnvironmentFailures(environment map[string]interface{}) []*PredictedFailure {
	failures := []*PredictedFailure{}
	// Check for common environment issues
	if dockerVersion, exists := environment["docker_version"]; exists {
		if version, ok := dockerVersion.(string); ok && version == "" {
			failures = append(failures, &PredictedFailure{
				FailureType:       "docker_unavailable",
				Probability:       0.8,
				TriggerConditions: []string{"Docker daemon not running", "Docker not installed"},
				PreventiveActions: []string{"Start Docker daemon", "Install Docker", "Check Docker permissions"},
				ImpactLevel:       "critical",
			})
		}
	}
	// Check resource availability
	if memory, exists := environment["available_memory"]; exists {
		if mem, ok := memory.(int64); ok && mem < 1024*1024*1024 { // Less than 1GB
			failures = append(failures, &PredictedFailure{
				FailureType:       "out_of_memory",
				Probability:       0.6,
				TriggerConditions: []string{"Insufficient memory", "Memory leaks", "Large build artifacts"},
				PreventiveActions: []string{"Free up memory", "Use swap space", "Optimize build process"},
				ImpactLevel:       "high",
			})
		}
	}
	return failures
}
func (fp *FailurePredictor) predictResourceFailures(currentState *BuildState) []*PredictedFailure {
	failures := []*PredictedFailure{}
	if currentState == nil {
		return failures
	}
	// Check current resource usage
	if currentState.CurrentResources != nil {
		resources := currentState.CurrentResources
		// CPU usage prediction
		if resources.CPU > 0.9 {
			failures = append(failures, &PredictedFailure{
				FailureType:       "cpu_exhaustion",
				Probability:       0.7,
				TriggerConditions: []string{"High CPU usage", "CPU-intensive operations", "Parallel builds"},
				PreventiveActions: []string{"Reduce parallelism", "Optimize build process", "Monitor CPU usage"},
				ImpactLevel:       "medium",
			})
		}
		// Memory usage prediction
		if resources.Memory > 1024*1024*1024*8 { // More than 8GB
			failures = append(failures, &PredictedFailure{
				FailureType:       "memory_leak",
				Probability:       0.4,
				TriggerConditions: []string{"Memory leaks", "Large data processing", "Inefficient algorithms"},
				PreventiveActions: []string{"Monitor memory usage", "Optimize data structures", "Use memory profiling"},
				ImpactLevel:       "high",
			})
		}
		// Disk usage prediction
		if resources.Disk > 1024*1024*1024*50 { // More than 50GB
			failures = append(failures, &PredictedFailure{
				FailureType:       "disk_space_exhaustion",
				Probability:       0.5,
				TriggerConditions: []string{"Large build artifacts", "Insufficient disk space", "Disk usage growth"},
				PreventiveActions: []string{"Clean up artifacts", "Monitor disk usage", "Increase disk space"},
				ImpactLevel:       "critical",
			})
		}
	}
	// Check for error patterns
	if len(currentState.Errors) > 0 {
		failures = append(failures, &PredictedFailure{
			FailureType:       "recurring_errors",
			Probability:       0.6,
			TriggerConditions: []string{"Previous errors", "Configuration issues", "Environmental problems"},
			PreventiveActions: []string{"Address previous errors", "Check configuration", "Verify environment"},
			ImpactLevel:       "high",
		})
	}
	return failures
}
func (fp *FailurePredictor) predictFromHistoricalPatterns(patterns []*HistoricalPattern) []*PredictedFailure {
	failures := []*PredictedFailure{}
	for _, pattern := range patterns {
		if pattern.Frequency > 2 && pattern.RecentOccurrences > 0 {
			probability := fp.calculatePatternProbability(pattern)
			failures = append(failures, &PredictedFailure{
				FailureType:       pattern.FailureType,
				Probability:       probability,
				TriggerConditions: pattern.CommonTriggers,
				PreventiveActions: pattern.PreventiveActions,
				ImpactLevel:       pattern.AverageImpact,
			})
		}
	}
	return failures
}
func (fp *FailurePredictor) predictDependencyFailures(projectInfo *ProjectMetadata) []*PredictedFailure {
	failures := []*PredictedFailure{}
	if len(projectInfo.Dependencies) > 50 {
		failures = append(failures, &PredictedFailure{
			FailureType:       "dependency_resolution_timeout",
			Probability:       0.3,
			TriggerConditions: []string{"Too many dependencies", "Complex dependency tree", "Network issues"},
			PreventiveActions: []string{"Optimize dependencies", "Use dependency caching", "Improve network connection"},
			ImpactLevel:       "medium",
		})
	}
	// Check for commonly problematic dependencies
	problematicDeps := []string{"node-sass", "canvas", "sqlite3", "bcrypt"}
	for _, dep := range projectInfo.Dependencies {
		for _, problematic := range problematicDeps {
			if strings.Contains(strings.ToLower(dep), problematic) {
				failures = append(failures, &PredictedFailure{
					FailureType:       "native_dependency_failure",
					Probability:       0.4,
					TriggerConditions: []string{"Missing native libraries", "Compilation issues", "Platform incompatibility"},
					PreventiveActions: []string{"Install native dependencies", "Use precompiled binaries", "Check platform compatibility"},
					ImpactLevel:       "high",
				})
				break
			}
		}
	}
	return failures
}
func (fp *FailurePredictor) generatePreventiveActions(buildContext *AnalysisBuildContext, failures []*PredictedFailure) []string {
	actions := []string{}
	actionSet := make(map[string]bool) // For deduplication
	// Add general preventive actions
	generalActions := []string{
		"Verify all prerequisites are installed",
		"Check network connectivity",
		"Ensure sufficient system resources",
		"Update build tools to latest versions",
		"Run preliminary validation checks",
	}
	for _, action := range generalActions {
		if !actionSet[action] {
			actions = append(actions, action)
			actionSet[action] = true
		}
	}
	// Add failure-specific actions
	for _, failure := range failures {
		if failure.Probability > 0.3 { // Only include likely failures
			for _, action := range failure.PreventiveActions {
				if !actionSet[action] {
					actions = append(actions, action)
					actionSet[action] = true
				}
			}
		}
	}
	return actions
}
func (fp *FailurePredictor) identifyMonitoringPoints(buildContext *AnalysisBuildContext, failures []*PredictedFailure) []string {
	points := []string{}
	pointSet := make(map[string]bool) // For deduplication
	// Add general monitoring points
	generalPoints := []string{
		"Build execution time",
		"Memory usage during build",
		"CPU utilization",
		"Disk space consumption",
		"Network activity",
		"Error and warning counts",
	}
	for _, point := range generalPoints {
		if !pointSet[point] {
			points = append(points, point)
			pointSet[point] = true
		}
	}
	// Add failure-specific monitoring points
	for _, failure := range failures {
		if failure.Probability > 0.3 {
			switch failure.FailureType {
			case "dependency_conflict", "dependency_resolution_timeout":
				if !pointSet["Dependency resolution time"] {
					points = append(points, "Dependency resolution time")
					pointSet["Dependency resolution time"] = true
				}
			case "out_of_memory", "memory_leak":
				if !pointSet["Memory allocation patterns"] {
					points = append(points, "Memory allocation patterns")
					pointSet["Memory allocation patterns"] = true
				}
			case "docker_unavailable":
				if !pointSet["Docker daemon status"] {
					points = append(points, "Docker daemon status")
					pointSet["Docker daemon status"] = true
				}
			}
		}
	}
	return points
}
func (fp *FailurePredictor) calculatePredictionConfidence(buildContext *AnalysisBuildContext, patterns []*HistoricalPattern, failures []*PredictedFailure) float64 {
	confidence := 0.5 // Base confidence
	// Increase confidence based on historical data
	if len(patterns) > 0 {
		confidence += 0.2
	}
	// Increase confidence based on build history
	if len(buildContext.BuildHistory) > 5 {
		confidence += 0.1
	}
	// Increase confidence for known project types
	if buildContext.ProjectInfo.Language != "" {
		confidence += 0.1
	}
	// Decrease confidence for complex scenarios
	if len(failures) > 10 {
		confidence -= 0.1
	}
	// Ensure confidence stays within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}
	return confidence
}
func (fp *FailurePredictor) adjustRiskScore(baseScore float64, failures []*PredictedFailure, patterns []*HistoricalPattern) float64 {
	adjustedScore := baseScore
	// Adjust based on high-probability failures
	for _, failure := range failures {
		if failure.Probability > 0.7 {
			adjustedScore += 0.1
		}
	}
	// Adjust based on critical impact failures
	for _, failure := range failures {
		if failure.ImpactLevel == "critical" {
			adjustedScore += 0.15
		}
	}
	// Adjust based on historical patterns
	for _, pattern := range patterns {
		if pattern.RecentOccurrences > 0 {
			adjustedScore += 0.05
		}
	}
	// Ensure score stays within bounds
	if adjustedScore > 1.0 {
		adjustedScore = 1.0
	}
	if adjustedScore < 0.0 {
		adjustedScore = 0.0
	}
	return adjustedScore
}
func (fp *FailurePredictor) calculatePatternProbability(pattern *HistoricalPattern) float64 {
	// Base probability from frequency
	baseProbability := float64(pattern.Frequency) / 100.0
	if baseProbability > 0.8 {
		baseProbability = 0.8 // Cap at 80%
	}
	// Adjust based on recent occurrences
	if pattern.RecentOccurrences > 0 {
		baseProbability += 0.1
	}
	// Adjust based on trend
	if pattern.Trend > 0 {
		baseProbability += 0.05
	}
	return baseProbability
}
func (fp *FailurePredictor) deduplicateAndSort(failures []*PredictedFailure) []*PredictedFailure {
	// Deduplicate by failure type
	seen := make(map[string]*PredictedFailure)
	for _, failure := range failures {
		if existing, exists := seen[failure.FailureType]; exists {
			// Keep the one with higher probability
			if failure.Probability > existing.Probability {
				seen[failure.FailureType] = failure
			}
		} else {
			seen[failure.FailureType] = failure
		}
	}
	// Convert back to slice
	result := make([]*PredictedFailure, 0, len(seen))
	for _, failure := range seen {
		result = append(result, failure)
	}
	// Sort by probability (highest first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Probability < result[j].Probability {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// Supporting types and components
type HistoricalPattern struct {
	FailureType       string    `json:"failure_type"`
	Frequency         int       `json:"frequency"`
	RecentOccurrences int       `json:"recent_occurrences"`
	CommonTriggers    []string  `json:"common_triggers"`
	PreventiveActions []string  `json:"preventive_actions"`
	AverageImpact     string    `json:"average_impact"`
	Trend             float64   `json:"trend"` // Positive for increasing, negative for decreasing
	LastSeen          time.Time `json:"last_seen"`
}
type PatternAnalyzer struct {
	logger zerolog.Logger
}

func NewPatternAnalyzer(logger zerolog.Logger) *PatternAnalyzer {
	return &PatternAnalyzer{
		logger: logger.With().Str("component", "pattern_analyzer").Logger(),
	}
}
func (pa *PatternAnalyzer) AnalyzeHistoricalPatterns(history []*BuildHistoryEntry) ([]*HistoricalPattern, error) {
	patterns := []*HistoricalPattern{}
	// Group failures by type
	failureGroups := make(map[string][]*BuildHistoryEntry)
	for _, entry := range history {
		if !entry.Success && entry.ErrorType != "" {
			if failureGroups[entry.ErrorType] == nil {
				failureGroups[entry.ErrorType] = []*BuildHistoryEntry{}
			}
			failureGroups[entry.ErrorType] = append(failureGroups[entry.ErrorType], entry)
		}
	}
	// Analyze each group
	for errorType, entries := range failureGroups {
		pattern := &HistoricalPattern{
			FailureType:       errorType,
			Frequency:         len(entries),
			RecentOccurrences: pa.countRecentOccurrences(entries, 7*24*time.Hour), // Last 7 days
			CommonTriggers:    pa.extractCommonTriggers(entries),
			PreventiveActions: pa.generatePreventiveActions(errorType),
			AverageImpact:     pa.calculateAverageImpact(entries),
			Trend:             pa.calculateTrend(entries),
			LastSeen:          pa.getLastOccurrence(entries),
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}
func (pa *PatternAnalyzer) countRecentOccurrences(entries []*BuildHistoryEntry, duration time.Duration) int {
	cutoff := time.Now().Add(-duration)
	count := 0
	for _, entry := range entries {
		if entry.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}
func (pa *PatternAnalyzer) extractCommonTriggers(entries []*BuildHistoryEntry) []string {
	// Simple implementation - could be enhanced with more sophisticated analysis
	triggers := []string{}
	if len(entries) > 3 {
		triggers = append(triggers, "Recurring issue pattern")
	}
	// Check for time-based patterns
	weekendCount := 0
	for _, entry := range entries {
		if entry.Timestamp.Weekday() == time.Saturday || entry.Timestamp.Weekday() == time.Sunday {
			weekendCount++
		}
	}
	if float64(weekendCount)/float64(len(entries)) > 0.7 {
		triggers = append(triggers, "Weekend deployment pattern")
	}
	return triggers
}
func (pa *PatternAnalyzer) generatePreventiveActions(errorType string) []string {
	var actions []string
	switch strings.ToLower(errorType) {
	case "dependency_error":
		actions = []string{
			"Update dependency lock files",
			"Clear dependency cache",
			"Verify dependency versions",
		}
	case "build_error":
		actions = []string{
			"Clean build artifacts",
			"Verify build configuration",
			"Check compiler version",
		}
	case "test_failure":
		actions = []string{
			"Run tests in isolation",
			"Check test data setup",
			"Verify test environment",
		}
	default:
		actions = []string{
			"Review error logs",
			"Check system resources",
			"Verify configuration",
		}
	}
	return actions
}
func (pa *PatternAnalyzer) calculateAverageImpact(entries []*BuildHistoryEntry) string {
	if len(entries) == 0 {
		return "unknown"
	}
	// Simple heuristic based on failure frequency
	if len(entries) > 10 {
		return "high"
	} else if len(entries) > 5 {
		return "medium"
	} else {
		return "low"
	}
}
func (pa *PatternAnalyzer) calculateTrend(entries []*BuildHistoryEntry) float64 {
	if len(entries) < 2 {
		return 0.0
	}
	// Simple trend calculation based on time distribution
	now := time.Now()
	recentWeight := 0.0
	oldWeight := 0.0
	for _, entry := range entries {
		age := now.Sub(entry.Timestamp)
		if age < 30*24*time.Hour { // Last 30 days
			recentWeight += 1.0
		} else {
			oldWeight += 1.0
		}
	}
	if oldWeight == 0 {
		return 1.0 // All recent, positive trend
	}
	return (recentWeight - oldWeight) / (recentWeight + oldWeight)
}
func (pa *PatternAnalyzer) getLastOccurrence(entries []*BuildHistoryEntry) time.Time {
	if len(entries) == 0 {
		return time.Time{}
	}
	latest := entries[0].Timestamp
	for _, entry := range entries {
		if entry.Timestamp.After(latest) {
			latest = entry.Timestamp
		}
	}
	return latest
}

type RiskCalculator struct {
	logger zerolog.Logger
}

func NewRiskCalculator(logger zerolog.Logger) *RiskCalculator {
	return &RiskCalculator{
		logger: logger.With().Str("component", "risk_calculator").Logger(),
	}
}
func (rc *RiskCalculator) CalculateBaseRisk(buildContext *AnalysisBuildContext) float64 {
	risk := 0.1 // Base risk
	// Project complexity factors
	if buildContext.ProjectInfo.Complexity == "high" {
		risk += 0.2
	} else if buildContext.ProjectInfo.Complexity == "medium" {
		risk += 0.1
	}
	// Dependency count factor
	depCount := len(buildContext.ProjectInfo.Dependencies)
	if depCount > 100 {
		risk += 0.3
	} else if depCount > 50 {
		risk += 0.2
	} else if depCount > 20 {
		risk += 0.1
	}
	// Historical failure rate
	if len(buildContext.BuildHistory) > 0 {
		failureCount := 0
		for _, entry := range buildContext.BuildHistory {
			if !entry.Success {
				failureCount++
			}
		}
		failureRate := float64(failureCount) / float64(len(buildContext.BuildHistory))
		risk += failureRate * 0.3
	}
	// Current state factors
	if buildContext.CurrentState != nil {
		if len(buildContext.CurrentState.Errors) > 0 {
			risk += 0.1
		}
		if len(buildContext.CurrentState.Warnings) > 5 {
			risk += 0.05
		}
	}
	// Environment factors
	if buildContext.Environment["unstable"] == true {
		risk += 0.2
	}
	// Cap risk at 1.0
	if risk > 1.0 {
		risk = 1.0
	}
	return risk
}
