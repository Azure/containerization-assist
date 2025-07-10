# GitHub Actions Workflows

This directory contains automated quality gates and CI pipelines for the Container Kit project with **beautiful, updatable PR comments**.

## 🚀 Enhanced PR Dashboard System

The workflows now provide a unified, visually appealing dashboard that **updates existing comments** rather than creating new ones on every push.

### 🎯 Key Workflows

| Workflow | Purpose | PR Comments | Status |
|----------|---------|-------------|--------|
| [pr-status-unified.yml](pr-status-unified.yml) | **🌟 Main PR Dashboard** | Single unified comment | ✅ Enhanced |
| [quality-gates-combined.yml](quality-gates-combined.yml) | Quality enforcement | Integrated with dashboard | ✅ Enhanced |
| [ci-dashboard.yml](ci-dashboard.yml) | CI pipeline tracking | Dedicated CI summary | ✅ Enhanced |
| [quality-gates-enhanced.yml](quality-gates-enhanced.yml) | Detailed quality analysis | Rich quality breakdown | ✅ Enhanced |

### 🎨 PR Comment Features

**✨ Visual Excellence:**
- 🏆 Status badges with color coding
- 📊 Progress indicators and metrics
- 🔍 Collapsible detailed sections
- 📈 Quality scores and trends

**🔄 Smart Updates:**
- Comments update in-place (no spam)
- Real-time status changes
- Comprehensive change analysis
- Historical tracking

**📋 Unified Information:**
- Overall PR health at a glance
- Quality gate results with actionable fixes
- CI pipeline status and links
- Code change impact assessment

## 📊 Dashboard Components

### Main PR Dashboard (`pr-status-unified.yml`)
```
🎉 Container Kit PR Dashboard

📊 Overall Status: 🎉 EXCELLENT
🛡️ Quality Score: 85/100
📝 Change Size: Small

🔍 Quick Summary
├── CI Pipeline: ✅ PASSED (8/8 checks)
├── Code Quality: ✅ EXCELLENT (2 issues)
├── Code Changes: 📝 Small (+45 -12 lines)
└── Files Modified: 5 files (3 Go, 2 tests)
```

### Quality Gate Results
- ✅/❌ Status for each quality check
- 📁 Oversized files with line counts
- 🧮 Complex functions requiring refactoring
- 🔗 Context usage violations
- 🚫 Print statement locations

### CI Pipeline Tracking
- 🔄 Real-time build status
- 📊 Test results and coverage
- 🔍 Direct links to failing checks
- ⚡ Performance metrics

## Current Quality Status

### ✅ Passing Gates
- **Import Cycles**: No cycles detected
- **Context Usage**: Proper propagation maintained
- **Package Depth**: All packages within 5-level limit
- **Constructor Parameters**: Functional options pattern used
- **Logging Standards**: Consistent key usage

### ⚠️ Issues to Address

**File Length Violations (5 files):**
```
pkg/mcp/internal/migration/analysis.go         (938 lines)
pkg/mcp/internal/analyze/validate_dockerfile_atomic.go (984 lines)
pkg/mcp/internal/pipeline/production_validation.go (838 lines)
pkg/mcp/internal/pipeline/docker_optimizer.go  (804 lines)
pkg/mcp/internal/server/server.go              (874 lines)
```

**Complexity Violations (Top 5):**
```
build.categorizeFailure                         (27 complexity)
deploy.RecreateStrategy.Deploy                  (26 complexity)
build.classifyFailure                          (26 complexity)
transport.HTTPLLMTransport.InvokeTool          (26 complexity)
tools.NumberConstraint.Validate               (25 complexity)
```

**Print Statement Violations:**
```
pkg/mcp/internal/observability/distributed_tracing.go (fmt.Printf comment)
pkg/mcp/validation/utils/pattern_analysis.go          (debug printf)
pkg/mcp/validation/doc.go                             (example printf)
```

## Usage

### Automatic Execution
All quality gates run automatically on pull requests. No manual intervention required.

### Local Testing
Test quality gates before pushing:

```bash
# File length check
find pkg/mcp -name '*.go' -exec wc -l {} + | awk '$1>800{print $2 " exceeds 800 lines"}'

# Complexity check
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
gocyclo -over 15 pkg/mcp

# Import cycles
go list -json ./pkg/mcp/... 2>&1 | grep "import cycle"

# Print statements
grep -r -E '(fmt|log)\.Print' pkg/mcp --include="*.go" | grep -v '_test\.go'
```

### Disabling Gates
To temporarily disable a gate for emergency fixes:

```yaml
# Add to pull request description:
skip-quality-gates: file-length,complexity
```

## Customization

### Adjusting Thresholds
Edit the workflow files to modify limits:

```yaml
# In file-length-gate.yml
THRESHOLD=800  # Change to your preferred limit

# In complexity-gate.yml
gocyclo -over 15  # Change 15 to your preferred complexity
```

### Adding New Gates
1. Create new workflow file in this directory
2. Use existing gates as templates
3. Add entry to this README
4. Test locally before committing

## Integration with Existing Workflows

### Sequential Execution
```yaml
jobs:
  tests:
    # ... existing test job

  quality-gates:
    needs: tests
    uses: ./.github/workflows/quality-gates-combined.yml
```

### Parallel Execution
```yaml
jobs:
  tests:
    # ... existing test job

  quality-gates:
    uses: ./.github/workflows/quality-gates-combined.yml
```

## Troubleshooting

### Common Issues

**Quality gate failing on unrelated changes:**
- Gates only run on changes to `pkg/mcp/**` paths
- Check if your changes modify files outside this scope

**False positives:**
- Review the specific threshold that's failing
- Consider if the code genuinely needs refactoring
- Use local testing to verify fixes before pushing

**Tool installation failures:**
- GitHub runners use cached Go installations
- Tools are installed fresh on each run to ensure latest versions

### Getting Help

1. **Check workflow logs**: Click on failed workflow in GitHub Actions tab
2. **Test locally**: Use the local testing commands above
3. **Review violations**: Focus on high-impact issues first (complexity, file length)
4. **Gradual improvement**: Fix violations incrementally across multiple PRs

## Maintenance

### Monthly Review
- Review threshold effectiveness
- Check for new code patterns that need gates
- Update tool versions in workflows

### Quarterly Updates
- Analyze quality trends
- Adjust thresholds based on codebase evolution
- Add new gates for emerging anti-patterns

## Related Documentation

- [CI Quality Gates Plan](../docs/notes/plan/CI_QUALITY_GATES.md) - Original requirements
- [Dead Code Cleanup Guide](../docs/DEAD_CODE_CLEANUP_GUIDE.md) - Manual cleanup procedures
- [Architecture Guidelines](../docs/ARCHITECTURE.md) - Coding standards and patterns
