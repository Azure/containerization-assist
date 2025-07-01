package rich

import (
	"fmt"
	"reflect"
	"time"
)

// GenericRichError provides type-safe error contexts
type GenericRichError[TContext any] struct {
	*RichError
	TypedContext TContext
}

// NewGenericError creates a new generic error with typed context
func NewGenericError[TContext any]() *GenericErrorBuilder[TContext] {
	return &GenericErrorBuilder[TContext]{
		ErrorBuilder: NewError(),
	}
}

// GenericErrorBuilder provides a fluent API for constructing GenericRichError instances
type GenericErrorBuilder[TContext any] struct {
	*ErrorBuilder
	typedContext TContext
}

// TypedContext sets the typed context
func (b *GenericErrorBuilder[TContext]) TypedContext(context TContext) *GenericErrorBuilder[TContext] {
	b.typedContext = context
	return b
}

// Override methods to return GenericErrorBuilder instead of ErrorBuilder

// Code sets the error code
func (b *GenericErrorBuilder[TContext]) Code(code ErrorCode) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Code(code)
	return b
}

// Type sets the error type
func (b *GenericErrorBuilder[TContext]) Type(errType ErrorType) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Type(errType)
	return b
}

// Severity sets the error severity
func (b *GenericErrorBuilder[TContext]) Severity(severity ErrorSeverity) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Severity(severity)
	return b
}

// Message sets the error message
func (b *GenericErrorBuilder[TContext]) Message(message string) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Message(message)
	return b
}

// Messagef sets a formatted error message
func (b *GenericErrorBuilder[TContext]) Messagef(format string, args ...interface{}) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Messagef(format, args...)
	return b
}

// Cause sets the underlying cause
func (b *GenericErrorBuilder[TContext]) Cause(cause error) *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.Cause(cause)
	return b
}

// WithLocation captures the current location
func (b *GenericErrorBuilder[TContext]) WithLocation() *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.WithLocation()
	return b
}

// WithStack captures the current stack trace
func (b *GenericErrorBuilder[TContext]) WithStack() *GenericErrorBuilder[TContext] {
	b.ErrorBuilder.WithStack()
	return b
}

// BuildTyped finalizes and returns the GenericRichError
func (b *GenericErrorBuilder[TContext]) BuildTyped() *GenericRichError[TContext] {
	richErr := b.ErrorBuilder.Build()
	return &GenericRichError[TContext]{
		RichError:    richErr,
		TypedContext: b.typedContext,
	}
}

// GetTypedContext returns the typed context
func (e *GenericRichError[TContext]) GetTypedContext() TContext {
	return e.TypedContext
}

// Tool-specific error types and contexts

// ToolExecutionContext provides context for tool execution errors
type ToolExecutionContext struct {
	ToolName      string
	ToolType      string
	Parameters    interface{}
	SessionID     string
	RequestID     string
	ExecutionID   string
	StartTime     int64
	Duration      int64
	RetryCount    int
	MaxRetries    int
	ResourcesUsed map[string]interface{}
}

// DockerBuildContext provides context for Docker build errors
type DockerBuildContext struct {
	DockerfilePath string
	ContextPath    string
	ImageName      string
	BuildArgs      map[string]string
	Tags           []string
	BuildStage     string
	LineNumber     *int
	Instruction    string
	BaseImage      string
	BuildOutput    []string
	CacheStatus    string
	RegistryURL    string
}

// KubernetesContext provides context for Kubernetes operation errors
type KubernetesContext struct {
	Namespace      string
	ResourceType   string
	ResourceName   string
	ManifestPath   string
	ClusterName    string
	ClusterVersion string
	APIVersion     string
	Operation      string
	KubectlOutput  []string
	ResourceStatus string
	Events         []string
}

// SecurityScanContext provides context for security scan errors
type SecurityScanContext struct {
	ScanType         string
	TargetType       string
	TargetName       string
	ScannerName      string
	ScannerVersion   string
	DatabaseVersion  string
	ScanDuration     int64
	VulnCount        int
	CriticalCount    int
	HighCount        int
	MediumCount      int
	LowCount         int
	PolicyViolations []string
}

// NetworkContext provides context for network errors
type NetworkContext struct {
	Protocol        string
	Host            string
	Port            int
	URL             string
	Method          string
	Headers         map[string]string
	StatusCode      int
	ResponseBody    string
	RequestBody     string
	Timeout         int64
	RetryAttempt    int
	DNSResolution   string
	TLSVersion      string
	CertificateInfo string
}

// ValidationContext provides context for validation errors
type ValidationContext struct {
	FieldName       string
	FieldValue      interface{}
	FieldType       string
	Constraint      string
	ValidatorName   string
	SchemaVersion   string
	ValidationRules []string
	RelatedFields   map[string]interface{}
	InputSource     string
}

// ResourceContext provides context for resource errors
type ResourceContext struct {
	ResourceType string
	ResourceID   string
	ResourcePath string
	Operation    string
	CurrentState string
	DesiredState string
	Owner        string
	Permissions  string
	Size         int64
	MaxSize      int64
	Usage        float64
	Limits       map[string]interface{}
	Dependencies []string
}

// Generic error constructor functions with typed contexts

// ToolExecutionGenericError creates a tool execution error with typed context
func ToolExecutionGenericError(toolName string, params interface{}, cause error) *GenericRichError[ToolExecutionContext] {
	context := ToolExecutionContext{
		ToolName:   toolName,
		Parameters: params,
		StartTime:  time.Now().Unix(),
	}

	return NewGenericError[ToolExecutionContext]().
		Code(CodeToolExecutionFailed).
		Type(ErrTypeBusiness).
		Severity(SeverityHigh).
		Messagef("Failed to execute tool: %s", toolName).
		Cause(cause).
		TypedContext(context).
		WithLocation().
		WithStack().
		BuildTyped()
}

// DockerBuildGenericError creates a Docker build error with typed context
func DockerBuildGenericError(imageName, dockerfilePath string, cause error) *GenericRichError[DockerBuildContext] {
	context := DockerBuildContext{
		DockerfilePath: dockerfilePath,
		ImageName:      imageName,
	}

	return NewGenericError[DockerBuildContext]().
		Code(CodeImageBuildFailed).
		Type(ErrTypeBusiness).
		Severity(SeverityHigh).
		Messagef("Failed to build Docker image: %s", imageName).
		Cause(cause).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// KubernetesGenericError creates a Kubernetes error with typed context
func KubernetesGenericError(operation, resource, namespace string, cause error) *GenericRichError[KubernetesContext] {
	context := KubernetesContext{
		Namespace:    namespace,
		ResourceName: resource,
		Operation:    operation,
	}

	return NewGenericError[KubernetesContext]().
		Code(CodeKubernetesAPIError).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Kubernetes %s failed for %s in namespace %s", operation, resource, namespace).
		Cause(cause).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// SecurityScanGenericError creates a security scan error with typed context
func SecurityScanGenericError(scanType, target string, cause error) *GenericRichError[SecurityScanContext] {
	context := SecurityScanContext{
		ScanType:   scanType,
		TargetName: target,
	}

	return NewGenericError[SecurityScanContext]().
		Code("SECURITY_SCAN_FAILED").
		Type(ErrTypeSecurity).
		Severity(SeverityHigh).
		Messagef("Security scan failed: %s on %s", scanType, target).
		Cause(cause).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// NetworkGenericError creates a network error with typed context
func NetworkGenericError(host string, port int, cause error) *GenericRichError[NetworkContext] {
	context := NetworkContext{
		Host: host,
		Port: port,
	}

	return NewGenericError[NetworkContext]().
		Code(CodeConnectionFailed).
		Type(ErrTypeNetwork).
		Severity(SeverityHigh).
		Messagef("Network connection failed to %s:%d", host, port).
		Cause(cause).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// ValidationGenericError creates a validation error with typed context
func ValidationGenericError(fieldName string, value interface{}, constraint string) *GenericRichError[ValidationContext] {
	context := ValidationContext{
		FieldName:  fieldName,
		FieldValue: value,
		Constraint: constraint,
		FieldType:  reflect.TypeOf(value).String(),
	}

	return NewGenericError[ValidationContext]().
		Code(CodeInvalidParameter).
		Type(ErrTypeValidation).
		Severity(SeverityMedium).
		Messagef("Validation failed for field '%s': %s", fieldName, constraint).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// ResourceGenericError creates a resource error with typed context
func ResourceGenericError(resourceType, resourceID, operation string) *GenericRichError[ResourceContext] {
	context := ResourceContext{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Operation:    operation,
	}

	return NewGenericError[ResourceContext]().
		Code(CodeResourceNotFound).
		Type(ErrTypeResource).
		Severity(SeverityMedium).
		Messagef("Resource operation failed: %s on %s %s", operation, resourceType, resourceID).
		TypedContext(context).
		WithLocation().
		BuildTyped()
}

// Helper functions for working with generic errors

// IsGenericError checks if an error is a GenericRichError
func IsGenericError[TContext any](err error) (*GenericRichError[TContext], bool) {
	if gErr, ok := err.(*GenericRichError[TContext]); ok {
		return gErr, true
	}
	return nil, false
}

// ExtractTypedContext extracts typed context from an error if possible
func ExtractTypedContext[TContext any](err error) (TContext, bool) {
	var zero TContext
	if gErr, ok := IsGenericError[TContext](err); ok {
		return gErr.GetTypedContext(), true
	}
	return zero, false
}

// ConvertToGeneric converts a regular RichError to a GenericRichError
func ConvertToGeneric[TContext any](err *RichError, context TContext) *GenericRichError[TContext] {
	return &GenericRichError[TContext]{
		RichError:    err,
		TypedContext: context,
	}
}

// ChainGenericErrors creates a chain of generic errors
func ChainGenericErrors[TContext any](errors ...*GenericRichError[TContext]) *GenericRichError[TContext] {
	if len(errors) == 0 {
		return nil
	}

	root := errors[0]
	current := root

	for i := 1; i < len(errors); i++ {
		current.Cause = errors[i]
		current = errors[i]
	}

	return root
}

// MergeContexts merges multiple contexts into a map
func MergeContexts(contexts ...interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	for i, ctx := range contexts {
		key := fmt.Sprintf("context_%d", i)
		merged[key] = ctx
	}

	return merged
}
