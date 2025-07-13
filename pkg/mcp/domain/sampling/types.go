package sampling

import "time"

// DockerfileAnalysis represents the result of analyzing a Dockerfile.
type DockerfileAnalysis struct {
	Language      string
	Framework     string
	Port          int
	BuildSteps    []string
	Dependencies  []string
	Issues        []string
	Suggestions   []string
	BaseImage     string
	EstimatedSize string
}

// ManifestAnalysis represents the result of analyzing Kubernetes manifests.
type ManifestAnalysis struct {
	ResourceTypes []string
	Issues        []string
	Suggestions   []string
	SecurityRisks []string
	BestPractices []string
}

// SecurityAnalysis represents the result of analyzing security scan results.
type SecurityAnalysis struct {
	RiskLevel       string
	Vulnerabilities []Vulnerability
	Recommendations []string
	Remediations    []string
}

// Vulnerability represents a security vulnerability.
type Vulnerability struct {
	ID          string
	Severity    string
	Description string
	Package     string
	Version     string
	FixVersion  string
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
