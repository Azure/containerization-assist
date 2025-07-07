# MCP Tool Implementation Standards

This document defines the canonical patterns that ALL MCP tools must follow to ensure consistency, maintainability, and reliability.

## 1. Tool Registration Pattern (CANONICAL)

All tools MUST be registered using the unified api.Tool interface in the MCP server core:

```go
// Tools are automatically registered through the unified interface system
// Each tool implements api.Tool interface
type api.Tool interface {
    Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
    Name() string
    Description() string
    Schema() api.ToolSchema
}
```

### ❌ FORBIDDEN Patterns:
```go
// DON'T use legacy interfaces or methods - all removed
// DON'T use type casting in registration
if typed, ok := result.(*SomeType); ok { return typed, nil }
```

## 2. Tool Interface Standard (CANONICAL)

All tools MUST implement the api.Tool interface:

```go
type Tool interface {
    Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
    Name() string
    Description() string
    Schema() api.ToolSchema
}
```

### ✅ Required Method Signatures:
- **Execute**: `Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)`
- **Name**: `Name() string`
- **Description**: `Description() string`
- **Schema**: `Schema() api.ToolSchema`

## 3. Parameter Structure Standard (CANONICAL)

All tools receive parameters through the unified api.ToolInput structure:

```go
type ToolInput struct {
    SessionID string                 `json:"session_id,omitempty"`
    Data      map[string]interface{} `json:"data"`
}
```

### Parameter Extraction Pattern:
```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract tool-specific parameters from input.Data
    var params ToolSpecificParams
    if rawParams, ok := input.Data["params"]; ok {
        if typedParams, ok := rawParams.(ToolSpecificParams); ok {
            params = typedParams
        } else {
            return api.ToolOutput{
                Success: false,
                Error:   "Invalid input type",
            }, fmt.Errorf("invalid input type")
        }
    }

    // Use session ID from input if available
    if input.SessionID != "" {
        params.SessionID = input.SessionID
    }

    // Continue with execution...
}
```

### Parameter Validation Rules:
- **session_id**: Available from api.ToolInput.SessionID
- **dry_run**: Tool-specific parameter in params struct
- Parameters are validated within each tool's Execute method

## 4. Response Structure Standard (CANONICAL)

All tools return results through the unified api.ToolOutput structure:

```go
type ToolOutput struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}
```

### Response Pattern:
```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Execute tool logic...

    // On success:
    return api.ToolOutput{
        Success: true,
        Data:    map[string]interface{}{"result": toolResult},
    }, nil

    // On failure:
    return api.ToolOutput{
        Success: false,
        Data:    map[string]interface{}{"result": partialResult},
        Error:   err.Error(),
    }, err
}
```

### ✅ Required Response Fields:
- **Success**: Always present, indicates operation success
- **Data**: Contains tool-specific results in "result" key
- **Error**: Present when Success is false

## 5. Error Handling Standard (CANONICAL)

Tools MUST return both api.ToolOutput and error consistently:

```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Validation errors
    if err := validateInput(input); err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    // Execute tool logic
    result, err := t.executeOperation(ctx, input)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Data:    map[string]interface{}{"result": result},
            Error:   err.Error(),
        }, err
    }

    // Success case
    return api.ToolOutput{
        Success: true,
        Data:    map[string]interface{}{"result": result},
    }, nil
}
```

### ✅ Required Error Patterns:
- **Validation errors**: Return ToolOutput with Success=false AND return error
- **Operation errors**: Return ToolOutput with partial data AND return error
- **Success**: Return ToolOutput with Success=true AND nil error

### ❌ FORBIDDEN Error Patterns:
```go
// DON'T return success without error when operation fails
return api.ToolOutput{Success: false}, nil

// DON'T return error without setting Success=false
return api.ToolOutput{Success: true}, err
```

## 6. Session Management Standard (CANONICAL)

All tools MUST use this standard session acquisition pattern:

```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract session ID from input
    sessionID := input.SessionID
    if sessionID == "" {
        return api.ToolOutput{
            Success: false,
            Error:   "session_id is required",
        }, fmt.Errorf("session_id is required")
    }

    // Get or create session
    sess, err := t.sessionManager.GetOrCreateSession(ctx, sessionID)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   "Failed to get session",
        }, err
    }

    // Update session state
    sess.UpdateLastAccessed()

    // Continue with tool execution...
}
```

## 7. Progress Reporting Standard (CANONICAL)

All tools SHOULD use consistent logging patterns:

```go
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    startTime := time.Now()

    t.logger.Info().
        Str("session_id", input.SessionID).
        Str("tool", t.Name()).
        Msg("Starting tool execution")

    // Execute tool logic...

    t.logger.Info().
        Str("session_id", input.SessionID).
        Str("tool", t.Name()).
        Dur("duration", time.Since(startTime)).
        Bool("success", result.Success).
        Msg("Tool execution completed")

    return result, nil
}
```

## 8. Schema Definition Standard (CANONICAL)

All tools MUST implement the Schema() method:

```go
func (t *Tool) Schema() api.ToolSchema {
    return api.ToolSchema{
        Name:        t.Name(),
        Description: t.Description(),
        Version:     "1.0.0",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "params": map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "session_id": map[string]interface{}{
                            "type":        "string",
                            "description": "Session identifier",
                        },
                        // Tool-specific parameters...
                    },
                    "required": []string{"session_id"},
                },
            },
        },
        OutputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "success": map[string]interface{}{
                    "type":        "boolean",
                    "description": "Operation success indicator",
                },
                "data": map[string]interface{}{
                    "type":        "object",
                    "description": "Tool-specific result data",
                },
            },
            "required": []string{"success"},
        },
    }
}
```

## 9. Testing Standards (CANONICAL)

All tools MUST have consistent test patterns:

```go
func TestToolExecute(t *testing.T) {
    // Setup
    tool := NewTool(mockSessionManager, testLogger)

    testCases := []struct {
        name    string
        input   api.ToolInput
        wantErr bool
        validate func(*testing.T, api.ToolOutput, error)
    }{
        {
            name: "success_case",
            input: api.ToolInput{
                SessionID: "test-session",
                Data: map[string]interface{}{
                    "params": ToolSpecificParams{
                        SessionID: "test-session",
                        // ... required fields
                    },
                },
            },
            wantErr: false,
            validate: func(t *testing.T, output api.ToolOutput, err error) {
                assert.NoError(t, err)
                assert.True(t, output.Success)
                assert.NotNil(t, output.Data)
            },
        },
        {
            name: "missing_session_id",
            input: api.ToolInput{
                Data: map[string]interface{}{
                    "params": ToolSpecificParams{},
                },
            },
            wantErr: true,
            validate: func(t *testing.T, output api.ToolOutput, err error) {
                assert.Error(t, err)
                assert.False(t, output.Success)
                assert.Contains(t, err.Error(), "session_id")
            },
        },
    }

    for _, tt := range testCases {
        t.Run(tt.name, func(t *testing.T) {
            output, err := tool.Execute(context.Background(), tt.input)
            tt.validate(t, output, err)
        })
    }
}
```

## 10. Documentation Standards (CANONICAL)

All tools MUST have consistent documentation:

```go
// Tool provides [brief description of what the tool does].
// It implements the api.Tool interface for unified tool execution.
type Tool struct {
    sessionManager session.UnifiedSessionManager
    logger         zerolog.Logger
    // ...
}

// NewTool creates a new [ToolName] that implements api.Tool interface.
func NewTool(sessionManager session.UnifiedSessionManager, logger zerolog.Logger) api.Tool {
    return &Tool{
        sessionManager: sessionManager,
        logger:         logger.With().Str("tool", "tool_name").Logger(),
    }
}

// Execute executes [tool operation] with the provided input.
// It validates inputs, acquires session state, and performs [main operation].
//
// Returns:
//   - api.ToolOutput: Operation results with success indicator
//   - error: Any validation, session, or execution errors
func (t *Tool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // ...
}
```

## Compliance Checklist

When implementing or updating a tool, verify:

- [ ] Tool implements api.Tool interface (Execute, Name, Description, Schema)
- [ ] Execute method signature: `Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)`
- [ ] Constructor returns api.Tool interface
- [ ] Parameters extracted from api.ToolInput.Data["params"]
- [ ] Session ID used from api.ToolInput.SessionID
- [ ] Returns api.ToolOutput with Success, Data, and Error fields
- [ ] Returns both ToolOutput and error consistently
- [ ] Uses standard session acquisition pattern
- [ ] Has consistent logging with tool name in logger context
- [ ] Has test coverage for success and error cases
- [ ] Has proper documentation for all public methods
- [ ] Schema() method returns complete api.ToolSchema

---

**Note**: These standards are mandatory for all MCP tools. All tools now consistently implement the api.Tool interface - no legacy patterns remain in active use.
