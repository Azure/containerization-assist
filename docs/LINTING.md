# Linting Strategy and Error Budgets

This document describes our approach to managing code quality through linting with error budgets.

## Overview

We use [golangci-lint](https://golangci-lint.run/) with an error budget approach that allows controlled technical debt while preventing regression. This enables us to:

1. **Ship code** without being blocked by existing lint issues
2. **Prevent regression** by ensuring issues don't increase
3. **Track progress** toward code quality goals
4. **Focus efforts** on the most impactful improvements

## Error Budget Thresholds

Current thresholds are defined in `.github/lint-thresholds.json`:

| Package Group | Error Threshold | Warning Threshold | Description |
|---------------|-----------------|-------------------|-------------|
| MCP | 100 | 50 | MCP packages (higher due to current debt) |
| Core | 50 | 30 | Core packages should maintain higher quality |
| CLI | 50 | 30 | CLI/AI packages |
| Combined | 150 | 100 | MCP + Core packages together |
| All | 200 | 150 | Entire codebase |

## CI Integration

### Pull Requests
- Each package group is linted with its specific threshold
- Builds fail if issues exceed the error threshold
- Warnings are shown when approaching limits

### Main Branch
- Daily lint dashboard generates comprehensive reports
- Automatic issues created when thresholds exceeded
- Baseline tracking for historical trends

## Local Development

### Available Commands

```bash
# Standard linting (strict mode - fails on any issue)
make lint

# Linting with error budget (allows up to threshold)
make lint-threshold

# Generate detailed lint report
make lint-report

# Set current issue count as baseline
make lint-baseline

# Ratchet mode - ensure no regression from baseline
make lint-ratchet
```

### Quick Checks

```bash
# Check current issue count for MCP
golangci-lint run ./pkg/mcp/... 2>&1 | grep -E "^[^:]+:[0-9]+:[0-9]+:" | wc -l

# See issues by linter
golangci-lint run ./pkg/mcp/... 2>&1 | grep -oE '\([a-z]+\)$' | sort | uniq -c

# Get detailed JSON report
golangci-lint run --out-format json ./pkg/mcp/... > lint-report.json
jq '.Issues | group_by(.FromLinter) | map({linter: .[0].FromLinter, count: length})' lint-report.json
```

## Gradual Improvement Strategy

### Phase 1: Stabilization (Current)
- Set thresholds above current issue counts
- Prevent regression with ratchet checks
- Focus on not introducing new issues

### Phase 2: Reduction
- Quarterly targets in `.github/lint-thresholds.json`
- Focus on high-impact improvements:
  - Security issues (gosec)
  - Error handling (errcheck, nilerr)
  - Race conditions (govet)

### Phase 3: Maintenance
- Strict linting for new code
- Gradual refactoring of legacy code
- Automated fixes where possible

## Common Issues and Fixes

### High Priority
1. **errcheck**: Unchecked error returns
   ```go
   // Bad
   result, _ := someFunc()

   // Good
   result, err := someFunc()
   if err != nil {
       return fmt.Errorf("failed to do something: %w", err)
   }
   ```

2. **gosec**: Security issues
   - Use `filepath.Clean()` for path operations
   - Avoid `math/rand` for security-sensitive operations
   - Set proper file permissions (0600 for sensitive files)

3. **govet**: Shadow declarations
   ```go
   // Bad
   err := doSomething()
   if err != nil {
       err := doSomethingElse() // shadows outer err
   }

   // Good
   err := doSomething()
   if err != nil {
       if err2 := doSomethingElse(); err2 != nil {
           // handle err2
       }
   }
   ```

### Medium Priority
1. **goconst**: Repeated strings
   - Extract to package constants
   - Use types.Constants for shared values

2. **ineffassign**: Ineffectual assignments
   - Remove unused assignments
   - Check for logic errors

3. **staticcheck**: Various style/correctness issues
   - Update deprecated functions
   - Fix incorrect comparisons

### Low Priority
1. **revive**: Missing comments on exported items
   - Add godoc comments to exported types/functions
   - Use standard comment format

2. **funlen**: Functions too long
   - Extract helper functions
   - Consider refactoring complex logic

## Workflow Integration

### For Contributors
1. Run `make lint-threshold` before pushing
2. Fix any **new** issues your changes introduce
3. Optionally fix existing issues in touched files

### For Reviewers
1. Check CI lint results in PR
2. Ensure no regression in issue count
3. Encourage fixing issues in modified code

### For Maintainers
1. Monitor weekly lint dashboards
2. Plan debt reduction sprints
3. Update thresholds quarterly

## Automation

### GitHub Actions
- `.github/workflows/lint-with-budget.yml`: Main threshold checking
- `.github/workflows/lint-dashboard.yml`: Daily reporting
- `.github/workflows/unit-test.yml`: Per-package linting

### Scripts
- `scripts/lint-with-threshold.sh`: Threshold checking
- `scripts/lint-ratchet.sh`: Ratchet enforcement

## FAQ

**Q: Why not fix all issues immediately?**
A: With 300+ existing issues, fixing everything would block feature development. The error budget allows gradual improvement.

**Q: Can I increase thresholds?**
A: Only with team consensus and documented reason. Prefer using the ratchet approach.

**Q: What if I need to ignore a specific issue?**
A: Use `//nolint:lintername` with explanation, or add to `.golangci.yml` exclusions for systemic false positives.

**Q: How do we track progress?**
A: Daily dashboards, quarterly reviews, and the `.lint-baseline.json` file track our improvement over time.
