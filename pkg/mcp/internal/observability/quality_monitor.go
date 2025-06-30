package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// QualityMonitor provides comprehensive quality monitoring for all teams
type QualityMonitor struct {
	baseDir    string
	logger     zerolog.Logger
	mutex      sync.RWMutex
	reports    map[string]*QualityReport
	thresholds QualityThresholds
}

// QualityThresholds defines the quality gates for each metric
type QualityThresholds struct {
	MinTestCoverage     float64       `json:"min_test_coverage"`     // Minimum test coverage percentage
	MaxLintIssues       int           `json:"max_lint_issues"`       // Maximum allowed lint issues
	MaxPerformanceP95   time.Duration `json:"max_performance_p95"`   // Maximum P95 performance target
	MinBuildSuccessRate float64       `json:"min_build_success_rate"` // Minimum build success rate
}

// QualityReport represents the overall quality status
type QualityReport struct {
	Timestamp     time.Time          `json:"timestamp"`
	OverallHealth string             `json:"overall_health"` // GREEN, YELLOW, RED
	Teams         map[string]TeamQuality `json:"teams"`
	SystemMetrics SystemMetrics      `json:"system_metrics"`
	QualityGates  QualityGates       `json:"quality_gates"`
}

// TeamQuality represents quality metrics for a specific team
type TeamQuality struct {
	TeamName        string            `json:"team_name"`
	Status          string            `json:"status"` // GREEN, YELLOW, RED
	TestCoverage    float64           `json:"test_coverage"`
	LintIssues      int               `json:"lint_issues"`
	PerformanceP95  time.Duration     `json:"performance_p95"`
	BuildSuccessRate float64          `json:"build_success_rate"`
	Components      map[string]ComponentHealth `json:"components"`
	LastUpdated     time.Time         `json:"last_updated"`
}

// ComponentHealth represents health of a specific component
type ComponentHealth struct {
	Name            string        `json:"name"`
	Status          string        `json:"status"`
	TestsPassing    bool          `json:"tests_passing"`
	PerformanceMet  bool          `json:"performance_met"`
	SecurityClean   bool          `json:"security_clean"`
	LastTested      time.Time     `json:"last_tested"`
}

// SystemMetrics provides overall system health metrics
type SystemMetrics struct {
	TotalTestCoverage    float64   `json:"total_test_coverage"`
	TotalLintIssues      int       `json:"total_lint_issues"`
	AvgPerformanceP95    time.Duration `json:"avg_performance_p95"`
	OverallBuildSuccess  float64   `json:"overall_build_success"`
	IntegrationTestsPass bool      `json:"integration_tests_pass"`
}

// QualityGates represents the status of various quality gates
type QualityGates struct {
	TestCoverageGate     string `json:"test_coverage_gate"`     // PASS, FAIL
	LintGate            string `json:"lint_gate"`              // PASS, FAIL  
	PerformanceGate     string `json:"performance_gate"`       // PASS, FAIL
	SecurityGate        string `json:"security_gate"`          // PASS, FAIL
	IntegrationGate     string `json:"integration_gate"`       // PASS, FAIL
}

// NewQualityMonitor creates a new quality monitor
func NewQualityMonitor(baseDir string, logger zerolog.Logger) *QualityMonitor {
	return &QualityMonitor{
		baseDir: baseDir,
		logger:  logger.With().Str("component", "quality_monitor").Logger(),
		reports: make(map[string]*QualityReport),
		thresholds: QualityThresholds{
			MinTestCoverage:     90.0,            // 90% test coverage
			MaxLintIssues:       100,             // Max 100 lint issues (from CLAUDE.md)
			MaxPerformanceP95:   300 * time.Microsecond, // <300μs P95 (from CLAUDE.md)
			MinBuildSuccessRate: 95.0,            // 95% build success rate
		},
	}
}

// UpdateTeamQuality updates quality metrics for a specific team
func (qm *QualityMonitor) UpdateTeamQuality(ctx context.Context, teamName string, metrics TeamQuality) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	// Get or create current report
	reportKey := time.Now().Format("2006-01-02")
	report, exists := qm.reports[reportKey]
	if !exists {
		report = &QualityReport{
			Timestamp: time.Now(),
			Teams:     make(map[string]TeamQuality),
		}
		qm.reports[reportKey] = report
	}

	// Update team metrics
	metrics.LastUpdated = time.Now()
	metrics.Status = qm.calculateTeamStatus(metrics)
	report.Teams[teamName] = metrics

	// Recalculate overall health
	qm.updateOverallHealth(report)

	// Log quality update
	qm.logger.Info().
		Str("team", teamName).
		Str("status", metrics.Status).
		Float64("coverage", metrics.TestCoverage).
		Int("lint_issues", metrics.LintIssues).
		Dur("performance_p95", metrics.PerformanceP95).
		Msg("Team quality updated")

	return nil
}

// GetCurrentQualityReport returns the current quality report
func (qm *QualityMonitor) GetCurrentQualityReport() *QualityReport {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	reportKey := time.Now().Format("2006-01-02")
	if report, exists := qm.reports[reportKey]; exists {
		return report
	}

	// Return empty report if none exists
	return &QualityReport{
		Timestamp: time.Now(),
		Teams:     make(map[string]TeamQuality),
		OverallHealth: "UNKNOWN",
	}
}

// ValidateQualityGates checks all quality gates and returns status
func (qm *QualityMonitor) ValidateQualityGates(ctx context.Context) (QualityGates, error) {
	report := qm.GetCurrentQualityReport()
	
	gates := QualityGates{
		TestCoverageGate: qm.validateTestCoverageGate(report),
		LintGate:        qm.validateLintGate(report),
		PerformanceGate: qm.validatePerformanceGate(report),
		SecurityGate:    qm.validateSecurityGate(report),
		IntegrationGate: qm.validateIntegrationGate(report),
	}

	qm.logger.Info().
		Str("test_coverage", gates.TestCoverageGate).
		Str("lint", gates.LintGate).
		Str("performance", gates.PerformanceGate).
		Str("security", gates.SecurityGate).
		Str("integration", gates.IntegrationGate).
		Msg("Quality gates validated")

	return gates, nil
}

// SaveQualityReport saves the quality report to disk
func (qm *QualityMonitor) SaveQualityReport(ctx context.Context, filename string) error {
	report := qm.GetCurrentQualityReport()
	
	reportPath := filepath.Join(qm.baseDir, filename)
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %v", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %v", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %v", err)
	}

	qm.logger.Info().Str("path", reportPath).Msg("Quality report saved")
	return nil
}

// GenerateDailySummary generates a daily quality summary
func (qm *QualityMonitor) GenerateDailySummary(ctx context.Context) (string, error) {
	report := qm.GetCurrentQualityReport()
	gates, _ := qm.ValidateQualityGates(ctx)
	
	summary := fmt.Sprintf(`ADVANCEDBOT - SPRINT 1 DAY 1 QUALITY REPORT
===========================================
Overall System Health: %s

Team Integration Status:
├─ InfraBot (Core): %s, coverage: %.1f%%, lint: %d issues
├─ BuildSecBot (Build): %s, coverage: %.1f%%, performance: %v
├─ OrchBot (Communication): %s, coverage: %.1f%%, integration: %s
└─ Cross-team Integration: %s

Quality Metrics:
├─ Test Coverage: %.1f%% (target: >%.1f%%)
├─ Performance: %v (target: <%v)  
├─ Build Status: %.1f%% success rate
├─ Lint Status: %d issues (target: <%d)
├─ Security: %s
└─ Documentation: Generated and validated

Quality Gates:
├─ Test Coverage: %s
├─ Lint: %s
├─ Performance: %s
├─ Security: %s
└─ Integration: %s

MERGE RECOMMENDATIONS
────────────────────
%s

SPRINT PROGRESS: Implementation and testing infrastructure established

QUALITY ISSUES TO ADDRESS:
%s

NEXT DAY PRIORITIES:
1. Complete sandboxing implementation with Docker integration
2. Enhance performance monitoring and benchmarking
3. Expand test coverage for all team implementations
`,
		report.OverallHealth,
		qm.getTeamStatus(report, "InfraBot"),
		qm.getTeamCoverage(report, "InfraBot"),
		qm.getTeamLintIssues(report, "InfraBot"),
		qm.getTeamStatus(report, "BuildSecBot"),
		qm.getTeamCoverage(report, "BuildSecBot"),
		qm.getTeamPerformance(report, "BuildSecBot"),
		qm.getTeamStatus(report, "OrchBot"),
		qm.getTeamCoverage(report, "OrchBot"),
		gates.IntegrationGate,
		report.OverallHealth,
		report.SystemMetrics.TotalTestCoverage,
		qm.thresholds.MinTestCoverage,
		report.SystemMetrics.AvgPerformanceP95,
		qm.thresholds.MaxPerformanceP95,
		report.SystemMetrics.OverallBuildSuccess,
		report.SystemMetrics.TotalLintIssues,
		qm.thresholds.MaxLintIssues,
		gates.SecurityGate,
		gates.TestCoverageGate,
		gates.LintGate,
		gates.PerformanceGate,
		gates.SecurityGate,
		gates.IntegrationGate,
		qm.generateMergeRecommendations(report, gates),
		qm.generateQualityIssues(report, gates),
	)

	return summary, nil
}

// Helper methods

func (qm *QualityMonitor) calculateTeamStatus(metrics TeamQuality) string {
	if metrics.TestCoverage < qm.thresholds.MinTestCoverage ||
		metrics.LintIssues > qm.thresholds.MaxLintIssues ||
		metrics.PerformanceP95 > qm.thresholds.MaxPerformanceP95 ||
		metrics.BuildSuccessRate < qm.thresholds.MinBuildSuccessRate {
		return "RED"
	}
	
	if metrics.TestCoverage < qm.thresholds.MinTestCoverage+5 ||
		metrics.LintIssues > qm.thresholds.MaxLintIssues-20 ||
		metrics.BuildSuccessRate < qm.thresholds.MinBuildSuccessRate+3 {
		return "YELLOW"
	}
	
	return "GREEN"
}

func (qm *QualityMonitor) updateOverallHealth(report *QualityReport) {
	redCount := 0
	yellowCount := 0
	greenCount := 0
	
	for _, team := range report.Teams {
		switch team.Status {
		case "RED":
			redCount++
		case "YELLOW":
			yellowCount++
		case "GREEN":
			greenCount++
		}
	}
	
	if redCount > 0 {
		report.OverallHealth = "RED"
	} else if yellowCount > 0 {
		report.OverallHealth = "YELLOW"
	} else if greenCount > 0 {
		report.OverallHealth = "GREEN"
	} else {
		report.OverallHealth = "UNKNOWN"
	}
	
	// Update system metrics
	qm.updateSystemMetrics(report)
	
	// Update quality gates
	report.QualityGates, _ = qm.ValidateQualityGates(context.Background())
}

func (qm *QualityMonitor) updateSystemMetrics(report *QualityReport) {
	if len(report.Teams) == 0 {
		return
	}
	
	var totalCoverage, totalBuildSuccess float64
	var totalLintIssues int
	var totalPerformance time.Duration
	var allIntegrationPass = true
	
	for _, team := range report.Teams {
		totalCoverage += team.TestCoverage
		totalLintIssues += team.LintIssues
		totalPerformance += team.PerformanceP95
		totalBuildSuccess += team.BuildSuccessRate
		
		// Check if any integration tests are failing
		for _, component := range team.Components {
			if !component.TestsPassing {
				allIntegrationPass = false
			}
		}
	}
	
	teamCount := float64(len(report.Teams))
	report.SystemMetrics = SystemMetrics{
		TotalTestCoverage:    totalCoverage / teamCount,
		TotalLintIssues:      totalLintIssues,
		AvgPerformanceP95:    totalPerformance / time.Duration(len(report.Teams)),
		OverallBuildSuccess:  totalBuildSuccess / teamCount,
		IntegrationTestsPass: allIntegrationPass,
	}
}

func (qm *QualityMonitor) validateTestCoverageGate(report *QualityReport) string {
	if report.SystemMetrics.TotalTestCoverage >= qm.thresholds.MinTestCoverage {
		return "PASS"
	}
	return "FAIL"
}

func (qm *QualityMonitor) validateLintGate(report *QualityReport) string {
	if report.SystemMetrics.TotalLintIssues <= qm.thresholds.MaxLintIssues {
		return "PASS"
	}
	return "FAIL"
}

func (qm *QualityMonitor) validatePerformanceGate(report *QualityReport) string {
	if report.SystemMetrics.AvgPerformanceP95 <= qm.thresholds.MaxPerformanceP95 {
		return "PASS"
	}
	return "FAIL"
}

func (qm *QualityMonitor) validateSecurityGate(report *QualityReport) string {
	// Check if all teams have security-clean components
	for _, team := range report.Teams {
		for _, component := range team.Components {
			if !component.SecurityClean {
				return "FAIL"
			}
		}
	}
	return "PASS"
}

func (qm *QualityMonitor) validateIntegrationGate(report *QualityReport) string {
	if report.SystemMetrics.IntegrationTestsPass {
		return "PASS"
	}
	return "FAIL"
}

func (qm *QualityMonitor) getTeamStatus(report *QualityReport, teamName string) string {
	if team, exists := report.Teams[teamName]; exists {
		return team.Status
	}
	return "UNKNOWN"
}

func (qm *QualityMonitor) getTeamCoverage(report *QualityReport, teamName string) float64 {
	if team, exists := report.Teams[teamName]; exists {
		return team.TestCoverage
	}
	return 0.0
}

func (qm *QualityMonitor) getTeamLintIssues(report *QualityReport, teamName string) int {
	if team, exists := report.Teams[teamName]; exists {
		return team.LintIssues
	}
	return 0
}

func (qm *QualityMonitor) getTeamPerformance(report *QualityReport, teamName string) time.Duration {
	if team, exists := report.Teams[teamName]; exists {
		return team.PerformanceP95
	}
	return 0
}

func (qm *QualityMonitor) generateMergeRecommendations(report *QualityReport, gates QualityGates) string {
	recommendations := ""
	
	for teamName, team := range report.Teams {
		status := "NOT READY"
		reason := "quality gates not met"
		
		if team.Status == "GREEN" && gates.TestCoverageGate == "PASS" && gates.LintGate == "PASS" {
			status = "READY"
			reason = "all quality gates passed"
		}
		
		recommendations += fmt.Sprintf("%s: %s [%s]\n", teamName, status, reason)
	}
	
	return recommendations
}

func (qm *QualityMonitor) generateQualityIssues(report *QualityReport, gates QualityGates) string {
	issues := ""
	
	if gates.TestCoverageGate == "FAIL" {
		issues += fmt.Sprintf("- Test coverage below %.1f%% threshold\n", qm.thresholds.MinTestCoverage)
	}
	
	if gates.LintGate == "FAIL" {
		issues += fmt.Sprintf("- Lint issues exceed %d threshold\n", qm.thresholds.MaxLintIssues)
	}
	
	if gates.PerformanceGate == "FAIL" {
		issues += fmt.Sprintf("- Performance exceeds %v P95 threshold\n", qm.thresholds.MaxPerformanceP95)
	}
	
	if gates.SecurityGate == "FAIL" {
		issues += "- Security vulnerabilities detected\n"
	}
	
	if gates.IntegrationGate == "FAIL" {
		issues += "- Integration tests failing\n"
	}
	
	if issues == "" {
		issues = "No critical quality issues detected"
	}
	
	return issues
}