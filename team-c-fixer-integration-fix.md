# Team C - Fix Fixer Integration

## Problem
The tool registry is creating tools without proper initialization:
```go
r.registerTool("build_image_atomic", &build.AtomicBuildImageTool{})
```

This creates tools with nil dependencies, including nil analyzer, which means:
1. Tools don't have the proper clients/analyzer
2. SetAnalyzer is never called
3. Fixing capabilities are non-functional

## Solution

### Option 1: Use Tool Factories
Instead of registering tool instances, register factories that create properly initialized tools:
```go
r.registerToolFactory("build_image_atomic", func(deps ToolDependencies) interface{} {
    tool := build.NewAtomicBuildImageTool(deps.PipelineAdapter, deps.SessionManager, deps.Logger)
    if deps.Analyzer != nil {
        tool.SetAnalyzer(deps.Analyzer)
    }
    return tool
})
```

### Option 2: Initialize Tools with Dependencies
Modify the registry to accept dependencies and create tools properly:
```go
func (r *MCPToolRegistry) registerAtomicTools(deps ToolDependencies) {
    // Create properly initialized tools
    buildTool := build.NewAtomicBuildImageTool(deps.PipelineAdapter, deps.SessionManager, deps.Logger)
    buildTool.SetAnalyzer(deps.Analyzer)
    r.registerTool("build_image_atomic", buildTool)
}
```

### Option 3: Lazy Initialization
Keep the current registration but initialize tools when they're retrieved:
```go
func (r *MCPToolRegistry) GetTool(name string) (interface{}, error) {
    toolInfo := r.tools[name]
    // Create new instance with dependencies
    return r.createToolInstance(toolInfo, r.dependencies)
}
```

## Recommended Approach
Option 1 (Tool Factories) is the cleanest approach as it:
- Separates registration from instantiation
- Allows tools to be created with proper dependencies each time
- Supports different analyzers for different contexts (stub vs caller)