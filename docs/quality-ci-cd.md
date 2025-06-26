# Quality CI/CD Integration

This document describes the quality gates and CI/CD enhancements for the MCP project.

## Overview

The quality CI/CD pipeline enforces code quality standards through:
- Automated quality gates in GitHub Actions
- Pre-commit and pre-push hooks for local development
- Configurable quality thresholds
- Comprehensive reporting

## Components

### 1. GitHub Actions Workflow (`quality-gates.yml`)

Runs on every PR and push to main:

- **Interface Validation**: Ensures all tools implement required interfaces
- **Quality Metrics**: Generates comprehensive quality dashboard
- **Regression Detection**: Compares metrics with base branch
- **PR Comments**: Posts quality report as PR comment
- **Status Checks**: Sets commit status based on quality gates

### 2. Pre-commit Hooks

Quick checks before each commit:

```bash
# Install hooks
make install-hooks
# or
bash scripts/install-hooks.sh
```

Checks performed:
- Interface validation (blocking)
- Code formatting with gofmt (blocking)
- Go vet static analysis (blocking)
- Error handling adoption (warning)
- Test coverage (warning)

### 3. Pre-push Hooks

Comprehensive checks before pushing:

- Run tests with timeout
- Build verification
- Strict quality gates
- Interface validation

### 4. Makefile Integration

```bash
# Run all quality checks
make quality

# Check specific areas
make quality-interfaces
make quality-dashboard
make quality-gates

# Generate reports
make quality-report

# Continuous monitoring
make quality-watch

# Find improvement areas
make improve-errors
make improve-coverage
make improve-interfaces
```

## Configuration

### Quality Thresholds (`.quality-thresholds.json`)

Customize thresholds for your project:

```json
{
  "quality_gates": {
    "error_handling": {
      "minimum": 20.0,    // Fails below this
      "warning": 60.0,    // Warning threshold
      "target": 80.0      // Target goal
    },
    "test_coverage": {
      "minimum": 10.0,
      "warning": 50.0,
      "target": 70.0
    }
  }
}
```

### Bypass Mechanisms

For emergency situations only:

```bash
# Bypass pre-commit hooks
git commit --no-verify

# Bypass pre-push hooks
git push --no-verify

# Skip quality gates in CI (requires admin)
# Add [skip-quality] to commit message
```

## Quality Gates

### Required Gates (Blocking)

1. **Interface Compliance**: 100% - All tools must implement required interfaces
2. **Error Handling**: ≥20% - Minimum RichError adoption
3. **Test Coverage**: ≥10% - Minimum test coverage
4. **Build Success**: Must compile without errors

### Warning Gates (Non-blocking)

1. **Error Handling Target**: 60% - Ideal RichError adoption
2. **Test Coverage Target**: 50% - Ideal test coverage
3. **Empty Directories**: ≤5 - Clean directory structure
4. **TODO Comments**: ≤50 - Address technical debt

### Regression Prevention

PRs fail if metrics regress beyond tolerance:
- Error Handling: -5% maximum regression
- Test Coverage: -2% maximum regression

## Integration Examples

### GitHub Branch Protection

Add these status checks as required:
1. `quality-gates`
2. `quality-checks`

### CI Pipeline Integration

```yaml
# .github/workflows/main.yml
jobs:
  quality:
    uses: ./.github/workflows/quality-gates.yml
    
  build:
    needs: quality
    # ... rest of build
```

### Local Development Workflow

```bash
# Before starting work
make quality

# After making changes
make quality-gates

# Before committing
git add .
git commit  # Pre-commit hooks run automatically

# Before pushing
git push   # Pre-push hooks run automatically
```

## Monitoring and Reporting

### Dashboard Access

1. **Local**: Open `quality-dashboard.html` in browser
2. **CI**: Download artifacts from GitHub Actions
3. **Watch Mode**: `make quality-watch` for live updates

### Metrics Tracked

- Error handling adoption rate
- Test coverage by package
- Build and test times
- Directory structure complexity
- Code quality metrics
- Interface compliance

### Report Formats

- **JSON**: Machine-readable metrics
- **HTML**: Interactive dashboard
- **Text**: CLI-friendly summary

## Troubleshooting

### "Interface validation failed"

```bash
# See detailed errors
go run tools/validate-interfaces/main.go --verbose

# Check specific tool
go run tools/validate-interfaces/main.go --tool MyTool
```

### "Quality gates failed"

```bash
# Check current metrics
make quality-dashboard

# See specific failures
cat quality-metrics.json | jq '.recommendations'
```

### Hook Issues

```bash
# Reinstall hooks
make install-hooks

# Check hook logs
cat .git/hooks/pre-commit.log

# Temporary bypass
git commit --no-verify -m "Emergency fix"
```

## Best Practices

1. **Run quality checks frequently** during development
2. **Address warnings** before they become blockers
3. **Don't bypass hooks** unless absolutely necessary
4. **Monitor trends** not just absolute values
5. **Celebrate improvements** in quality metrics

## Future Enhancements

Planned improvements:
- Security vulnerability scanning
- Performance regression detection
- Automated fix suggestions
- IDE integration
- Slack/Teams notifications