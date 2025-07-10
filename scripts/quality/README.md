# Container Kit Quality Gates

This directory contains the quality gates and quality assurance infrastructure for Container Kit.

## Overview

The quality gates system provides comprehensive validation of code quality, ensuring that the codebase maintains high standards throughout development and refactoring.

## Quality Gates

### 1. Code Formatting
- **Check**: All Go files are properly formatted using `gofmt`
- **Fix**: Run `make fmt` or `gofmt -w pkg/`
- **Enforcement**: Blocking in CI/CD

### 2. Linting
- **Check**: Code passes static analysis with error budget (100 issues)
- **Tools**: `go vet`, pattern matching for common issues
- **Fix**: Address specific linting issues reported
- **Enforcement**: Error budget system

### 3. Build Verification
- **Check**: All packages build successfully
- **Timeout**: 5 minutes maximum
- **Fix**: Resolve compilation errors
- **Enforcement**: Blocking in CI/CD

### 4. Test Coverage
- **Check**: Test coverage meets minimum threshold (15%)
- **Target**: 55% overall, 80% for new code
- **Tools**: `go test -cover`
- **Enforcement**: Threshold-based

### 5. Performance Benchmarks
- **Check**: Benchmarks complete within performance targets (<300μs)
- **Tools**: `go test -bench`
- **Fix**: Optimize slow operations
- **Enforcement**: Performance regression detection

### 6. Architecture Validation
- **Check**: Clean architecture boundaries maintained
- **Rules**: Domain layer has no external dependencies
- **Fix**: Remove violating imports, refactor dependencies
- **Enforcement**: Architectural integrity

### 7. Security Checks
- **Check**: No hardcoded secrets, minimal dangerous function usage
- **Patterns**: Password/token/key patterns, unsafe operations
- **Fix**: Use secure configuration management
- **Enforcement**: Security baseline

## Scripts

### Core Quality Scripts

#### `quality_gates.sh`
Comprehensive quality gates validation with all checks.

```bash
# Run all quality gates
scripts/quality/quality_gates.sh

# With custom thresholds
COVERAGE_THRESHOLD=20 LINT_ERROR_BUDGET=50 scripts/quality/quality_gates.sh
```

#### `coverage_tracker.sh`
Advanced test coverage analysis and reporting.

```bash
# Generate coverage report
scripts/quality/coverage_tracker.sh

# View HTML report
open test/reports/coverage.html
```

#### `pre_commit_hook.sh`
Local pre-commit validation for developers.

```bash
# Run pre-commit checks
scripts/quality/pre_commit_hook.sh

# Quick mode (faster)
QUICK_MODE=true scripts/quality/pre_commit_hook.sh

# Skip tests
SKIP_TESTS=true scripts/quality/pre_commit_hook.sh
```

#### `run_test_suite.sh`
Comprehensive test runner with multiple test types.

```bash
# Run all tests
scripts/quality/run_test_suite.sh

# Unit tests only
scripts/quality/run_test_suite.sh --unit-only

# Include integration tests
scripts/quality/run_test_suite.sh --integration

# Include benchmarks
scripts/quality/run_test_suite.sh --benchmarks

# Custom coverage threshold
scripts/quality/run_test_suite.sh --threshold 20
```

#### `generate_tests.sh`
Automated test generation for packages.

```bash
# Generate unit tests
scripts/quality/generate_tests.sh pkg/mcp/domain/errors unit

# Generate integration tests
scripts/quality/generate_tests.sh pkg/mcp/application/core integration

# Generate benchmark tests
scripts/quality/generate_tests.sh pkg/mcp/domain/security benchmark
```

#### `generate_metrics_dashboard.sh`
Quality metrics dashboard and reporting.

```bash
# Generate metrics dashboard
scripts/quality/generate_metrics_dashboard.sh

# View dashboard
open docs/quality/dashboard.md
```

### Make Targets

```bash
# Individual quality checks
make fmt-check          # Check code formatting
make lint              # Run linting
make coverage          # Generate coverage report
make quality-gates     # Run all quality gates
make quality-dashboard # Generate metrics dashboard

# Combined targets
make quality-all       # Run all quality checks
make quality-gates-ci  # CI-optimized quality gates
make pre-commit-hook   # Local pre-commit validation
```

## Configuration

### Environment Variables

```bash
# Coverage threshold (default: 15.0)
export COVERAGE_THRESHOLD=20.0

# Lint error budget (default: 100)
export LINT_ERROR_BUDGET=150

# Performance threshold in nanoseconds (default: 300000)
export PERFORMANCE_THRESHOLD_NS=500000

# Maximum build time in seconds (default: 300)
export MAX_BUILD_TIME=600

# Test timeout (default: 10m)
export TEST_TIMEOUT=15m

# Skip tests in pre-commit (default: false)
export SKIP_TESTS=true

# Quick mode for faster checks (default: false)
export QUICK_MODE=true
```

### Quality Thresholds

| Metric | Current Target | Long-term Target | Enforcement |
|--------|----------------|------------------|-------------|
| Test Coverage | 15% | 55% | Threshold |
| Package Test Coverage | 37% | 80% | Monitoring |
| Lint Issues | 100 | 0 | Error Budget |
| Performance (P95) | 300μs | 300μs | Regression |
| Large Files | Monitoring | 0 | Warning |
| Security Issues | Monitoring | 0 | Review |

## CI/CD Integration

### GitHub Actions

The quality gates are integrated into GitHub Actions workflows:

- **`.github/workflows/quality-gates.yml`**: Main quality gates workflow
- **Triggers**: Push to main/develop, Pull requests
- **Artifacts**: Coverage reports, quality dashboard, test results
- **Status Checks**: All gates must pass for PR approval

### Workflow Steps

1. **Setup**: Go environment, dependencies, make alias
2. **Code Formatting**: `gofmt` compliance check
3. **Build Verification**: Compilation success
4. **Linting**: Static analysis with error budget
5. **Test Execution**: Unit and integration tests
6. **Coverage Analysis**: Coverage threshold validation
7. **Performance Benchmarks**: Regression detection
8. **Architecture Validation**: Boundary enforcement
9. **Comprehensive Gates**: Full validation suite
10. **Artifact Upload**: Reports and dashboards

## Local Development

### Setup Pre-commit Hook

```bash
# Install pre-commit hook
cp scripts/quality/pre_commit_hook.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

# Or use existing install script if available
make install-hooks
```

### Daily Workflow

```bash
# Before starting work
make quality-dashboard

# During development
make fmt                    # Format code
make lint                   # Check for issues
scripts/quality/generate_tests.sh pkg/mcp/mypackage unit

# Before committing
make pre-commit-hook        # Local validation
git add .
git commit -m "feature: add new functionality"

# Weekly quality review
make quality-all
open docs/quality/dashboard.md
```

## Troubleshooting

### Common Issues

#### 1. Coverage Threshold Not Met
```bash
# Check current coverage
make coverage

# Identify packages needing tests
grep "0.0% of statements" test/reports/coverage_summary.txt

# Generate tests for low-coverage packages
scripts/quality/generate_tests.sh pkg/mcp/domain/errors unit
```

#### 2. Lint Issues Exceed Budget
```bash
# Check specific issues
make lint

# Common fixes
gofmt -w pkg/                    # Format code
goimports -w pkg/               # Organize imports
go vet ./pkg/mcp/...            # Address vet issues
```

#### 3. Build Failures
```bash
# Check build issues
go build ./pkg/mcp/...

# Common causes
go mod tidy                     # Fix module issues
go get -u ./...                # Update dependencies
```

#### 4. Performance Regressions
```bash
# Run benchmarks
go test -bench=. ./pkg/mcp/...

# Compare with baseline
scripts/performance/track_benchmarks.sh

# Identify slow functions
go test -bench=. -benchmem ./pkg/mcp/... | grep "ns/op"
```

### Bypass Options

```bash
# Skip specific checks (development only)
SKIP_TESTS=true scripts/quality/pre_commit_hook.sh
QUICK_MODE=true scripts/quality/pre_commit_hook.sh

# Git bypass (emergency only)
git commit --no-verify

# CI bypass (not recommended)
git commit -m "fix: urgent fix [skip ci]"
```

## Quality Metrics

### Current State
- **237 unit tests** across 15 packages with tests
- **37% package coverage** (15/40 packages have tests)
- **22 packages** using testify framework
- **12 benchmark functions** for performance monitoring

### Improvement Plan
1. **Week 1-2**: Fix build issues, increase coverage to 25%
2. **Week 3-4**: Add tests to 60% of packages
3. **Month 2**: Achieve 55% overall coverage
4. **Month 3**: 80% coverage for new code

## Resources

- [Test Coverage Improvement Plan](../../test/TEST_COVERAGE_IMPROVEMENT_PLAN.md)
- [Quality Dashboard](../../docs/quality/dashboard.md)
- [Architecture Documentation](../../docs/architecture/README.md)
- [Performance Baseline](../../benchmarks/PERFORMANCE_BASELINE.md)

## Support

For questions or improvements to the quality gates system:

1. Check the [Quality Dashboard](../../docs/quality/dashboard.md)
2. Review [troubleshooting](#troubleshooting) section
3. Run diagnostics: `make quality-dashboard`
4. Contact the quality team or create an issue

---

*Quality Gates Infrastructure - Ensuring code excellence throughout development*
