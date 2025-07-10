# Container Kit Architecture Feedback & Refactoring Roadmap

This document outlines ten high-impact, architecture-level refactors that would pay the biggest dividends inside `pkg/mcp`. Each item includes the rationale, concrete changes needed, and specific code locations that demonstrate the issue.

## 🎯 High-Impact Refactoring Recommendations

### 1. **Flatten the Package Tree (≤ 3 levels deep)**

**Current Issue**: Deep package nesting violates Go's "import simplicity" ideal and creates maintainability issues.

**Specific Examples**:
- `pkg/mcp/application/internal/conversation/validators.go` (5 levels deep)
- `pkg/mcp/application/internal/retry/coordinator.go` (5 levels deep)
- `pkg/mcp/application/orchestration/pipeline/atomic/builder.go` (6 levels deep)
- `pkg/mcp/infra/adapters/transport/stdio/handler.go` (6 levels deep)

**Target Structure**: Flatten to maximum 3 levels: `pkg/mcp/{domain,application,infra}/module`

**Impact**: Simplifies imports, reduces cognitive load, and eliminates depth violations in quality gates.

---

### 2. **Adopt a Single Clean-Architecture Ring**

**Current Issue**: Three overlapping architectural rings with circular dependencies.

**Specific Problems**:
- `pkg/mcp/domain/internal/` packages imported by application layer (violates clean architecture)
- `pkg/mcp/application/internal/runtime/` contains reflection-heavy helpers accessible by any layer
- Circular imports between application and infrastructure layers

**Files Affected**:
- `pkg/mcp/domain/internal/common/utils.go:45` - shared utilities breaking domain purity
- `pkg/mcp/application/internal/runtime/registry.go:123` - reflection-heavy tool registration
- `pkg/mcp/application/services/container.go:67` - manual dependency wiring

**Target**: Clear dependency inversion with interfaces, eliminating import cycles.

---

### 3. **Consolidate Duplicated Retry/Coordinator Logic**

**Current Issue**: Two identical retry implementations with 95% code overlap.

**Duplicate Files**:
- `pkg/mcp/infra/retry/coordinator.go:1-156` - Full retry implementation
- `pkg/mcp/application/internal/retry/coordinator.go:1-152` - Near-identical copy
- `pkg/mcp/application/internal/conversation/retry.go:23-89` - Third variant

**Specific Duplication**:
- Both define identical `BackoffStrategy` and `Policy` structures
- Same exponential backoff algorithms with minor variations
- Duplicate test suites testing identical behavior

**Target**: Single retry interface in domain layer, implemented once in infrastructure.

---

### 4. **Merge the Three "Tool Registry" Layers**

**Current Issue**: Three separate registry implementations with different interfaces.

**Registry Implementations**:
1. **Core Registry** (`pkg/mcp/application/core/registry.go:45`)
   - Deprecated but still used in 15+ files
   - Basic string-based tool lookup

2. **Commands Registry** (`pkg/mcp/application/commands/command_registry.go:89`)
   - Adds metadata system and validation
   - Used by CLI implementations

3. **Runtime Registry** (`pkg/mcp/application/internal/runtime/registry.go:123`)
   - Uses reflection and `interface{}` parameters
   - Auto-registration via `RegisterAllTools()`

**Target**: Single `ToolRegistry` interface with dependency injection.

---

### 5. **Replace Manual Wiring with Dependency Injection**

**Current Issue**: Manual dependency wiring in multiple locations creates maintenance burden.

**Specific Examples**:
- `pkg/mcp/application/services/container.go:67` - 17+ manual service dependencies
- `cmd/mcp-schema-gen/templates/tool_template.go:123` - Generated tool constructors
- `pkg/mcp/application/core/server.go:156` - Manual service initialization

**Wiring Complexity**:
```go
// Current manual wiring example
container := &DefaultServiceContainer{
    sessionStore: NewSessionStore(db),
    buildExecutor: NewBuildExecutor(docker, logger),
    toolRegistry: NewToolRegistry(validator, logger),
    // ... 14+ more manual dependencies
}
```

**Target**: Lightweight DI container using **Google Wire** for compile-time dependency injection.

---

### 6. **Extract Common Validator Helpers to Shared Module**

**Current Issue**: Validator patterns duplicated across multiple packages.

**Duplicated Validation Logic**:
- `pkg/mcp/application/internal/conversation/validators.go:34-89` - String length, pattern matching
- `pkg/mcp/domain/security/validators.go:23-67` - Similar string validation
- `pkg/mcp/common/validation/utils.go:45-123` - Core validation utilities
- `pkg/mcp/common/validation-core/validators/base.go:12-78` - Base validator patterns

**Target**: Single validation module with composable validators.

---

### 7. **Introduce Declarative Command Routing**

**Current Issue**: Large switch/case statements for command routing.

**Specific Locations**:
- `pkg/mcp/application/commands/command_registry.go:145-289` - 144-line switch statement
- `pkg/mcp/application/server/handler.go:234-378` - Similar routing logic
- `pkg/mcp/infra/transport/stdio/handler.go:123-267` - Third routing implementation

**Current Pattern**:
```go
switch req.Method {
case "analyze":
    return h.handleAnalyze(ctx, req)
case "build":
    return h.handleBuild(ctx, req)
// ... 20+ more cases
}
```

**Target**: Map-based routing with single registration point.

---

### 8. **Collapse Orchestration Pipeline Variants**

**Current Issue**: Three parallel pipeline implementations with overlapping functionality.

**Pipeline Variants**:
- `pkg/mcp/application/orchestration/pipeline/atomic/` - 12 files, atomic operations
- `pkg/mcp/application/orchestration/pipeline/legacy/` - 8 files, deprecated but used
- `pkg/mcp/application/orchestration/pipeline/simple/` - 6 files, basic workflows

**Specific Overlap**:
- All three implement similar stage execution patterns
- Duplicate context propagation and error handling
- Similar builder patterns with different APIs

**Target**: Single `Pipeline` interface with fluent builder configuration.

---

### 9. **Centralize Rich Error Construction**

**Current Issue**: Mixed error handling despite RichError system.

**Error Pattern Analysis**:
- **622 instances** of `fmt.Errorf` across 83 files
- **189 instances** of `RichError` construction
- **34 files** mix both patterns in the same file

**Specific Examples**:
- `pkg/mcp/domain/session/manager.go:89` - Uses `fmt.Errorf` in domain layer
- `pkg/mcp/application/tools/analyze.go:156` - Mixed error patterns
- `pkg/mcp/infra/docker/client.go:234` - Raw errors for transport issues

**Target**: Standardized error construction with linting enforcement.

---

### 10. **Automate Code Generation & Remove Boilerplate**

**Current Issue**: 80% identical generated files with manual maintenance.

**Generated Code Examples**:
- `cmd/mcp-schema-gen/templates/tool_template.go` - 234 lines of boilerplate
- `pkg/mcp/application/tools/*/validator.go` - Near-identical validation logic
- `pkg/mcp/application/tools/*/test.go` - Duplicate test patterns

**Specific Boilerplate**:
- Tool registration code (23 lines per tool)
- Validator setup (45 lines per tool)
- Test scaffolding (67 lines per tool)

**Target**: `go generate` templates with only custom logic in repository.

---

### 11. **Remove Deprecated Code and Update Callers**

**Current Issue**: Extensive deprecated code throughout the codebase with mixed usage patterns.

**Deprecated Code Analysis**:
- **72 deprecated items** across 25 files
- **Major deprecated systems**: Reflection-based validation, old service interfaces, legacy pipeline components
- **Mixed usage**: Some deprecated code still actively used alongside new implementations

**Critical Deprecated Components**:

1. **Service Interfaces** (High Priority):
   - `pkg/mcp/application/services/retry.go:21` - `api.RetryCoordinator` replacement available
   - `pkg/mcp/application/services/transport.go:8` - `api.Transport` replacement available
   - `pkg/mcp/application/core/server.go:20,32,38` - `ServerService` and `TransportService` replacements

2. **Validation System** (High Priority):
   - `pkg/common/validation/unified_validator.go:20,26,91` - Entire reflection-based system
   - `pkg/common/validation-core/standard.go:3,25,121,219` - Legacy validation patterns
   - `pkg/common/validation-core/core/interfaces.go:8,46,108` - Non-generic validator interfaces

3. **Tool Registry** (High Priority):
   - `pkg/mcp/application/core/tool_registry.go:10` - Use `services.ToolRegistry` instead
   - `pkg/mcp/application/core/types.go:81` - `KnownRegistries` → `RegistryService`

4. **Workflow Engine** (Medium Priority):
   - `pkg/mcp/application/workflows/engine.go:28` - Legacy workflow executor
   - `pkg/mcp/application/workflows/job_execution_service.go:426,430,434,440` - Old job execution API

5. **Schema Generation** (Medium Priority):
   - `pkg/mcp/domain/tools/schema.go:89,95,162,442,684,711,752,789,813,841` - 10+ deprecated schema functions
   - `pkg/mcp/domain/tools/tool_validation.go:18,27` - Legacy validation error construction

6. **State Management** (Medium Priority):
   - `pkg/mcp/application/state/integration.go:17` - Use `services.ServiceContainer`
   - `pkg/mcp/application/state/context_enrichers.go:16,33` - Use `services.SessionStore/SessionState`

**Removal Strategy**:
1. **Phase 1**: Update all callers to use new APIs (identify with `grep -r "deprecated_function_name"`)
2. **Phase 2**: Remove deprecated code after ensuring no active usage
3. **Phase 3**: Update documentation and examples to reflect new patterns

**Impact**: Eliminates 15-20% of technical debt and reduces maintenance burden.

---

## 📈 Implementation Strategy

### Phase 0: Quick Wins (Week 1)
Front-load low-risk, high-impact changes that create a clean baseline for all subsequent work.

**Priority Actions**:
1. **Single Logging Backend** - Standardize on `slog` throughout codebase
2. **Context Propagation Plumbing** - Add context parameters (without timeouts yet)
3. **Background Worker Ticker Fix** - Stop goroutine leaks in worker manager
4. **Basic CI Gates** - Package depth and architecture boundary linting

**Success Metrics**:
- Zero mixed logging frameworks (`slog` only)
- All functions accept `context.Context` parameter
- No goroutine leaks in worker tests
- CI fails on package depth >3 levels

### Phase 1: Foundation (Weeks 2-3)
Start with package flattening – it has the widest cascading effect but almost no functional risk.

**Priority Actions**:
1. **Package Restructuring** - Move deep packages to 3-level maximum
2. **Dependency Cleanup** - Remove circular imports and domain violations
3. **Import Rewriting** - Automated scripts with code-freeze window
4. **Deprecation Audit** - Identify all deprecated code usage and migration paths

**CI Gates Added**:
- Maximum package depth ≤ 3 levels
- No circular imports between layers
- Architecture boundary violations fail builds

**Success Metrics**:
- All packages at ≤3 levels: `pkg/mcp/{domain,application,infra}/module`
- Zero circular import cycles
- All deprecated code catalogued with migration paths

### Phase 2: Consolidation (Weeks 4-5)
Cut duplicate libraries and merge registry systems before DI introduction.

**Priority Actions**:
1. **Duplicate Removal** - Consolidate retry and validation logic
2. **Registry + DI Unification** - Single registry with Wire-based DI
3. **RichError Standardization** - Enforce RichError everywhere with linting
4. **Deprecated Code Removal** - Remove high-priority deprecated components

**CI Gates Added**:
- RichError usage enforced (max 10 grandfathered `fmt.Errorf`)
- No duplicate retry implementations
- Registry reflection calls = 0

**Success Metrics**:
- Single retry interface in domain layer
- Unified `ToolRegistry` with Google Wire injection
- <10 `fmt.Errorf` calls in pkg/mcp (grandfathered only)

### Phase 3: Modernization (Weeks 6-7)
Introduce advanced patterns and consolidate pipeline implementations.

**Priority Actions**:
1. **Pipeline Consolidation** - Single pipeline interface with fluent builder
2. **Command Routing Cleanup** - Map-based routing tables
3. **Code Generation** - Automate boilerplate creation with `go generate`
4. **Interface Modernization** - Replace `interface{}` with generics

**CI Gates Added**:
- No reflection in registry code
- Command routing uses declarative maps only
- Generated code matches templates

**Success Metrics**:
- Single `Pipeline` interface with builder pattern
- Map-based command routing (no switch statements)
- 80% reduction in boilerplate code

### Phase 4: Optimization (Weeks 8-9)
Finally, add semantic context usage, performance optimizations, and comprehensive testing.

**Priority Actions**:
1. **Context Timeouts** - Real timeouts and cancellation propagation
2. **Performance Optimization** - Generic types, reduced allocations
3. **Documentation & Testing** - Complete coverage and API docs
4. **OpenTelemetry Integration** - Distributed tracing and metrics

**CI Gates Added**:
- Context timeout enforcement
- Performance benchmarks <300μs P95
- Test coverage >55% global, >80% new code

**Success Metrics**:
- All operations respect context timeouts
- P95 latency <300μs for tool operations
- 55% global test coverage, 80% for new code

---

## 🔄 Phase-Aware Quality Gates

### CI Matrix Configuration
```yaml
strategy:
  matrix:
    phase_target: [0, 1, 2, 3, 4]

jobs:
  quality-gates:
    runs-on: ubuntu-latest
    steps:
      - name: Check Phase 0 Rules
        if: matrix.phase_target >= 0
        run: |
          # Single logging backend
          ! grep -r "zerolog\|logrus" pkg/mcp/
          # Context plumbing
          scripts/check-context-params.sh

      - name: Check Phase 1 Rules
        if: matrix.phase_target >= 1
        run: |
          # Package depth ≤ 3
          scripts/check_import_depth.sh --max-depth=3
          # No circular imports
          go mod graph | scripts/check-cycles.sh

      - name: Check Phase 2 Rules
        if: matrix.phase_target >= 2
        run: |
          # RichError enforcement
          scripts/check-error-patterns.sh --max-fmt-errorf=10
          # No duplicate retry
          ! find pkg/mcp -name "*retry*" -type f | wc -l | grep -v "^1$"
```

### Coverage Ratchets by Phase
| Phase | Min Global Coverage | Key Packages | Benchmark P95 |
|-------|-------------------|--------------|---------------|
| 0     | 15% (baseline)    | —            | No limit      |
| 1     | 20%               | session, retry | 500μs        |
| 2     | 30%               | runtime, internal/* | 400μs   |
| 3     | 45%               | all internal/* | 350μs       |
| 4     | 55% global, 80% new | domain, application | 300μs |

---

## 🛡️ Risk Management & Mitigation

### External Integration Risks
| Risk | Impact | Mitigation | Owner |
|------|--------|------------|-------|
| **go:generate directives** | Build failures | Audit and update all directives | DevOps |
| **Downstream consumers** | Import breakage | Temporary re-export stubs | API Team |
| **Release tags** | Version confusion | Tag cleanup scripts | Release Team |

### Migration Process
1. **Dry-run Weekend** - Test migration scripts against main branch
2. **Code-freeze Window** - 1-2 hour freeze for import rewriting
3. **Immediate Rebase** - Push rewritten imports to minimize conflicts
4. **Rollback Plan** - Temporary compatibility shims for external users

### Developer Experience
- **Autofix Pre-commit Hook** - Runs import updates locally
- **Package Map Documentation** - Short video + wiki page
- **Slack Bot** - Reminds contributors of active phase rules
- **Migration Scripts** - Automated tools for common transformations

---

## 📊 Success Metrics & Exit Criteria

### Quantitative Targets
| Metric | Target | Measured By | Phase |
|--------|--------|-------------|-------|
| **Max import depth** | ≤ 3 levels | quality-gates job | 1 |
| **Duplicate retry implementations** | 0 copies | dupl linter | 2 |
| **fmt.Errorf in pkg/mcp** | <10 (grandfathered) | grep + CI | 2 |
| **Registry reflection calls** | 0 instances | go vet --tags=registry | 2 |
| **Command routing switch statements** | 0 instances | AST analysis | 3 |
| **Generated boilerplate** | 80% reduction | line count diff | 3 |
| **Context timeout violations** | 0 instances | custom linter | 4 |
| **P95 latency** | <300μs | benchmark CI | 4 |

### Timeline Adjustments
- **Unknown-unknown buffer**: 20% slack per phase
- **Holiday/freeze windows**: Account for release schedules
- **Part-time team**: Stretch to 12 weeks if team is not dedicated
- **Parallel workstreams**: CI improvements and docs don't block code moves

---

## 🗓️ Revised Implementation Timeline

| Week | Major Deliverables | Key Milestones |
|------|-------------------|----------------|
| **1** | Quick-wins patch-set, context plumbing, CI gates | Single logging, context params, basic linting |
| **2-3** | Package flattening, import rewrite, depth limits | Max 3-level packages, no circular imports |
| **4-5** | Registry + DI unification, RichError enforcement | Wire-based DI, <10 fmt.Errorf violations |
| **6-7** | Pipeline consolidation, command routing, code-gen | Single pipeline, map-based routing, templates |
| **8-9** | Context timeouts, performance tuning, documentation | Real timeouts, <300μs P95, 55% coverage |

---

## 🔍 Detailed Analysis

### High-level Architecture

The repo documents a clean three-layer model (domain ⇄ application ⇄ infra) and even codifies it in scripts and boundary-checking tools. That is excellent, but several implementation details currently violate (or at least blur) the intended separations:

| Area | Observation | Recommendation |
|------|-------------|----------------|
| **Internal packages inside domain** | The `domain/internal/**` subtree is imported by other layers, defeating "pure domain" intent. | Rename to something outside domain or mark truly private helpers with `internal/...` paths located outside the domain root. |
| **Generic "registry"/"runtime" helpers** | `application/internal/runtime/*` exposes reflection-heavy helpers (e.g. `RegisterAllTools`) that any layer can reach by taking an `interface{}` parameter. | Provide a typed `ToolRegistry` interface in api and inject it; eliminate `interface{}` and reflection in favour of generics/functions with explicit signatures. |
| **Deprecated parallel APIs** | There are two parallel sets of service abstractions (`application/services/*.go` versus the canonical `application/api/*.go`), with comments marking the older ones as "Deprecated". | Delete the deprecated wrappers once downstream code is migrated; keeping both increases cognitive load and leaves room for accidental cross-use. |

### Error Handling & Propagation

**Strengths**: The RichError builder pattern gives structured data, stack location, suggestions and machine codes—great for observability and client UX.

**Improvements**:

#### Builder versus Sentinel Costs
For hot paths (e.g. transport IO errors) the builder allocates heavily. Create a small set of sentinel errors or var templates for the tight loop cases; reserve RichError for high-level business failures.

#### Wrapping Rules
Many builder calls create new errors but drop the original cause (e.g. many `NewError().Messagef("X: %w", err)` calls are still TODO). Audit with `go vet -wrapcheck` or `errcheck` and wrap consistently.

#### Translate to JSON-RPC Once
Transport layer currently maps `RichError` → `JSON-RPC error` in several places; centralise that logic in a single adapter so that codes/maps stay consistent.

### Context, Cancellation & Timeouts

`adaptMCPContext` simply returns `context.Background()`, which discards cancellation, tracing and deadlines sent by the JSON-RPC layer.

**Action Items**:

| Step | Detail |
|------|--------|
| **Propagate upstream ctx** | Change the signature of every tool invocation and worker start to accept a real context (with req.ID or sessionID in WithValue). |
| **Timeouts per operation** | Read a default timeout from config or request params and set it via `context.WithTimeout`. |
| **Tracing hook** | If you adopt OpenTelemetry, inject the span here so lower layers get distributed tracing "for free". |

### Logging Consistency

You translate `slog` → `zerolog` but then ignore the original level mapping. Decide on one logging facade for the whole repo (my vote: `slog` in Go 1.22+) and provide shim adaptors only at process boundaries (CLI → structured).

### Concurrency & Shared State

The background-worker service is nicely abstracted and guards map access with an RW-mutex, but a few race avenues remain:

- **Double registration** – `RegisterWorker` silently overwrites; add a check and return an error if a name already exists.
- **Lifecycle sequencing** – `StopAll()` loops workers in unspecified order; if workers depend on each other add an explicit dependency graph or topological shutdown.
- **Health ticker leak** – `StartAll()` creates a `time.Ticker` but I could not find a `Stop()` call for it; ensure it stops during shutdown to avoid goroutine leaks.

Run `go test -race ./...` in CI (already gated in Makefile) regularly.

### Interface Hygiene & Generics

**Problems**:
- The registry utilities and some orchestration helpers use `interface{}` in new code (Go ≥1.22 supports type parameters).
- Tool factories return `interface{}` and get cast at call-time, moving failures to runtime.

**Recommendations**:
- Introduce a generic type `Factory[T any] func() T` and make `ToolRegistry.Register[T any](name string, f Factory[T])`.
- Replace reflection in `auto_registration.go` with compile-time `//go:generate` code or a static `var _ = ...` registration list—faster, safer, testable.

### Boundary & Dead-Code Enforcement

You already have shell scripts to find unused exports and mock files and a custom boundary checker tool. Hook both into CI so PRs fail when:
- A new exported identifier is unreferenced
- An import breaks a layer rule

### Testing & Coverage

Integration tests start a real server binary—excellent for regression defence. A few gaps to close:

| Gap | Suggested Action |
|-----|------------------|
| **Unit tests for domain rules** | Many business entities (session, configuration validators, error builders) have no direct unit tests. Aim for 80%+ coverage in `pkg/mcp/domain/**`. |
| **Property tests for retry/back-off** | Use `testing/quick` or `github.com/stretchr/testify/require` with fuzz inputs to ensure exponential/jitter maths never panics. |
| **Concurrency fuzz** | Run `go test -race -fuzz .` on the worker manager and transport client. |

### Documentation & Deprecations

There are prominent `Deprecated` comments (e.g. `ServiceRetryCoordinator`). Keep a living removal schedule (in `docs/DEPRECATIONS.md`) and prune periodically; otherwise the codebase accretes legacy layers.

### Security Posture

- **Input validation** – Transport client parses raw JSON into `map[string]any`; switch to typed structs plus `jsonschema` or `go-playground/validator` where feasible.
- **Shell-exec utilities** (e.g. Docker tagging) should sanitise refs and quote args—prefer Go Docker client APIs over `exec.Command`.
- **Secrets redaction** – Make sure RichError Context never logs environment variables or credentials.

---

## ✅ Quick Wins Checklist

- [ ] Replace `context.Background()` placeholders with propagated ctx
- [ ] Remove deprecated `services/*` wrappers
- [ ] Add compile-time type-safe tool registry
- [ ] Enforce layer boundaries in CI
- [ ] Stop background worker ticker leak
- [ ] Establish a single logging framework
- [ ] Create deprecation removal tracking document (`docs/DEPRECATIONS.md`)

---

## 🛣️ Longer-term Roadmap

- **Implement real SessionManager** (currently a no-op stub) to unlock multi-tenant isolation
- **Introduce OpenTelemetry spans** and metrics exporters
- **Migrate reflection utilities** to generics + codegen for zero-alloc hot paths
- **Formalise error code taxonomy** and expose it in API docs for clients

---

## 💭 Closing Thoughts

The `pkg/mcp` module shows thoughtful architecture and a strong focus on quality gates, but it still carries "scaffolding" typical of an evolving codebase. Addressing the items above will tighten correctness, performance and maintainability while preserving the flexible design you already have.

Feel free to ask for deep dives on any specific file or subsystem—happy to help.
