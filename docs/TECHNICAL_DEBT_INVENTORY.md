# Technical Debt Inventory

> **Generated**: During Week 3 of MCP Reorganization  
> **Purpose**: Document all placeholder, stub, legacy, backwards compatibility, and TODO comments in the codebase

## Overview

This document provides a comprehensive inventory of technical debt markers found in the Container Kit codebase. These markers indicate areas that need future attention, refactoring, or removal.

## Summary Statistics

- **TODO Comments**: 15 actionable items
- **FIXME Comments**: 0 found
- **HACK Comments**: 1 found
- **Legacy Interface Methods**: 30+ methods across 5 files
- **Deprecated Methods**: 4 methods
- **Backward Compatibility Code**: 10+ locations
- **Stub/Placeholder Code**: 2 components
- **Temporary Code**: 5+ references

## TODO Items

### 1. Interface Migration (4 items)
**Location**: `pkg/mcp/internal/runtime/auto_registration.go`
- Line 21: Update `build_image` to unified interface
- Line 24: Update `push_image` to unified interface
- Line 36: Update `analyze_repository` to unified interface
- Line 38: Update `generate_dockerfile_enhanced` to unified interface

**Impact**: Medium - These tools need to be updated to use the new unified interface pattern

### 2. External Service Integration (3 items)

#### Registry Connectivity
**Location**: `pkg/mcp/internal/registry/multi_registry_manager.go:172`
```go
// TODO: Implement actual registry connectivity test
```
**Impact**: Low - Currently validates credentials without making API requests

#### Vulnerability Database
**Location**: `pkg/mcp/internal/build/image_validator.go`
- Line 311: Integrate with actual vulnerability database (CVE, Trivy, Grype)
- Line 377: Parse COPY --from instructions for stage references

**Impact**: High - Security scanning functionality is limited without proper database integration

### 3. Parser Implementation (1 item)
**Location**: `pkg/mcp/internal/build/build_validator.go:113`
```go
// TODO: Properly parse Trivy JSON output
```
**Impact**: Medium - Currently using simple string matching instead of structured parsing

### 4. AI Context Implementation (2 items)
**Location**: `pkg/mcp/internal/ai_context/enrichment.go`
- Line 24: Implement proper analyzer when mcptypes are fully defined
- Line 44: Implement proper type conversion when mcptypes are fully defined

**Impact**: Medium - AI context functionality is limited

### 5. Fixing Integration (2 items)
**Location**: `pkg/mcp/internal/fixing/analyzer_integration.go`
- Line 141: Fix ShareContext signature
- Line 181: Implement proper workspace directory retrieval

**Impact**: Medium - Affects error recovery capabilities

### 6. Docker Optimization (1 item)
**Location**: `pkg/mcp/internal/customizer/docker/optimization.go:138`
```go
content = strings.ReplaceAll(content, ":latest", ":specific-version # TODO: Replace with actual version")
```
**Impact**: Low - Placeholder for version pinning

### 7. Testing (1 item)
**Location**: `test/integration/integration_test.go:18`
```go
// TODO: Add actual integration tests for MCP server functionality
```
**Impact**: High - Missing test coverage for core functionality

### 8. Documentation (1 item)
**Location**: `pkg/mcp/types/interfaces.go:15`
```go
// TODO: Import cycles resolved - interface definitions moved to pkg/mcp/interfaces.go
```
**Impact**: Low - Documentation update needed

## Legacy Code

### 1. Legacy SimpleTool Interface Methods
Found in multiple files, these methods provide backward compatibility:
- `pkg/mcp/internal/scan/scan_image_security.go` (lines 1289-1306)
- `pkg/mcp/internal/analyze/analyze_repository_atomic.go` (lines 719-736)
- `pkg/mcp/internal/build/tag_image.go` (lines 524-541)
- `pkg/mcp/internal/build/build_image_atomic.go` (lines 252-269)
- `pkg/mcp/internal/build/pull_image.go` (lines 608-625)

**Pattern**:
```go
// Legacy SimpleTool compatibility methods
func (t *Tool) Name() string { return t.ToolName }
func (t *Tool) GetToolMetadata() interface{} { return t.GetMetadata() }
```

### 2. Legacy Session Formats
**Location**: `pkg/mcp/internal/session/state.go`
- Lines 400, 471: Conversion methods between legacy and new formats
- Methods: `ToLegacyFormat()`, `FromLegacyData()`

### 3. Legacy Orchestrator Support
**Location**: `pkg/mcp/internal/orchestration/`
- `no_reflect_orchestrator_impl.go`: Support for old field names
- `checkpoint_manager.go:473`: Legacy format unmarshal support
- `stage_executor.go:252`: Legacy variable expansion method

## Deprecated Methods

### 1. Pipeline Adapter Methods
**Location**: Multiple orchestration files
```go
// Deprecated: Use SetPipelineOperations instead
func (o *Orchestrator) SetPipelineAdapter(adapter PipelineAdapter) 
```

### 2. Build Execution
**Location**: `pkg/mcp/internal/build/build_image_atomic.go:119`
```go
// Deprecated: Use ExecuteWithContext instead
func (t *BuildImageTool) ExecuteBuild(ctx context.Context, args *BuildImageArgs)
```

### 3. Mock Constructor
**Location**: `pkg/mcp/internal/transport/llm/llm_mock.go:44`
```go
// Deprecated: Use NewWithRequestHandler instead
```

## Stub/Placeholder Components

### 1. StubAnalyzer
**Location**: `pkg/mcp/internal/adapter/clients.go:20-54`
```go
// StubAnalyzer provides a stub implementation with warnings about production use
```
**Warning**: Not for production use - provides minimal functionality

### 2. Placeholder Secret Values
**Location**: `pkg/mcp/internal/scan/scan_secrets.go`
- Lines 870, 1019-1070: Generates placeholder values for redacted secrets
```go
func generatePlaceholderValue(secretType string) string
```

## Hack/Workaround Code

### 1. Log Field Extraction
**Location**: `pkg/mcp/internal/utils/log_capture.go:26`
```go
// Extract fields from the event (this is a bit hacky but zerolog doesn't expose fields directly)
```
**Impact**: Low - Works but could be cleaner with proper zerolog API

## Temporary Code

### 1. Docker Build Context
**Location**: `pkg/clients/docker.go`
- Creates temporary directories and Dockerfiles for build context
- Should be cleaned up after use

### 2. Transient Error Handling
**Location**: `pkg/mcp/internal/orchestration/retry_manager.go:189`
- References temporary/transient errors in retry logic

## Backward Compatibility Layers

### 1. Logger Migration
**Location**: `pkg/logger/migration.go`
- Provides zerolog to slog adapter for gradual migration
- Methods: `ZerologToSlogAdapter`, `WrapZerologLogger`

### 2. CLI Compatibility
**Location**: `pkg/ai/analyzer_cli.go:9`
- `AzureAnalyzer` wrapper for CLI backward compatibility

### 3. Snapshot Format
**Location**: `pkg/pipeline/snapshot.go:51`
- Maintains backward compatibility for error fields in snapshots

## Recommendations

### High Priority
1. **Add Integration Tests**: Critical for ensuring MCP server functionality
2. **Integrate Vulnerability Database**: Essential for security scanning
3. **Update Tools to Unified Interface**: Complete the interface migration

### Medium Priority
1. **Implement Proper Parsers**: Replace string matching with structured parsing
2. **Fix AI Context Implementation**: Complete type definitions
3. **Fix ShareContext Signature**: Improve error recovery

### Low Priority
1. **Remove Legacy Interface Methods**: After migration period
2. **Clean Up Deprecated Methods**: Provide migration timeline
3. **Replace Stub Components**: Implement full functionality

### Long Term
1. **Remove Backward Compatibility Layers**: After all consumers migrate
2. **Consolidate Legacy Formats**: Standardize on new formats
3. **Document Migration Paths**: For each deprecated component

## Migration Strategy

1. **Phase 1** (Current): Maintain all compatibility layers
2. **Phase 2** (3 months): Deprecate legacy methods with warnings
3. **Phase 3** (6 months): Remove deprecated code
4. **Phase 4** (9 months): Remove compatibility layers

## Tracking

Use the following labels in issue tracking:
- `tech-debt`: General technical debt
- `todo`: For TODO items
- `legacy-code`: For legacy compatibility code
- `deprecated`: For deprecated methods
- `migration`: For migration-related tasks