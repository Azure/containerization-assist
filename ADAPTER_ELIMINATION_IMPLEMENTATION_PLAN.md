# Adapter Elimination Implementation Plan

## Executive Summary

This plan provides a detailed, step-by-step approach to eliminate 1,474 lines of adapter code across 11 files while achieving true interface unification. The strategy focuses on replacing adapter patterns with proper dependency injection and interface standardization.

**Target Elimination**: 11 adapter files totaling 1,474 lines of bridging code
**Timeline**: 14 days across 6 phases
**Core Strategy**: Replace adapters with standardized interfaces and dependency injection

## Current State Analysis

### Adapter Inventory by Priority

| Priority | Adapter File | Lines | Dependencies | Elimination Strategy |
|----------|-------------|-------|--------------|---------------------|
| **HIGH** | repository_analyzer_adapter.go | 357 | 2 files | Interface unification |
| **HIGH** | auto_registration_adapter.go | 176 | 2 files | Migration completion |
| **MODERATE** | Progress adapters (3 files) | 370 | 12 files | Interface standardization |
| **MODERATE** | Operation wrappers (3 files) | 287 | 6 files | Logic consolidation |
| **MODERATE** | transport_adapter.go | 73 | 4 files | Transport standardization |
| **EASY** | dockerfile_adapter.go | 40 | 1 file | Direct integration |

### Root Cause Analysis

**Why Adapters Were Created:**
1. **Import Cycles**: Prevented direct interface usage
2. **Interface Fragmentation**: Multiple similar interfaces in different packages
3. **Migration Debt**: Temporary bridges during interface transitions
4. **Architecture Mismatch**: Different patterns (factory vs instance, sync vs async)

**Why They Must Be Eliminated:**
1. **Maintenance Burden**: 1,474 lines of bridging code to maintain
2. **Performance Overhead**: Double function calls and type conversions
3. **Testing Complexity**: Need to test both original and adapted interfaces
4. **Architecture Obscurity**: Hides true dependencies and relationships

## Phase 1: Foundation - Interface Standardization (Days 1-3)

### Step 1.1: Create Core Interface Package

**Objective**: Establish single source of truth for all interfaces

**Action**: Create `pkg/mcp/core/interfaces.go`

```go
// pkg/mcp/core/interfaces.go
package core

import "context"

// Core Tool Interface - Single definition used throughout system
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}

// Progress Interface - Unified progress reporting
type ProgressReporter interface {
    StartStage(stage string) ProgressToken
    UpdateProgress(token ProgressToken, message string, percent int)
    CompleteStage(token ProgressToken, success bool, message string)
}

// Repository Analysis Interface - Breaks import cycle
type RepositoryAnalyzer interface {
    AnalyzeStructure(ctx context.Context, path string) (*RepositoryInfo, error)
    AnalyzeDockerfile(ctx context.Context, path string) (*DockerfileInfo, error)
    GetBuildRecommendations(ctx context.Context, repo *RepositoryInfo) (*BuildRecommendations, error)
}

// Transport Interface - Single transport abstraction
type Transport interface {
    Serve(ctx context.Context) error
    Stop(ctx context.Context) error
    SetHandler(handler RequestHandler)
    Name() string
}

// Request Handler Interface - Unified request handling
type RequestHandler interface {
    HandleRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error)
}

// Tool Registry Interface - Simplified registration
type ToolRegistry interface {
    Register(tool Tool)
    Get(name string) (Tool, bool)
    List() []string
}
```

**Dependencies**: Import only standard library and essential types
**Migration Strategy**: Packages import from core, not each other

### Step 1.2: Migrate Interface Definitions

**Move from `pkg/mcp/interfaces.go` to `pkg/mcp/core/interfaces.go`:**
- Core interface definitions
- Essential type definitions
- Remove duplicated types

**Update `pkg/mcp/interfaces.go`:**
- Re-export core interfaces for backward compatibility
- Add deprecation notices
- Maintain API surface during transition

```go
// pkg/mcp/interfaces.go
package mcp

import "github.com/Azure/container-kit/pkg/mcp/core"

// Deprecated: Use core.Tool instead
type Tool = core.Tool

// Deprecated: Use core.ProgressReporter instead  
type ProgressReporter = core.ProgressReporter

// ... other re-exports
```

### Step 1.3: Update Internal Packages

**Target Packages**: `internal/analyze`, `internal/build`, `internal/deploy`, `internal/scan`

**Action**: Update imports to use core interfaces
```go
// Before
import "github.com/Azure/container-kit/pkg/mcp"

// After  
import "github.com/Azure/container-kit/pkg/mcp/core"
```

**Validation**: 
```bash
# Verify no import cycles
go build -tags mcp ./pkg/mcp/core/...
go build -tags mcp ./pkg/mcp/internal/...
```

## Phase 2: High Priority Adapter Elimination (Days 4-7)

### Step 2.1: Eliminate Repository Analyzer Adapter (Day 4)

**Target**: `pkg/mcp/internal/orchestration/repository_analyzer_adapter.go` (357 lines)

**Current Problem**: 
- `analyze` package can't import `build` package (import cycle)
- `build` package needs repository analysis functionality
- Adapter bridges with 200+ lines of conversion logic

**Solution Architecture**:
```go
// pkg/mcp/core/interfaces.go - Add to existing file
type RepositoryAnalyzer interface {
    AnalyzeStructure(ctx context.Context, path string) (*RepositoryInfo, error)
    AnalyzeDockerfile(ctx context.Context, path string) (*DockerfileInfo, error)
    GetBuildRecommendations(ctx context.Context, repo *RepositoryInfo) (*BuildRecommendations, error)
}

// Move shared types to core
type RepositoryInfo struct {
    Path           string            `json:"path"`
    Type           string            `json:"type"`
    Languages      []string          `json:"languages"`
    Dependencies   map[string]string `json:"dependencies"`
    BuildTools     []string          `json:"build_tools"`
    HasDockerfile  bool              `json:"has_dockerfile"`
    Metadata       map[string]interface{} `json:"metadata"`
}
```

**Implementation Steps**:

1. **Move shared types to core package**:
   ```bash
   # Extract RepositoryInfo, DockerfileInfo, BuildRecommendations
   # Move from adapter file to pkg/mcp/core/types.go
   ```

2. **Update analyze package to implement core interface**:
   ```go
   // pkg/mcp/internal/analyze/repository_analyzer.go
   package analyze
   
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type repositoryAnalyzer struct {
       logger zerolog.Logger
   }
   
   func NewRepositoryAnalyzer() core.RepositoryAnalyzer {
       return &repositoryAnalyzer{
           logger: zerolog.Logger{},
       }
   }
   
   func (r *repositoryAnalyzer) AnalyzeStructure(ctx context.Context, path string) (*core.RepositoryInfo, error) {
       // Direct implementation - no conversion needed
   }
   ```

3. **Update build package to use core interface**:
   ```go
   // pkg/mcp/internal/build/analyzer_integration.go
   package build
   
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type EnhancedBuildAnalyzer struct {
       repositoryAnalyzer core.RepositoryAnalyzer // Direct dependency injection
   }
   
   func NewEnhancedBuildAnalyzer(analyzer core.RepositoryAnalyzer) *EnhancedBuildAnalyzer {
       return &EnhancedBuildAnalyzer{
           repositoryAnalyzer: analyzer,
       }
   }
   ```

4. **Update dependency injection in main**:
   ```go
   // Inject analyzer into build tools during initialization
   repoAnalyzer := analyze.NewRepositoryAnalyzer()
   buildAnalyzer := build.NewEnhancedBuildAnalyzer(repoAnalyzer)
   ```

5. **Remove adapter file**:
   ```bash
   rm pkg/mcp/internal/orchestration/repository_analyzer_adapter.go
   ```

**Expected Reduction**: -357 lines, -1 adapter file

### Step 2.2: Eliminate Auto Registration Adapter (Day 5)

**Target**: `pkg/mcp/internal/runtime/auto_registration_adapter.go` (176 lines)

**Current Problem**:
- Bridges between old tool registration and unified interface
- Uses `interface{}` and complex type assertions
- Temporary migration adapter that should be removed

**Solution**: Complete tool migration to unified interface

**Implementation Steps**:

1. **Audit remaining tools not using unified interface**:
   ```bash
   # Find tools still using old patterns
   grep -r "runtime\.UnifiedTool" pkg/mcp/internal/ | wc -l
   grep -r "mcptypes\.Tool" pkg/mcp/internal/ | wc -l
   ```

2. **Update remaining tools to implement core.Tool directly**:
   ```go
   // Example: Update a tool still using old pattern
   // Before (old pattern)
   type SomeTool struct{}
   
   func (t *SomeTool) Execute(args interface{}) (interface{}, error)
   
   // After (unified pattern)
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type SomeTool struct{}
   
   func (t *SomeTool) Execute(ctx context.Context, args interface{}) (interface{}, error)
   func (t *SomeTool) GetMetadata() core.ToolMetadata
   func (t *SomeTool) Validate(ctx context.Context, args interface{}) error
   ```

3. **Update tool registration to use core interfaces directly**:
   ```go
   // pkg/mcp/internal/runtime/registry.go
   package runtime
   
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type Registry struct {
       tools map[string]core.Tool
   }
   
   func (r *Registry) Register(tool core.Tool) {
       metadata := tool.GetMetadata()
       r.tools[metadata.Name] = tool
   }
   
   func (r *Registry) Get(name string) (core.Tool, bool) {
       tool, exists := r.tools[name]
       return tool, exists
   }
   ```

4. **Remove auto registration adapter**:
   ```bash
   rm pkg/mcp/internal/runtime/auto_registration_adapter.go
   ```

5. **Update imports and remove `OrchestratorRegistryAdapter`**:
   - Remove adapter-specific imports
   - Update tool initialization to use direct registration

**Expected Reduction**: -176 lines, -1 adapter file

## Phase 3: Progress Adapter Consolidation (Days 8-9)

### Step 3.1: Standardize Progress Interface (Day 8)

**Target**: 3 progress adapter files (370 lines total)

**Current Problem**:
- 3 different progress implementations doing similar work
- Inconsistent progress reporting across tools
- Complex adapter chains for GoMCP integration

**Solution**: Single progress interface in core package

**Implementation Steps**:

1. **Define unified progress interface in core**:
   ```go
   // pkg/mcp/core/interfaces.go - Add to existing
   type ProgressReporter interface {
       StartStage(stage string) ProgressToken
       UpdateProgress(token ProgressToken, message string, percent int)
       CompleteStage(token ProgressToken, success bool, message string)
   }
   
   type ProgressToken string
   
   type ProgressStage struct {
       Name        string `json:"name"`
       Description string `json:"description"`
       Status      string `json:"status"` // "pending", "running", "completed", "failed"
       Progress    int    `json:"progress"` // 0-100
       Message     string `json:"message"`
   }
   ```

2. **Create single progress implementation**:
   ```go
   // pkg/mcp/internal/observability/progress.go
   package observability
   
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type ProgressReporter struct {
       serverCtx *server.Context // GoMCP integration
       stages    map[core.ProgressToken]*core.ProgressStage
   }
   
   func NewProgressReporter(serverCtx *server.Context) core.ProgressReporter {
       return &ProgressReporter{
           serverCtx: serverCtx,
           stages:    make(map[core.ProgressToken]*core.ProgressStage),
       }
   }
   
   func (p *ProgressReporter) StartStage(stage string) core.ProgressToken {
       token := core.ProgressToken(fmt.Sprintf("stage_%d", time.Now().UnixNano()))
       p.stages[token] = &core.ProgressStage{
           Name:   stage,
           Status: "running",
       }
       
       // Direct GoMCP integration - no adapter needed
       if p.serverCtx != nil {
           p.serverCtx.NotifyProgress(string(token), &server.Progress{
               Token: string(token),
               Title: stage,
           })
       }
       
       return token
   }
   ```

3. **Update all atomic tools to use unified progress**:
   ```go
   // Example: Update atomic tool
   // Before (with adapter)
   progress := NewGoMCPProgressAdapter(serverCtx, "build_image")
   
   // After (direct interface)
   progress := observability.NewProgressReporter(serverCtx)
   token := progress.StartStage("build_image")
   progress.UpdateProgress(token, "Building image...", 50)
   progress.CompleteStage(token, true, "Image built successfully")
   ```

4. **Remove all 3 progress adapter files**:
   ```bash
   rm pkg/mcp/types/progress_adapter.go
   rm pkg/mcp/internal/gomcp_progress_adapter.go
   rm pkg/mcp/internal/runtime/gomcp_progress_adapter.go
   ```

**Expected Reduction**: -370 lines, -3 adapter files

### Step 3.2: Update Tool Implementations (Day 9)

**Update all tools using progress adapters** (12 files identified):

1. **Batch update imports**:
   ```bash
   # Replace progress adapter imports with core interface
   find pkg/mcp/internal -name "*.go" -exec sed -i \
     's|progress_adapter|core|g; s|GoMCPProgressAdapter|ProgressReporter|g' {} \;
   ```

2. **Update tool constructors to accept progress interface**:
   ```go
   // Example tool update
   type AtomicBuildTool struct {
       progress core.ProgressReporter
   }
   
   func NewAtomicBuildTool(progress core.ProgressReporter) *AtomicBuildTool {
       return &AtomicBuildTool{
           progress: progress,
       }
   }
   ```

3. **Test progress integration**:
   ```bash
   # Verify progress reporting works
   go test -tags mcp ./pkg/mcp/internal/*/...
   ```

## Phase 4: Operation Wrapper Consolidation (Days 10-11)

### Step 4.1: Create Generic Operation Wrapper (Day 10)

**Target**: 3 operation wrapper files (287 lines total)

**Current Problem**:
- Duplicate wrapper logic for pull/push/tag operations
- Each wrapper implements similar retry and error handling
- No code reuse between operation types

**Solution**: Single configurable operation wrapper

**Implementation Steps**:

1. **Create generic operation wrapper**:
   ```go
   // pkg/mcp/internal/build/docker_operation.go
   package build
   
   import "github.com/Azure/container-kit/pkg/mcp/core"
   
   type OperationType string
   
   const (
       OperationPull OperationType = "pull"
       OperationPush OperationType = "push"
       OperationTag  OperationType = "tag"
   )
   
   type DockerOperation struct {
       Type         OperationType
       RetryAttempts int
       Timeout      time.Duration
       Progress     core.ProgressReporter
       
       // Operation-specific functions
       Execute  func(ctx context.Context) error
       Analyze  func() error
       Prepare  func() error
       Validate func() error
   }
   
   func (op *DockerOperation) Run(ctx context.Context) error {
       token := op.Progress.StartStage(string(op.Type))
       
       // Common retry logic
       for attempt := 0; attempt < op.RetryAttempts; attempt++ {
           if err := op.executeWithRetry(ctx, attempt); err == nil {
               op.Progress.CompleteStage(token, true, "Operation completed")
               return nil
           }
       }
       
       op.Progress.CompleteStage(token, false, "Operation failed after retries")
       return fmt.Errorf("operation failed after %d attempts", op.RetryAttempts)
   }
   ```

2. **Update atomic tools to use generic wrapper**:
   ```go
   // pkg/mcp/internal/build/pull_image_atomic.go
   func (t *AtomicPullImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
       pullArgs := args.(*PullImageArgs)
       
       operation := &DockerOperation{
           Type:         OperationPull,
           RetryAttempts: 3,
           Timeout:      5 * time.Minute,
           Progress:     t.progress,
           Execute: func(ctx context.Context) error {
               return t.dockerClient.ImagePull(ctx, pullArgs.ImageRef, types.ImagePullOptions{})
           },
           Validate: func() error {
               return validateImageRef(pullArgs.ImageRef)
           },
       }
       
       return operation.Run(ctx)
   }
   ```

3. **Remove individual wrapper files**:
   ```bash
   rm pkg/mcp/internal/build/pull_operation_wrapper.go
   rm pkg/mcp/internal/build/push_operation_wrapper.go  
   rm pkg/mcp/internal/build/tag_operation_wrapper.go
   ```

**Expected Reduction**: -287 lines, -3 files (consolidated to 1 file ~100 lines)

## Phase 5: Transport and Minor Adapters (Days 12-13)

### Step 5.1: Eliminate Transport Adapter (Day 12)

**Target**: `pkg/mcp/internal/core/transport_adapter.go` (73 lines)

**Solution**: Standardize on single transport interface

**Implementation Steps**:

1. **Update transport implementations to use core interface directly**:
   ```go
   // Update stdio transport to implement core.Transport
   // Update HTTP transport to implement core.Transport
   ```

2. **Remove transport adapter**:
   ```bash
   rm pkg/mcp/internal/core/transport_adapter.go
   ```

3. **Update server to use transports directly**:
   ```go
   // No adapter needed - direct interface usage
   server.SetTransport(stdioTransport)
   ```

### Step 5.2: Eliminate Dockerfile Adapter (Day 12)

**Target**: `pkg/mcp/internal/analyze/dockerfile_adapter.go` (40 lines)

**Solution**: Direct integration of validation logic

**Implementation Steps**:

1. **Move validation logic directly to validate_dockerfile_atomic.go**
2. **Remove adapter file**
3. **Update imports**

**Expected Reduction**: -113 lines, -2 adapter files

## Phase 6: Validation and Integration (Day 14)

### Step 6.1: Comprehensive Testing

**Validation Commands**:
```bash
# Build verification
go build -tags mcp ./pkg/mcp/...

# Test suite
make test-mcp

# Adapter elimination verification
find pkg/mcp -name "*adapter*.go" | wc -l  # Target: 0
find pkg/mcp -name "*wrapper*.go" | grep -v docker_operation | wc -l  # Target: 0

# Interface unification verification
grep -r "type.*Tool.*interface" pkg/mcp/ | wc -l  # Target: 1
```

### Step 6.2: Performance Validation

**Before/After Metrics**:
- Lines of code reduction: -1,474 lines
- File count reduction: -10 files
- Interface definitions: 1 (from multiple)
- Import cycle count: 0

### Step 6.3: Documentation Update

**Update Architecture Documentation**:
- Document new core interface package
- Update dependency injection patterns
- Remove adapter pattern references

## Migration Timeline

| Phase | Days | Primary Tasks | Expected Reduction |
|-------|------|---------------|-------------------|
| **Phase 1** | 1-3 | Interface standardization | Foundation work |
| **Phase 2** | 4-7 | High priority adapters | -533 lines, -2 files |
| **Phase 3** | 8-9 | Progress consolidation | -370 lines, -3 files |
| **Phase 4** | 10-11 | Operation consolidation | -287 lines, -3 files |
| **Phase 5** | 12-13 | Transport & minor adapters | -113 lines, -2 files |
| **Phase 6** | 14 | Validation & integration | Testing & docs |

**Total Expected Reduction**: -1,303 lines (net after new core interfaces), -10 files

## Risk Mitigation

### Rollback Strategy
1. **Git branches**: Create feature branch for each phase
2. **Incremental commits**: Commit after each step
3. **Tagged releases**: Tag working states for rollback points

### Testing Strategy
1. **Unit tests**: Maintain existing test coverage
2. **Integration tests**: Verify tool execution
3. **Build verification**: Continuous build testing

### Dependency Management
1. **Gradual migration**: Update packages incrementally
2. **Backward compatibility**: Maintain re-exports during transition
3. **Clear deprecation**: Mark old interfaces as deprecated

## Success Criteria

### Functional Requirements
- [ ] All tools execute without errors
- [ ] MCP server starts and handles requests
- [ ] Progress reporting works correctly
- [ ] Docker operations succeed with retry logic

### Architectural Requirements  
- [ ] Zero adapter files in codebase
- [ ] Single Tool interface definition
- [ ] No import cycles between packages
- [ ] Core interfaces provide single source of truth

### Quality Requirements
- [ ] Test coverage maintained at 70%+
- [ ] Build time improved by 15%+
- [ ] Linting errors reduced to <50
- [ ] Documentation reflects new architecture

This plan eliminates the complex adapter patterns while maintaining all functionality and improving the overall architecture through proper interface design and dependency injection.