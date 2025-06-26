# Team C: Fixer Module Integration Fix Plan

## Problem Summary
The fixer module is only partially integrated due to interface mismatches and incomplete implementations. The module provides AI-driven iterative fixing capabilities but is not functioning properly across all tools.

## Critical Issues Found

### 1. Interface Mismatches
**File**: `pkg/mcp/internal/fixers/analyzer_integration.go`
- Missing methods in `IterativeFixer` interface:
  - `AttemptFix()` - Currently using `Fix()` as workaround (line 103-104)
  - `GetFailureRouting()` - Not part of interface (line 117-118)
  - `GetFixStrategies()` - Not part of interface (line 203-204)

### 2. Mock Implementations
**File**: `pkg/mcp/internal/fixers/analyzer_integration.go`
- Using `mockIterativeFixer` instead of real implementation
- Using `mockContextSharer` instead of real implementation

### 3. Limited Tool Integration
- Only `generate_manifests_atomic.go` has fixer properly integrated
- `build_image_atomic.go` has fixing infrastructure but doesn't use `AtomicToolFixingMixin`
- Other tools lack fixer integration entirely

## Implementation Plan

### Phase 1: Fix Interface Definitions (Priority: High)
1. Update `pkg/mcp/types/interfaces.go` to add missing methods to `IterativeFixer`:
   ```go
   type IterativeFixer interface {
       Fix(ctx context.Context, issue interface{}) (*FixingResult, error)
       AttemptFix(ctx context.Context, issue interface{}, attempt int) (*FixingResult, error)
       SetMaxAttempts(max int)
       GetFixHistory() []FixAttempt
       GetFailureRouting() map[string]string
       GetFixStrategies() []string
   }
   ```

2. Update `DefaultIterativeFixer` in `iterative_fixer.go` to implement new methods

### Phase 2: Replace Mock Implementations (Priority: High)
1. Remove mock implementations from `analyzer_integration.go`
2. Use real `DefaultIterativeFixer` instance
3. Implement proper `ContextSharer` or remove if not needed

### Phase 3: Integrate Fixer into Tools (Priority: Medium)
1. Add fixer to `build_image_atomic.go`:
   ```go
   // In NewAtomicBuildImageTool
   tool.AtomicToolFixingMixin.SetAnalyzer(analyzer)
   ```

2. Add fixer to other critical tools:
   - `deploy_kubernetes_atomic.go`
   - `scan_image_security_atomic.go`
   - `validate_dockerfile_atomic.go`

### Phase 4: Test and Validate (Priority: High)
1. Run existing fixer tests
2. Add integration tests for each tool with fixer
3. Test error recovery scenarios

## File Changes Required

### 1. `pkg/mcp/types/interfaces.go`
- Add missing methods to `IterativeFixer` interface

### 2. `pkg/mcp/internal/fixers/iterative_fixer.go`
- Implement `AttemptFix()` method
- Implement `GetFailureRouting()` method
- Implement `GetFixStrategies()` method

### 3. `pkg/mcp/internal/fixers/analyzer_integration.go`
- Remove mock implementations
- Use real `DefaultIterativeFixer`
- Remove TODO comments after fixing interface

### 4. Tool files to update:
- `pkg/mcp/internal/tools/build_image_atomic.go`
- `pkg/mcp/internal/tools/deploy_kubernetes_atomic.go`
- `pkg/mcp/internal/tools/scan_image_security_atomic.go`
- `pkg/mcp/internal/tools/validate_dockerfile_atomic.go`
- `pkg/mcp/internal/tools/pull_image_atomic.go`
- `pkg/mcp/internal/tools/tag_image_atomic.go`
- `pkg/mcp/internal/tools/push_image_atomic.go`

## Success Criteria
- All interface methods properly defined and implemented
- No mock implementations in production code
- Fixer integrated into all atomic tools
- All tests passing
- Error recovery working in build/deploy scenarios

## Risk Assessment
- **Risk Level**: Medium - Modifying core interfaces affects multiple components
- **Impact**: High - Fixer functionality is critical for error recovery
- **Mitigation**: Comprehensive testing after each phase