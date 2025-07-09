# WORKSTREAM ALPHA: Struct Field & Type Definitions Implementation Guide

## ⚠️ CRITICAL IMPLEMENTATION RULES

1. **INCREMENTAL CHANGES ONLY**: Make ONE small change at a time, validate it compiles, then proceed
2. **NO PLACEHOLDERS/STUBS**: Every field, type, and method must be fully functional - no TODOs, no empty implementations
3. **VALIDATE AFTER EACH CHANGE**: Run `go build` after every single modification
4. **FOLLOW ADRs**: Respect all architectural decisions in docs/architecture/adr/
5. **SIMPLIFICATION GOAL**: We're simplifying the codebase - don't add complexity

### Validation Command After EVERY Change:
```bash
# After each file edit, immediately run:
go build ./pkg/mcp/... 2>&1 | grep -E "(error|undefined|redeclared)" | wc -l
# This number should never increase!
```

## 🎯 Mission
Fix all struct field compilation errors by adding missing fields and creating the foundational shared types package that other workstreams will depend on.

## 📋 Context
- **Project**: Container Kit - Three-layer architecture pre-commit fixes
- **Your Role**: Foundation layer - providing core types for the entire system
- **Timeline**: Week 1, Days 1-2 (2 days)
- **Dependencies**: None - you are the foundation
- **Deliverables**: ValidationError.Code field, AnalysisMetadata.Options field, domain/shared package with base types

## 🎯 Success Metrics
- Struct field errors: 10 → 0
- ValidationError usages: All updated with Code field
- domain/shared package: Created and compiling
- BaseToolArgs/Response: Available for other workstreams

## 📁 File Ownership
You have exclusive ownership of these files/directories:
```
pkg/mcp/domain/errors/validation_error.go
pkg/mcp/domain/errors/factories.go (if it exists)
pkg/mcp/domain/shared/** (create this)
pkg/mcp/domain/containerization/analyze/types.go (AnalysisMetadata only)
pkg/mcp/domain/containerization/analyze/common.go (if needed)
```

Shared files requiring coordination:
```
pkg/mcp/application/commands/analyze_consolidated.go (read-only until your changes merged)
```

## 🗓️ Implementation Schedule

### Day 1: Core Struct Field Fixes

#### Morning Goals (4 hours):
**Task: Fix ValidationError Missing Code Field**
- [ ] Read pkg/mcp/application/commands/analyze_consolidated.go to understand Code field usage
- [ ] Locate ValidationError definition
- [ ] Add Code field (likely string type based on usage)
- [ ] Update any factory functions or constructors
- [ ] Test compilation of errors package

**Files to modify**:
```go
// pkg/mcp/domain/errors/validation_error.go
type ValidationError struct {
    // ... existing fields ...
    Code string `json:"code,omitempty"` // Add this field
}
```

**Validation Commands**:
```bash
# Check current errors
grep -n "unknown field Code" /home/tng/workspace/container-kit/pkg/mcp/application/commands/analyze_consolidated.go

# After fix, validate
go build ./pkg/mcp/domain/errors/...
go build ./pkg/mcp/application/commands/analyze_consolidated.go
```

#### Afternoon Goals (4 hours):
**Task: Fix AnalysisMetadata Missing Options Field**
- [ ] Find AnalysisMetadata struct definition
- [ ] Analyze line 219 of analyze_consolidated.go to understand Options type
- [ ] Add Options field with appropriate type
- [ ] Update any related constructors

**Search Commands**:
```bash
# Find AnalysisMetadata definition
grep -r "type AnalysisMetadata" pkg/mcp/
find pkg/mcp -name "*.go" -exec grep -l "AnalysisMetadata struct" {} \;

# Understand Options usage
sed -n '210,230p' pkg/mcp/application/commands/analyze_consolidated.go
```

**End of Day Checklist**:
- [ ] ValidationError compiles with Code field
- [ ] AnalysisMetadata compiles with Options field
- [ ] No more "unknown field" errors in analyze_consolidated.go
- [ ] Changes committed with descriptive message

### Day 2: Shared Types Package Creation

#### Morning Goals (4 hours):
**Task: Create domain/shared Package**
- [ ] Create pkg/mcp/domain/shared directory
- [ ] Create base_types.go with BaseToolArgs and BaseToolResponse
- [ ] Analyze registry.go to understand required structure
- [ ] Implement types that can be embedded

**Implementation**:
```go
// pkg/mcp/domain/shared/base_types.go
package shared

// BaseToolArgs provides common fields for all tool arguments
type BaseToolArgs struct {
    // Common fields based on registry.go usage
}

// BaseToolResponse provides common fields for all tool responses  
type BaseToolResponse struct {
    // Common fields based on registry.go usage
}
```

**Validation Commands**:
```bash
# Understand usage in registry
grep -A5 -B5 "types.BaseToolArgs" pkg/mcp/application/core/registry.go
grep -A5 -B5 "types.BaseToolResponse" pkg/mcp/application/core/registry.go

# Test compilation
go build ./pkg/mcp/domain/shared/...
```

#### Afternoon Goals (4 hours):
**Task: Integration Testing**
- [ ] Create simple test to verify types work correctly
- [ ] Document the types for other workstreams
- [ ] Prepare clear commit message
- [ ] Coordinate with BETA workstream for handoff

**Test Creation**:
```go
// pkg/mcp/domain/shared/base_types_test.go
package shared_test

import (
    "testing"
    "github.com/Azure/container-kit/pkg/mcp/domain/shared"
)

func TestBaseTypes(t *testing.T) {
    // Verify types can be instantiated and used
}
```

**Final Validation**:
```bash
# Run all domain tests
go test ./pkg/mcp/domain/...

# Verify no struct field errors remain
/usr/bin/make pre-commit 2>&1 | grep "unknown field" | wc -l
# Should output: 0
```

## 🔧 Technical Guidelines

### Required Tools/Setup
- Go 1.21+ 
- Make alias: `alias make='/usr/bin/make'`
- Git configured with proper commit message format

### Coding Standards
- All structs must have json tags
- Fields should include omitempty where appropriate  
- Comments required for exported types
- Follow existing code style in the package

### Testing Requirements
- Each new type must have at least instantiation test
- Compilation is the primary validation
- No reduction in existing test coverage

## 🤝 Coordination Points

### Dependencies FROM Other Workstreams
| Workstream | What You Need | When | Contact |
|------------|---------------|------|---------|
| None | You are the foundation | N/A | N/A |

### Dependencies TO Other Workstreams  
| Workstream | What They Need | When | Format |
|------------|----------------|------|--------|
| BETA | shared.BaseToolArgs/Response types | Day 2 PM | Compiled package |
| GAMMA | ValidationError with Code field | Day 1 PM | Compiled + tested |
| ALL | AnalysisMetadata with Options | Day 1 PM | Compiled + tested |

## 📊 Progress Tracking

### Daily Status Template
```markdown
## WORKSTREAM ALPHA - Day [X] Status

### Completed Today:
- [x] ValidationError.Code field added
- [x] Found AnalysisMetadata in [location]
- [x] Created domain/shared package structure

### Blockers:
- None / [Specific issue if any]

### Metrics:
- Struct field errors fixed: X/10
- Files modified: X
- Packages compiling: domain/errors, domain/shared

### Tomorrow's Focus:
- Complete shared types implementation
- Coordinate handoff with BETA team
```

### Key Commands
```bash
# Morning setup
alias make='/usr/bin/make'
cd /home/tng/workspace/container-kit

# Check starting error count
/usr/bin/make pre-commit 2>&1 | grep "unknown field" | tee alpha_errors_start.txt | wc -l

# Validation after each change
go build ./pkg/mcp/domain/errors/...
go build ./pkg/mcp/domain/shared/...
go build ./pkg/mcp/application/commands/analyze_consolidated.go

# Progress tracking
/usr/bin/make pre-commit 2>&1 | grep "unknown field" | wc -l

# Final validation
go test ./pkg/mcp/domain/...
```

## 🚨 Common Issues & Solutions

### Issue 1: Can't find struct definition
**Symptoms**: grep returns no results for type definition
**Solution**: Try searching with different patterns
```bash
# Broader search
find pkg/mcp -name "*.go" -exec grep -l "AnalysisMetadata" {} \;
# Check if it's in a different package
grep -r "AnalysisMetadata" pkg/mcp --include="*.go" | grep -v "_test"
```

### Issue 2: Type inference unclear
**Symptoms**: Not sure what type Options field should be
**Solution**: Analyze usage context
```bash
# Look at how it's being set
grep -B10 -A10 "Options:" pkg/mcp/application/commands/analyze_consolidated.go
# Check for similar patterns
grep -r "Options" pkg/mcp/domain --include="*.go" | grep "type"
```

### Issue 3: Circular import when adding shared types
**Symptoms**: Import cycle detected
**Solution**: Ensure domain/shared has no imports from other domain packages
- Only use standard library in shared package
- No imports from application or infra layers

## 📞 Escalation Path

1. **Can't locate struct**: Check with team lead for possible renamed/moved types
2. **Type ambiguity**: Post question in daily standup for team input
3. **Breaking changes**: If changes would break many files, coordinate with GAMMA workstream

## ✅ Definition of Done

Your workstream is complete when:
- [x] All "unknown field" errors resolved (0 remaining)
- [x] ValidationError has working Code field
- [x] AnalysisMetadata has working Options field  
- [x] domain/shared package created with BaseToolArgs/Response
- [x] All domain packages compile successfully
- [x] Basic tests pass for new types
- [x] Clear documentation for other workstreams
- [x] Commit merged to main branch

## 📚 Resources

- Three-layer architecture: `docs/architecture/THREE_LAYER_ARCHITECTURE.md`
- Error system ADR: `docs/architecture/adr/2025-01-07-unified-error-system.md`
- Git commit format: Include `🤖 Generated with [Claude Code]`
- Team chat: Post updates with #alpha tag

---

**Remember**: You are laying the foundation for all other workstreams. Your types will be imported across the codebase. Take time to get the structure right - a solid foundation prevents future issues. When in doubt, check existing patterns in the codebase.