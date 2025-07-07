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

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
)

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
func (hv *HadolintValidator) ValidateWithHadolint(ctx context.Context, dockerfileContent string) (*types.BuildValidationResult, error) {
	// Check if Hadolint is installed
	hadolintPath, err := hv.findHadolint()
	if err != nil {
		hv.logger.Warn("Hadolint not found, falling back to basic validation", "error", err)
		return nil, mcperrors.NewError().Messagef("hadolint not available: %w", err).WithLocation().Build()
	}
	hv.hadolintPath = hadolintPath

	// Create temporary file for Dockerfile content
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("dockerfile_%d", os.Getpid()))
	if err := os.WriteFile(tmpFile, []byte(dockerfileContent), 0644); err != nil {
		return nil, mcperrors.NewError().Messagef("failed to write temporary Dockerfile: %w", err).WithLocation().Build()
	}
	defer os.Remove(tmpFile)

	// Run Hadolint with JSON output
	cmd := exec.CommandContext(ctx, hv.hadolintPath, "--format", "json", tmpFile)
	output, err := cmd.Output()

	// Hadolint returns non-zero exit code when it finds issues
	// We need to check if we got valid output regardless of exit code
	if err != nil && len(output) == 0 {
		return nil, mcperrors.NewError().Messagef("hadolint execution failed: %w", err).WithLocation(

		// Parse Hadolint JSON output
		).Build()
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
	result := types.NewBuildResult()
	result.Metadata.ValidatorName = "hadolint"
	result.Metadata.ValidatorVersion = hv.getHadolintVersion()

	result.Metadata.Context["hadolint_version"] = hv.getHadolintVersion()
	result.Metadata.Context["total_issues"] = fmt.Sprintf("%d", len(hadolintResults))

	// Process each Hadolint finding
	criticalCount := 0
	for _, hr := range hadolintResults {
		switch hr.Level {
		case "error":
			hadolintError := &validation.Error{
				Code:     hr.Code,
				Message:  fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
				Severity: validation.SeverityHigh,
				Context:  make(map[string]string),
			}
			hadolintError.Context["line"] = fmt.Sprintf("%d", hr.Line)
			hadolintError.Context["column"] = fmt.Sprintf("%d", hr.Column)
			result.Errors = append(result.Errors, *hadolintError)
			criticalCount++

		case "warning":
			// Treat DL3008 (pin versions) and DL3009 (delete apt lists) as critical
			if hr.Code == "DL3008" || hr.Code == "DL3009" {
				securityError := &validation.Error{
					Code:     hr.Code,
					Message:  fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
					Severity: validation.SeverityHigh,
					Context:  make(map[string]string),
				}
				securityError.Context["line"] = fmt.Sprintf("%d", hr.Line)
				securityError.Context["column"] = fmt.Sprintf("%d", hr.Column)
				result.Errors = append(result.Errors, *securityError)
				criticalCount++
			} else {
				hadolintWarning := &validation.Warning{
					Code:    hr.Code,
					Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
					Context: make(map[string]string),
				}
				// Add line information to context
				hadolintWarning.Context["line"] = fmt.Sprintf("%d", hr.Line)
				hadolintWarning.Context["column"] = fmt.Sprintf("%d", hr.Column)
				suggestion := hv.getSuggestionForCode(hr.Code)
				if suggestion != "" {
					hadolintWarning.Suggestion = suggestion
				}
				result.Warnings = append(result.Warnings, *hadolintWarning)
			}

		case "info", "style":
			styleWarning := &validation.Warning{
				Code:    hr.Code,
				Message: fmt.Sprintf("[%s] %s", hr.Code, hr.Message),
				Context: make(map[string]string),
			}
			styleWarning.Context["line"] = fmt.Sprintf("%d", hr.Line)
			styleWarning.Context["column"] = fmt.Sprintf("%d", hr.Column)
			result.Warnings = append(result.Warnings, *styleWarning)
		}
	}

	// Add suggestions based on common issues
	hv.addHadolintSuggestions(hadolintResults, result)

	// Set validity based on critical errors
	result.Valid = criticalCount == 0
	result.Metadata.Context["critical_issues"] = fmt.Sprintf("%d", criticalCount)

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

// getSuggestionForCode provides specific suggestions for Hadolint codes
func (hv *HadolintValidator) getSuggestionForCode(code string) string {
	suggestions := map[string]string{
		"DL3000": "Use absolute WORKDIR paths for clarity",
		"DL3001": "Consider using --no-install-recommends with apt-get install",
		"DL3002": "Last USER should not be root for security",
		"DL3003": "Use WORKDIR to switch to a directory",
		"DL3006": "Always tag the version of an image explicitly",
		"DL3007": "Using latest is prone to errors if the image updates",
		"DL3008": "Pin versions in apt-get install for reproducibility",
		"DL3009": "Delete the apt-get lists after installing packages",
		"DL3015": "Avoid additional packages by specifying --no-install-recommends",
		"DL3020": "Use COPY instead of ADD for files and folders",
		"DL4006": "Set the SHELL option -o pipefail before RUN with pipe",
		"SC2086": "Double quote variables to prevent globbing and word splitting",
	}

	if suggestion, ok := suggestions[code]; ok {
		return suggestion
	}
	return ""
}

// addHadolintSuggestions adds general suggestions based on Hadolint findings
func (hv *HadolintValidator) addHadolintSuggestions(results []HadolintResult, validation *types.BuildValidationResult) {
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

	// Store suggestions in Details map
	if validation.Details == nil {
		validation.Details = make(map[string]interface{})
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

	validation.Details["suggestions"] = suggestions
}

// CheckHadolintInstalled checks if Hadolint is available
func (hv *HadolintValidator) CheckHadolintInstalled() bool {
	_, err := hv.findHadolint()
	return err == nil
}
