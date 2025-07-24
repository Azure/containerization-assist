package validation

// BuildValidationResult extends ValidationResult for Docker build validations
type BuildValidationResult struct {
	// Embed ValidationResult fields directly for compatibility
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`

	// Build-specific context
	SyntaxValid       bool `json:"syntax_valid"`
	BestPractices     bool `json:"best_practices"`
	SecurityIssues    int  `json:"security_issues"`
	PerformanceIssues int  `json:"performance_issues"`
}

// NewBuildValidationResult creates a new build validation result
func NewBuildValidationResult() *BuildValidationResult {
	return &BuildValidationResult{
		Valid:             true,
		Errors:            make([]ValidationError, 0),
		Warnings:          make([]ValidationWarning, 0),
		Metadata:          make(map[string]interface{}),
		SyntaxValid:       true,
		BestPractices:     true,
		SecurityIssues:    0,
		PerformanceIssues: 0,
	}
}

// AddError adds a validation error and marks the result as invalid
func (r *BuildValidationResult) AddError(field, message, code string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
		Level:   "error",
	})
}

// AddWarning adds a validation warning
func (r *BuildValidationResult) AddWarning(field, message, code string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

// SetContext adds context information to the validation result
func (r *BuildValidationResult) SetContext(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

// AddSecurityIssue adds a security-related validation error
func (r *BuildValidationResult) AddSecurityIssue(field, message, code string) {
	r.AddError(field, message, code)
	r.SecurityIssues++
	r.SetContext("has_security_issues", true)
}

// AddPerformanceIssue adds a performance-related validation warning
func (r *BuildValidationResult) AddPerformanceIssue(field, message, code string) {
	r.AddWarning(field, message, code)
	r.PerformanceIssues++
	r.SetContext("has_performance_issues", true)
}

// MarkSyntaxInvalid marks the syntax as invalid
func (r *BuildValidationResult) MarkSyntaxInvalid(field, message string) {
	r.SyntaxValid = false
	r.AddError(field, message, "SYNTAX_ERROR")
}

// MarkBestPracticeViolation marks a best practice violation
func (r *BuildValidationResult) MarkBestPracticeViolation(field, message string) {
	r.BestPractices = false
	r.AddWarning(field, message, "BEST_PRACTICE")
}

// ManifestValidationResult extends ValidationResult for Kubernetes manifest validations
type ManifestValidationResult struct {
	// Embed ValidationResult fields directly for compatibility
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`

	// Manifest-specific context
	APIVersionValid        bool `json:"api_version_valid"`
	ResourcesValid         bool `json:"resources_valid"`
	SecurityCompliant      bool `json:"security_compliant"`
	NetworkPolicyCompliant bool `json:"network_policy_compliant"`
}

// NewManifestValidationResult creates a new manifest validation result
func NewManifestValidationResult() *ManifestValidationResult {
	return &ManifestValidationResult{
		Valid:                  true,
		Errors:                 make([]ValidationError, 0),
		Warnings:               make([]ValidationWarning, 0),
		Metadata:               make(map[string]interface{}),
		APIVersionValid:        true,
		ResourcesValid:         true,
		SecurityCompliant:      true,
		NetworkPolicyCompliant: true,
	}
}

// AddAPIVersionError adds an API version validation error
func (r *ManifestValidationResult) AddAPIVersionError(field, message string) {
	r.APIVersionValid = false
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    "API_VERSION",
		Level:   "error",
	})
}

// AddResourceError adds a resource validation error
func (r *ManifestValidationResult) AddResourceError(field, message string) {
	r.ResourcesValid = false
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    "RESOURCE",
		Level:   "error",
	})
}

// AddSecurityViolation adds a security compliance violation
func (r *ManifestValidationResult) AddSecurityViolation(field, message string) {
	r.SecurityCompliant = false
	r.Warnings = append(r.Warnings, ValidationWarning{
		Field:   field,
		Message: message,
		Code:    "SECURITY",
	})
}

// AddNetworkPolicyViolation adds a network policy violation
func (r *ManifestValidationResult) AddNetworkPolicyViolation(field, message string) {
	r.NetworkPolicyCompliant = false
	r.Warnings = append(r.Warnings, ValidationWarning{
		Field:   field,
		Message: message,
		Code:    "NETWORK_POLICY",
	})
}

// RepositoryValidationResult extends ValidationResult for repository validations
type RepositoryValidationResult struct {
	*ValidationResult

	// Repository-specific context
	StructureValid     bool   `json:"structure_valid"`
	DependenciesValid  bool   `json:"dependencies_valid"`
	ConfigurationValid bool   `json:"configuration_valid"`
	DetectedLanguage   string `json:"detected_language"`
	DetectedFramework  string `json:"detected_framework"`
}

// NewRepositoryValidationResult creates a new repository validation result
func NewRepositoryValidationResult() *RepositoryValidationResult {
	return &RepositoryValidationResult{
		ValidationResult:   NewValidationResult(),
		StructureValid:     true,
		DependenciesValid:  true,
		ConfigurationValid: true,
	}
}

// SetDetectedLanguage sets the detected programming language
func (r *RepositoryValidationResult) SetDetectedLanguage(language string) {
	r.DetectedLanguage = language
	r.SetContext("language", language)
}

// SetDetectedFramework sets the detected framework
func (r *RepositoryValidationResult) SetDetectedFramework(framework string) {
	r.DetectedFramework = framework
	r.SetContext("framework", framework)
}

// AddStructureError adds a repository structure validation error
func (r *RepositoryValidationResult) AddStructureError(field, message string) {
	r.StructureValid = false
	r.AddError(field, message, "STRUCTURE")
}

// AddDependencyError adds a dependency validation error
func (r *RepositoryValidationResult) AddDependencyError(field, message string) {
	r.DependenciesValid = false
	r.AddError(field, message, "DEPENDENCY")
}

// AddConfigurationError adds a configuration validation error
func (r *RepositoryValidationResult) AddConfigurationError(field, message string) {
	r.ConfigurationValid = false
	r.AddError(field, message, "CONFIGURATION")
}
