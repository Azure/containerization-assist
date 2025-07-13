# ADR-006: Four-Layer MCP Architecture

**Date**: 2025-07-12  
**Status**: Accepted  
**Deciders**: Development Team  

## Context

The MCP package structure has grown organically and accumulated directory sprawl with scattered functionality across multiple packages. The current structure includes:

- Over-abstracted API layer with 1,065 lines of interfaces
- Duplicated analysis logic between `infrastructure/analysis/` and `infrastructure/steps/analyze.go`
- Mixed abstraction levels in infrastructure packages
- Domain concepts like progress tracking and elicitation scattered at the root level
- Unclear boundaries between business logic and technical implementation

The CLAUDE.md file references a simplified architecture goal but the current structure doesn't align with clean architecture principles or the workflow-focused design.

## Decision

We will restructure the `pkg/mcp/` package to follow a clean 4-layer Domain-Driven Design architecture:

### Layer Structure

```
pkg/mcp/
├── api/                    # Interface definitions and contracts
├── application/            # Application services and orchestration
├── domain/                # Business logic and workflows
└── infrastructure/        # Technical implementations and external concerns
```

### Layer Responsibilities

#### API Layer (`api/`)
- Essential interfaces and contracts between layers
- MCP tool interfaces
- Streamlined from current over-abstracted design
- Focus on core workflow and server interfaces only

#### Application Layer (`application/`)
- MCP server orchestration and lifecycle management
- Chat mode integration and session management
- Application services that coordinate domain logic
- Request/response handling and MCP protocol implementation

#### Domain Layer (`domain/`)
- Core business logic including the containerization workflow
- Domain entities, value objects, and business rules
- Rich error handling system (existing, well-designed)
- Progress tracking (business concept, not technical concern)
- User elicitation and input gathering (business process)

#### Infrastructure Layer (`infrastructure/`)
- Technical implementations of domain interfaces
- External service integrations (AI, Docker, Kubernetes)
- Workflow step implementations (analyze, build, deploy)
- MCP-specific infrastructure (prompts, resources, sampling)
- Security utilities and retry mechanisms

### Package Movements

- `progress/` → `domain/progress/` (business concept)
- `elicitation/` → `domain/elicitation/` (domain process)
- `security/` → `infrastructure/security/` (technical utility)
- `sampling/`, `prompts/`, `resources/` → `infrastructure/` (technical integrations)
- Consolidate analysis logic within `infrastructure/analysis/`
- Streamline `api/interfaces.go` to essential contracts only

## Rationale

### Benefits

1. **Clean Architecture**: Follows established DDD and clean architecture patterns
2. **Clear Boundaries**: Each layer has well-defined responsibilities and dependencies
3. **Workflow-Focused**: Domain layer emphasizes the core containerization workflow
4. **Reduced Complexity**: Consolidates scattered functionality and removes over-abstraction
5. **Maintainable**: Familiar architecture pattern for new team members
6. **Testable**: Clear separation makes unit testing and mocking easier

### Alignment with Goals

- **Simplified Architecture**: Reduces complexity while maintaining functionality
- **Workflow-Driven**: Domain layer becomes the heart of the workflow-focused design
- **25 Core Files Goal**: Consolidation reduces file count and complexity
- **Clean Dependencies**: Infrastructure depends on domain, not vice versa

## Consequences

### Positive

- **Improved Maintainability**: Clear separation of concerns
- **Better Testability**: Domain logic isolated from infrastructure concerns
- **Reduced Coupling**: Clean dependency directions between layers
- **Easier Onboarding**: Standard architecture pattern

### Negative

- **Migration Effort**: Requires moving packages and updating imports
- **Temporary Disruption**: Short-term impact on development during migration
- **Import Changes**: Some packages change location requiring import updates

## Implementation Plan

1. **Phase 1**: Create new package structure and move domain concepts
2. **Phase 2**: Consolidate analysis logic and streamline infrastructure
3. **Phase 3**: Reduce API layer complexity and over-abstraction
4. **Phase 4**: Update documentation and import statements
5. **Phase 5**: Verify tests pass and functionality is preserved

## Related Decisions

- Builds upon ADR-005 (Single Workflow Architecture)
- Consolidates the error handling system from ADR-004
- Maintains the manual dependency injection approach from ADR-003

## References

- [Clean Architecture by Robert Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Domain-Driven Design patterns](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- Container Kit CLAUDE.md simplified architecture goals