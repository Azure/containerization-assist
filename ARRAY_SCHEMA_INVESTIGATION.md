# MCP Array Schema Validation Issue Investigation

## Problem Summary

The Container Kit MCP server is experiencing validation errors when used with VS Code/GitHub Copilot:

```
Failed to validate tool mcp_containerkit_update_session_labels: Error: tool parameters array type must have items
Failed to validate tool mcp_containerkit_generate_manifests: Error: tool parameters array type must have items
```

This issue affects multiple critical tools in the containerization workflow.

## Root Cause Analysis

### The Core Issue
The `github.com/alecthomas/jsonschema` library, when used with the `github.com/localrivet/gomcp` library, generates JSON schemas for array fields that are missing the required `items` property.

**Example of problematic schema:**
```json
{
  "labels": {
    "type": "array",
    "description": "Complete set of labels to apply to the session"
  }
}
```

**What MCP validation expects:**
```json
{
  "labels": {
    "type": "array", 
    "items": {"type": "string"},
    "description": "Complete set of labels to apply to the session"
  }
}
```

### Affected Tools

| Tool | Problematic Array Fields |
|------|-------------------------|
| `update_session_labels` | `labels` |
| `generate_manifests` | `ingress_hosts`, `ingress_tls`, `registry_secrets`, `secrets`, `service_ports` |
| `scan_image_security` | `vuln_types` |
| `validate_dockerfile` | `ignore_rules`, `trusted_registries` |
| `scan_secrets` | `exclude_patterns`, `file_patterns` |

### Technical Details

1. **Schema Generation Path**: Tools are registered via `runtime.RegisterSimpleTool()` → `gomcp.Tool()` → automatic schema generation
2. **Library Limitation**: The `jsonschema.Reflector` in the current configuration doesn't generate `items` for `[]string` fields
3. **Validation Point**: VS Code/Copilot validates MCP tool schemas and rejects arrays without `items`

### Impact on Workflow

**Currently Disabled Tools:**
- ❌ `update_session_labels` - Session management
- ❌ `generate_manifests` - **Critical** - Kubernetes manifest generation  
- ❌ `scan_image_security` - Security scanning
- ❌ `validate_dockerfile` - Dockerfile validation
- ❌ `scan_secrets` - Secret scanning

**Working Tools:**
- ✅ `analyze_repository` - Repository analysis
- ✅ `generate_dockerfile` - Dockerfile generation
- ✅ `build_image` - Docker image building
- ✅ `push_image` - Image pushing
- ✅ `pull_image` - Image pulling
- ✅ `tag_image` - Image tagging
- ✅ `validate_deployment` - Deployment validation

**Broken Workflow**: The loss of `generate_manifests` breaks the end-to-end containerization workflow.

## Investigation Attempts

### Attempt 1: JSON Schema Tags
```go
Labels []string `json:"labels" jsonschema:"type=array,items={type:string}"`
```
**Result**: ❌ Failed - Tags were ignored by gomcp

### Attempt 2: Flattened Structs
Removed embedded `types.BaseToolArgs` thinking it might interfere with schema generation.
**Result**: ❌ Failed - Issue persisted

### Attempt 3: Post-Processing Schema Fix
Created `AddMissingArrayItems()` utility to fix schemas after generation.
**Result**: ❌ Failed - Applied to wrong registration path (internal registry vs gomcp)

### Attempt 4: Typed Tool Registration
Replaced generic `func(ctx, args interface{})` with typed functions like `func(ctx, args *SpecificArgs)`.
**Result**: ✅ Partial success - Fixed empty schemas, but array items issue remained

## Potential Solutions

### Option 1: Fix Schema Generation at Source ⭐
**Approach**: Modify the `jsonschema.Reflector` configuration or add post-processing
**Complexity**: Medium
**Impact**: Fixes all tools permanently
**Implementation**: 
- Add `AddMissingArrayItems()` to the schema generation pipeline
- Hook into gomcp's schema generation process
- May require forking gomcp or using reflection hacks

### Option 2: Custom Tool Registration Wrapper
**Approach**: Create a wrapper that generates proper schemas before calling gomcp
**Complexity**: High  
**Impact**: Requires rewriting all tool registrations
**Implementation**:
```go
RegisterToolWithSchemaFix(name, description, argsType, resultType, handler)
```

### Option 3: Selective Re-enabling with Validation Bypass
**Approach**: Re-enable critical tools and document that validation warnings are expected
**Complexity**: Low
**Impact**: Tools work but generate validation errors in VS Code
**Risk**: VS Code might refuse to use tools with validation errors

### Option 4: Alternative Schema Library
**Approach**: Replace `github.com/alecthomas/jsonschema` with a different library
**Complexity**: High
**Impact**: May break other parts of the system
**Risk**: Unknown compatibility issues

### Option 5: Fork and Fix gomcp Library  
**Approach**: Fork gomcp and fix the schema generation
**Complexity**: Very High
**Impact**: Complete control over schema generation
**Maintenance**: Need to maintain fork

### Option 6: Use Internal Registry for All Tools
**Approach**: Convert all tools to use the internal `runtime.RegisterTool` instead of gomcp's `Tool()` 
**Complexity**: Medium-High
**Impact**: Consistent schema generation across all tools
**Implementation**: Already has working schema fixes in `runtime/registry.go`

## Recommended Solution

**Hybrid Approach**: Option 6 + Option 1

1. **Short-term**: Convert critical tools (`generate_manifests`, `scan_*`) to use internal registry
2. **Medium-term**: Implement schema post-processing in the gomcp registration path
3. **Long-term**: Contribute fix back to gomcp or jsonschema libraries

### Implementation Plan

1. **Phase 1 - Critical Tool Recovery**:
   - Convert `generate_manifests` to internal registry registration
   - Test that it generates proper schemas with `items`
   - Restore core workflow functionality

2. **Phase 2 - Schema Fix Infrastructure**:
   - Implement `AddMissingArrayItems()` in gomcp registration path
   - Apply to all remaining tools
   - Remove tool-by-tool workarounds

3. **Phase 3 - Upstream Contribution**:
   - Investigate if this is a known issue in gomcp/jsonschema
   - Submit patches if possible

## Current Status

- **Tools disabled**: 5 critical tools
- **Workflow impact**: End-to-end containerization broken
- **User impact**: Cannot generate Kubernetes manifests via MCP
- **Priority**: High - blocks core functionality

## Files Modified

- `pkg/mcp/internal/core/gomcp_tools.go` - Tool registrations (disabled problematic tools)
- `pkg/mcp/internal/session/manage_session_labels.go` - Attempted struct modifications
- `pkg/mcp/internal/utils/schema_utils.go` - Added `AddMissingArrayItems()` utility

## Next Steps

1. **Immediate**: Implement recommended hybrid solution to restore workflow
2. **Research**: Investigate gomcp library internals for hook points
3. **Testing**: Verify fix works with VS Code/Copilot integration
4. **Documentation**: Update tool documentation to reflect current limitations

---

*Investigation conducted on 2024-12-25*  
*Issue affects: VS Code + GitHub Copilot + Container Kit MCP integration*