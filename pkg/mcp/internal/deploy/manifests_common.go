package deploy

import (
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
)

// ManifestGeneratorInterface defines the interface for manifest generation
type ManifestGeneratorInterface interface {
	GenerateManifests(args GenerateManifestsRequest) (*kubernetes.ManifestGenerationResult, error)
}

// SecretHandler defines the interface for secret handling
type SecretHandler interface {
	ScanForSecrets(environment []SecretValue) ([]SecretInfo, error)
	GenerateSecretManifests(secrets []SecretInfo, namespace string) ([]ManifestFile, error)
	ExternalizeSecrets(environment []SecretValue, secrets []SecretInfo) ([]SecretValue, error)
}

// GenerateManifestsRequest contains the input parameters for manifest generation
type GenerateManifestsRequest struct {
	SessionID      string
	ImageReference string
	AppName        string
	Port           int
	Namespace      string
	CPURequest     string
	MemoryRequest  string
	CPULimit       string
	MemoryLimit    string
	Environment    []SecretValue
	IncludeIngress bool
	IngressHost    string
}

// SecretValue represents a secret or environment variable value
type SecretValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SecretInfo contains information about a detected secret
type SecretInfo struct {
	Name        string
	Value       string
	Type        string
	SecretName  string
	SecretKey   string
	IsSecret    bool
	IsSensitive bool
	Pattern     string
	Confidence  float64
	Reason      string
}

// ManifestFile represents a generated Kubernetes manifest file with content
type ManifestFile struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	FilePath   string `json:"filePath"`
	IsSecret   bool   `json:"isSecret"`
	SecretInfo string `json:"secretInfo,omitempty"`
}

// ValidationResult represents the result of manifest validation
type ValidationResult struct {
	ManifestName string
	Valid        bool
	Errors       []string
	Warnings     []string
}

// CommonManifestContext provides rich context about the manifest generation
type CommonManifestContext struct {
	ManifestsGenerated    int      `json:"manifestsGenerated"`
	SecretsDetected       int      `json:"secretsDetected"`
	SecretsExternalized   int      `json:"secretsExternalized"`
	ResourceTypes         []string `json:"resourceTypes"`
	DeploymentStrategy    string   `json:"deploymentStrategy"`
	TotalResources        int      `json:"totalResources"`
	IngressEnabled        bool     `json:"ingressEnabled"`
	ResourceLimitsSet     bool     `json:"resourceLimitsSet"`
	SecurityLevel         string   `json:"securityLevel"`
	BestPractices         []string `json:"bestPractices"`
	SecurityIssues        []string `json:"securityIssues,omitempty"`
	TemplateUsed          string   `json:"templateUsed,omitempty"`
	TemplateSelectionInfo string   `json:"templateSelectionInfo,omitempty"`
}

// Error types specific to manifest generation
type ManifestError struct {
	Code    string
	Message string
	Type    string
}

func (e *ManifestError) Error() string {
	return e.Message
}

// NewManifestError creates a new manifest-specific error
func NewManifestError(code, message string, errType string) *ManifestError {
	return &ManifestError{
		Code:    code,
		Message: message,
		Type:    errType,
	}
}
