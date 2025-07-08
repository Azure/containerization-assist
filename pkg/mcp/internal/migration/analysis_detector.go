package migration

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// NewDetector creates a new migration opportunity detector
func NewDetector(config Config, logger zerolog.Logger) *Detector {
	md := &Detector{
		logger:  logger.With().Str("component", "migration_detector").Logger(),
		config:  config,
		fileSet: token.NewFileSet(),
	}

	// Initialize patterns and analyzers
	md.initializePatterns()
	md.initializeAnalyzers()

	return md
}

// DetectMigrations analyzes the codebase and returns migration opportunities
func (md *Detector) DetectMigrations(rootPath string) (*Report, error) {
	md.logger.Info().Str("path", rootPath).Msg("Starting migration detection")

	startTime := time.Now()
	report := &Report{
		GeneratedAt:   startTime,
		Opportunities: []Opportunity{},
		Statistics: Statistics{
			ByType:       make(map[string]int),
			ByPriority:   make(map[string]int),
			ByConfidence: make(map[string]int),
			ByEffort:     make(map[string]int),
			ByFile:       make(map[string]int),
		},
	}

	// Walk the directory tree
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check if should ignore
		if md.shouldIgnore(path) {
			return nil
		}

		report.TotalFiles++

		// Analyze the file
		opportunities, err := md.analyzeFile(path)
		if err != nil {
			md.logger.Error().Err(err).Str("file", path).Msg("Failed to analyze file")
			return nil // Continue with other files
		}

		if len(opportunities) > 0 {
			report.AnalyzedFiles++
			report.Opportunities = append(report.Opportunities, opportunities...)
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "migration", "failed to walk directory")
	}

	// Run pattern analysis if enabled
	if md.config.EnablePatternDetection {
		analyzer := NewPatternAnalyzer(PatternAnalysisConfig{
			EnableComplexityAnalysis:   true,
			EnableDuplicationDetection: true,
			EnableAntiPatternDetection: true,
			ComplexityThreshold:        10,
			DuplicationThreshold:       0.1,
		}, md.logger)

		if patternResult, err := analyzer.AnalyzePatterns(rootPath); err == nil {
			report.PatternAnalysis = patternResult
		}
	}

	// Calculate statistics
	md.calculateStatistics(report)

	// Generate recommendations
	report.Recommendations = md.generateRecommendations(report)

	// Calculate effort estimate
	report.EstimatedEffort = md.calculateEffortEstimate(report)

	// Generate summary
	report.Summary = md.generateSummary(report)

	md.logger.Info().
		Int("opportunities", len(report.Opportunities)).
		Dur("duration", time.Since(startTime)).
		Msg("Migration detection completed")

	return report, nil
}

// analyzeFile analyzes a single file for migration opportunities
func (md *Detector) analyzeFile(filePath string) ([]Opportunity, error) {
	var allOpportunities []Opportunity

	// Read file content for pattern detection
	if md.config.EnablePatternDetection {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		patternOpportunities := md.analyzePatterns(string(content), filePath)
		allOpportunities = append(allOpportunities, patternOpportunities...)
	}

	// Perform structural analysis
	if md.config.EnableStructuralAnalysis {
		structuralOpportunities, err := md.analyzeStructure(filePath)
		if err != nil {
			md.logger.Warn().Err(err).Str("file", filePath).Msg("Structural analysis failed")
		} else {
			allOpportunities = append(allOpportunities, structuralOpportunities...)
		}
	}

	return allOpportunities, nil
}

// shouldIgnore checks if a path should be ignored
func (md *Detector) shouldIgnore(path string) bool {
	// Check ignore directories
	for _, dir := range md.config.IgnoreDirectories {
		if strings.Contains(path, dir) {
			return true
		}
	}

	// Check ignore files
	for _, file := range md.config.IgnoreFiles {
		if strings.HasSuffix(path, file) {
			return true
		}
	}

	// Always ignore vendor and test files by default
	if strings.Contains(path, "vendor/") || strings.HasSuffix(path, "_test.go") {
		return true
	}

	return false
}

// calculateStatistics calculates migration statistics
func (md *Detector) calculateStatistics(report *Report) {
	for _, opp := range report.Opportunities {
		// By type
		report.Statistics.ByType[opp.Type]++

		// By priority
		report.Statistics.ByPriority[opp.Priority]++

		// By confidence
		confidenceRange := fmt.Sprintf("%.1f-%.1f",
			float64(int(opp.Confidence*10))/10,
			float64(int(opp.Confidence*10)+1)/10)
		report.Statistics.ByConfidence[confidenceRange]++

		// By effort
		report.Statistics.ByEffort[opp.EstimatedEffort]++

		// By file
		report.Statistics.ByFile[opp.File]++
	}
}

// generateRecommendations generates migration recommendations
func (md *Detector) generateRecommendations(report *Report) []string {
	var recommendations []string

	// High priority recommendations
	highPriorityCount := report.Statistics.ByPriority["HIGH"]
	if highPriorityCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Address %d high-priority migration opportunities first", highPriorityCount))
	}

	// Pattern-specific recommendations
	if errorIgnoreCount := report.Statistics.ByType["error_ignore"]; errorIgnoreCount > 0 {
		recommendations = append(recommendations,
			"Improve error handling across the codebase - found multiple instances of ignored errors")
	}

	if interfaceCount := report.Statistics.ByType["large_interface"]; interfaceCount > 0 {
		recommendations = append(recommendations,
			"Apply Interface Segregation Principle to large interfaces")
	}

	// Effort-based recommendations
	if trivialCount := report.Statistics.ByEffort["TRIVIAL"]; trivialCount > 10 {
		recommendations = append(recommendations,
			fmt.Sprintf("Start with %d trivial fixes for quick wins", trivialCount))
	}

	return recommendations
}

// calculateEffortEstimate calculates effort estimation
func (md *Detector) calculateEffortEstimate(report *Report) EffortEstimate {
	effortHours := map[string]float64{
		"TRIVIAL":  0.5,
		"MINOR":    2.0,
		"MAJOR":    8.0,
		"CRITICAL": 16.0,
	}

	estimate := EffortEstimate{
		EffortByType:     make(map[string]float64),
		EffortByPriority: make(map[string]float64),
	}

	// Calculate total effort
	for effort, count := range report.Statistics.ByEffort {
		hours := effortHours[effort] * float64(count)
		estimate.TotalEffortHours += hours
	}

	// Calculate effort by type
	for _, opp := range report.Opportunities {
		hours := effortHours[opp.EstimatedEffort]
		estimate.EffortByType[opp.Type] += hours
		estimate.EffortByPriority[opp.Priority] += hours
	}

	// Generate timeline
	estimate.Timeline = md.calculateTimeline(estimate.TotalEffortHours)

	// Identify risk factors
	estimate.RiskFactors = md.identifyRiskFactors(report)

	return estimate
}

// calculateTimeline calculates migration timeline
func (md *Detector) calculateTimeline(totalHours float64) TimelineEstimate {
	// Assume 6 productive hours per day
	productiveHoursPerDay := 6.0
	totalDays := int(totalHours / productiveHoursPerDay)

	// Add buffer for testing and review
	bufferDays := int(float64(totalDays) * 0.3)

	timeline := TimelineEstimate{
		MinDays:         totalDays,
		MaxDays:         totalDays + bufferDays,
		RecommendedDays: totalDays + (bufferDays / 2),
	}

	// Define phases
	timeline.Phases = []Phase{
		{
			Name:        "Planning & Prioritization",
			Duration:    2,
			Description: "Review opportunities and create migration plan",
			Tasks: []string{
				"Review migration report",
				"Prioritize opportunities",
				"Create implementation plan",
			},
		},
		{
			Name:        "Quick Wins",
			Duration:    timeline.MinDays / 4,
			Description: "Address trivial and minor fixes",
			Tasks: []string{
				"Fix trivial issues",
				"Implement minor improvements",
				"Run tests",
			},
		},
		{
			Name:        "Major Refactoring",
			Duration:    timeline.MinDays / 2,
			Description: "Implement major architectural changes",
			Tasks: []string{
				"Refactor large components",
				"Apply design patterns",
				"Update documentation",
			},
		},
		{
			Name:        "Testing & Validation",
			Duration:    timeline.MinDays / 4,
			Description: "Comprehensive testing and validation",
			Tasks: []string{
				"Run full test suite",
				"Performance testing",
				"Code review",
			},
		},
	}

	return timeline
}

// identifyRiskFactors identifies potential risks
func (md *Detector) identifyRiskFactors(report *Report) []string {
	var risks []string

	// Check for high complexity migrations
	if report.Statistics.ByEffort["CRITICAL"] > 0 {
		risks = append(risks, "Contains critical complexity migrations requiring careful planning")
	}

	// Check for widespread changes
	filesAffected := len(report.Statistics.ByFile)
	if filesAffected > 50 {
		risks = append(risks, fmt.Sprintf("Changes affect %d files - consider phased approach", filesAffected))
	}

	// Check for interface changes
	if report.Statistics.ByType["large_interface"] > 0 {
		risks = append(risks, "Interface changes may impact multiple implementations")
	}

	return risks
}

// generateSummary generates report summary
func (md *Detector) generateSummary(report *Report) ReportSummary {
	summary := ReportSummary{
		TotalOpportunities:  len(report.Opportunities),
		HighPriorityCount:   report.Statistics.ByPriority["HIGH"],
		MediumPriorityCount: report.Statistics.ByPriority["MEDIUM"],
		LowPriorityCount:    report.Statistics.ByPriority["LOW"],
	}

	// Calculate average confidence
	totalConfidence := 0.0
	for _, opp := range report.Opportunities {
		totalConfidence += opp.Confidence
	}
	if len(report.Opportunities) > 0 {
		summary.AverageConfidence = totalConfidence / float64(len(report.Opportunities))
	}

	// Find most common type
	maxCount := 0
	for oppType, count := range report.Statistics.ByType {
		if count > maxCount {
			maxCount = count
			summary.MostCommonType = oppType
			summary.MostCommonTypeCount = count
		}
	}

	// Find files most impacted
	type fileCount struct {
		file  string
		count int
	}
	var fileCounts []fileCount
	for file, count := range report.Statistics.ByFile {
		fileCounts = append(fileCounts, fileCount{file, count})
	}
	// Sort by count
	for i := 0; i < len(fileCounts)-1; i++ {
		for j := i + 1; j < len(fileCounts); j++ {
			if fileCounts[j].count > fileCounts[i].count {
				fileCounts[i], fileCounts[j] = fileCounts[j], fileCounts[i]
			}
		}
	}
	// Take top 5
	for i := 0; i < 5 && i < len(fileCounts); i++ {
		summary.FilesMostImpacted = append(summary.FilesMostImpacted, fileCounts[i].file)
	}

	// Recommend starting point
	if summary.HighPriorityCount > 0 {
		summary.RecommendedStartingPoint = "Start with high-priority issues"
	} else if report.Statistics.ByEffort["TRIVIAL"] > 10 {
		summary.RecommendedStartingPoint = "Begin with trivial fixes for quick wins"
	} else {
		summary.RecommendedStartingPoint = "Focus on the most common pattern: " + summary.MostCommonType
	}

	summary.EstimatedTotalEffort = fmt.Sprintf("%.1f hours", report.EstimatedEffort.TotalEffortHours)

	return summary
}
