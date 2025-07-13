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
│   ├── bootstrap.go       # Dependency injection setup
│   ├── commands/          # CQRS command handlers
│   ├── queries/           # CQRS query handlers
│   ├── config/            # Application configuration
│   └── session/           # Session management
├── domain/                # Business logic and workflows
│   ├── workflow/          # Core containerization workflow
│   ├── events/            # Domain events and handlers
│   ├── progress/          # Progress tracking (business concept)
│   ├── saga/              # Saga pattern coordination
│   └── sampling/          # Domain sampling contracts
└── infrastructure/        # Technical implementations
    ├── steps/             # Workflow step implementations
    ├── ml/                # Machine learning integrations
    ├── sampling/          # LLM integration
    ├── progress/          # Progress tracking implementations
    ├── prompts/           # MCP prompt management
    ├── resources/         # MCP resource providers
    ├── tracing/           # Observability integration
    ├── utilities/         # Infrastructure utilities
    └── validation/        # Validation implementations
```

### Why This Architecture Works

1. **Clean Dependencies**: Infrastructure → Application → Domain → API
2. **Single Workflow**: `containerize_and_deploy` handles complete process
3. **CQRS Implementation**: Separate command and query handling for scalability
4. **Event-Driven Design**: Domain events for workflow coordination and observability
5. **Saga Orchestration**: Distributed transaction coordination for complex workflows
6. **ML Integration**: Machine learning for build optimization and pattern recognition
7. **Domain-Driven**: Core business logic isolated in domain layer
8. **Separation of Concerns**: Each layer has clear responsibilities
9. **AI-Enhanced**: Built-in AI error recovery and analysis capabilities

## How the Workflow Tool Works

### Unified Workflow Process
Container Kit provides a single, comprehensive workflow with AI orchestration that handles the complete containerization process:

1. **Analyze Repository** (1/10): Repository structure and technology detection
2. **Generate Dockerfile** (2/10): Generate optimized Dockerfile with AI assistance
3. **Build Image** (3/10): Docker image construction with AI-powered error fixing
4. **Setup Kind Cluster** (4/10): Local Kubernetes cluster setup with registry
5. **Load Image** (5/10): Load Docker image into Kubernetes cluster
6. **Generate K8s Manifests** (6/10): Generate Kubernetes deployment manifests
7. **Deploy to K8s** (7/10): Application deployment with AI-powered error recovery
8. **Health Probe** (8/10): Health check and endpoint validation
9. **Vulnerability Scan** (9/10): Security vulnerability scanning with AI analysis (optional)
10. **Finalize Result** (10/10): Workflow completion and cleanup

### Progress Tracking
Each step provides:
- **Progress indicator**: "3/10" style progress with percentage
- **Human-readable message**: "[30%] Analyzing repository structure..."
- **AI-powered error recovery**: Detailed error context with AI suggestions
- **Duration tracking**: Time spent on each step
- **Metadata tracking**: Structured metadata for progress monitoring
- **Event publishing**: Domain events for workflow coordination
- **ML optimization**: Machine learning insights for build improvement

### Workflow Tool Structure
```go
// RegisterWorkflowTools registers the comprehensive containerization workflow
func RegisterWorkflowTools(mcpServer interface {
    AddTool(tool mcp.Tool, handler server.ToolHandlerFunc)
}, logger *slog.Logger) error {
    tool := mcp.Tool{
        Name:        "containerize_and_deploy",
        Description: "Complete end-to-end containerization and deployment with AI-powered error fixing",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "repo_url": map[string]interface{}{
                    "type":        "string",
                    "description": "Repository URL to containerize",
                },
                "deploy": map[string]interface{}{
                    "type":        "boolean",
                    "description": "Deploy to Kubernetes (optional, defaults to true)",
                },
            },
            Required: []string{"repo_url"},
        },
    }

    mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Use new orchestrator-based workflow
        orchestrator := NewOrchestrator(logger)
        result, err := orchestrator.Execute(ctx, &req, &args)
        return result, err
    })
}

// AI-powered orchestrator handles workflow execution with ML optimization
func (o *Orchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error) {
    // Create unified progress tracker
    totalSteps := 10
    progressTracker := infraprogress.NewProgressTracker(ctx, req, totalSteps, o.logger)
    
    // Execute workflow with AI-powered error recovery and event coordination
    return executeContainerizeAndDeploy(ctx, req, args, o.logger)
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
3. **Error handling**: Use Rich error system from `pkg/common/errors/`
4. **Progress tracking**: Include progress indicators with metadata
5. **Event publishing**: Emit domain events for workflow coordination
6. **AI Integration**: Consider AI-powered error recovery where applicable
7. **ML Integration**: Consider ML optimization opportunities
8. **Testing**: Unit tests for the step with both success and failure scenarios

### Step Integration
Steps are integrated into the orchestrator workflow with AI retry logic and event coordination:

```go
// Execute step with AI-powered retry logic and ML optimization
if err := executeStepWithRetryEnhanced(ctx, result, "analyze_repository", 2, func() error {
    logger.Info("Step 1: Analyzing repository", "repo_url", args.RepoURL)

    var err error
    analyzeResult, err = steps.AnalyzeRepository(args.RepoURL, args.Branch, logger)
    if err != nil {
        return fmt.Errorf("repository analysis failed: %v", err)
    }

    // Enhance analysis with AI if available
    if server.ServerFromContext(ctx) != nil {
        logger.Info("Enhancing repository analysis with AI")
        enhancedResult, enhanceErr := steps.EnhanceRepositoryAnalysis(ctx, analyzeResult, logger)
        if enhanceErr == nil {
            analyzeResult = enhancedResult
            logger.Info("Repository analysis enhanced by AI")
        }
    }

    return nil
}, logger, updateProgress, "Analyzing repository structure and detecting language/framework", progressTracker, workflowProgress, errorContext, stateManager); err != nil {
    result.Success = false
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
import "github.com/Azure/container-kit/pkg/common/errors"

func BuildImage(ctx context.Context, args BuildArgs) (*BuildResult, error) {
    // Docker build with AI-powered error recovery
    if err := performBuild(args); err != nil {
        // Use Rich error system from common/errors package
        richErr := errors.New(
            errors.CodeBuildFailed,
            "build",
            "Docker build failed",
            err,
        )
        richErr.Severity = errors.SeverityHigh
        richErr.Fields = map[string]any{
            "image_name": args.ImageName,
            "dockerfile_path": args.DockerfilePath,
            "build_context": args.BuildContext,
        }
        richErr.UserFacing = true
        richErr.Retryable = true
        return nil, richErr
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
// Use the Rich error system from pkg/common/errors
import "github.com/Azure/container-kit/pkg/common/errors"

richErr := errors.New(
    errors.CodeValidationFailed,
    "workflow",
    "invalid repository path",
    err,
)
richErr.Severity = errors.SeverityMedium
richErr.Fields = map[string]any{
    "path": repoPath,
    "validation_rule": "path_exists",
}
richErr.UserFacing = true
richErr.Retryable = false
return richErr
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
2. **Explore Code**: Look at the 4-layer architecture with CQRS and event-driven patterns
3. **Run Tests**: Execute `make test` to see the test suite including property-based tests
4. **Try MCP Client**: Connect with Claude Desktop to see the workflow in action

### For Workflow Development
1. **Choose Your Step**: Decide which step needs modification or enhancement
2. **Understand Context**: See how the step fits in the overall workflow
3. **Implement Changes**: Focus on the specific step logic
4. **Add Tests**: Write comprehensive unit tests for your changes
5. **Integration Test**: Test the complete workflow end-to-end

### For Architecture Understanding
1. **Study 4-Layer Design**: Understand the clean architecture with API/Application/Domain/Infrastructure layers
2. **Trace Workflow Execution**: Follow a workflow call from start to finish through CQRS command handlers
3. **Understand Event Flow**: See how domain events coordinate workflow steps
4. **Review Saga Orchestration**: Learn distributed transaction coordination patterns
5. **Explore ML Integration**: Understand machine learning optimization features
6. **Study Error Handling**: Learn the Rich error patterns from pkg/common/errors

### Resources
- **Documentation**: `docs/` directory contains updated guides
- **Examples**: `pkg/mcp/infrastructure/steps/` has workflow step implementations
- **Tests**: Look at `*_test.go` files for usage patterns
- **Architecture**: See [Four-Layer MCP Architecture](docs/architecture/adr/2025-07-12-four-layer-mcp-architecture.md)
- **ADRs**: Review Architectural Decision Records in `docs/architecture/adr/`

---

Welcome to Container Kit! The 4-layer clean architecture provides a seamless, unified workflow for containerization with AI-powered automation and error recovery. The Domain-Driven Design approach makes it easy to understand, maintain, and extend while keeping business logic separate from technical implementations. Take your time to understand the orchestrator-based workflow pattern, and don't hesitate to ask questions or refer back to this guide as you work with the codebase.