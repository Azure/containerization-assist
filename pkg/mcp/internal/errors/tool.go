package errors

import (
	"fmt"
)

// Tool-specific error codes and types
const (
	// Build tool errors
	CodeBuildFailed         = "BUILD_FAILED"
	CodeDockerfileGenFailed = "DOCKERFILE_GEN_FAILED"
	CodeImagePushFailed     = "IMAGE_PUSH_FAILED"
	CodeImagePullFailed     = "IMAGE_PULL_FAILED"

	// Deploy tool errors
	CodeDeployFailed        = "DEPLOY_FAILED"
	CodeManifestGenFailed   = "MANIFEST_GEN_FAILED"
	CodeK8sConnectionFailed = "K8S_CONNECTION_FAILED"

	// Scan tool errors
	CodeScanFailed          = "SCAN_FAILED"
	CodeVulnerabilityFound  = "VULNERABILITY_FOUND"
	CodeScannerNotAvailable = "SCANNER_NOT_AVAILABLE"

	// Analysis tool errors
	CodeAnalysisFailed       = "ANALYSIS_FAILED"
	CodeNoSupportedFiles     = "NO_SUPPORTED_FILES"
	CodeLanguageNotSupported = "LANGUAGE_NOT_SUPPORTED"

	// Session errors
	CodeSessionNotFound    = "SESSION_NOT_FOUND"
	CodeSessionExpired     = "SESSION_EXPIRED"
	CodeSessionStoreFailed = "SESSION_STORE_FAILED"
)

// Tool-specific error constructors

// BuildFailedError creates a build failure error
func BuildFailedError(stage, reason string) *CoreError {
	return Build("build", fmt.Sprintf("Build failed at %s: %s", stage, reason)).
		WithContext("stage", stage).
		WithContext("reason", reason)
}

// DockerfileGenerationError creates a Dockerfile generation error
func DockerfileGenerationError(reason string) *CoreError {
	return Build("analyze", fmt.Sprintf("Failed to generate Dockerfile: %s", reason)).
		WithContext("error_code", CodeDockerfileGenFailed)
}

// ImagePushError creates an image push error
func ImagePushError(image, registry, reason string) *CoreError {
	return Build("build", fmt.Sprintf("Failed to push image %s to %s: %s", image, registry, reason)).
		WithContext("image", image).
		WithContext("registry", registry).
		WithContext("error_code", CodeImagePushFailed)
}

// DeploymentError creates a deployment error
func DeploymentError(resource, reason string) *CoreError {
	return Deploy("deploy", fmt.Sprintf("Failed to deploy %s: %s", resource, reason)).
		WithContext("resource", resource).
		WithContext("error_code", CodeDeployFailed)
}

// ManifestGenerationError creates a manifest generation error
func ManifestGenerationError(kind, reason string) *CoreError {
	return Deploy("deploy", fmt.Sprintf("Failed to generate %s manifest: %s", kind, reason)).
		WithContext("kind", kind).
		WithContext("error_code", CodeManifestGenFailed)
}

// K8sConnectionError creates a Kubernetes connection error
func K8sConnectionError(cluster, reason string) *CoreError {
	return Network("deploy", fmt.Sprintf("Failed to connect to Kubernetes cluster %s: %s", cluster, reason)).
		WithContext("cluster", cluster).
		WithContext("error_code", CodeK8sConnectionFailed).
		SetRetryable(true)
}

// ScanError creates a security scan error
func ScanError(scanner, target, reason string) *CoreError {
	return Security("scan", fmt.Sprintf("Security scan failed for %s using %s: %s", target, scanner, reason)).
		WithContext("scanner", scanner).
		WithContext("target", target).
		WithContext("error_code", CodeScanFailed)
}

// VulnerabilityError creates a vulnerability found error
func VulnerabilityError(severity string, count int, details string) *CoreError {
	return Security("scan", fmt.Sprintf("Found %d %s vulnerabilities: %s", count, severity, details)).
		WithContext("severity", severity).
		WithContext("count", count).
		WithContext("error_code", CodeVulnerabilityFound).
		SetFatal(severity == "critical")
}

// AnalysisError creates a repository analysis error
func AnalysisError(path, reason string) *CoreError {
	return Analysis("analyze", fmt.Sprintf("Failed to analyze repository at %s: %s", path, reason)).
		WithContext("path", path).
		WithContext("error_code", CodeAnalysisFailed)
}

// NoSupportedFilesError creates a no supported files error
func NoSupportedFilesError(path string, languages []string) *CoreError {
	return Analysis("analyze", fmt.Sprintf("No supported files found in %s. Looked for: %v", path, languages)).
		WithContext("path", path).
		WithContext("languages", languages).
		WithContext("error_code", CodeNoSupportedFiles)
}

// LanguageNotSupportedError creates a language not supported error
func LanguageNotSupportedError(language string) *CoreError {
	return Analysis("analyze", fmt.Sprintf("Language %s is not supported", language)).
		WithContext("language", language).
		WithContext("error_code", CodeLanguageNotSupported)
}

// SessionNotFoundError creates a session not found error
func SessionNotFoundError(sessionID string) *CoreError {
	return Session("session", fmt.Sprintf("Session %s not found", sessionID)).
		WithContext("session_id", sessionID).
		WithContext("error_code", CodeSessionNotFound)
}

// SessionExpiredError creates a session expired error
func SessionExpiredError(sessionID string) *CoreError {
	return Session("session", fmt.Sprintf("Session %s has expired", sessionID)).
		WithContext("session_id", sessionID).
		WithContext("error_code", CodeSessionExpired)
}

// SessionStoreError creates a session storage error
func SessionStoreError(operation string, err error) *CoreError {
	return Wrap(err, "session", fmt.Sprintf("Session store operation failed: %s", operation)).
		WithContext("operation", operation).
		WithContext("error_code", CodeSessionStoreFailed)
}
