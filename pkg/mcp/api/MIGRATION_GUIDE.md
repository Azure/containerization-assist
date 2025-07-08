# Type-Safe API Migration Guide

This guide helps developers migrate from the `interface{}`-based tool APIs to the new type-safe generic APIs.

## Overview

The migration eliminates `interface{}` usage in favor of:
- Generic type parameters for compile-time safety
- Strongly typed context and details structures
- Type-safe schema generation from Go types

## Migration Steps

### 1. Update Tool Interfaces

**Before:**
```go
type MyTool struct{}

func (t *MyTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
    // Extract data from map[string]interface{}
    data := input.Data
    repoURL, _ := data["repo_url"].(string)

    // Process...

    return ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "result": "value",
        },
    }, nil
}
```

**After:**
```go
type MyTool struct{}

// Implement TypedTool with specific types
func (t *MyTool) Execute(ctx context.Context, input TypedToolInput[TypedAnalyzeInput, AnalysisContext]) (TypedToolOutput[TypedAnalyzeOutput, AnalysisDetails], error) {
    // Direct access to typed fields
    repoURL := input.Data.RepoURL
    branch := input.Context.Branch

    // Process...

    return TypedToolOutput[TypedAnalyzeOutput, AnalysisDetails]{
        Success: true,
        Data: TypedAnalyzeOutput{
            SessionID: input.SessionID,
            Language: "go",
            Framework: "gin",
        },
        Details: AnalysisDetails{
            ExecutionDetails: ExecutionDetails{
                Duration: time.Since(startTime),
            },
            FilesScanned: 42,
        },
    }, nil
}
```

### 2. Define Custom Input/Output Types

For tools with unique requirements, define custom types:

```go
// Custom input type
type MyCustomInput struct {
    SessionID   string            `json:"session_id"`
    CustomField string            `json:"custom_field"`
    Options     map[string]string `json:"options"`
}

// Implement ToolInputConstraint
func (m *MyCustomInput) GetSessionID() string { return m.SessionID }
func (m *MyCustomInput) Validate() error {
    if m.CustomField == "" {
        return errors.New("custom_field is required")
    }
    return nil
}
func (m *MyCustomInput) GetContext() map[string]interface{} {
    return nil // Will be replaced with typed context
}

// Custom output type
type MyCustomOutput struct {
    Success bool   `json:"success"`
    Result  string `json:"result"`
    Metrics MyMetrics `json:"metrics"`
}

// Implement ToolOutputConstraint
func (m *MyCustomOutput) IsSuccess() bool { return m.Success }
func (m *MyCustomOutput) GetData() interface{} { return m }
func (m *MyCustomOutput) GetError() string { return "" }
```

### 3. Use Type-Safe Adapters

For backward compatibility, use the type-safe adapters:

```go
// Create typed tool
typedTool := &MyAnalyzeTool{}

// Wrap with adapter for legacy compatibility
legacyTool := AnalyzeToolAdapter(typedTool)

// Register with legacy registry
registry.Register(legacyTool)
```

### 4. Generate Schemas Automatically

Replace manual schema definitions with automatic generation:

**Before:**
```go
func (t *MyTool) Schema() ToolSchema {
    return ToolSchema{
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "repo_url": map[string]interface{}{
                    "type": "string",
                },
            },
        },
    }
}
```

**After:**
```go
func (t *MyTool) Schema() TypedToolSchemaV2 {
    return GenerateToolSchema[TypedAnalyzeInput, TypedAnalyzeOutput](
        "analyze",
        "Analyzes a repository",
        "1.0.0",
    )
}
```

### 5. Update Tool Registration

Use the new typed registry methods:

```go
// Create typed registry
registry := NewTypedRegistry()

// Register typed tools
registry.RegisterAnalyzeTool(analyzeImpl)
registry.RegisterBuildTool(buildImpl)
registry.RegisterDeployTool(deployImpl)

// Get typed tool
tool, err := registry.GetAnalyzeTool("analyze")
```

## Common Patterns

### Pattern 1: Context Types

Use specialized context types instead of `map[string]interface{}`:

```go
// Analysis with proper context
ctx := AnalysisContext{
    ExecutionContext: ExecutionContext{
        RequestID: "req-123",
        TraceID:   "trace-456",
        Timeout:   5 * time.Minute,
    },
    Branch:        "main",
    AnalysisDepth: 3,
}
```

### Pattern 2: Details Types

Use structured details instead of arbitrary maps:

```go
// Build with proper details
details := BuildDetails{
    ExecutionDetails: ExecutionDetails{
        Duration:  buildTime,
        StartTime: startTime,
        EndTime:   endTime,
    },
    ImageSize:  1024 * 1024 * 50, // 50MB
    LayerCount: 12,
    CacheHit:   true,
}
```

### Pattern 3: Validation

Implement validation in the type itself:

```go
func (i *TypedBuildInput) Validate() error {
    if i.Image == "" {
        return errors.New("image name is required")
    }
    if i.Dockerfile == "" && i.ContextPath == "" {
        return errors.New("either dockerfile or context is required")
    }
    return nil
}
```

## Testing

Test with concrete types:

```go
func TestAnalyzeTool(t *testing.T) {
    tool := &MyAnalyzeTool{}

    input := TypedToolInput[TypedAnalyzeInput, AnalysisContext]{
        SessionID: "test-session",
        Data: TypedAnalyzeInput{
            RepoURL: "https://github.com/example/repo",
            Branch:  "main",
        },
        Context: AnalysisContext{
            ExecutionContext: ExecutionContext{
                RequestID: "test-req",
            },
        },
    }

    output, err := tool.Execute(context.Background(), input)
    assert.NoError(t, err)
    assert.True(t, output.Success)
    assert.Equal(t, "go", output.Data.Language)
}
```

## Gradual Migration

1. Start with new tools using typed interfaces
2. Wrap existing tools with adapters
3. Gradually update tools to use typed interfaces
4. Remove adapters once all consumers are updated
5. Deprecate legacy interfaces

## Benefits

- **Compile-time safety**: Catch type errors during compilation
- **Better IDE support**: Auto-completion and type hints
- **Clearer contracts**: Self-documenting code
- **Reduced runtime errors**: No more type assertions
- **Performance**: Avoid reflection and JSON marshaling
