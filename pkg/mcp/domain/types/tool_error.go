package types

import (
	"errors"
	"time"
)

// ToolError represents an error that occurred during tool execution
type ToolError struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Type        string                 `json:"type"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	ToolName    string                 `json:"tool_name,omitempty"`
	Retryable   bool                   `json:"retryable,omitempty"`
	RetryCount  int                    `json:"retry_count,omitempty"`
	MaxRetries  int                    `json:"max_retries,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *ToolError) Error() string {
	return e.Message
}

// NewToolError creates a new ToolError
func NewToolError(code, message, errorType string) *ToolError {
	return &ToolError{
		Code:        code,
		Message:     message,
		Type:        errorType,
		Timestamp:   time.Now(),
		Details:     make(map[string]interface{}),
		Suggestions: make([]string, 0),
		Context:     make(map[string]interface{}),
		Retryable:   false,
		RetryCount:  0,
		MaxRetries:  3,
	}
}

// ImageReference represents a Docker image reference
type ImageReference struct {
	Registry   string            `json:"registry,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Repository string            `json:"repository"`
	Tag        string            `json:"tag,omitempty"`
	Digest     string            `json:"digest,omitempty"`
	Platform   string            `json:"platform,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// String returns the full image reference as a string
func (ir *ImageReference) String() string {
	result := ""

	if ir.Registry != "" {
		result += ir.Registry + "/"
	}

	if ir.Namespace != "" {
		result += ir.Namespace + "/"
	}

	result += ir.Repository

	if ir.Tag != "" {
		result += ":" + ir.Tag
	}

	if ir.Digest != "" {
		result += "@" + ir.Digest
	}

	return result
}

// NewImageReference creates a new ImageReference from a string
func NewImageReference(ref string) *ImageReference {
	// This is a simplified parser - in practice you'd want more robust parsing
	ir := &ImageReference{
		Repository: ref,
		Tag:        "latest",
		Labels:     make(map[string]string),
	}
	return ir
}

// SecurityScanParams represents parameters for security scanning
type SecurityScanParams struct {
	Target        string            `json:"target" validate:"required"`
	ScanType      string            `json:"scan_type,omitempty"` // vulnerability, secrets, compliance
	Scanner       string            `json:"scanner,omitempty"`   // trivy, grype, etc.
	Severity      string            `json:"severity,omitempty"`  // LOW, MEDIUM, HIGH, CRITICAL
	Format        string            `json:"format,omitempty"`    // json, table, sarif
	OutputFile    string            `json:"output_file,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	IgnoreUnfixed bool              `json:"ignore_unfixed,omitempty"`
	OfflineMode   bool              `json:"offline_mode,omitempty"`
}

// Validate validates the security scan parameters
func (p *SecurityScanParams) Validate() error {
	if p.Target == "" {
		return errors.New("target is required")
	}
	if p.ScanType == "" {
		return errors.New("scan_type is required")
	}
	return nil
}
