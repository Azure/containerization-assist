# Container Kit MCP Refactoring TODO

## Current Architecture Analysis

### Package Structure Issues

| **Problem Area** | **Current State** | **Impact** |
|------------------|-------------------|------------|
| **Package Depth** | 5-level imports (`pkg/mcp/application/internal/pipeline/...`) | Poor IDE navigation, complex import rules |
| **Interface Inflation** | 170 interfaces (target: ≤50) | Difficult navigation, poor API discovery |
| **Leaky Layering** | GomcpManager reaches into infra, services, domain directly | Long dependency chains, slow refactors |
| **Global State** | Tool registries and sync.Map in orchestrator | Breaks test isolation, complex shutdown |
| **Concurrency Model** | Job pools exist but orchestrator can spawn unlimited goroutines | Ambiguous resource management |
| **Registry Duplication** | 4 registry implementations with interface{} casting | Unnecessary complexity, type safety issues |
| **Wrapper Classes** | Many wrappers (`ServiceSessionWrapper`, `ZerologToSlogAdapter`) | Indirection without value, maintenance overhead |
| **CI Quality Gates** | Security/quality audits report but don't fail builds | False confidence, technical debt accumulation |

### Current Package Stats
- **606 Go files** across pkg/mcp/
- **Three main layers**: application/, domain/, infra/
- **23 files** in pipeline/ directory alone
- **4 registry implementations** with overlapping functionality
- **170 interfaces** (target: ≤50 for maintainability)
- **Complex dependency chains** affecting refactor velocity

### Key Files by Layer

#### Application Layer (`pkg/mcp/application/`)
- **Core Services**: `core/mcp.go`, `core/factory.go`
- **API Interface**: `api/interfaces.go` (831 lines - single source of truth)
- **Pipeline**: `internal/pipeline/manager.go`, `internal/pipeline/background_workers.go`
- **Registry**: `orchestration/registry/tool_creation.go`
- **Server**: `services/core/server.go`, `services/core/server_lifecycle.go`

#### Domain Layer (`pkg/mcp/domain/`)
- **Containerization**: `containerization/analyze/`, `containerization/build/`, `containerization/deploy/`, `containerization/scan/`
- **Error Handling**: `errors/rich.go`, `errors/constructors.go`
- **Session Management**: `session/manager.go`, `session/session_manager_unified.go`
- **Types**: `types/core.go`, `types/tools/schema.go`

#### Infrastructure Layer (`pkg/mcp/infra/`)
- **Transport**: `transport/stdio.go`, `transport/http.go`
- **Persistence**: `persistence/boltdb/persistence.go`
- **Templates**: `templates/manifests/`, `templates/workflows/`

---

## Refactoring Principles

### 1. Three Bounded Contexts Only

```
pkg/mcp/
├── domain/     # Pure business logic, zero external dependencies
├── app/        # Orchestration & policies (formerly application/)
└── infra/      # External integrations (Docker, K8s, BoltDB, HTTP)
```

### 2. Flatten Import Paths
- **Target**: 3 import segments maximum
- **Current**: Up to 5 levels deep
- **Goal**: Simplified navigation and cleaner dependency management

### 3. Single Registry Pattern
- Consolidate 4 registry implementations into one canonical `app.Registry`
- Remove interface{} casting and type aliases
- Provide deprecation aliases for backward compatibility

### 4. Remove Indirection
- Delete wrapper/adapter classes that only add logging or type casting
- Pass dependencies directly via dependency injection
- Simplify call chains

### 5. Library-First Design
- Expose `pipeline.Run(ctx, Job)` helper functions
- Use functional options pattern (`WithWorkerPool(n)`)
- Avoid framework-style initialization

---

## Concrete Refactoring Tasks

### 1. Collapse Manager Chain → Single Scheduler
**Goal**: Replace Manager → BackgroundWorkerManager → JobOrchestrator with unified Scheduler

**Files to Replace**:
- `pkg/mcp/application/internal/pipeline/manager.go`
- `pkg/mcp/application/internal/pipeline/background_workers.go`
- `pkg/mcp/application/orchestration/workflow/job_manager.go`

**New File**: `pkg/mcp/app/pipeline/scheduler.go`

**Benefits**:
- Removes ~800 lines of code
- Eliminates 3-layer control chain
- Provides clean `Start()`, `Stop()`, `Submit()` API

**Implementation**:
```go
type Scheduler struct {
    workers   int
    queue     chan Job
    log       zerolog.Logger
    ctx       context.Context
    cancel    context.CancelFunc
}

func NewScheduler(l zerolog.Logger, opts ...Option) *Scheduler
func (s *Scheduler) Start() error
func (s *Scheduler) Stop() error
func (s *Scheduler) Submit(j Job) error
```

### 2. Delete Performance Optimization Stubs
**Goal**: Remove unused performance tuning code

**Files to Delete**:
- `pkg/mcp/application/internal/pipeline/performance_optimizations.go`
- `pkg/mcp/application/internal/pipeline/distributed_caching.go`
- `pkg/mcp/application/internal/pipeline/distributed_operations.go`

**Benefits**:
- Removes ~1,800 lines of unused code
- Eliminates premature optimization
- Simplifies maintenance

**Command**:
```bash
git rm pkg/mcp/application/internal/pipeline/performance_optimizations.go
git rm pkg/mcp/application/internal/pipeline/distributed_caching.go
git rm pkg/mcp/application/internal/pipeline/distributed_operations.go
```

### 3. Consolidate Registry Implementations
**Goal**: Replace 4 registry types with single canonical implementation

**Current Registries**:
- `TypedToolRegistry` (pkg/mcp/application/orchestration/registry/)
- `FederatedRegistry` (pkg/mcp/application/core/registry.go)
- `ToolRegistry` (pkg/mcp/application/api/registry.go)
- `MemoryRegistry` (pkg/mcp/services/registry/memory_registry.go)

**New Implementation**: `pkg/mcp/app/registry/registry.go`

**Migration Strategy**:
1. Create new unified registry
2. Add type aliases: `type TypedToolRegistry = registry.Registry`
3. Update imports gradually
4. Remove old implementations

### 4. Error System Consolidation
**Goal**: Finish migration to unified error handling

**Current State**:
- `pkg/mcp/domain/errors/` (partially complete)
- Multiple local error aliases causing import cycles
- Inconsistent error handling patterns

**Target Structure**:
```
pkg/mcp/domain/errors/
├── errors.go          # Core error types
├── rich.go           # RichError implementation
├── constructors.go   # Error factory functions
├── codes/            # Error code constants
└── factories.go      # Domain-specific error factories
```

**Benefits**:
- Single import path for all error handling
- Consistent error structure across domains
- Eliminates import cycles

### 5. Remove Wrapper/Adapter Classes
**Goal**: Delete zero-value wrappers and simplify dependency injection

**Candidates for Removal**:
- `ServiceSessionWrapper` → Direct `services.SessionStore` usage
- `ZerologToSlogAdapter` → Direct logger injection
- Various `*Wrapper` and `*Adapter` classes

**Strategy**:
1. Identify wrappers that only add logging/type casting
2. Replace with direct dependency injection
3. Update call sites to use concrete dependencies

### 6. Flatten Package Structure
**Goal**: Reorganize to 3-level import structure

**Current Structure**:
```
pkg/mcp/application/internal/pipeline/...  # 5 levels
pkg/mcp/application/orchestration/...      # 4 levels
```

**Target Structure**:
```
pkg/mcp/app/pipeline/...     # 3 levels
pkg/mcp/app/server/...       # 3 levels
pkg/mcp/domain/...           # 3 levels
pkg/mcp/infra/...            # 3 levels
```

**Implementation**:
1. Create new directory structure
2. Move files to appropriate locations
3. Update import statements
4. Provide migration script

### 7. Standardize Constructor Patterns
**Goal**: Consistent functional options pattern across all types

**Current Issues**:
- Inconsistent constructor signatures
- Some types use complex initialization
- Missing zero-config defaults

**Target Pattern**:
```go
type Option func(*T)

func NewScheduler(opts ...Option) *Scheduler
func NewRegistry(opts ...Option) *Registry
func WithWorkers(n int) Option
func WithQueueSize(n int) Option
```

### 8. Simplify Health Check Implementation
**Goal**: Replace complex health check logic with simple HTTP endpoint

**Current Issues**:
- Custom `GetManagerStats()` JSON responses
- Duplicate health check logic
- Over-engineered monitoring

**Target Implementation**:
- Simple HTTP `/health` endpoint
- Basic status checks (scheduler running, queue depth)
- Remove complex metrics collection

### 9. Remove All OpenTelemetry Usage from pkg/mcp
**Goal**: Eliminate all OpenTelemetry imports and usage from the MCP package

**Current Issues**:
- OpenTelemetry imports scattered throughout pkg/mcp
- Complex tracing and metrics collection
- Unnecessary external dependencies

**Target Implementation**:
- Remove all `go.opentelemetry.io/otel/*` imports
- Remove all `trace.Span` and `metric.Meter` usage
- Replace with simple logging where needed
- Clean up related configuration

**Files to Check**:
```bash
# Search for OpenTelemetry imports
grep -r "go.opentelemetry.io/otel" pkg/mcp/

# Search for trace/metric usage
grep -r "trace\." pkg/mcp/
grep -r "metric\." pkg/mcp/
grep -r "otel\." pkg/mcp/
```

**Commands to Execute**:
```bash
# Remove OpenTelemetry imports
find pkg/mcp -name "*.go" -exec sed -i '/go\.opentelemetry\.io\/otel/d' {} \;

# Remove trace/metric variable declarations
find pkg/mcp -name "*.go" -exec sed -i '/trace\./d' {} \;
find pkg/mcp -name "*.go" -exec sed -i '/metric\./d' {} \;
```

### 10. Enforce Code Quality Standards
**Goal**: Add automated checks for code quality

**Standards to Enforce**:
- File size ≤ 800 lines
- Package import depth ≤ 3 levels
- Architecture boundary compliance
- Coverage thresholds

**Implementation**:
```bash
# File size check
scripts/check_file_size.sh

# Architecture boundary check
go run tools/check-boundaries -strict ./...

# Coverage enforcement
make coverage-html
```

### 11. Interface Reduction Strategy
**Goal**: Reduce interface count from 170 to ≤50

**Current Issues**:
- 170 interfaces making API discovery difficult
- Many interfaces with single implementations
- Redundant abstractions

**Target Approach**:
1. Identify interfaces with single implementations
2. Remove unnecessary abstractions
3. Consolidate related interfaces
4. Keep only essential domain boundaries

**Commands to Execute**:
```bash
# Find all interface definitions
grep -r "type.*interface" pkg/mcp/ | wc -l

# Identify single-implementation interfaces
go run tools/interface-analyzer pkg/mcp/
```

### 12. Fix Leaky Layering
**Goal**: Eliminate cross-layer dependencies

**Current Issues**:
- GomcpManager reaches into infra, services, domain directly
- Long dependency chains
- Violated separation of concerns

**Target Approach**:
1. Audit all cross-layer imports
2. Introduce proper abstraction layers
3. Use dependency injection for cross-layer communication
4. Enforce layer boundaries with linting

### 13. Eliminate Global State
**Goal**: Remove global state for better testability

**Current Issues**:
- Tool registries as global variables
- sync.Map in orchestrator
- Breaks test isolation

**Target Approach**:
1. Convert global registries to dependency-injected services
2. Replace sync.Map with proper state management
3. Ensure clean shutdown procedures
4. Enable parallel test execution

### 14. Clarify Concurrency Model
**Goal**: Establish clear concurrency boundaries

**Current Issues**:
- Job pools exist but orchestrator can spawn unlimited goroutines
- Ambiguous resource management
- Potential resource leaks

**Target Approach**:
1. Enforce worker pool limits
2. Prevent tools from spawning unbounded goroutines
3. Add context-based cancellation
4. Monitor goroutine counts

### 15. Enforce CI Quality Gates
**Goal**: Make CI fail on quality violations

**Current Issues**:
- Security/quality audits report but don't fail builds
- False confidence in code quality
- Technical debt accumulation

**Target Approach**:
1. Enable fail-on-violations for security scans
2. Add quality gates for interface count
3. Enforce architecture boundary violations
4. Block PRs that introduce technical debt

### 16. Testing Strategy
**Goal**: Improve test coverage and fix boundary violations

**Current Issues**:
- Tests importing `application/internal` packages
- Several tests marked with `t.Skip`
- Inconsistent test patterns

**Target Approach**:
1. Fix architecture boundary violations in tests
2. Increase coverage thresholds by +5% per package
3. Ensure `go test -race ./...` passes
4. Remove `t.Skip` statements

---

## Migration Timeline

| **Week** | **Task** | **Deliverable** |
|----------|----------|-----------------|
| **1** | Scheduler Implementation | New `scheduler.go` with legacy aliases |
| **2** | Remove Performance Stubs | Delete unused optimization files |
| **3** | Registry Consolidation | Single registry with migration aliases |
| **4** | Error System Completion | Unified error package structure |
| **5** | Package Restructuring | 3-level import paths |
| **6** | Quality Enforcement | CI checks and coverage improvements |

---

## Success Metrics

| **Metric** | **Before** | **Target** |
|------------|------------|------------|
| Package Depth | 5 levels | 3 levels |
| Interface Count | 170 | ≤50 |
| Pipeline Files | 23 files | 8 files |
| Registry Implementations | 4 | 1 |
| Lines of Code | ~159,570 | ~156,870 (-2,700) |
| Global State Variables | Multiple | 0 |
| Cross-layer Dependencies | Multiple | 0 |
| Test Coverage | Current | +5% per package |
| Architecture Violations | Multiple | 0 |
| CI Quality Gates | Report only | Fail on violations |

---

## Ready-to-Run AI Assistant Prompts

### Prompt 1: Scheduler Consolidation
```
Goal: Replace the three-layer control chain (Manager, BackgroundWorkerManager, JobOrchestrator) with a flattened Scheduler API.

Context: Current Manager orchestrates only Start() and Stop() operations and passes everything through to other structs. BackgroundWorkerManager & JobOrchestrator duplicate queueing and health logic.

Constraints:
- Public API becomes: type Scheduler interface { Start() error; Stop() error; Submit(ctx context.Context, j Job) error }
- New file goes in pkg/mcp/app/pipeline/scheduler.go
- Provide LegacyManager type alias for backward compatibility
- Remove ≥800 lines of dead code

Deliverables:
- Full content of scheduler.go
- Deletion list for obsolete files
- Verification that go vet ./... passes
```

### Prompt 2: Performance Stub Removal
```
Goal: Remove unused performance-tuning stubs that are never invoked outside tests.

Context: performance_optimizations.go and distributed_caching.go contain placeholder "optimizer" code with no callers.

Constraints:
- Delete files and update any references in go.mod or import lines
- Ensure build succeeds and unit tests compile

Deliverables:
- git rm command list
- Updated go.mod diff (if any)
- Confirmation of green build
```

### Prompt 3: Registry Consolidation
```
Goal: Keep one canonical tool registry; deprecate TypedToolRegistry, FederatedRegistry, etc.

Context: Analysis shows 4 registry types and 170+ interfaces with 75% slated for removal.

Constraints:
- Create pkg/mcp/app/registry/registry.go with basic Register/Get/Stats API
- Add type aliases in old paths to avoid breaking imports
- Refactor callers in ≤2 steps using search/replace

Deliverables:
- New registry.go file content
- Import replacement patches
- Verification of successful migration
```

### Prompt 4: Error System Migration
```
Goal: Finish error consolidation plan to pkg/mcp/errors with single coherent package.

Context: Duplicate RichError definitions and local aliases create import cycles.

Constraints:
- Maximum 7 files (errors.go, rich.go, constructors.go, codes/, factories.go)
- No internal/errors remain
- Import path must be github.com/Azure/container-kit/pkg/mcp/errors

Deliverables:
- Complete directory structure with file contents
- Script output proving error package uniqueness
- Verification of eliminated import cycles
```

### Prompt 5: Wrapper Elimination
```
Goal: Delete zero-value wrappers such as ServiceSessionWrapper & ZerologToSlogAdapter.

Context: Static analysis flagged 40+ wrapper/adapter occurrences with no additional behavior.

Constraints:
- Confirm each candidate only adds logs or type-casts
- Replace call-sites with direct dependency injection
- Provide removal vs replacement table

Deliverables:
- Diff removing wrappers
- Updated call-sites
- Verification of boundary compliance
```

### Prompt 6: Package Flattening
```
Goal: Adopt proposed folder layout (pkg/mcp/domain, pkg/mcp/app, pkg/mcp/infra).

Context: Current paths like pkg/mcp/application/internal/pipeline/... require 5-segment imports.

Constraints:
- Only internal code may import /internal/ paths
- External packages must import public APIs
- Provide automated import rewrite script

Deliverables:
- Directory tree before/after
- Import update script
- Verification of successful migration
```

---

## Implementation Notes

### Key Interface Files
- **`pkg/mcp/application/api/interfaces.go`**: Single source of truth (831 lines)
- **`pkg/mcp/application/interfaces/interfaces.go`**: Compatibility layer with type aliases
- **`pkg/mcp/services/interfaces.go`**: Service container interfaces

### Critical Dependencies
- **BoltDB**: Session persistence
- **Docker Client**: Container operations
- **Kubernetes Client**: Deployment operations
- **Standard HTTP**: Basic health checks

### Architecture Compliance
- Follow three-context architecture (ADR-001)
- Use manual dependency injection (ADR-006)
- Implement unified error system (ADR-004)
- Use tag-based validation DSL (ADR-005)
- Simple health check endpoints (no complex observability)

### Quality Gates
- Error budget: 100 lint issues maximum
- Performance target: <300μs P95 per request
- File size limit: 800 lines maximum
- Architecture boundary enforcement