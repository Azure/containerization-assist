package observability

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityMonitor(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	monitor := NewQualityMonitor(t.TempDir(), logger)
	ctx := context.Background()

	t.Run("InitialState", func(t *testing.T) {
		report := monitor.GetCurrentQualityReport()
		assert.Equal(t, "UNKNOWN", report.OverallHealth)
		assert.Empty(t, report.Teams)
	})

	t.Run("UpdateTeamQuality", func(t *testing.T) {
		// Test updating InfraBot quality
		infraBotMetrics := TeamQuality{
			TeamName:         "InfraBot",
			TestCoverage:     95.0,
			LintIssues:       50,
			PerformanceP95:   250 * time.Microsecond,
			BuildSuccessRate: 98.0,
			Components: map[string]ComponentHealth{
				"docker_operations": {
					Name:           "docker_operations",
					Status:         "GREEN",
					TestsPassing:   true,
					PerformanceMet: true,
					SecurityClean:  true,
					LastTested:     time.Now(),
				},
			},
		}

		err := monitor.UpdateTeamQuality(ctx, "InfraBot", infraBotMetrics)
		require.NoError(t, err)

		report := monitor.GetCurrentQualityReport()
		assert.Equal(t, "GREEN", report.OverallHealth)
		assert.Contains(t, report.Teams, "InfraBot")
		assert.Equal(t, "GREEN", report.Teams["InfraBot"].Status)
	})

	t.Run("QualityThresholds", func(t *testing.T) {
		// Test team that fails quality thresholds
		failingTeam := TeamQuality{
			TeamName:         "FailingBot",
			TestCoverage:     75.0,                   // Below 90% threshold
			LintIssues:       150,                    // Above 100 threshold
			PerformanceP95:   500 * time.Microsecond, // Above 300μs threshold
			BuildSuccessRate: 85.0,                   // Below 95% threshold
		}

		err := monitor.UpdateTeamQuality(ctx, "FailingBot", failingTeam)
		require.NoError(t, err)

		report := monitor.GetCurrentQualityReport()
		assert.Equal(t, "RED", report.OverallHealth)
		assert.Equal(t, "RED", report.Teams["FailingBot"].Status)
	})

	t.Run("QualityGatesValidation", func(t *testing.T) {
		// Add teams with mixed quality
		goodTeam := TeamQuality{
			TeamName:         "GoodBot",
			TestCoverage:     95.0,
			LintIssues:       20,
			PerformanceP95:   200 * time.Microsecond,
			BuildSuccessRate: 99.0,
			Components: map[string]ComponentHealth{
				"component1": {
					Name:          "component1",
					Status:        "GREEN",
					TestsPassing:  true,
					SecurityClean: true,
				},
			},
		}

		err := monitor.UpdateTeamQuality(ctx, "GoodBot", goodTeam)
		require.NoError(t, err)

		gates, err := monitor.ValidateQualityGates(ctx)
		require.NoError(t, err)

		// Should pass most gates due to good team metrics
		assert.Equal(t, "PASS", gates.TestCoverageGate)
		assert.Equal(t, "FAIL", gates.LintGate)        // Still fails due to FailingBot having 150 issues
		assert.Equal(t, "FAIL", gates.PerformanceGate) // Still fails due to FailingBot
		assert.Equal(t, "PASS", gates.SecurityGate)
	})

	t.Run("DailySummaryGeneration", func(t *testing.T) {
		summary, err := monitor.GenerateDailySummary(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, summary)

		// Verify summary contains expected sections
		assert.Contains(t, summary, "ADVANCEDBOT - SPRINT 1 DAY 1 QUALITY REPORT")
		assert.Contains(t, summary, "Overall System Health:")
		assert.Contains(t, summary, "Team Integration Status:")
		assert.Contains(t, summary, "Quality Metrics:")
		assert.Contains(t, summary, "Quality Gates:")
		assert.Contains(t, summary, "MERGE RECOMMENDATIONS")
		assert.Contains(t, summary, "QUALITY ISSUES TO ADDRESS:")
		assert.Contains(t, summary, "NEXT DAY PRIORITIES:")
	})

	t.Run("SaveQualityReport", func(t *testing.T) {
		filename := "quality_report_test.json"
		err := monitor.SaveQualityReport(ctx, filename)
		require.NoError(t, err)
	})
}

func TestQualityThresholds(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	monitor := NewQualityMonitor(t.TempDir(), logger)

	tests := []struct {
		name           string
		metrics        TeamQuality
		expectedStatus string
	}{
		{
			name: "AllGreen",
			metrics: TeamQuality{
				TestCoverage:     95.0,
				LintIssues:       50,
				PerformanceP95:   200 * time.Microsecond,
				BuildSuccessRate: 98.0,
			},
			expectedStatus: "GREEN",
		},
		{
			name: "YellowCoverage",
			metrics: TeamQuality{
				TestCoverage:     92.0, // Just above threshold but in yellow range
				LintIssues:       50,
				PerformanceP95:   200 * time.Microsecond,
				BuildSuccessRate: 96.0,
			},
			expectedStatus: "YELLOW",
		},
		{
			name: "RedCoverage",
			metrics: TeamQuality{
				TestCoverage:     85.0, // Below 90% threshold
				LintIssues:       50,
				PerformanceP95:   200 * time.Microsecond,
				BuildSuccessRate: 98.0,
			},
			expectedStatus: "RED",
		},
		{
			name: "RedLint",
			metrics: TeamQuality{
				TestCoverage:     95.0,
				LintIssues:       150, // Above 100 threshold
				PerformanceP95:   200 * time.Microsecond,
				BuildSuccessRate: 98.0,
			},
			expectedStatus: "RED",
		},
		{
			name: "RedPerformance",
			metrics: TeamQuality{
				TestCoverage:     95.0,
				LintIssues:       50,
				PerformanceP95:   400 * time.Microsecond, // Above 300μs threshold
				BuildSuccessRate: 98.0,
			},
			expectedStatus: "RED",
		},
		{
			name: "RedBuildSuccess",
			metrics: TeamQuality{
				TestCoverage:     95.0,
				LintIssues:       50,
				PerformanceP95:   200 * time.Microsecond,
				BuildSuccessRate: 90.0, // Below 95% threshold
			},
			expectedStatus: "RED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := monitor.calculateTeamStatus(tt.metrics)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func TestSystemMetricsCalculation(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	monitor := NewQualityMonitor(t.TempDir(), logger)
	ctx := context.Background()

	// Add multiple teams
	teams := []TeamQuality{
		{
			TeamName:         "Team1",
			TestCoverage:     90.0,
			LintIssues:       30,
			PerformanceP95:   200 * time.Microsecond,
			BuildSuccessRate: 95.0,
			Components: map[string]ComponentHealth{
				"comp1": {TestsPassing: true, SecurityClean: true},
			},
		},
		{
			TeamName:         "Team2",
			TestCoverage:     85.0,
			LintIssues:       40,
			PerformanceP95:   300 * time.Microsecond,
			BuildSuccessRate: 90.0,
			Components: map[string]ComponentHealth{
				"comp2": {TestsPassing: true, SecurityClean: true},
			},
		},
		{
			TeamName:         "Team3",
			TestCoverage:     95.0,
			LintIssues:       20,
			PerformanceP95:   150 * time.Microsecond,
			BuildSuccessRate: 98.0,
			Components: map[string]ComponentHealth{
				"comp3": {TestsPassing: false, SecurityClean: true}, // Failing test
			},
		},
	}

	for _, team := range teams {
		err := monitor.UpdateTeamQuality(ctx, team.TeamName, team)
		require.NoError(t, err)
	}

	report := monitor.GetCurrentQualityReport()

	// Check system metrics calculations
	expectedAvgCoverage := (90.0 + 85.0 + 95.0) / 3
	assert.InDelta(t, expectedAvgCoverage, report.SystemMetrics.TotalTestCoverage, 0.1)

	expectedTotalLint := 30 + 40 + 20
	assert.Equal(t, expectedTotalLint, report.SystemMetrics.TotalLintIssues)

	expectedAvgBuildSuccess := (95.0 + 90.0 + 98.0) / 3
	assert.InDelta(t, expectedAvgBuildSuccess, report.SystemMetrics.OverallBuildSuccess, 0.1)

	// Integration tests should fail due to Team3 having failing tests
	assert.False(t, report.SystemMetrics.IntegrationTestsPass)
}
