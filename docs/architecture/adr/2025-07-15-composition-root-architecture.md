# ADR-008: Composition Root Architecture

## Status
Accepted

## Context
The Container Kit codebase had scattered dependency injection logic across multiple layers, making it difficult to:

1. **Understand Dependencies**: Dependency creation was spread across API layer files
2. **Test Components**: Complex wiring made unit testing difficult
3. **Manage Lifecycle**: No clear separation between composition and business logic
4. **Follow Clean Architecture**: Dependency injection logic violated layer boundaries

The previous approach mixed dependency creation with business logic in files like:
- `pkg/mcp/api/wiring/` - Contained both interfaces and dependency injection
- Various `*_manager.go` files with complex initialization code
- Scattered provider functions across domain and infrastructure layers

## Decision
We will implement a **Composition Root** pattern that separates dependency injection from business logic:

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

### Migration Impact
1. **Moved from API layer**: Dependency injection logic moved from `api/wiring/` to `composition/`
2. **Added Provider Files**: Each infrastructure package now has a `providers.go` file
3. **Simplified Business Logic**: Application and domain layers are cleaner
4. **Enhanced Testing**: Test setup is significantly simplified

## Implementation Details

### Before (Scattered Dependencies)
```go
// In API layer (violation of clean architecture)
func NewWorkflowOrchestrator(...) workflow.WorkflowOrchestrator {
    // Complex dependency setup mixed with interfaces
}
```

### After (Composition Root)
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
**Service Locator Pattern**: Could have used a service locator, but composition root provides better compile-time safety and clearer dependency graphs.

## References
- Martin Fowler's "Inversion of Control Containers and the Dependency Injection pattern"
- Robert C. Martin's "Clean Architecture"
- Google Wire documentation
- Performance impact: No runtime overhead, compile-time generation only