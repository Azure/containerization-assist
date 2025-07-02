package rich

import (
	"fmt"
	"strings"
)

// Docker/Container specific errors

// DockerfileSyntaxError creates an error for Dockerfile syntax issues
func DockerfileSyntaxError(lineNum int, instruction, issue string) *RichError {
	return NewError().
		Code(CodeDockerfileSyntaxError).
		Type(ErrTypeValidation).
		Severity(SeverityHigh).
		Messagef("Invalid Dockerfile syntax at line %d: %s", lineNum, issue).
		Context("line_number", lineNum).
		Context("instruction", instruction).
		Context("issue", issue).
		Suggestion("Check Dockerfile syntax documentation").
		HelpURL("https://docs.docker.com/engine/reference/builder/").
		WithLocation().
		Build()
}

// ImageBuildError creates an error for Docker image build failures
func ImageBuildError(imageName string, cause error) *RichError {
	return NewError().
		Code(CodeImageBuildFailed).
		Type(ErrTypeBusiness).
		Severity(SeverityHigh).
		Messagef("Failed to build Docker image: %s", imageName).
		Context("image_name", imageName).
		Cause(cause).
		WithLocation().
		Build()
}

// ImagePushError creates an error for Docker push failures
func ImagePushError(image, registry string, cause error) *RichError {
	return NewError().
		Code(CodeImagePushFailed).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Failed to push image %s to registry %s", image, registry).
		Context("image", image).
		Context("registry", registry).
		Cause(cause).
		Suggestion("Check registry credentials and network connectivity").
		WithLocation().
		Build()
}

// ImagePullError creates an error for Docker pull failures
func ImagePullError(image string, cause error) *RichError {
	return NewError().
		Code(CodeImagePullFailed).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Failed to pull image: %s", image).
		Context("image", image).
		Cause(cause).
		Suggestion("Check image name and registry access").
		WithLocation().
		Build()
}

// Kubernetes specific errors

// KubernetesAPIError creates an error for K8s API failures
func KubernetesAPIError(operation, resource string, cause error) *RichError {
	return NewError().
		Code(CodeKubernetesAPIError).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Kubernetes API error during %s on %s", operation, resource).
		Context("operation", operation).
		Context("resource", resource).
		Cause(cause).
		Suggestion("Check cluster connectivity and permissions").
		WithLocation().
		Build()
}

// ManifestValidationError creates an error for invalid K8s manifests
func ManifestValidationError(file string, issues []string) *RichError {
	return NewError().
		Code(CodeManifestInvalid).
		Type(ErrTypeValidation).
		Severity(SeverityHigh).
		Messagef("Invalid Kubernetes manifest: %s", file).
		Context("manifest_file", file).
		Context("validation_issues", issues).
		Context("issue_count", len(issues)).
		Suggestion(fmt.Sprintf("Fix validation issues: %s", strings.Join(issues, ", "))).
		HelpURL("https://kubernetes.io/docs/concepts/overview/working-with-objects/").
		WithLocation().
		Build()
}

// DeploymentError creates an error for deployment failures
func DeploymentError(deployment, namespace string, cause error) *RichError {
	return NewError().
		Code(CodeDeploymentFailed).
		Type(ErrTypeBusiness).
		Severity(SeverityCritical).
		Messagef("Failed to deploy %s to namespace %s", deployment, namespace).
		Context("deployment", deployment).
		Context("namespace", namespace).
		Cause(cause).
		Suggestion("Check deployment configuration and cluster state").
		WithLocation().
		Build()
}

// NamespaceNotFoundError creates an error for missing namespaces
func NamespaceNotFoundError(namespace string) *RichError {
	return NewError().
		Code(CodeNamespaceNotFound).
		Type(ErrTypeResource).
		Severity(SeverityMedium).
		Messagef("Kubernetes namespace not found: %s", namespace).
		Context("namespace", namespace).
		Suggestion(fmt.Sprintf("Create namespace with: kubectl create namespace %s", namespace)).
		WithLocation().
		Build()
}

// Tool/Registry specific errors

// ToolNotFoundError creates an error for missing tools
func ToolNotFoundError(toolName string, availableTools []string) *RichError {
	return NewError().
		Code(CodeToolNotFound).
		Type(ErrTypeBusiness).
		Severity(SeverityHigh).
		Messagef("Tool not found: %s", toolName).
		Context("tool_name", toolName).
		Context("available_tools", availableTools).
		Suggestion(fmt.Sprintf("Available tools: %s", strings.Join(availableTools, ", "))).
		WithLocation().
		Build()
}

// ToolExecutionError creates an error for tool execution failures
func ToolExecutionError(toolName string, params interface{}, cause error) *RichError {
	return NewError().
		Code(CodeToolExecutionFailed).
		Type(ErrTypeBusiness).
		Severity(SeverityHigh).
		Messagef("Failed to execute tool: %s", toolName).
		Context("tool_name", toolName).
		Context("parameters", params).
		Cause(cause).
		WithLocation().
		WithStack().
		Build()
}

// ToolAlreadyRegisteredError creates an error for duplicate tool registration
func ToolAlreadyRegisteredError(toolName string) *RichError {
	return NewError().
		Code(CodeToolAlreadyRegistered).
		Type(ErrTypeConfiguration).
		Severity(SeverityMedium).
		Messagef("Tool already registered: %s", toolName).
		Context("tool_name", toolName).
		Suggestion("Use a different name or unregister the existing tool first").
		WithLocation().
		Build()
}

// Validation specific errors

// ParameterValidationError creates an error for parameter validation failures
func ParameterValidationError(param string, value interface{}, constraint string) *RichError {
	return NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Invalid parameter '%s': %s", param, constraint).
		Context("parameter", param).
		Context("value", value).
		Context("constraint", constraint).
		WithLocation().
		Build()
}

// MissingParameterError creates an error for missing required parameters
func MissingParameterError(param string) *RichError {
	return NewError().
		Code(CodeMissingParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Missing required parameter: %s", param).
		Context("parameter", param).
		Suggestion(fmt.Sprintf("Provide the required parameter: %s", param)).
		WithLocation().
		Build()
}

// TypeConversionError creates an error for type conversion failures
func TypeConversionError(from, to string, value interface{}) *RichError {
	return NewError().
		Code(CodeTypeConversionFailed).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Cannot convert %s to %s", from, to).
		Context("from_type", from).
		Context("to_type", to).
		Context("value", value).
		WithLocation().
		Build()
}

// Network specific errors

// ConnectionError creates an error for connection failures
func ConnectionError(host string, port int, cause error) *RichError {
	return NewError().
		Code(CodeConnectionFailed).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Failed to connect to %s:%d", host, port).
		Context("host", host).
		Context("port", port).
		Cause(cause).
		Suggestion("Check network connectivity and firewall rules").
		WithLocation().
		Build()
}

// TimeoutError creates an error for operation timeouts
func TimeoutError(operation string, timeout string) *RichError {
	return NewError().
		Code(CodeNetworkTimeout).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Operation timed out: %s (timeout: %s)", operation, timeout).
		Context("operation", operation).
		Context("timeout", timeout).
		Suggestion("Increase timeout or check for performance issues").
		WithLocation().
		Build()
}

// Security specific errors

// AuthenticationError creates an error for auth failures
func AuthenticationError(method string, cause error) *RichError {
	return NewError().
		Code(CodeAuthenticationFailed).
		Type(ErrTypeSecurity).
		Severity(SeverityCritical).
		Message("Authentication failed").
		Context("auth_method", method).
		Cause(cause).
		Suggestion("Check credentials and authentication configuration").
		WithLocation().
		Build()
}

// AuthorizationError creates an error for authz failures
func AuthorizationError(user, resource, action string) *RichError {
	return NewError().
		Code(CodeAuthorizationFailed).
		Type(ErrTypeSecurity).
		Severity(SeverityCritical).
		Messagef("User %s not authorized to %s resource %s", user, action, resource).
		Context("user", user).
		Context("resource", resource).
		Context("action", action).
		Suggestion("Check user permissions and roles").
		WithLocation().
		Build()
}

// SecretNotFoundError creates an error for missing secrets
func SecretNotFoundError(secretName string) *RichError {
	return NewError().
		Code(CodeSecretNotFound).
		Type(ErrTypeSecurity).
		Severity(SeverityHigh).
		Messagef("Secret not found: %s", secretName).
		Context("secret_name", secretName).
		Suggestion("Ensure the secret exists and is accessible").
		WithLocation().
		Build()
}

// Resource specific errors

// ResourceNotFoundError creates an error for missing resources
func ResourceNotFoundError(resourceType, resourceID string) *RichError {
	return NewError().
		Code(CodeResourceNotFound).
		Type(ErrTypeResource).
		Severity(SeverityMedium).
		Messagef("%s not found: %s", resourceType, resourceID).
		Context("resource_type", resourceType).
		Context("resource_id", resourceID).
		WithLocation().
		Build()
}

// ResourceAlreadyExistsError creates an error for duplicate resources
func ResourceAlreadyExistsError(resourceType, resourceID string) *RichError {
	return NewError().
		Code(CodeResourceAlreadyExists).
		Type(ErrTypeResource).
		Severity(SeverityMedium).
		Messagef("%s already exists: %s", resourceType, resourceID).
		Context("resource_type", resourceType).
		Context("resource_id", resourceID).
		Suggestion("Use a different identifier or update the existing resource").
		WithLocation().
		Build()
}

// ResourceLockedError creates an error for locked resources
func ResourceLockedError(resourceType, resourceID, lockedBy string) *RichError {
	return NewError().
		Code(CodeResourceLocked).
		Type(ErrTypeResource).
		Severity(SeverityMedium).
		Messagef("%s is locked: %s (locked by: %s)", resourceType, resourceID, lockedBy).
		Context("resource_type", resourceType).
		Context("resource_id", resourceID).
		Context("locked_by", lockedBy).
		Suggestion("Wait for the lock to be released or contact the lock owner").
		WithLocation().
		Build()
}

// ResourceExhaustedError creates an error for exhausted resources
func ResourceExhaustedError(resourceType string, limit, used int) *RichError {
	return NewError().
		Code(CodeResourceExhausted).
		Type(ErrTypeResource).
		Severity(SeverityCritical).
		Messagef("%s exhausted: %d/%d used", resourceType, used, limit).
		Context("resource_type", resourceType).
		Context("limit", limit).
		Context("used", used).
		Suggestion("Increase resource limits or clean up unused resources").
		WithLocation().
		Build()
}

// Enhanced Tool-Specific Validation Errors

// ToolValidationError creates a comprehensive tool validation error
func ToolValidationError(toolName, field, message, code string, value interface{}) *RichError {
	return NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Tool '%s' validation error on field '%s': %s", toolName, field, message).
		Context("tool_name", toolName).
		Context("field", field).
		Context("validation_code", code).
		Context("value", value).
		Suggestion(fmt.Sprintf("Fix validation error for field '%s'", field)).
		WithLocation().
		Build()
}

// ToolConfigValidationError creates a configuration validation error for tools
func ToolConfigValidationError(field, message string, value interface{}) *RichError {
	return NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeConfiguration).
		Severity(SeverityMedium).
		Messagef("Configuration validation error for field '%s': %s", field, message).
		Context("field", field).
		Context("value", value).
		Suggestion(fmt.Sprintf("Check configuration format for field '%s'", field)).
		WithLocation().
		Build()
}

// ToolConstraintViolationError creates an error for constraint violations
func ToolConstraintViolationError(field, constraint, message string, value interface{}) *RichError {
	return NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Constraint violation on field '%s': %s", field, message).
		Context("field", field).
		Context("constraint", constraint).
		Context("value", value).
		Suggestion(fmt.Sprintf("Ensure field '%s' meets constraint: %s", field, constraint)).
		WithLocation().
		Build()
}

// ToolSchemaValidationError creates an error for JSON schema validation failures
func ToolSchemaValidationError(schemaPath, message string, value interface{}) *RichError {
	return NewError().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Schema validation error at path '%s': %s", schemaPath, message).
		Context("schema_path", schemaPath).
		Context("value", value).
		Suggestion("Check input format against tool schema requirements").
		WithLocation().
		Build()
}

// ToolVersionCompatibilityError creates an error for version compatibility issues
func ToolVersionCompatibilityError(toolName, currentVersion, requiredVersion string) *RichError {
	return NewError().
		Code(CodeVersionMismatch).
		Type(ErrTypeCompatibility).
		Severity(SeverityHigh).
		Messagef("Tool '%s' version %s is incompatible with required version %s", toolName, currentVersion, requiredVersion).
		Context("tool_name", toolName).
		Context("current_version", currentVersion).
		Context("required_version", requiredVersion).
		Suggestion(fmt.Sprintf("Upgrade tool to version %s or adjust version requirements", requiredVersion)).
		WithLocation().
		Build()
}
