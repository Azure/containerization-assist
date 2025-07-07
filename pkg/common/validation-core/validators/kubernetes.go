package validators

// Kubernetes Validation Package
//
// This package provides comprehensive Kubernetes manifest validation functionality.
// The original large file (1541 LOC) has been split into focused modules following
// WORKSTREAM ETA standards for code readability and maintainability.
//
// Validation functionality is distributed across these files:
//
// - kubernetes_yaml_validator.go: YAML parsing and basic structure validation
// - kubernetes_metadata_validator.go: Metadata, labels, and annotations validation
// - kubernetes_resource_validator.go: Resource-specific validation (Pod, Service, Deployment, etc.)
// - kubernetes_security_validator.go: Security context and policy validation
// - kubernetes_type_converter.go: Type conversion utilities and safe extraction helpers
//
// The KubernetesValidator struct and type definitions remain in kubernetes_types.go
//
// All validation methods follow these patterns:
// - Hierarchical validation (basic structure → specific details)
// - Conditional validation based on resource kind
// - Configurable strict vs. lenient modes
// - Type safety progression from interface{} to fully typed validation
// - Error accumulation rather than fail-fast behavior
//
// This modular structure ensures each file has a single responsibility and
// remains under the 800 LOC limit while maintaining comprehensive validation coverage.
