# WORKSTREAM BETA: Error System & Generics Implementation
**AI Assistant Prompt - Container Kit MCP Cleanup**

## 🎯 MISSION OVERVIEW

You are the **Error & Type Safety Specialist** responsible for implementing RichError system and strongly-typed generics throughout the Container Kit MCP codebase. Your work eliminates interface{} usage and creates compile-time type safety.

**Duration**: Week 2-3 (10 days)
**Dependencies**: WORKSTREAM ALPHA completion (unified validation)
**Critical Success**: Type-safe system with rich error context

## 📋 YOUR SPECIFIC RESPONSIBILITIES

### Week 2 (Days 6-10): Foundation Implementation

#### Day 6-7: RichError Foundation (CRITICAL)
```bash
# WAIT: Until WORKSTREAM ALPHA Week 1 complete and merged

# Create RichError infrastructure:
mkdir -p pkg/mcp/errors/rich

# File 1: pkg/mcp/errors/rich/types.go
# Create comprehensive RichError struct:
# - ErrorCode, ErrorType, ErrorSeverity enums
# - ErrorContext, ErrorLocation types
# - Stack trace capture functionality
# - Builder pattern for fluent API

# File 2: pkg/mcp/errors/rich/builder.go
# Create error builder with fluent API:
# - NewError() builder entry point
# - Method chaining (Code(), Message(), Type(), etc.)
# - Auto-context capture
# - Suggestion and help URL support

# File 3: pkg/mcp/errors/rich/constructors.go
# Create common error constructors:
# - ValidationError, NetworkError, SecurityError
# - ConfigurationError, ResourceError, etc.
# - Integration with unified validation system

# VALIDATION REQUIRED:
go test ./pkg/mcp/errors/rich/... && echo "✅ RichError foundation complete"
go fmt ./pkg/mcp/errors/...
```

#### Day 8-9: Generics Foundation
```bash
# Create generic tool interfaces:
mkdir -p pkg/mcp/types/tools

# File 1: pkg/mcp/types/tools/generic.go
# Core Tool[TParams, TResult] interface:
# - Tool[TParams, TResult] with Execute method
# - ToolParams and ToolResult constraint interfaces
# - ConfigurableTool, StatefulTool, StreamingTool interfaces

# File 2: pkg/mcp/types/tools/constraints.go
# Type constraint system:
# - Parameter validation constraints
# - Result processing constraints
# - Type safety utilities

# File 3: pkg/mcp/types/tools/registry.go
# Generic registry interfaces:
# - Registry[T, TParams, TResult] interface
# - ToolFactory[T, TParams, TResult] interface
# - Batch operation interfaces

# VALIDATION REQUIRED:
go test ./pkg/mcp/types/tools/... && echo "✅ Generics foundation complete"
```

#### Day 10: Integration Foundation
```bash
# RichError + Generics integration:

# File 1: pkg/mcp/errors/rich/generic.go
# Generic error types:
# - RichError[TContext] for type-safe contexts
# - Tool-specific error types
# - Type-safe error building with context

# File 2: pkg/mcp/types/tools/schema.go
# Schema generation system:
# - Auto-generate schemas from generic types
# - JSON schema generation
# - API documentation generation

# File 3: pkg/mcp/types/tools/migration.go
# Migration utilities:
# - Legacy interface{} to generics conversion
# - Standard error to RichError conversion
# - Backward compatibility layers

# CHECKPOINT VALIDATION:
go test ./pkg/mcp/errors/... ./pkg/mcp/types/tools/...

# COMMIT AND PAUSE:
git add .
git commit -m "feat(errors,generics): implement RichError and generics foundation

- Created comprehensive RichError system with builder pattern
- Implemented generic Tool[TParams, TResult] interfaces
- Added type-safe error handling with context
- Created schema generation from generic types
- Built migration utilities for legacy code

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"

# PAUSE POINT: Wait for external merge before Week 3
```

### Week 3 (Days 11-15): Core Migration & Registry

#### Day 11-12: Build Tools Migration (HIGH PRIORITY)
```bash
# WAIT: Until Week 2 changes merged and WORKSTREAM ALPHA complete

# Create strongly-typed build tools:

# File 1: pkg/mcp/internal/build/types.go
# Define DockerBuildParams:
type DockerBuildParams struct {
    DockerfilePath string            `json:"dockerfile_path" validate:"required,file"`
    ContextPath    string            `json:"context_path" validate:"required,dir"`
    BuildArgs      map[string]string `json:"build_args,omitempty"`
    Tags           []string          `json:"tags,omitempty"`
    NoCache        bool              `json:"no_cache,omitempty"`
    SessionID      string            `json:"session_id,omitempty"`
}

# Define DockerBuildResult:
type DockerBuildResult struct {
    Success     bool          `json:"success"`
    ImageID     string        `json:"image_id,omitempty"`
    ImageSize   int64         `json:"image_size,omitempty"`
    Duration    time.Duration `json:"duration"`
    BuildLog    []string      `json:"build_log,omitempty"`
    CacheHits   int           `json:"cache_hits"`
    CacheMisses int           `json:"cache_misses"`
    SessionID   string        `json:"session_id"`
}

# Migrate these files to use strongly-typed interfaces:
# - pkg/mcp/internal/build/build_image_atomic.go
# - pkg/mcp/internal/build/pull_image_atomic.go
# - pkg/mcp/internal/build/push_image_atomic.go
# - pkg/mcp/internal/build/tag_image_atomic.go

# Add RichError for build failures with context:
# - Dockerfile parsing errors with line numbers
# - Build execution errors with container logs
# - Image operation errors with Docker context

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/build/... && echo "✅ Build tools strongly typed"
```

#### Day 13-14: Deploy & Scan Tools Migration
```bash
# Create strongly-typed deploy tools:

# File 1: pkg/mcp/internal/deploy/types.go
# Define KubernetesDeployParams:
type KubernetesDeployParams struct {
    ManifestPath string                 `json:"manifest_path" validate:"required,file"`
    Namespace    string                 `json:"namespace" validate:"required,k8s-name"`
    Values       map[string]interface{} `json:"values,omitempty"`
    DryRun       bool                   `json:"dry_run,omitempty"`
    Wait         bool                   `json:"wait,omitempty"`
    Timeout      time.Duration          `json:"timeout,omitempty"`
    SessionID    string                 `json:"session_id,omitempty"`
}

# Define KubernetesDeployResult with detailed status
# Define SecurityScanParams and SecurityScanResult

# Migrate these files:
# - pkg/mcp/internal/deploy/deploy_kubernetes_atomic.go
# - pkg/mcp/internal/deploy/generate_manifests_atomic.go
# - pkg/mcp/internal/scan/scan_image_security_atomic.go
# - pkg/mcp/internal/scan/scan_secrets_atomic.go

# Add RichError for deployment and security failures:
# - Kubernetes operation errors with cluster context
# - Security scan errors with vulnerability details
# - Configuration errors with help URLs

# VALIDATION REQUIRED:
go test -short ./pkg/mcp/internal/deploy/... ./pkg/mcp/internal/scan/...
```

#### Day 15: Generic Registry Implementation (CRITICAL FOR OTHER WORKSTREAMS)
```bash
# Create type-safe registry system:

# File 1: pkg/mcp/internal/orchestration/generic_registry.go
# Implement GenericRegistry[T, TParams, TResult]:
type GenericRegistry[T tools.Tool[TParams, TResult], TParams tools.ToolParams, TResult tools.ToolResult] struct {
    tools   map[string]T
    schemas map[string]tools.Schema[TParams, TResult]
    mu      sync.RWMutex
}

func (r *GenericRegistry[T, TParams, TResult]) Register(name string, tool T) error {
    // Type-safe registration with RichError on conflicts
}

func (r *GenericRegistry[T, TParams, TResult]) Execute(name string, params TParams) (TResult, error) {
    // Type-safe execution with RichError for failures
}

# File 2: pkg/mcp/internal/orchestration/specialized_registries.go
# Create specialized registries:
type BuildRegistry = GenericRegistry[BuildTool, BuildParams, BuildResult]
type DeployRegistry = GenericRegistry[DeployTool, DeployParams, DeployResult]
type ScanRegistry = GenericRegistry[ScanTool, ScanParams, ScanResult]

# File 3: pkg/mcp/internal/orchestration/federated_registry.go
# Registry federation for type-safe dispatch

# Update pkg/mcp/internal/orchestration/tool_orchestrator.go:
# - Replace interface{} with strongly-typed generics
# - Add RichError for all failure cases
# - Integrate with unified validation system

# FINAL VALIDATION:
go test ./... && echo "✅ BETA WORKSTREAM COMPLETE"

# FINAL COMMIT:
git add .
git commit -m "feat(errors,generics): complete type-safe system implementation

- Implemented strongly-typed build, deploy, and scan tools
- Created generic registry system with type safety
- Added comprehensive RichError context throughout
- Eliminated interface{} usage from tool registry
- Achieved 95% compile-time type checking

BETA WORKSTREAM COMPLETE ✅

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com)"
```

## 🎯 SUCCESS CRITERIA

### Must Achieve (100% Required):
- ✅ **100% elimination of interface{}** in tool registry
- ✅ **80% of errors use RichError** with context
- ✅ **95% of type errors** caught at compile time
- ✅ **Type-safe tool execution** throughout system
- ✅ **All tests pass** with improved error handling
- ✅ **Performance within 5%** of baseline

### Quality Gates (Enforce Strictly):
```bash
# REQUIRED before each commit:
go test -short ./pkg/mcp/errors/...     # RichError tests
go test -short ./pkg/mcp/types/tools/... # Generics tests
go test -short ./pkg/mcp/internal/*/...  # Tool integration tests
go fmt ./pkg/mcp/...                     # Code formatting
go build ./pkg/mcp/...                   # Must compile

# TYPE SAFETY validation:
# Count interface{} instances - should decrease dramatically
rg "interface{}" pkg/mcp/internal/orchestration/ | wc -l  # Target: 0

# Count type assertions - should decrease with generics
rg "\.(" pkg/mcp/internal/orchestration/ | wc -l  # Target: <5
```

### Daily Validation Commands
```bash
# Morning startup:
go test -short ./pkg/mcp/... && echo "✅ Ready to work"

# After RichError implementation:
go test ./pkg/mcp/errors/rich/... && echo "✅ RichError system working"

# After generics implementation:
go test ./pkg/mcp/types/tools/... && echo "✅ Generics system working"

# After tool migration:
go test -short ./pkg/mcp/internal/build/... && echo "✅ Build tools type-safe"
go test -short ./pkg/mcp/internal/deploy/... && echo "✅ Deploy tools type-safe"
go test -short ./pkg/mcp/internal/scan/... && echo "✅ Scan tools type-safe"

# End of day:
go test ./... && echo "✅ All systems functional"
```

## 🚨 CRITICAL COORDINATION POINTS

### Dependencies You Need:
- **WORKSTREAM ALPHA** unified validation system - MUST be complete before you start
- External merge of ALPHA changes - Wait for clean branch

### Dependencies on Your Work:
- **WORKSTREAM GAMMA** needs RichError and generics for testing framework
- **WORKSTREAM EPSILON** needs your generic types to replace interface{}
- **WORKSTREAM DELTA** needs your tool interfaces for consolidation

### Files You Own (Full Authority):
- `pkg/mcp/errors/` (entire package) - You create the error system
- `pkg/mcp/types/tools/` (entire package) - You create generic tool types
- Tool type definitions in `*_atomic.go` files - You strongly type them
- Registry implementation - You make it type-safe

### Files to Coordinate On:
- `pkg/mcp/core/interfaces.go` - Work with WORKSTREAM DELTA on tool interfaces
- Any file with interface{} usage - Coordinate with WORKSTREAM EPSILON

## 📊 PROGRESS TRACKING

### Daily Metrics to Track:
```bash
# RichError adoption:
rg "rich\.NewError\|rich\..*Error" pkg/mcp/ | wc -l  # Should increase

# Interface{} elimination in critical paths:
rg "interface{}" pkg/mcp/internal/orchestration/ | wc -l  # Should decrease to 0

# Type assertions remaining:
rg "\.(" pkg/mcp/internal/orchestration/ | wc-l  # Should decrease dramatically

# Generic type usage:
rg "Tool\[.*\]" pkg/mcp/ | wc -l  # Should increase

# Strongly-typed tool definitions:
rg "type.*Params struct" pkg/mcp/internal/ | wc -l  # Should increase
rg "type.*Result struct" pkg/mcp/internal/ | wc -l  # Should increase
```

### Daily Summary Format:
```
WORKSTREAM BETA - DAY X SUMMARY
===============================
Progress: X% complete
RichError implementation: X% of critical paths
Generic types: X tool types created
Interface{} elimination: X instances removed

Files modified today:
- pkg/mcp/errors/rich/types.go (created)
- pkg/mcp/internal/build/build_image_atomic.go (strongly typed)
- [other files]

Type safety improvements:
- X compile-time type checks added
- X type assertions removed
- X interface{} instances eliminated

Issues encountered:
- [any blockers or concerns]

Coordination needed:
- [shared file concerns with other workstreams]

Tomorrow's focus:
- [next priorities]

Quality status: All tests passing ✅
Performance impact: <X% overhead
```

## 🛡️ ERROR HANDLING & ROLLBACK

### If Things Go Wrong:
1. **Compilation fails**: Check generic type constraints
2. **Tests fail**: Verify type conversions and error handling
3. **Performance regression**: Review generic instantiation overhead
4. **Breaking changes**: Add type conversion helpers

### Rollback Procedure:
```bash
# Emergency rollback:
git checkout HEAD~1 -- pkg/mcp/errors/
git checkout HEAD~1 -- pkg/mcp/types/tools/
git checkout HEAD~1 -- pkg/mcp/internal/*/

# Selective rollback:
git checkout HEAD~1 -- <specific-problematic-file>
```

## 🎯 KEY IMPLEMENTATION PATTERNS

### RichError Builder Pattern:
```go
// Example: Rich Dockerfile parsing error
return rich.NewError().
    Code("DOCKERFILE_SYNTAX_ERROR").
    Message("Invalid FROM instruction syntax").
    Type(rich.ErrTypeValidation).
    Severity(rich.SeverityHigh).
    Context("line_number", lineNum).
    Context("instruction", "FROM").
    Suggestion("Add base image after FROM keyword").
    HelpURL("https://docs.docker.com/engine/reference/builder/#from").
    WithLocation().
    Build()
```

### Generic Tool Pattern:
```go
// Example: Strongly-typed build tool
type DockerBuildTool = tools.Tool[DockerBuildParams, DockerBuildResult]

func (t *dockerBuildToolImpl) Execute(ctx context.Context, params DockerBuildParams) (DockerBuildResult, error) {
    // Validate params at compile time
    if err := params.Validate(); err != nil {
        return DockerBuildResult{}, rich.NewError().
            Code("INVALID_BUILD_PARAMS").
            Message("Build parameters validation failed").
            Type(rich.ErrTypeValidation).
            Cause(err).
            Build()
    }

    // Type-safe implementation
    // ...

    return DockerBuildResult{
        Success: true,
        ImageID: imageID,
        Duration: duration,
        SessionID: params.SessionID,
    }, nil
}
```

### Generic Registry Pattern:
```go
// Example: Type-safe registry implementation
func (r *GenericRegistry[T, TParams, TResult]) Execute(name string, params TParams) (TResult, error) {
    r.mu.RLock()
    tool, exists := r.tools[name]
    r.mu.RUnlock()

    if !exists {
        var zero TResult
        return zero, rich.NewError().
            Code("TOOL_NOT_FOUND").
            Message(fmt.Sprintf("Tool '%s' not found", name)).
            Type(rich.ErrTypeBusiness).
            Severity(rich.SeverityHigh).
            Context("tool_name", name).
            Context("available_tools", r.ListTools()).
            Suggestion("Check available tools with ListTools()").
            Build()
    }

    return tool.Execute(ctx, params)
}
```

## 🎯 FINAL DELIVERABLES

At completion, you must deliver:

1. **Complete RichError system** (`pkg/mcp/errors/rich/`) with builder pattern
2. **Generic tool type system** (`pkg/mcp/types/tools/`) with constraints
3. **Strongly-typed tool implementations** for all atomic tools
4. **Type-safe registry system** with zero interface{} usage
5. **80% RichError adoption** in critical error paths
6. **95% compile-time type checking** throughout system
7. **Performance within 5%** of baseline with better error context

**Remember**: Your type safety work enables other workstreams to eliminate interface{} usage and improve code quality. Focus on creating robust, performant, and type-safe foundations! 🚀

---

**CRITICAL**: Stop work and create summary at end of each day. Do not attempt merges - external coordination will handle branch management. Your job is to implement type safety systematically and maintain quality throughout.
