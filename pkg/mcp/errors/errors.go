// Package errors provides a single rich error type used across MCP.
//
//go:generate go run ./cmd/gen-error-codes
package errors

import (
	"encoding/json"
	"fmt"
	"runtime"
)

type Code string

// Error codes are now generated from codes.yaml
// Use 'go generate' to regenerate from the YAML file

// Severity represents how bad the error is from 0‥3.
type Severity uint8

const (
	SeverityUnknown Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// Rich wraps every error flowing through MCP.
type Rich struct {
	Code       Code            `json:"code"`
	Domain     string          `json:"domain,omitempty"`
	Severity   Severity        `json:"severity"`
	Message    string          `json:"message"`
	Retryable  bool            `json:"retryable"`
	UserFacing bool            `json:"user_facing"`
	Location   string          `json:"location"`
	Cause      error           `json:"-"`
	Fields     map[string]any  `json:"fields,omitempty"`
}

// Error implements error.
func (r *Rich) Error() string { return fmt.Sprintf("%s: %s", r.Code, r.Message) }
func (r *Rich) Unwrap() error { return r.Cause }

// New builds a Rich error in one line.
//
//	errors.New(CodeValidationFailed, "deploy", "invalid image", err)
func New(code Code, domain, msg string, cause error) *Rich {
	_, file, line, _ := runtime.Caller(1)
	
	// Get metadata from generated code, with sensible defaults
	var severity Severity = SeverityMedium
	var retryable bool = false
	if _, sev, retry, exists := GetCodeMetadata(code); exists {
		severity = sev
		retryable = retry
	}
	
	return &Rich{
		Code:       code,
		Domain:     domain,
		Message:    msg,
		Cause:      cause,
		Severity:   severity,
		Retryable:  retryable,
		UserFacing: true, // Default to user-facing
		Location:   fmt.Sprintf("%s:%d", file, line),
	}
}

func (r *Rich) With(key string, val any) *Rich {
	if r.Fields == nil {
		r.Fields = make(map[string]any, 4)
	}
	r.Fields[key] = val
	return r
}

func (r *Rich) JSON() string {
	out, _ := json.Marshal(r)
	return string(out)
}

