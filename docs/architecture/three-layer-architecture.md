# Three-Layer Architecture

## Overview

Container Kit follows a clean three-layer architecture pattern to ensure proper separation of concerns, maintainability, and testability.

## Architecture Layers

### 1. Domain Layer (`pkg/mcp/domain/`)
The domain layer contains the core business logic and entities. It has no dependencies on other layers.

**Key Components:**
- **config/**: Configuration entities and validation rules
- **containerization/**: Container-related domain logic (analyze, build, deploy, scan)
- **errors/**: Rich error types and error handling
- **security/**: Security policies and validation rules
- **session/**: Session entities and business rules
- **types/**: Core domain types and interfaces
- **workflow/**: Workflow domain logic ⚠️ (directory exists but empty)
- **internal/**: Shared utilities (common, utils, types, constants)

**Files:** 96 Go files

### 2. Application Layer (`pkg/mcp/application/`)
The application layer orchestrates use cases and coordinates between domain and infrastructure.

**Key Components:**
- **api/**: Canonical interface definitions (single source of truth) - 831 lines
- **commands/**: Consolidated command implementations
- **interfaces/**: ⚠️ Compatibility layer (directory exists but empty)
- **core/**: Server lifecycle and registry management
- **conversation/**: Conversation handling and auto-fix helpers
- **knowledge/**: Knowledge management and AI integration
- **orchestration/**: Tool coordination and workflow execution
- **services/**: Service interfaces for dependency injection
- **state/**: Application state management
- **tools/**: Tool implementations
- **workflows/**: Workflow management
- **internal/**: Internal implementations (conversation, retry, runtime)

**Files:** 150 Go files

### 3. Infrastructure Layer (`pkg/mcp/infra/`)
The infrastructure layer handles external integrations and technical concerns.

**Key Components:**
- **persistence/**: BoltDB storage implementation (session_store.go, persistence.go, memory_store.go)
- **templates/**: YAML template resources with go:embed integration
- **transport/**: MCP protocol transports (stdio, HTTP, client implementations)
- **file_access.go**: FileAccessService implementation with security validation and session isolation
- **docker_integration.go** & **docker_operations.go**: Docker client integration
- **k8s_integration.go** & **k8s_operations.go**: Kubernetes client integration
- **internal/**: Infrastructure utilities (logging, migration)

**Files:** 42 Go files

**Note:** Infrastructure components are implemented as individual files rather than separate package directories, providing a clean and efficient organization.

## Dependency Rules

1. **Domain layer** cannot depend on Application or Infrastructure layers
2. **Application layer** can depend on Domain but not Infrastructure
3. **Infrastructure layer** can depend on both Domain and Application

## Dependency Injection

The architecture uses manual dependency injection through interfaces:

### Service Interfaces (Application Layer)
```go
// Template loading interface
type TemplateLoader interface {
    LoadTemplate(name string) (string, error)
    ListTemplates() ([]string, error)
}

// Transport interface
type Transport interface {
    Start(ctx context.Context) error
    Stop() error
    SetHandler(handler interface{})
}
```

### Adapter Implementations (Infrastructure Layer)
```go
// Concrete implementation in infra/adapters/
type TemplateLoaderImpl struct{}

func (t *TemplateLoaderImpl) LoadTemplate(name string) (string, error) {
    return templates.LoadTemplate(name)
}
```

## Configuration

Server configuration includes dependency injection:

```go
type ServerConfig struct {
    // ... other config fields ...

    // Dependency injection
    TransportFactory services.TransportFactory
    TemplateLoader   services.TemplateLoader
}
```

## Migration Summary

The architecture was migrated from a flat structure with 30+ packages to this clean three-layer architecture:

### Before
- Mixed concerns across packages
- Circular dependencies
- Unclear boundaries
- 30+ top-level packages

### After
- Clear separation of concerns
- No circular dependencies
- Well-defined interfaces
- 3 main layers + internal packages
- Total: 326 Go files organized by layer

## Benefits

1. **Testability**: Each layer can be tested independently
2. **Maintainability**: Clear boundaries make changes easier
3. **Flexibility**: Infrastructure can be swapped without affecting business logic
4. **Scalability**: New features fit naturally into the architecture
5. **Understanding**: Clear structure makes onboarding easier

## Validation

Run the architecture validation script to ensure compliance:

```bash
./validate_architecture.sh
```

Expected output:
```
=== Three-Layer Architecture Validation ===

=== Checking Dependency Rules ===
Domain → Application: ✅ OK
Domain → Infrastructure: ✅ OK
Application → Infrastructure: ✅ OK

=== Checking Package Structure ===
Packages outside three-layer structure: ✅ None found
```

## Directory Structure

The pkg/mcp directory contains exactly 3 subdirectories:
- `application/` - Application layer (use cases, orchestration)
- `domain/` - Domain layer (business logic, entities)
- `infra/` - Infrastructure layer (external integrations)

No other directories should exist at this level.
