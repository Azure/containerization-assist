# MCP Module Reorganization Plan

## Executive Summary

The current MCP module has grown organically to **343 Go files across 62 directories** with **106K+ lines of code**. This analysis reveals significant architectural debt that impacts maintainability, performance, and developer productivity.

## Critical Issues Identified

### 1. **Interface Explosion** 🚨
- **11 different `interfaces.go` files** with overlapping responsibilities
- **5+ tool interfaces** that define nearly identical contracts:
  - `dispatch.Tool`
  - `tools.SimpleTool` 
  - `tools.AtomicTool`
  - `base.AtomicTool`
  - `utils.Tool`

### 2. **Excessive Nesting** 📁
- Directory depth up to **5 levels**: `/internal/orchestration/dispatch/generated/adapters/`
- **150+ files** crammed into `/internal/tools/` package
- **4 separate testutil packages** scattered across modules

### 3. **Code Duplication** 🔄
- **24 nearly identical adapter files** (auto-generated boilerplate)
- **7 files named `types.go`** with similar structures
- **7 files named `common.go`** indicating scattered utilities

### 4. **Tight Coupling** 🔗
- Circular dependency risks between tools ↔ orchestration
- Session management spread across **3 different packages**
- Business logic leaking into transport layer

## Recommended Reorganization

### Phase 1: Interface Consolidation (High Impact, Low Risk)

**Current Problem:**
```go
// 11 different interface files defining similar contracts
dispatch/interfaces.go: Tool interface
tools/interfaces.go: SimpleTool interface  
base/atomic_tool.go: AtomicTool interface
// ... 8 more
```

**Solution:**
```go
// pkg/mcp/interfaces.go - Single source of truth
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

type Session interface {
    ID() string
    GetWorkspace() string
    UpdateState(func(*SessionState))
}

type Transport interface {
    Serve(ctx context.Context) error
    Stop() error
}
```

### Phase 2: Package Restructuring (Medium Impact, Medium Risk)

**Current Structure:**
```
pkg/mcp/internal/
├── tools/ (150+ files!)
├── orchestration/dispatch/generated/adapters/ (24 adapters)
├── store/session/
├── types/session/ 
├── engine/conversation/
└── 58 other directories...
```

**Proposed Structure:**
```
pkg/mcp/
├── mcp.go                 # Public API
├── interfaces.go          # Core interfaces
├── internal/
│   ├── server/           # Core server (was engine/)
│   │   ├── server.go
│   │   ├── conversation.go
│   │   └── middleware.go
│   ├── tools/            # Simplified tool system
│   │   ├── registry.go   # Tool registration
│   │   ├── atomic/       # Atomic tools by domain
│   │   │   ├── build.go  # build + tag + push tools
│   │   │   ├── deploy.go # deploy + health tools  
│   │   │   ├── scan.go   # security + secrets tools
│   │   │   └── analyze.go # analyze + validate tools
│   │   └── base/         # Common tool functionality
│   ├── session/          # Unified session management
│   │   ├── manager.go
│   │   ├── state.go
│   │   └── store.go
│   ├── transport/        # Transport implementations
│   │   ├── stdio.go
│   │   ├── tcp.go
│   │   └── websocket.go
│   ├── workflow/         # Simplified orchestration
│   │   ├── executor.go
│   │   └── stages.go
│   └── shared/           # Common utilities
│       ├── constants.go
│       ├── errors.go
│       ├── validation.go
│       └── testutil.go
```

### Phase 3: Eliminate Generated Code Overhead

**Current Problem:**
- **24 auto-generated adapter files** with identical patterns
- Unnecessary abstraction layer adding complexity
- Maintenance burden for generated code

**Solution:**
```go
// Direct tool registration instead of adapters
type ToolRegistry struct {
    tools map[string]Tool
}

func (r *ToolRegistry) Register(name string, tool Tool) {
    r.tools[name] = tool
}

func (r *ToolRegistry) Execute(name string, ctx context.Context, args interface{}) (interface{}, error) {
    tool, exists := r.tools[name]
    if !exists {
        return nil, fmt.Errorf("tool not found: %s", name)
    }
    return tool.Execute(ctx, args)
}
```

### Phase 4: Tool Domain Consolidation

**Current:** 11 separate atomic tool files
**Proposed:** 4 domain-grouped files

```go
// internal/tools/atomic/build.go
type BuildTools struct {
    BuildImage    *BuildImageTool
    TagImage      *TagImageTool  
    PushImage     *PushImageTool
    PullImage     *PullImageTool
}

// internal/tools/atomic/deploy.go  
type DeployTools struct {
    Deploy        *DeployTool
    GenerateManifests *ManifestsTool
    CheckHealth   *HealthTool
}

// internal/tools/atomic/scan.go
type SecurityTools struct {
    ScanImageSecurity *SecurityScanTool
    ScanSecrets      *SecretsScanTool
}

// internal/tools/atomic/analyze.go
type AnalysisTools struct {
    AnalyzeRepository   *AnalysisTool
    ValidateDockerfile  *ValidationTool
    GenerateDockerfile  *DockerfileGenTool
}
```

## Migration Strategy

### Step 1: Interface Unification (Week 1)
1. Create unified `pkg/mcp/interfaces.go`
2. Update all packages to use unified interfaces
3. Remove redundant interface files
4. **Risk:** Low - mostly find/replace operations

### Step 2: Package Consolidation (Week 2)
1. Move session management to single package
2. Consolidate testutil packages
3. Flatten excessive directory nesting
4. **Risk:** Medium - requires import path updates

### Step 3: Tool Simplification (Week 3) 
1. Eliminate adapter layer
2. Implement direct tool registration
3. Group related tools by domain
4. **Risk:** Medium - changes tool loading mechanism

### Step 4: Cleanup and Validation (Week 4)
1. Remove dead code and unused files
2. Update documentation
3. Performance testing
4. **Risk:** Low - cleanup activities

## Expected Benefits

### Quantified Improvements
- **🗂️ File Reduction:** 343 → ~80 files (-75%)
- **📁 Directory Reduction:** 62 → ~15 directories (-75%)  
- **📄 Interface Consolidation:** 11 → 1 interface file (-90%)
- **🔧 Tool Files:** 11 atomic tools → 4 domain files (-65%)

### Developer Experience
- **📖 Easier Navigation:** Clear package hierarchy
- **🚀 Faster Builds:** Reduced compilation complexity
- **🧪 Simpler Testing:** Unified test utilities
- **🔍 Better IDE Support:** Cleaner import paths

### Maintenance Benefits
- **🔄 Reduced Duplication:** Centralized common functionality
- **🔗 Loose Coupling:** Clear module boundaries
- **📏 Consistent Patterns:** Standardized interfaces
- **🛡️ Lower Bug Risk:** Simplified dependencies

## Risk Mitigation

### High-Risk Areas
1. **Import Path Changes:** Use automated refactoring tools
2. **Interface Changes:** Gradual migration with compatibility layers
3. **Tool Loading:** Comprehensive testing of new registration system

### Rollback Strategy
1. Keep feature branches for each phase
2. Comprehensive test coverage before changes
3. Automated integration tests
4. Performance benchmarks at each step

## Success Metrics

### Code Quality
- [ ] Cyclomatic complexity reduction by 30%
- [ ] Test coverage maintained at 70%+
- [ ] Zero circular dependencies
- [ ] Lint violations under 10 total

### Performance  
- [ ] Build time reduction by 25%
- [ ] Binary size reduction by 15%
- [ ] Memory usage reduction by 10%

### Maintainability
- [ ] New developer onboarding time reduced by 50%
- [ ] Code review time reduced by 40% 
- [ ] Bug fix time reduced by 35%

This reorganization will transform the MCP module from a complex, tightly-coupled system into a clean, maintainable architecture following Go best practices while preserving all existing functionality.