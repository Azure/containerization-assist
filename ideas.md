# Container Kit MCP-Go Integration Improvements

This document outlines opportunities to simplify Container Kit by leveraging built-in mcp-go features instead of custom implementations. Each section includes concrete examples from the current codebase and recommended refactors.

## Table of Contents

1. [Notification & Server Access](#1-notification--server-access)
2. [Transport Layer](#2-transport-layer)
3. [Progress Reporting](#3-progress-reporting)
4. [Tool Registration](#4-tool-registration)
5. [DI / Provider Sets](#5-di--provider-sets)
6. [Clean-up Checklist](#6-clean-up-checklist)
7. [Additional MCP-Go Features](#7-additional-mcp-go-features)
8. [Summary](#summary)

---

## 1. Notification & Server Access

### Current Implementation
Container Kit wraps MCP server notifications with custom interfaces and type conversions.

**File**: `pkg/mcp/domain/workflow/shared.go:14-46`
```go
// NotificationSender is an interface for sending MCP notifications
type NotificationSender interface {
    SendNotificationToClient(ctx context.Context, method string, params interface{}) error
}

// mcpServerWrapper wraps the mcp-go MCPServer to match our interface
type mcpServerWrapper struct {
    server interface {
        SendNotificationToClient(ctx context.Context, method string, params map[string]any) error
    }
}

// SendNotificationToClient implements NotificationSender interface
func (w *mcpServerWrapper) SendNotificationToClient(ctx context.Context, method string, params interface{}) error {
    // Convert params to map[string]any
    paramsMap, ok := params.(map[string]interface{})
    if !ok {
        return errors.NewWorkflowError(...)
    }
    // Convert map[string]interface{} to map[string]any
    anyMap := make(map[string]any, len(paramsMap))
    for k, v := range paramsMap {
        anyMap[k] = v
    }
    return w.server.SendNotificationToClient(ctx, method, anyMap)
}
```

### Why It Can Be Removed
The mcp-go library already provides:
- `server.ServerFromContext(ctx)` to get the server from context
- `SendNotificationToClient` that accepts `map[string]any` directly
- The wrapper only performs redundant type conversion (`map[string]interface{}` → `map[string]any`)

### Recommended Refactor
Replace the entire `shared.go` with direct mcp-go usage:

```go
// File: pkg/mcp/domain/workflow/shared.go
package workflow

import (
    "context"
    "github.com/mark3labs/mcp-go/server"
)

// GetMCPServer returns the *server.MCPServer stored in ctx (or nil)
func GetMCPServer(ctx context.Context) *server.MCPServer {
    return server.ServerFromContext(ctx)
}

// Notify forwards a JSON-serializable payload to the connected client
// It no-ops gracefully when no server is present
func Notify(ctx context.Context, method string, params map[string]any) error {
    if srv := server.ServerFromContext(ctx); srv != nil {
        return srv.SendNotificationToClient(ctx, method, params)
    }
    return nil
}
```

**Impact**: Removes ~90 lines of code including `NotificationSender`, `mcpServerWrapper`, `generateTraceID`, and helper functions.

---

## 2. Transport Layer

### Current Implementation
Container Kit wraps `server.ServeStdio` in extra goroutines and channels.

**File**: `pkg/mcp/application/transport/stdio.go:24-54`
```go
func (t *StdioTransport) ServeStdio(ctx context.Context, mcpServer *server.MCPServer) error {
    t.logger.Info("Starting stdio transport")
    
    // Create error channel for transport
    transportDone := make(chan error, 1)
    
    // Run transport in goroutine
    go func() {
        transportDone <- server.ServeStdio(mcpServer)
    }()
    
    // Wait for context cancellation or transport error
    select {
    case <-ctx.Done():
        t.logger.Info("Stdio transport stopped by context cancellation")
        return ctx.Err()
    case err := <-transportDone:
        if err != nil {
            t.logger.Error("Stdio transport stopped with error", "error", err)
        } else {
            t.logger.Info("Stdio transport stopped gracefully")
        }
        return err
    }
}
```

### Why It Can Be Removed
- `server.ServeStdio` already blocks until EOF or error
- The extra goroutine + channel only adds complexity
- Context cancellation can be handled more simply

### Recommended Refactor
Replace with a simple wrapper:

```go
// File: pkg/mcp/application/transport/stdio.go
package transport

import (
    "context"
    "github.com/mark3labs/mcp-go/server"
    "log/slog"
)

// Serve starts stdio transport and honors ctx cancellation
func Serve(ctx context.Context, s *server.MCPServer, lg *slog.Logger) error {
    lg.Info("stdio transport started")
    go func() { <-ctx.Done(); s.Close() }() // Allow ctx to close the server
    return server.ServeStdio(s)
}
```

**Impact**: Removes ~60 lines including `StdioTransport` struct and methods.

---

## 3. Progress Reporting

### Current Implementation
Container Kit uses custom progress sinks and factories for MCP notifications.

**File**: `pkg/mcp/infrastructure/messaging/progress/mcp_sink.go:18-50`
```go
type MCPSink struct {
    *baseSink
    srv   MCPServerInterface
    token interface{}
}

func (s *MCPSink) Publish(ctx context.Context, u progress.Update) error {
    if s.srv == nil {
        s.logger.Debug("No MCP server in context; skipping progress publish")
        return nil
    }
    
    // Use base sink to build payload
    basePayload := s.buildBasePayload(u)
    basePayload["progressToken"] = s.token
    
    // Throttle heartbeat noise to once every 2s
    if s.shouldThrottleHeartbeat(u, 2*time.Second) {
        return nil
    }
    
    // Convert to map[string]any for MCP server interface
    payload := make(map[string]any)
    // ... more conversion logic
}
```

### Why It Can Be Removed
The mcp-go library provides native progress support:
- `mcp.CallToolRequest` carries a `Meta.ProgressToken`
- Standard pattern uses `mcp.Progress` type directly
- No need for custom conversion or wrapping

### Recommended Refactor
Replace with direct mcp-go progress notifications:

```go
// Simple progress emitter function
type ProgressEmitter func(ctx context.Context, p mcp.Progress) error

// Factory creates progress emitter
func NewProgressEmitter(srv *server.MCPServer, token string) ProgressEmitter {
    return func(ctx context.Context, p mcp.Progress) error {
        if srv == nil {
            return nil
        }
        p.Token = token
        return srv.SendNotificationToClient(ctx, "mcp/progress", p)
    }
}

// Usage in workflow
emitter := NewProgressEmitter(server, req.Meta.ProgressToken)
emitter(ctx, mcp.Progress{
    Percent: 42,
    Stage: "build",
    Message: "building docker image",
})
```

**Impact**: Removes ~180 lines including `MCPSink`, factory logic, and mock types.

---

## 4. Tool Registration

### Current Implementation
Container Kit's tool registration is already well-structured and uses mcp-go appropriately.

**File**: `pkg/mcp/application/workflow/containerize.go` (example)
```go
func RegisterWorkflowTools(s *server.MCPServer, orchestrator domainworkflow.WorkflowOrchestrator, logger *slog.Logger) error {
    containerizeTool := mcp.NewTool(
        "containerize_and_deploy",
        mcp.WithString("repository_path", 
            mcp.Required(), 
            mcp.Description("Absolute path to the repository")),
        mcp.WithString("project_type",
            mcp.Description("Optional: Project type override")),
    )
    
    s.AddTool(containerizeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Handler implementation
    })
}
```

**Status**: ✅ Already optimal - no changes needed.

---

## 5. DI / Provider Sets

### Current State
The Wire-generated dependency injection doesn't use custom wrappers, so removing the above custom code won't affect DI.

### Recommended Action
After removing custom types, run:
```bash
make wire-gen && go vet ./...
```

---

## 6. Clean-up Checklist

### Search and Replace
- [ ] `workflow.NotificationSender` → `*server.MCPServer` (or use `Notify` helper)
- [ ] `getServerFromContext(ctx)` → `server.ServerFromContext(ctx)`
- [ ] `MCPSink` usage → Direct `mcp.Progress` notifications

### Files to Delete/Replace
- [ ] `pkg/mcp/domain/workflow/shared.go` - Replace with simplified version
- [ ] `pkg/mcp/application/transport/stdio.go` - Replace with simplified version
- [ ] `pkg/mcp/infrastructure/messaging/progress/mcp_sink.go` - Delete
- [ ] `pkg/mcp/infrastructure/messaging/progress/factory.go` - Simplify to closure-based emitter
- [ ] Test files mocking old interfaces - Update to use simpler stubs

### Validation
- [ ] Run `make test` - Update compilation errors from removed symbols
- [ ] Run `make test-integration` - Verify functionality remains intact

---

## 7. Additional MCP-Go Features

### 7.1 Panic Recovery
**Current**: Manual defer/recover blocks  
**Replace with**: `server.WithRecovery()` option

```go
// Before: Manual panic handling
defer func() {
    if r := recover(); r != nil {
        log.Errorf("panic: %v", r)
    }
}()

// After: Built-in recovery
s := server.NewMCPServer(
    "Container-Kit", "1.0.0",
    server.WithToolCapabilities(false),
    server.WithRecovery(), // ← Built-in panic recovery
)
```

### 7.2 Session & Hooks
**Current**: Custom session management  
**Replace with**: `server.WithHooks()` and `server.ClientSessionFromContext()`

```go
// Configure hooks for logging/metrics
hooks := server.Hooks{
    OnRequest: func(info server.RequestInfo) {
        logger.Info("→ request", "id", info.RequestID, "method", info.Method)
    },
    OnResponse: func(info server.ResponseInfo) {
        logger.Info("← response", "id", info.RequestID, "duration", info.Duration)
    },
}
s := server.NewMCPServer("Container-Kit", "1.0.0", server.WithHooks(hooks))

// Access session in handlers
func handler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    session := server.ClientSessionFromContext(ctx)
    // Use session...
}
```

### 7.3 Advanced Sampling
**Current**: Custom retry loops  
**Replace with**: Built-in sampling configuration

```go
// Configure advanced sampling
s := server.NewMCPServer(
    "Container-Kit", "1.0.0",
    server.WithSampling(
        mcp.WithMultiRound(),     // Multi-pass sampling
        mcp.WithTopK(5),          // Top-K filtering
        mcp.WithTemperature(0.7), // Sampling temperature
    ),
)
```

### 7.4 Resource Templates
**Current**: Manual URI parsing  
**Replace with**: `mcp.NewResourceTemplate()`

```go
// Define resource template
tmpl := mcp.NewResourceTemplate(
    "container://{namespace}/{name}",
    "Container Resources",
    mcp.WithTemplateDescription("Access container resources by namespace/name"),
)

// Add handler
s.AddResourceTemplate(tmpl, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
    namespace := req.Params.URIParams["namespace"]
    name := req.Params.URIParams["name"]
    // Load and return resource...
})
```

### 7.5 Completions
**Current**: Custom autocomplete  
**Replace with**: Native completion handler

```go
s.OnRequest("completion/complete", func(req mcp.CompleteRequest) mcp.CompleteResponse {
    // Return completion suggestions based on partial input
    return mcp.CompleteResponse{
        Choices: []string{
            "containerize_and_deploy",
            "container_status",
            "container_logs",
        },
    }
})
```

---

## Summary

### Impact Analysis

| Area | Lines Removed | Benefit |
|------|--------------|---------|
| Notification wrapper | ~90 | Direct server usage, no redundant conversions |
| Stdio transport | ~60 | Simpler lifecycle management |
| Progress sink + factory | ~180 | Native mcp.Progress, fewer abstractions |
| **Total** | **~330** | Smaller API surface, better maintainability |

### Key Benefits
1. **Less Code**: ~330 lines removed without losing functionality
2. **Fewer Mocks**: Simpler testing with standard mcp-go types
3. **Better Maintenance**: Rely on well-tested mcp-go implementations
4. **Native Features**: Access to panic recovery, hooks, sampling, etc.
5. **Cleaner Architecture**: Remove unnecessary abstraction layers

### Migration Strategy
1. Start with notification wrapper (easiest, highest impact)
2. Follow with transport layer simplification
3. Refactor progress reporting to use native types
4. Add new mcp-go features (hooks, recovery, etc.)
5. Run comprehensive tests after each change

The refactoring maintains all current functionality while significantly reducing complexity and custom code maintenance burden.