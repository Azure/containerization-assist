// Package errors provides utilities for converting domain-specific errors to Rich errors
package errors

import (
	"fmt"
	"reflect"
	"strings"
)

// DomainErrorConverter provides methods to convert domain-specific errors to Rich errors
type DomainErrorConverter struct {
	defaultDomain string
}

// NewDomainErrorConverter creates a new converter with a default domain
func NewDomainErrorConverter(domain string) *DomainErrorConverter {
	return &DomainErrorConverter{
		defaultDomain: domain,
	}
}

// ConvertError converts any error to a Rich error, preserving domain-specific context
func (dec *DomainErrorConverter) ConvertError(err error, code Code) *Rich {
	if err == nil {
		return nil
	}

	// If already a Rich error, return as-is
	if richErr, ok := err.(*Rich); ok {
		return richErr
	}

	// If it's a WorkflowError, extract the Rich error
	if wfErr, ok := err.(*WorkflowError); ok {
		return wfErr.Rich
	}

	// Create new Rich error and attempt to extract domain-specific fields
	rich := New(code, dec.defaultDomain, err.Error(), err)

	// Use reflection to extract fields from domain errors
	dec.extractDomainFields(err, rich)

	return rich
}

// ConvertToWorkflowError converts any error to a WorkflowError
func (dec *DomainErrorConverter) ConvertToWorkflowError(err error, code Code, step string) *WorkflowError {
	if err == nil {
		return nil
	}

	// If already a WorkflowError, update step if needed
	if wfErr, ok := err.(*WorkflowError); ok {
		if step != "" && wfErr.Step == "" {
			wfErr.Step = step
		}
		return wfErr
	}

	// Create new workflow error
	wfErr := NewWorkflowError(code, dec.defaultDomain, step, err.Error(), err)

	// Extract domain-specific fields
	dec.extractDomainFields(err, wfErr.Rich)

	return wfErr
}

// extractDomainFields uses reflection to extract fields from domain error types
func (dec *DomainErrorConverter) extractDomainFields(err error, rich *Rich) {
	errValue := reflect.ValueOf(err)

	// Handle pointer types
	if errValue.Kind() == reflect.Ptr && !errValue.IsNil() {
		errValue = errValue.Elem()
	}

	// Only process struct types
	if errValue.Kind() != reflect.Struct {
		return
	}

	errType := errValue.Type()

	// Extract fields based on common patterns
	for i := 0; i < errValue.NumField(); i++ {
		field := errType.Field(i)
		fieldValue := errValue.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Map common field names
		switch field.Name {
		case "Code", "ErrorCode":
			if code, ok := fieldValue.Interface().(string); ok && code != "" {
				rich.With("domain_code", code)
			}
		case "ExitCode":
			if exitCode, ok := fieldValue.Interface().(int); ok {
				rich.With("exit_code", exitCode)
			}
		case "BuildLogs", "Logs":
			if logs, ok := fieldValue.Interface().([]string); ok && len(logs) > 0 {
				rich.With("logs", logs)
			}
		case "ImageRef", "Image":
			if img, ok := fieldValue.Interface().(string); ok && img != "" {
				rich.With("image_ref", img)
			}
		case "Namespace":
			if ns, ok := fieldValue.Interface().(string); ok && ns != "" {
				rich.With("namespace", ns)
			}
		case "Resource":
			if res, ok := fieldValue.Interface().(string); ok && res != "" {
				rich.With("resource", res)
			}
		case "Endpoint", "URL":
			if endpoint, ok := fieldValue.Interface().(string); ok && endpoint != "" {
				rich.With("endpoint", endpoint)
			}
		case "Details":
			if details, ok := fieldValue.Interface().(map[string]interface{}); ok {
				for k, v := range details {
					rich.With(k, v)
				}
			}
		}
	}
}

// DomainErrorMapping defines how to map domain error types to error codes
type DomainErrorMapping struct {
	TypeName string
	Code     Code
	Domain   string
}

// CommonDomainMappings provides default mappings for known domain error types
var CommonDomainMappings = []DomainErrorMapping{
	// Docker errors
	{TypeName: "BuildError", Code: CodeImageBuildFailed, Domain: "docker"},
	{TypeName: "PushError", Code: CodeImagePushFailed, Domain: "docker"},
	{TypeName: "PullError", Code: CodeImagePullFailed, Domain: "docker"},

	// Kubernetes errors
	{TypeName: "DeploymentError", Code: CodeDeploymentFailed, Domain: "kubernetes"},
	{TypeName: "ManifestError", Code: CodeManifestInvalid, Domain: "kubernetes"},
	{TypeName: "HealthCheckError", Code: CodeDeploymentFailed, Domain: "kubernetes"},

	// Analysis errors
	{TypeName: "AnalysisError", Code: CodeOperationFailed, Domain: "analysis"},
	{TypeName: "ValidationError", Code: CodeValidationFailed, Domain: "validation"},
	{TypeName: "SecretError", Code: CodeSecurityViolation, Domain: "security"},
}

// GetCodeForError attempts to determine the appropriate error code for a domain error
func GetCodeForError(err error) Code {
	if err == nil {
		return CodeUnknown
	}

	// Check if it's already a Rich error
	if richErr, ok := err.(*Rich); ok {
		return richErr.Code
	}

	// Check type name against mappings
	errType := reflect.TypeOf(err)
	if errType != nil {
		typeName := errType.Name()
		if typeName == "" && errType.Kind() == reflect.Ptr {
			typeName = errType.Elem().Name()
		}

		for _, mapping := range CommonDomainMappings {
			if mapping.TypeName == typeName {
				return mapping.Code
			}
		}
	}

	// Default based on error message patterns
	errMsg := err.Error()
	switch {
	case contains(errMsg, "build", "docker build"):
		return CodeImageBuildFailed
	case contains(errMsg, "push", "registry"):
		return CodeImagePushFailed
	case contains(errMsg, "deploy", "deployment"):
		return CodeDeploymentFailed
	case contains(errMsg, "validate", "validation"):
		return CodeValidationFailed
	case contains(errMsg, "unauthorized", "permission", "access denied"):
		return CodePermissionDenied
	case contains(errMsg, "timeout", "timed out"):
		return CodeTimeoutError
	case contains(errMsg, "not found", "404"):
		return CodeResourceNotFound
	default:
		return CodeOperationFailed
	}
}

// contains checks if any of the patterns exist in the string (case-insensitive)
func contains(s string, patterns ...string) bool {
	lower := strings.ToLower(s)
	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// WrapDomainError wraps a domain error with appropriate Rich error metadata
func WrapDomainError(err error, context string) *Rich {
	if err == nil {
		return nil
	}

	code := GetCodeForError(err)
	domain := "unknown"

	// Try to determine domain from error type
	for _, mapping := range CommonDomainMappings {
		if reflect.TypeOf(err).Name() == mapping.TypeName {
			domain = mapping.Domain
			break
		}
	}

	rich := New(code, domain, fmt.Sprintf("%s: %v", context, err), err)

	// Extract any domain-specific fields
	converter := NewDomainErrorConverter(domain)
	converter.extractDomainFields(err, rich)

	return rich
}
