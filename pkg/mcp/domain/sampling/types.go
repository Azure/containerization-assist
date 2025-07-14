// Package sampling provides domain types for AI-powered analysis and code generation.
// This package defines the core data structures used for LLM sampling operations
// in the containerization workflow.
package sampling

import "time"

// DockerfileAnalysis represents the comprehensive result of analyzing a Dockerfile.
type DockerfileAnalysis struct {
	// Language is the primary programming language detected (e.g., "go", "nodejs", "python")
	Language string
	// Framework is the application framework detected (e.g., "express", "gin", "django")
	Framework string
	// Port is the application's primary listening port
	Port int
	// BuildSteps contains the ordered list of build commands
	BuildSteps []string
	// Dependencies lists the package dependencies found
	Dependencies []string
	// Issues contains identified problems or inefficiencies
	Issues []string
	// Suggestions provides actionable improvement recommendations
	Suggestions []string
	// BaseImage is the recommended or detected base container image
	BaseImage string
	// EstimatedSize is the predicted final container image size
	EstimatedSize string
}

// ManifestAnalysis represents the comprehensive result of analyzing Kubernetes manifests.
// This structure provides detailed insights into manifest structure, best practices
// compliance, security considerations, and optimization opportunities.
//
// The analysis covers:
//   - Resource type validation and recommendations
//   - Security policy compliance
//   - Performance and scaling considerations
//   - Kubernetes best practices adherence
type ManifestAnalysis struct {
	// ResourceTypes lists the Kubernetes resource types found (e.g., "Deployment", "Service")
	ResourceTypes []string
	// Issues contains identified configuration problems or anti-patterns
	Issues []string
	// Suggestions provides actionable improvement recommendations
	Suggestions []string
	// SecurityRisks identifies potential security vulnerabilities in the manifests
	SecurityRisks []string
	// BestPractices contains recommendations for following Kubernetes best practices
	BestPractices []string
}

// SecurityAnalysis represents the comprehensive result of analyzing security scan results.
// This structure processes vulnerability scanner output and provides prioritized
// remediation guidance with actionable fix recommendations.
//
// The analysis includes:
//   - Risk assessment and prioritization
//   - Vulnerability classification and impact analysis
//   - Automated remediation recommendations
//   - Compliance and policy guidance
type SecurityAnalysis struct {
	// RiskLevel is the overall security risk assessment ("low", "medium", "high", "critical")
	RiskLevel string
	// Vulnerabilities contains detailed information about each security issue found
	Vulnerabilities []Vulnerability
	// Recommendations provides prioritized security improvement actions
	Recommendations []string
	// Remediations contains specific steps to fix identified vulnerabilities
	Remediations []string
}

// Vulnerability represents a single security vulnerability found during scanning.
// This structure standardizes vulnerability information across different scanner
// tools (Trivy, Grype, etc.) and provides consistent remediation guidance.
type Vulnerability struct {
	// ID is the unique vulnerability identifier (e.g., "CVE-2023-1234", "GHSA-xxxx")
	ID string
	// Severity indicates the vulnerability severity level ("low", "medium", "high", "critical")
	Severity string
	// Description provides a human-readable explanation of the vulnerability
	Description string
	// Package is the affected package or component name
	Package string
	// Version is the current vulnerable version
	Version string
	// FixVersion is the recommended version that fixes the vulnerability
	FixVersion string
}

// DockerfileFix represents a fixed Dockerfile with metadata.
type DockerfileFix struct {
	OriginalContent string
	FixedContent    string
	Changes         []string
	Explanation     string
	Metadata        FixMetadata
}

// ManifestFix represents a fixed Kubernetes manifest with metadata.
type ManifestFix struct {
	OriginalContent string
	FixedContent    string
	Changes         []string
	Explanation     string
	Metadata        FixMetadata
}

// FixMetadata contains metadata about a fix operation.
type FixMetadata struct {
	TemplateID     string
	TokensUsed     int
	Temperature    float32
	ProcessingTime time.Duration
	Timestamp      time.Time
}
