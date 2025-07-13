# Wire Dependency Injection

This package provides Google Wire-based dependency injection for the Container Kit MCP Server.

## Overview

Google Wire is a compile-time dependency injection tool that generates code to initialize your application's dependencies. It provides several benefits:

- **Compile-time safety**: Errors are caught at compile time, not runtime
- **No reflection**: Generated code is simple and fast
- **Clear dependencies**: Makes the dependency graph explicit
- **Easy testing**: Mock injection is straightforward

## Usage

### Using Wire in Your Application

```go
import (
    "github.com/Azure/container-kit/pkg/mcp/application"
    "github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

func main() {
    logger := slog.Default()
    config := workflow.DefaultServerConfig()
    
    // Create server using Wire
    server, err := application.NewMCPServerWithWire(ctx, logger, &config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start server
    server.Start(ctx)
}
```

### Generating Wire Code

1. Install Wire:
   ```bash
   go install github.com/google/wire/cmd/wire@latest
   ```

2. Generate the dependency injection code:
   ```bash
   go generate ./pkg/mcp/wire
   ```

3. The `wire_gen.go` file will be updated with the generated code.

### Adding New Dependencies

To add a new dependency to the Wire provider set:

1. Add the provider function to `wire.go`:
   ```go
   func provideMyService(logger *slog.Logger) *MyService {
       return &MyService{logger: logger}
   }
   ```

2. Add it to the ProviderSet:
   ```go
   var ProviderSet = wire.NewSet(
       // ... existing providers
       provideMyService,
   )
   ```

3. Update the Dependencies struct if needed
4. Run `go generate ./pkg/mcp/wire` to regenerate

### Testing with Wire

Wire makes it easy to inject mocks for testing:

```go
func TestWithMocks(t *testing.T) {
    mockSession := &MockSessionManager{}
    mockSampling := &MockSamplingClient{}
    
    deps := &application.Dependencies{
        SessionManager: mockSession,
        SamplingClient: mockSampling,
        // ... other dependencies
    }
    
    server := application.NewServer(
        application.WithDependencies(deps),
    )
    
    // Test server behavior
}
```

## Migration Strategy

The codebase supports both manual dependency injection and Wire:

1. **Current**: Manual DI using functional options in `bootstrap.go`
2. **Wire**: Compile-time DI using `wire.go` and `wire_gen.go`
3. **Hybrid**: Both approaches work side-by-side during migration

Use `NewMCPServer` for manual DI or `NewMCPServerWithWire` for Wire-based DI.

## Benefits

1. **Type Safety**: Compile-time checking of dependencies
2. **Performance**: No runtime reflection
3. **Clarity**: Dependencies are explicit in provider functions
4. **Testability**: Easy mock injection
5. **Maintainability**: Adding/removing dependencies is straightforward