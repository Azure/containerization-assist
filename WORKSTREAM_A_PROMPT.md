# AI Assistant Prompt: Workstream A - Interface & Type System Consolidation

## 🎯 Mission Brief
You are the **Lead Developer for Workstream A** in a critical architecture cleanup project. Your mission is to **consolidate duplicate interfaces and eliminate type conversion systems** in the Container Kit MCP server codebase over **3 days**.

## 📋 Project Context
- **Repository**: Container Kit MCP server (`pkg/mcp/` directory)
- **Goal**: Clean architecture with single unified interfaces and direct typing
- **Team**: 4 parallel workstreams (you are Workstream A - foundation work)
- **Timeline**: 3 days (Day 1-3 of parallel implementation)
- **Impact**: Foundation for other workstreams, ~650 lines of code cleanup

## 🚨 Critical Success Factors

### Must-Do Items
1. **Interface Unification**: Reduce from 8+ duplicate interfaces to 1 canonical definition
2. **Type System Cleanup**: Remove `map[string]interface{}` conversion patterns 
3. **Import Consolidation**: All interface imports point to single source
4. **Validation**: Zero import cycles, all tests pass

### Must-Not-Do Items
- ❌ **Do NOT modify legacy CLI code** (only `pkg/mcp/` directory)
- ❌ **Do NOT break existing functionality** 
- ❌ **Do NOT work on adapter files** (that's Workstream B)
- ❌ **Do NOT remove legacy methods** (that's Workstream C)

## 📂 Your File Ownership (You Own These)

### Primary Targets
```
pkg/mcp/core/interfaces.go                          # Keep as canonical
pkg/mcp/internal/core/tool_middleware.go            # DELETE interface duplicate  
pkg/mcp/internal/runtime/registry.go                # Remove UnifiedTool interface
pkg/mcp/internal/orchestration/types.go             # Fix ToolMetadata field types
pkg/mcp/internal/orchestration/no_reflect_orchestrator*.go  # Remove conversions
pkg/mcp/internal/utils/schema_utils.go              # Simplify schema conversions
pkg/mcp/internal/pipeline/helpers.go                # Remove conversion utilities
```

### Do NOT Touch (Other Workstreams)
```
pkg/mcp/client_factory.go                           # Workstream B (adapters)
pkg/mcp/internal/analyze/analyzer.go                # Workstream B (adapters)  
pkg/mcp/internal/core/gomcp_tools.go                # Workstream B (session wrapper)
pkg/mcp/internal/state/migrators.go                 # Workstream C (legacy)
pkg/mcp/internal/config/migration.go                # Workstream C (legacy)
*_test.go files                                      # Workstream D (testing)
```

## 📅 3-Day Implementation Plan

### Day 1: Interface Audit & Consolidation (8 hours)

#### Morning (4 hours): Interface Audit
```bash
# 1. Create baseline and audit current state
# (Branch already created - just start working)

# 2. Map all Tool interface definitions
rg "type Tool interface" pkg/mcp/ -A 5 > interface_audit.txt
rg "type.*Tool.*interface" pkg/mcp/ -A 3 >> interface_audit.txt
rg "UnifiedTool" pkg/mcp/ >> interface_audit.txt
echo "📊 Found interface definitions - review interface_audit.txt"

# 3. Map ToolMetadata duplicates  
rg "type ToolMetadata struct" pkg/mcp/ -A 10 > metadata_audit.txt
echo "📊 Found ToolMetadata definitions - review metadata_audit.txt"
```

#### Afternoon (4 hours): Start Interface Consolidation
```bash
# 1. Keep pkg/mcp/core/interfaces.go as canonical Tool interface
# 2. DELETE pkg/mcp/internal/core/tool_middleware.go (entire file)
rm pkg/mcp/internal/core/tool_middleware.go

# 3. Remove UnifiedTool interface from registry.go
# Edit pkg/mcp/internal/runtime/registry.go - remove lines 23-27 (UnifiedTool interface)
# Replace all references to UnifiedTool with core.Tool

# 4. Fix ToolMetadata field type inconsistency
# Edit pkg/mcp/internal/orchestration/types.go line ~19
# Change: Parameters map[string]interface{} 
# To:     Parameters map[string]string
```

### Day 2: Import Updates & Type Conversion Start (8 hours)

#### Morning (4 hours): Update Interface Imports
```bash
# 1. Update all UnifiedTool references to Tool
find pkg/mcp -name "*.go" -exec sed -i 's/UnifiedTool/Tool/g' {} \;

# 2. Update interface import paths
find pkg/mcp -name "*.go" -exec grep -l "tool_middleware" {} \; | xargs sed -i '/tool_middleware/d'
find pkg/mcp -name "*.go" -exec sed -i 's|internal/runtime.*Tool|core.Tool|g' {} \;

# 3. Validate no import cycles
go build -tags mcp ./pkg/mcp/...
if [ $? -eq 0 ]; then echo "✅ No import cycles"; else echo "❌ Import cycles detected"; fi
```

#### Afternoon (4 hours): Start Type Conversion Removal
```bash
# 1. Begin removing map[string]interface{} conversions in orchestration
# Focus on pkg/mcp/internal/orchestration/no_reflect_orchestrator_impl.go
# Remove or simplify BuildArgs conversion patterns (lines ~50-100)
# Remove or simplify Environment conversion patterns 
# Document changes in conversion_changes.txt
```

### Day 3: Complete Type Conversions & Validation (8 hours)

#### Morning (4 hours): Complete Conversion Removal
```bash
# 1. Finish orchestration conversion removal
# Remove slice conversion patterns ([]interface{} -> []string)
# Remove complex type construction from interface{} maps

# 2. Simplify schema utilities
# Edit pkg/mcp/internal/utils/schema_utils.go
# Keep only essential MCP protocol compliance
# Remove unnecessary conversion helpers

# 3. Clean up pipeline helpers
# Edit pkg/mcp/internal/pipeline/helpers.go  
# Remove or simplify MetadataManager conversion utilities
```

#### Afternoon (4 hours): Final Validation
```bash
# 1. Run comprehensive validation
./validation.sh > workstream_a_final_metrics.txt

# 2. Verify success criteria
interface_count=$(rg "type Tool interface" pkg/mcp/ | wc -l)
echo "Tool interfaces found: $interface_count (target: 1)"

# 3. Test compilation and basic functionality
go test -short -tags mcp ./pkg/mcp/...
golangci-lint run ./pkg/mcp/...

# 4. Document completion
echo "✅ Workstream A Complete - Interface consolidation and type cleanup finished"

# 5. Create completion summary
cat > day3_completion_summary.txt << EOF
WORKSTREAM A - DAY 3 COMPLETION
================================
Interface consolidation: COMPLETE
Type conversion removal: COMPLETE
Import cycle elimination: COMPLETE

Key changes made:
- [List major files modified]
- [List interface consolidations]
- [List conversion removals]

Ready for merge: YES
EOF
```

## 🎯 Detailed Task Instructions

### Task 1: Interface Consolidation (Day 1)

**Objective**: Single canonical Tool interface in `pkg/mcp/core/interfaces.go`

**Steps**:
1. **Identify duplicates**: Use `rg "type Tool interface" pkg/mcp/` to find all Tool interface definitions
2. **Keep canonical**: Preserve `pkg/mcp/core/interfaces.go:19-23` as the only Tool interface
3. **Remove duplicates**: 
   - Delete `pkg/mcp/internal/core/tool_middleware.go` (lines 14-16)
   - Delete UnifiedTool interface from `pkg/mcp/internal/runtime/registry.go` (lines 23-27)
4. **Validate**: Ensure `rg "type Tool interface" pkg/mcp/ | wc -l` returns 1

### Task 2: ToolMetadata Type Fix (Day 1)

**Objective**: Consistent ToolMetadata.Parameters field type

**Issue**: 
- `pkg/mcp/core/interfaces.go`: `Parameters map[string]string`
- `pkg/mcp/internal/orchestration/types.go`: `Parameters map[string]interface{}`

**Fix**: Change orchestration version to match core: `Parameters map[string]string`

### Task 3: Import Path Updates (Day 2)

**Objective**: All Tool interface usage points to `pkg/mcp/core`

**Commands**:
```bash
# Replace UnifiedTool with Tool
find pkg/mcp -name "*.go" -exec sed -i 's/UnifiedTool/Tool/g' {} \;

# Remove tool_middleware imports  
find pkg/mcp -name "*.go" -exec sed -i '/tool_middleware/d' {} \;

# Update import paths to core
find pkg/mcp -name "*.go" -exec sed -i 's|internal/runtime\..*Tool|core.Tool|g' {} \;
```

### Task 4: Type Conversion Removal (Day 2-3)

**Objective**: Remove `map[string]interface{}` conversion patterns

**Focus Files**:
- `pkg/mcp/internal/orchestration/no_reflect_orchestrator_impl.go`
- `pkg/mcp/internal/utils/schema_utils.go`  
- `pkg/mcp/internal/pipeline/helpers.go`

**Patterns to Remove**:
```go
// REMOVE: Map conversion patterns
if buildArgs, ok := argsMap["build_args"].(map[string]interface{}); ok {
    args.BuildArgs = make(map[string]string)
    for k, v := range buildArgs {
        args.BuildArgs[k] = fmt.Sprintf("%v", v)
    }
}

// REMOVE: Slice conversion patterns  
if vulnTypes, ok := argsMap["vuln_types"].([]interface{}); ok {
    args.VulnTypes = make([]string, len(vulnTypes))
    for i, v := range vulnTypes {
        args.VulnTypes[i] = fmt.Sprintf("%v", v)
    }
}
```

## 📊 Success Criteria Validation

### After Day 1
```bash
# Interface consolidation check
interface_count=$(rg "type Tool interface" pkg/mcp/ | wc -l)
[ $interface_count -eq 1 ] && echo "✅ Interface consolidation" || echo "❌ Multiple interfaces remain"

# ToolMetadata consistency check
metadata_inconsistent=$(rg "Parameters.*map\[string\]interface" pkg/mcp/)
[ -z "$metadata_inconsistent" ] && echo "✅ ToolMetadata consistent" || echo "❌ Type inconsistency remains"
```

### After Day 2  
```bash
# Import validation
import_cycles=$(go build -tags mcp ./pkg/mcp/... 2>&1 | grep -c "import cycle")
[ $import_cycles -eq 0 ] && echo "✅ No import cycles" || echo "❌ Import cycles detected"

# UnifiedTool removal validation
unified_refs=$(rg "UnifiedTool" pkg/mcp/ | wc -l)
[ $unified_refs -eq 0 ] && echo "✅ UnifiedTool removed" || echo "❌ UnifiedTool references remain"
```

### After Day 3
```bash
# Conversion pattern reduction
conversion_patterns=$(rg "make\(map\[string\]string\)" pkg/mcp/internal/orchestration/ | wc -l)
echo "Conversion patterns remaining: $conversion_patterns (target: <5)"

# Test validation
go test -short -tags mcp ./pkg/mcp/... && echo "✅ Tests pass" || echo "❌ Tests failing"
golangci-lint run ./pkg/mcp/... && echo "✅ Lint clean" || echo "❌ Lint issues"
```

## 🚨 Common Pitfalls & How to Avoid

### Pitfall 1: Breaking Import Cycles
**Problem**: Removing interfaces can create new import cycles
**Solution**: Always test with `go build -tags mcp ./pkg/mcp/...` after changes

### Pitfall 2: Type Assertion Failures  
**Problem**: Changing interface types breaks existing code
**Solution**: Update all interface usage when consolidating

### Pitfall 3: Test Failures
**Problem**: Tests expect old interface structure  
**Solution**: Let Workstream D handle test updates, focus on core functionality

### Pitfall 4: Conflicting with Other Workstreams
**Problem**: Modifying files owned by other workstreams
**Solution**: Stick to your file ownership list, coordinate in daily standup

## 🤝 Source Code Management

### Daily Work Process
1. **Start each day**: You'll already be on the correct branch
2. **Make your changes**: Follow the daily plan
3. **Test frequently**: Ensure changes don't break functionality
4. **Commit regularly**: Save progress throughout the day

### End-of-Day Process
```bash
# At the end of each day, commit all your changes:
git add -A
git commit -m "feat(workstream-a): day X interface consolidation progress"

# Create a summary of your changes
cat > day_X_summary.txt << EOF
WORKSTREAM A - DAY X SUMMARY
============================
Progress: X% complete
Interfaces consolidated: ✅/❌
Conversions removed: ✅/❌

Files modified:
- [list key files changed]

Issues encountered:
- [any blockers or concerns]

Tomorrow's focus:
- [next priorities]
EOF

# STOP HERE - Merge will be handled externally
echo "✅ Day X work complete - ready for external merge"
```

### Coordination Notes
- **Shared files**: If you need to modify files owned by other workstreams, document this in your summary
- **Blockers**: Note any dependencies on other workstreams in your summary
- **Testing**: Always ensure `go test -short -tags mcp ./pkg/mcp/...` passes before ending the day

## 🎯 Success Metrics

### Quantitative Targets
- **Interface Definitions**: 8+ → 1 (reduce by ~90%)
- **ToolMetadata Duplicates**: 3 → 1 (eliminate duplicates)
- **Conversion Patterns**: ~50 patterns → <5 (reduce by 90%)
- **Import Cycle Errors**: 0 (maintain clean imports)

### Qualitative Goals  
- **Foundation Complete**: Other workstreams can build on unified interfaces
- **Type Safety**: Direct typed interfaces replace interface{} patterns
- **Maintainability**: Single source of truth for all interfaces
- **Performance**: Reduced type assertions and conversions

## 📚 Reference Materials

- **Main Analysis**: `/home/tng/workspace/container-kit/ARCHITECTURE_VIOLATIONS_ANALYSIS.md`
- **Architecture Goals**: `/home/tng/workspace/container-kit/plan.md` (lines 125-141)
- **Cleanup Plan**: `/home/tng/workspace/container-kit/MCP_ARCHITECTURE_CLEANUP_PLAN.md`
- **CLAUDE.md**: Project instructions and build commands

---

**Remember**: You are the **foundation workstream**. Your success enables the other workstreams to complete their objectives. Focus on clean, unified interfaces and direct typing patterns. Good luck! 🚀