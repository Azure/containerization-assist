# ADR-010: AI-Assisted Error Recovery Architecture

Date: 2025-07-11
Status: Accepted
Context: Traditional error handling in containerization workflows often leaves users stranded when builds fail, deployments error, or configurations are invalid. Users typically need to manually interpret error messages, research solutions, and apply fixes. In an AI-assistant context, Container Kit needed an error recovery system designed for AI workflow integration that could provide structured error information and enable automated recovery.

Decision: Implement an AI-assisted error recovery architecture that provides structured error context, automated retry logic, and AI-friendly error information to enable intelligent error recovery workflows. The system integrates with the unified rich error system and provides external AI assistants with actionable error information.

## Architecture Details

### Core Components
- **AI Retry System**: `pkg/mcp/infrastructure/retry/ai_retry.go` - Intelligent retry logic
- **Rich Error Integration**: Structured error context for AI consumption
- **Recovery Strategies**: Built-in recovery patterns for common failure scenarios
- **Workflow Integration**: Seamless integration with single workflow architecture

### AI Retry Logic
```go
type AIRetryConfig struct {
    MaxRetries          int           `json:"max_retries"`
    InitialDelay        time.Duration `json:"initial_delay"`
    MaxDelay            time.Duration `json:"max_delay"`
    BackoffMultiplier   float64       `json:"backoff_multiplier"`
    EnableAIAssistance  bool          `json:"enable_ai_assistance"`
    ContextExtraction   bool          `json:"context_extraction"`
}

type RetryableError interface {
    error
    IsRetryable() bool
    GetRetryStrategy() RetryStrategy
    GetAIContext() map[string]interface{}
}
```

### Error Recovery Workflow
1. **Error Detection**: Structured error capture with rich context
2. **Retry Assessment**: Determine if error is retryable and strategy
3. **Context Extraction**: Gather environment and execution context
4. **AI Communication**: Provide structured error info to AI assistant
5. **Recovery Application**: Apply AI-suggested fixes
6. **Retry Execution**: Re-execute failed operation with modifications

### AI Integration Points
- **Structured Error Export**: JSON serialization for AI consumption
- **Context Enrichment**: Environment, logs, and execution state
- **Fix Suggestion Framework**: Template for AI-generated solutions
- **Retry Coordination**: Handoff between system and AI assistant

## Previous Error Handling Limitations

### Before: Manual Error Resolution
- **Opaque Errors**: Generic error messages without context
- **Manual Research**: Users had to research and apply fixes manually
- **No Retry Logic**: Failed operations required complete restart
- **Limited Context**: Minimal information for troubleshooting
- **AI Incompatibility**: Errors not structured for AI assistant consumption

### Problems Addressed
- **Poor User Experience**: Users stuck when errors occurred
- **No Automation**: No automated recovery or retry mechanisms
- **Context Loss**: Important debugging information lost
- **AI Integration Gap**: Errors not consumable by AI assistants
- **Workflow Interruption**: Errors broke entire workflow execution

## Key Features

### Intelligent Retry Logic
- **Backoff Strategies**: Exponential backoff with jitter
- **Retry Classification**: Automatic retryable vs non-retryable detection
- **Resource Awareness**: Consider system resources in retry decisions
- **Failure Patterns**: Learn from common failure patterns

### AI Assistant Integration
- **Structured Context**: Rich error information for AI processing
- **Fix Templates**: Standardized format for AI-generated solutions
- **Execution Handoff**: Seamless transition between system and AI
- **Learning Integration**: AI feedback improves retry strategies

### Error Context Enrichment
- **Environment State**: Capture system and environment information
- **Execution History**: Previous steps and their outcomes
- **Resource Status**: Container, cluster, and registry status
- **Log Extraction**: Relevant log snippets for error analysis

### Recovery Strategies
- **Build Failures**: Dependency resolution, configuration fixes
- **Deployment Errors**: Resource allocation, networking issues
- **Registry Issues**: Authentication, connectivity problems
- **Cluster Problems**: Node availability, resource constraints

## Implementation Examples

### Dockerfile Build Retry
```go
func (r *AIRetry) RetryDockerBuild(ctx context.Context, buildFunc func() error) error {
    return r.ExecuteWithRetry(ctx, "docker_build", func() error {
        err := buildFunc()
        if err != nil {
            // Extract build context for AI
            buildError := r.enrichBuildError(err)
            // Signal AI for assistance if needed
            if r.shouldRequestAIHelp(buildError) {
                return r.requestAIAssistance(ctx, buildError)
            }
        }
        return err
    })
}
```

### Kubernetes Deployment Recovery
```go
func (r *AIRetry) RetryDeployment(ctx context.Context, deployFunc func() error) error {
    return r.ExecuteWithRetry(ctx, "k8s_deploy", func() error {
        err := deployFunc()
        if err != nil {
            // Gather cluster state for AI analysis
            clusterState := r.gatherClusterState(ctx)
            deployError := r.enrichDeploymentError(err, clusterState)
            
            if r.shouldRequestAIHelp(deployError) {
                return r.requestAIAssistance(ctx, deployError)
            }
        }
        return err
    })
}
```

## Consequences

### Benefits
- **Automated Recovery**: Intelligent retry with context-aware decisions
- **AI Integration**: Seamless handoff to AI assistants for complex issues
- **Better User Experience**: Reduced manual intervention for common failures
- **Learning System**: Improves recovery strategies over time
- **Workflow Continuity**: Errors don't break entire workflow execution
- **Rich Debugging**: Comprehensive context for error analysis

### Trade-offs
- **Complexity**: More complex error handling logic
- **Dependencies**: Requires AI assistant integration
- **Resource Usage**: Additional context gathering and retry attempts
- **Retry Storms**: Potential for excessive retry attempts

### Performance Impact
- **Retry Overhead**: Additional execution time for retries
- **Context Gathering**: Time spent collecting error context
- **AI Communication**: Latency for AI assistant interaction
- **Memory Usage**: Rich error context storage

## Integration with Other Systems

### Rich Error System (ADR-009)
- **Error Enrichment**: AI retry system enhances rich errors with retry context
- **Structured Output**: Rich errors provide AI-consumable format
- **Context Preservation**: Error context maintained across retry attempts

### Single Workflow Architecture (ADR-008)
- **Workflow Integration**: Retry logic embedded in workflow steps
- **Progress Tracking**: Retry attempts tracked in workflow progress
- **State Management**: Workflow state preserved across retry attempts

### MCP Protocol Integration
- **Tool Communication**: MCP protocol carries retry context to AI
- **Progress Updates**: Retry status communicated through MCP
- **Interactive Recovery**: AI can request additional context through MCP

## Implementation Status
- ✅ AI retry system core implementation
- ✅ Integration with rich error system
- ✅ Workflow step retry integration
- ✅ Context extraction for common failure scenarios
- ✅ MCP protocol integration for AI communication
- ✅ Backoff and retry strategy implementation
- 🚧 AI assistant interaction templates (ongoing)
- 🚧 Machine learning for retry optimization (future)

## Usage Guidelines
1. **Error Classification**: Classify errors as retryable/non-retryable
2. **Context Enrichment**: Include relevant debugging context
3. **AI-Friendly Format**: Structure errors for AI consumption
4. **Retry Limits**: Set appropriate retry limits to prevent infinite loops
5. **Fallback Strategies**: Provide manual recovery options when AI fails

## Related ADRs
- ADR-009: Unified Rich Error System (provides structured error foundation)
- ADR-008: Single Workflow Tool Architecture (workflow context for retry)
- ADR-007: Model Context Protocol as Primary Interface (AI communication channel)