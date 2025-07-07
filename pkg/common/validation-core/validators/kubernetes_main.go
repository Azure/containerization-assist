package validators

// Kubernetes Validator Main File
// This file serves as the main entry point for Kubernetes validation functionality.
// The validation logic has been split into focused modules:
//
// - kubernetes_yaml_validator.go: YAML parsing and basic structure validation
// - kubernetes_metadata_validator.go: Metadata, labels, and annotations validation
// - kubernetes_resource_validator.go: Resource-specific validation (Pod, Service, Deployment, etc.)
// - kubernetes_security_validator.go: Security context and policy validation
// - kubernetes_type_converter.go: Type conversion utilities and safe extraction helpers
//
// This modular approach follows single responsibility principle and makes the code
// more maintainable while keeping the file sizes under 800 LOC as per WORKSTREAM ETA standards.

// The KubernetesValidator type and its core functionality remain in kubernetes_types.go
// All validation methods are distributed across the specific validator files above.
