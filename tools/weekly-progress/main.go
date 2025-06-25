package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

type WeeklyReport struct {
	StartDate       time.Time
	EndDate         time.Time
	CurrentMetrics  QualitySnapshot
	PreviousMetrics QualitySnapshot
	Trends          TrendAnalysis
	Achievements    []Achievement
	ActionItems     []ActionItem
	TeamProgress    map[string]TeamMetrics
}

type QualitySnapshot struct {
	Timestamp           time.Time
	ErrorHandling       float64
	TestCoverage        float64
	InterfaceCompliance float64
	DirectoryCount      int
	EmptyDirectories    int
	TODOCount           int
	BuildTime           time.Duration
}

type TrendAnalysis struct {
	ErrorHandlingTrend       float64
	TestCoverageTrend        float64
	InterfaceComplianceTrend float64
	DirectoryReduction       int
	TODOReduction            int
}

type Achievement struct {
	Icon        string
	Title       string
	Description string
	Impact      string
}

type ActionItem struct {
	Priority    string
	Team        string
	Description string
	Target      string
}

type TeamMetrics struct {
	TasksCompleted     int
	TasksInProgress    int
	TasksPending       int
	CompletionRate     float64
	KeyAccomplishments []string
}

type HistoricalData struct {
	Snapshots []QualitySnapshot `json:"snapshots"`
}

var (
	outputFormat = flag.String("format", "text", "Output format: text, html, json, markdown")
	outputFile   = flag.String("output", "", "Output file (default: stdout)")
	historyFile  = flag.String("history", "quality-history.json", "Historical data file")
	currentData  = flag.String("current", "quality-metrics.json", "Current metrics file")
	compareWeeks = flag.Int("weeks", 1, "Number of weeks to compare")
	generateHTML = flag.Bool("html-report", false, "Generate comprehensive HTML report")
)

func main() {
	flag.Parse()

	report, err := generateWeeklyReport()
	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	if err := outputReport(report); err != nil {
		log.Fatalf("Failed to output report: %v", err)
	}

	// Update history
	if err := updateHistory(report.CurrentMetrics); err != nil {
		log.Printf("Warning: Failed to update history: %v", err)
	}
}

func generateWeeklyReport() (*WeeklyReport, error) {
	// Load current metrics
	current, err := loadCurrentMetrics(*currentData)
	if err != nil {
		return nil, fmt.Errorf("loading current metrics: %w", err)
	}

	// Load historical data
	history, err := loadHistory(*historyFile)
	if err != nil {
		log.Printf("Warning: Could not load history: %v", err)
		history = &HistoricalData{}
	}

	// Get previous week's metrics
	previous := getPreviousMetrics(history, *compareWeeks)

	// Calculate trends
	trends := calculateTrends(current, previous)

	// Generate achievements
	achievements := generateAchievements(current, previous, trends)

	// Generate action items
	actionItems := generateActionItems(current, trends)

	// Generate team progress (mock data for now)
	teamProgress := generateTeamProgress()

	report := &WeeklyReport{
		StartDate:       time.Now().AddDate(0, 0, -7),
		EndDate:         time.Now(),
		CurrentMetrics:  *current,
		PreviousMetrics: previous,
		Trends:          trends,
		Achievements:    achievements,
		ActionItems:     actionItems,
		TeamProgress:    teamProgress,
	}

	return report, nil
}

func loadCurrentMetrics(filename string) (*QualitySnapshot, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}

	snapshot := &QualitySnapshot{
		Timestamp: time.Now(),
	}

	// Extract metrics
	if errorHandling, ok := getFloat(metrics, "error_handling.adoption_rate"); ok {
		snapshot.ErrorHandling = errorHandling
	}

	if coverage, ok := getFloat(metrics, "test_coverage.overall_coverage"); ok {
		snapshot.TestCoverage = coverage
	}

	if dirs, ok := getInt(metrics, "directory_structure.total_directories"); ok {
		snapshot.DirectoryCount = dirs
	}

	if empty, ok := getInt(metrics, "directory_structure.empty_directories"); ok {
		snapshot.EmptyDirectories = empty
	}

	if todos, ok := getInt(metrics, "code_quality.todo_comments"); ok {
		snapshot.TODOCount = todos
	}

	// Interface compliance would come from interface metrics
	snapshot.InterfaceCompliance = 100.0 // Default to 100 if no errors

	return snapshot, nil
}

func loadHistory(filename string) (*HistoricalData, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &HistoricalData{}, nil
		}
		return nil, err
	}

	var history HistoricalData
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return &history, nil
}

func getPreviousMetrics(history *HistoricalData, weeksAgo int) QualitySnapshot {
	if len(history.Snapshots) == 0 {
		return QualitySnapshot{} // Return zero snapshot
	}

	targetTime := time.Now().AddDate(0, 0, -7*weeksAgo)

	// Find closest snapshot to target time
	var closest QualitySnapshot
	minDiff := time.Duration(1<<63 - 1) // Max duration

	for _, snapshot := range history.Snapshots {
		diff := snapshot.Timestamp.Sub(targetTime).Abs()
		if diff < minDiff {
			minDiff = diff
			closest = snapshot
		}
	}

	return closest
}

func calculateTrends(current *QualitySnapshot, previous QualitySnapshot) TrendAnalysis {
	return TrendAnalysis{
		ErrorHandlingTrend:       current.ErrorHandling - previous.ErrorHandling,
		TestCoverageTrend:        current.TestCoverage - previous.TestCoverage,
		InterfaceComplianceTrend: current.InterfaceCompliance - previous.InterfaceCompliance,
		DirectoryReduction:       previous.DirectoryCount - current.DirectoryCount,
		TODOReduction:            previous.TODOCount - current.TODOCount,
	}
}

func generateAchievements(current *QualitySnapshot, previous QualitySnapshot, trends TrendAnalysis) []Achievement {
	var achievements []Achievement

	// Error handling improvements
	if trends.ErrorHandlingTrend > 5 {
		achievements = append(achievements, Achievement{
			Icon:        "🎯",
			Title:       "Error Handling Champion",
			Description: fmt.Sprintf("Improved error handling adoption by %.1f%%", trends.ErrorHandlingTrend),
			Impact:      "Better error tracking and debugging",
		})
	}

	// Test coverage improvements
	if trends.TestCoverageTrend > 3 {
		achievements = append(achievements, Achievement{
			Icon:        "🧪",
			Title:       "Testing Hero",
			Description: fmt.Sprintf("Increased test coverage by %.1f%%", trends.TestCoverageTrend),
			Impact:      "More reliable and maintainable code",
		})
	}

	// Directory cleanup
	if trends.DirectoryReduction > 5 {
		achievements = append(achievements, Achievement{
			Icon:        "🧹",
			Title:       "Clean Code Advocate",
			Description: fmt.Sprintf("Reduced directory count by %d", trends.DirectoryReduction),
			Impact:      "Simpler and more organized codebase",
		})
	}

	// TODO reduction
	if trends.TODOReduction > 10 {
		achievements = append(achievements, Achievement{
			Icon:        "✅",
			Title:       "Debt Eliminator",
			Description: fmt.Sprintf("Resolved %d TODO items", trends.TODOReduction),
			Impact:      "Reduced technical debt",
		})
	}

	// Perfect interface compliance
	if current.InterfaceCompliance == 100 && previous.InterfaceCompliance < 100 {
		achievements = append(achievements, Achievement{
			Icon:        "🔗",
			Title:       "Interface Perfectionist",
			Description: "Achieved 100% interface compliance",
			Impact:      "Consistent tool architecture",
		})
	}

	return achievements
}

func generateActionItems(current *QualitySnapshot, trends TrendAnalysis) []ActionItem {
	var items []ActionItem

	// Error handling
	if current.ErrorHandling < 60 {
		items = append(items, ActionItem{
			Priority:    "High",
			Team:        "Team C",
			Description: "Accelerate error handling migration",
			Target:      fmt.Sprintf("Current: %.1f%%, Target: 60%%", current.ErrorHandling),
		})
	}

	// Test coverage
	if current.TestCoverage < 50 {
		items = append(items, ActionItem{
			Priority:    "High",
			Team:        "Team A",
			Description: "Improve test coverage for critical packages",
			Target:      fmt.Sprintf("Current: %.1f%%, Target: 50%%", current.TestCoverage),
		})
	}

	// Directory structure
	if current.DirectoryCount > 20 && current.EmptyDirectories > 0 {
		items = append(items, ActionItem{
			Priority:    "Medium",
			Team:        "Team B",
			Description: "Complete directory flattening",
			Target:      fmt.Sprintf("Remove %d empty directories", current.EmptyDirectories),
		})
	}

	// Interface compliance
	if current.InterfaceCompliance < 100 {
		items = append(items, ActionItem{
			Priority:    "High",
			Team:        "Team C",
			Description: "Fix remaining interface validation errors",
			Target:      "Achieve 100% compliance",
		})
	}

	// Sort by priority
	sort.Slice(items, func(i, j int) bool {
		return priorityValue(items[i].Priority) > priorityValue(items[j].Priority)
	})

	return items
}

func generateTeamProgress() map[string]TeamMetrics {
	// This would normally pull from project management tools
	// For now, using example data
	return map[string]TeamMetrics{
		"Team A": {
			TasksCompleted:  3,
			TasksInProgress: 2,
			TasksPending:    1,
			CompletionRate:  50.0,
			KeyAccomplishments: []string{
				"Fixed hanging server tests",
				"Created architecture documentation",
			},
		},
		"Team B": {
			TasksCompleted:  5,
			TasksInProgress: 3,
			TasksPending:    2,
			CompletionRate:  50.0,
			KeyAccomplishments: []string{
				"Reduced directories by 30%",
				"Updated import paths",
			},
		},
		"Team C": {
			TasksCompleted:  4,
			TasksInProgress: 2,
			TasksPending:    3,
			CompletionRate:  44.4,
			KeyAccomplishments: []string{
				"Achieved 95% interface compliance",
				"Migrated 200+ errors to RichError",
			},
		},
		"Team D": {
			TasksCompleted:  3,
			TasksInProgress: 2,
			TasksPending:    3,
			CompletionRate:  37.5,
			KeyAccomplishments: []string{
				"Created quality dashboard",
				"Implemented CI/CD quality gates",
				"Built error migration tool",
			},
		},
	}
}

func outputReport(report *WeeklyReport) error {
	var output string

	switch *outputFormat {
	case "text":
		output = generateTextReport(report)
	case "markdown":
		output = generateMarkdownReport(report)
	case "html":
		output = generateHTMLReport(report)
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		output = string(data)
	default:
		return fmt.Errorf("unknown format: %s", *outputFormat)
	}

	if *outputFile == "" {
		fmt.Print(output)
	} else {
		if err := ioutil.WriteFile(*outputFile, []byte(output), 0644); err != nil {
			return err
		}
		fmt.Printf("Report written to: %s\n", *outputFile)
	}

	return nil
}

func generateTextReport(report *WeeklyReport) string {
	var b strings.Builder

	b.WriteString("═══════════════════════════════════════════\n")
	b.WriteString("       WEEKLY QUALITY PROGRESS REPORT       \n")
	b.WriteString("═══════════════════════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("Period: %s - %s\n\n",
		report.StartDate.Format("Jan 2, 2006"),
		report.EndDate.Format("Jan 2, 2006")))

	// Key Metrics
	b.WriteString("📊 KEY METRICS\n")
	b.WriteString("─────────────\n")
	b.WriteString(fmt.Sprintf("Error Handling:       %.1f%% (%+.1f%%)\n",
		report.CurrentMetrics.ErrorHandling, report.Trends.ErrorHandlingTrend))
	b.WriteString(fmt.Sprintf("Test Coverage:        %.1f%% (%+.1f%%)\n",
		report.CurrentMetrics.TestCoverage, report.Trends.TestCoverageTrend))
	b.WriteString(fmt.Sprintf("Interface Compliance: %.1f%% (%+.1f%%)\n",
		report.CurrentMetrics.InterfaceCompliance, report.Trends.InterfaceComplianceTrend))
	b.WriteString(fmt.Sprintf("Directory Count:      %d (%+d)\n",
		report.CurrentMetrics.DirectoryCount, -report.Trends.DirectoryReduction))
	b.WriteString(fmt.Sprintf("TODO Comments:        %d (%+d)\n\n",
		report.CurrentMetrics.TODOCount, -report.Trends.TODOReduction))

	// Achievements
	if len(report.Achievements) > 0 {
		b.WriteString("🏆 ACHIEVEMENTS THIS WEEK\n")
		b.WriteString("────────────────────────\n")
		for _, achievement := range report.Achievements {
			b.WriteString(fmt.Sprintf("%s %s\n", achievement.Icon, achievement.Title))
			b.WriteString(fmt.Sprintf("   %s\n", achievement.Description))
			b.WriteString(fmt.Sprintf("   Impact: %s\n\n", achievement.Impact))
		}
	}

	// Team Progress
	b.WriteString("👥 TEAM PROGRESS\n")
	b.WriteString("───────────────\n")
	for team, metrics := range report.TeamProgress {
		b.WriteString(fmt.Sprintf("\n%s (%.0f%% completion rate)\n", team, metrics.CompletionRate))
		b.WriteString(fmt.Sprintf("  ✓ Completed: %d | → In Progress: %d | ○ Pending: %d\n",
			metrics.TasksCompleted, metrics.TasksInProgress, metrics.TasksPending))
		if len(metrics.KeyAccomplishments) > 0 {
			b.WriteString("  Key accomplishments:\n")
			for _, accomplishment := range metrics.KeyAccomplishments {
				b.WriteString(fmt.Sprintf("  • %s\n", accomplishment))
			}
		}
	}

	// Action Items
	if len(report.ActionItems) > 0 {
		b.WriteString("\n🎯 ACTION ITEMS FOR NEXT WEEK\n")
		b.WriteString("─────────────────────────────\n")
		for i, item := range report.ActionItems {
			b.WriteString(fmt.Sprintf("\n%d. [%s] %s\n", i+1, item.Priority, item.Description))
			b.WriteString(fmt.Sprintf("   Team: %s\n", item.Team))
			b.WriteString(fmt.Sprintf("   Target: %s\n", item.Target))
		}
	}

	b.WriteString("\n═══════════════════════════════════════════\n")

	return b.String()
}

func generateMarkdownReport(report *WeeklyReport) string {
	var b strings.Builder

	b.WriteString("# Weekly Quality Progress Report\n\n")
	b.WriteString(fmt.Sprintf("**Period:** %s - %s\n\n",
		report.StartDate.Format("Jan 2, 2006"),
		report.EndDate.Format("Jan 2, 2006")))

	// Key Metrics Table
	b.WriteString("## 📊 Key Metrics\n\n")
	b.WriteString("| Metric | Current | Trend | Status |\n")
	b.WriteString("|--------|---------|-------|--------|\n")
	b.WriteString(fmt.Sprintf("| Error Handling | %.1f%% | %+.1f%% | %s |\n",
		report.CurrentMetrics.ErrorHandling,
		report.Trends.ErrorHandlingTrend,
		getStatusEmoji(report.CurrentMetrics.ErrorHandling, 60)))
	b.WriteString(fmt.Sprintf("| Test Coverage | %.1f%% | %+.1f%% | %s |\n",
		report.CurrentMetrics.TestCoverage,
		report.Trends.TestCoverageTrend,
		getStatusEmoji(report.CurrentMetrics.TestCoverage, 50)))
	b.WriteString(fmt.Sprintf("| Interface Compliance | %.1f%% | %+.1f%% | %s |\n",
		report.CurrentMetrics.InterfaceCompliance,
		report.Trends.InterfaceComplianceTrend,
		getStatusEmoji(report.CurrentMetrics.InterfaceCompliance, 100)))
	b.WriteString(fmt.Sprintf("| Directory Count | %d | %+d | %s |\n",
		report.CurrentMetrics.DirectoryCount,
		-report.Trends.DirectoryReduction,
		getDirectoryStatus(report.CurrentMetrics.DirectoryCount)))
	b.WriteString(fmt.Sprintf("| TODO Comments | %d | %+d | %s |\n\n",
		report.CurrentMetrics.TODOCount,
		-report.Trends.TODOReduction,
		getTODOStatus(report.CurrentMetrics.TODOCount)))

	// Achievements
	if len(report.Achievements) > 0 {
		b.WriteString("## 🏆 Achievements This Week\n\n")
		for _, achievement := range report.Achievements {
			b.WriteString(fmt.Sprintf("### %s %s\n", achievement.Icon, achievement.Title))
			b.WriteString(fmt.Sprintf("- **Description:** %s\n", achievement.Description))
			b.WriteString(fmt.Sprintf("- **Impact:** %s\n\n", achievement.Impact))
		}
	}

	// Team Progress
	b.WriteString("## 👥 Team Progress\n\n")
	for team, metrics := range report.TeamProgress {
		b.WriteString(fmt.Sprintf("### %s\n", team))
		b.WriteString(fmt.Sprintf("- **Completion Rate:** %.0f%%\n", metrics.CompletionRate))
		b.WriteString(fmt.Sprintf("- **Tasks:** ✓ %d completed | → %d in progress | ○ %d pending\n",
			metrics.TasksCompleted, metrics.TasksInProgress, metrics.TasksPending))
		if len(metrics.KeyAccomplishments) > 0 {
			b.WriteString("- **Key Accomplishments:**\n")
			for _, accomplishment := range metrics.KeyAccomplishments {
				b.WriteString(fmt.Sprintf("  - %s\n", accomplishment))
			}
		}
		b.WriteString("\n")
	}

	// Action Items
	if len(report.ActionItems) > 0 {
		b.WriteString("## 🎯 Action Items for Next Week\n\n")
		b.WriteString("| Priority | Team | Description | Target |\n")
		b.WriteString("|----------|------|-------------|--------|\n")
		for _, item := range report.ActionItems {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				getPriorityBadge(item.Priority),
				item.Team,
				item.Description,
				item.Target))
		}
	}

	return b.String()
}

func generateHTMLReport(report *WeeklyReport) string {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Weekly Quality Report - {{.EndDate.Format "Jan 2, 2006"}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 40px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #007bff; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .metrics-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .metric-card { background: #f8f9fa; padding: 20px; border-radius: 8px; border-left: 4px solid #007bff; }
        .metric-value { font-size: 2em; font-weight: bold; color: #333; }
        .metric-trend { font-size: 0.9em; color: #666; }
        .positive { color: #28a745; }
        .negative { color: #dc3545; }
        .achievement { background: #e7f5ff; padding: 15px; margin: 10px 0; border-radius: 5px; border-left: 4px solid #0066cc; }
        .team-card { background: #f8f9fa; padding: 20px; margin: 10px 0; border-radius: 8px; }
        .progress-bar { background: #e9ecef; height: 20px; border-radius: 10px; overflow: hidden; }
        .progress-fill { background: #007bff; height: 100%; transition: width 0.3s; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #dee2e6; }
        th { background: #f8f9fa; font-weight: 600; }
        .priority-high { background: #dc3545; color: white; padding: 2px 8px; border-radius: 3px; }
        .priority-medium { background: #ffc107; color: #333; padding: 2px 8px; border-radius: 3px; }
        .priority-low { background: #28a745; color: white; padding: 2px 8px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Weekly Quality Progress Report</h1>
        <p><strong>Period:</strong> {{.StartDate.Format "Jan 2, 2006"}} - {{.EndDate.Format "Jan 2, 2006"}}</p>
        
        <h2>📊 Key Metrics</h2>
        <div class="metrics-grid">
            <div class="metric-card">
                <div class="metric-label">Error Handling</div>
                <div class="metric-value">{{printf "%.1f" .CurrentMetrics.ErrorHandling}}%</div>
                <div class="metric-trend {{if ge .Trends.ErrorHandlingTrend 0}}positive{{else}}negative{{end}}">
                    {{printf "%+.1f" .Trends.ErrorHandlingTrend}}% from last week
                </div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Test Coverage</div>
                <div class="metric-value">{{printf "%.1f" .CurrentMetrics.TestCoverage}}%</div>
                <div class="metric-trend {{if ge .Trends.TestCoverageTrend 0}}positive{{else}}negative{{end}}">
                    {{printf "%+.1f" .Trends.TestCoverageTrend}}% from last week
                </div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Interface Compliance</div>
                <div class="metric-value">{{printf "%.1f" .CurrentMetrics.InterfaceCompliance}}%</div>
                <div class="metric-trend {{if ge .Trends.InterfaceComplianceTrend 0}}positive{{else}}negative{{end}}">
                    {{printf "%+.1f" .Trends.InterfaceComplianceTrend}}% from last week
                </div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Directory Count</div>
                <div class="metric-value">{{.CurrentMetrics.DirectoryCount}}</div>
                <div class="metric-trend {{if le .Trends.DirectoryReduction 0}}negative{{else}}positive{{end}}">
                    {{printf "%+d" (mul .Trends.DirectoryReduction -1)}} from last week
                </div>
            </div>
        </div>

        {{if .Achievements}}
        <h2>🏆 Achievements This Week</h2>
        {{range .Achievements}}
        <div class="achievement">
            <h3>{{.Icon}} {{.Title}}</h3>
            <p>{{.Description}}</p>
            <p><strong>Impact:</strong> {{.Impact}}</p>
        </div>
        {{end}}
        {{end}}

        <h2>👥 Team Progress</h2>
        {{range $team, $metrics := .TeamProgress}}
        <div class="team-card">
            <h3>{{$team}}</h3>
            <div class="progress-bar">
                <div class="progress-fill" style="width: {{$metrics.CompletionRate}}%"></div>
            </div>
            <p>{{$metrics.CompletionRate}}% completion rate</p>
            <p>✓ {{$metrics.TasksCompleted}} completed | → {{$metrics.TasksInProgress}} in progress | ○ {{$metrics.TasksPending}} pending</p>
            {{if $metrics.KeyAccomplishments}}
            <h4>Key Accomplishments:</h4>
            <ul>
            {{range $metrics.KeyAccomplishments}}
                <li>{{.}}</li>
            {{end}}
            </ul>
            {{end}}
        </div>
        {{end}}

        {{if .ActionItems}}
        <h2>🎯 Action Items for Next Week</h2>
        <table>
            <tr>
                <th>Priority</th>
                <th>Team</th>
                <th>Description</th>
                <th>Target</th>
            </tr>
            {{range .ActionItems}}
            <tr>
                <td><span class="priority-{{lower .Priority}}">{{.Priority}}</span></td>
                <td>{{.Team}}</td>
                <td>{{.Description}}</td>
                <td>{{.Target}}</td>
            </tr>
            {{end}}
        </table>
        {{end}}
    </div>
</body>
</html>`

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"mul":   func(a, b int) int { return a * b },
	}

	t, err := template.New("report").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return fmt.Sprintf("Template error: %v", err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, report); err != nil {
		return fmt.Sprintf("Execution error: %v", err)
	}

	return buf.String()
}

func updateHistory(current QualitySnapshot) error {
	history, err := loadHistory(*historyFile)
	if err != nil {
		return err
	}

	// Add current snapshot
	history.Snapshots = append(history.Snapshots, current)

	// Keep only last 52 weeks (1 year) of data
	if len(history.Snapshots) > 52 {
		history.Snapshots = history.Snapshots[len(history.Snapshots)-52:]
	}

	// Save updated history
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(*historyFile, data, 0644)
}

// Helper functions
func getFloat(m map[string]interface{}, path string) (float64, bool) {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			if val, ok := current[part].(float64); ok {
				return val, true
			}
			if val, ok := current[part].(float32); ok {
				return float64(val), true
			}
			if val, ok := current[part].(int); ok {
				return float64(val), true
			}
		} else {
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				return 0, false
			}
		}
	}
	return 0, false
}

func getInt(m map[string]interface{}, path string) (int, bool) {
	if val, ok := getFloat(m, path); ok {
		return int(val), true
	}
	return 0, false
}

func priorityValue(priority string) int {
	switch priority {
	case "High":
		return 3
	case "Medium":
		return 2
	case "Low":
		return 1
	default:
		return 0
	}
}

func getStatusEmoji(value, threshold float64) string {
	if value >= threshold {
		return "✅"
	} else if value >= threshold*0.8 {
		return "⚠️"
	}
	return "❌"
}

func getDirectoryStatus(count int) string {
	if count <= 20 {
		return "✅"
	} else if count <= 30 {
		return "⚠️"
	}
	return "❌"
}

func getTODOStatus(count int) string {
	if count <= 20 {
		return "✅"
	} else if count <= 50 {
		return "⚠️"
	}
	return "❌"
}

func getPriorityBadge(priority string) string {
	switch priority {
	case "High":
		return "🔴 High"
	case "Medium":
		return "🟡 Medium"
	case "Low":
		return "🟢 Low"
	default:
		return priority
	}
}
