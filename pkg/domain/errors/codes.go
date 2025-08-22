package errors

// Code represents an error code
type Code string

// Error codes - migrated from pkg/common/errors
const (
	CodeUnknown               Code = "UNKNOWN"                 // Unknown error occurred
	CodeInternalError         Code = "INTERNAL_ERROR"          // Internal system error
	CodeValidationFailed      Code = "VALIDATION_FAILED"       // Input validation failed
	CodeInvalidParameter      Code = "INVALID_PARAMETER"       // Invalid parameter provided
	CodeMissingParameter      Code = "MISSING_PARAMETER"       // Required parameter missing
	CodeTypeConversionFailed  Code = "TYPE_CONVERSION_FAILED"  // Type conversion failed
	CodeNetworkTimeout        Code = "NETWORK_TIMEOUT"         // Network operation timed out
	CodeIoError               Code = "IO_ERROR"                // Input/output operation failed
	CodeFileNotFound          Code = "FILE_NOT_FOUND"          // File not found
	CodePermissionDenied      Code = "PERMISSION_DENIED"       // Permission denied
	CodeResourceNotFound      Code = "RESOURCE_NOT_FOUND"      // Resource not found
	CodeResourceAlreadyExists Code = "RESOURCE_ALREADY_EXISTS" // Resource already exists
	CodeResourceExhausted     Code = "RESOURCE_EXHAUSTED"      // Resource exhausted
	CodeDockerfileSyntaxError Code = "DOCKERFILE_SYNTAX_ERROR" // Dockerfile syntax error
	CodeImageBuildFailed      Code = "IMAGE_BUILD_FAILED"      // Image build failed
	CodeImagePushFailed       Code = "IMAGE_PUSH_FAILED"       // Image push failed
	CodeImagePullFailed       Code = "IMAGE_PULL_FAILED"       // Image pull failed
	CodeContainerStartFailed  Code = "CONTAINER_START_FAILED"  // Container start failed
	CodeKubernetesApiError    Code = "KUBERNETES_API_ERROR"    // Kubernetes API error
	CodeManifestInvalid       Code = "MANIFEST_INVALID"        // Kubernetes manifest invalid
	CodeDeploymentFailed      Code = "DEPLOYMENT_FAILED"       // Deployment failed
	CodeNamespaceNotFound     Code = "NAMESPACE_NOT_FOUND"     // Kubernetes namespace not found
	CodeToolNotFound          Code = "TOOL_NOT_FOUND"          // Tool not found
	CodeToolExecutionFailed   Code = "TOOL_EXECUTION_FAILED"   // Tool execution failed
	CodeToolAlreadyRegistered Code = "TOOL_ALREADY_REGISTERED" // Tool already registered
	CodeVersionMismatch       Code = "VERSION_MISMATCH"        // Version mismatch
	CodeConfigurationInvalid  Code = "CONFIGURATION_INVALID"   // Configuration invalid
	CodeNetworkError          Code = "NETWORK_ERROR"           // Network error
	CodeOperationFailed       Code = "OPERATION_FAILED"        // Operation failed
	CodeTimeoutError          Code = "TIMEOUT_ERROR"           // Timeout error
	CodeTypeMismatch          Code = "TYPE_MISMATCH"           // Type mismatch
	CodeSecurityError         Code = "SECURITY_ERROR"          // Security error
	CodeValidationError       Code = "VALIDATION_ERROR"        // Validation error
	CodeSecurityViolation     Code = "SECURITY_VIOLATION"      // Security violation
	CodeVulnerabilityFound    Code = "VULNERABILITY_FOUND"     // Vulnerability found
	CodeNotImplemented        Code = "NOT_IMPLEMENTED"         // Not implemented
	CodeAlreadyExists         Code = "ALREADY_EXISTS"          // Already exists
	CodeInvalidState          Code = "INVALID_STATE"           // Invalid state
	CodeNotFound              Code = "NOT_FOUND"               // Not found
	CodeDisabled              Code = "DISABLED"                // Disabled
	CodeInternal              Code = "INTERNAL"                // Internal error
	CodeInvalidType           Code = "INVALID_TYPE"            // Invalid type

	// MCP-specific error codes
	CodeWorkflowFailed Code = "WORKFLOW_FAILED" // Workflow execution failed
	CodeSessionExpired Code = "SESSION_EXPIRED" // Session has expired
	CodeBuildFailed    Code = "BUILD_FAILED"    // Build process failed
	CodeScanFailed     Code = "SCAN_FAILED"     // Security scan failed
)
