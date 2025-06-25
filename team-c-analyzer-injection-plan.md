# Team C - Analyzer Injection Plan

## Current Issue
The ToolFactory creates tools but doesn't inject the analyzer, so tools can't use fixing capabilities.

## Solution: Update ToolFactory

### 1. Add analyzer field to ToolFactory
```go
type ToolFactory struct {
    pipelineOperations mcptypes.PipelineOperations
    sessionManager     *session.SessionManager
    analyzer          mcptypes.AIAnalyzer  // Add this
    logger            zerolog.Logger
}
```

### 2. Update constructor
```go
func NewToolFactory(
    pipelineOperations mcptypes.PipelineOperations,
    sessionManager *session.SessionManager,
    analyzer mcptypes.AIAnalyzer,  // Add this parameter
    logger zerolog.Logger,
) *ToolFactory {
    return &ToolFactory{
        pipelineOperations: pipelineOperations,
        sessionManager:     sessionManager,
        analyzer:          analyzer,
        logger:            logger,
    }
}
```

### 3. Update each Create method to call SetAnalyzer
```go
func (f *ToolFactory) CreateBuildImageTool() *build.AtomicBuildImageTool {
    tool := build.NewAtomicBuildImageTool(f.pipelineOperations, f.sessionManager, f.logger)
    if f.analyzer != nil {
        tool.SetAnalyzer(f.analyzer)
    }
    return tool
}
```

## Files to Update
1. pkg/mcp/internal/orchestration/tool_factory.go - Add analyzer support
2. All places that create ToolFactory - Pass analyzer parameter