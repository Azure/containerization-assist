# Container Kit MCP Package Organization Guide

> **📖 For Complete Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full architectural overview and current package structure.

## Package Organization Principles

The Container Kit MCP package follows a layered architecture with clear separation of concerns:

### API Layer
- **api/**: Interface definitions and contracts (single source of truth)
  - All public interfaces for tools, registry, session, and workflow
  - No implementation code, only contracts
  - Zero dependencies on other packages

### Core Layer
- **core/**: Server lifecycle, registry management, and coordination
  - `/registry`: Tool registry implementation
  - `/state`: State management
  - `/types`: Core type definitions
  - Depends only on API and session packages

- **tools/**: Container operations (analyze, build, deploy, scan)
  - `/analyze`: Repository analysis and Dockerfile generation
  - `/build`: Docker build operations with AI-powered fixing
  - `/deploy`: Kubernetes manifest generation and deployment
  - `/scan`: Security vulnerability scanning
  - `/detectors`: Database and framework detection
  - Independent of core, uses API contracts

- **session/**: Session management and persistence
  - Session lifecycle (create, get, delete)
  - Workspace management
  - State persistence
  - Used by tools and workflow

- **workflow/**: Multi-step operation orchestration
  - Workflow engine for complex operations
  - Tool coordination
  - Atomic operations with rollback
  - Depends on tools through API

### Infrastructure Layer
- **transport/**: MCP protocol transports (stdio, HTTP)
  - Protocol implementation
  - Request/response handling
  - Error handling
  - Depends on core for server integration

- **storage/**: Persistence implementations (BoltDB)
  - `/boltdb`: BoltDB implementation
  - Key-value storage
  - Session persistence
  - Minimal dependencies

- **security/**: Validation and security scanning
  - `/validation`: Input validation and sanitization
  - `/scanner`: Security vulnerability scanning integration
  - Used by tools for validation
  - Independent of other packages

- **templates/**: Kubernetes manifest templates
  - YAML templates for K8s resources
  - Template rendering
  - Pure data/configuration
  - No code dependencies

- **internal/**: Implementation details and utilities
  - `/errors`: Unified error system
  - `/types`: Shared type definitions
  - `/utils`: Utility functions
  - `/common`: Common functionality
  - `/retry`: Retry mechanisms
  - `/logging`: Logging utilities
  - No dependencies on higher layers

## Import Guidelines

### Maximum Depth: 3 Levels
All imports must follow the pattern: `pkg/mcp/package/[subpackage]`

✅ **Good Examples**:
```go
import "github.com/Azure/container-kit/pkg/mcp/api"
import "github.com/Azure/container-kit/pkg/mcp/tools/build"
import "github.com/Azure/container-kit/pkg/mcp/core/registry"
```

❌ **Bad Examples**:
```go
import "github.com/Azure/container-kit/pkg/mcp/application/internal/pipeline/distributed"
import "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build/strategies"
```

### Dependency Rules

#### API Package
- **Purpose**: Interface definitions only
- **Dependencies**: None (interfaces only)
- **Used by**: All other packages

#### Core Package
- **Purpose**: Server lifecycle and coordination
- **Dependencies**: api/, session/
- **Used by**: transport/, workflow/
- **Forbidden**: tools/ (use API interfaces)

#### Tools Package
- **Purpose**: Container operations implementation
- **Dependencies**: api/, session/, security/
- **Used by**: workflow/
- **Forbidden**: core/ (use API interfaces)

#### Session Package
- **Purpose**: Session and workspace management
- **Dependencies**: api/, storage/
- **Used by**: core/, tools/, workflow/
- **Forbidden**: No circular dependencies

#### Workflow Package
- **Purpose**: Multi-step operation orchestration
- **Dependencies**: api/, tools/, session/
- **Used by**: External consumers
- **Forbidden**: core/ (use API interfaces)

## Architecture Boundaries

Boundaries are enforced by `tools/check-boundaries`:

```bash
# Validate architecture compliance
tools/check-boundaries/check-boundaries -strict ./pkg/mcp
```

### Layer Rules
1. **API**: No dependencies (pure interfaces)
2. **Core**: Only depends on API and session
3. **Tools**: Only depends on API, session, and security
4. **Infrastructure**: Minimal dependencies, no circular refs
5. **Internal**: No dependencies on higher layers

### Enforcement
- Automated boundary checking in CI/CD
- Pre-commit hooks for local validation
- Zero tolerance for violations
- Clear error messages with resolution guidance

## Migration from Old Structure

### Before (86 packages, 5-level depth)
```
pkg/mcp/
├── application/
│   ├── api/
│   ├── core/
│   ├── internal/
│   │   ├── pipeline/
│   │   ├── runtime/
│   │   └── ...
│   ├── orchestration/
│   └── services/
├── domain/
│   ├── containerization/
│   │   ├── analyze/
│   │   ├── build/
│   │   └── ...
│   ├── errors/
│   └── ...
├── infra/
│   ├── transport/
│   ├── persistence/
│   └── ...
└── services/
    └── ...
```

### After (27 packages, ≤3-level depth)
```
pkg/mcp/
├── api/
├── core/
│   ├── registry/
│   ├── state/
│   └── types/
├── tools/
│   ├── analyze/
│   ├── build/
│   ├── deploy/
│   ├── scan/
│   └── detectors/
├── session/
├── workflow/
├── transport/
├── storage/
│   └── boltdb/
├── security/
│   ├── validation/
│   └── scanner/
├── templates/
└── internal/
    ├── errors/
    ├── types/
    ├── utils/
    ├── common/
    ├── retry/
    ├── logging/
    └── processing/
```

### Import Updates
```go
// Old structure
import "github.com/Azure/container-kit/pkg/mcp/application/api"
import "github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
import "github.com/Azure/container-kit/pkg/mcp/infra/transport"

// New structure
import "github.com/Azure/container-kit/pkg/mcp/api"
import "github.com/Azure/container-kit/pkg/mcp/tools/build"
import "github.com/Azure/container-kit/pkg/mcp/transport"
```

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

- [Architecture Decision Records](../architecture/adr/)
- [Migration Guide](./MCP_MIGRATION_GUIDE.md)
- [Boundary Checker Tool](../../tools/check-boundaries/)
- [Quality Gates](../../scripts/quality_gates.sh)
