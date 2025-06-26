# Weekly Progress Report Generator

Automated tool for generating comprehensive weekly quality progress reports with trend analysis and team metrics.

## Features

- **Trend Analysis**: Compares current metrics with previous weeks
- **Achievement Recognition**: Automatically identifies and celebrates improvements
- **Action Items**: Generates prioritized action items based on current state
- **Team Progress**: Tracks progress by team (when integrated with project management)
- **Multiple Formats**: Text, Markdown, HTML, and JSON output
- **Historical Tracking**: Maintains history for year-over-year comparisons

## Usage

### Generate Weekly Report

```bash
# Generate report in text format (default)
go run tools/weekly-progress/main.go

# Generate HTML report
go run tools/weekly-progress/main.go -format html -output weekly-report.html

# Generate Markdown for wiki
go run tools/weekly-progress/main.go -format markdown -output weekly-report.md

# Compare with 2 weeks ago
go run tools/weekly-progress/main.go -weeks 2
```

### Automated Weekly Generation

Use the provided script for comprehensive report generation:

```bash
# Generate all formats
./scripts/generate-weekly-report.sh

# Generate and send notifications
./scripts/generate-weekly-report.sh --notify

# Generate and open in browser
./scripts/generate-weekly-report.sh --notify --open
```

### Schedule via Cron

Add to crontab for automatic weekly reports:

```bash
# Every Monday at 9 AM
0 9 * * 1 /path/to/quality/scripts/generate-weekly-report.sh --notify
```

## Report Sections

### 1. Key Metrics
- Error Handling Adoption (% and trend)
- Test Coverage (% and trend)
- Interface Compliance (% and trend)
- Directory Count (total and change)
- TODO Comments (count and change)

### 2. Achievements
Automatically recognizes:
- **Error Handling Champion**: >5% improvement
- **Testing Hero**: >3% coverage increase
- **Clean Code Advocate**: >5 directories reduced
- **Debt Eliminator**: >10 TODOs resolved
- **Interface Perfectionist**: Achieving 100% compliance

### 3. Team Progress
Shows for each team:
- Completion rate
- Tasks completed/in-progress/pending
- Key accomplishments

### 4. Action Items
Prioritized list with:
- Priority level (High/Medium/Low)
- Assigned team
- Specific targets
- Current vs desired state

## Output Examples

### Text Format
```
═══════════════════════════════════════════
       WEEKLY QUALITY PROGRESS REPORT
═══════════════════════════════════════════

Period: Jan 3, 2024 - Jan 10, 2024

📊 KEY METRICS
─────────────
Error Handling:       23.5% (+5.2%)
Test Coverage:        45.3% (+3.1%)
Interface Compliance: 100.0% (+5.0%)
Directory Count:      45 (-8)
TODO Comments:        35 (-12)

🏆 ACHIEVEMENTS THIS WEEK
────────────────────────
🎯 Error Handling Champion
   Improved error handling adoption by 5.2%
   Impact: Better error tracking and debugging
```

### HTML Format
Generates a beautifully formatted HTML report with:
- Visual progress bars
- Color-coded metrics
- Responsive design
- Print-friendly layout

### Markdown Format
Perfect for:
- GitHub wiki pages
- Confluence documentation
- Slack posts (with formatting)
- Email reports

## Configuration

### Environment Variables

```bash
# For notifications
export SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
export REPORT_EMAIL="team@example.com"

# For custom thresholds
export ACHIEVEMENT_ERROR_THRESHOLD=5
export ACHIEVEMENT_COVERAGE_THRESHOLD=3
```

### History File

The tool maintains a history file (`quality-history.json`) for trend tracking:

```json
{
  "snapshots": [
    {
      "timestamp": "2024-01-10T09:00:00Z",
      "error_handling": 23.5,
      "test_coverage": 45.3,
      "interface_compliance": 100.0,
      "directory_count": 45,
      "todo_count": 35
    }
  ]
}
```

## Integration

### With CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Generate Weekly Report
  if: github.event.schedule == '0 9 * * 1'  # Monday 9 AM
  run: |
    ./scripts/generate-weekly-report.sh

- name: Upload Weekly Report
  uses: actions/upload-artifact@v4
  with:
    name: weekly-report-${{ github.run_number }}
    path: reports/weekly/latest.*
```

### With Project Management

Extend `generateTeamProgress()` to pull from:
- Jira API
- GitHub Projects
- Azure DevOps
- Linear

Example:
```go
func generateTeamProgress() map[string]TeamMetrics {
    // Pull from Jira
    tasks := jiraClient.GetSprintTasks()
    return calculateTeamMetrics(tasks)
}
```

## Customization

### Adding New Metrics

1. Add field to `QualitySnapshot` struct
2. Update `loadCurrentMetrics()` to extract the metric
3. Add to trend calculation in `calculateTrends()`
4. Update report generators to display the metric

### Custom Achievements

Edit `generateAchievements()` to add new achievement types:

```go
// Speed improvement achievement
if buildTimeReduction > 30*time.Second {
    achievements = append(achievements, Achievement{
        Icon:        "⚡",
        Title:       "Speed Demon",
        Description: fmt.Sprintf("Reduced build time by %v", buildTimeReduction),
        Impact:      "Faster development cycles",
    })
}
```

### Custom Action Items

Modify `generateActionItems()` to add new rules:

```go
// Security-related action
if securityScore < 80 {
    items = append(items, ActionItem{
        Priority:    "High",
        Team:        "Security Team",
        Description: "Address security vulnerabilities",
        Target:      fmt.Sprintf("Improve score from %.1f to 80+", securityScore),
    })
}
```

## Best Practices

1. **Regular Reviews**: Schedule team meetings to review weekly reports
2. **Action Item Tracking**: Ensure action items are assigned and tracked
3. **Celebrate Success**: Share achievements with the broader team
4. **Trend Analysis**: Look at multi-week trends, not just week-to-week
5. **Continuous Improvement**: Update thresholds as team improves

## Troubleshooting

### "No previous metrics found"
- First run won't have comparison data
- History builds over time

### "Failed to load current metrics"
- Ensure quality-dashboard has been run first
- Check that quality-metrics.json exists

### Report looks empty
- Verify metrics files are in expected location
- Check file permissions
