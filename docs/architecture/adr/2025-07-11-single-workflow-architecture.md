# ADR-001: Single Workflow Tool Architecture

Date: 2025-07-11
Status: Accepted
Context: Container Kit originally provided 30+ individual atomic tools for containerization tasks, leading to complex orchestration challenges, tool coordination problems, and a fragmented user experience. Users had to understand multiple tools and manage their interdependencies manually.

Decision: Replace the entire atomic tool ecosystem with a single comprehensive workflow tool called `containerize_and_deploy`. This workflow implements a 10-step sequential process that handles the complete containerization lifecycle from repository analysis to deployment verification.

## Architecture Details

### Single Workflow Implementation
- **Tool**: `containerize_and_deploy` (only MCP tool exposed)
- **Location**: `pkg/mcp/domain/workflow/containerize.go`
- **Registration**: `pkg/mcp/application/registrar/tools.go` 
- **Steps**: 10 sequential stages with built-in progress tracking
- **Integration**: MCP protocol for AI assistant communication

### Workflow Steps
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

### Key Features
- **Progress Tracking**: Visual feedback for each step
- **Error Recovery**: AI-powered retry logic with fix suggestions
- **Session Management**: Persistent workflow state
- **Atomic Operations**: Each step can be retried independently

## Replaced Architecture

### Before: Atomic Tool Ecosystem (294 files)
- Individual tools for each operation (analyze, build, scan, deploy, etc.)
- Complex tool orchestration and dependency management
- Manual tool chaining by users or external systems
- Fragmented error handling across tools
- Inconsistent progress reporting

### After: Single Workflow (25 core files - 82% reduction)
- One comprehensive tool handling entire process
- Built-in orchestration and error recovery
- Unified progress tracking and user experience
- Consistent error handling with structured context
- Simplified maintenance and testing

## Consequences

### Benefits
- **Simplified User Experience**: One tool instead of 30+
- **Reduced Complexity**: 82% reduction in core files
- **Better Error Recovery**: AI-assisted retry with contextual fixes
- **Consistent Progress Tracking**: Unified progress reporting
- **Easier Maintenance**: Single workflow to test and maintain
- **Improved Reliability**: Built-in error handling and recovery
- **Better AI Integration**: Designed for AI assistant workflows

### Trade-offs
- **Less Granular Control**: Cannot use individual tools in isolation
- **Larger Tool Surface**: Single tool handles all functionality
- **Migration Effort**: Existing integrations need to adapt
- **Workflow Coupling**: All steps are part of single execution path

### Technical Impact
- **Testing**: Simplified to workflow-level integration tests
- **Development**: Focus on workflow steps rather than tool coordination
- **Documentation**: Single comprehensive workflow documentation
- **Debugging**: Unified error reporting with step-by-step context

## Implementation Status
- ✅ Single workflow tool implemented
- ✅ 10-step process with progress tracking
- ✅ AI-powered error recovery integration
- ✅ MCP protocol integration complete
- ✅ Legacy pipeline interfaces maintained for compatibility
- ✅ Session management for workflow state persistence

## Related ADRs
- ADR-004: Unified Rich Error System (enables workflow error handling)
- ADR-005: AI-Assisted Error Recovery (powers workflow retry logic)
- ADR-006: Four-Layer MCP Architecture (workflow domain architecture)
- ADR-007: CQRS, Saga, and Wire Patterns (advanced workflow coordination)