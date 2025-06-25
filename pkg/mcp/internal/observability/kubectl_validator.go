package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// KubectlValidator implements K8sValidationClient using kubectl
type KubectlValidator struct {
	logger      zerolog.Logger
	kubectlPath string
	kubeContext string
	timeout     time.Duration
}

// KubectlValidationOptions holds options for kubectl validation
type KubectlValidationOptions struct {
	KubectlPath string        `json:"kubectl_path,omitempty"`
	KubeContext string        `json:"kube_context,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	DryRunMode  string        `json:"dry_run_mode,omitempty"` // "client", "server", "none"
}

// KubectlError represents an error from kubectl command
type KubectlError struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Message  string `json:"message"`
}

func (e *KubectlError) Error() string {
	return fmt.Sprintf("kubectl error (exit %d): %s - %s", e.ExitCode, e.Message, e.Stderr)
}

// KubectlServerInfo represents kubectl server information
type KubectlServerInfo struct {
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	GitVersion string `json:"gitVersion"`
}

// NewKubectlValidator creates a new kubectl-based validator
func NewKubectlValidator(logger zerolog.Logger, options KubectlValidationOptions) *KubectlValidator {
	kubectlPath := options.KubectlPath
	if kubectlPath == "" {
		kubectlPath = "kubectl"
	}

	timeout := options.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &KubectlValidator{
		logger:      logger,
		kubectlPath: kubectlPath,
		kubeContext: options.KubeContext,
		timeout:     timeout,
	}
}

// ValidateManifest validates a manifest using kubectl
func (kv *KubectlValidator) ValidateManifest(ctx context.Context, manifest []byte) (*ValidationResult, error) {
	start := time.Now()

	result := &ValidationResult{
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Timestamp: start,
	}

	// Create temporary file for the manifest
	tmpFile, err := kv.createTempManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp manifest file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Run kubectl validate
	validateErr := kv.runKubectlValidate(ctx, tmpFile, result)
	if validateErr != nil {
		kv.logger.Debug().Err(validateErr).Msg("kubectl validate had issues")
	}

	result.Duration = time.Since(start)
	return result, nil
}

// DryRunManifest performs a dry-run validation using kubectl
func (kv *KubectlValidator) DryRunManifest(ctx context.Context, manifest []byte) (*DryRunResult, error) {
	start := time.Now()

	result := &DryRunResult{
		Accepted:  true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Timestamp: start,
	}

	// Create temporary file for the manifest
	tmpFile, err := kv.createTempManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp manifest file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Run kubectl apply --dry-run=server
	dryRunErr := kv.runKubectlDryRun(ctx, tmpFile, result)
	if dryRunErr != nil {
		kv.logger.Debug().Err(dryRunErr).Msg("kubectl dry-run had issues")
		result.Accepted = false
	}

	result.Duration = time.Since(start)
	return result, nil
}

// GetSupportedVersions returns supported Kubernetes API versions
func (kv *KubectlValidator) GetSupportedVersions(ctx context.Context) ([]string, error) {
	args := []string{"api-versions"}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd := exec.CommandContext(ctx, kv.kubectlPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get API versions: %w", err)
	}

	versions := strings.Split(strings.TrimSpace(string(output)), "\n")
	return versions, nil
}

// GetServerVersion returns the Kubernetes server version
func (kv *KubectlValidator) GetServerVersion(ctx context.Context) (*KubectlServerInfo, error) {
	args := []string{"version", "--output=json", "--short"}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd := exec.CommandContext(ctx, kv.kubectlPath, args...)
	output, err := cmd.Output()
	if err != nil {
		// Try without --short flag for older kubectl versions
		args = []string{"version", "--output=json"}
		if kv.kubeContext != "" {
			args = append([]string{"--context", kv.kubeContext}, args...)
		}
		cmd = exec.CommandContext(ctx, kv.kubectlPath, args...)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get server version: %w", err)
		}
	}

	var versionInfo struct {
		ServerVersion *KubectlServerInfo `json:"serverVersion"`
	}

	if err := json.Unmarshal(output, &versionInfo); err != nil {
		return nil, fmt.Errorf("failed to parse version info: %w", err)
	}

	if versionInfo.ServerVersion == nil {
		return nil, fmt.Errorf("server version not available")
	}

	return versionInfo.ServerVersion, nil
}

// IsAvailable checks if kubectl is available and can connect to a cluster
func (kv *KubectlValidator) IsAvailable(ctx context.Context) bool {
	// Check if kubectl binary exists
	args := []string{"version", "--client"}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd := exec.CommandContext(ctx, kv.kubectlPath, args...)
	if err := cmd.Run(); err != nil {
		kv.logger.Debug().Err(err).Msg("kubectl client not available")
		return false
	}

	// Check if server is reachable
	args = []string{"cluster-info"}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd = exec.CommandContext(ctx, kv.kubectlPath, args...)
	if err := cmd.Run(); err != nil {
		kv.logger.Debug().Err(err).Msg("kubectl server not reachable")
		return false
	}

	return true
}

// runKubectlValidate runs kubectl validate command
func (kv *KubectlValidator) runKubectlValidate(ctx context.Context, manifestFile string, result *ValidationResult) error {
	args := []string{"apply", "--validate=true", "--dry-run=client", "-f", manifestFile}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd := exec.CommandContext(ctx, kv.kubectlPath, args...)
	output, err := cmd.CombinedOutput()

	outputStr := string(output)

	if err != nil {
		// Parse kubectl error output
		kv.parseKubectlError(outputStr, result, "validation")
		return err
	}

	// Parse any warnings from successful validation
	kv.parseKubectlWarnings(outputStr, result)

	return nil
}

// runKubectlDryRun runs kubectl dry-run command
func (kv *KubectlValidator) runKubectlDryRun(ctx context.Context, manifestFile string, result *DryRunResult) error {
	args := []string{"apply", "--dry-run=server", "-f", manifestFile}
	if kv.kubeContext != "" {
		args = append([]string{"--context", kv.kubeContext}, args...)
	}

	cmd := exec.CommandContext(ctx, kv.kubectlPath, args...)
	output, err := cmd.CombinedOutput()

	outputStr := string(output)

	if err != nil {
		// Parse kubectl error output
		kv.parseDryRunError(outputStr, result)
		return err
	}

	// Parse any warnings from successful dry-run
	kv.parseDryRunWarnings(outputStr, result)

	return nil
}

// parseKubectlError parses kubectl error output into ValidationErrors
func (kv *KubectlValidator) parseKubectlError(output string, result *ValidationResult, context string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse common kubectl error patterns
		var validationError ValidationError

		if strings.Contains(line, "error validating data") {
			validationError = ValidationError{
				Field:    "validation",
				Message:  line,
				Code:     "KUBECTL_VALIDATION_ERROR",
				Severity: SeverityError,
			}
		} else if strings.Contains(line, "unable to recognize") {
			validationError = ValidationError{
				Field:    "apiVersion",
				Message:  line,
				Code:     "UNRECOGNIZED_API_VERSION",
				Severity: SeverityError,
			}
		} else if strings.Contains(line, "no matches for kind") {
			validationError = ValidationError{
				Field:    "kind",
				Message:  line,
				Code:     "UNRECOGNIZED_KIND",
				Severity: SeverityError,
			}
		} else if strings.Contains(line, "error:") || strings.Contains(line, "Error:") {
			validationError = ValidationError{
				Field:    context,
				Message:  line,
				Code:     "KUBECTL_ERROR",
				Severity: SeverityError,
			}
		} else {
			// Generic error
			validationError = ValidationError{
				Field:    context,
				Message:  line,
				Code:     "KUBECTL_UNKNOWN_ERROR",
				Severity: SeverityWarning,
			}
		}

		if validationError.Message != "" {
			result.Errors = append(result.Errors, validationError)
			result.Valid = false
		}
	}
}

// parseKubectlWarnings parses kubectl warning output
func (kv *KubectlValidator) parseKubectlWarnings(output string, result *ValidationResult) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Warning:") || strings.Contains(line, "warning") {
			warning := ValidationWarning{
				Field:   "kubectl",
				Message: strings.TrimPrefix(line, "Warning: "),
				Code:    "KUBECTL_WARNING",
			}
			result.Warnings = append(result.Warnings, warning)
		}
	}
}

// parseDryRunError parses dry-run error output
func (kv *KubectlValidator) parseDryRunError(output string, result *DryRunResult) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var validationError ValidationError

		if strings.Contains(line, "admission webhook") {
			validationError = ValidationError{
				Field:    "admission",
				Message:  line,
				Code:     "ADMISSION_WEBHOOK_ERROR",
				Severity: SeverityError,
			}
		} else if strings.Contains(line, "forbidden") {
			validationError = ValidationError{
				Field:    "authorization",
				Message:  line,
				Code:     "AUTHORIZATION_ERROR",
				Severity: SeverityError,
			}
		} else if strings.Contains(line, "already exists") {
			validationError = ValidationError{
				Field:    "resource",
				Message:  line,
				Code:     "RESOURCE_EXISTS",
				Severity: SeverityWarning,
			}
		} else if strings.Contains(line, "error:") || strings.Contains(line, "Error:") {
			validationError = ValidationError{
				Field:    "dry_run",
				Message:  line,
				Code:     "DRY_RUN_ERROR",
				Severity: SeverityError,
			}
		}

		if validationError.Message != "" {
			result.Errors = append(result.Errors, validationError)
		}
	}
}

// parseDryRunWarnings parses dry-run warning output
func (kv *KubectlValidator) parseDryRunWarnings(output string, result *DryRunResult) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Warning:") || strings.Contains(line, "warning") {
			warning := ValidationWarning{
				Field:   "dry_run",
				Message: strings.TrimPrefix(line, "Warning: "),
				Code:    "DRY_RUN_WARNING",
			}
			result.Warnings = append(result.Warnings, warning)
		}
	}
}

// createTempManifest creates a temporary file with the manifest content
func (kv *KubectlValidator) createTempManifest(manifest []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "manifest-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(manifest); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write manifest to temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
