# Workstream 2: Fix Undefined Types and Missing Type Definitions

## Objective
Fix all undefined type errors identified in the pre-commit checks. These issues involve missing type definitions that are expected by various implementation files.

## Scope
Focus exclusively on creating missing type definitions and fixing undefined type references without changing any other code logic.

## Affected Areas and Issues

### 1. Core Type Definitions (in pkg/mcp/core/)

**Missing types that need to be defined**:
- `ResourceLimits` - Used in kubernetes_operations_test.go
- `ResourceSpec` - Used in kubernetes_operations_test.go  
- `HealthCheckConfig` - Used in kubernetes_operations_test.go
- `GenerateDockerfileResult` - Used in generate_dockerfile.go
- `core.Analyzer` - Used in partial_analyzers.go

### 2. Config Types (in pkg/mcp/internal/config/)

**Missing types**:
- `config.WorkerConfig` - Used in integration_test.go

### 3. Missing Fields in Existing Structs

**GenerateManifestsResult**:
- Missing `ManifestCount` field

**DeployParams**:
- Missing `Wait` field (bool)
- Missing `Timeout` field (duration or int)

### 4. Tool Implementation Functions

**Missing functions in pkg/mcp/tools/scan/**:
- `newAtomicScanSecretsToolImpl`
- `NewSecurityScanToolWithMocks`
- `calculateRiskScore`
- `calculateRiskLevel`
- `securityScanToolImpl`

### 5. Types Package References

**Undefined types references**:
- Various `types` package references in dockerfile_validation_core.go
- Need to fix import paths or define missing types

## Instructions

1. **Create missing type definitions**: Define the missing types with appropriate fields based on their usage context.

2. **Add missing struct fields**: Add the missing fields to existing structs with appropriate types.

3. **Create missing functions**: Implement stub/mock versions of missing functions for tests.

4. **Fix import references**: Ensure all type references have correct import paths.

5. **Maintain backward compatibility**: Use omitempty tags and optional fields where appropriate.

## Success Criteria
- All "undefined:" errors from pre-commit output are resolved
- All new types have appropriate JSON tags for serialization
- Test files can reference all expected types and functions
- No new compilation errors are introduced

## Example Pattern
```go
// Define missing types with appropriate fields
type ResourceLimits struct {
    Memory string `json:"memory,omitempty"`
    CPU    string `json:"cpu,omitempty"`
}

type ResourceSpec struct {
    Limits   ResourceLimits `json:"limits,omitempty"`
    Requests ResourceLimits `json:"requests,omitempty"`
}

// Add missing fields to existing structs
type GenerateManifestsResult struct {
    BaseToolResponse
    ManifestPaths []string `json:"manifest_paths"`
    ManifestCount int      `json:"manifest_count"`
    Namespace     string   `json:"namespace"`
}
```