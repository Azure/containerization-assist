# Tool Development Guide

This guide explains how to add new tools to the Container Kit MCP system using the unified interface pattern.

## Table of Contents

1. [Overview](#overview)
2. [Tool Interface](#tool-interface)
3. [Auto-Registration System](#auto-registration-system)
4. [Domain-Specific Examples](#domain-specific-examples)
5. [Testing Your Tool](#testing-your-tool)
6. [Best Practices](#best-practices)

## Overview

Tools in the MCP system implement a unified interface and are automatically discovered and registered at build time. This guide walks through creating tools for different domains.

## Tool Interface

All tools must implement the unified `Tool` interface (or `InternalTool` for internal packages):

```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

### Interface Choice

- **External tools**: Implement `mcp.Tool` from `pkg/mcp/interfaces.go`
- **Internal tools**: Implement `mcptypes.InternalTool` from `pkg/mcp/types/interfaces.go`

## Auto-Registration System

The auto-registration system eliminates the need for manual tool registration by using build-time code generation. This provides automatic tool discovery and registration with zero boilerplate.

### How It Works

1. **Build-Time Discovery**: The registration generator scans the codebase for tool implementations
2. **Automatic Registration**: Generated code registers all discovered tools at startup
3. **Type Safety**: Full compile-time verification of interface implementation
4. **Zero Boilerplate**: No manual registration or adapter code needed

### Step 1: Create Your Tool

Place your tool in the appropriate domain directory:
- `pkg/mcp/internal/build/` - Build and container tools
- `pkg/mcp/internal/deploy/` - Deployment tools
- `pkg/mcp/internal/scan/` - Security scanning tools
- `pkg/mcp/internal/analyze/` - Analysis tools

### Step 2: Implement the Tool Interface

Your tool struct name must end with "Tool" to be discovered:

```go
package build

type MyNewTool struct {  // ✓ Ends with "Tool"
    clients  *adapter.MCPClients
    logger   zerolog.Logger
}

// Implement all required methods
func (t *MyNewTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Implementation
}

func (t *MyNewTool) GetMetadata() mcptypes.ToolMetadata {
    return mcptypes.ToolMetadata{
        Name: "my_new",  // Will be registered as "my_new"
        // ... metadata
    }
}

func (t *MyNewTool) Validate(ctx context.Context, args interface{}) error {
    // Validation
}
```

### Step 3: Run Code Generation

```bash
# Generate registration code
go generate ./...

# Or manually run the generator
go run tools/register_tools.go
```

This will automatically:
- Scan all packages for tools
- Validate interface implementation
- Generate registration code in `pkg/mcp/internal/registry/generated.go`
- Register tools with standardized names (e.g., `BuildImageTool` → `build_image`)

### Tool Naming Convention

Tool names are automatically derived from struct names:
- `BuildImageTool` → `build_image`
- `DeployKubernetesTool` → `deploy_kubernetes`
- `ScanImageSecurityTool` → `scan_image_security`

The conversion follows these rules:
1. Remove "Tool" suffix
2. Convert CamelCase to snake_case
3. Lowercase all characters

## Domain-Specific Examples

### Build Domain Tool

```go
// pkg/mcp/internal/build/optimize_image.go
package build

import (
    "context"
    "fmt"

    "github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
    mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
    "github.com/rs/zerolog"
)

// OptimizeImageTool optimizes container images for size and performance
// +tool:name=optimize_image
// +tool:category=build
// +tool:description=Optimizes Docker images by removing unnecessary layers
type OptimizeImageTool struct {
    clients *adapter.MCPClients
    logger  zerolog.Logger
}

// OptimizeImageArgs defines the arguments for image optimization
type OptimizeImageArgs struct {
    SessionID   string `json:"session_id" description:"Session ID for tracking"`
    ImageName   string `json:"image_name" description:"Name of the image to optimize"`
    TargetSize  string `json:"target_size,omitempty" description:"Target size (e.g., 'small', 'medium', 'minimal')"`
    KeepCache   bool   `json:"keep_cache,omitempty" description:"Keep build cache after optimization"`
}

// OptimizeImageResult contains the optimization results
type OptimizeImageResult struct {
    OriginalSize   int64  `json:"original_size"`
    OptimizedSize  int64  `json:"optimized_size"`
    ReductionRatio float64 `json:"reduction_ratio"`
    OptimizedImage string `json:"optimized_image"`
    Report         string `json:"report"`
}

// NewOptimizeImageTool creates a new image optimization tool
func NewOptimizeImageTool(clients *adapter.MCPClients, logger zerolog.Logger) *OptimizeImageTool {
    return &OptimizeImageTool{
        clients: clients,
        logger:  logger.With().Str("tool", "optimize_image").Logger(),
    }
}

// Execute implements the Tool interface
func (t *OptimizeImageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Convert args
    toolArgs, ok := args.(*OptimizeImageArgs)
    if !ok {
        return nil, fmt.Errorf("invalid arguments type: expected *OptimizeImageArgs, got %T", args)
    }

    t.logger.Info().
        Str("image", toolArgs.ImageName).
        Str("target_size", toolArgs.TargetSize).
        Msg("Starting image optimization")

    // Implementation logic here
    // 1. Analyze current image
    // 2. Identify optimization opportunities
    // 3. Rebuild with optimizations
    // 4. Measure results

    result := &OptimizeImageResult{
        OriginalSize:   1024 * 1024 * 100, // 100MB
        OptimizedSize:  1024 * 1024 * 30,  // 30MB
        ReductionRatio: 0.70,              // 70% reduction
        OptimizedImage: toolArgs.ImageName + "-optimized",
        Report:         "Removed 5 unnecessary layers, optimized package manager cache",
    }

    return result, nil
}

// GetMetadata returns tool metadata
func (t *OptimizeImageTool) GetMetadata() mcptypes.ToolMetadata {
    return mcptypes.ToolMetadata{
        Name:        "optimize_image",
        Description: "Optimizes Docker images by removing unnecessary layers and reducing size",
        Version:     "1.0.0",
        Category:    "build",
        Capabilities: []string{
            "image-analysis",
            "layer-optimization",
            "cache-management",
        },
        Requirements: []string{
            "docker",
        },
        Parameters: map[string]string{
            "image_name":  "required",
            "target_size": "optional",
            "keep_cache":  "optional",
        },
        Examples: []mcptypes.ToolExample{
            {
                Description: "Optimize an image for minimal size",
                Args: map[string]interface{}{
                    "image_name":  "myapp:latest",
                    "target_size": "minimal",
                },
            },
        },
    }
}

// Validate checks if the arguments are valid
func (t *OptimizeImageTool) Validate(ctx context.Context, args interface{}) error {
    toolArgs, ok := args.(*OptimizeImageArgs)
    if !ok {
        return fmt.Errorf("invalid arguments type")
    }

    if toolArgs.ImageName == "" {
        return fmt.Errorf("image_name is required")
    }

    if toolArgs.TargetSize != "" {
        validSizes := []string{"minimal", "small", "medium", "balanced"}
        valid := false
        for _, size := range validSizes {
            if toolArgs.TargetSize == size {
                valid = true
                break
            }
        }
        if !valid {
            return fmt.Errorf("invalid target_size: must be one of %v", validSizes)
        }
    }

    return nil
}

// Ensure interface compliance
var _ mcptypes.InternalTool = (*OptimizeImageTool)(nil)
```

### Deploy Domain Tool

```go
// pkg/mcp/internal/deploy/rollback_deployment.go
package deploy

import (
    "context"
    "fmt"

    "github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
    mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
    "github.com/rs/zerolog"
)

// RollbackDeploymentTool handles Kubernetes deployment rollbacks
// +tool:name=rollback_deployment
// +tool:category=deploy
// +tool:description=Rolls back a Kubernetes deployment to a previous version
type RollbackDeploymentTool struct {
    clients *adapter.MCPClients
    logger  zerolog.Logger
}

// RollbackArgs defines rollback parameters
type RollbackArgs struct {
    SessionID    string `json:"session_id"`
    Namespace    string `json:"namespace"`
    Deployment   string `json:"deployment"`
    ToRevision   int    `json:"to_revision,omitempty"`
    DryRun       bool   `json:"dry_run,omitempty"`
}

// Execute performs the rollback
func (t *RollbackDeploymentTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    toolArgs, ok := args.(*RollbackArgs)
    if !ok {
        return nil, fmt.Errorf("invalid arguments type")
    }

    // Implementation
    // 1. Get deployment history
    // 2. Validate target revision
    // 3. Perform rollback
    // 4. Wait for stability

    return map[string]interface{}{
        "status": "success",
        "previous_revision": 5,
        "new_revision": toolArgs.ToRevision,
        "deployment": toolArgs.Deployment,
    }, nil
}

// GetMetadata returns tool metadata
func (t *RollbackDeploymentTool) GetMetadata() mcptypes.ToolMetadata {
    return mcptypes.ToolMetadata{
        Name:        "rollback_deployment",
        Description: "Rolls back a Kubernetes deployment to a previous version",
        Version:     "1.0.0",
        Category:    "deploy",
        Capabilities: []string{
            "deployment-rollback",
            "revision-management",
            "health-checking",
        },
        Requirements: []string{
            "kubernetes",
        },
    }
}

// Validate ensures arguments are correct
func (t *RollbackDeploymentTool) Validate(ctx context.Context, args interface{}) error {
    toolArgs, ok := args.(*RollbackArgs)
    if !ok {
        return fmt.Errorf("invalid arguments type")
    }

    if toolArgs.Namespace == "" {
        toolArgs.Namespace = "default"
    }

    if toolArgs.Deployment == "" {
        return fmt.Errorf("deployment name is required")
    }

    return nil
}
```

### Scan Domain Tool

```go
// pkg/mcp/internal/scan/scan_vulnerabilities.go
package scan

import (
    "context"
    "fmt"

    mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// VulnerabilityScanTool scans for known vulnerabilities
// +tool:name=scan_vulnerabilities
// +tool:category=scan
// +tool:description=Scans container images for known vulnerabilities
type VulnerabilityScanTool struct {
    // tool implementation
}

// Execute performs vulnerability scanning
func (t *VulnerabilityScanTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Scan implementation
    return map[string]interface{}{
        "vulnerabilities": []map[string]interface{}{
            {
                "cve": "CVE-2023-1234",
                "severity": "high",
                "package": "openssl",
                "fixed_version": "1.1.1u",
            },
        },
        "summary": map[string]int{
            "critical": 0,
            "high": 1,
            "medium": 3,
            "low": 7,
        },
    }, nil
}

// GetMetadata and Validate methods follow similar pattern...
```

### Analyze Domain Tool

```go
// pkg/mcp/internal/analyze/analyze_dependencies.go
package analyze

import (
    "context"

    mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// DependencyAnalyzerTool analyzes project dependencies
// +tool:name=analyze_dependencies
// +tool:category=analyze
// +tool:description=Analyzes and reports on project dependencies
type DependencyAnalyzerTool struct {
    // tool implementation
}

// Execute analyzes dependencies
func (t *DependencyAnalyzerTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Analysis implementation
    return map[string]interface{}{
        "total_dependencies": 42,
        "direct_dependencies": 15,
        "transitive_dependencies": 27,
        "outdated": 5,
        "security_issues": 2,
        "license_issues": 0,
    }, nil
}
```

## Testing Your Tool

### Unit Tests

Create a test file alongside your tool:

```go
// optimize_image_test.go
package build

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestOptimizeImageTool_Execute(t *testing.T) {
    tool := NewOptimizeImageTool(nil, testLogger)

    args := &OptimizeImageArgs{
        SessionID: "test-session",
        ImageName: "test:latest",
        TargetSize: "minimal",
    }

    result, err := tool.Execute(context.Background(), args)
    require.NoError(t, err)

    optimizeResult, ok := result.(*OptimizeImageResult)
    require.True(t, ok)

    assert.Greater(t, optimizeResult.ReductionRatio, 0.0)
    assert.NotEmpty(t, optimizeResult.OptimizedImage)
}

func TestOptimizeImageTool_Validate(t *testing.T) {
    tool := NewOptimizeImageTool(nil, testLogger)

    tests := []struct {
        name    string
        args    interface{}
        wantErr bool
    }{
        {
            name: "valid args",
            args: &OptimizeImageArgs{
                ImageName: "test:latest",
            },
            wantErr: false,
        },
        {
            name: "missing image name",
            args: &OptimizeImageArgs{},
            wantErr: true,
        },
        {
            name: "invalid target size",
            args: &OptimizeImageArgs{
                ImageName: "test:latest",
                TargetSize: "huge",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tool.Validate(context.Background(), tt.args)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

Test your tool with the full system:

```go
func TestOptimizeImageTool_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test environment
    clients := setupTestClients(t)
    tool := NewOptimizeImageTool(clients, testLogger)

    // Build a test image first
    buildResult := buildTestImage(t, clients)

    // Run optimization
    args := &OptimizeImageArgs{
        SessionID: "test-session",
        ImageName: buildResult.ImageName,
        TargetSize: "minimal",
    }

    result, err := tool.Execute(context.Background(), args)
    require.NoError(t, err)

    // Verify optimization worked
    optimizeResult := result.(*OptimizeImageResult)
    assert.Less(t, optimizeResult.OptimizedSize, optimizeResult.OriginalSize)

    // Verify optimized image exists
    exists := verifyImageExists(t, clients, optimizeResult.OptimizedImage)
    assert.True(t, exists)
}
```

## Best Practices

### 1. Error Handling

Use rich errors for better debugging:

```go
import "github.com/Azure/container-copilot/pkg/mcp/types"

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    if err := t.validatePrerequisites(); err != nil {
        return nil, types.NewRichError(
            "PREREQ_FAILED",
            "Prerequisites check failed",
            err,
        ).WithContext(map[string]interface{}{
            "tool": "my_tool",
            "phase": "validation",
        })
    }
    // ...
}
```

### 2. Progress Reporting

For long-running operations, use progress reporting:

```go
func (t *MyTool) ExecuteWithProgress(ctx context.Context, args interface{}, reporter mcptypes.ProgressReporter) (interface{}, error) {
    reporter.ReportStage(0.0, "Starting operation")

    // Step 1: 30% of work
    if err := t.step1(); err != nil {
        return nil, err
    }
    reporter.ReportStage(0.3, "Completed step 1")

    // Step 2: 60% of work
    if err := t.step2(); err != nil {
        return nil, err
    }
    reporter.ReportStage(0.9, "Completed step 2")

    // Finalize
    result := t.finalize()
    reporter.ReportStage(1.0, "Operation complete")

    return result, nil
}
```

### 3. Argument Validation

Always validate arguments thoroughly:

```go
func (t *MyTool) Validate(ctx context.Context, args interface{}) error {
    toolArgs, ok := args.(*MyToolArgs)
    if !ok {
        return fmt.Errorf("expected *MyToolArgs, got %T", args)
    }

    // Required fields
    if toolArgs.RequiredField == "" {
        return fmt.Errorf("required_field is mandatory")
    }

    // Validate enums
    if !isValidOption(toolArgs.Option) {
        return fmt.Errorf("invalid option: %s", toolArgs.Option)
    }

    // Validate ranges
    if toolArgs.Count < 1 || toolArgs.Count > 100 {
        return fmt.Errorf("count must be between 1 and 100")
    }

    return nil
}
```

### 4. Metadata Best Practices

Provide comprehensive metadata:

```go
func (t *MyTool) GetMetadata() mcptypes.ToolMetadata {
    return mcptypes.ToolMetadata{
        Name:        "my_tool",
        Description: "Clear, concise description of what the tool does",
        Version:     "1.0.0",
        Category:    "appropriate-domain",
        Capabilities: []string{
            "capability-1",
            "capability-2",
        },
        Requirements: []string{
            "docker",
            "kubernetes", // if needed
        },
        Parameters: map[string]string{
            "required_param": "required - Description of parameter",
            "optional_param": "optional - Description with default value",
        },
        Examples: []mcptypes.ToolExample{
            {
                Description: "Basic usage example",
                Args: map[string]interface{}{
                    "required_param": "value",
                },
            },
            {
                Description: "Advanced usage with all options",
                Args: map[string]interface{}{
                    "required_param": "value",
                    "optional_param": "custom",
                    "advanced_option": true,
                },
            },
        },
    }
}
```

### 5. Logging Guidelines

Use structured logging:

```go
func NewMyTool(clients *adapter.MCPClients, logger zerolog.Logger) *MyTool {
    return &MyTool{
        clients: clients,
        logger:  logger.With().Str("tool", "my_tool").Logger(),
    }
}

func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    t.logger.Info().
        Str("session_id", toolArgs.SessionID).
        Interface("args", args).
        Msg("Starting tool execution")

    // On error
    if err != nil {
        t.logger.Error().
            Err(err).
            Str("phase", "validation").
            Msg("Tool execution failed")
        return nil, err
    }

    // On success
    t.logger.Info().
        Interface("result", result).
        Dur("duration", time.Since(start)).
        Msg("Tool execution completed")
}
```

### 6. Context Handling

Always respect context cancellation:

```go
func (t *MyTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
    // Check context at start
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // For long operations, check periodically
    for _, item := range items {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            // Process item
            if err := t.processItem(ctx, item); err != nil {
                return nil, err
            }
        }
    }

    return result, nil
}
```

## Tool Registration Flow

1. **Build Time**: `go generate` discovers tools with `+tool:` annotations
2. **Registration**: Tools are registered with the orchestrator at startup
3. **Discovery**: Clients can list available tools and their metadata
4. **Execution**: Tools are executed through the unified orchestrator

## Troubleshooting

### Tool Not Found

If your tool isn't being discovered:

1. **Check struct naming**: Ensure struct name ends with "Tool"
2. **Verify location**: Tool must be in one of the domain directories
3. **Run generation**: Execute `go generate ./...` from repository root
4. **Check generated file**: Verify tool appears in `pkg/mcp/internal/registry/generated.go`
5. **Build errors**: Run `go build ./...` to check for compilation issues

Debug commands:
```bash
# List discovered tools
go run tools/register_tools.go --list

# Verify tool implementation
go run tools/validate_tools.go

# Check registration code
cat pkg/mcp/internal/registry/generated.go | grep "my_tool"
```

### Interface Compliance

Ensure your tool implements all required methods:

```go
// Add this line to catch interface compliance issues at compile time
var _ mcptypes.InternalTool = (*MyTool)(nil)
```

Common interface issues:
- Missing methods (Execute, GetMetadata, Validate)
- Incorrect method signatures
- Wrong return types

### Registration Conflicts

If you get "tool already registered" errors:
1. Ensure unique tool names across all packages
2. Check that tool names don't conflict after snake_case conversion
3. Verify the auto-registration didn't run twice
4. Look for manual registration of the same tool

### Build Integration

Add to your build process:

```makefile
# Makefile
.PHONY: generate
generate:
	go generate ./...

.PHONY: build
build: generate
	go build ./...
```

### Auto-Registration Validation

Validate registration with this script:

```bash
#!/bin/bash
# tools/validate_registration.sh

echo "🔍 Validating auto-registration system..."

# Generate registration
go generate ./...

# Check for compilation errors
if go build ./...; then
    echo "✅ Registration code compiles successfully"
else
    echo "❌ Registration code has compilation errors"
    exit 1
fi

# Verify all tools are registered
go run tools/test_auto_registration.go

echo "✅ Auto-registration validation complete"
```

This completes the tool development guide with comprehensive examples for each domain and best practices for creating robust, well-integrated tools in the MCP system.
