# Atomic Tool Development Standards

This document defines the development standards for atomic tools in the MCP (Model Context Protocol) system.

## Overview

Atomic tools are the foundational building blocks of the container-copilot system. They provide deterministic, composable operations that can be used independently or orchestrated together in workflows.

## Architecture Standards

### 1. Tool Structure

Every atomic tool must follow this standardized structure:

```go
// Tool struct with required components
type AtomicToolName struct {
    pipelineAdapter PipelineAdapter
    sessionManager  SessionManager
    errorHandler    *errors.ErrorHandler
    progressTracker interfaces.ProgressTracker
    fixingMixin     *fixing.AtomicToolFixingMixin
    logger          zerolog.Logger
}

// Constructor following naming convention
func NewAtomicToolName(adapter PipelineAdapter, sessionManager SessionManager, logger zerolog.Logger) *AtomicToolName {
    toolLogger := logger.With().Str("tool", "atomic_tool_name").Logger()
    return &AtomicToolName{
        pipelineAdapter: adapter,
        sessionManager:  sessionManager,
        errorHandler:    errors.NewErrorHandler(toolLogger),
        progressTracker: nil, // Will be injected via SetProgressTracker if needed
        fixingMixin:     nil, // Will be set via SetAnalyzer if fixing is enabled
        logger:          toolLogger,
    }
}
```

### 2. Required Interfaces

All atomic tools must implement these interfaces:

#### Core Execution Interface
```go
type ExecutableTool[TArgs, TResult any] interface {
    Tool
    Execute(ctx context.Context, args TArgs) (*TResult, error)
    PreValidate(ctx context.Context, args TArgs) error
}
```

#### Progress Tracking Interface
```go
type ProgressCapable interface {
    SetProgressTracker(tracker interfaces.ProgressTracker)
}
```

#### AI-Enhanced Fixing Interface (Optional)
```go
type FixingCapable interface {
    SetAnalyzer(analyzer analyzer.Analyzer)
    ExecuteWithFixes(ctx context.Context, args TArgs) (*TResult, error)
}
```

### 3. Arguments and Results Structure

#### Arguments Structure
```go
type AtomicToolNameArgs struct {
    types.BaseToolArgs                    // Required: includes SessionID, DryRun

    // Tool-specific arguments with proper validation tags
    RequiredField string `json:"required_field" jsonschema:"required" description:"Field description"`
    OptionalField string `json:"optional_field,omitempty" description:"Optional field description"`
}
```

#### Results Structure
```go
type AtomicToolNameResult struct {
    types.BaseToolResponse                // Required: includes common response fields
    Success bool `json:"success"`        // Required: operation success status

    // Session context (standard across all tools)
    SessionID    string `json:"session_id"`
    WorkspaceDir string `json:"workspace_dir"`

    // Tool-specific results
    OperationResult *SpecificResult `json:"operation_result"`

    // Timing information (standard pattern)
    Duration time.Duration `json:"duration"`

    // AI context for decision-making (required for AI enhancement)
    AIContext *AIContextInfo `json:"ai_context"`

    // Rich error information if operation failed
    Error *types.RichError `json:"error,omitempty"`
}
```

## Progress Reporting Standards

### 1. Use Centralized Stage Definitions

**DO NOT** define local stage functions. Use centralized stage definitions:

```go
// ❌ WRONG: Local stage definition
func standardBuildStages() []interfaces.ProgressStage {
    return []interfaces.ProgressStage{...}
}

// ✅ CORRECT: Use centralized stages
import "github.com/Azure/container-copilot/pkg/mcp/internal/core"

err := t.progressTracker.RunWithProgress(ctx, "Operation Name", core.StandardBuildStages(), func(...) {
    // Implementation
})
```

### 2. Available Centralized Stage Types

- `core.StandardBuildStages()` - For build operations
- `core.StandardDeployStages()` - For deployment operations
- `core.StandardScanStages()` - For security scanning operations
- `core.StandardAnalysisStages()` - For repository analysis operations
- `core.StandardPushStages()` - For registry push operations
- `core.StandardPullStages()` - For registry pull operations
- `core.StandardTagStages()` - For Docker tag operations
- `core.StandardValidationStages()` - For validation operations
- `core.StandardHealthStages()` - For health check operations
- `core.StandardGenerateStages()` - For generation operations

### 3. Progress Implementation Pattern

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    startTime := time.Now()
    result := &Result{...} // Initialize result early for error handling

    // Use RunWithProgress if we have a progress tracker
    if t.progressTracker != nil {
        err := t.progressTracker.RunWithProgress(ctx, "Operation Name", core.StandardXxxStages(), func(ctx context.Context, reporter interfaces.ProgressReporter) error {
            return t.executeWithProgress(ctx, args, result, startTime, reporter)
        })

        result.Duration = time.Since(startTime)
        if err != nil {
            result.Success = false
            return result, nil
        }
        return result, nil
    }

    // Fallback: execute without progress tracking
    return t.executeWithoutProgress(ctx, args, result, startTime)
}
```

### 4. Stage Reporting Pattern

```go
func (t *AtomicToolName) executeWithProgress(ctx context.Context, args Args, result *Result, startTime time.Time, reporter interfaces.ProgressReporter) error {
    // Stage 1: Initialize (weight: 0.10)
    reporter.ReportStage(0.1, "Loading session")
    // ... do initialization work ...
    reporter.ReportStage(1.0, "Session loaded")

    // Stage 2: Process (weight: 0.60)
    reporter.NextStage("Processing data")
    // ... do processing work with interim progress ...
    reporter.ReportStage(0.5, "Half done")
    // ... continue processing ...
    reporter.ReportStage(1.0, "Processing complete")

    // Continue through remaining stages...
    reporter.NextStage("Finalizing")
    reporter.ReportStage(1.0, "Operation completed")

    return nil
}
```

## Validation Standards

### 1. Use Standardized Validation Utilities

```go
import "github.com/Azure/container-copilot/pkg/mcp/internal/utils"

// Initialize validation mixin
validationMixin := utils.NewStandardizedValidationMixin(t.logger)

// Standard session validation
validatedSession, richError := validationMixin.StandardValidateSession(ctx, t.sessionManager, args.SessionID)
if richError != nil {
    result.Error = richError
    return result, nil
}

// Standard required field validation
validationResult := validationMixin.StandardValidateRequiredFields(args, []string{"RequiredField1", "RequiredField2"})
if !validationResult.Valid {
    result.Error = validationMixin.ConvertValidationToRichError(validationResult, "validation", "input_validation")
    return result, nil
}

// Standard path validation
pathResult := validationMixin.StandardValidatePath(args.FilePath, "file_path", utils.PathRequirements{
    Required:        true,
    MustExist:       true,
    MustBeFile:      true,
    MustBeReadable:  true,
})
if !pathResult.Valid {
    result.Error = validationMixin.ConvertValidationToRichError(pathResult, "validation", "path_validation")
    return result, nil
}
```

### 2. PreValidate Implementation

```go
func (t *AtomicToolName) PreValidate(ctx context.Context, args Args) error {
    validationMixin := utils.NewStandardizedValidationMixin(t.logger)

    // Session validation
    _, richError := validationMixin.StandardValidateSession(ctx, t.sessionManager, args.SessionID)
    if richError != nil {
        return fmt.Errorf("session validation failed: %s", richError.Message)
    }

    // Required field validation
    validationResult := validationMixin.StandardValidateRequiredFields(args, []string{"RequiredField"})
    if !validationResult.Valid {
        return fmt.Errorf("field validation failed: %s", validationResult.GetFirstError().Message)
    }

    // Custom validation logic specific to this tool
    return t.validateCustomRequirements(ctx, args)
}
```

## Error Handling Standards

### 1. Use Standardized Error Creation

**Always use the error builders for consistent error formatting:**

```go
import "github.com/Azure/container-copilot/pkg/mcp/internal/errors"

// ✅ CORRECT: Use error builders
richError := errors.NewSystemError(
    types.ErrCodeSessionNotFound,
    "Failed to get session",
).
    WithField("session_id", args.SessionID).
    WithOperation("session_retrieval").
    WithStage("initialization").
    WithDiagnostic("session_lookup_failed", err.Error()).
    WithResolutionStep("Verify session ID is correct").
    WithResolutionStep("Check if session still exists").
    Build()

// ❌ WRONG: Manual error creation
richError := &types.RichError{
    Code:    "SESSION_NOT_FOUND",
    Message: "Failed to get session",
    // Missing structured context...
}
```

### 2. Error Categories and When to Use Them

#### System Errors
Use for infrastructure-level failures:
```go
errors.NewSystemError(types.ErrCodeSessionNotFound, message)     // Session issues
errors.NewSystemError(types.ErrCodePermissionDenied, message)   // Permission issues
errors.NewSystemError(types.ErrCodeDiskFull, message)           // Resource issues
errors.NewSystemError(types.ErrCodeTimeout, message)            // Timeout issues
```

#### Build Errors
Use for Docker build-related failures:
```go
errors.NewBuildError(message)                                   // General build failures
    .WithBuildStage("dockerfile_validation")                   // Specific build stage
    .WithDockerfile(dockerfilePath)                            // Include Dockerfile path
```

#### Deployment Errors
Use for Kubernetes deployment failures:
```go
errors.NewDeploymentError(message)                             // General deployment failures
    .WithNamespace(namespace)                                  // Include namespace
    .WithManifest(manifestPath)                               // Include manifest path
```

#### Security Errors
Use for security-related failures:
```go
errors.NewSecurityError("VULNERABILITY_FOUND", message)        // Security vulnerabilities
errors.NewSecurityError("SECRETS_DETECTED", message)          // Secrets detection
```

#### Analysis Errors
Use for repository analysis failures:
```go
errors.NewAnalysisError(message)                              // General analysis failures
    .WithRepository(repoURL)                                  // Include repository URL
    .WithLanguage(detectedLanguage)                           // Include detected language
```

### 3. Error Context Enrichment

**Always enrich errors with relevant context:**

```go
richError := errors.NewBuildError("Docker build failed").
    WithField("image_name", args.ImageName).                  // Include relevant fields
    WithField("dockerfile_path", dockerfilePath).
    WithOperation("docker_build").                            // What operation failed
    WithStage("build_execution").                             // What stage failed
    WithDiagnostic("build_output", buildOutput).              // Include diagnostic info
    WithDiagnostic("exit_code", fmt.Sprintf("%d", exitCode)).
    WithResolutionStep("Check Dockerfile syntax").           // Provide resolution steps
    WithResolutionStep("Verify base image is accessible").
    WithResolutionStep("Check build dependencies").
    Build()
```

## AI Context Integration Standards

### 1. Implement AI Context Interfaces

All atomic tools must implement the AI context interfaces for enhanced decision-making:

```go
// Implement ai_context.Assessable
func (r *ToolResult) CalculateScore() int { /* Implementation */ }
func (r *ToolResult) DetermineRiskLevel() types.Severity { /* Implementation */ }
func (r *ToolResult) GetStrengths() []string { /* Implementation */ }
func (r *ToolResult) GetChallenges() []string { /* Implementation */ }
func (r *ToolResult) GetAssessment() *ai_context.UnifiedAssessment { /* Implementation */ }

// Implement ai_context.Enrichable
func (r *ToolResult) EnrichWithContext(insights []ai_context.ToolContextualInsight) { /* Implementation */ }
func (r *ToolResult) GetEnrichmentMetadata() map[string]interface{} { /* Implementation */ }
```

### 2. AI Context Information Structure

```go
type ToolAIContext struct {
    // Operation insights
    OperationComplexity string   `json:"operation_complexity"` // simple, moderate, complex
    PerformanceMetrics  Metrics  `json:"performance_metrics"`

    // Decision-making context
    AlternativeApproaches []string `json:"alternative_approaches"`
    RiskFactors          []string `json:"risk_factors"`
    Recommendations      []string `json:"recommendations"`

    // Learning context
    CommonIssues        []Issue   `json:"common_issues"`
    OptimizationTips    []string  `json:"optimization_tips"`
    TroubleshootingTips []string  `json:"troubleshooting_tips"`
}
```

## Testing Standards

### 1. Test Structure

```go
func TestAtomicToolName_Execute(t *testing.T) {
    tests := []struct {
        name           string
        args           AtomicToolNameArgs
        setupMocks     func(*MockPipelineAdapter, *MockSessionManager)
        expectSuccess  bool
        expectError    string
        validateResult func(*testing.T, *AtomicToolNameResult)
    }{
        {
            name: "successful_operation",
            args: AtomicToolNameArgs{
                BaseToolArgs: types.BaseToolArgs{SessionID: "test-session"},
                RequiredField: "test-value",
            },
            setupMocks: func(adapter *MockPipelineAdapter, sessionMgr *MockSessionManager) {
                // Setup mock expectations
            },
            expectSuccess: true,
            validateResult: func(t *testing.T, result *AtomicToolNameResult) {
                assert.True(t, result.Success)
                assert.NotEmpty(t, result.SessionID)
            },
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 2. Required Test Coverage

- **Happy path execution**
- **Input validation failures**
- **Session validation failures**
- **Permission errors**
- **Network errors** (if applicable)
- **Progress reporting** (if applicable)
- **AI context generation**
- **Dry-run mode**

## Documentation Standards

### 1. Code Documentation

```go
// AtomicToolName provides [brief description of tool functionality]
//
// This tool performs [detailed description] and supports:
// - Feature 1: Description
// - Feature 2: Description
// - Feature 3: Description
//
// The tool integrates with the pipeline adapter for [purpose] and
// supports progress reporting through the standard progress interface.
//
// Example usage:
//   tool := NewAtomicToolName(adapter, sessionManager, logger)
//   result, err := tool.Execute(ctx, args)
type AtomicToolName struct {
    // Fields...
}
```

### 2. Argument Documentation

```go
type AtomicToolNameArgs struct {
    types.BaseToolArgs

    // RequiredField specifies [purpose and constraints]
    // This field is required and must [validation requirements]
    RequiredField string `json:"required_field" jsonschema:"required,minLength=1" description:"Field description with constraints"`

    // OptionalField provides [purpose and default behavior]
    // When not specified, [default behavior description]
    OptionalField string `json:"optional_field,omitempty" description:"Optional field with default behavior"`
}
```

## Deployment and Integration Standards

### 1. Tool Registration

Tools should be registered in the tool registry with proper metadata:

```go
func (r *MCPToolRegistry) registerAtomicTools() {
    r.registerTool("atomic_tool_name", &tools.AtomicToolName{})
    // Metadata will be automatically inferred from the tool structure
}
```

### 2. Pipeline Integration

Tools must work seamlessly with the pipeline adapter:

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    // Get session workspace through adapter
    workspaceDir := t.pipelineAdapter.GetSessionWorkspace(session.ID)

    // Use adapter for resource management
    resource, err := t.pipelineAdapter.AcquireResource(ctx, "docker")
    if err != nil {
        return nil, err
    }
    defer t.pipelineAdapter.ReleaseResource(resource)

    // Continue with tool implementation...
}
```

## Performance Standards

### 1. Timeout Handling

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    // Respect context timeouts
    select {
    case <-ctx.Done():
        return &Result{
            Success: false,
            Error: errors.NewSystemError(types.ErrCodeTimeout, "Operation cancelled").Build(),
        }, ctx.Err()
    default:
        // Continue execution
    }
}
```

### 2. Resource Management

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    // Always clean up resources
    defer func() {
        if err := t.cleanup(); err != nil {
            t.logger.Warn().Err(err).Msg("Failed to cleanup resources")
        }
    }()

    // Tool implementation...
}
```

## Security Standards

### 1. Input Sanitization

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    // Sanitize file paths
    cleanPath := filepath.Clean(args.FilePath)
    if !strings.HasPrefix(cleanPath, allowedBasePath) {
        return &Result{
            Success: false,
            Error: errors.NewSecurityError("PATH_TRAVERSAL", "Path traversal attempt detected").Build(),
        }, nil
    }

    // Continue with sanitized inputs...
}
```

### 2. Secret Handling

```go
func (t *AtomicToolName) Execute(ctx context.Context, args Args) (*Result, error) {
    // Never log sensitive information
    t.logger.Info().
        Str("session_id", args.SessionID).
        Str("operation", "tool_execution").
        // Do NOT log passwords, tokens, or other secrets
        Msg("Starting tool execution")

    // Use secure credential handling
    creds, err := t.getSecureCredentials(ctx, args.CredentialRef)
    if err != nil {
        return nil, err
    }
    defer creds.Clear() // Always clear sensitive data
}
```

## Migration Guidelines

When updating existing tools to follow these standards:

1. **Update imports** to include standardized utilities
2. **Replace local stage definitions** with centralized ones
3. **Implement standardized validation** using validation mixins
4. **Update error creation** to use error builders
5. **Add AI context interfaces** for enhanced decision-making
6. **Update tests** to cover new standardized patterns
7. **Update documentation** to reflect new standards

## Conclusion

Following these standards ensures:

- **Consistency** across all atomic tools
- **Maintainability** through shared utilities and patterns
- **Testability** through standardized interfaces
- **Reliability** through comprehensive error handling
- **Performance** through efficient resource management
- **Security** through consistent input validation and sanitization
- **AI Enhancement** through rich context integration

All new atomic tools must follow these standards, and existing tools should be migrated to comply with these patterns.
