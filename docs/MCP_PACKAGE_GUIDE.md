# Container Kit MCP Package Organization Guide

> **📖 For Complete Architecture**: See [THREE_LAYER_ARCHITECTURE.md](./THREE_LAYER_ARCHITECTURE.md) for the full architectural overview and current package structure.

## Package Organization Principles

The Container Kit MCP package follows a **three-layer architecture** with clear separation of concerns as defined in [ADR-001](./architecture/adr/2025-07-07-three-context-architecture.md):

### Domain Layer (Pure Business Logic)
- **domain/**: Business rules and entities with no dependencies
  - `/containerization/`: Core containerization domain models
    - `/analyze/`: Repository analysis entities and rules
    - `/build/`: Build operation entities and rules
    - `/deploy/`: Deployment entities and rules
    - `/scan/`: Security scanning entities and rules
  - `/errors/`: Unified rich error system (ADR-004)
    - `/codes/`: Centralized error code definitions
    - Rich error context and suggestions
  - `/security/`: Security policies and validation (ADR-005)
    - Tag-based validation DSL
    - Security scanning policies
  - `/session/`: Session entities and rules
  - `/config/`: Configuration entities and validation
  - `/types/`: Core domain types
  - `/internal/`: Shared domain utilities

### Application Layer (Orchestration)
- **application/**: Coordinates domain logic and external integrations
  - `/api/`: Canonical interface definitions (single source of truth)
    - All public interfaces for tools, registry, session, and workflow
    - No implementation code, only contracts
  - `/core/`: Server lifecycle & registry management
    - MCP server implementation
    - Tool registry and orchestration
    - Service container integration
  - `/commands/`: Consolidated command implementations
    - `analyze_consolidated.go`: Repository analysis tool
    - `build_consolidated.go`: Docker build operations
    - `deploy_consolidated.go`: Kubernetes deployment
    - `scan_consolidated.go`: Security scanning
  - `/orchestration/`: Tool coordination & workflow execution
    - Pipeline management
    - Background workers
    - Atomic operations
  - `/services/`: Service interfaces for dependency injection (ADR-006)
    - Service container pattern
    - Dependency injection interfaces
  - `/state/`: Application state management
    - Session state coordination
    - Context enrichment
  - `/workflows/`: Workflow management
    - Multi-step operations
    - Job execution service
  - `/internal/`: Internal application implementations
    - Conversation handling
    - Runtime management
    - Retry coordination

### Infrastructure Layer (External Integrations)
- **infra/**: External system integrations and adapters
  - `/transport/`: MCP protocol transports (stdio, HTTP)
    - Protocol implementation
    - Request/response handling
    - Error handling
  - `/persistence/`: BoltDB storage layer
    - Session persistence
    - State management
  - `/templates/`: YAML templates (ADR-002)
    - Kubernetes manifest templates
    - Dockerfile templates
    - Template rendering with `go:embed`
  - `/docker/`: Docker client integration
    - Build operations
    - Image management
  - `/k8s/`: Kubernetes integration
    - Manifest generation
    - Deployment operations
  - `/telemetry/`: Monitoring and observability
    - Metrics collection
    - Tracing integration

## Import Guidelines

### Three-Layer Architecture Rules
All imports must follow the three-layer dependency rules:

✅ **Good Examples**:
```go
import "github.com/Azure/container-kit/pkg/mcp/application/api"
import "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
import "github.com/Azure/container-kit/pkg/mcp/infra/transport"
```

❌ **Bad Examples**:
```go
import "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build/strategies/deep/nested"
import "github.com/Azure/container-kit/pkg/mcp/infra/transport/http/handlers/middleware/auth"
```

### Dependency Rules

#### Domain Layer
- **Purpose**: Pure business logic and entities
- **Dependencies**: None (no external dependencies)
- **Used by**: Application and Infrastructure layers
- **Forbidden**: Any imports from application/ or infra/

#### Application Layer
- **Purpose**: Orchestration and coordination
- **Dependencies**: Domain layer only
- **Used by**: Infrastructure layer and external consumers
- **Forbidden**: Imports from infra/ (use dependency injection)

#### Infrastructure Layer
- **Purpose**: External integrations and adapters
- **Dependencies**: Domain and Application layers
- **Used by**: External consumers
- **Forbidden**: No restrictions (top level of dependency hierarchy)

## Architecture Boundaries

Boundaries are enforced by architectural decision records (ADRs):

### Layer Rules (ADR-001)
1. **Domain**: No dependencies (pure business logic)
2. **Application**: Only depends on Domain layer
3. **Infrastructure**: Depends on Domain and Application layers
4. **No circular dependencies**: Strict enforcement
5. **Service container**: Manual dependency injection (ADR-006)

### Enforcement
- Architectural review in code reviews
- ADR compliance checking
- Clear layer separation
- Service container pattern for dependency injection

## Current Architecture Structure

### Three-Layer Architecture (Final)
```
pkg/mcp/
├── domain/              # Business logic (no dependencies)
│   ├── containerization/
│   │   ├── analyze/
│   │   ├── build/
│   │   ├── deploy/
│   │   └── scan/
│   ├── errors/          # Rich error system (ADR-004)
│   │   └── codes/
│   ├── security/        # Tag-based validation (ADR-005)
│   ├── session/
│   ├── config/
│   └── internal/
├── application/         # Orchestration (depends on domain)
│   ├── api/            # Canonical interfaces
│   ├── core/           # Server & registry
│   ├── commands/       # Consolidated tools
│   ├── orchestration/  # Pipeline management
│   ├── services/       # Service container (ADR-006)
│   ├── state/          # State management
│   ├── workflows/      # Multi-step operations
│   └── internal/       # Internal implementations
└── infra/              # External integrations (depends on both)
    ├── transport/      # MCP protocol
    ├── persistence/    # BoltDB storage
    ├── templates/      # YAML templates (ADR-002)
    ├── docker/         # Docker integration
    ├── k8s/            # Kubernetes integration
    └── telemetry/      # Monitoring
```

### Key Patterns
- **Service Container**: Manual dependency injection (ADR-006)
- **Rich Errors**: Unified error system with context (ADR-004)
- **Tag Validation**: Struct tag-based validation DSL (ADR-005)
- **Embedded Templates**: YAML templates with go:embed (ADR-002)
- **Consolidated Commands**: Single files replace multiple tool packages

## Quality Standards

### Code Organization
- **File size**: ≤800 lines per file
- **Package focus**: Single responsibility per package
- **Interface segregation**: Small, focused interfaces
- **Dependency injection**: Manual DI through service containers

### Architecture Compliance
- **Import depth**: ≤3 levels maximum
- **Circular dependencies**: Zero tolerance
- **Layer violations**: Automatically detected
- **Boundary enforcement**: CI/CD integration

### Performance
- **Build time**: Optimized dependency graph
- **Import resolution**: Faster with shallow paths
- **Compile time**: Reduced with clear boundaries
- **Runtime**: No overhead from architecture

## Best Practices

### Adding New Features
1. Identify the appropriate package based on functionality
2. Follow existing patterns within that package
3. Use interfaces from api/ package
4. Validate boundaries with check-boundaries tool
5. Keep imports shallow (≤3 levels)

### Refactoring Existing Code
1. Move code to appropriate package
2. Update imports throughout codebase
3. Validate no circular dependencies
4. Run boundary checks
5. Update tests to match new structure

### Testing
1. Unit tests co-located with code
2. Integration tests in appropriate package
3. Use interfaces for mocking
4. Test boundary compliance in CI

## Troubleshooting

### Common Issues

#### Import Cycle Detected
**Problem**: Circular dependency between packages
**Solution**:
- Use interfaces from api/ package
- Move shared code to internal/
- Refactor to break the cycle

#### Boundary Violation
**Problem**: Package depends on forbidden package
**Solution**:
- Check allowed dependencies for package
- Use API interfaces instead of direct imports
- Move code if in wrong package

#### Deep Import Path
**Problem**: Import path exceeds 3 levels
**Solution**:
- Flatten directory structure
- Move to appropriate top-level package
- Consolidate related functionality

## References

- [Three-Layer Architecture](./THREE_LAYER_ARCHITECTURE.md)
- [Architecture Decision Records](./architecture/adr/)
- [Tool Development Guide](./ADDING_NEW_TOOLS.md)
- [Error Handling Guide](./ERROR_HANDLING_GUIDE.md)
- [Service Container Pattern](./architecture/adr/2025-01-07-manual-dependency-injection.md)
- [Quality Standards](./QUALITY_STANDARDS.md)
