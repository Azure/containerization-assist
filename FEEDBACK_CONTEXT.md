# Feedback Context: Specific Code References

This document provides concrete file paths and line numbers corresponding to the architectural feedback in feedback.md.

## 1. Deep Package Nesting (>3 levels)

**Current Examples:**
- `/pkg/mcp/application/internal/conversation/` - 5 levels deep
- `/pkg/mcp/domain/containerization/analyze/` - 5 levels deep
- `/pkg/mcp/infra/templates/components/` - 5 levels deep
- `/pkg/mcp/domain/security/validation/` - 5 levels deep

**Specific Files Affected:**
- `pkg/mcp/application/internal/conversation/validators.go`
- `pkg/mcp/application/internal/retry/coordinator.go`
- `pkg/mcp/application/internal/runtime/registry.go`
- `pkg/mcp/infra/internal/migration/analysis_patterns.go`

## 2. Duplicate Retry/Coordinator Logic

**Two Parallel Implementations:**

### Implementation 1: Application Layer
- `pkg/mcp/application/internal/retry/coordinator.go` (lines 1-30)
  - Defines `BackoffStrategy`, `Policy` struct
  - Implements retry logic with exponential backoff

### Implementation 2: Infrastructure Layer
- `pkg/mcp/infra/retry/coordinator.go` (lines 1-30)
  - Identical `BackoffStrategy`, `Policy` struct definitions
  - Duplicate implementation of the same retry patterns

**Additional Retry Logic:**
- `pkg/mcp/application/orchestration/pipeline/simple_retry.go`
- `pkg/mcp/application/internal/conversation/intelligent_retry_system.go`
- `pkg/mcp/application/internal/conversation/retry_service.go`

## 3. Multiple Tool Registry Implementations

**Three Separate Registries:**

### Registry 1: Core Registry
- `pkg/mcp/application/core/tool_registry.go` (lines 11-23)
  - Interface: `CoreToolRegistry`
  - Marked as deprecated but still used

### Registry 2: Commands Registry
- `pkg/mcp/application/commands/tool_registry.go` (lines 30-40)
  - Type: `UnifiedRegistry` struct
  - Has its own metadata and config system

### Registry 3: Runtime Registry
- `pkg/mcp/application/internal/runtime/registry.go`
  - Used for auto-registration
  - Uses reflection and `interface{}`

**Additional Registry Files:**
- `pkg/mcp/application/commands/command_registry.go`
- `pkg/mcp/application/orchestration/pipeline/worker_registry.go`

## 4. Manual Dependency Injection Patterns

**Service Container Implementation:**
- `pkg/mcp/application/services/container.go` (lines 14-39)
  - `DefaultServiceContainer` struct with 17+ dependencies
  - Manual wiring of all services

**Manual Wiring Examples:**
- `pkg/mcp/application/core/server_impl.go` - Server construction
- `pkg/mcp/application/orchestration/pipeline/services.go` - Pipeline services
- `pkg/mcp/infra/infra.go` - Infrastructure wiring

**Generated Tool Wiring:**
- `cmd/mcp-schema-gen/templates/` - Templates for manual tool registration
- Each tool requires manual registration code

## 5. Mixed Error Handling Patterns

**Statistics:** 622 occurrences of `fmt.Errorf` across 83 files despite RichError system

**Examples of Mixed Usage:**

### Domain Layer (Should use RichError only):
- `pkg/mcp/domain/config/loader.go` (line 15) - Uses fmt.Errorf
- `pkg/mcp/domain/tools/schema.go` (line 4) - Uses fmt.Errorf
- `pkg/mcp/domain/internal/utils/assertions.go` (line 11) - Uses fmt.Errorf

### Application Layer:
- `pkg/mcp/application/orchestration/pipeline/docker_operations.go` (28 occurrences)
- `pkg/mcp/application/orchestration/pipeline/kubernetes_operations.go` (17 occurrences)
- `pkg/mcp/application/orchestration/pipeline/background_workers.go` (20 occurrences)

### RichError Implementation:
- `pkg/mcp/domain/errors/rich.go` - The unified error system
- `pkg/mcp/domain/errors/factories.go` (lines 44) - Builder pattern implementation

## 6. Orchestration Pipeline Variants

**Multiple Pipeline Implementations in `/pkg/mcp/application/orchestration/pipeline/`:**

### Variant 1: Legacy Pipeline
- `pipeline_legacy.go`
- `pipeline_legacy_methods.go`
- `pipeline_legacy_methods_test.go`

### Variant 2: Atomic Framework
- `atomic_framework.go`

### Variant 3: Simple Operations
- `simple_operations.go`
- `simple_retry.go`
- `simple_cache.go`

### Core Pipeline Service
- `pipeline_service.go`
- `config.go` (21 error handling instances)

## 7. Context Handling Issues

**Problematic Context Usage:**
- `pkg/mcp/application/core/server_impl.go` - Uses `context.Background()` in multiple places
- Transport layer drops cancellation context
- No timeout propagation from MCP protocol layer

## 8. Interface{} Usage (Should Use Generics)

**Files Using interface{} in Go 1.22+ Code:**
- `pkg/mcp/application/internal/runtime/registry.go` - Tool registration
- `pkg/mcp/application/internal/runtime/auto_registration.go` - Reflection-based registration
- Various orchestration helpers use `interface{}` for tool factories

## 9. Deprecated Code Still Present

**Marked as Deprecated but Not Removed:**
- `pkg/mcp/application/core/tool_registry.go` (line 10) - "Deprecated: Use services.ToolRegistry"
- `pkg/mcp/application/services/interfaces.go` - Multiple deprecated service definitions
- Parallel API implementations in `application/services/` vs `application/api/`

## 10. Validation Duplication

**Multiple Validator Implementations:**
- `pkg/mcp/application/internal/conversation/validators.go`
- `pkg/mcp/domain/security/validators/`
- `pkg/mcp/domain/security/validation/`
- `pkg/mcp/application/orchestration/pipeline/basic_validator.go`
- `pkg/mcp/application/orchestration/pipeline/production_validation.go`

## Architecture Violations

**Domain Layer Importing Application:**
- `pkg/mcp/domain/internal/` is imported by other layers
- Defeats "pure domain" intent

**Circular Dependencies:**
- Application and Infrastructure layers have partial mesh imports
- Internal packages expose public interfaces

## Command Routing Complexity

**No Unified Command Routing:**
- Commands are handled through multiple mechanisms
- No central routing table or dispatcher
- Each command type has its own registration pattern

## Background Worker Issues

**Worker Management Problems:**
- `pkg/mcp/application/orchestration/pipeline/background_workers.go`
- No ticker cleanup on shutdown
- Silent overwrite on duplicate registration
- No dependency ordering for shutdown

## Template/Code Generation

**Extensive Boilerplate Generation:**
- `cmd/mcp-schema-gen/templates/` contains repetitive templates
- ~80% identical code for each new tool
- Hand-edited copies mixed with generated code

## Metrics and Telemetry Gaps

**Limited Observability:**
- Basic Prometheus metrics only
- No distributed tracing despite OpenTelemetry mentions
- No span propagation through layers

This context document provides specific, actionable references for each architectural concern raised in the feedback, enabling targeted refactoring efforts.
