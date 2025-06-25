package build

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// BuildValidatorImpl implements the BuildValidator interface
type BuildValidatorImpl struct {
	logger zerolog.Logger
}

// NewBuildValidator creates a new build validator
func NewBuildValidator(logger zerolog.Logger) *BuildValidatorImpl {
	return &BuildValidatorImpl{
		logger: logger.With().Str("component", "build_validator").Logger(),
	}
}

// ValidateDockerfile validates a Dockerfile
func (v *BuildValidatorImpl) ValidateDockerfile(dockerfilePath string) (*ValidationResult, error) {
	v.logger.Info().Str("dockerfile", dockerfilePath).Msg("Validating Dockerfile")

	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Info:     []string{},
	}

	// Check if file exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: fmt.Sprintf("Dockerfile not found: %s", dockerfilePath),
			Rule:    "file-exists",
		})
		return result, nil
	}

	// Read and parse Dockerfile
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Dockerfile: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Validation state
	hasFrom := false
	inRun := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for continuation
		if strings.HasSuffix(line, "\\") {
			inRun = true
			continue
		}

		// Parse instruction
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		instruction := strings.ToUpper(parts[0])

		// Validate instructions
		switch instruction {
		case "FROM":
			hasFrom = true
			v.validateFromInstruction(parts, lineNum, result)

		case "RUN":
			v.validateRunInstruction(line, lineNum, result)

		case "COPY", "ADD":
			v.validateCopyAddInstruction(parts, lineNum, result, instruction)

		case "USER":
			v.validateUserInstruction(parts, lineNum, result)

		case "EXPOSE":
			v.validateExposeInstruction(parts, lineNum, result)

		case "ENV", "ARG":
			v.validateEnvArgInstruction(parts, lineNum, result, instruction)

		case "WORKDIR":
			v.validateWorkdirInstruction(parts, lineNum, result)

		case "CMD", "ENTRYPOINT":
			v.validateCmdEntrypointInstruction(parts, lineNum, result, instruction)

		default:
			// Check if it's a valid instruction
			if !v.isValidInstruction(instruction) {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					Message: fmt.Sprintf("Unknown instruction: %s", instruction),
					Rule:    "valid-instruction",
				})
				result.Valid = false
			}
		}

		if !inRun {
			inRun = false
		}
	}

	// Final validations
	if !hasFrom {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: "Dockerfile must have at least one FROM instruction",
			Rule:    "required-from",
		})
	}

	// Add general recommendations
	v.addGeneralRecommendations(result)

	v.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Dockerfile validation completed")

	return result, nil
}

// ValidateBuildContext validates the build context
func (v *BuildValidatorImpl) ValidateBuildContext(ctx BuildContext) (*ValidationResult, error) {
	v.logger.Info().Str("build_path", ctx.BuildPath).Msg("Validating build context")

	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Info:     []string{},
	}

	// Check if build context exists
	if _, err := os.Stat(ctx.BuildPath); os.IsNotExist(err) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: fmt.Sprintf("Build context not found: %s", ctx.BuildPath),
			Rule:    "context-exists",
		})
		return result, nil
	}

	// Check context size
	contextSize, fileCount, err := v.calculateContextSize(ctx.BuildPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate context size: %w", err)
	}

	// Warn if context is too large
	if contextSize > 100*1024*1024 { // 100MB
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: fmt.Sprintf("Build context is large (%d MB). Consider using .dockerignore", contextSize/(1024*1024)),
			Rule:    "context-size",
		})
	}

	result.Info = append(result.Info, fmt.Sprintf("Build context: %d files, %.2f MB", fileCount, float64(contextSize)/(1024*1024)))

	// Check for .dockerignore
	dockerignorePath := filepath.Join(ctx.BuildPath, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "No .dockerignore file found. Consider adding one to exclude unnecessary files",
			Rule:    "dockerignore-exists",
		})
	}

	// Check for sensitive files
	v.checkForSensitiveFiles(ctx.BuildPath, result)

	return result, nil
}

// ValidateSecurityRequirements validates security aspects
func (v *BuildValidatorImpl) ValidateSecurityRequirements(dockerfilePath string) (*SecurityValidationResult, error) {
	v.logger.Info().Str("dockerfile", dockerfilePath).Msg("Validating security requirements")

	result := &SecurityValidationResult{
		Secure:               true,
		CriticalIssues:       []SecurityIssue{},
		HighIssues:           []SecurityIssue{},
		MediumIssues:         []SecurityIssue{},
		LowIssues:            []SecurityIssue{},
		BestPractices:        []string{},
		ComplianceViolations: []ComplianceViolation{},
	}

	// Read Dockerfile
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Security checks
	v.checkBaseImageSecurity(lines, result)
	v.checkUserPrivileges(lines, result)
	v.checkSecretsExposure(lines, result)
	v.checkNetworkExposure(lines, result)
	v.checkPackageManagement(lines, result)

	// Update secure status based on critical issues
	if len(result.CriticalIssues) > 0 {
		result.Secure = false
	}

	v.logger.Info().
		Bool("secure", result.Secure).
		Int("critical", len(result.CriticalIssues)).
		Int("high", len(result.HighIssues)).
		Msg("Security validation completed")

	return result, nil
}

// Helper methods for Dockerfile validation

func (v *BuildValidatorImpl) validateFromInstruction(parts []string, lineNum int, result *ValidationResult) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: "FROM instruction requires an image name",
			Rule:    "from-syntax",
		})
		result.Valid = false
		return
	}

	image := parts[1]

	// Check for latest tag
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "Using 'latest' tag or no tag is not recommended for reproducible builds",
			Rule:    "no-latest-tag",
		})
	}

	// Check for official images
	if !strings.Contains(image, "/") {
		result.Info = append(result.Info, fmt.Sprintf("Using official image: %s", image))
	}
}

func (v *BuildValidatorImpl) validateRunInstruction(line string, lineNum int, result *ValidationResult) {
	runCmd := strings.TrimPrefix(line, "RUN")
	runCmd = strings.TrimSpace(runCmd)

	// Check for apt-get update without install
	if strings.Contains(runCmd, "apt-get update") && !strings.Contains(runCmd, "apt-get install") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "apt-get update without install creates unnecessary layers",
			Rule:    "apt-update-install",
		})
	}

	// Check for missing -y flag
	if strings.Contains(runCmd, "apt-get install") && !strings.Contains(runCmd, "-y") {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: "apt-get install requires -y flag for non-interactive mode",
			Rule:    "apt-noninteractive",
		})
	}

	// Check for cd commands
	if strings.HasPrefix(runCmd, "cd ") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "Use WORKDIR instead of cd for changing directories",
			Rule:    "use-workdir",
		})
	}
}

func (v *BuildValidatorImpl) validateCopyAddInstruction(parts []string, lineNum int, result *ValidationResult, instruction string) {
	if len(parts) < 3 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: fmt.Sprintf("%s instruction requires source and destination", instruction),
			Rule:    "copy-syntax",
		})
		result.Valid = false
		return
	}

	// Prefer COPY over ADD
	if instruction == "ADD" && !v.isAddNecessary(parts[1]) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "Use COPY instead of ADD unless you need ADD's tar extraction or URL features",
			Rule:    "prefer-copy",
		})
	}
}

func (v *BuildValidatorImpl) isAddNecessary(source string) bool {
	// ADD is necessary for URLs and tar files
	return strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasSuffix(source, ".tar") ||
		strings.HasSuffix(source, ".tar.gz") ||
		strings.HasSuffix(source, ".tgz")
}

func (v *BuildValidatorImpl) isValidInstruction(instruction string) bool {
	validInstructions := []string{
		"FROM", "RUN", "CMD", "LABEL", "MAINTAINER", "EXPOSE", "ENV",
		"ADD", "COPY", "ENTRYPOINT", "VOLUME", "USER", "WORKDIR",
		"ARG", "ONBUILD", "STOPSIGNAL", "HEALTHCHECK", "SHELL",
	}

	for _, valid := range validInstructions {
		if instruction == valid {
			return true
		}
	}
	return false
}

// Security validation helpers

func (v *BuildValidatorImpl) checkBaseImageSecurity(lines []string, result *SecurityValidationResult) {
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "FROM") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				image := parts[1]

				// Check for vulnerable base images
				vulnerableImages := []string{
					"ubuntu:14.04", "ubuntu:16.04", "debian:8", "debian:9",
					"centos:6", "centos:7", "alpine:3.3", "alpine:3.4",
				}

				for _, vulnerable := range vulnerableImages {
					if strings.Contains(image, vulnerable) {
						result.HighIssues = append(result.HighIssues, SecurityIssue{
							Severity:    "HIGH",
							Type:        "outdated-base-image",
							Message:     fmt.Sprintf("Base image '%s' is outdated and may contain vulnerabilities", image),
							Line:        i + 1,
							Remediation: "Use a more recent version of the base image",
						})
					}
				}
			}
		}
	}
}

func (v *BuildValidatorImpl) checkUserPrivileges(lines []string, result *SecurityValidationResult) {
	hasUser := false
	lastUser := ""

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "USER") {
			hasUser = true
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				lastUser = parts[1]
				if lastUser == "root" || lastUser == "0" {
					result.MediumIssues = append(result.MediumIssues, SecurityIssue{
						Severity:    "MEDIUM",
						Type:        "root-user",
						Message:     "Container runs as root user",
						Line:        i + 1,
						Remediation: "Create and use a non-root user",
					})
				}
			}
		}
	}

	if !hasUser {
		result.HighIssues = append(result.HighIssues, SecurityIssue{
			Severity:    "HIGH",
			Type:        "no-user",
			Message:     "No USER instruction found, container will run as root by default",
			Line:        0,
			Remediation: "Add a USER instruction to run as non-root",
		})
	}

	// Best practice
	if hasUser && lastUser != "root" && lastUser != "0" {
		result.BestPractices = append(result.BestPractices, "Container configured to run as non-root user")
	}
}

func (v *BuildValidatorImpl) checkSecretsExposure(lines []string, result *SecurityValidationResult) {
	secretPatterns := []string{
		"password", "passwd", "secret", "api_key", "apikey",
		"token", "private_key", "privatekey",
	}

	for i, line := range lines {
		lineLower := strings.ToLower(line)

		// Check ENV instructions
		if strings.HasPrefix(strings.ToUpper(line), "ENV") {
			for _, pattern := range secretPatterns {
				if strings.Contains(lineLower, pattern) {
					result.CriticalIssues = append(result.CriticalIssues, SecurityIssue{
						Severity:    "CRITICAL",
						Type:        "hardcoded-secret",
						Message:     "Potential secret exposed in ENV instruction",
						Line:        i + 1,
						Remediation: "Use Docker secrets or build arguments instead of hardcoding secrets",
					})
				}
			}
		}

		// Check ARG instructions with defaults
		if strings.HasPrefix(strings.ToUpper(line), "ARG") && strings.Contains(line, "=") {
			for _, pattern := range secretPatterns {
				if strings.Contains(lineLower, pattern) {
					result.HighIssues = append(result.HighIssues, SecurityIssue{
						Severity:    "HIGH",
						Type:        "default-secret",
						Message:     "Potential secret in ARG default value",
						Line:        i + 1,
						Remediation: "Remove default values for sensitive arguments",
					})
				}
			}
		}
	}
}

// Utility methods

func (v *BuildValidatorImpl) calculateContextSize(path string) (int64, int, error) {
	var size int64
	var count int

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
			count++
		}
		return nil
	})

	return size, count, err
}

func (v *BuildValidatorImpl) checkForSensitiveFiles(path string, result *ValidationResult) {
	sensitiveFiles := []string{
		".git", ".env", ".aws", ".ssh", "id_rsa", "id_dsa",
		".npmrc", ".pypirc", ".netrc", ".bash_history",
	}

	for _, sensitive := range sensitiveFiles {
		checkPath := filepath.Join(path, sensitive)
		if _, err := os.Stat(checkPath); err == nil {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Message: fmt.Sprintf("Sensitive file/directory '%s' found in build context", sensitive),
				Rule:    "sensitive-files",
			})
		}
	}
}

func (v *BuildValidatorImpl) addGeneralRecommendations(result *ValidationResult) {
	result.Info = append(result.Info, "Consider using multi-stage builds to reduce image size")
	result.Info = append(result.Info, "Use specific version tags for base images")
	result.Info = append(result.Info, "Combine RUN commands to reduce layers")
}
