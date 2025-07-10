# MCP Tool Implementation Standards

This document defines the canonical patterns that ALL MCP tools must follow to ensure consistency, maintainability, and reliability.

## 1. Tool Registration Pattern (CANONICAL)

Tools are currently registered using the gomcp library pattern in the MCP server:

```go
// Current implementation uses gomcp library for tool registration
s.server.Tool("analyze_repository", "Analyze repository structure and generate Dockerfile recommendations",
    func(_ *server.Context, args *struct {
        RepoURL      string `json:"repo_url"`
        Context      string `json:"context,omitempty"`
        // ... other parameters
    }) (*struct {
        Success    bool                   `json:"success"`
        Message    string                 `json:"message,omitempty"`
        // ... other response fields
    }, error) {
        // Tool implementation
    })
```

### Consolidated Command Pattern (Future)
Consolidated commands implement the api.Tool interface for unified tool execution:

```go
type Tool interface {
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
    Name() string
    Description() string
    Schema() ToolSchema
}
```

## 2. Tool Interface Standard (CANONICAL)

### Current Implementation: gomcp Library Pattern
Tools are currently implemented using gomcp library functions:

```go
// Current pattern: gomcp library function with typed parameters
func toolImplementation(_ *server.Context, args *ToolArgs) (*ToolResponse, error) {
    // Validation
    if args.RequiredParam == "" {
        return &ToolResponse{
            Success: false,
            Message: "required parameter missing",
        }, nil
    }

    // Business logic
    result, err := performOperation(args)
    if err != nil {
        return &ToolResponse{
            Success: false,
            Message: err.Error(),
        }, nil
    }

    return &ToolResponse{
        Success: true,
        Message: "Operation completed successfully",
        Data:    result,
    }, nil
}
```

### Consolidated Command Pattern (Available)
Consolidated commands implement the api.Tool interface:

```go
type Tool interface {
    Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
    Name() string
    Description() string
    Schema() ToolSchema
}
```

## 3. Parameter Structure Standard (CANONICAL)

### Current Implementation: gomcp Typed Parameters
Tools currently receive typed parameters through gomcp library:

```go
// Tool function signature with typed parameters
func(_ *server.Context, args *struct {
    RepoURL      string `json:"repo_url"`
    Context      string `json:"context,omitempty"`
    Branch       string `json:"branch,omitempty"`
    LanguageHint string `json:"language_hint,omitempty"`
    Shallow      bool   `json:"shallow,omitempty"`
}) (*ResponseType, error) {
    // Direct access to typed parameters
    if args.RepoURL == "" {
        return &ResponseType{
            Success: false,
            Message: "repo_url is required",
        }, nil
    }

    // Business logic using typed parameters
    result := processRepository(args.RepoURL, args.Context)
    return &ResponseType{
        Success: true,
        Data:    result,
    }, nil
}
```

### Consolidated Command Pattern: api.ToolInput
Consolidated commands use the api.ToolInput structure:

```go
type ToolInput struct {
    SessionID string                 `json:"session_id,omitempty"`
    Data      map[string]interface{} `json:"data"`
}

// Parameter extraction in consolidated commands
func (cmd *ConsolidatedAnalyzeCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract parameters from input.Data
    request, err := cmd.parseAnalysisInput(input)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    // Business logic
    result, err := cmd.performAnalysis(ctx, request, workspaceDir)
    // ...
}
```

## 4. Response Structure Standard (CANONICAL)

### Current Implementation: gomcp Typed Response
Tools currently return typed response structures:

```go
// Typed response structure for gomcp tools
type AnalyzeResponse struct {
    Success    bool                   `json:"success"`
    Message    string                 `json:"message,omitempty"`
    Analysis   map[string]interface{} `json:"analysis,omitempty"`
    RepoURL    string                 `json:"repo_url"`
    Language   string                 `json:"language,omitempty"`
    Framework  string                 `json:"framework,omitempty"`
    Dockerfile string                 `json:"dockerfile,omitempty"`
    SessionID  string                 `json:"session_id,omitempty"`
}

// Return pattern for current implementation
return &AnalyzeResponse{
    Success:    true,
    Message:    "Analysis completed successfully",
    Analysis:   analysisResult,
    RepoURL:    args.RepoURL,
    Language:   detectedLanguage,
    Framework:  detectedFramework,
    Dockerfile: generatedDockerfile,
    SessionID:  generatedSessionID,
}, nil
```

### Consolidated Command Pattern: api.ToolOutput
Consolidated commands return api.ToolOutput:

```go
type ToolOutput struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   string                 `json:"error,omitempty"`
}

// Return pattern for consolidated commands
return api.ToolOutput{
    Success: true,
    Data: map[string]interface{}{
        "analysis_result": response,
    },
}, nil
```

## 5. Error Handling Standard (CANONICAL)

### Current Implementation: gomcp Error Handling
Tools currently handle errors within the response structure:

```go
// Error handling in gomcp tools
func toolImplementation(_ *server.Context, args *ToolArgs) (*ToolResponse, error) {
    // Validation errors
    if args.RequiredParam == "" {
        return &ToolResponse{
            Success: false,
            Message: "required parameter missing",
        }, nil // Return nil error, encode error in response
    }

    // Operation errors
    result, err := performOperation(args)
    if err != nil {
        return &ToolResponse{
            Success: false,
            Message: err.Error(),
        }, nil // Return nil error, encode error in response
    }

    return &ToolResponse{
        Success: true,
        Data:    result,
    }, nil
}
```

### Consolidated Command Pattern: Dual Error Handling
Consolidated commands return both ToolOutput and error:

```go
func (cmd *ConsolidatedCommand) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Validation errors
    if err := validateInput(input); err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    // Operation errors
    result, err := cmd.executeOperation(ctx, input)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "result": result,
        },
    }, nil
}
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

### Current Implementation (gomcp library)
When implementing or updating a tool, verify:

- [ ] Tool registered using `s.server.Tool(name, description, handlerFunc)`
- [ ] Handler function signature: `func(_ *server.Context, args *StructType) (*ResponseType, error)`
- [ ] Parameters defined as typed structs with JSON tags
- [ ] Response structure includes Success field
- [ ] Error handling encoded in response structure (Success=false)
- [ ] Return nil error for gomcp library (errors in response)
- [ ] Has consistent logging with tool name in logger context
- [ ] Has test coverage for success and error cases
- [ ] Has proper documentation for all public methods

### Consolidated Command Pattern (Available)
When implementing consolidated commands, verify:

- [ ] Command implements api.Tool interface (Execute, Name, Description, Schema)
- [ ] Execute method signature: `Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)`
- [ ] Constructor returns api.Tool interface
- [ ] Parameters extracted from api.ToolInput.Data
- [ ] Session ID used from api.ToolInput.SessionID
- [ ] Returns api.ToolOutput with Success, Data, and Error fields
- [ ] Returns both ToolOutput and error consistently
- [ ] Uses standard session acquisition pattern
- [ ] Schema() method returns complete api.ToolSchema

---

**Note**: Container Kit currently uses both patterns. The gomcp library pattern is actively used in the server implementation, while consolidated commands provide the api.Tool interface for potential future migration.
