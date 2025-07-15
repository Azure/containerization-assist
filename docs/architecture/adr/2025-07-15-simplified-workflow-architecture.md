# ADR-010: Simplified Single Workflow Architecture

## Status
Accepted

## Context
The Container Kit implements a streamlined architecture with a single, comprehensive containerization workflow:

1. **Unified Tool**: One primary MCP tool handles the complete containerization process
2. **Automated Orchestration**: Built-in workflow orchestration eliminates manual tool chaining
3. **Integrated State Management**: Session persistence maintains state throughout the workflow
4. **Superior User Experience**: Simple interface requires no knowledge of internal steps
5. **Robust Error Recovery**: Unified error handling with progressive context

The architecture consolidates the entire containerization pipeline into a single coherent workflow, providing users with a straightforward, reliable experience.

## Decision
The system implements a **Single Workflow Architecture** with one primary tool that handles the complete containerization process:

### Core Design

1. **Single Entry Point**: `containerize_and_deploy` tool
   - Handles complete containerization pipeline from repository analysis to deployment
   - 10-step process with built-in progress tracking
   - Unified error handling and recovery

2. **Workflow Steps**:
   1. **Analyze** (1/10): Repository analysis and technology detection
   2. **Dockerfile** (2/10): Generate optimized Dockerfile
   3. **Build** (3/10): Docker image construction
   4. **Scan** (4/10): Security vulnerability scanning
   5. **Tag** (5/10): Image tagging with version info
   6. **Push** (6/10): Push to container registry
   7. **Manifest** (7/10): Generate Kubernetes manifests
   8. **Cluster** (8/10): Cluster setup and validation
   9. **Deploy** (9/10): Application deployment
   10. **Verify** (10/10): Health check and validation

3. **State Management**: Built-in session persistence with BoltDB
4. **Progress Tracking**: Real-time progress updates through MCP protocol
5. **Error Recovery**: Progressive error context with AI-assisted retry

### Tool Interface
```go
type ContainerizeAndDeployArgs struct {
    RepoURL    string `json:"repo_url"`
    Branch     string `json:"branch,omitempty"`
    Scan       bool   `json:"scan,omitempty"`
    Deploy     bool   `json:"deploy,omitempty"`
    TestMode   bool   `json:"test_mode,omitempty"`
}

type ContainerizeAndDeployResult struct {
    Success      bool           `json:"success"`
    Steps        []WorkflowStep `json:"steps"`
    ImageRef     string         `json:"image_ref,omitempty"`
    Endpoint     string         `json:"endpoint,omitempty"`
    K8sNamespace string         `json:"k8s_namespace,omitempty"`
    Duration     string         `json:"duration"`
    Error        string         `json:"error,omitempty"`
}
```

## Consequences

### Positive
- **Simplified UX**: Single tool call replaces 15+ tool orchestration
- **Better State Management**: Built-in session persistence across workflow execution
- **Unified Error Handling**: Progressive error context with intelligent retry
- **Consistent Progress**: Real-time progress tracking through all steps
- **Reduced Complexity**: Eliminates need for external workflow orchestration
- **Better Testing**: Single workflow path is easier to test end-to-end
- **Improved Reliability**: Built-in error recovery and escalation

### Negative
- **Less Granular Control**: Users cannot easily execute individual steps
- **Longer Execution**: Single call may take 5-10 minutes for complete pipeline
- **Resource Usage**: Higher resource usage during extended execution
- **Debugging Complexity**: Harder to debug individual step failures

### Architecture Impact
1. **Tool Consolidation**: Single primary tool + minimal utilities (ping, server_status)
2. **Client Simplification**: Clients use one tool call for complete containerization
3. **Documentation**: Streamlined documentation focuses on single workflow
4. **Testing**: Comprehensive end-to-end workflow testing

## Implementation Details

### Workflow Execution
```go
func (w *WorkflowOrchestrator) ExecuteWorkflow(
    ctx context.Context, 
    args ContainerizeAndDeployArgs,
) (*ContainerizeAndDeployResult, error) {
    
    session := w.sessionManager.GetOrCreate(ctx, generateSessionID())
    progressEmitter := w.progressFactory.Create(ctx, session.ID)
    
    result := &ContainerizeAndDeployResult{
        Steps: make([]WorkflowStep, 0, 10),
    }
    
    // Execute 10-step workflow with progress tracking
    for i, step := range w.workflowSteps {
        progressEmitter.EmitProgress(i+1, 10, step.Description)
        
        stepResult, err := step.Execute(ctx, session, args)
        result.Steps = append(result.Steps, stepResult)
        
        if err != nil {
            if shouldRetry(err, step.Name) {
                // Progressive error context retry logic
                continue
            }
            return result, err
        }
    }
    
    result.Success = true
    return result, nil
}
```

### Progress Tracking
- Real-time progress updates via MCP progress notifications
- Step-by-step status reporting
- Duration tracking for performance analysis
- Error context accumulation for retry decisions

### Session Management
- BoltDB persistence for workflow state
- Session recovery across server restarts
- Cleanup of expired sessions
- Multi-session support for concurrent workflows

## Utility Tools
The system maintains essential utility tools alongside the main workflow:

1. **ping**: Connectivity testing
2. **server_status**: Server health and statistics  
3. **analyze_repository**: Optional standalone repository analysis (for debugging)

## Performance Characteristics
- **Latency**: 5-10 minutes for complete workflow (expected)
- **Memory**: Bounded by session storage and error context limits
- **Concurrency**: Supports multiple concurrent workflow executions
- **Reliability**: Built-in retry and error recovery mechanisms

## Compliance
This ADR implements:
- **User-Centered Design**: Simplified UX prioritizing user goals over technical implementation
- **Reliability**: Comprehensive error handling and recovery
- **Observability**: Full progress tracking and metrics
- **Maintainability**: Single workflow path easier to maintain and test

## Alternative Considered
**Micro-Tools Architecture**: User feedback strongly indicates preference for the simplified single-workflow approach over granular control through individual tools.

## References
- User experience research on containerization workflows
- Industry patterns for CI/CD pipeline tools
- MCP protocol best practices for long-running operations
- Session management patterns for stateful workflows