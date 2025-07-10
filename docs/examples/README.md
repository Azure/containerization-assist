# Container Kit Examples

This directory contains examples of using Container Kit for various containerization tasks.

## Quick Start Examples

- [Basic Containerization](basic-containerization.md)
- [Multi-Stage Builds](multi-stage-builds.md)
- [Security Scanning](security-scanning.md)
- [Kubernetes Deployment](kubernetes-deployment.md)

## Advanced Examples

- [Custom Tool Implementation](custom-tool.md)
- [Pipeline Composition](pipeline-composition.md)
- [Workflow Automation](workflow-automation.md)
- [Error Handling Patterns](error-handling.md)

## Code Examples

### Basic Tool Usage

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/Azure/container-kit/pkg/mcp/application/core"
)

func main() {
    // Initialize server
    server, err := core.NewServer(core.ServerConfig{
        Mode: core.ModeDual,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Get tool registry
    registry := server.ToolRegistry()

    // Execute analyze tool
    args := map[string]interface{}{
        "repository": "/path/to/repo",
        "framework": "auto-detect",
    }

    argsJSON, _ := json.Marshal(args)
    result, err := registry.Execute(context.Background(), "analyze", argsJSON)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Analysis result: %s\n", result)
}
```

### Pipeline Example

```go
// Create a build pipeline
pipeline := NewPipeline("container-build").
    AddStage(&AnalyzeStage{}).
    AddStage(&BuildStage{
        EnableCache: true,
        Platforms: []string{"linux/amd64", "linux/arm64"},
    }).
    AddStage(&ScanStage{
        Severity: "CRITICAL,HIGH",
    }).
    AddStage(&PushStage{
        Registry: "myregistry.azurecr.io",
    })

// Execute pipeline
request := &PipelineRequest{
    ID: "build-123",
    Input: map[string]interface{}{
        "repository": "/workspace/myapp",
        "tag": "v1.0.0",
    },
}

response, err := pipeline.Execute(ctx, request)
```

### Session Management

```go
// Create a session
session, err := sessionManager.Create(ctx, SessionConfig{
    Name: "containerization-session",
    Metadata: map[string]string{
        "project": "myapp",
        "environment": "production",
    },
})

// Use session workspace
workspace := session.Workspace
fmt.Printf("Working in: %s\n", workspace)

// Create checkpoint
err = sessionManager.Checkpoint(ctx, session.ID)

// ... do work ...

// Cleanup
err = sessionManager.Delete(ctx, session.ID)
```

## Best Practices

1. **Always use contexts** for cancellation and timeout support
2. **Handle errors properly** using the RichError system
3. **Set appropriate timeouts** for long-running operations
4. **Use sessions** for isolated workspaces
5. **Monitor performance** using built-in metrics

## Getting Help

- [API Documentation](../api/README.md)
- [Architecture Guide](../architecture/README.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)
