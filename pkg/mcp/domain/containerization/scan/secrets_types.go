package scan

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

// AtomicScanSecretsArgs represents the arguments for secret scanning
type AtomicScanSecretsArgs struct {
	types.BaseToolArgs

	ScanPath        string   `json:"scan_path,omitempty" validate:"omitempty,secure_path" description:"Path to scan (default: session workspace)"`
	FilePatterns    []string `json:"file_patterns,omitempty" validate:"omitempty,dive,file_pattern" description:"File patterns to include in scan (e.g., '*.py', '*.js')"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty" validate:"omitempty,dive,file_pattern" description:"File patterns to exclude from scan"`

	ScanDockerfiles bool `json:"scan_dockerfiles,omitempty" description:"Include Dockerfiles in scan"`
	ScanManifests   bool `json:"scan_manifests,omitempty" description:"Include Kubernetes manifests in scan"`
	ScanSourceCode  bool `json:"scan_source_code,omitempty" description:"Include source code files in scan"`
	ScanEnvFiles    bool `json:"scan_env_files,omitempty" description:"Include .env files in scan"`

	SuggestRemediation bool `json:"suggest_remediation,omitempty" description:"Provide remediation suggestions"`
	GenerateSecrets    bool `json:"generate_secrets,omitempty" description:"Generate Kubernetes Secret manifests"`
}

// Validate validates the arguments
func (a AtomicScanSecretsArgs) Validate() error {
	// Use tag-based validation
	return validation.ValidateTaggedStruct(a)
}

// GetSessionID returns the session ID
func (a AtomicScanSecretsArgs) GetSessionID() string {
	return a.SessionID
}

// AtomicScanSecretsResult represents the result of secret scanning
type AtomicScanSecretsResult struct {
	types.BaseToolResponse
	types.BaseAIContextResult

	SessionID    string        `json:"session_id"`
	ScanPath     string        `json:"scan_path"`
	FilesScanned int           `json:"files_scanned"`
	Duration     time.Duration `json:"duration"`

	SecretsFound      int             `json:"secrets_found"`
	DetectedSecrets   []ScannedSecret `json:"detected_secrets"`
	SeverityBreakdown map[string]int  `json:"severity_breakdown"`

	FileResults []FileSecretScanResult `json:"file_results"`

	RemediationPlan  *SecretRemediationPlan    `json:"remediation_plan,omitempty"`
	GeneratedSecrets []GeneratedSecretManifest `json:"generated_secrets,omitempty"`

	SecurityScore   int      `json:"security_score"`
	RiskLevel       string   `json:"risk_level"`
	Recommendations []string `json:"recommendations"`

	ScanContext map[string]interface{} `json:"scan_context"`
}

// IsSuccess returns whether the scan was successful
func (r AtomicScanSecretsResult) IsSuccess() bool {
	// Consider scan successful if we scanned at least one file
	return r.FilesScanned > 0
}

// ScannedSecret represents a detected secret
type ScannedSecret struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Type       string `json:"type"`
	Pattern    string `json:"pattern"`
	Value      string `json:"value"`
	Severity   string `json:"severity"`
	Context    string `json:"context"`
	Confidence int    `json:"confidence"`
}

// FileSecretScanResult represents scan results for a single file
type FileSecretScanResult struct {
	FilePath     string          `json:"file_path"`
	FileType     string          `json:"file_type"`
	SecretsFound int             `json:"secrets_found"`
	Secrets      []ScannedSecret `json:"secrets"`
	CleanStatus  string          `json:"clean_status"`
}

// SecretRemediationPlan provides remediation guidance
type SecretRemediationPlan struct {
	ImmediateActions []string          `json:"immediate_actions"`
	SecretReferences []SecretReference `json:"secret_references"`
	ConfigMapEntries map[string]string `json:"config_map_entries"`
	PreferredManager string            `json:"preferred_manager"`
	MigrationSteps   []string          `json:"migration_steps"`
}

// SecretReference represents a reference to an external secret
type SecretReference struct {
	SecretName     string `json:"secret_name"`
	SecretKey      string `json:"secret_key"`
	OriginalEnvVar string `json:"original_env_var"`
	KubernetesRef  string `json:"kubernetes_ref"`
}

// GeneratedSecretManifest represents a generated Kubernetes secret
type GeneratedSecretManifest struct {
	Name     string   `json:"name"`
	Content  string   `json:"content"`
	FilePath string   `json:"file_path"`
	Keys     []string `json:"keys"`
}

// standardSecretScanStages returns the standard progress stages for secret scanning
func standardSecretScanStages() []types.ProgressStage {
	return []types.ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating scan path"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing file patterns and scan configuration"},
		{Name: "Scan", Weight: 0.50, Description: "Scanning files for secrets"},
		{Name: "Process", Weight: 0.20, Description: "Processing results and generating recommendations"},
		{Name: "Finalize", Weight: 0.05, Description: "Generating reports and remediation plans"},
	}
}
