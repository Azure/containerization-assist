# Quality Gates

This document describes the quality gates system used to maintain code quality standards in the project.

## Overview

The quality gates system validates code quality metrics against established baselines using a ratcheting approach - metrics can improve but should not regress without explicit approval.

## Script Implementation

The quality gates system has been migrated from a shell script to a **TypeScript script** (`scripts/quality-gates.ts`) for better:
- Cross-platform compatibility (no `jq`, `bc`, or shell dependencies)
- Type safety and maintainability
- Integration with the existing TypeScript tooling ecosystem
- Simplified testing and debugging

### Local Development (Reporting-Only Mode)

For local quality checks without modifying tracked files:

```bash
npm run quality:gates
```

This mode:
- Reports current quality metrics
- Compares against baselines
- **Does NOT** modify `quality-gates.json` or `knip-deadcode-output.txt`
- Safe to run repeatedly without git churn
- Runs via `tsx scripts/quality-gates.ts` (no external dependencies required)

### CI Pipeline (Baseline Update Mode)

In CI pipelines, enable baseline updates when quality improves:

```bash
UPDATE_BASELINES=true npm run quality:gates
```

This mode:
- Reports current quality metrics
- **Updates baselines** when metrics improve
- Writes `knip-deadcode-output.txt` for artifact storage
- Used in `.github/workflows/test-pipeline.yml`

## Quality Gates

### Gate 1: ESLint Errors (Zero Tolerance)

**Threshold:** 0 errors (hard requirement)

All ESLint errors must be fixed before merging. No regressions allowed.

### Gate 2: ESLint Warnings (Ratcheting)

**Current Baseline:** Tracked in `quality-gates.json`
**Target:** < 400 warnings

The baseline automatically improves when warning count decreases. New warnings that increase the count above baseline will fail the gate (unless `ALLOW_REGRESSION=true`).

### Gate 3: TypeScript Compilation

All code must compile without TypeScript errors. Skip with `SKIP_TYPECHECK=true` if needed.

### Gate 4: Dead Code Check (Ratcheting)

**Tool:** knip
**Current Baseline:** Tracked in `quality-gates.json`
**Target:** < 200 unused exports

Detects unused exports and prevents introducing new dead code. Baseline improves as unused code is removed.

### Gate 5: Build Performance

**Threshold:** < 60 seconds

Monitors build time to catch performance regressions. Baseline is tracked per environment (Node version, OS, CPU).

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `UPDATE_BASELINES` | `false` | Enable baseline updates (CI-only) |
| `ALLOW_REGRESSION` | `false` | Allow quality metric regressions |
| `SKIP_TYPECHECK` | `false` | Skip TypeScript compilation check |
| `VERBOSE` | `false` | Enable detailed output |

## Configuration

Quality thresholds and baselines are stored in `quality-gates.json`:

```json
{
  "metrics": {
    "thresholds": {
      "lint": { "maxErrors": 0, "maxWarnings": 400 },
      "deadcode": { "max": 200 },
      "typescript": { "maxErrors": 0 },
      "build": { "maxTimeMs": 60000 }
    },
    "baselines": {
      "lint": { "warnings": 750, "warningSignatures": [...] },
      "deadcode": { "count": 350 },
      "build": { "timeMs": 5000, "environment": {...} }
    }
  }
}
```

## Pre-Commit Hooks

As of the tooling simplification (Phase 1), quality gates are **NOT** run in pre-commit hooks to keep local commits fast. Instead:

1. **Pre-commit**: Only `lint-staged` runs (linting + formatting on staged files)
2. **CI/PR**: Full quality gates run in GitHub Actions

To run quality gates manually before pushing:

```bash
npm run validate        # Lint, typecheck, tests
npm run quality:gates   # Quality metrics check
```

## When to Run Manually

### Rarely Needed

Quality gates are primarily designed for CI automation. Developers rarely need to run them manually because:

- Pre-commit hooks handle linting/formatting
- CI provides comprehensive quality reports on PRs
- The `validate` script covers most quality checks

### Run Manually When:

1. **Before major refactoring** - Establish baseline before large changes
2. **Investigating CI failures** - Reproduce quality gate failures locally
3. **Baseline verification** - Confirm current quality metrics before proposing new thresholds

## CI Integration

Quality gates run in the `test-pipeline.yml` workflow:

1. Runs `npm run validate` (lint, typecheck, test) - single unified validation command
2. Runs `npm run quality:gates` with `UPDATE_BASELINES=true`
3. Extracts metrics and generates PR comment
4. Uploads quality reports as artifacts

The workflow is **non-blocking** by default but provides visibility into quality trends.

## Troubleshooting

### "Skipping baseline update" messages

This is expected in local mode. Baselines only update in CI when `UPDATE_BASELINES=true`.

### Warning count regression

If the gate fails due to increased warnings:
- Review the new warnings listed in the output
- Fix the warnings or document why regression is acceptable
- Use `ALLOW_REGRESSION=true` only for exceptional cases

### knip-deadcode-output.txt changes

This file is only created when `UPDATE_BASELINES=true` (CI mode). Local runs won't create or modify it.

## Future Improvements

- Add test coverage ratcheting
- Add bundle size tracking
- Add security vulnerability thresholds
- Integrate with PR review requirements