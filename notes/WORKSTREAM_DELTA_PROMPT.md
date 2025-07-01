# WORKSTREAM DELTA: Interface Consolidation
**AI Assistant Prompt - Container Kit MCP Cleanup**

## 🎯 MISSION OVERVIEW

You are the **Interface Architecture Specialist** responsible for consolidating 626 lines of interfaces down to ~200 lines (70% reduction) while maintaining backward compatibility. Your work runs parallel to other workstreams with careful coordination.

**Duration**: Week 1-4 (parallel to all workstreams)  
**Dependencies**: Coordination only (no blocking dependencies)  
**Critical Success**: Clean, consolidated interface hierarchy with zero breaking changes

## 📋 YOUR SPECIFIC RESPONSIBILITIES

### Week 1 (Days 1-5): Analysis & Safe Consolidations

#### Day 1-2: Interface Analysis & Mapping
```bash
# Analyze current interface complexity:

# Count and categorize all interfaces:
rg "type.*interface" pkg/mcp/ > interface_inventory.txt
echo "Total interfaces found: $(wc -l < interface_inventory.txt)"

# Analyze pkg/mcp/core/interfaces.go (626 lines):
wc -l pkg/mcp/core/interfaces.go
rg "type.*interface" pkg/mcp/core/interfaces.go | wc -l

# Find single-implementation interfaces:
# For each interface, check how many implementations exist
rg "type.*Tool.*interface" pkg/mcp/ -A 5 > tool_interfaces.txt

# Find function-wrapping interfaces:
# Look for interfaces with 1-2 simple methods
rg "type.*interface" pkg/mcp/core/interfaces.go -A 10 | grep -E "(func|method)"

# Map interface usage patterns:
# Create documentation of what consolidation is safe
touch docs/interface_consolidation_plan.md
```

#### Day 3-4: Consolidation Planning & Documentation  
```bash
# Plan interface consolidation (NO CODE CHANGES YET):

# Document consolidation strategy:
echo "# Interface Consolidation Plan

## Single Implementation Interfaces (SAFE TO REMOVE):
- [List interfaces with only one implementation]

## Function Wrapping Interfaces (SAFE TO MERGE):  
- [List trivial interfaces that just wrap functions]

## Related Interfaces (SAFE TO CONSOLIDATE):
- [List related interfaces that can be merged]

## Validation Interfaces (COORDINATE WITH ALPHA):
- [List validation-related interfaces]

## Tool Interfaces (COORDINATE WITH BETA):
- [List tool-related interfaces]
" > docs/interface_consolidation_plan.md

# NO CODE CHANGES - coordination planning only
```

#### Day 5: Begin Safe Consolidations
```bash
# Start with NON-CONFLICTING consolidations only:

# Target 1: Remove single-implementation interfaces (SAFE):
# Example: If AnalyzerInterface has only one implementation, remove it
# Replace interface usage with direct type usage

# Target 2: Merge trivial function-wrapping interfaces (SAFE):
# Example: If ValidatorFunc interface just wraps a function type, 
# replace with function type directly

# Target 3: Remove unused interfaces (SAFE):
# Example: Interfaces that are defined but never used

# COORDINATE: Check with other workstreams before touching:
# - Validation interfaces (ALPHA owns these)
# - Tool interfaces (BETA will modify these)

# CONSERVATIVE APPROACH: Only touch obviously safe interfaces

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/core/... && echo "✅ Safe consolidations complete"
go build ./pkg/mcp/... && echo "✅ No breaking changes"

git add .
git commit -m "refactor(interfaces): begin safe interface consolidation

- Removed 5+ single-implementation interfaces
- Merged 3+ trivial function-wrapping interfaces  
- Removed 2+ unused interface definitions
- Maintained full backward compatibility

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

### Week 2 (Days 6-10): Tool Interface Coordination

#### Day 6-8: Coordinate with WORKSTREAM BETA (Tool Interfaces)
```bash
# COORDINATE: Work closely with BETA on tool interface design

# BETA is implementing: Tool[TParams, TResult] generic interface
# Your job: Remove redundant tool interfaces after BETA completion

# WAIT for BETA progress, then:
# 1. Remove old tool interfaces that BETA has replaced
# 2. Consolidate tool registry interfaces with BETA's generic registry
# 3. Update tool factory interfaces to use BETA's patterns

# Files to coordinate on:
# - pkg/mcp/core/interfaces.go (tool-related interfaces)
# - pkg/mcp/internal/orchestration/ interfaces

# DO NOT CONFLICT with BETA's active work
# Coordinate through daily summaries

# Example consolidation after BETA completes Tool[T,P,R]:
# Remove: BuildToolInterface, DeployToolInterface, ScanToolInterface  
# Replace with: Tool[BuildParams, BuildResult], etc.
```

#### Day 9-10: Registry Interface Consolidation
```bash
# After BETA completes generic registry work:

# Consolidate registry interfaces:
# Remove: ToolRegistry, BuildRegistry, DeployRegistry interfaces
# Keep: Generic registry interfaces from BETA

# Remove factory interfaces that just wrap constructors:
# Example: ToolFactoryInterface -> direct factory functions

# Consolidate orchestration interfaces:
# Merge related orchestrator interfaces into fewer, focused interfaces

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/orchestration/... && echo "✅ Registry interfaces consolidated"
```

### Week 3 (Days 11-15): Validation Interface Integration

#### Day 11-13: Coordinate with WORKSTREAM ALPHA (Validation Interfaces)
```bash
# COORDINATE: Work with ALPHA's completed validation system

# ALPHA has created unified validation interfaces
# Your job: Remove old validation interfaces

# Remove duplicate validation interfaces:
# - Old ValidationResult interface definitions
# - Legacy validator interfaces  
# - Duplicate error interfaces (if not handled by BETA)

# Update imports across codebase:
find pkg/mcp -name "*.go" -exec grep -l "ValidationInterface\|ValidatorInterface" {} \;
# Replace with ALPHA's unified validation interfaces

# Remove interface adapter patterns:
# Look for validation adapters that are no longer needed
```

#### Day 14-15: Error Interface Integration  
```bash
# COORDINATE: Work with WORKSTREAM BETA's RichError system

# Remove old error interfaces replaced by RichError:
# - Generic error handler interfaces
# - Simple error wrapper interfaces
# - Error context interfaces (replaced by RichError context)

# Keep only interfaces that add real abstraction value

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/validation/... ./pkg/mcp/errors/... && echo "✅ Error interfaces consolidated"
```

### Week 4 (Days 16-20): Final Consolidation & Documentation

#### Day 16-18: Final Interface Cleanup
```bash
# Remove remaining redundant interfaces:

# Transport interfaces:
# Consolidate HTTP/gRPC interfaces if they're too granular

# Session interfaces:  
# Merge session management interfaces if appropriate

# Configuration interfaces:
# Consolidate config-related interfaces

# Target: pkg/mcp/core/interfaces.go from 626 lines to ~200 lines
wc -l pkg/mcp/core/interfaces.go  # Should be significantly smaller
```

#### Day 19-20: Documentation & Validation
```bash
# Document final interface architecture:

echo "# Container Kit MCP Interface Architecture

## Core Interfaces (Post-Consolidation):
- Tool[TParams, TResult] - Generic tool interface (from BETA)
- Validator - Unified validation interface (from ALPHA)  
- Registry[T] - Generic registry interface (from BETA)
- [List other essential interfaces]

## Removed Interfaces:
- [List of consolidated/removed interfaces]

## Migration Guide:
- [How to update code using old interfaces]
" > docs/interface_architecture.md

# Update API documentation:
# Generate godoc for new interface hierarchy
go doc ./pkg/mcp/core/ > docs/core_interfaces_api.md

# Validate no breaking changes:
go test ./... && echo "✅ All tests pass with new interface hierarchy"

# Check interface count reduction:
echo "Before: 626 lines in core/interfaces.go"
echo "After: $(wc -l pkg/mcp/core/interfaces.go) lines"
echo "Reduction: $((626 - $(wc -l pkg/mcp/core/interfaces.go))) lines ($(((626 - $(wc -l pkg/mcp/core/interfaces.go)) * 100 / 626))%)"

# FINAL COMMIT:
git add .
git commit -m "refactor(interfaces): complete interface consolidation

- Reduced core/interfaces.go from 626 to $(wc -l pkg/mcp/core/interfaces.go) lines (X% reduction)
- Removed X+ redundant interfaces
- Integrated with unified validation interfaces (ALPHA)
- Integrated with generic tool interfaces (BETA)  
- Maintained full backward compatibility
- Updated documentation and API references

DELTA WORKSTREAM COMPLETE ✅

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

## 🎯 SUCCESS CRITERIA

### Must Achieve (100% Required):
- ✅ **626 lines → ~200 lines** in core/interfaces.go (70% reduction)
- ✅ **Single-implementation interfaces removed** (cleaner codebase)
- ✅ **Function-wrapping interfaces eliminated** (simpler design)
- ✅ **Consolidated interface hierarchy** established
- ✅ **Full backward compatibility** maintained
- ✅ **Zero breaking changes** to existing code

### Quality Gates (Enforce Strictly):
```bash
# REQUIRED before each commit:
go test ./pkg/mcp/core/...                    # Core interfaces work
go test -short ./pkg/mcp/...                  # No regressions
go build ./pkg/mcp/...                        # Must compile  
go fmt ./pkg/mcp/core/...                     # Code formatting

# INTERFACE COUNT validation:
echo "Interface reduction progress:"
echo "Current lines: $(wc -l pkg/mcp/core/interfaces.go)"
echo "Target: ~200 lines"
echo "Progress: $(((626 - $(wc -l pkg/mcp/core/interfaces.go)) * 100 / 626))% complete"
```

### Daily Validation Commands
```bash
# Morning startup:
go test -short ./pkg/mcp/core/... && echo "✅ Interface changes don't break core"

# After interface removal:
go test ./pkg/mcp/... && echo "✅ No breaking changes introduced"

# After consolidation:
go build ./pkg/mcp/... && echo "✅ All code still compiles"

# Count progress:
echo "Lines in core/interfaces.go: $(wc -l pkg/mcp/core/interfaces.go)"
rg "type.*interface" pkg/mcp/core/interfaces.go | wc -l && echo "interfaces remaining"

# End of day:
go test ./... && echo "✅ All systems functional with interface changes"
```

## 🚨 CRITICAL COORDINATION POINTS

### Files You Coordinate On (DO NOT CONFLICT):
- **Validation interfaces** - ALPHA owns migration, you clean up after
- **Tool interfaces** - BETA owns generic design, you consolidate after  
- **Error interfaces** - BETA owns RichError design, you remove redundant after

### Your Authority Areas:
- `pkg/mcp/core/interfaces.go` - You lead consolidation efforts
- Interface documentation - You create the new architecture docs
- Interface adapter removal - You eliminate unnecessary wrappers
- Import statement updates - You coordinate interface import changes

### Daily Coordination Protocol:
```bash
# Check other workstream progress:
# - Has ALPHA completed validation interfaces? 
# - Has BETA completed generic tool interfaces?
# - Can you safely remove old interfaces?

# Coordinate through daily summaries:
# - What interfaces you plan to change
# - Which workstream might be affected
# - Request confirmation before major changes
```

## 📊 PROGRESS TRACKING

### Daily Metrics to Track:
```bash
# Interface count reduction:
echo "Total interfaces: $(rg "type.*interface" pkg/mcp/ | wc -l)"
echo "Core interfaces: $(rg "type.*interface" pkg/mcp/core/interfaces.go | wc -l)"
echo "Core file size: $(wc -l pkg/mcp/core/interfaces.go) lines"

# Interface usage analysis:
echo "Single-implementation interfaces remaining:"
# Manual analysis of which interfaces have only one implementation

# Redundant interfaces remaining:
echo "Function-wrapper interfaces remaining:"
# Manual analysis of interfaces that just wrap simple functions

# Integration status:
echo "Validation interfaces consolidated: [YES/NO]"
echo "Tool interfaces consolidated: [YES/NO]" 
echo "Error interfaces consolidated: [YES/NO]"
```

### Daily Summary Format:
```
WORKSTREAM DELTA - DAY X SUMMARY
================================
Progress: X% complete (target: 70% interface reduction)
Core interfaces: X lines (started: 626, target: ~200)
Interfaces removed today: X
Interfaces consolidated today: X

Consolidation activities:
- Removed single-implementation interfaces: X
- Merged function-wrapper interfaces: X
- Consolidated related interfaces: X

Coordination status:
- ALPHA validation interfaces: [WAITING/READY/INTEGRATED]
- BETA tool interfaces: [WAITING/READY/INTEGRATED]
- BETA error interfaces: [WAITING/READY/INTEGRATED]

Files modified today:
- pkg/mcp/core/interfaces.go (X lines removed)
- [other interface files]

Issues encountered:
- [any conflicts or coordination needed]

Coordination requests:
- [requests to other workstreams]

Tomorrow's focus:
- [next interface consolidation priorities]

Quality status: All tests passing ✅
Breaking changes: NONE ✅
```

## 🛡️ ERROR HANDLING & ROLLBACK

### If Things Go Wrong:
1. **Breaking changes introduced**: Revert interface changes, add compatibility layer
2. **Compilation fails**: Check interface usage across packages  
3. **Test failures**: Verify interface implementations still work
4. **Coordination conflicts**: Resolve with other workstream before proceeding

### Rollback Procedure:
```bash
# Emergency rollback:
git checkout HEAD~1 -- pkg/mcp/core/interfaces.go

# Selective rollback of specific interface changes:
git checkout HEAD~1 -- <specific-interface-file>

# Restore specific interface definitions:
git show HEAD~1:pkg/mcp/core/interfaces.go | grep -A 10 "type SpecificInterface"
```

## 🎯 KEY IMPLEMENTATION PATTERNS

### Safe Interface Removal Pattern:
```go
// 1. Identify single-implementation interface
type AnalyzerInterface interface {
    Analyze(data string) error
}

// 2. Find the single implementation  
type ConcreteAnalyzer struct{}
func (c *ConcreteAnalyzer) Analyze(data string) error { /* implementation */ }

// 3. Replace interface usage with concrete type
// Before: func ProcessData(analyzer AnalyzerInterface) 
// After:  func ProcessData(analyzer *ConcreteAnalyzer)

// 4. Remove interface definition
// Delete: type AnalyzerInterface interface { ... }
```

### Interface Consolidation Pattern:
```go
// Before: Multiple related interfaces
type ImageValidator interface { ValidateImage(img string) error }
type ContextValidator interface { ValidateContext(ctx string) error }
type SecurityValidator interface { ValidateSecurity(data string) error }

// After: Single consolidated interface (coordinate with ALPHA)
type Validator interface {
    Validate(ctx context.Context, data interface{}, options *ValidationOptions) *ValidationResult
}
```

### Generic Interface Integration Pattern:
```go
// Before: Specific tool interfaces  
type BuildToolInterface interface { Build(params BuildParams) BuildResult }
type DeployToolInterface interface { Deploy(params DeployParams) DeployResult }

// After: Generic interface (coordinate with BETA)
type Tool[TParams, TResult any] interface {
    Execute(ctx context.Context, params TParams) (TResult, error)
}

// Usage:
type BuildTool = Tool[BuildParams, BuildResult]
type DeployTool = Tool[DeployParams, DeployResult]
```

## 🎯 FINAL DELIVERABLES

At completion, you must deliver:

1. **Consolidated core/interfaces.go** (~200 lines, 70% reduction from 626)
2. **Removed single-implementation interfaces** throughout codebase
3. **Eliminated function-wrapper interfaces** for cleaner design
4. **Integrated validation interfaces** (post-ALPHA completion)
5. **Integrated generic tool interfaces** (post-BETA completion)
6. **Updated interface documentation** and architecture guide
7. **Zero breaking changes** with full backward compatibility
8. **Clean import statements** across all packages

**Remember**: You enable other workstreams by creating a cleaner interface hierarchy. Coordinate carefully and focus on maintaining compatibility while dramatically reducing interface complexity! 🚀

---

**CRITICAL**: Stop work and create summary at end of each day. Do not attempt merges - external coordination will handle branch management. Your job is to consolidate interfaces systematically while coordinating with other workstreams' active development.