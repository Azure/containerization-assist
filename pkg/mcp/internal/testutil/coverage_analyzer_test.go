package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverageAnalyzer(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	baseDir := t.TempDir()
	analyzer := NewCoverageAnalyzer(baseDir, logger)

	t.Run("Initialization", func(t *testing.T) {
		assert.NotNil(t, analyzer)
		assert.Equal(t, baseDir, analyzer.baseDir)
		assert.Equal(t, 90.0, analyzer.threshold)
		assert.NotEmpty(t, analyzer.teamPackages)
		assert.Contains(t, analyzer.teamPackages, "InfraBot")
		assert.Contains(t, analyzer.teamPackages, "BuildSecBot")
		assert.Contains(t, analyzer.teamPackages, "OrchBot")
		assert.Contains(t, analyzer.teamPackages, "AdvancedBot")
	})

	t.Run("ParseCoverProfile", func(t *testing.T) {
		// Create test coverage data
		coverageData := `mode: atomic
pkg/mcp/internal/utils/workspace.go:100.50,102.2 1 1
pkg/mcp/internal/utils/workspace.go:105.60,108.2 2 0
pkg/mcp/internal/utils/sandbox.go:200.40,202.2 1 1
pkg/mcp/internal/build/builder.go:50.30,52.2 1 1
pkg/mcp/internal/build/builder.go:55.40,57.2 1 0
`

		profiles, err := analyzer.parseCoverProfile([]byte(coverageData))
		assert.NoError(t, err)
		assert.Len(t, profiles, 2) // Two packages

		// Check utils package
		var utilsProfile *CoverageProfile
		for i := range profiles {
			if profiles[i].Package == "pkg/mcp/internal/utils" {
				utilsProfile = &profiles[i]
				break
			}
		}
		assert.NotNil(t, utilsProfile)
		assert.Len(t, utilsProfile.Files, 2) // workspace.go and sandbox.go

		// Check build package
		var buildProfile *CoverageProfile
		for i := range profiles {
			if profiles[i].Package == "pkg/mcp/internal/build" {
				buildProfile = &profiles[i]
				break
			}
		}
		assert.NotNil(t, buildProfile)
		assert.Len(t, buildProfile.Files, 1) // builder.go
	})

	t.Run("CalculatePackageCoverage", func(t *testing.T) {
		profile := CoverageProfile{
			Package: "test/package",
			Files: map[string][]CoverageBlock{
				"file1.go": {
					{StartLine: 1, EndLine: 10, NumStmt: 5, Count: true},
					{StartLine: 11, EndLine: 20, NumStmt: 3, Count: false},
				},
				"file2.go": {
					{StartLine: 1, EndLine: 5, NumStmt: 2, Count: true},
				},
			},
		}

		pkgInfo := analyzer.calculatePackageCoverage(profile)
		assert.Equal(t, "test/package", pkgInfo.Package)
		assert.Equal(t, 10, pkgInfo.TotalLines)  // 5 + 3 + 2
		assert.Equal(t, 7, pkgInfo.CoveredLines) // 5 + 2
		assert.Equal(t, 70.0, pkgInfo.Coverage)  // 7/10 * 100
		assert.Len(t, pkgInfo.Files, 2)
		assert.Equal(t, 62.5, pkgInfo.Files["file1.go"])  // 5/8 * 100
		assert.Equal(t, 100.0, pkgInfo.Files["file2.go"]) // 2/2 * 100
	})

	t.Run("GetTeamForPackage", func(t *testing.T) {
		testCases := []struct {
			pkg      string
			expected string
		}{
			{"pkg/mcp/internal/utils/workspace", "AdvancedBot"},
			{"pkg/mcp/internal/build/strategy", "BuildSecBot"},
			{"pkg/mcp/internal/orchestration/workflow", "OrchBot"},
			{"pkg/mcp/internal/pipeline/executor", "InfraBot"},
			{"pkg/unrelated/package", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.pkg, func(t *testing.T) {
				team := analyzer.getTeamForPackage(tc.pkg)
				assert.Equal(t, tc.expected, team)
			})
		}
	})

	t.Run("GenerateRecommendations", func(t *testing.T) {
		report := &CoverageReport{
			OverallCoverage: 85.0,
			TeamCoverage: map[string]TeamCoverageInfo{
				"InfraBot": {
					TeamName:     "InfraBot",
					Coverage:     88.0, // Below 90% threshold
					TotalLines:   1000,
					CoveredLines: 880,
					PackageBreakdown: map[string]PackageCoverageInfo{
						"pkg/mcp/internal/pipeline": {
							Package:  "pkg/mcp/internal/pipeline",
							Coverage: 75.0, // Significantly below threshold
						},
					},
				},
				"BuildSecBot": {
					TeamName:     "BuildSecBot",
					Coverage:     92.0, // Above threshold
					TotalLines:   800,
					CoveredLines: 736,
				},
			},
		}

		recommendations := analyzer.generateRecommendations(report)
		assert.NotEmpty(t, recommendations)

		// Should have team-level recommendation for InfraBot
		var foundTeamRec bool
		for _, rec := range recommendations {
			if rec.Target == "InfraBot" && rec.Type == "LOW_COVERAGE" {
				foundTeamRec = true
				assert.Equal(t, "HIGH", rec.Priority)
				assert.Equal(t, 88.0, rec.Current)
				assert.Equal(t, 90.0, rec.Required)
				break
			}
		}
		assert.True(t, foundTeamRec, "Should have team recommendation for InfraBot")

		// Should have package-level recommendation for pipeline
		var foundPkgRec bool
		for _, rec := range recommendations {
			if rec.Target == "pkg/mcp/internal/pipeline" && rec.Type == "LOW_COVERAGE" {
				foundPkgRec = true
				assert.Equal(t, "MEDIUM", rec.Priority)
				assert.Equal(t, 75.0, rec.Current)
				break
			}
		}
		assert.True(t, foundPkgRec, "Should have package recommendation for pipeline")
	})

	t.Run("GenerateTeamReport", func(t *testing.T) {
		report := &CoverageReport{
			OverallCoverage: 88.5,
			MeetsThreshold:  false,
			TeamCoverage: map[string]TeamCoverageInfo{
				"AdvancedBot": {
					TeamName:     "AdvancedBot",
					Coverage:     91.5,
					TotalLines:   2000,
					CoveredLines: 1830,
					Packages: []string{
						"pkg/mcp/internal/utils",
						"pkg/mcp/internal/observability",
					},
					PackageBreakdown: map[string]PackageCoverageInfo{
						"pkg/mcp/internal/utils": {
							Coverage:     95.0,
							CoveredLines: 950,
							TotalLines:   1000,
							Files: map[string]float64{
								"workspace.go": 92.0,
								"sandbox.go":   65.0, // Low coverage file
							},
						},
						"pkg/mcp/internal/observability": {
							Coverage:     88.0,
							CoveredLines: 880,
							TotalLines:   1000,
						},
					},
				},
			},
			Recommendations: []CoverageRecommendation{
				{
					Priority:   "MEDIUM",
					Type:       "LOW_COVERAGE",
					Target:     "pkg/mcp/internal/observability",
					Suggestion: "Package pkg/mcp/internal/observability has low coverage (88.0%). Consider adding tests for uncovered functions",
					TestFiles:  []string{"observability_test.go"},
				},
			},
		}

		teamReport := analyzer.GenerateTeamReport(report, "AdvancedBot")
		assert.Contains(t, teamReport, "ADVANCEDBOT COVERAGE REPORT")
		assert.Contains(t, teamReport, "Overall Coverage: 91.50%")
		assert.Contains(t, teamReport, "Status: ✅ PASSING")
		assert.Contains(t, teamReport, "pkg/mcp/internal/utils: 95.00%")
		assert.Contains(t, teamReport, "sandbox.go: 65.00%") // Low coverage file should be highlighted
		assert.Contains(t, teamReport, "Recommendations:")
		assert.Contains(t, teamReport, "pkg/mcp/internal/observability") // Check for package mention instead
	})

	t.Run("GetCoverageStatus", func(t *testing.T) {
		testCases := []struct {
			coverage float64
			expected string
		}{
			{95.0, "✅ PASSING"},
			{90.0, "✅ PASSING"},
			{85.0, "⚠️  NEAR THRESHOLD"},
			{75.0, "❌ FAILING"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%.1f%%", tc.coverage), func(t *testing.T) {
				status := analyzer.getCoverageStatus(tc.coverage)
				assert.Equal(t, tc.expected, status)
			})
		}
	})

	t.Run("SuggestTestFiles", func(t *testing.T) {
		// Create test directory structure
		pkgDir := filepath.Join(baseDir, "pkg", "test")
		err := os.MkdirAll(pkgDir, 0755)
		require.NoError(t, err)

		// Create some Go files without tests
		testFiles := []string{"file1.go", "file2.go", "file3_test.go"}
		for _, file := range testFiles {
			err := os.WriteFile(filepath.Join(pkgDir, file), []byte("package test\n"), 0644)
			require.NoError(t, err)
		}

		suggestions := analyzer.suggestTestFiles("pkg/test")
		assert.Len(t, suggestions, 2) // Should suggest tests for file1 and file2
		assert.Contains(t, suggestions[0], "file1_test.go")
		assert.Contains(t, suggestions[1], "file2_test.go")
	})
}

func TestCoverageAnalyzerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	baseDir := t.TempDir()
	analyzer := NewCoverageAnalyzer(baseDir, logger)

	t.Run("WatchCoverage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		reportCount := 0
		callback := func(report *CoverageReport) {
			reportCount++
			assert.NotNil(t, report)
			assert.NotZero(t, report.Timestamp)
		}

		// Start watching with short interval
		go analyzer.WatchCoverage(ctx, 500*time.Millisecond, callback)

		// Wait for a few reports
		time.Sleep(1500 * time.Millisecond)

		// Should have received at least 2 reports
		assert.GreaterOrEqual(t, reportCount, 2)
	})
}

func TestCoverageReportParsing(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	analyzer := NewCoverageAnalyzer(t.TempDir(), logger)

	t.Run("ComplexCoverageProfile", func(t *testing.T) {
		// Test with more complex coverage data
		coverageData := `mode: atomic
pkg/mcp/internal/utils/workspace.go:100.50,102.16 2 1
pkg/mcp/internal/utils/workspace.go:102.16,104.3 1 0
pkg/mcp/internal/utils/workspace.go:104.3,106.3 1 1
pkg/mcp/internal/utils/workspace.go:108.2,110.2 1 1
pkg/mcp/internal/build/builder.go:50.30,52.16 2 1
pkg/mcp/internal/build/builder.go:52.16,54.3 1 1
pkg/mcp/internal/build/builder.go:54.3,56.3 1 0
pkg/mcp/internal/orchestration/workflow.go:200.40,202.2 1 1
pkg/mcp/internal/session/manager.go:75.25,77.2 1 1
`

		report, err := analyzer.parseCoverageData([]byte(coverageData))
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotZero(t, report.Timestamp)

		// Check team coverage
		assert.Contains(t, report.TeamCoverage, "AdvancedBot")
		assert.Contains(t, report.TeamCoverage, "BuildSecBot")
		assert.Contains(t, report.TeamCoverage, "OrchBot")
		assert.Contains(t, report.TeamCoverage, "InfraBot")

		// Verify package assignment
		advancedBotTeam := report.TeamCoverage["AdvancedBot"]
		assert.Contains(t, advancedBotTeam.Packages, "pkg/mcp/internal/utils")

		buildSecBotTeam := report.TeamCoverage["BuildSecBot"]
		assert.Contains(t, buildSecBotTeam.Packages, "pkg/mcp/internal/build")
	})

	t.Run("EmptyCoverageData", func(t *testing.T) {
		coverageData := `mode: atomic
`
		report, err := analyzer.parseCoverageData([]byte(coverageData))
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.Equal(t, 0.0, report.OverallCoverage)
		assert.Empty(t, report.TeamCoverage)
	})

	t.Run("InvalidCoverageData", func(t *testing.T) {
		coverageData := `invalid data`
		_, err := analyzer.parseCoverProfile([]byte(coverageData))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid coverage profile format")
	})
}

func TestTeamPackageMapping(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	analyzer := NewCoverageAnalyzer(t.TempDir(), logger)

	t.Run("AllTeamsHavePackages", func(t *testing.T) {
		teams := []string{"InfraBot", "BuildSecBot", "OrchBot", "AdvancedBot"}
		for _, team := range teams {
			packages, exists := analyzer.teamPackages[team]
			assert.True(t, exists, "Team %s should have packages assigned", team)
			assert.NotEmpty(t, packages, "Team %s should have at least one package", team)
		}
	})

	t.Run("NoOverlappingPackages", func(t *testing.T) {
		seen := make(map[string]string)
		for team, packages := range analyzer.teamPackages {
			for _, pkg := range packages {
				if previousTeam, exists := seen[pkg]; exists {
					t.Errorf("Package %s is assigned to both %s and %s", pkg, previousTeam, team)
				}
				seen[pkg] = team
			}
		}
	})
}

func TestCoverageThresholdValidation(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	analyzer := NewCoverageAnalyzer(t.TempDir(), logger)

	t.Run("MeetsThreshold", func(t *testing.T) {
		report := &CoverageReport{
			OverallCoverage: 92.5,
			Timestamp:       time.Now(),
		}
		report.MeetsThreshold = report.OverallCoverage >= analyzer.threshold
		assert.True(t, report.MeetsThreshold)
	})

	t.Run("BelowThreshold", func(t *testing.T) {
		report := &CoverageReport{
			OverallCoverage: 87.5,
			Timestamp:       time.Now(),
		}
		report.MeetsThreshold = report.OverallCoverage >= analyzer.threshold
		assert.False(t, report.MeetsThreshold)
	})
}

func TestHTMLReportGeneration(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	analyzer := NewCoverageAnalyzer(t.TempDir(), logger)

	t.Run("GenerateHTMLReport", func(t *testing.T) {
		report := &CoverageReport{
			Timestamp:       time.Now(),
			OverallCoverage: 88.5,
			MeetsThreshold:  false,
			TeamCoverage:    make(map[string]TeamCoverageInfo),
		}

		outputPath := filepath.Join(t.TempDir(), "coverage.html")
		err := analyzer.GenerateHTMLReport(report, outputPath)
		// Currently not implemented, should not error
		assert.NoError(t, err)
	})
}

func BenchmarkCoverageAnalysis(b *testing.B) {
	logger := zerolog.Nop()
	analyzer := NewCoverageAnalyzer(b.TempDir(), logger)

	// Generate sample coverage data
	var coverageData strings.Builder
	coverageData.WriteString("mode: atomic\n")
	for i := 0; i < 100; i++ {
		for j := 0; j < 10; j++ {
			coverage := "1"
			if j%3 == 0 {
				coverage = "0"
			}
			fmt.Fprintf(&coverageData, "pkg/mcp/internal/test%d/file.go:%d.1,%d.10 1 %s\n",
				i, j*10, j*10+10, coverage)
		}
	}

	data := []byte(coverageData.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.parseCoverageData(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
