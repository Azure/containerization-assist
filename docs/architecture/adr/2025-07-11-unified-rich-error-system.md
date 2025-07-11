# ADR-009: Unified Rich Error System with Code Generation

Date: 2025-07-11
Status: Accepted
Context: Container Kit originally had multiple competing error handling systems scattered across different packages, leading to inconsistent error reporting, poor debugging experience, and lack of structured error context for AI-assisted recovery. The system needed a unified approach to error handling that could provide rich context for both human operators and AI assistants.

Decision: Implement a unified rich error system with code generation that provides structured error context, severity levels, actionable messages, and AI-friendly error information. The system uses YAML-based error code definitions with Go code generation to ensure type safety and consistency.

## Architecture Details

### Core Components
- **Rich Error Type**: `pkg/common/errors/errors.go` - Structured error with metadata
- **Code Generation**: `codes.yaml` → `codes_gen.go` - Type-safe error codes
- **Builder Pattern**: Fluent API for constructing rich errors
- **AI Integration**: Structured context for AI-assisted error recovery

### Rich Error Structure
```go
type RichError struct {
    Code         ErrorCode             `json:"code"`
    Type         ErrorType             `json:"type"`
    Severity     ErrorSeverity         `json:"severity"`
    Message      string                `json:"message"`
    Context      map[string]interface{} `json:"context"`
    Suggestions  []string              `json:"suggestions"`
    Location     *ErrorLocation        `json:"location,omitempty"`
    Cause        error                 `json:"cause,omitempty"`
    Timestamp    time.Time             `json:"timestamp"`
    Component    string                `json:"component"`
    Operation    string                `json:"operation"`
}
```

### Builder Pattern Usage
```go
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("validation failed").
    Context("field", fieldName).
    Suggestion("Check field format").
    WithLocation().
    Build()
```

### Code Generation System
- **Input**: `pkg/common/errors/codes.yaml` - Human-readable error definitions
- **Output**: `pkg/common/errors/codes_gen.go` - Type-safe Go constants
- **Categories**: Validation, Infrastructure, Business Logic, Security
- **Severity Levels**: Critical, High, Medium, Low, Info

## Previous Architecture Issues

### Before: Multiple Error Systems
- **Standard Go errors**: Basic error strings with no context
- **Custom error types**: Inconsistent across packages
- **Logging integration**: Ad-hoc error logging patterns
- **No AI integration**: Errors not structured for AI consumption
- **Poor debugging**: Limited context and debugging information

### Problems Addressed
- **Inconsistent Error Handling**: 3+ different error patterns across codebase
- **Poor Context**: Errors lacked structured information for debugging
- **No Retry Logic**: No standardized way to determine if errors are retryable
- **AI Incompatibility**: Errors not structured for AI assistant integration
- **Debugging Difficulty**: No standardized location or context information

## Key Features

### Structured Context
- **Typed Context**: Strongly typed context information
- **Location Tracking**: Automatic file/line/function capture
- **Component Identification**: Clear component and operation context
- **Causal Chains**: Error wrapping with cause preservation

### AI-Friendly Design
- **Structured JSON**: Errors serialize to structured JSON for AI consumption
- **Fix Suggestions**: Built-in actionable suggestions for common issues
- **Severity Classification**: Helps AI prioritize error resolution
- **Retry Indicators**: Clear signals for AI retry logic

### Developer Experience
- **Builder Pattern**: Fluent API for error construction
- **Code Generation**: Type-safe error codes from YAML definitions
- **IDE Integration**: Full IDE support with auto-completion
- **Consistent Formatting**: Standardized error message formatting

## Consequences

### Benefits
- **Consistent Error Handling**: Single pattern across entire codebase
- **Rich Debugging Context**: Structured information for troubleshooting
- **AI Integration Ready**: Errors designed for AI-assisted recovery
- **Type Safety**: Generated error codes prevent typos and inconsistencies
- **Better Monitoring**: Structured errors enable better observability
- **Easier Maintenance**: Centralized error code definitions

### Trade-offs
- **Learning Curve**: Developers need to learn builder pattern
- **Verbosity**: More verbose than simple error strings
- **Build Process**: Adds code generation step to build process
- **Memory Overhead**: Rich errors use more memory than simple errors

### Performance Impact
- **Minimal Runtime Cost**: Builder pattern is compile-time overhead
- **JSON Serialization**: Additional cost for AI integration scenarios
- **Memory Usage**: Structured errors use more memory but provide more value
- **Location Capture**: Optional location capture has minimal performance impact

## Implementation Status
- ✅ Rich error type with builder pattern implemented
- ✅ YAML-based error code definitions
- ✅ Go code generation from YAML definitions
- ✅ AI-friendly JSON serialization
- ✅ Location tracking and context capture
- ✅ Integration with retry system (see ADR-010)
- ✅ Used by 54+ files across the codebase

## Usage Guidelines
1. **Use Builder Pattern**: Always use the fluent builder API
2. **Include Context**: Add relevant context information for debugging
3. **Provide Suggestions**: Include actionable fix suggestions when possible
4. **Set Appropriate Severity**: Use severity levels to guide error handling
5. **Preserve Cause**: Wrap underlying errors to maintain error chains

## Related ADRs
- ADR-010: AI-Assisted Error Recovery (consumes rich error context)
- ADR-008: Single Workflow Tool Architecture (unified error handling across workflow)
- ADR-011: Manual Dependency Injection (simplified error propagation)