# Legacy Implementation References

When implementing missing functionality in the MCP module, consult these legacy pipeline components for patterns and implementation guidance:

## Docker Operations
**Location**: `pkg/pipeline/dockerstage/`
- Docker build patterns and error handling
- Build-fix loop implementations
- Registry operations (push, pull, tag)
- Dockerfile generation and validation

## Kubernetes Operations  
**Location**: `pkg/pipeline/manifeststage/`
- Kubernetes manifest generation patterns
- Deployment fix patterns and error handling
- Secret manifest generation
- Deploy-fix loop implementations
- Pod health validation

## Repository Analysis
**Location**: `pkg/pipeline/repoanalysisstage/`
- Repository structure analysis patterns
- Language and framework detection
- Dependency identification
- Configuration analysis

## Core Components
- **Pipeline Orchestration**: `pkg/pipeline/runner.go` - Workflow patterns, dependency resolution
- **State Management**: `pkg/pipeline/state.go` - Session state patterns, metadata handling
- **Snapshot System**: `pkg/pipeline/snapshot.go` - Iteration snapshots, debugging aids
- **AI Integration**: `pkg/ai/` - Azure OpenAI client patterns, tool-calling examples

## Common Patterns to Reference

### Error Handling and Fixes
Look for `*Fix()` methods in the pipeline stages to understand how errors are analyzed and remediated.

### Variable Handling
Check how context and variables are passed between pipeline stages for workflow implementation.

### Session Management
Review how pipeline state is persisted and retrieved across iterations.

### Tool Integration
Examine how external tools (Docker, kubectl, Kind) are wrapped and errors are handled.

These implementations contain working, tested code that can guide the MCP atomic tool development. Focus on the patterns rather than copying code directly, as the MCP architecture uses different abstractions.