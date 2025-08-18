# GitHub Actions Workflows

This directory contains the CI/CD workflows for Containerization Assist.

## Active Workflows

Containerization Assist uses **2 workflows** for all CI/CD operations:

### 1. `ci-pipeline.yml` - Main CI Pipeline
**Runs on**: Every pull request and push to main branch

**What it does**:
- **Parallel execution**: Build and quality checks run simultaneously
- **Smart caching**: Go modules and quality tools cached across runs
- **Comprehensive quality analysis**: Formatting, linting, static analysis, security, architecture scoring
- **Ratcheting error budgets**: Automatically suggests tightening quality limits as code improves
- **Conditional testing**: CLI integration tests only run when needed
- **Beautiful PR comments**: Automated status summaries with detailed breakdowns
- **Fast feedback**: Typically completes in 3-4 minutes (40-50% improvement)
- **Optimized matrix**: Reduced integration tests from 30 to 9 parallel jobs

### 2. `release.yml` - Release Pipeline
**Runs on**: Version tags (e.g., `v1.0.0`)

**What it does**:
- Builds production binaries
- Runs comprehensive tests
- Creates GitHub release with artifacts
- Publishes release notes

## Ratcheting Error Budget System

Containerization Assist uses an intelligent **ratcheting error budget** that automatically suggests quality improvements:

### How It Works
1. **Current status**: 183 lint issues with 200-issue budget ✅
2. **Improvement detection**: When issues drop 10+ below budget
3. **Auto-suggestion**: Suggests new budget = current issues + 5 buffer
4. **Example**: If issues drop to 170, suggests budget of 175 (170 + 5)

### Configuration (`.github/quality-config.json`)
```json
"linting": {
  "ratcheting_enabled": true,
  "ratcheting_config": {
    "improvement_threshold": 10,  // Ratchet when 10+ issues better
    "buffer_size": 5,             // Keep 5-issue safety buffer
    "auto_apply": false           // Manual review required
  }
}
```

### Benefits
- 🎯 **Prevents regression**: Can't exceed current budget
- 📈 **Encourages improvement**: Rewards quality fixes with tighter budgets  
- 🔒 **Locks in gains**: Quality improvements become permanent
- 🚀 **Gradual progress**: Sustainable quality improvement over time

**Current Opportunity**: Issues at 183, budget at 200 → Can ratchet to 188! 🎉

## Running Tests Locally

Before pushing changes, you can run the same checks locally:

```bash
# Format code
make fmt

# Run linter (will show current issue count)
make lint

# Run unit tests
make test

# Run integration tests
make test-integration
```

## Workflow Configuration

Both workflows are designed to be:
- **Fast**: Optimized parallel execution with Go module & tool caching
- **Intelligent**: Early termination on failures, conditional test execution
- **Comprehensive**: Detailed quality scoring and beautiful PR reporting
- **Reliable**: Structured error handling and artifact management
- **Efficient**: 70% reduction in integration test matrix size

## Troubleshooting

If CI fails:
1. Check the workflow logs in the GitHub Actions tab
2. Run the failing command locally to reproduce
3. Ensure all dependencies are properly declared in `go.mod`

For integration test failures:
- Review Docker and Kubernetes requirements
- Check that all required services are available
- Look for timing-related issues in async operations

## Branch Protection Configuration

To use the CI status check for branch protection:

1. Go to Settings → Branches in your GitHub repository
2. Add or edit a branch protection rule for `main`
3. Enable "Require status checks to pass before merging"
4. Search for and select: **"CI Status Check"**
5. This ensures all CI checks pass before PRs can be merged

The `CI Status Check` job aggregates results from all CI jobs and provides a single status that can be used as a required check.