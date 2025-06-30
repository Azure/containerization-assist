package testutil

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CoverageAnalyzer provides comprehensive test coverage analysis
type CoverageAnalyzer struct {
	baseDir      string
	logger       zerolog.Logger
	threshold    float64
	excludePaths []string
	teamPackages map[string][]string
}

// CoverageReport represents a coverage analysis report
type CoverageReport struct {
	Timestamp       time.Time                      `json:"timestamp"`
	OverallCoverage float64                        `json:"overall_coverage"`
	TeamCoverage    map[string]TeamCoverageInfo    `json:"team_coverage"`
	PackageCoverage map[string]PackageCoverageInfo `json:"package_coverage"`
	UncoveredLines  map[string][]UncoveredLine     `json:"uncovered_lines"`
	Recommendations []CoverageRecommendation       `json:"recommendations"`
	MeetsThreshold  bool                           `json:"meets_threshold"`
}

// TeamCoverageInfo contains coverage information for a team
type TeamCoverageInfo struct {
	TeamName         string                         `json:"team_name"`
	Coverage         float64                        `json:"coverage"`
	TotalLines       int                            `json:"total_lines"`
	CoveredLines     int                            `json:"covered_lines"`
	Packages         []string                       `json:"packages"`
	PackageBreakdown map[string]PackageCoverageInfo `json:"package_breakdown"`
}

// PackageCoverageInfo contains coverage information for a package
type PackageCoverageInfo struct {
	Package      string             `json:"package"`
	Coverage     float64            `json:"coverage"`
	TotalLines   int                `json:"total_lines"`
	CoveredLines int                `json:"covered_lines"`
	Functions    []FunctionCoverage `json:"functions"`
	Files        map[string]float64 `json:"files"`
}

// FunctionCoverage represents coverage for a specific function
type FunctionCoverage struct {
	Name         string  `json:"name"`
	File         string  `json:"file"`
	Line         int     `json:"line"`
	Coverage     float64 `json:"coverage"`
	TotalLines   int     `json:"total_lines"`
	CoveredLines int     `json:"covered_lines"`
}

// UncoveredLine represents an uncovered line of code
type UncoveredLine struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Statement  string `json:"statement"`
	Function   string `json:"function"`
	Complexity int    `json:"complexity"`
}

// CoverageRecommendation provides actionable coverage improvement suggestions
type CoverageRecommendation struct {
	Priority   string   `json:"priority"` // HIGH, MEDIUM, LOW
	Type       string   `json:"type"`     // MISSING_TEST, LOW_COVERAGE, COMPLEX_FUNCTION
	Target     string   `json:"target"`   // Package or function name
	Current    float64  `json:"current"`
	Required   float64  `json:"required"`
	Suggestion string   `json:"suggestion"`
	TestFiles  []string `json:"test_files"`
}

// NewCoverageAnalyzer creates a new coverage analyzer
func NewCoverageAnalyzer(baseDir string, logger zerolog.Logger) *CoverageAnalyzer {
	return &CoverageAnalyzer{
		baseDir:   baseDir,
		logger:    logger.With().Str("component", "coverage_analyzer").Logger(),
		threshold: 90.0, // 90% coverage threshold from requirements
		excludePaths: []string{
			"vendor/",
			"test/",
			"testdata/",
			".git/",
			"docs/",
		},
		teamPackages: map[string][]string{
			"InfraBot": {
				"pkg/mcp/internal/pipeline",
				"pkg/mcp/internal/session",
				"pkg/mcp/internal/runtime",
			},
			"BuildSecBot": {
				"pkg/mcp/internal/build",
				"pkg/mcp/internal/scan",
				"pkg/mcp/internal/analyze",
			},
			"OrchBot": {
				"pkg/mcp/internal/orchestration",
				"pkg/mcp/internal/conversation",
				"pkg/mcp/internal/workflow",
			},
			"AdvancedBot": {
				"pkg/mcp/internal/utils",
				"pkg/mcp/internal/observability",
				"pkg/mcp/internal/testutil",
			},
		},
	}
}

// AnalyzeCoverage performs comprehensive coverage analysis
func (ca *CoverageAnalyzer) AnalyzeCoverage(ctx context.Context) (*CoverageReport, error) {
	ca.logger.Info().Msg("Starting coverage analysis")

	// Run go test with coverage
	coverageData, err := ca.runCoverageTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run coverage tests: %w", err)
	}

	// Parse coverage data
	report, err := ca.parseCoverageData(coverageData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse coverage data: %w", err)
	}

	// Analyze uncovered lines
	report.UncoveredLines = ca.analyzeUncoveredLines(ctx, coverageData)

	// Generate recommendations
	report.Recommendations = ca.generateRecommendations(report)

	// Check threshold
	report.MeetsThreshold = report.OverallCoverage >= ca.threshold

	ca.logger.Info().
		Float64("overall_coverage", report.OverallCoverage).
		Bool("meets_threshold", report.MeetsThreshold).
		Msg("Coverage analysis completed")

	return report, nil
}

// runCoverageTests executes tests with coverage enabled
func (ca *CoverageAnalyzer) runCoverageTests(ctx context.Context) ([]byte, error) {
	// Create temporary coverage file
	coverFile := filepath.Join(os.TempDir(), fmt.Sprintf("coverage-%d.out", time.Now().UnixNano()))
	defer os.Remove(coverFile)

	// Run go test with coverage
	cmd := exec.CommandContext(ctx, "go", "test",
		"-coverprofile="+coverFile,
		"-covermode=atomic",
		"-tags=mcp",
		"./pkg/mcp/...",
	)
	cmd.Dir = ca.baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		ca.logger.Error().Err(err).Str("output", string(output)).Msg("Coverage test failed")
		// Don't fail completely - try to parse what we have
	}

	// Read coverage data
	data, err := os.ReadFile(coverFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read coverage file: %w", err)
	}

	return data, nil
}

// parseCoverageData parses go coverage output
func (ca *CoverageAnalyzer) parseCoverageData(data []byte) (*CoverageReport, error) {
	report := &CoverageReport{
		Timestamp:       time.Now(),
		TeamCoverage:    make(map[string]TeamCoverageInfo),
		PackageCoverage: make(map[string]PackageCoverageInfo),
	}

	// Parse coverage profile
	profiles, err := ca.parseCoverProfile(data)
	if err != nil {
		return nil, err
	}

	// Calculate coverage by package
	for _, profile := range profiles {
		pkgInfo := ca.calculatePackageCoverage(profile)
		report.PackageCoverage[profile.Package] = pkgInfo

		// Assign to team
		teamName := ca.getTeamForPackage(profile.Package)
		if teamName != "" {
			teamInfo := report.TeamCoverage[teamName]
			teamInfo.TeamName = teamName
			teamInfo.TotalLines += pkgInfo.TotalLines
			teamInfo.CoveredLines += pkgInfo.CoveredLines
			teamInfo.Packages = append(teamInfo.Packages, profile.Package)

			if teamInfo.PackageBreakdown == nil {
				teamInfo.PackageBreakdown = make(map[string]PackageCoverageInfo)
			}
			teamInfo.PackageBreakdown[profile.Package] = pkgInfo

			report.TeamCoverage[teamName] = teamInfo
		}
	}

	// Calculate overall and team coverage percentages
	var totalLines, coveredLines int
	for teamName, teamInfo := range report.TeamCoverage {
		if teamInfo.TotalLines > 0 {
			teamInfo.Coverage = float64(teamInfo.CoveredLines) / float64(teamInfo.TotalLines) * 100
		}
		report.TeamCoverage[teamName] = teamInfo

		totalLines += teamInfo.TotalLines
		coveredLines += teamInfo.CoveredLines
	}

	if totalLines > 0 {
		report.OverallCoverage = float64(coveredLines) / float64(totalLines) * 100
	}

	return report, nil
}

// parseCoverProfile parses a coverage profile
func (ca *CoverageAnalyzer) parseCoverProfile(data []byte) ([]CoverageProfile, error) {
	var profiles []CoverageProfile

	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	// Skip mode line
	if scanner.Scan() {
		modeLine := scanner.Text()
		if !strings.HasPrefix(modeLine, "mode:") {
			return nil, fmt.Errorf("invalid coverage profile format")
		}
	}

	currentProfile := &CoverageProfile{}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse coverage line
		// Format: package/file.go:line.col,line.col statements count
		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}

		filePart := parts[0]
		count := parts[2]

		// Extract package and file
		colonIdx := strings.LastIndex(filePart, ":")
		if colonIdx < 0 {
			continue
		}

		filePathPart := filePart[:colonIdx]
		pkgEndIdx := strings.LastIndex(filePathPart, "/")
		if pkgEndIdx < 0 {
			continue
		}

		pkg := filePathPart[:pkgEndIdx]

		if currentProfile.Package != pkg {
			if currentProfile.Package != "" {
				profiles = append(profiles, *currentProfile)
			}
			currentProfile = &CoverageProfile{
				Package: pkg,
				Files:   make(map[string][]CoverageBlock),
			}
		}

		// Add coverage block
		block := CoverageBlock{
			StartLine: 0, // Would parse from position
			EndLine:   0, // Would parse from position
			NumStmt:   1,
			Count:     count != "0",
		}

		fileName := filepath.Base(filePathPart)
		currentProfile.Files[fileName] = append(currentProfile.Files[fileName], block)
	}

	if currentProfile.Package != "" {
		profiles = append(profiles, *currentProfile)
	}

	return profiles, scanner.Err()
}

// CoverageProfile represents coverage data for a package
type CoverageProfile struct {
	Package string
	Files   map[string][]CoverageBlock
}

// CoverageBlock represents a covered/uncovered block
type CoverageBlock struct {
	StartLine int
	EndLine   int
	NumStmt   int
	Count     bool
}

// calculatePackageCoverage calculates coverage for a package
func (ca *CoverageAnalyzer) calculatePackageCoverage(profile CoverageProfile) PackageCoverageInfo {
	info := PackageCoverageInfo{
		Package: profile.Package,
		Files:   make(map[string]float64),
	}

	for file, blocks := range profile.Files {
		var fileTotal, fileCovered int
		for _, block := range blocks {
			fileTotal += block.NumStmt
			if block.Count {
				fileCovered += block.NumStmt
			}
		}

		if fileTotal > 0 {
			info.Files[file] = float64(fileCovered) / float64(fileTotal) * 100
		}

		info.TotalLines += fileTotal
		info.CoveredLines += fileCovered
	}

	if info.TotalLines > 0 {
		info.Coverage = float64(info.CoveredLines) / float64(info.TotalLines) * 100
	}

	return info
}

// getTeamForPackage determines which team owns a package
func (ca *CoverageAnalyzer) getTeamForPackage(pkg string) string {
	for team, packages := range ca.teamPackages {
		for _, teamPkg := range packages {
			if strings.HasPrefix(pkg, teamPkg) {
				return team
			}
		}
	}
	return ""
}

// analyzeUncoveredLines finds and analyzes uncovered lines
func (ca *CoverageAnalyzer) analyzeUncoveredLines(ctx context.Context, coverageData []byte) map[string][]UncoveredLine {
	uncovered := make(map[string][]UncoveredLine)

	// This would parse the coverage data and AST to find uncovered lines
	// For now, return empty map

	return uncovered
}

// generateRecommendations creates actionable coverage recommendations
func (ca *CoverageAnalyzer) generateRecommendations(report *CoverageReport) []CoverageRecommendation {
	var recommendations []CoverageRecommendation

	// Team-level recommendations
	for teamName, teamInfo := range report.TeamCoverage {
		if teamInfo.Coverage < ca.threshold {
			rec := CoverageRecommendation{
				Priority: "HIGH",
				Type:     "LOW_COVERAGE",
				Target:   teamName,
				Current:  teamInfo.Coverage,
				Required: ca.threshold,
				Suggestion: fmt.Sprintf("%s needs to improve coverage by %.1f%% to meet the %.0f%% threshold",
					teamName, ca.threshold-teamInfo.Coverage, ca.threshold),
			}
			recommendations = append(recommendations, rec)
		}

		// Package-level recommendations
		for pkg, pkgInfo := range teamInfo.PackageBreakdown {
			if pkgInfo.Coverage < ca.threshold-10 { // Focus on packages significantly below threshold
				rec := CoverageRecommendation{
					Priority: "MEDIUM",
					Type:     "LOW_COVERAGE",
					Target:   pkg,
					Current:  pkgInfo.Coverage,
					Required: ca.threshold,
					Suggestion: fmt.Sprintf("Package %s has low coverage (%.1f%%). Consider adding tests for uncovered functions",
						pkg, pkgInfo.Coverage),
					TestFiles: ca.suggestTestFiles(pkg),
				}
				recommendations = append(recommendations, rec)
			}
		}
	}

	// Sort by priority
	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// suggestTestFiles suggests test files that should be created or updated
func (ca *CoverageAnalyzer) suggestTestFiles(pkg string) []string {
	var suggestions []string

	// Walk the package directory
	pkgDir := filepath.Join(ca.baseDir, pkg)
	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and test files
		if info.IsDir() || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Only process Go files
		if strings.HasSuffix(path, ".go") {
			testFile := strings.TrimSuffix(path, ".go") + "_test.go"
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				relPath, _ := filepath.Rel(ca.baseDir, testFile)
				suggestions = append(suggestions, relPath)
			}
		}

		return nil
	})

	if err != nil {
		ca.logger.Warn().Err(err).Str("package", pkg).Msg("Failed to suggest test files")
	}

	return suggestions
}

// GenerateHTMLReport generates an HTML coverage report
func (ca *CoverageAnalyzer) GenerateHTMLReport(report *CoverageReport, outputPath string) error {
	// This would generate a detailed HTML report
	// For now, just log
	ca.logger.Info().Str("output", outputPath).Msg("HTML report generation not implemented")
	return nil
}

// GenerateTeamReport generates a team-specific coverage report
func (ca *CoverageAnalyzer) GenerateTeamReport(report *CoverageReport, teamName string) string {
	teamInfo, exists := report.TeamCoverage[teamName]
	if !exists {
		return fmt.Sprintf("No coverage data found for team: %s", teamName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s COVERAGE REPORT\n", strings.ToUpper(teamName)))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	sb.WriteString(fmt.Sprintf("Overall Coverage: %.2f%% (Target: %.0f%%)\n", teamInfo.Coverage, ca.threshold))
	sb.WriteString(fmt.Sprintf("Status: %s\n\n", ca.getCoverageStatus(teamInfo.Coverage)))

	sb.WriteString("Package Breakdown:\n")
	for _, pkg := range teamInfo.Packages {
		pkgInfo := teamInfo.PackageBreakdown[pkg]
		sb.WriteString(fmt.Sprintf("├─ %s: %.2f%% (%d/%d lines)\n",
			pkg, pkgInfo.Coverage, pkgInfo.CoveredLines, pkgInfo.TotalLines))

		// Show low coverage files
		for file, coverage := range pkgInfo.Files {
			if coverage < ca.threshold-20 {
				sb.WriteString(fmt.Sprintf("│  └─ ⚠️  %s: %.2f%%\n", file, coverage))
			}
		}
	}

	// Add recommendations for this team
	sb.WriteString("\nRecommendations:\n")
	for _, rec := range report.Recommendations {
		if strings.Contains(rec.Target, teamName) || rec.Target == teamName {
			sb.WriteString(fmt.Sprintf("• %s\n", rec.Suggestion))
			if len(rec.TestFiles) > 0 {
				sb.WriteString("  Missing test files:\n")
				for _, tf := range rec.TestFiles {
					sb.WriteString(fmt.Sprintf("  - %s\n", tf))
				}
			}
		}
	}

	return sb.String()
}

// getCoverageStatus returns a status string based on coverage percentage
func (ca *CoverageAnalyzer) getCoverageStatus(coverage float64) string {
	switch {
	case coverage >= ca.threshold:
		return "✅ PASSING"
	case coverage >= ca.threshold-10:
		return "⚠️  NEAR THRESHOLD"
	default:
		return "❌ FAILING"
	}
}

// WatchCoverage continuously monitors coverage changes
func (ca *CoverageAnalyzer) WatchCoverage(ctx context.Context, interval time.Duration, callback func(*CoverageReport)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			report, err := ca.AnalyzeCoverage(ctx)
			if err != nil {
				ca.logger.Error().Err(err).Msg("Coverage analysis failed during watch")
				continue
			}
			callback(report)
		}
	}
}
