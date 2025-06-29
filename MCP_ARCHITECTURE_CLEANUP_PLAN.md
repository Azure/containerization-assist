# MCP Architecture Cleanup Implementation Plan

## Executive Summary

This plan addresses the remaining architectural issues in the pkg/mcp modules, focusing on completing the interface unification, eliminating remaining adapters, removing legacy code, and consolidating type systems. Since this is a new system with no users, we can make aggressive breaking changes without compatibility concerns.

**Total Duration**: 6-8 days
**Team Size**: 1-2 developers
**Lines to Remove**: ~2,500+ (estimated)

## Current State Analysis

### Issues to Address
1. **Interface Re-exports**: ~175 lines in `pkg/mcp/interfaces.go`
2. **Remaining Adapters**: ~150 lines across multiple files
3. **Legacy Interfaces**: ~97 lines of legacy types
4. **Error System Fragmentation**: 3 parallel error systems with converters
5. **Type Conversions**: ~500 lines of conversion code
6. **Wrapper Functions**: ~800 lines in gomcp_tools.go alone

## Phase 1: Direct Interface Cleanup (Days 1-2)

### Day 1: Remove Interface Re-exports and Legacy Code

#### Task 1.1: Create Baseline Metrics (1 hour)
```bash
./validation.sh > baseline_metrics.txt
```

#### Task 1.2: Remove Interface Re-exports (3 hours)
1. Delete the entire BACKWARD COMPATIBILITY section from `pkg/mcp/interfaces.go`
2. Delete the entire LEGACY TYPES section from `pkg/mcp/interfaces.go`
3. Update all imports to use `pkg/mcp/core` directly

#### Task 1.3: Remove sessionManagerAdapter (2 hours)
1. Delete `sessionManagerAdapterImpl` from `pkg/mcp/internal/core/server.go`
2. Update orchestration to use core.SessionManager directly

#### Task 1.4: Remove Error Migration Code (2 hours)
1. Delete `pkg/mcp/internal/errors/migration.go`
2. Choose CoreError as single error type
3. Update all error usage

#### Task 2.1: Remove Conversion Functions (4 hours)
1. Delete `pkg/mcp/internal/runtime/conversation/stage_conversion.go`
2. Remove all `BuildArgsMap()` functions
3. Update tools to accept typed arguments directly

#### Task 2.2: Remove Tool Registration Wrappers (4 hours)
1. Update `pkg/mcp/internal/core/gomcp_tools.go` to register tools directly
2. Remove intermediate wrapper functions
3. Direct tool execution through Tool.Execute()

## Phase 2: Final Cleanup and Validation (Days 3-4)

### Day 3: Complete Cleanup

#### Task 3.1: Remove Operation Wrappers (3 hours)
1. Evaluate if Operation wrapper in deploy/operation.go is needed
2. Move retry logic to individual tools if appropriate
3. Remove unnecessary abstractions

#### Task 3.2: Clean Up Remaining Issues (3 hours)
1. Remove any remaining wrapper files
2. Consolidate duplicate type definitions
3. Remove dead code

#### Task 3.3: Update Imports (2 hours)
```bash
# Update all imports to use core interfaces
find . -name "*.go" -type f -exec sed -i 's|"github.com/Azure/container-kit/pkg/mcp"|"github.com/Azure/container-kit/pkg/mcp/core"|g' {} \;
```

### Day 4: Testing and Validation

#### Task 4.1: Comprehensive Testing (4 hours)
```bash
# Run validation script
./validation.sh

# Run all tests
make test-all

# Performance testing
make bench
```

#### Task 4.2: Final Documentation (4 hours)
1. Update architecture documentation
2. Update CLAUDE.md if needed
3. Create cleanup summary report

## Implementation Checklist

### Pre-Implementation
- [ ] Baseline metrics collected
- [ ] Dependency analysis complete
- [ ] Risk assessment documented
- [ ] Stakeholders notified

### Interface Unification
- [ ] Re-exports removed
- [ ] Legacy interfaces deleted
- [ ] Imports updated throughout codebase
- [ ] No import cycles

### Adapter Elimination
- [ ] Session manager adapter removed
- [ ] Progress adapters eliminated
- [ ] Conversion functions deleted
- [ ] Direct type usage implemented

### Error Consolidation
- [ ] Single error type chosen
- [ ] All errors migrated
- [ ] Migration code removed
- [ ] Tests updated

### Wrapper Removal
- [ ] Tool registration simplified
- [ ] Operation wrappers evaluated
- [ ] Direct execution implemented
- [ ] Unnecessary abstractions removed

### Validation
- [ ] All tests passing
- [ ] Performance benchmarks acceptable
- [ ] Integration tests successful
- [ ] Documentation complete

## Risk Mitigation

### Performance Risks
- Benchmark before and after each phase
- Profile hot paths
- Optimize if regression > 5%

### Build Risks
- Test compilation after each major change
- Maintain git commits for easy rollback
- Run tests frequently

## Success Metrics

### Quantitative
- **Lines removed**: 2,500+ (target)
- **Interface definitions**: 1 (down from multiple)
- **Adapter files**: 0 (down from 10+)
- **Error types**: 1 (down from 3)
- **Build time improvement**: 20%+
- **Test coverage**: Maintained at 70%+

### Qualitative
- Cleaner architecture
- Easier to understand codebase
- Reduced cognitive load
- Better maintainability
- Clear separation of concerns

## Rollback Plan

If issues arise:
```bash
# Tag before starting
git tag pre-cleanup-baseline

# Create rollback branch
git checkout -b cleanup-rollback
git reset --hard pre-cleanup-baseline

# Selective rollback
# Can revert individual phases if needed
```

## Quick Start Implementation

To begin the cleanup immediately:

```bash
# 1. Create baseline
./validation.sh > baseline_metrics.txt

# 2. Start with interface cleanup
git checkout -b mcp-architecture-cleanup

# 3. Follow the tasks in order
```

## Appendix: Automation Scripts

### A. Import Update Script
```bash
#!/bin/bash
# update-imports.sh

echo "Updating imports from pkg/mcp to pkg/mcp/core..."

# Backup current state
git stash
git checkout -b import-updates

# Update Go imports
find . -name "*.go" -type f | while read file; do
    sed -i.bak 's|"github.com/Azure/container-kit/pkg/mcp"|"github.com/Azure/container-kit/pkg/mcp/core"|g' "$file"
    # Remove backup files
    rm "${file}.bak"
done

# Verify builds
if go build -tags mcp ./pkg/mcp/...; then
    echo "✅ Import updates successful"
else
    echo "❌ Build failed after import updates"
    exit 1
fi
```

### B. Validation Script
```bash
#!/bin/bash
# validate-cleanup.sh

echo "=== MCP Architecture Cleanup Validation ==="

# Check for adapters
ADAPTER_COUNT=$(find pkg/mcp -name "*adapter*.go" | wc -l)
echo "Adapter files: $ADAPTER_COUNT (target: 0)"

# Check for converters
CONVERTER_COUNT=$(grep -r "convert\|Convert" pkg/mcp --include="*.go" | grep -v "comment" | wc -l)
echo "Converter references: $CONVERTER_COUNT (target: <10)"

# Check interface definitions
INTERFACE_COUNT=$(grep -r "type Tool interface" pkg/mcp --include="*.go" | wc -l)
echo "Tool interface definitions: $INTERFACE_COUNT (target: 1)"

# Check error types
ERROR_TYPES=$(grep -r "type.*Error.*struct" pkg/mcp --include="*.go" | wc -l)
echo "Error type definitions: $ERROR_TYPES (target: 1)"

# Run tests
echo -e "\n=== Running Tests ==="
make test-mcp

# Final verdict
if [ $ADAPTER_COUNT -eq 0 ] && [ $INTERFACE_COUNT -eq 1 ] && [ $ERROR_TYPES -le 2 ]; then
    echo -e "\n✅ Architecture cleanup successful!"
else
    echo -e "\n❌ Cleanup incomplete - check metrics above"
    exit 1
fi
```

This implementation plan provides a structured, phased approach to cleaning up the MCP architecture while minimizing risk and ensuring proper validation at each step.
