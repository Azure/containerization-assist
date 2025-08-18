package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mcperrors "github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/Azure/containerization-assist/pkg/mcp/api"
)

// Simple local validation types to replace deleted domain/security package
type Error struct {
	Code     string
	Field    string
	Message  string
	Severity string
	Context  map[string]interface{}
}

type Warning struct {
	Field   string
	Message string
	Context map[string]interface{}
}

const (
	SeverityHigh = "high"
)

// BuildValidationResult is a local type to avoid import cycles
type BuildValidationResult = api.BuildValidationResult

// NewBuildResult creates a new BuildValidationResult
func NewBuildResult() *BuildValidationResult {
	return &api.BuildValidationResult{
		ValidationResult: api.ValidationResult{
			Valid:    true,
			Errors:   make([]api.ValidationError, 0),
			Warnings: make([]api.ValidationWarning, 0),
			Metadata: make(map[string]interface{}),
		},
	}
}

// HadolintValidator provides Hadolint-based Dockerfile validation
type HadolintValidator struct {
	logger       *slog.Logger
	hadolintPath string
}

// NewHadolintValidator creates a new Hadolint validator
func NewHadolintValidator(logger *slog.Logger) *HadolintValidator {
	return &HadolintValidator{
		logger: logger.With("component", "hadolint_validator"),
	}
}

// HadolintResult represents the output from Hadolint
type HadolintResult struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Level   string `json:"level"` // error, warning, info, style
	Code    string `json:"code"`  // DL3000, SC2086, etc.
	Message string `json:"message"`
	File    string `json:"file"`
}

// ValidateWithHadolint validates a Dockerfile using Hadolint
func (hv *HadolintValidator) ValidateWithHadolint(ctx context.Context, dockerfileContent string) (*BuildValidationResult, error) {
	// Check if Hadolint is installed
	hadolintPath, err := hv.findHadolint()
	if err != nil {
		hv.logger.Warn("Hadolint not found, falling back to basic validation", "error", err)
		return nil, mcperrors.New(mcperrors.CodeInternalError, "core", "hadolint not available", err)
	}
	hv.hadolintPath = hadolintPath

	// Create temporary file for Dockerfile content
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("dockerfile_%d", os.Getpid()))
	if err := os.WriteFile(tmpFile, []byte(dockerfileContent), 0644); err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to write temporary Dockerfile", err)
	}
	defer os.Remove(tmpFile)

	// Run Hadolint with JSON output
	cmd := exec.CommandContext(ctx, hv.hadolintPath, "--format", "json", tmpFile)
	output, err := cmd.Output()

	// Hadolint returns non-zero exit code when it finds issues
	// We need to check if we got valid output regardless of exit code
	if err != nil && len(output) == 0 {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "docker", "hadolint execution failed", err)
	}

	var hadolintResults []HadolintResult
	if len(output) > 0 {
		if err := json.Unmarshal(output, &hadolintResults); err != nil {
			// Try to parse as line-separated JSON (some versions output this way)
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var result HadolintResult
				if err := json.Unmarshal([]byte(line), &result); err == nil {
					hadolintResults = append(hadolintResults, result)
				}
			}
		}
	}

	// Convert Hadolint results to ValidationResult using factory function
	result := NewBuildResult()
	result.Metadata["validator_name"] = "hadolint"
	result.Metadata["validator_version"] = hv.getHadolintVersion()

	result.Metadata["hadolint_version"] = hv.getHadolintVersion()
	result.Metadata["total_issues"] = fmt.Sprintf("%d", len(hadolintResults))

	// Process each Hadolint finding
	criticalCount := 0
	for _, hr := range hadolintResults {
		switch hr.Level {
		case "error":
			hadolintError := api.ValidationError{
				Code:    hr.Code,
				Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
				Field:   fmt.Sprintf("line:%d", hr.Line),
			}
			result.Errors = append(result.Errors, hadolintError)
			criticalCount++

		case "warning":
			// Treat DL3008 (pin versions) and DL3009 (delete apt lists) as critical
			if hr.Code == "DL3008" || hr.Code == "DL3009" {
				securityError := api.ValidationError{
					Code:    hr.Code,
					Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
					Field:   fmt.Sprintf("line:%d", hr.Line),
				}
				result.Errors = append(result.Errors, securityError)
				criticalCount++
			} else {
				hadolintWarning := api.ValidationWarning{
					Field:   fmt.Sprintf("line:%d", hr.Line),
					Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
				}
				result.Warnings = append(result.Warnings, hadolintWarning)
			}

		case "info", "style":
			styleWarning := api.ValidationWarning{
				Field:   fmt.Sprintf("line:%d", hr.Line),
				Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
			}
			result.Warnings = append(result.Warnings, styleWarning)
		}
	}

	// Add suggestions based on common issues
	hv.addHadolintSuggestions(hadolintResults, result)

	// Set validity based on critical errors
	result.Valid = criticalCount == 0
	result.Metadata["critical_issues"] = fmt.Sprintf("%d", criticalCount)

	hv.logger.Info("Hadolint validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
		"critical", criticalCount)

	return result, nil
}

// findHadolint locates the Hadolint executable
func (hv *HadolintValidator) findHadolint() (string, error) {
	// Check common locations
	paths := []string{
		"hadolint",
		"/usr/local/bin/hadolint",
		"/usr/bin/hadolint",
		"/opt/hadolint/hadolint",
	}

	for _, path := range paths {
		if p, err := exec.LookPath(path); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("hadolint executable not found in PATH")
}

// getHadolintVersion gets the Hadolint version
func (hv *HadolintValidator) getHadolintVersion() string {
	if hv.hadolintPath == "" {
		return "unknown"
	}

	cmd := exec.Command(hv.hadolintPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	version := strings.TrimSpace(string(output))
	// Extract version number from output like "Haskell Dockerfile Linter 2.12.0"
	parts := strings.Fields(version)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return version
}

// addHadolintSuggestions adds general suggestions based on Hadolint findings
func (hv *HadolintValidator) addHadolintSuggestions(results []HadolintResult, result *BuildValidationResult) {
	hasSecurityIssues := false
	hasVersionPinning := false
	hasRootUser := false

	for _, result := range results {
		switch result.Code {
		case "DL3002":
			hasRootUser = true
		case "DL3006", "DL3007", "DL3008":
			hasVersionPinning = true
		case "DL3009", "DL3015":
			hasSecurityIssues = true
		}
	}

	// Store suggestions in Metadata map
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}

	suggestions := []string{}

	if hasSecurityIssues {
		suggestions = append(suggestions,
			"Security: Clean package manager cache and use --no-install-recommends to minimize attack surface")
	}

	if hasVersionPinning {
		suggestions = append(suggestions,
			"Reproducibility: Pin all package versions and base image tags for consistent builds")
	}

	if hasRootUser {
		suggestions = append(suggestions,
			"Security: Create and switch to a non-root user for running the application")
	}

	// General best practices
	suggestions = append(suggestions,
		"Consider using multi-stage builds to reduce final image size",
		"Add a .dockerignore file to exclude unnecessary files from the build context",
	)

	result.Metadata["suggestions"] = suggestions
}

// CheckHadolintInstalled checks if Hadolint is available
func (hv *HadolintValidator) CheckHadolintInstalled() bool {
	_, err := hv.findHadolint()
	return err == nil
}
