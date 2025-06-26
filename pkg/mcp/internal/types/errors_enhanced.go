package types

import (
	"context"
	"runtime"
	"time"
)

// ErrorWithContext creates a RichError and records it in metrics
func ErrorWithContext(ctx context.Context, code, message, errorType string) *RichError {
	err := NewRichError(code, message, errorType)

	// Capture stack trace for diagnostics
	pc := make([]uintptr, 10)
	n := runtime.Callers(2, pc)
	if n > 0 {
		frames := runtime.CallersFrames(pc[:n])
		var stackInfo []string
		for {
			frame, more := frames.Next()
			stackInfo = append(stackInfo, frame.Function)
			if !more {
				break
			}
		}
		err.Context.Metadata.AddCustom("stack_trace", stackInfo)
	}

	// Import observability to avoid circular dependency
	// This will be handled by the caller
	return err
}

// ErrorCodeMapping provides a mapping of error codes to metrics labels
var ErrorCodeMapping = map[string]struct {
	MetricCode string
	Category   string
	Severity   string
}{
	// Build errors
	"BUILD_FAILED":       {"build.failed", "build", "high"},
	"DOCKERFILE_INVALID": {"build.dockerfile_invalid", "build", "medium"},
	"BUILD_TIMEOUT":      {"build.timeout", "build", "high"},
	"IMAGE_PUSH_FAILED":  {"build.push_failed", "build", "high"},

	// Deployment errors
	"DEPLOY_FAILED":           {"deploy.failed", "deployment", "high"},
	"MANIFEST_INVALID":        {"deploy.manifest_invalid", "deployment", "medium"},
	"CLUSTER_UNREACHABLE":     {"deploy.cluster_unreachable", "deployment", "critical"},
	"RESOURCE_QUOTA_EXCEEDED": {"deploy.quota_exceeded", "deployment", "high"},

	// Analysis errors
	"REPO_UNREACHABLE": {"analysis.repo_unreachable", "analysis", "medium"},
	"ANALYSIS_FAILED":  {"analysis.failed", "analysis", "high"},
	"LANGUAGE_UNKNOWN": {"analysis.language_unknown", "analysis", "low"},
	"CLONE_FAILED":     {"analysis.clone_failed", "analysis", "high"},

	// System errors
	"DISK_FULL":         {"system.disk_full", "system", "critical"},
	"NETWORK_ERROR":     {"system.network_error", "system", "high"},
	"PERMISSION_DENIED": {"system.permission_denied", "system", "medium"},
	"TIMEOUT":           {"system.timeout", "system", "high"},

	// Session errors
	"SESSION_NOT_FOUND":        {"session.not_found", "session", "medium"},
	"SESSION_EXPIRED":          {"session.expired", "session", "low"},
	"WORKSPACE_QUOTA_EXCEEDED": {"session.workspace_quota", "session", "high"},

	// Security errors
	"SECURITY_VULNERABILITIES": {"security.vulnerabilities", "security", "critical"},
}

// GetMetricLabels returns standardized metric labels for an error code
func GetMetricLabels(code string) (metricCode, category, severity string) {
	if mapping, ok := ErrorCodeMapping[code]; ok {
		return mapping.MetricCode, mapping.Category, mapping.Severity
	}
	// Default mapping
	return "unknown." + code, "unknown", "medium"
}

// EnhanceErrorMetadata adds additional tracking fields to existing ErrorMetadata
func EnhanceErrorMetadata(em *ErrorMetadata, correlationID, requestID, userID string) *ErrorMetadata {
	if em == nil {
		return nil
	}

	if em.Custom == nil {
		em.Custom = make(map[string]interface{})
	}

	if correlationID != "" {
		em.Custom["correlation_id"] = correlationID
	}
	if requestID != "" {
		em.Custom["request_id"] = requestID
	}
	if userID != "" {
		em.Custom["user_id"] = userID
	}

	em.Custom["created_at"] = time.Now()

	return em
}
