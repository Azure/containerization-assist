package security

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// FileSystemValidator provides common file system validation functions
type FileSystemValidator struct{}

// NewFileSystemValidator creates a new file system validator
func NewFileSystemValidator() *FileSystemValidator {
	return &FileSystemValidator{}
}

// ValidateDockerfileExists checks if a Dockerfile exists at the given path
func (fsv *FileSystemValidator) ValidateDockerfileExists(dockerfilePath string) error {
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return errors.Validationf("filesystem", "Dockerfile not found at %s", dockerfilePath)
	}
	return nil
}

// ValidateDirectoryExists checks if a directory exists at the given path
func (fsv *FileSystemValidator) ValidateDirectoryExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return errors.Validationf("filesystem", "Directory not found at %s", dirPath)
	}
	return nil
}

// ValidateFileExists checks if a file exists at the given path
func (fsv *FileSystemValidator) ValidateFileExists(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.Validationf("filesystem", "File not found at %s", filePath)
	}
	return nil
}

// SystemValidator provides common system validation functions
type SystemValidator struct{}

// NewSystemValidator creates a new system validator
func NewSystemValidator() *SystemValidator {
	return &SystemValidator{}
}

// ValidateDockerAvailable checks if Docker is available and running
func (sv *SystemValidator) ValidateDockerAvailable() error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return errors.Validation("system", "Docker is not available. Please ensure Docker is installed and running")
	}
	return nil
}

// ValidateCommandAvailable checks if a command is available in PATH
func (sv *SystemValidator) ValidateCommandAvailable(command string) error {
	_, err := exec.LookPath(command)
	if err != nil {
		return errors.Validationf("system", "Command '%s' not found in PATH", command)
	}
	return nil
}

// InputValidator provides common input validation functions
type InputValidator struct{}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateSessionID validates a session ID is not empty
func (iv *InputValidator) ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return errors.Validation("input", "session_id is required")
	}
	return nil
}

// ValidateImageName validates Docker image name format
func (iv *InputValidator) ValidateImageName(imageName string) error {
	if imageName == "" {
		return errors.Validation("input", "image name is required")
	}

	// Basic Docker image name validation
	// Allow: registry.com/repo/image:tag or repo/image:tag or image:tag
	imageNameRegex := regexp.MustCompile(`^([a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:([a-zA-Z0-9._-]+))?$`)
	if !imageNameRegex.MatchString(imageName) {
		return errors.Validationf("input", "invalid image name format: %s", imageName)
	}

	return nil
}

// ValidateGitURL validates a Git repository URL
func (iv *InputValidator) ValidateGitURL(repoURL string) error {
	if repoURL == "" {
		return errors.Validation("input", "repository URL is required")
	}

	// Basic Git URL validation
	if !strings.HasPrefix(repoURL, "https://") &&
		!strings.HasPrefix(repoURL, "http://") &&
		!strings.HasPrefix(repoURL, "git@") {
		return errors.Validationf("input", "invalid repository URL format: %s", repoURL)
	}

	return nil
}

// ValidateKubernetesName validates Kubernetes resource name format
func (iv *InputValidator) ValidateKubernetesName(name string) error {
	if name == "" {
		return errors.Validation("input", "Kubernetes name is required")
	}

	// Kubernetes name validation (RFC 1123 subdomain)
	nameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !nameRegex.MatchString(name) {
		return errors.Validationf("input", "invalid Kubernetes name format: %s", name)
	}

	if len(name) > 63 {
		return errors.Validationf("input", "Kubernetes name too long (max 63 characters): %s", name)
	}

	return nil
}

// ManifestValidator provides Kubernetes manifest validation functions
type ManifestValidator struct{}

// NewManifestValidator creates a new manifest validator
func NewManifestValidator() *ManifestValidator {
	return &ManifestValidator{}
}

// ValidateManifestFiles validates that manifest files exist and are readable
func (mv *ManifestValidator) ValidateManifestFiles(manifests []string) error {
	if len(manifests) == 0 {
		return errors.Validation("manifest", "at least one manifest file is required")
	}

	for _, manifest := range manifests {
		if manifest == "" {
			return errors.Validation("manifest", "manifest path cannot be empty")
		}

		if _, err := os.Stat(manifest); os.IsNotExist(err) {
			return errors.Validationf("manifest", "manifest file not found: %s", manifest)
		}

		// Check if it's a YAML file
		ext := filepath.Ext(manifest)
		if ext != ".yaml" && ext != ".yml" {
			return errors.Validationf("manifest", "manifest file must be YAML: %s", manifest)
		}
	}

	return nil
}

// UnifiedValidator combines all validators for convenience
type UnifiedValidator struct {
	FileSystem *FileSystemValidator
	System     *SystemValidator
	Input      *InputValidator
	Manifest   *ManifestValidator
}

// NewUnifiedValidator creates a new unified validator with all sub-validators
func NewUnifiedValidator() *UnifiedValidator {
	return &UnifiedValidator{
		FileSystem: NewFileSystemValidator(),
		System:     NewSystemValidator(),
		Input:      NewInputValidator(),
		Manifest:   NewManifestValidator(),
	}
}

// ValidateContext provides validation context for tracking
type ValidateContext struct {
	ctx      context.Context
	errors   []error
	warnings []string
}

// NewValidateContext creates a new validation context
func NewValidateContext(ctx context.Context) *ValidateContext {
	return &ValidateContext{
		ctx:      ctx,
		errors:   make([]error, 0),
		warnings: make([]string, 0),
	}
}

// AddError adds an error to the validation context
func (vc *ValidateContext) AddError(err error) {
	vc.errors = append(vc.errors, err)
}

// AddWarning adds a warning to the validation context
func (vc *ValidateContext) AddWarning(warning string) {
	vc.warnings = append(vc.warnings, warning)
}

// HasErrors returns true if there are any validation errors
func (vc *ValidateContext) HasErrors() bool {
	return len(vc.errors) > 0
}

// GetErrors returns all validation errors
func (vc *ValidateContext) GetErrors() []error {
	return vc.errors
}

// GetWarnings returns all validation warnings
func (vc *ValidateContext) GetWarnings() []string {
	return vc.warnings
}

// GetFirstError returns the first validation error or nil
func (vc *ValidateContext) GetFirstError() error {
	if len(vc.errors) > 0 {
		return vc.errors[0]
	}
	return nil
}
