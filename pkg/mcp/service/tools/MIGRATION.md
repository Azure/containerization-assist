# Tool Registration Migration Guide

This guide helps migrate from individual tool registration files to the new consolidated table-driven approach.

## Overview

The tool registration has been consolidated from 13 individual files to a single table-driven system that:
- Reduces code duplication by 80%
- Centralizes tool configuration
- Simplifies adding new tools
- Improves consistency

## ⚠️ IMPORTANT: No Backward Compatibility

**This migration follows a clean-break approach. There is NO backward compatibility layer.**

Why?
- Backward compatibility adds unnecessary complexity
- It perpetuates old patterns we're trying to eliminate
- Clean migrations force immediate adoption of better patterns
- Reduces technical debt accumulation

## Migration Steps

### 1. Update All Imports Immediately

Replace ALL occurrences:
```go
// Old - DELETE THIS
import (
    "github.com/Azure/container-kit/pkg/mcp/application/registrar"
)

// New - USE THIS
import (
    "github.com/Azure/container-kit/pkg/mcp/service/tools"
)
```

### 2. Update All Registration Code

Replace individual tool registrations:
```go
// Old approach - DELETE THIS PATTERN
registrar.RegisterAnalyzeRepositoryTool(mcpServer, stepProvider, progressFactory, sessionManager, logger)
registrar.RegisterGenerateDockerfileTool(mcpServer, stepProvider, progressFactory, sessionManager, logger)
registrar.RegisterBuildImageTool(mcpServer, stepProvider, progressFactory, sessionManager, logger)
// ... more individual registrations

// New approach - USE THIS INSTEAD
deps := tools.ToolDependencies{
    StepProvider:    stepProvider,
    ProgressFactory: progressFactory,
    SessionManager:  sessionManager,
    Logger:          logger,
}

err := tools.RegisterTools(mcpServer, deps)
if err != nil {
    return errors.Wrap(err, "failed to register tools")
}
```

### 3. Delete Old Registration Files

Remove these files immediately:
```bash
rm -rf pkg/mcp/application/registrar/
```

This includes:
- analyze_repository.go
- build_image.go
- deploy_application.go
- generate_dockerfile.go
- generate_k8s_manifests.go
- list_tools.go
- prepare_cluster.go
- push_image.go
- scan_image.go
- start_workflow.go
- tag_image.go
- verify_deployment.go
- workflow_status.go

### 4. Update Wire Dependencies

If using Wire for dependency injection:
```go
// Old - DELETE
wire.Build(
    registrar.RegisterAnalyzeRepositoryTool,
    registrar.RegisterGenerateDockerfileTool,
    // ... etc
)

// New - ADD
wire.Build(
    tools.RegisterTools,
)
```

## Adding New Tools

With the new system, adding a tool requires only configuration:

### 1. Add Tool Configuration
```go
// In pkg/mcp/service/tools/registry.go
var toolConfigs = []ToolConfig{
    // ... existing tools
    {
        Name:                 "new_tool",
        Description:          "Description of the new tool",
        Category:             CategoryWorkflow,
        RequiredParams:       []string{"session_id", "custom_param"},
        OptionalParams:       map[string]interface{}{"option1": "string"},
        NeedsStepProvider:    true,
        NeedsProgressFactory: true,
        NeedsSessionManager:  true,
        NeedsLogger:          true,
        StepGetterName:       "GetNewToolStep",
        NextTool:             "next_tool_in_chain",
        ChainReason:          "New tool completed successfully",
    },
}
```

### 2. Add Step Provider Method
```go
// In the StepProvider interface implementation
func (p *StepProviderImpl) GetNewToolStep() domainworkflow.Step {
    return &NewToolStep{
        // step implementation
    }
}
```

That's it! No new files, no boilerplate.

## Custom Tool Handlers

For tools needing special logic:
```go
{
    Name:        "special_tool",
    Description: "Tool with custom logic",
    Category:    CategoryUtility,
    CustomHandler: func(deps ToolDependencies) mcp.HandlerFunc {
        return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            // Custom implementation
        }
    },
}
```

## Common Migration Errors

### Error: "undefined: registrar"
**Fix**: Update imports to use `tools` package

### Error: "RegisterAnalyzeRepositoryTool not found"
**Fix**: Use `tools.RegisterTools()` to register all tools at once

### Error: Multiple tool registration
**Fix**: Remove individual registrations, use single `RegisterTools()` call

## Benefits of Clean Migration

1. **Immediate simplification**: No transition period complexity
2. **Force adoption**: No option to use old patterns
3. **Clean codebase**: No legacy code lingering
4. **Clear ownership**: New pattern is the only pattern

## Checklist

- [ ] All imports updated to use `tools` package
- [ ] All individual registrations replaced with `RegisterTools()`
- [ ] Old `registrar` directory deleted
- [ ] No references to old registration functions
- [ ] Tests updated to use new pattern
- [ ] CI/CD updated if it references old code

## Remember

**DO NOT ADD BACKWARD COMPATIBILITY**

If you're tempted to add compatibility shims:
1. Don't
2. Update all code to use the new pattern
3. Delete the old code
4. Move forward

This approach ensures clean, maintainable code without technical debt.