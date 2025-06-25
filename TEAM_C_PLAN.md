# Team C: Tool System Rewrite Plan

## Team Information
- **Team**: C - Tool System Rewrite  
- **Members**: 2 Developers
- **Timeline**: Weeks 2-3 (after Team A completes interfaces)
- **Domain**: Complete tool system overhaul with auto-registration

## Dependencies & Execution Status

### LATEST UPDATE (After Team A Completion):
- **STATUS**: ✅ UNBLOCKED - Team A completed their initial work
- **NEW CRITICAL TASK**: Fix non-functional fixer module integration (added to our workload)
- **COMPLETED TASKS**:
  - ✅ Deleted all 11 adapter files (-3,763 lines of code)
  - ✅ Removed all progress adapter usages from tools
  - ✅ Created direct PipelineOperations implementation
  - ✅ All tests passing (go build, go vet, go test)
- **IN PROGRESS**: Week 2 remaining tasks (auto-registration, zero-code approach)

### Previous Blocking Status:
- **DEPENDENCY**: Team A must complete unified interface creation (`pkg/mcp/interfaces.go`)
- **PREVIOUS STATUS**: ⏳ WAITING - Team A has not yet completed interface unification

## Detailed Blocking Analysis
**Interface Validation Results (50 errors):**
- Missing unified interfaces file: `pkg/mcp/interfaces.go`
- Legacy interface files to be removed: 8 files
- Tool implementations missing methods: 34 structs
- Duplicate interface definitions: 10 interfaces

**~~Current~~ Deleted Adapter Files (11 files):** ✅ **ALL DELETED**
- ~~`/internal/adapter/mcp/adapters.go`~~ **DELETED**
- ~~`/internal/adapter/mcp/pipeline_adapter.go`~~ **DELETED**
- ~~`/internal/tools/analysis_adapter.go`~~ **DELETED**
- ~~`/internal/tools/dockerfile_adapter.go`~~ **DELETED**
- ~~`/internal/tools/generate_dockerfile_adapter.go`~~ **DELETED**
- ~~`/internal/tools/gomcp_progress_adapter.go`~~ **DELETED**
- ~~`/internal/tools/manifests_adapter.go`~~ **DELETED**
- ~~`/internal/tools/security_adapter.go`~~ **DELETED**
- ~~`/internal/tools/base/adapter.go`~~ **DELETED**
- ~~`/internal/orchestration/dispatch/example_tool_adapter.go`~~ **DELETED**
- ~~`/internal/engine/conversation/adapters.go`~~ **DELETED**

## Tasks Overview

### Week 2: Auto-Registration System
**Priority Tasks:**
1. **Delete all 24 generated adapters** (complete removal)
   - Remove `internal/orchestration/dispatch/generated/adapters/`
   - Delete adapter generation scripts
   - Remove adapter interfaces

2. **Implement auto-registration with //go:generate**
   - Auto-discovery via build-time codegen
   - Zero-code registration approach
   - Use unified interface from Team A

3. **Replace generated adapters with zero-code approach**
   - Use generics + build-time registration
   - Eliminate 24 boilerplate files

### Week 3: Domain Consolidation with Sub-packages
**Priority Tasks:**
1. **Split into sub-packages instead of mega-files**
   - `internal/build/`: Individual files per tool
   - `internal/deploy/`: Split deployment tools  
   - `internal/scan/`: Security tools
   - `internal/analyze/`: Analysis tools

2. **Fix error handling**
   - Replace `fmt.Errorf` with `types.NewRichError`
   - Remove "not yet implemented" TODO stubs
   - Use proper error types throughout

3. **Standardize all tools** with unified patterns
   - Every tool follows exact same pattern
   - Consistent method signatures
   - Proper validation methods

## Detailed Implementation Plan

### Week 2 Implementation

#### Task 1: Delete Generated Adapters
```bash
# Remove adapter directory
rm -rf internal/orchestration/dispatch/generated/adapters/

# Delete generation scripts
find . -name "*generate*adapter*" -delete

# Remove adapter interfaces
# (Will be handled after Team A completes)
```

#### Task 2: Auto-Registration System
```go
// Auto-discovery via build-time codegen
//go:generate go run tools/register_tools.go

// Zero-code registration approach
type ToolRegistry struct {
    tools map[string]Tool  // Uses unified interface from Team A
}

// Auto-generated registration (replaces manual maps)
func init() {
    // Generated at build time
    RegisterTool("build_image", &BuildImageTool{})
    RegisterTool("deploy_kubernetes", &DeployKubernetesTool{})
    // ... all tools auto-registered
}
```

#### Task 3: Zero-Code Approach
```go
// Use generics + build-time registration instead of 24 boilerplate files
func RegisterTool[T Tool](name string, tool T) {
    registry.tools[name] = tool
}
```

### Week 3 Implementation

#### Task 1: Sub-package Structure
```
internal/build/               # Individual files per tool
├── build_image.go
├── tag_image.go 
├── push_image.go
└── pull_image.go

internal/deploy/              # Split deployment tools
├── deploy_kubernetes.go
├── generate_manifests.go
└── check_health.go

internal/scan/                # Security tools
├── scan_image_security.go
└── scan_secrets.go

internal/analyze/             # Analysis tools
├── analyze_repository.go
├── validate_dockerfile.go
└── generate_dockerfile.go
```

#### Task 2: Error Handling Fixes
```go
// Replace fmt.Errorf with project's types.NewRichError
// Remove "not yet implemented" TODO stubs
// Use proper error types throughout
```

#### Task 3: Standardized Tool Pattern
```go
// Every tool follows this exact pattern
type BuildImageTool struct { /* ... */ }

func (t *BuildImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) { /* ... */ }
func (t *BuildImageTool) GetMetadata() ToolMetadata { /* ... */ }
func (t *BuildImageTool) Validate(ctx context.Context, args interface{}) error { /* ... */ }
```

## Quality Gates
- Run `go build && go vet && go test ./...` after each major task
- Git commit clean changes after each task completion
- Ensure no breaking changes to external APIs
- Maintain test coverage throughout refactoring

## Risk Assessment
- **Risk Level**: Medium - complete rewrite of tool system but with proven patterns
- **Dependencies**: Team A (unified interfaces) - CRITICAL BLOCKER
- **Blocks**: None
- **Mitigation**: Comprehensive testing and gradual migration approach

## Success Criteria
- ✅ All 24 generated adapters removed
- ✅ Auto-registration system working
- ✅ Clean sub-package structure implemented
- ✅ Proper error handling throughout
- ✅ All tools standardized with unified interface
- ✅ Build/test/vet passes after each task
- ✅ Clean git commits documenting progress

## Next Actions
1. **WAIT** for Team A to complete `pkg/mcp/interfaces.go` creation
2. **WAIT** for Team A to update all tool implementations to new interface
3. Once Team A completes: Begin Week 2 tasks immediately
4. Execute tasks systematically with testing and commits after each