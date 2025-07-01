# Interface Consolidation Plan

## Overview
This document outlines the plan to consolidate interfaces in the Container Kit MCP codebase from **626 lines** in core/interfaces.go down to approximately **200 lines** (70% reduction).

**Current State:**
- Total interfaces found: **163** across pkg/mcp/
- Core interfaces file: **625 lines** with **16 interfaces**
- Target: **~200 lines** (70% reduction)

## Single Implementation Interfaces (SAFE TO REMOVE)

### High Priority Consolidation Candidates

1. **ContextSharer interface** (`pkg/mcp/core/interfaces.go:487`)
   - **Implementation**: `DefaultContextSharer` in `pkg/mcp/internal/build/context_sharer.go`
   - **Lines saved**: ~8 interface lines
   - **Action**: Replace interface usage with concrete type

2. **IterativeFixer interface** (`pkg/mcp/core/interfaces.go:467`)
   - **Implementation**: `DefaultIterativeFixer` in `pkg/mcp/internal/build/iterative_fixer.go`
   - **Lines saved**: ~5 interface lines
   - **Action**: Replace interface usage with concrete type

3. **PipelineOperations interface** (`pkg/mcp/core/interfaces.go:522`)
   - **Implementation**: `Operations` in `pkg/mcp/internal/pipeline/operations.go`
   - **Lines saved**: ~25 interface lines (large interface)
   - **Action**: Replace interface usage with concrete type

4. **ToolRegistry interface** (`pkg/mcp/core/interfaces.go:154`)
   - **Implementation**: `MCPToolRegistry` in `pkg/mcp/internal/orchestration/tool_registry.go`
   - **Lines saved**: ~7 interface lines
   - **Action**: Replace interface usage with concrete type

**Total estimated lines saved from single-implementation interfaces: ~45 lines**

## Function Wrapping Interfaces (SAFE TO MERGE)

### Medium Priority Consolidation Candidates

5. **ProgressReporter interface** (`pkg/mcp/core/interfaces.go:52`)
   - **Analysis**: Simple function-wrapper interface for progress reporting
   - **Lines saved**: ~5 interface lines
   - **Action**: Convert to function type or embed in consuming types

6. **ToolSessionManager interface** (`pkg/mcp/core/interfaces.go:548`)
   - **Analysis**: Adapter interface for session management
   - **Lines saved**: ~13 interface lines
   - **Action**: Merge functionality into consuming types

7. **Session interface** (`pkg/mcp/core/interfaces.go:187`)
   - **Analysis**: Underutilized interface, implementations use adapters
   - **Lines saved**: ~6 interface lines
   - **Action**: Simplify or merge with SessionState

**Total estimated lines saved from function-wrapping interfaces: ~24 lines**

## Related Interfaces (SAFE TO CONSOLIDATE)

### Lower Priority Consolidation Candidates

8. **BuildImageSessionManager interface** (`pkg/mcp/internal/build/build_image.go`)
   - **Analysis**: Specific adapter interface
   - **Action**: Merge with general session management

9. **BuildImagePipelineAdapter interface** (`pkg/mcp/internal/build/build_image.go`)
   - **Analysis**: Specific adapter interface
   - **Action**: Remove adapter pattern, use direct dependencies

10. **Multiple ToolAnalyzer interfaces** (scattered across packages)
    - **Analysis**: Duplicate interfaces with same purpose
    - **Action**: Consolidate into single interface

**Total estimated lines saved from related interfaces: ~15 lines**

## Validation Interfaces (COORDINATE WITH ALPHA)

**Note**: These interfaces are owned by WORKSTREAM ALPHA and should not be modified until ALPHA completes their work.

- Validation interfaces in `pkg/mcp/validation/core/interfaces.go` (261 lines)
- Wait for ALPHA to complete unified validation system
- Then remove old validation interface duplicates

## Tool Interfaces (COORDINATE WITH BETA)

**Note**: These interfaces will be affected by WORKSTREAM BETA's generic tool interface work.

- Current `Tool` interface may be replaced with `Tool[TParams, TResult]` generic interface
- Wait for BETA to complete generic tool design
- Then remove old tool-specific interfaces

## Error Interfaces (COORDINATE WITH BETA)

**Note**: These interfaces will be affected by WORKSTREAM BETA's RichError system.

- Error handler interfaces may be replaced by RichError
- Coordinate with BETA on error interface consolidation

## Implementation Strategy

### Phase 1: Safe Single-Implementation Removals (Days 1-5)
1. Replace `ContextSharer` interface with `DefaultContextSharer` concrete type
2. Replace `IterativeFixer` interface with `DefaultIterativeFixer` concrete type
3. Replace `PipelineOperations` interface with `Operations` concrete type
4. Replace `ToolRegistry` interface with `MCPToolRegistry` concrete type

### Phase 2: Function-Wrapper Simplification (Week 2)
1. Convert `ProgressReporter` to function type or embed in consumers
2. Merge `ToolSessionManager` functionality into consuming types
3. Simplify `Session` interface or merge with `SessionState`

### Phase 3: Coordination Phase (Weeks 3-4)
1. Wait for ALPHA validation interface completion
2. Wait for BETA tool interface completion
3. Remove old interfaces replaced by other workstreams
4. Final cleanup and documentation

## Validation Requirements

### Before Each Consolidation:
```bash
# Core interfaces must still work
go test -short ./pkg/mcp/core/...

# No regressions in packages
go test -short ./pkg/mcp/...

# All code must compile
go build ./pkg/mcp/...

# Code formatting
go fmt ./pkg/mcp/core/...
```

### Progress Tracking:
```bash
# Check current line count
wc -l pkg/mcp/core/interfaces.go

# Check interface count
rg "type.*interface" pkg/mcp/core/interfaces.go | wc -l

# Calculate progress
echo "Progress: $(((625 - $(wc -l pkg/mcp/core/interfaces.go | cut -d' ' -f1)) * 100 / 625))% complete"
```

## Expected Results

**Conservative Estimate:**
- Single-implementation interfaces: **-45 lines**
- Function-wrapper interfaces: **-24 lines**
- Related interface consolidation: **-15 lines**
- Additional cleanup: **-10 lines**
- **Total reduction: ~94 lines** (15% reduction)

**Optimistic Estimate (with coordination):**
- Above consolidations: **-94 lines**
- Validation interface cleanup: **-50 lines** (after ALPHA)
- Tool interface cleanup: **-30 lines** (after BETA)
- Error interface cleanup: **-20 lines** (after BETA)
- **Total reduction: ~194 lines** (31% reduction)

**Stretch Goal (aggressive consolidation):**
- With aggressive consolidation of adapter patterns: **-250+ lines**
- **Target: 625 → 375 lines** (40% reduction)

## Risk Mitigation

### Low Risk Consolidations (Do First):
- Single-implementation interfaces
- Unused interface definitions
- Obvious function wrappers

### Medium Risk Consolidations (Coordinate):
- Interfaces shared between packages
- Interfaces with complex usage patterns

### High Risk Consolidations (Do Last):
- Core protocol interfaces
- Interfaces modified by other workstreams
- Interfaces with external dependencies

## Success Metrics

**Must Achieve:**
- ✅ Zero breaking changes
- ✅ All tests pass after each consolidation
- ✅ Code compiles successfully
- ✅ Minimum 15% reduction in interface lines

**Stretch Goals:**
- 🎯 30%+ reduction in interface lines
- 🎯 Elimination of all single-implementation interfaces
- 🎯 Removal of all function-wrapper interfaces
- 🎯 Clean integration with ALPHA and BETA workstreams
