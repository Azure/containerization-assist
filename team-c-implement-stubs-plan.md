# Team C - Implement TODO Stubs Plan

## Overview
Implement the actual functionality for the 3 "not yet implemented" stubs found in the codebase.

## Implementation Tasks

### 1. Resource Limits in Deployment Generation
**File**: `pkg/mcp/internal/deploy/generate_manifests_yaml.go:104`
**Current**: Only logs what would happen
**Implementation**: Actually modify the deployment YAML to add resource specifications

### 2. Auto-Registration Adapter  
**File**: `pkg/mcp/internal/runtime/auto_registration_adapter.go`
**Current**: Returns error for dependency-injected tools
**Implementation**: Implement proper auto-registration for DI tools

### 3. Async Builds
**File**: `pkg/mcp/internal/build/build_image.go:202-203`
**Current**: Falls back to synchronous builds with warning
**Implementation**: Implement proper async build support with job tracking

## Execution Order
1. Resource limits (simplest, isolated change)
2. Async builds (medium complexity, adds job tracking)
3. Auto-registration (most complex, affects tool registration system)