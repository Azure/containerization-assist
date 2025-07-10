# Error Pattern Progressive Reduction Plan

## Current State
- **Starting fmt.Errorf count**: 131 instances (as of Day 3)
- **Target**: <10 fmt.Errorf instances
- **Timeline**: 6 weeks (20 working days)

## Progressive Reduction Targets

### Week 1-2 (Days 1-10)
- **Target**: 135 → 100 instances
- **Focus**: Domain layer conversion
- **Priority Files**:
  - `pkg/mcp/domain/errors/root_errors.go` (25 instances)
  - Other domain layer files (~5 instances)

### Week 3-4 (Days 11-15)  
- **Target**: 100 → 50 instances
- **Focus**: Application layer high-traffic code
- **Priority Files**:
  - `pkg/mcp/application/orchestration/pipeline/background_workers.go`
  - `pkg/mcp/application/services/`
  - Service interface boundaries

### Week 5-6 (Days 16-20)
- **Target**: 50 → 10 instances
- **Focus**: Grandfathering and final cleanup
- **Actions**:
  - Identify performance-critical paths for grandfathering
  - Mark grandfathered instances with `// GRANDFATHERED: reason`
  - Convert remaining non-critical instances

## Enforcement Strategy

### CI/CD Integration
The `.github/workflows/error-linting.yml` workflow enforces progressive targets:
```yaml
if [ $DAYS_ELAPSED -lt 14 ]; then
  MAX_ALLOWED=100
elif [ $DAYS_ELAPSED -lt 28 ]; then
  MAX_ALLOWED=50
else
  MAX_ALLOWED=10
fi
```

### Local Development
Run error pattern checks locally:
```bash
# Check current count
scripts/check-error-patterns.sh 135

# Check against target
scripts/check-error-patterns.sh 50

# Check final target
scripts/check-error-patterns.sh 10
```

### Daily Tracking
Use the linter to track progress:
```bash
cd tools/linters/richerror-boundary
./richerror-boundary ../../../pkg/mcp 10 | grep "Total"
```

## Conversion Guidelines

### High Priority Patterns
1. **Domain Layer** - Must be 100% RichError
2. **Service Boundaries** - Public APIs should use RichError
3. **Error Aggregation** - Use `NewMultiError` for multiple errors

### Grandfathering Criteria
Only grandfather fmt.Errorf in:
1. Performance-critical hot paths (with benchmarks)
2. Test utilities (already excluded from linting)
3. Third-party library constraints

Mark grandfathered instances:
```go
return fmt.Errorf("error: %w", err) // GRANDFATHERED: Hot path performance
```

## Progress Tracking

| Week | Target | Status | Notes |
|------|--------|--------|-------|
| 1-2  | 100    | 🟡     | Domain layer focus |
| 3-4  | 50     | ⏳     | Application layer |
| 5-6  | <10    | ⏳     | Final cleanup |

## Success Metrics
- ✅ <10 fmt.Errorf instances by Day 20
- ✅ 100% RichError in domain layer
- ✅ All grandfathered instances documented
- ✅ CI enforcement active