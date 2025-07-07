package deploy

import (
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
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
	SessionID          string `validate:"required,session_id"`
	ImageReference     string `validate:"required,docker_image"`
	ImageRef           string `validate:"omitempty,docker_image"` // Alternative field name used in some places
	AppName            string `validate:"required,k8s_name"`
	Port               int    `validate:"omitempty,port"`
	Namespace          string `validate:"omitempty,namespace"`
	CPURequest         string `validate:"omitempty,resource_spec"`
	MemoryRequest      string `validate:"omitempty,resource_spec"`
	CPULimit           string `validate:"omitempty,resource_spec"`
	MemoryLimit        string `validate:"omitempty,resource_spec"`
	Environment        []SecretValue
	EnvironmentVars    map[string]string `validate:"omitempty,dive,keys,required,endkeys,no_sensitive"` // Alternative format
	IncludeIngress     bool
	IngressEnabled     bool   // Alternative field name
	IngressHost        string `validate:"omitempty,domain"`
	IngressPath        string `validate:"omitempty,secure_path"`
	IngressClassName   string `validate:"omitempty,k8s_name"`
	IngressTLS         bool
	IngressAnnotations map[string]string `validate:"omitempty"`
	ManifestDir        string            `validate:"omitempty,secure_path"`
	Replicas           int               `validate:"omitempty,min=1,max=100"`
	ServiceType        string            `validate:"omitempty,service_type"`
	NodePort           int               `validate:"omitempty,port"`
	SkipService        bool
	Resources          map[string]string `validate:"omitempty"`
	ConfigMapData      map[string]string `validate:"omitempty,dive,keys,required,endkeys,no_sensitive"`
	RegistrySecrets    []string          `validate:"omitempty,dive,k8s_name"`
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

// TemplateInfo represents template information for manifest generation
type TemplateInfo struct {
	Name        string                 `json:"name"`
	Path        string                 `json:"path"`
	Content     string                 `json:"content"`
	Description string                 `json:"description"`
	Languages   []string               `json:"languages"`
	Frameworks  []string               `json:"frameworks"`
	Features    []string               `json:"features"`
	Priority    int                    `json:"priority"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ValidationResult now uses the unified validation framework for deploy domain
type DeployValidationResult = validation.Result

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
