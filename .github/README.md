# GitHub Workflows Architecture

This document describes the GitHub Actions workflow architecture for the Container Kit project.

## Overview

Container Kit uses a streamlined CI/CD pipeline with **2 primary workflows**:

- **`ci-pipeline.yml`** - Main CI pipeline for all pull requests and merges
- **`release.yml`** - Handles production releases on version tags

## Core Workflows

### 1. CI Pipeline (`ci-pipeline.yml`)

The main CI workflow that runs on every pull request and push to main.

**Key Features**:
- Fast feedback with parallel execution
- Comprehensive test coverage (unit + integration)
- Race detection enabled
- Automatic retries for flaky tests
- Efficient caching for dependencies

**Workflow Jobs** (Optimized for Speed & Efficiency):
1. **Setup** - Path detection and cache key generation
2. **Build** (Parallel) - Compile MCP server and CLI binaries
3. **Quality** (Parallel) - Comprehensive code quality analysis
4. **Test** - Unit tests with race detection and coverage
5. **MCP Integration** - Core MCP workflow validation
6. **CLI Integration** - Multi-repository integration testing (conditional)
7. **PR Comment** - Beautiful status summary on pull requests
8. **CI Status** - Final aggregated status for branch protection

### 2. Release Workflow (`release.yml`)

Handles production releases when version tags are pushed.

**Triggers**: Push of tags matching `v*`

**Steps**:
1. Build production binaries
2. Run comprehensive tests
3. Create GitHub release with artifacts
4. Publish release notes

## Custom Actions

Container Kit maintains **3 custom actions** in `.github/actions/`:

### 1. `quality-checks/`
- **Parallel quality analysis** with tool caching
- Includes: formatting, linting, static analysis, security scanning, architecture validation
- **Error budget system**: Tracks quality debt while allowing CI to pass within defined limits
- **Performance**: 40-50% faster through parallel execution and caching
- Provides detailed quality metrics and scoring (0-100)

### 2. `integration-test-runner/`
- **Multi-repository integration testing**
- Tests complete containerization workflows
- Validates Docker operations, Kubernetes deployments, and MCP protocol
- **Optimized matrix**: Reduced from 30 to 9 parallel jobs (70% reduction)

### 3. `pr-comment/`
- **Beautiful CI/CD status comments** on pull requests
- Auto-updates existing comments (no spam)
- Includes quality scores, test results, and detailed logs
- **Smart formatting** with expandable sections and direct links

## Workflow Triggers

| Workflow | Triggers | When to Run |
|----------|----------|-------------|
| CI Pipeline | `pull_request`, `push` to main | Every PR and main branch push |
| Release | `push` tags `v*` | On version tags |

## Performance

The optimized CI architecture provides:
- **Fast feedback**: Most CI runs complete in under 3-4 minutes (40-50% improvement)
- **Efficient resource usage**: Parallel job execution with Go module & tool caching
- **Smart execution**: Early termination on quality failures, conditional CLI testing
- **Optimized matrix**: Reduced integration tests from 30 to 9 parallel jobs
- **Beautiful reporting**: Automated PR comments with comprehensive status summaries

## Quality Management

### Error Budget System

Container Kit uses an **error budget approach** to manage code quality:

- **Current baseline**: 183 lint issues (within 200-issue budget)
- **Budget tracking**: Configured in `.github/quality-config.json`
- **CI behavior**: Passes when within budget, fails when exceeded
- **Quality visibility**: All issues tracked and reported in CI output

**Error Budget Configuration**:
```json
"linting": {
  "error_budgets": {
    "combined": {
      "errors": 200,    // Total lint issues allowed
      "warnings": 120   // Warning threshold
    }
  }
}
```

**Benefits**:
- ✅ **CI stability**: Prevents blocking development on style issues
- ✅ **Quality tracking**: All 183 issues visible and monitored
- ✅ **Regression prevention**: Fails if issues exceed budget
- ✅ **Gradual improvement**: Budget can be lowered over time

### Tool Versions (Pinned for Consistency)

- **Go**: 1.24.4 (fixes 4 security vulnerabilities)
- **golangci-lint**: v2.2.2 (matches config file version)
- **staticcheck**: latest compatible
- **govulncheck**: v1.1.3

## Best Practices

### For Contributors

1. **Before pushing**:
   - Run `make fmt` to format code
   - Run `make lint` to check for issues
   - Run `make test` for unit tests
   - **Note**: CI will pass with lint issues if within budget

2. **Writing tests**:
   - Ensure tests are isolated and can run in parallel
   - Use proper cleanup in test teardown
   - Avoid hardcoded paths or external dependencies

3. **Quality improvements**:
   - Fix critical issues (security, compilation errors) immediately
   - Address style issues gradually to stay within budget
   - Prioritize errcheck and staticcheck issues over revive style issues

### For Maintainers

1. **Workflow updates**:
   - Test changes in a branch first
   - Keep workflows simple and focused
   - Document any new requirements

2. **Performance monitoring**:
   - Review workflow run times regularly
   - Optimize slow steps
   - Update caching strategies as needed

## Troubleshooting

### Common Issues

1. **CI failures**:
   - Check the workflow logs for specific errors
   - Ensure all dependencies are properly declared
   - Verify tests run successfully locally

2. **Integration test failures**:
   - Review the test output in the CI logs
   - Check for timing-related issues
   - Ensure Docker/Kubernetes resources are available

### Debug Mode

Enable debug logging by setting repository secrets:
- `ACTIONS_RUNNER_DEBUG=true`
- `ACTIONS_STEP_DEBUG=true`
