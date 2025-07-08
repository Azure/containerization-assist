# New Developer Guide - Container Kit MCP Server

Welcome to Container Kit! This guide will help you understand the MCP (Model Context Protocol) portion of the codebase, how it's organized, and why the architecture was designed this way.

## Table of Contents

1. [What is MCP and Why Do We Use It?](#what-is-mcp-and-why-do-we-use-it)
2. [Understanding the Three-Layer Architecture](#understanding-the-three-layer-architecture)
3. [How MCP Tools Work](#how-mcp-tools-work)
4. [Service Container and Dependency Injection](#service-container-and-dependency-injection)
5. [Session Management](#session-management)
6. [Tool Development Workflow](#tool-development-workflow)
7. [Code Organization Rationale](#code-organization-rationale)
8. [Common Patterns and Examples](#common-patterns-and-examples)
9. [Debugging and Troubleshooting](#debugging-and-troubleshooting)
10. [Next Steps](#next-steps)

## What is MCP and Why Do We Use It?

### What is MCP?
MCP (Model Context Protocol) is a standardized protocol for AI assistants to interact with external tools and services. It defines how tools are registered, discovered, and executed in a consistent way.

### Why Container Kit Uses MCP
Container Kit uses MCP because it provides:
- **Standardized Interface**: Consistent way to expose containerization tools to AI assistants
- **Tool Composability**: Tools can be combined to create complex workflows
- **Session Management**: Persistent state across multiple tool executions
- **Type Safety**: Strongly typed interfaces with JSON schema validation
- **Extensibility**: Easy to add new tools and capabilities

### MCP in Action
When you use Container Kit with Claude Desktop or other MCP clients, you're using MCP tools like:
- `analyze_repository_atomic` - Analyze a repository for containerization
- `build_image_atomic` - Build Docker images
- `deploy_kubernetes_atomic` - Deploy to Kubernetes
- `scan_image_security_atomic` - Scan images for vulnerabilities

## Understanding the Architecture

Container Kit follows a **three-context architecture** as defined in [ADR-001](architecture/adr/2025-07-07-three-context-architecture.md). For detailed architecture information, see [ARCHITECTURE.md](./ARCHITECTURE.md).

### Key Architectural Layers

1. **API Layer**: Single source of truth for interfaces (`pkg/mcp/api/`)
2. **Core Layer**: Server lifecycle and registry management (`pkg/mcp/core/`)
3. **Tools Layer**: Domain-specific business logic (`pkg/mcp/tools/`)
4. **Infrastructure Layer**: External integrations (`pkg/mcp/transport/`, `pkg/mcp/storage/`)
5. **Internal Layer**: Shared utilities (`pkg/mcp/internal/`)

### Why This Organization Works

1. **Clear Dependencies**: Application depends on Domain, Domain is independent, Infrastructure depends on Domain
2. **Easy Testing**: Each layer can be tested independently
3. **Maintainability**: Changes in one layer don't ripple through others
4. **Onboarding**: New developers can focus on one layer at a time

## How MCP Tools Work

### Tool Lifecycle
1. **Registration**: Tools register themselves with the MCP server
2. **Discovery**: MCP clients discover available tools and their schemas
3. **Execution**: Clients call tools with structured inputs
4. **Response**: Tools return structured outputs with success/error information

### Atomic vs Conversational Tools
Container Kit provides two types of tools:

#### Atomic Tools
- **Purpose**: Single, focused operations
- **Examples**: `build_image_atomic`, `scan_image_security_atomic`
- **Usage**: Workflow orchestration, scripting, precise control
- **Location**: Individual files in domain packages

#### Conversational Tools
- **Purpose**: Multi-step interactive workflows
- **Examples**: `containerize_repository`, `deploy_application`
- **Usage**: Guided user interactions, complex scenarios
- **Location**: Orchestration layer with conversation handlers

### Tool Interface Structure
Every tool implements the canonical interface from `pkg/mcp/application/api/interfaces.go`:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
}
```

## Service Container and Dependency Injection

### Why We Use Service Containers
Previously, Container Kit used 4 large "Manager" interfaces with 65+ methods total. This created:
- **Testing Complexity**: Hard to mock comprehensive interfaces
- **Interface Bloat**: Managers violated single responsibility principle
- **Hidden Dependencies**: Unclear relationships between components

### Our Solution: Manual Dependency Injection
We replaced the large managers with 8 focused services:

```go
type ServiceContainer interface {
    SessionStore() SessionStore        // Session CRUD operations (4 methods)
    SessionState() SessionState        // State management (4 methods)
    BuildExecutor() BuildExecutor      // Container builds (5 methods)
    ToolRegistry() ToolRegistry        // Tool registration (5 methods)
    WorkflowExecutor() WorkflowExecutor // Workflows (4 methods)
    Scanner() Scanner                  // Security scanning (3 methods)
    ConfigValidator() ConfigValidator  // Configuration validation (4 methods)
    ErrorReporter() ErrorReporter      // Error handling (3 methods)
}
```

### Benefits of This Approach
- **Focused Interfaces**: 3-5 methods per service vs 13-19 per manager
- **Easy Testing**: Mock individual services without complexity
- **Clear Dependencies**: Explicit service injection shows relationships
- **Performance**: No manager/adapter overhead

### How to Use Services in Your Code
```go
// Tool constructor receives specific services it needs
func NewAnalyzeTool(
    sessionStore services.SessionStore,
    configValidator services.ConfigValidator,
    errorReporter services.ErrorReporter,
) *AnalyzeTool {
    return &AnalyzeTool{
        sessionStore: sessionStore,
        configValidator: configValidator,
        errorReporter: errorReporter,
    }
}
```

## Session Management

### Why Sessions Matter
Container Kit operations often involve multiple steps:
1. Analyze repository
2. Generate Dockerfile
3. Build image
4. Scan for vulnerabilities
5. Deploy to Kubernetes

Sessions provide persistent state across these operations.

### Session Architecture
- **Storage**: BoltDB for lightweight, embedded persistence
- **Isolation**: Each session gets its own workspace directory
- **Metadata**: Labels for organization and querying
- **Lifecycle**: Automatic cleanup and expiration

### Session Usage Pattern
```go
// Create or retrieve session
session, err := sessionStore.CreateSession(ctx, &types.SessionRequest{
    Labels: map[string]string{
        "project": "my-app",
        "user": "developer",
    },
})

// Use session in tool operations
result, err := tool.Execute(ctx, ToolInput{
    SessionID: session.ID,
    Data: map[string]interface{}{
        "repository_path": "/path/to/repo",
    },
})
```

## Tool Development Workflow

### 1. Choose the Right Domain
- **Analyze**: Repository analysis, Dockerfile generation
- **Build**: Docker operations (build, push, pull, tag)
- **Deploy**: Kubernetes deployment and management
- **Scan**: Security scanning and vulnerability detection

### 2. Create Your Tool
```go
// pkg/mcp/domain/containerization/analyze/my_new_tool.go
type MyNewTool struct {
    sessionStore services.SessionStore
    validator    services.ConfigValidator
}

func (t *MyNewTool) Name() string {
    return "my_new_tool_atomic"
}

func (t *MyNewTool) Description() string {
    return "Performs a specific analysis operation"
}

func (t *MyNewTool) Schema() api.ToolSchema {
    return api.ToolSchema{
        Type: "object",
        Properties: map[string]api.PropertySchema{
            "session_id": {Type: "string", Description: "Session ID"},
            "input_param": {Type: "string", Description: "Input parameter"},
        },
        Required: []string{"session_id", "input_param"},
    }
}

func (t *MyNewTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Validate input
    if err := t.validator.ValidateInput(input); err != nil {
        return api.ToolOutput{}, err
    }

    // Get session
    session, err := t.sessionStore.GetSession(ctx, input.SessionID)
    if err != nil {
        return api.ToolOutput{}, err
    }

    // Perform operation
    result := performOperation(input.Data)

    // Return structured output
    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "result": result,
        },
    }, nil
}
```

### 3. Register Your Tool
Tools auto-register through the registry system. The registry discovers tools through interface implementation.

### 4. Test Your Tool
```go
// pkg/mcp/domain/containerization/analyze/my_new_tool_test.go
func TestMyNewTool(t *testing.T) {
    // Create mock services
    mockSessionStore := &MockSessionStore{}
    mockValidator := &MockConfigValidator{}

    // Create tool instance
    tool := &MyNewTool{
        sessionStore: mockSessionStore,
        validator: mockValidator,
    }

    // Test execution
    output, err := tool.Execute(ctx, api.ToolInput{
        SessionID: "test-session",
        Data: map[string]interface{}{
            "input_param": "test-value",
        },
    })

    assert.NoError(t, err)
    assert.True(t, output.Success)
}
```

## Code Organization Rationale

### Why Atomic Tools Are Separate Files
Each atomic tool is in its own file because:
- **Single Responsibility**: Each file has one clear purpose
- **Easy Navigation**: Developers can quickly find specific functionality
- **Independent Testing**: Each tool can be tested in isolation
- **Clear Dependencies**: Tool dependencies are explicit in the file

### Why We Have Both Atomic and Conversational Tools
- **Atomic Tools**: Precise, composable operations for workflows
- **Conversational Tools**: User-friendly, guided interactions
- **Flexibility**: Support both programmatic and interactive use cases

### Why Services Are Interface-Based
- **Testability**: Easy to mock for unit testing
- **Flexibility**: Can swap implementations without changing clients
- **Dependency Inversion**: High-level modules don't depend on low-level modules

### Why Domain Logic Is Separate from Infrastructure
- **Portability**: Domain logic can work with different databases, protocols
- **Testing**: Domain logic can be tested without external dependencies
- **Maintainability**: Changes in external systems don't affect business logic

## Common Patterns and Examples

### Error Handling Pattern
```go
// Use the unified RichError system
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("invalid input parameter").
    Context("parameter", paramName).
    Context("value", paramValue).
    Suggestion("Check parameter format and try again").
    WithLocation().
    Build()
```

### Validation Pattern
```go
// Use struct tags for validation
type BuildRequest struct {
    ImageName string `validate:"required,image_name"`
    Tag       string `validate:"required,tag_format"`
    Registry  string `validate:"url"`
}

// Generate validation code
//go:generate validation-gen -type=BuildRequest
```

### Session Context Pattern
```go
// Always pass session context
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    session, err := t.sessionStore.GetSession(ctx, input.SessionID)
    if err != nil {
        return api.ToolOutput{}, err
    }

    // Use session workspace for file operations
    workspacePath := session.WorkspacePath

    // Update session state
    session.State["last_operation"] = "analyze"
    err = t.sessionStore.UpdateSession(ctx, session)

    return api.ToolOutput{Success: true}, nil
}
```

## Debugging and Troubleshooting

### Common Issues and Solutions

#### Tool Not Found
**Problem**: MCP client can't find your tool
**Solution**: Check tool registration in the registry
**Debug**: Look at server startup logs for registration messages

#### Schema Validation Errors
**Problem**: Tool input doesn't match schema
**Solution**: Verify your tool's `Schema()` method matches expected input
**Debug**: Compare input JSON with schema requirements

#### Session Errors
**Problem**: Session operations fail
**Solution**: Check session ID validity and BoltDB connection
**Debug**: Enable session logging to see database operations

#### Import Cycle Errors
**Problem**: Cannot import package due to cycles
**Solution**: Follow the three-layer architecture dependencies
**Debug**: Use `go mod graph` to visualize dependencies

### Debugging Tools
- **Make Commands**: Use `make test`, `make lint`, `make bench`
- **Logging**: Enable debug logging with environment variables
- **MCP Client**: Test tools directly with MCP-compatible clients
- **Unit Tests**: Run focused tests with `go test -v ./pkg/mcp/domain/...`

## Next Steps

### For New Contributors
1. **Read the ADRs**: Understand architectural decisions in `docs/architecture/adr/`
2. **Explore Examples**: Look at existing tools in `pkg/mcp/domain/containerization/`
3. **Run Tests**: Execute `make test-all` to see the full test suite
4. **Try MCP Client**: Connect with Claude Desktop to see tools in action

### For Tool Development
1. **Choose Your Domain**: Decide which domain fits your new tool
2. **Define Interface**: Create clear input/output schemas
3. **Implement Logic**: Focus on the domain logic first
4. **Add Tests**: Write comprehensive unit tests
5. **Integration Test**: Test with MCP client end-to-end

### For Architecture Understanding
1. **Study Service Container**: See how dependency injection works
2. **Trace Tool Execution**: Follow a tool call from client to domain logic
3. **Understand Sessions**: See how state persists across operations
4. **Review Error Handling**: Learn the RichError patterns

### Resources
- **Documentation**: `docs/` directory contains detailed guides
- **Examples**: `pkg/mcp/domain/containerization/` has many tool examples
- **Tests**: Look at `*_test.go` files for usage patterns
- **MCP Specification**: [Model Context Protocol documentation](https://modelcontextprotocol.io/)

---

Welcome to Container Kit! The architecture might seem complex at first, but it's designed to be maintainable, testable, and extensible. Each architectural decision was made to solve specific problems and improve the developer experience. Take your time to understand the patterns, and don't hesitate to ask questions or refer back to this guide as you work with the codebase.
