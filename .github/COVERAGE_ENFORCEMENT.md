# Test Coverage Enforcement

This document describes the test coverage enforcement system implemented as part of **Sprint D: Testing & Quality Foundation**.

## Overview

Our CI system enforces test coverage through multiple layers:

1. **Core Package Coverage Enforcement** - Ensures core packages meet minimum standards
2. **Coverage Ratchet** - Prevents coverage regressions
3. **Global Coverage Tracking** - Monitors overall project health

## Coverage Thresholds

### Current Enforcement (Sprint D)

| Package Group | Minimum | Target | Status |
|---------------|---------|--------|--------|
| Core Packages | 20% | 80% | ⚠️ Building up |
| MCP Packages | 20% | 80% | ✅ Improving |
| AI/Pipeline | 15% | 75% | 🔨 In progress |
| Global | 15% | 70% | 📈 Tracking |

### Staged Rollout Plan

**Phase 1: Foundation (Current - Sprint D)**
- Establish minimum 20% coverage for packages with tests
- Build comprehensive error handling and boundary condition tests
- Replace stub tests with real integration tests

**Phase 2: Growth (Next Sprint)**
- Raise minimum to 40% for core packages
- Enforce 60% for critical paths
- Full coverage ratchet enforcement

**Phase 3: Maturity (Future)**
- Achieve 80% target for core packages
- Enforce strict coverage requirements
- Maintain quality through automated checks

## Workflows

### 1. Core Coverage Enforcement (`.github/workflows/core-coverage-enforcement.yml`)

**Purpose**: Ensures core packages meet minimum coverage requirements

**Packages Monitored**:
- `./pkg/mcp/internal/core/...`
- `./pkg/mcp/internal/runtime/...`
- `./pkg/mcp/internal/orchestration/...`
- `./pkg/mcp/internal/session/...`
- `./pkg/mcp/internal/build/...`
- `./pkg/mcp/internal/deploy/...`
- `./pkg/mcp/internal/analyze/...`
- `./pkg/mcp/internal/registry/...`
- `./pkg/core/docker/...`
- `./pkg/core/kubernetes/...`
- `./pkg/core/git/...`
- `./pkg/core/analysis/...`
- `./pkg/pipeline/...`
- `./pkg/ai/...`
- `./pkg/clients/...`

**Current Behavior**:
- ❌ **Fails CI** if any package is below 20% minimum
- ⚠️ **Warns** if any package is below 80% target
- 📊 **Reports** detailed coverage for each package
- 💬 **Comments** on PRs with coverage status

### 2. Coverage Ratchet (`.github/workflows/coverage-ratchet.yml`)

**Purpose**: Prevents coverage regressions and tracks overall project health

**Features**:
- Compares coverage against base branch
- Allows up to 2% regression tolerance
- Enforces global minimum thresholds
- Tracks progress toward targets

**Thresholds** (from `.github/coverage-thresholds.json`):
- Global minimum: 60%
- Global target: 80%
- Regression tolerance: 2%

## Configuration Files

### `.github/coverage-thresholds.json`

Contains all coverage configuration:

```json
{
  "global": {
    "line_coverage": {
      "minimum": 60,
      "target": 80
    }
  },
  "packages": {
    "core": { "minimum": 70, "target": 85 },
    "mcp": { "minimum": 60, "target": 80 },
    "ai_pipeline": { "minimum": 50, "target": 75 }
  },
  "ratchet": {
    "regression_tolerance": 2.0
  },
  "enforcement": {
    "strict_mode": false,
    "warning_only": true
  }
}
```

## Sprint D Achievements

As part of Sprint D: Testing & Quality Foundation, we have:

### ✅ Completed
1. **Documentation Plan**: Comprehensive testing strategy documented
2. **Coverage Baseline**: Analyzed current coverage across all packages
3. **Orphaned Tests**: Identified and cataloged 28+ orphaned test files
4. **Table-Driven Tests**: Implemented for high-priority public functions:
   - `pkg/mcp/internal/build/build_image_atomic_validate_test.go`
   - `pkg/mcp/internal/registry/multi_registry_manager_test.go`
5. **Integration Tests**: Replaced stub with real MCP server integration tests
6. **Error Handling Tests**: Added comprehensive error handling and boundary condition tests:
   - `pkg/mcp/internal/session/session_error_handling_test.go`
   - `pkg/mcp/internal/analyze/analyze_error_handling_test.go`
7. **CI Enforcement**: Set up coverage enforcement with staged rollout

### 📈 Coverage Improvements

| Package | Before | After | Improvement |
|---------|--------|--------|-------------|
| build | 7.8% | 8.3% | +0.5% |
| registry | 0% | 14.1% | +14.1% |
| session | ~10% | 17.3% | +7.3% |
| analyze | 0% | 11.2% | +11.2% |

## Usage for Developers

### Running Coverage Locally

```bash
# Check coverage for specific package
go test -cover ./pkg/mcp/internal/session/...

# Generate detailed coverage report
go test -coverprofile=coverage.out ./pkg/mcp/internal/session/...
go tool cover -html=coverage.out -o coverage.html

# Check all core packages
go test -cover ./pkg/mcp/internal/build/... ./pkg/mcp/internal/registry/... ./pkg/mcp/internal/session/... ./pkg/mcp/internal/analyze/...
```

### Understanding CI Failures

**"Critical: Packages below 20% minimum threshold"**
- Your changes caused a package to drop below the minimum required coverage
- Add tests to bring the package above 20%
- Focus on testing new code you added

**"Coverage regression detected"**
- Your changes decreased overall coverage by more than 2%
- Add tests for new functionality
- Ensure removed code had corresponding test cleanup

### Best Practices

1. **Add tests for new code**: Aim for >80% coverage on new functionality
2. **Test error paths**: Include error handling and boundary condition tests
3. **Use table-driven tests**: For functions with multiple input scenarios
4. **Test integration points**: Verify component interactions
5. **Mock external dependencies**: Focus tests on your code, not external services

## Monitoring and Metrics

Coverage metrics are tracked through:
- **GitHub Actions**: Automated reports on every PR
- **Artifacts**: Detailed HTML coverage reports
- **PR Comments**: Coverage summaries and comparisons
- **Step Summaries**: Quick overview in workflow results

## Future Enhancements

1. **Package-specific enforcement**: Different thresholds per package type
2. **Critical path coverage**: 100% coverage for security-sensitive code
3. **Mutation testing**: Verify test quality, not just coverage
4. **Performance regression prevention**: Combine with benchmark testing
5. **Dependency coverage**: Track coverage of external dependencies

## Support

For questions about coverage enforcement:
1. Check this documentation
2. Review workflow logs in GitHub Actions
3. Examine coverage artifacts for detailed reports
4. Refer to Sprint D implementation in git history

---

*Generated as part of Sprint D: Testing & Quality Foundation*
