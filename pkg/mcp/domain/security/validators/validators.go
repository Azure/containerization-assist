// Package validators provides consolidated validation functionality for the MCP system.
// This replaces the scattered validators throughout the codebase with a unified approach.
package validators

import (
	"context"
)

// AllValidators provides a single interface to access all validators
type AllValidators struct {
	Common     *UnifiedValidator
	Build      *BuildValidator
	Dockerfile *DockerfileValidator
	Security   *SecurityValidator
	Scan       *ScanValidator
	SecScan    *SecurityScanValidator
	Analyze    *AnalyzeValidator
	Deploy     *DeployValidator
	Manifest   *ManifestGenerator
	Health     *HealthCheckValidator
}

// NewAllValidators creates a new instance with all validators initialized
func NewAllValidators() *AllValidators {
	return &AllValidators{
		Common:     NewUnifiedValidator(),
		Build:      NewBuildValidator(),
		Dockerfile: NewDockerfileValidator(),
		Security:   NewSecurityValidator(),
		Scan:       NewScanValidator(),
		SecScan:    NewSecurityScanValidator(),
		Analyze:    NewAnalyzeValidator(),
		Deploy:     NewDeployValidator(),
		Manifest:   NewManifestGenerator(),
		Health:     NewHealthCheckValidator(),
	}
}

// ValidatorFactory provides factory methods for creating validators
type ValidatorFactory struct{}

// NewValidatorFactory creates a new validator factory
func NewValidatorFactory() *ValidatorFactory {
	return &ValidatorFactory{}
}

// CreateBuildValidator creates a build validator
func (vf *ValidatorFactory) CreateBuildValidator() *BuildValidator {
	return NewBuildValidator()
}

// CreateScanValidator creates a scan validator
func (vf *ValidatorFactory) CreateScanValidator() *ScanValidator {
	return NewScanValidator()
}

// CreateDeployValidator creates a deploy validator
func (vf *ValidatorFactory) CreateDeployValidator() *DeployValidator {
	return NewDeployValidator()
}

// CreateSecurityValidator creates a security validator
func (vf *ValidatorFactory) CreateSecurityValidator() *SecurityValidator {
	return NewSecurityValidator()
}

// ValidationService provides a high-level service interface for validation
type ValidationService struct {
	validators *AllValidators
	factory    *ValidatorFactory
}

// NewValidationService creates a new validation service
func NewValidationService() *ValidationService {
	return &ValidationService{
		validators: NewAllValidators(),
		factory:    NewValidatorFactory(),
	}
}

// ValidateSessionOperation validates common session-based operation parameters
func (vs *ValidationService) ValidateSessionOperation(ctx context.Context, sessionID string) error {
	return vs.validators.Common.Input.ValidateSessionID(sessionID)
}

// ValidateContainerBuild validates parameters for container build operations
func (vs *ValidationService) ValidateContainerBuild(ctx context.Context, sessionID, image, dockerfile, context string) error {
	return vs.validators.Build.ValidateBuildArgs(sessionID, image, dockerfile, context)
}

// ValidateContainerScan validates parameters for container security scanning
func (vs *ValidationService) ValidateContainerScan(ctx context.Context, sessionID, imageName string) error {
	return vs.validators.Scan.ValidateImageScanArgs(sessionID, imageName)
}

// ValidateKubernetesDeploy validates parameters for Kubernetes deployment
func (vs *ValidationService) ValidateKubernetesDeploy(ctx context.Context, sessionID string, manifests []string, namespace string) error {
	return vs.validators.Deploy.ValidateDeployArgs(ctx, sessionID, manifests, namespace)
}

// ValidateRepositoryAnalysis validates parameters for repository analysis
func (vs *ValidationService) ValidateRepositoryAnalysis(ctx context.Context, sessionID, repoURL string) error {
	return vs.validators.Analyze.ValidateAnalyzeArgs(sessionID, repoURL)
}

// GetValidators returns all validators for advanced usage
func (vs *ValidationService) GetValidators() *AllValidators {
	return vs.validators
}

// GetFactory returns the validator factory
func (vs *ValidationService) GetFactory() *ValidatorFactory {
	return vs.factory
}

// ValidationOptions provides options for validation behavior
type ValidationOptions struct {
	StrictMode       bool `json:"strict_mode"`
	WarningsAsErrors bool `json:"warnings_as_errors"`
	SkipSystemChecks bool `json:"skip_system_checks"`
}

// DefaultValidationOptions returns default validation options
func DefaultValidationOptions() *ValidationOptions {
	return &ValidationOptions{
		StrictMode:       false,
		WarningsAsErrors: false,
		SkipSystemChecks: false,
	}
}

// Migration helpers for existing code

// LegacyValidatorAdapter provides compatibility for existing validator interfaces
type LegacyValidatorAdapter struct {
	service *ValidationService
}

// NewLegacyValidatorAdapter creates an adapter for legacy validator interfaces
func NewLegacyValidatorAdapter() *LegacyValidatorAdapter {
	return &LegacyValidatorAdapter{
		service: NewValidationService(),
	}
}

// ValidateBuildPrerequisites provides compatibility for existing build validation
func (lva *LegacyValidatorAdapter) ValidateBuildPrerequisites(dockerfilePath, buildContext string) error {
	return lva.service.validators.Build.ValidateBuildPrerequisites(context.Background(), dockerfilePath, buildContext)
}

// AddPushTroubleshootingTips provides compatibility for existing troubleshooting
func (lva *LegacyValidatorAdapter) AddPushTroubleshootingTips(err error, registryURL string) []string {
	return lva.service.validators.Build.GeneratePushTroubleshootingTips(err, registryURL)
}

// ValidateUnified provides compatibility for existing unified validation
func (lva *LegacyValidatorAdapter) ValidateUnified(ctx context.Context, args interface{}) error {
	// This would need to be implemented based on the specific legacy interface requirements
	// For now, return nil to allow compilation
	return nil
}
