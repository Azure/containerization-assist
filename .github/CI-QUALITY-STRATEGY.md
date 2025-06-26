# CI/CD Quality Strategy

## Overview

This document describes our consistent approach to code quality enforcement across all CI/CD workflows.

## Core Principle: Ratcheting with Error Budgets

We use a **ratcheting strategy** that prevents quality regression while allowing gradual improvement:

1. **Prevent Regression**: Quality metrics must not get worse
2. **Focus on New Code**: PR checks primarily validate changed files
3. **Error Budgets**: Allow existing technical debt within limits
4. **Gradual Improvement**: Tighten thresholds over time

## Quality Metrics and Thresholds

### 1. Linting (Error Budget Approach)
- **MCP Package**: 100 errors, 50 warnings allowed
- **Core Package**: 50 errors, 30 warnings allowed
- **Combined**: 150 errors, 100 warnings allowed
- **New Code**: Must pass linting with `--new-from-rev`

### 2. Cyclomatic Complexity
- **Fail Threshold**: >20 (blocks PR)
- **Warning Threshold**: >15 (warns only)
- **Target**: <10 (long-term goal)
- **Current Baseline**: 263 functions >10, 84 functions >15

### 3. Code Formatting
- **Tools**: `gofmt -s`, `goimports`
- **Enforcement**: Warning only (doesn't block PRs)
- **Auto-fix**: Available via `make fmt`

### 4. Technical Debt (TODOs)
- **Limit per PR**: 10 new TODOs maximum
- **Warning Threshold**: >5 new TODOs
- **Enforcement**: Ratcheting (tracks total, prevents major increases)

### 5. Test Coverage
- **Target**: 50%
- **Minimum Budget**: 10%
- **Regression Tolerance**: 2% (allows small fluctuations)

### 6. Error Handling Adoption
- **Target**: 60%
- **Minimum Budget**: 30%
- **Regression Tolerance**: 5%

## Workflow Consistency

### Workflows Using Ratcheting ✅
1. **lint-with-budget.yml** - Error budgets for linting
2. **coverage-ratchet.yml** - Coverage regression prevention
3. **quality-gates.yml** - Multiple metrics with tolerances
4. **lint-strict.yml** - Checks only new/changed code

### Workflows with Fixed Thresholds
1. **unit-test.yml** - Uses error budgets via reusable workflow
2. **mcp-integration-tests.yml** - Higher thresholds for integration

## Implementation Guidelines

### For New Workflows
1. Read thresholds from `.github/quality-config.json`
2. Focus checks on changed files only
3. Use warnings instead of failures for non-critical issues
4. Implement tolerance for minor variations

### For Existing Code
1. Don't block PRs on legacy issues
2. Track metrics to show improvement over time
3. Gradually tighten thresholds as code improves

### For Developers
1. Run `make fmt` before committing
2. Check complexity with `make complexity-check`
3. Monitor lint issues with `make lint-report`
4. Use `make pre-commit` to run all checks locally

## Monitoring and Reporting

- **lint-dashboard.yml**: Tracks trends and creates issues
- **Complexity baseline**: Updated periodically
- **Quality gates**: Report metrics without blocking unnecessarily

## Future Improvements

1. Automate threshold adjustments based on trends
2. Create per-team or per-package thresholds
3. Add more granular complexity tracking
4. Implement automatic formatting on commit
