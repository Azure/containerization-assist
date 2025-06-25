# Team C: Error Handling Analysis Report

## Current State Analysis

### Error Usage Statistics
- **fmt.Errorf usages**: 218 instances (need replacement)
- **"not yet implemented" stubs**: 3 instances (need implementation)
- **types.NewRichError usages**: 179 instances (already correct)

### Error Types Available
The project has a comprehensive `RichError` system in `/pkg/mcp/internal/types/errors.go`:

#### Available Error Codes
**Build Errors:**
- `ErrCodeBuildFailed` -> "internal_server_error"
- `ErrCodeDockerfileInvalid` -> "invalid_arguments"
- `ErrCodeBuildTimeout` -> "internal_server_error"
- `ErrCodeImagePushFailed` -> "internal_server_error"

**Deployment Errors:**
- `ErrCodeDeployFailed` -> "internal_server_error"
- `ErrCodeManifestInvalid` -> "invalid_arguments"
- `ErrCodeClusterUnreachable` -> "internal_server_error"
- `ErrCodeResourceQuotaExceeded` -> "internal_server_error"

**Analysis Errors:**
- `ErrCodeRepoUnreachable` -> "invalid_request"
- `ErrCodeAnalysisFailed` -> "internal_server_error"
- `ErrCodeLanguageUnknown` -> "invalid_arguments"
- `ErrCodeCloneFailed` -> "internal_server_error"

**System/Session/Security Errors:** (12 additional codes available)

#### Error Type Categories
- `ErrTypeBuild`
- `ErrTypeDeployment`
- `ErrTypeAnalysis`
- `ErrTypeSystem`
- `ErrTypeSession`
- `ErrTypeValidation`
- `ErrTypeSecurity`

## Migration Strategy for Week 3

### Task 1: Replace fmt.Errorf with types.NewRichError

**Current Pattern (Bad):**
```go
return nil, fmt.Errorf("failed to build image: %v", err)
```

**Target Pattern (Good):**
```go
richErr := types.NewRichError(
    types.ErrCodeBuildFailed,
    fmt.Sprintf("failed to build image: %v", err),
    types.ErrTypeBuild,
)
return nil, richErr
```

### Task 2: Remove "not yet implemented" Stubs

**Current Pattern (Bad):**
```go
func (t *SomeTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    return nil, fmt.Errorf("not yet implemented")
}
```

**Target Pattern (Good):**
```go
func (t *SomeTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Actual implementation here
    richErr := types.NewRichError(
        types.ErrCodeAnalysisFailed,
        "tool execution failed",
        types.ErrTypeSystem,
    )
    // Add context, diagnostics, resolution steps
    return result, nil
}
```

### Task 3: Standardize Error Context

**Enhanced Error Pattern:**
```go
richErr := types.NewRichError(code, message, errorType)
richErr.Context.Operation = "build_image"
richErr.Context.Stage = "docker_build"
richErr.Context.Component = "docker_client"
richErr.Context.SetMetadata(sessionID, "build_image", "execute")

// Add diagnostic information
richErr.Diagnostics.RootCause = "dockerfile_syntax_error"
richErr.Diagnostics.Category = "user_input"

// Add resolution steps
richErr.Resolution.ImmediateSteps = []types.ResolutionStep{
    {
        Order:       1,
        Action:      "fix_dockerfile",
        Description: "Fix Dockerfile syntax errors",
        Expected:    "Valid Dockerfile",
    },
}

return nil, richErr
```

## Tool-Specific Error Mapping

### Build Tools
- BuildImageTool: `ErrCodeBuildFailed`, `ErrCodeDockerfileInvalid`, `ErrCodeBuildTimeout`
- PushImageTool: `ErrCodeImagePushFailed`
- TagImageTool: `ErrCodeBuildFailed`

### Deploy Tools
- DeployKubernetesTool: `ErrCodeDeployFailed`, `ErrCodeClusterUnreachable`
- GenerateManifestsTool: `ErrCodeManifestInvalid`
- ValidateDeploymentTool: `ErrCodeManifestInvalid`

### Analysis Tools
- AnalyzeRepositoryTool: `ErrCodeRepoUnreachable`, `ErrCodeAnalysisFailed`, `ErrCodeCloneFailed`
- ValidateDockerfileTool: `ErrCodeDockerfileInvalid`

### Security Tools
- ScanImageSecurityTool: `ErrCodeSecurityVulnerabilities`
- ScanSecretsTool: `ErrCodeSecurityVulnerabilities`

## Implementation Plan

### Phase 1: Automated Detection
Create script to find all fmt.Errorf patterns in tool files:
```bash
grep -r "fmt\.Errorf" pkg/mcp/internal/tools/ | grep -v "_test.go"
```

### Phase 2: Pattern-Based Replacement
For each tool file:
1. Identify the tool's domain (build/deploy/analyze/scan)
2. Map errors to appropriate error codes
3. Replace fmt.Errorf with types.NewRichError
4. Add context and diagnostic information

### Phase 3: Stub Implementation
Find and implement the 3 "not yet implemented" stubs:
```bash
grep -r "not yet implemented" pkg/mcp/internal/tools/
```

### Phase 4: Testing
- Ensure all tools return structured errors
- Verify error context is properly populated
- Test error serialization for LLM consumption

## Quality Gates

### Before (Current State)
- ❌ 218 fmt.Errorf usages
- ❌ 3 unimplemented stubs
- ❌ Inconsistent error handling
- ❌ Poor LLM error reasoning

### After (Target State)
- ✅ 0 fmt.Errorf usages in tools
- ✅ All tools fully implemented
- ✅ Consistent RichError usage
- ✅ Rich context for LLM reasoning
- ✅ Structured error resolution

## Dependencies

This error handling fix depends on:
- Team A completing unified interfaces (defines proper error return types)
- Current RichError system (already available)
- Tool standardization (part of Team C Week 3 work)

## Risk Assessment

**Low Risk:**
- RichError system already exists and is well-tested
- 179 existing usages provide good patterns to follow
- No breaking changes to public APIs

**Benefits:**
- Much better error messages for users
- Improved LLM reasoning about failures
- Consistent error handling across all tools
- Better debugging and observability