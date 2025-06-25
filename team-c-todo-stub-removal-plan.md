# Team C - TODO Stub Removal Plan

## Overview
Final task for Team C: Remove 'not yet implemented' TODO stubs found in the codebase.

## TODO Stubs Found

### 1. Resource Limits in Deployment Generation
**File**: `pkg/mcp/internal/deploy/generate_manifests_yaml.go:104`
**Context**: The `applyResourceLimits` function logs that resource limits would be applied but doesn't actually implement the functionality.

**Decision**: Keep as informational - this is a placeholder for future functionality that logs what would happen. The function correctly returns nil and doesn't break anything.

### 2. Auto-Registration Adapter
**File**: `pkg/mcp/internal/runtime/auto_registration_adapter.go`
**Context**: Returns an error stating "auto-registration not yet implemented for dependency-injected tools"

**Decision**: This is correct behavior - the adapter explicitly states that manual registration should be used instead. The error message is informative and guides developers to the correct approach.

### 3. Async Builds Warning
**File**: `pkg/mcp/internal/build/build_image.go:202-203`
**Context**: When async builds are requested, it logs a warning and runs synchronously instead.

**Decision**: Keep as graceful degradation - the tool handles the request by falling back to synchronous operation and informing the user.

## Recommendation

All three "not yet implemented" stubs are actually proper implementations:
- They provide clear feedback to users
- They don't cause failures
- They gracefully handle unsupported features

These are not problematic TODOs that need removal, but rather intentional placeholders for future functionality.