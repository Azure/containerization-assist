# Breaking Changes in v2.0

This document provides a comprehensive list of breaking changes introduced in Container Kit v2.0 with the unified interface reorganization.

## Summary of Breaking Changes

Container Kit v2.0 introduces a unified interface system that significantly improves the architecture but requires migration for existing users. The changes affect:

1. **Interface definitions and locations**
2. **Method signatures**
3. **Import paths**
4. **Error handling patterns**
5. **Tool registration mechanisms**

## Detailed Breaking Changes

### 1. Interface Consolidation

#### Removed Interfaces

The following interfaces have been removed or consolidated:

| Removed Interface | Replacement | Location |
|-------------------|-------------|----------|
| `tools.Runner` | `mcp.Tool` | `pkg/mcp/interfaces.go` |
| `tools.Executor` | `mcp.Tool` | `pkg/mcp/interfaces.go` |
| `common.Validator` | `mcp.Tool.Validate()` | Method on Tool interface |
| `session.Manager` | `mcp.Session` | `pkg/mcp/interfaces.go` |
| `transport.Handler` | `mcp.RequestHandler` | `pkg/mcp/interfaces.go` |

#### Renamed Interfaces

| Old Name | New Name | Reason |
|----------|----------|--------|
| `DockerValidator` | `DockerfileValidator` | Avoid naming conflicts |
| `RuntimeValidator` | `RuntimeAnalyzer` | Clearer purpose |
| `Handler` | `RequestHandler` | More specific naming |

### 2. Method Signature Changes

#### Tool Interface Methods

**Old Tool Interface:**
```go
type Tool interface {
    GetName() string
    GetDescription() string
    Run(ctx context.Context, params RunParams) error
    GetArgs() ToolArgs
}
```

**New Tool Interface:**
```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

#### Session Interface Methods

**Old:**
```go
type Session interface {
    ID() string
    Get(key string) interface{}
    Set(key string, value interface{})
    Delete(key string)
}
```

**New:**
```go
type Session interface {
    GetID() string
    GetData(key string) (interface{}, error)
    SetData(key string, value interface{}) error
    DeleteData(key string) error
    ListKeys() ([]string, error)
    Clear() error
}
```

### 3. Import Path Changes

#### Consolidated Imports

**Before:**
```go
import (
    "github.com/Azure/container-copilot/pkg/mcp/tools"
    "github.com/Azure/container-copilot/pkg/mcp/tools/interfaces"
    "github.com/Azure/container-copilot/pkg/mcp/common"
    "github.com/Azure/container-copilot/pkg/mcp/session"
    "github.com/Azure/container-copilot/pkg/mcp/transport"
)
```

**After:**
```go
import (
    "github.com/Azure/container-copilot/pkg/mcp"
    "github.com/Azure/container-copilot/pkg/mcp/types" // Only for type definitions
)
```

#### Moved Packages

| Old Location | New Location | Contents |
|--------------|--------------|----------|
| `pkg/mcp/tools/build/` | `pkg/mcp/internal/build/` | Build tools |
| `pkg/mcp/tools/deploy/` | `pkg/mcp/internal/deploy/` | Deploy tools |
| `pkg/mcp/tools/scan/` | `pkg/mcp/internal/scan/` | Scan tools |
| `pkg/mcp/tools/analyze/` | `pkg/mcp/internal/analyze/` | Analyze tools |

### 4. Type Changes

#### Return Types

Tools now return results instead of just errors:

**Old:**
```go
func (t *Tool) Run(ctx context.Context, params RunParams) error
```

**New:**
```go
func (t *Tool) Execute(ctx context.Context, args interface{}) (interface{}, error)
```

#### Metadata Structure

**Old:**
```go
type ToolInfo struct {
    Name        string
    Description string
    Args        []ArgSpec
}
```

**New:**
```go
type ToolMetadata struct {
    Name         string
    Description  string
    Version      string
    Category     string
    Capabilities []string
    Requirements []string
    Parameters   map[string]string
    Examples     []ToolExample
}
```

### 5. Error Handling Changes

#### Error Types

**Old:**
```go
// Simple error strings
return fmt.Errorf("operation failed: %v", err)
```

**New:**
```go
// Rich error types with context
return types.NewRichError(
    "OPERATION_FAILED",
    "Operation failed during execution",
    err,
).WithContext(map[string]interface{}{
    "tool": toolName,
    "phase": "execution",
})
```

#### Error Codes

New standardized error codes:

| Error Code | Description | Usage |
|------------|-------------|-------|
| `INVALID_ARGS` | Invalid arguments provided | Validation failures |
| `EXECUTION_FAILED` | Tool execution failed | Runtime errors |
| `DEPENDENCY_MISSING` | Required dependency not found | Missing prerequisites |
| `TIMEOUT` | Operation timed out | Long-running operations |
| `RESOURCE_NOT_FOUND` | Resource not found | Missing files/configs |

### 6. Registration Changes

#### Automatic Registration

**Old (Manual):**
```go
func init() {
    tools.Register("my_tool", &MyTool{})
}
```

**New (Automatic):**
```go
// Add annotations for code generation
// +tool:name=my_tool
// +tool:category=build
// +tool:description=Tool description
type MyTool struct {
    // implementation
}
```

Run `go generate ./...` to register tools automatically.

### 7. Configuration Changes

#### Tool Configuration

**Old:**
```go
// Global configuration
var config = LoadConfig()

func (t *Tool) Run(ctx context.Context, params RunParams) error {
    value := config.Get("key")
}
```

**New:**
```go
// Injected configuration
type MyTool struct {
    config *Config
}

func NewMyTool(config *Config) *MyTool {
    return &MyTool{config: config}
}
```

### 8. Testing Changes

#### Mock Interfaces

**Old:**
```go
type MockTool struct {
    mock.Mock
}

func (m *MockTool) Run(ctx context.Context, params RunParams) error {
    args := m.Called(ctx, params)
    return args.Error(0)
}
```

**New:**
```go
type MockTool struct {
    ExecuteFunc    func(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadataFunc func() ToolMetadata
    ValidateFunc   func(ctx context.Context, args interface{}) error
}

func (m *MockTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(ctx, args)
    }
    return nil, nil
}
```

### 9. Removed Features

The following features have been removed:

1. **Global Tool Registry**: Replaced with orchestrator-based registration
2. **Tool Aliases**: Use single canonical names
3. **Legacy Validators**: Integrated into Tool.Validate() method
4. **Sync/Async Modes**: All tools are now synchronous (use goroutines internally if needed)

### 10. Behavioral Changes

#### Session Handling

- Sessions now require explicit cleanup
- Session data is persisted by default
- Session IDs must be provided by the client

#### Progress Reporting

- Progress is now reported through a dedicated interface
- Progress updates are atomic and ordered
- Progress can be queried after completion

## Migration Timeline

| Phase | Duration | Description |
|-------|----------|-------------|
| Deprecation Notice | 3 months | v1.9 includes deprecation warnings |
| Migration Period | 6 months | Both v1.x and v2.0 APIs supported |
| v1.x End of Life | 9 months | v1.x APIs removed |

## Compatibility Notes

### Backward Compatibility

- v2.0 does NOT maintain backward compatibility with v1.x
- A compatibility adapter is available for gradual migration
- See `pkg/mcp/compat/` for the adapter implementation

### Forward Compatibility

- The new interface system is designed for stability
- Future changes will use interface composition
- Semantic versioning will be strictly followed

## Impact by User Type

### Tool Developers

**High Impact** - Must migrate all tools to new interface:
- Update method signatures
- Implement new required methods
- Change error handling
- Update tests

### API Consumers

**Medium Impact** - Must update client code:
- Change import paths
- Update method calls
- Handle new return types

### MCP Server Users

**Low Impact** - Mostly transparent:
- Update server version
- Some configuration changes
- New features available

## Getting Help

For assistance with migration:

1. See the [Migration Guide](migration-guide.md) for step-by-step instructions
2. Check [examples/](../examples/) for reference implementations
3. Use the compatibility adapter for gradual migration
4. Report issues at https://github.com/Azure/container-copilot/issues

## Appendix: Quick Reference

### Essential Changes Checklist

- [ ] Update all import paths
- [ ] Implement Tool.Execute() instead of Tool.Run()
- [ ] Add Tool.GetMetadata() method
- [ ] Add Tool.Validate() method
- [ ] Change error handling to use RichError
- [ ] Update tool registration to use annotations
- [ ] Modify tests for new interfaces
- [ ] Test with interface validator
- [ ] Update documentation

### Command Reference

```bash
# Validate interfaces
go run tools/validate-interfaces/main.go

# Generate tool registrations
go generate ./...

# Run migration helper
go run tools/migrate-v1-to-v2/main.go --check

# Test compatibility
go test -tags=compat ./...
```