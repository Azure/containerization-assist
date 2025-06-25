# Quality Dashboard

A comprehensive quality metrics tool for monitoring and improving code quality across the MCP codebase.

## Features

### Error Handling Metrics
- Tracks adoption of RichError vs standard error handling
- Identifies files with highest standard error usage
- Provides package-level breakdown
- Shows top files to migrate

### Directory Structure Metrics
- Monitors directory depth and complexity
- Identifies empty directories
- Tracks package organization
- Detects structural violations

### Test Coverage Metrics
- Overall coverage percentage
- Package-level coverage breakdown
- Identifies uncovered packages
- Shows top and bottom coverage packages

### Build Metrics
- Build time tracking
- Test execution time
- Binary size monitoring
- Dependency count
- Historical build performance (with --history flag)

### Code Quality Metrics
- Cyclomatic complexity analysis
- Long function detection (>50 lines)
- TODO/FIXME comment tracking
- Complexity hotspot identification

## Usage

### Basic Usage

```bash
# Generate metrics for current directory
go run tools/quality-dashboard/main.go

# Generate metrics for specific directory
go run tools/quality-dashboard/main.go -root ./pkg/mcp

# Output to specific file
go run tools/quality-dashboard/main.go -output metrics.json

# Different output formats
go run tools/quality-dashboard/main.go -format text    # Human-readable text
go run tools/quality-dashboard/main.go -format html    # HTML dashboard
go run tools/quality-dashboard/main.go -format json    # JSON (default)
```

### Watch Mode

Continuously monitor metrics:

```bash
# Update every 5 minutes (default)
go run tools/quality-dashboard/main.go -watch

# Custom interval
go run tools/quality-dashboard/main.go -watch -interval 10m
```

### Historical Tracking

Track build metrics over time:

```bash
go run tools/quality-dashboard/main.go -history build-history.json
```

## Output Examples

### JSON Output
```json
{
  "timestamp": "2024-01-10T10:30:00Z",
  "error_handling": {
    "total_errors": 150,
    "rich_errors": 50,
    "standard_errors": 100,
    "adoption_rate": 33.3,
    "top_files_to_migrate": [
      {
        "path": "pkg/mcp/internal/workflow/executor.go",
        "standard_errors": 25,
        "adoption_rate": 0
      }
    ]
  },
  "test_coverage": {
    "overall_coverage": 72.5,
    "uncovered_packages": ["pkg/mcp/tools/new-tool"],
    "bottom_coverage": [
      {"package": "pkg/mcp/internal/config", "coverage": 15.2}
    ]
  },
  "recommendations": [
    "🔴 Error Handling: Only 33.3% adoption of RichError. Target: 80%",
    "✅ Test Coverage: 72.5% (above target of 70%)"
  ]
}
```

### Text Output
```
Quality Dashboard Report
Generated: 2024-01-10T10:30:00Z

ERROR HANDLING METRICS
=====================
Total Errors: 150
Rich Errors: 50 (33.3%)
Standard Errors: 100

TEST COVERAGE
=============
Overall Coverage: 72.5%
Uncovered Packages: 3

RECOMMENDATIONS
===============
🔴 Error Handling: Only 33.3% adoption of RichError. Target: 80%
   Start with: pkg/mcp/internal/workflow/executor.go (25 standard errors)
✅ Test Coverage: 72.5% (above target of 70%)
```

### HTML Output
Generates an interactive HTML dashboard with color-coded metrics and visualizations.

## Integration with CI/CD

### GitHub Actions Example

```yaml
- name: Generate Quality Metrics
  run: |
    go run tools/quality-dashboard/main.go \
      -output quality-metrics.json \
      -format json

- name: Upload Quality Report
  uses: actions/upload-artifact@v4
  with:
    name: quality-metrics
    path: quality-metrics.json

- name: Check Quality Gates
  run: |
    ERROR_RATE=$(jq '.error_handling.adoption_rate' quality-metrics.json)
    if (( $(echo "$ERROR_RATE < 80" | bc -l) )); then
      echo "❌ Error handling adoption below 80%"
      exit 1
    fi
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Run quality check
go run tools/quality-dashboard/main.go -format text -output -

# Check for critical issues
if [ $? -ne 0 ]; then
  echo "Quality check failed. Please address the issues before committing."
  exit 1
fi
```

## Recommendations and Thresholds

The dashboard provides actionable recommendations based on these thresholds:

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| Error Handling Adoption | 80% | 60% | < 60% |
| Test Coverage | 70% | 50% | < 50% |
| Directory Depth | ≤ 5 | 6 | > 6 |
| Function Length | ≤ 50 lines | 75 lines | > 100 lines |
| Cyclomatic Complexity | ≤ 10 | 15 | > 20 |
| Build Time | < 30s | 60s | > 120s |

## Extending the Dashboard

To add new metrics:

1. Add new struct to represent the metric
2. Implement collection function `collectXxxMetrics()`
3. Add to main `collectMetrics()` function
4. Update recommendation generation
5. Update output formatters

Example:
```go
type SecurityMetrics struct {
    VulnerablePackages int `json:"vulnerable_packages"`
    OutdatedDeps      int `json:"outdated_dependencies"`
}

func collectSecurityMetrics(rootDir string) (*SecurityMetrics, error) {
    // Implementation
}
```

## Performance Considerations

- Coverage collection requires running tests (can be slow)
- Build metrics require compilation
- Use `-watch` mode with appropriate intervals for continuous monitoring
- Consider running different metric collections at different frequencies

## Troubleshooting

### "go test failed" warnings
- Normal if some tests are failing
- Coverage metrics will use partial data

### Empty coverage metrics
- Ensure tests exist for the packages
- Check that go test can run successfully

### High memory usage in watch mode
- Increase the interval between collections
- Consider running specific metrics only