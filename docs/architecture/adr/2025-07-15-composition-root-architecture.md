# ADR-008: Composition Root Architecture

## Status
Accepted

## Context
The Container Kit codebase implements a clean separation of dependency injection from business logic to ensure:

1. **Clear Dependencies**: All dependency creation is centralized in the composition root
2. **Testable Components**: Each layer can be tested independently with mock providers
3. **Lifecycle Management**: Clear separation between composition and business logic
4. **Clean Architecture Compliance**: Dependency injection remains outside business layers

The architecture maintains clear boundaries by:
- `pkg/mcp/composition/` - Contains all dependency injection and wiring
- Layer-specific `providers.go` files with focused provider functions
- Centralized composition logic outside the 4-layer clean architecture

## Decision
The system implements a **Composition Root** pattern that separates dependency injection from business logic:

### Architecture Overview
```
pkg/mcp/
├── composition/           # NEW: Composition root outside other layers
│   ├── providers.go       # Application-wide provider functions
│   ├── server.go          # Main server composition
│   └── wire_gen.go        # Wire-generated dependency graph
├── api/                   # Interface definitions only
├── application/           # Business logic
├── domain/                # Core domain logic
└── infrastructure/        # Technical implementations
```

### Key Components

1. **Composition Root** (`pkg/mcp/composition/`)
   - Lives outside the 4-layer clean architecture
   - Contains all dependency injection logic
   - Provides clear entry points for different configurations

2. **Layer-Specific Providers** (e.g., `pkg/mcp/infrastructure/*/providers.go`)
   - Each infrastructure package has its own provider functions
   - Focused on specific domain areas (AI/ML, messaging, orchestration, etc.)
   - Clean separation of concerns

3. **Wire Integration**
   - Uses Google Wire for compile-time dependency injection
   - Generated code in `wire_gen.go`
   - Type-safe dependency resolution

### Provider Pattern
```go
// Infrastructure layer providers
func ProvideOrchestrationServices(...) OrchestrationServices
func ProvideMessagingServices(...) MessagingServices
func ProvideAIMLServices(...) AIMLServices

// Composition root
func ProvideApplication(...) (*Application, error)
func ProvideServer(...) (*Server, error)
```

## Consequences

### Positive
- **Clear Separation**: Business logic is completely separated from dependency injection
- **Testability**: Each layer can be tested independently with mock providers
- **Maintainability**: Dependency changes are localized to provider functions
- **Type Safety**: Wire provides compile-time verification of dependency graphs
- **Clean Architecture**: Maintains strict layer boundaries
- **Flexibility**: Easy to swap implementations for different environments (test, production)

### Negative
- **Additional Complexity**: Requires understanding of dependency injection patterns
- **Wire Learning Curve**: Developers need to understand Wire syntax
- **Build-time Generation**: Wire code generation adds a build step

### Implementation Impact
1. **Centralized Location**: All dependency injection logic resides in `composition/`
2. **Provider Organization**: Each infrastructure package contains a `providers.go` file
3. **Clean Business Logic**: Application and domain layers focus solely on business logic
4. **Simplified Testing**: Test setup uses focused mock providers

## Implementation Details

### Current Implementation (Composition Root)
```go
// In composition root
//go:generate wire
func ProvideApplication(config Config) (*Application, error) {
    wire.Build(
        // Infrastructure providers
        orchestration.ProvideOrchestrationServices,
        messaging.ProvideMessagingServices,
        // ... etc
    )
    return &Application{}, nil
}
```

## Compliance
This ADR aligns with:
- **Clean Architecture**: Dependency injection is outside business layers
- **Single Responsibility**: Each provider has one focused responsibility
- **Dependency Inversion**: High-level modules don't depend on low-level modules
- **Open/Closed**: Easy to extend with new providers without modifying existing code

## Alternative Considered
**Service Locator Pattern**: The composition root approach provides better compile-time safety and clearer dependency graphs compared to a service locator pattern.

## References
- Martin Fowler's "Inversion of Control Containers and the Dependency Injection pattern"
- Robert C. Martin's "Clean Architecture"
- Google Wire documentation
- Performance impact: No runtime overhead, compile-time generation only