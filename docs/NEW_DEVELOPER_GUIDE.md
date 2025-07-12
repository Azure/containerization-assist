# New Developer Guide - Container Kit MCP Server

Welcome to Container Kit! This guide will help you understand our streamlined MCP (Model Context Protocol) architecture, how it's organized, and the design principles that make it maintainable and effective.

## Table of Contents

1. [What is MCP and Why Do We Use It?](#what-is-mcp-and-why-do-we-use-it)
2. [Understanding the Simplified Architecture](#understanding-the-simplified-architecture)
3. [How the Workflow Tool Works](#how-the-workflow-tool-works)
4. [Workflow Step Development](#workflow-step-development)
5. [Session Management](#session-management)
6. [Development Workflow](#development-workflow)
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
- **Unified Workflow**: Single tool for complete containerization process
- **Session Management**: Persistent state across workflow steps
- **Type Safety**: Strongly typed interfaces with JSON schema validation
- **AI Integration**: Seamless integration with AI assistants like Claude

### MCP in Action
When you use Container Kit with Claude Desktop or other MCP clients, you're using the unified workflow tool:
- `containerize_and_deploy` - Complete containerization workflow from analysis to deployment

## Understanding the Simplified Architecture

Container Kit uses a clean 4-layer architecture following Domain-Driven Design principles that delivers complete containerization functionality with AI-powered automation.

### Four-Layer Clean Architecture

```
pkg/mcp/
├── api/                    # Interface definitions and contracts
│   └── interfaces.go       # Essential MCP tool interfaces
├── application/            # Application services and orchestration
│   ├── server.go          # MCP server implementation
│   ├── chat_mode.go       # Chat mode integration
│   └── session/           # Session management
├── domain/                # Business logic and workflows
│   ├── workflow/          # Core containerization workflow
│   ├── errors/            # Rich error handling system
│   ├── progress/          # Progress tracking (business concept)
│   └── elicitation/       # User input gathering (business process)
└── infrastructure/        # Technical implementations
    ├── steps/             # Workflow step implementations
    ├── analysis/          # Repository analysis
    ├── retry/             # AI-powered retry logic
    ├── security/          # Security utilities
    ├── sampling/          # LLM integration
    ├── prompts/           # MCP prompt management
    └── resources/         # MCP resource providers
```

### Why This Architecture Works

1. **Clean Dependencies**: Infrastructure → Application → Domain → API
2. **Single Workflow**: `containerize_and_deploy` handles complete process
3. **Domain-Driven**: Core business logic isolated in domain layer
4. **Separation of Concerns**: Each layer has clear responsibilities
5. **AI-Enhanced**: Built-in AI error recovery and analysis capabilities

## How the Workflow Tool Works

### Unified Workflow Process
Container Kit provides a single, comprehensive workflow with AI orchestration that handles the complete containerization process:

1. **Analyze** (1/9): Repository structure and technology detection
2. **Dockerfile** (2/9): Generate optimized Dockerfile with AI assistance
3. **Build** (3/9): Docker image construction with AI-powered error fixing
4. **Scan** (4/9): Security vulnerability scanning with AI analysis
5. **Setup Cluster** (5/9): Local Kubernetes cluster setup with registry
6. **Load Image** (6/9): Load Docker image into Kubernetes cluster
7. **Generate Manifests** (7/9): Generate Kubernetes deployment manifests
8. **Deploy** (8/9): Application deployment with AI-powered error recovery
9. **Verify** (9/9): Health check and endpoint validation

### Progress Tracking
Each step provides:
- **Progress indicator**: "3/9" style progress with percentage
- **Human-readable message**: "[33%] Analyzing repository structure..."
- **AI-powered error recovery**: Detailed error context with AI suggestions
- **Duration tracking**: Time spent on each step
- **Metadata tracking**: Structured metadata for progress monitoring

### Workflow Tool Structure
```go
// RegisterWorkflowTools registers the comprehensive containerization workflow
func RegisterWorkflowTools(mcpServer *server.MCPServer, logger *slog.Logger) error {
	tool := mcp.Tool{
		Name:        "containerize_and_deploy",
		Description: "Complete containerization workflow from analysis to deployment",
	}

	mcpServer.RegisterTool(tool, func(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
		// Use new orchestrator-based workflow
		orchestrator := NewOrchestrator(logger)
		result, err := orchestrator.Execute(ctx, &req, &args)
		return result, err
	})
}

// AI-powered orchestrator handles workflow execution
type Orchestrator struct {
    logger *slog.Logger
}

func (o *Orchestrator) Execute(ctx context.Context, req *ContainerizeAndDeployRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
    // Create unified progress tracker
    totalSteps := 9
    progressTracker := progress.NewProgressTracker(ctx, req, totalSteps, o.logger)
    
    // Execute workflow with AI-powered error recovery
    return o.executeWorkflowWithProgress(ctx, req, args, progressTracker)
}
```

## Workflow Step Development

### Step Implementation Pattern
Each workflow step is implemented in `pkg/mcp/infrastructure/steps/`:

```go
// pkg/mcp/infrastructure/steps/analyze.go
func AnalyzeRepository(ctx context.Context, args AnalyzeArgs) (*AnalyzeResult, error) {
    // Repository analysis logic with optional AI enhancement
    result := &AnalyzeResult{
        Language:    detectedLanguage,
        Framework:   detectedFramework,
        Port:        detectedPort,
        Dependencies: dependencies,
    }
    
    // Optional AI enhancement for better recommendations
    if aiAnalysis, err := enhanceWithAI(ctx, result); err == nil {
        result.AIEnhanced = true
        result.Recommendations = aiAnalysis.Recommendations
    }
    
    return result, nil
}
```

### Adding New Steps
1. **Create step file**: Add to `pkg/mcp/infrastructure/steps/`
2. **Implement logic**: Focus on the specific operation with AI integration
3. **Error handling**: Use unified RichError system from `pkg/mcp/domain/errors/`
4. **Progress tracking**: Include progress indicators with metadata
5. **AI Integration**: Consider AI-powered error recovery where applicable
6. **Testing**: Unit tests for the step with both success and failure scenarios

### Step Integration
Steps are integrated into the orchestrator workflow with AI retry logic:

```go
// Execute step with AI-powered retry logic
if err := executeStepWithRetry(ctx, result, "analyze", 2, func() error {
    var err error
    analyzeResult, err = steps.AnalyzeRepository(analyzeArgs, logger)
    if err != nil {
        return fmt.Errorf("repository analysis failed: %v", err)
    }
    
    // Optional AI enhancement
    if enhanced, err := steps.AnalyzeEnhance(analyzeResult, logger); err == nil {
        analyzeResult = enhanced
        logger.Info("Repository analysis enhanced by AI")
    }
    
    return nil
}, logger, updateProgress, "Analyzing repository structure", progressTracker, workflowProgress); err != nil {
    return result, nil
}
```

## Session Management

### Why Sessions Matter
Sessions provide essential functionality:
- **State Persistence**: Maintain context across workflow steps
- **Workspace Isolation**: Each session gets its own workspace
- **Error Recovery**: Resume from failed steps
- **Progress Tracking**: Monitor workflow progress

### Session Architecture
- **Storage**: BoltDB for lightweight, embedded persistence
- **Isolation**: Each session gets its own workspace directory
- **Metadata**: Labels for organization and querying
- **Lifecycle**: Automatic cleanup and expiration

### Session Usage Pattern
```go
// Create or retrieve session
session, err := sessionManager.CreateSession(ctx, &SessionRequest{
    Labels: map[string]string{
        "project": "my-app",
        "workflow": "containerize_and_deploy",
    },
})

// Use session in workflow
result, err := tool.Execute(ctx, WorkflowInput{
    SessionID: session.ID,
    RepoPath: "/path/to/repo",
    Registry: "my-registry.com",
})
```

## Development Workflow

### 1. Understanding the Workflow
Start by understanding the 10-step process:
- Each step has a specific responsibility
- Steps are executed sequentially with progress tracking
- Error recovery is built into each step

### 2. Working with Existing Steps
```go
// pkg/mcp/infrastructure/steps/build.go
func BuildImage(ctx context.Context, args BuildArgs) (*BuildResult, error) {
    // Docker build with AI-powered error recovery
    if err := performBuild(args); err != nil {
        // Use unified RichError system from domain layer
        return nil, errors.NewError().
            Code(errors.CodeBuildFailed).
            Type(errors.ErrTypeBuild).
            Severity(errors.SeverityHigh).
            Message("Docker build failed").
            Context("image", args.ImageName).
            Context("dockerfile", args.DockerfilePath).
            Suggestion("Check Dockerfile syntax and dependencies").
            WithLocation().
            Build()
    }
    
    return &BuildResult{
        ImageName: args.ImageName,
        ImageTag:  args.ImageTag,
        Success:   true,
    }, nil
}
```

### 3. Adding New Functionality
For new features, consider:
- **New step**: Add to the workflow if it's a new phase
- **Enhanced step**: Modify existing step for related functionality
- **Configuration**: Add parameters to workflow arguments

### 4. Testing Your Changes
```go
// Test individual steps
func TestAnalyzeRepository(t *testing.T) {
    result, err := steps.AnalyzeRepository(ctx, AnalyzeArgs{
        RepoPath: "testdata/sample-repo",
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, result.TechnologyStack)
}

// Test complete workflow
func TestContainerizeAndDeploy(t *testing.T) {
    tool := &ContainerizeAndDeployTool{
        workspaceDir: "/tmp/test-workspace",
    }
    
    result, err := tool.Execute(ctx, ContainerizeAndDeployArgs{
        RepoPath: "testdata/sample-app",
        Registry: "localhost:5000",
    })
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
}
```

## Code Organization Rationale

### Why a Single Workflow Tool
The unified workflow approach provides:
- **Simplicity**: One tool instead of many atomic tools
- **User Experience**: Complete process with progress tracking
- **Reduced Coordination**: No need to orchestrate multiple tools
- **Error Recovery**: Centralized error handling and recovery

### Why Step-Based Implementation
Steps are separate files because:
- **Single Responsibility**: Each step has one clear purpose
- **Easy Navigation**: Developers can quickly find specific functionality
- **Independent Testing**: Each step can be tested in isolation
- **Reusability**: Steps can be reused in different workflows

### Why We Use the Rich Error System
The unified error handling system provides:
- **Core Infrastructure**: Essential component used throughout codebase
- **Rich Context**: Structured error information
- **Actionable Messages**: Clear guidance for resolution
- **Consistent Handling**: Standardized error patterns

### Design Principles
Our architecture follows these principles:
- **Direct Implementation**: Clear, understandable code paths
- **Essential Validation**: Only what's necessary for reliability
- **Unified Workflow**: Single process instead of coordinated tools
- **Minimal Abstraction**: Direct implementation for maintainability

## Common Patterns and Examples

### Error Handling Pattern
```go
// Use the unified RichError system
return errors.NewError().
    Code(errors.CodeValidationFailed).
    Type(errors.ErrTypeValidation).
    Severity(errors.SeverityMedium).
    Message("invalid repository path").
    Context("path", repoPath).
    Suggestion("Ensure the path exists and is accessible").
    WithLocation().
    Build()
```

### Progress Tracking Pattern
```go
// Update progress during workflow execution
func (t *ContainerizeAndDeployTool) executeStepWithRetry(ctx context.Context, stepName string, stepIndex int) error {
    progress := fmt.Sprintf("%d/%d", stepIndex+1, len(t.steps))
    message := fmt.Sprintf("Executing step: %s", stepName)
    
    // Update workflow status
    t.updateProgress(progress, message)
    
    // Execute step with retry logic
    return t.executeWithRetry(ctx, stepName)
}
```

### Session Context Pattern
```go
// Always use session context for operations
func (t *ContainerizeAndDeployTool) Execute(ctx context.Context, args ContainerizeAndDeployArgs) (interface{}, error) {
    // Create or get session
    session, err := t.getOrCreateSession(ctx, args.SessionID)
    if err != nil {
        return nil, err
    }
    
    // Use session workspace for file operations
    workspaceDir := session.WorkspaceDir
    
    // Update session state as workflow progresses
    for i, step := range t.steps {
        session.CurrentStep = step
        session.Progress = fmt.Sprintf("%d/%d", i+1, len(t.steps))
        t.updateSession(ctx, session)
        
        if err := t.executeStep(ctx, step); err != nil {
            session.LastError = err.Error()
            t.updateSession(ctx, session)
            return nil, err
        }
    }
    
    return WorkflowResult{Success: true}, nil
}
```

## Debugging and Troubleshooting

### Common Issues and Solutions

#### Workflow Step Failures
**Problem**: Specific step in workflow fails
**Solution**: Check step-specific logs and AI error analysis
**Debug**: Look at step implementation in `pkg/mcp/infrastructure/steps/`
**AI Recovery**: Check if AI retry logic provided actionable suggestions

#### Progress Not Updating
**Problem**: Workflow progress seems stuck
**Solution**: Verify progress tracking calls in step execution
**Debug**: Enable debug logging to see progress updates

#### Session Errors
**Problem**: Session operations fail
**Solution**: Check session ID validity and BoltDB connection
**Debug**: Enable session logging to see database operations

#### Build System Issues
**Problem**: Make commands fail
**Solution**: Use simplified make targets
**Debug**: Run `make help` to see available targets

### Debugging Tools
- **Make Commands**: Use `make test`, `make lint`, `make build`
- **Logging**: Enable debug logging with environment variables
- **MCP Client**: Test workflow directly with MCP-compatible clients
- **Unit Tests**: Run focused tests with `go test -v ./pkg/mcp/...`

### Available Make Targets
```bash
# Build and test
make build              # Build MCP server
make test               # Unit tests
make test-integration   # Integration tests

# Code quality
make fmt                # Format code
make lint               # Lint code
make clean              # Clean build artifacts

# Utility
make version            # Show version
```

## Next Steps

### For New Contributors
1. **Understand the Workflow**: Review the 10-step containerization process
2. **Explore Code**: Look at the 25 core files in the simplified architecture
3. **Run Tests**: Execute `make test` to see the test suite
4. **Try MCP Client**: Connect with Claude Desktop to see the workflow in action

### For Workflow Development
1. **Choose Your Step**: Decide which step needs modification or enhancement
2. **Understand Context**: See how the step fits in the overall workflow
3. **Implement Changes**: Focus on the specific step logic
4. **Add Tests**: Write comprehensive unit tests for your changes
5. **Integration Test**: Test the complete workflow end-to-end

### For Architecture Understanding
1. **Study Simplification**: See how complex architecture was simplified
2. **Trace Workflow Execution**: Follow a workflow call from start to finish
3. **Understand Sessions**: See how state persists across workflow steps
4. **Review Error Handling**: Learn the RichError patterns that were retained

### Resources
- **Documentation**: `docs/` directory contains updated guides
- **Examples**: `pkg/mcp/infrastructure/steps/` has workflow step implementations
- **Tests**: Look at `*_test.go` files for usage patterns
- **Architecture**: See [Four-Layer MCP Architecture](docs/architecture/adr/2025-07-12-four-layer-mcp-architecture.md)
- **ADRs**: Review Architectural Decision Records in `docs/architecture/adr/`

---

Welcome to Container Kit! The 4-layer clean architecture provides a seamless, unified workflow for containerization with AI-powered automation and error recovery. The Domain-Driven Design approach makes it easy to understand, maintain, and extend while keeping business logic separate from technical implementations. Take your time to understand the orchestrator-based workflow pattern, and don't hesitate to ask questions or refer back to this guide as you work with the codebase.