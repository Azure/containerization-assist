package docker

import (
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
)

// Type aliases are defined in hadolint.go to avoid redeclaration

// Validator provides mechanical Dockerfile validation operations
type Validator struct {
	logger *slog.Logger
}

// NewValidator creates a new Dockerfile validator
func NewValidator(logger *slog.Logger) *Validator {
	return &Validator{
		logger: logger.With("component", "dockerfile_validator"),
	}
}

// Type aliases are defined in hadolint.go to avoid redeclaration

// ValidateDockerfile performs comprehensive validation of Dockerfile content
func (v *Validator) ValidateDockerfile(dockerfileContent string) *BuildValidationResult {
	// Use factory function from unified validation framework
	result := NewBuildResult()
	if result.Metadata.Context == nil {
		result.Metadata.Context = make(map[string]string)
	}
	result.Metadata.ValidatorName = "dockerfile-validator"
	result.Metadata.ValidatorVersion = "1.0.0"

	v.logger.Debug("Starting Dockerfile validation")

	// Basic validation
	if strings.TrimSpace(dockerfileContent) == "" {
		result.Valid = false
		emptyError := &validation.Error{
			Code:     "DOCKERFILE_EMPTY",
			Message:  "Dockerfile is empty",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": "0"},
		}
		result.Errors = append(result.Errors, *emptyError)
		return result
	}

	lines := strings.Split(dockerfileContent, "\n")
	result.Metadata.Context["line_count"] = fmt.Sprintf("%d", len(lines))
	result.Metadata.Context["total_size"] = fmt.Sprintf("%d", len(dockerfileContent))

	// Parse and validate each line, handling line continuations
	var instructions []string
	var currentInstruction strings.Builder
	var currentInstructionStart int
	var currentInstructionName string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if this is a continuation of the previous line
		if currentInstruction.Len() > 0 {
			// Add to current instruction
			currentInstruction.WriteString(" ")
			if strings.HasSuffix(trimmed, "\\") {
				// Remove the backslash and continue
				currentInstruction.WriteString(strings.TrimSuffix(trimmed, "\\"))
			} else {
				// This is the end of the instruction
				currentInstruction.WriteString(trimmed)

				// Validate the complete instruction
				completeInstruction := currentInstruction.String()
				v.validateInstruction(currentInstructionName, completeInstruction, currentInstructionStart, result)

				// Reset for next instruction
				currentInstruction.Reset()
				currentInstructionName = ""
				currentInstructionStart = 0
			}
		} else {
			// Extract instruction
			parts := strings.Fields(trimmed)
			if len(parts) == 0 {
				continue
			}

			instruction := strings.ToUpper(parts[0])
			instructions = append(instructions, instruction)

			if strings.HasSuffix(trimmed, "\\") {
				// This instruction continues on the next line
				currentInstructionName = instruction
				currentInstructionStart = lineNum
				currentInstruction.WriteString(strings.TrimSuffix(trimmed, "\\"))
			} else {
				// This is a complete instruction
				v.validateInstruction(instruction, trimmed, lineNum, result)
			}
		}
	}

	// Handle case where file ends with a line continuation
	if currentInstruction.Len() > 0 {
		completeInstruction := currentInstruction.String()
		v.validateInstruction(currentInstructionName, completeInstruction, currentInstructionStart, result)
	}

	// Validate overall structure
	v.validateStructure(instructions, result)

	// Add general suggestions
	v.addGeneralSuggestions(dockerfileContent, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	v.logger.Debug("Dockerfile validation completed",
		"valid", result.Valid,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings))

	return result
}

// CheckDockerInstallation verifies Docker is installed and accessible
func (v *Validator) CheckDockerInstallation() error {
	// Check if docker executable exists
	if _, err := exec.LookPath("docker"); err != nil {
		return mcperrors.NewError().
			Message("docker executable not found in PATH. Please install Docker").
			WithLocation().
			Build()
	}

	// Check if docker daemon is accessible
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return mcperrors.NewError().
			Message("docker daemon is not running or not accessible. Please start Docker").
			WithLocation().
			Build()
	}

	return nil
}

func (v *Validator) validateInstruction(instruction, line string, lineNum int, result *BuildValidationResult) {
	switch instruction {
	case "FROM":
		v.validateFromInstruction(line, lineNum, result)
	case "RUN":
		v.validateRunInstruction(line, lineNum, result)
	case "COPY", "ADD":
		v.validateCopyAddInstruction(instruction, line, lineNum, result)
	case "EXPOSE":
		v.validateExposeInstruction(line, lineNum, result)
	case "USER":
		v.validateUserInstruction(line, lineNum, result)
	case "WORKDIR":
		v.validateWorkdirInstruction(line, lineNum, result)
	case "CMD", "ENTRYPOINT":
		v.validateCmdEntrypointInstruction(instruction, line, lineNum, result)
	default:
		// Check if it's a valid Dockerfile instruction
		validInstructions := []string{
			"FROM", "RUN", "CMD", "LABEL", "EXPOSE", "ENV", "ADD", "COPY",
			"ENTRYPOINT", "VOLUME", "USER", "WORKDIR", "ARG", "ONBUILD",
			"STOPSIGNAL", "HEALTHCHECK", "SHELL",
		}

		found := false
		for _, valid := range validInstructions {
			if instruction == valid {
				found = true
				break
			}
		}

		if !found {
			unknownError := &validation.Error{
				Code:     "UNKNOWN_INSTRUCTION",
				Message:  fmt.Sprintf("Unknown instruction: %s", instruction),
				Field:    "instruction",
				Severity: validation.SeverityHigh,
				Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			}
			result.Errors = append(result.Errors, *unknownError)
		}
	}
}

// validateFromInstruction validates FROM instructions
func (v *Validator) validateFromInstruction(line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		fromError := &validation.Error{
			Code:     "FROM_MISSING_IMAGE",
			Message:  "FROM instruction requires an image name",
			Field:    "FROM",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *fromError)
		return
	}

	imageName := parts[1]

	// Check for latest tag usage
	if strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":") {
		latestWarning := &validation.Warning{
			Code:       "FROM_LATEST_TAG",
			Message:    "Using 'latest' tag or no tag is not recommended for production",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use specific version tags for reproducible builds",
		}
		result.Warnings = append(result.Warnings, *latestWarning)
	}
}

// validateRunInstruction validates RUN instructions
func (v *Validator) validateRunInstruction(line string, lineNum int, result *BuildValidationResult) {
	// Check for apt-get without update
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
		aptWarning := &validation.Warning{
			Code:       "RUN_APT_UPDATE",
			Message:    "apt-get install should be preceded by apt-get update",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Combine 'apt-get update && apt-get install' in a single RUN instruction",
		}
		result.Warnings = append(result.Warnings, *aptWarning)
	}

	// Check for package manager cache cleanup
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
		cacheWarning := &validation.Warning{
			Code:       "RUN_CACHE_CLEANUP",
			Message:    "Consider cleaning package manager cache to reduce image size",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Add '&& rm -rf /var/lib/apt/lists/*' to clean up after apt-get",
		}
		result.Warnings = append(result.Warnings, *cacheWarning)
	}
}

// validateCopyAddInstruction validates COPY and ADD instructions
func (v *Validator) validateCopyAddInstruction(instruction, line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		copyError := &validation.Error{
			Code:     fmt.Sprintf("%s_MISSING_ARGS", instruction),
			Message:  fmt.Sprintf("%s instruction requires source and destination", instruction),
			Field:    instruction,
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *copyError)
		return
	}

	// Warn about ADD vs COPY
	if instruction == "ADD" && !strings.Contains(line, "http") && !strings.HasSuffix(parts[1], ".tar") {
		addWarning := &validation.Warning{
			Code:       "ADD_VS_COPY",
			Message:    "COPY is preferred over ADD for simple file copying",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use COPY instead of ADD unless you need URL download or tar extraction",
		}
		result.Warnings = append(result.Warnings, *addWarning)
	}
}

// validateExposeInstruction validates EXPOSE instructions
func (v *Validator) validateExposeInstruction(line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		exposeError := &validation.Error{
			Code:     "EXPOSE_MISSING_PORT",
			Message:  "EXPOSE instruction requires a port number",
			Field:    "EXPOSE",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *exposeError)
		return
	}

	// Basic port validation
	portRegex := regexp.MustCompile(`^\d+(/tcp|/udp)?$`)
	for _, port := range parts[1:] {
		if !portRegex.MatchString(port) {
			portError := &validation.Error{
				Code:     "EXPOSE_INVALID_PORT",
				Message:  fmt.Sprintf("Invalid port format: %s", port),
				Field:    "EXPOSE",
				Severity: validation.SeverityHigh,
				Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum), "port": port},
			}
			result.Errors = append(result.Errors, *portError)
		}
	}
}

// validateUserInstruction validates USER instructions
func (v *Validator) validateUserInstruction(line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		userError := &validation.Error{
			Code:     "USER_MISSING_NAME",
			Message:  "USER instruction requires a username or UID",
			Field:    "USER",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *userError)
		return
	}

	user := parts[1]
	if user == "root" {
		rootWarning := &validation.Warning{
			Code:       "USER_ROOT_SECURITY",
			Message:    "Running as root user is a security risk",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Create and use a non-root user for better security",
		}
		result.Warnings = append(result.Warnings, *rootWarning)
	}
}

// validateWorkdirInstruction validates WORKDIR instructions
func (v *Validator) validateWorkdirInstruction(line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		workdirError := &validation.Error{
			Code:     "WORKDIR_MISSING_PATH",
			Message:  "WORKDIR instruction requires a directory path",
			Field:    "WORKDIR",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *workdirError)
		return
	}

	workdir := parts[1]
	if !strings.HasPrefix(workdir, "/") {
		pathWarning := &validation.Warning{
			Code:       "WORKDIR_RELATIVE_PATH",
			Message:    "WORKDIR should use absolute paths",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use absolute paths starting with '/' for WORKDIR",
		}
		result.Warnings = append(result.Warnings, *pathWarning)
	}
}

// validateCmdEntrypointInstruction validates CMD and ENTRYPOINT instructions
func (v *Validator) validateCmdEntrypointInstruction(instruction, line string, lineNum int, result *BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		cmdError := &validation.Error{
			Code:     fmt.Sprintf("%s_MISSING_COMMAND", instruction),
			Message:  fmt.Sprintf("%s instruction requires a command", instruction),
			Field:    instruction,
			Severity: validation.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *cmdError)
	}
}

// validateStructure validates overall Dockerfile structure
func (v *Validator) validateStructure(instructions []string, result *BuildValidationResult) {
	if len(instructions) == 0 {
		emptyError := &validation.Error{
			Code:     "DOCKERFILE_NO_INSTRUCTIONS",
			Message:  "Dockerfile contains no instructions",
			Field:    "structure",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{},
		}
		result.Errors = append(result.Errors, *emptyError)
		return
	}

	// Must start with FROM
	if instructions[0] != "FROM" {
		fromError := &validation.Error{
			Code:     "DOCKERFILE_NO_FROM",
			Message:  "Dockerfile must start with FROM instruction",
			Field:    "structure",
			Severity: validation.SeverityHigh,
			Context:  map[string]string{},
		}
		result.Errors = append(result.Errors, *fromError)
	}

	// Check for multiple CMD or ENTRYPOINT
	cmdCount := 0
	entrypointCount := 0
	for _, instruction := range instructions {
		if instruction == "CMD" {
			cmdCount++
		}
		if instruction == "ENTRYPOINT" {
			entrypointCount++
		}
	}

	if cmdCount > 1 {
		cmdWarning := &validation.Warning{
			Code:       "MULTIPLE_CMD",
			Message:    "Multiple CMD instructions found, only the last one will be effective",
			Context:    map[string]string{"count": fmt.Sprintf("%d", cmdCount)},
			Suggestion: "Use only one CMD instruction",
		}
		result.Warnings = append(result.Warnings, *cmdWarning)
	}

	if entrypointCount > 1 {
		entrypointWarning := &validation.Warning{
			Code:       "MULTIPLE_ENTRYPOINT",
			Message:    "Multiple ENTRYPOINT instructions found, only the last one will be effective",
			Context:    map[string]string{"count": fmt.Sprintf("%d", entrypointCount)},
			Suggestion: "Use only one ENTRYPOINT instruction",
		}
		result.Warnings = append(result.Warnings, *entrypointWarning)
	}

	// Warn about ENTRYPOINT + CMD combination (informational) - only if exactly one of each
	if entrypointCount == 1 && cmdCount == 1 {
		comboWarning := &validation.Warning{
			Code:       "ENTRYPOINT_CMD_COMBO",
			Message:    "Using both ENTRYPOINT and CMD - CMD will be passed as arguments to ENTRYPOINT",
			Context:    map[string]string{},
			Suggestion: "Ensure this is the intended behavior",
		}
		result.Warnings = append(result.Warnings, *comboWarning)
	}
}

// addGeneralSuggestions adds general best practice suggestions
func (v *Validator) addGeneralSuggestions(dockerfileContent string, result *BuildValidationResult) {
	// Initialize Details map if needed
	if result.Details == nil {
		result.Details = make(map[string]interface{})
	}

	suggestions := []string{}

	// Check for health check
	if !strings.Contains(dockerfileContent, "HEALTHCHECK") {
		suggestions = append(suggestions, "Consider adding a HEALTHCHECK instruction for better container monitoring")
	}

	// Check for .dockerignore reference
	suggestions = append(suggestions, "Ensure you have a .dockerignore file to exclude unnecessary files")

	// Check for multi-stage build opportunities
	if strings.Count(dockerfileContent, "FROM") == 1 && (strings.Contains(dockerfileContent, "npm install") || strings.Contains(dockerfileContent, "go build") || strings.Contains(dockerfileContent, "mvn package")) {
		suggestions = append(suggestions, "Consider using multi-stage builds to reduce final image size")
	}

	// Security suggestions
	suggestions = append(suggestions, "Review file permissions and avoid running as root user")
	suggestions = append(suggestions, "Use specific version tags for base images to ensure reproducible builds")

	// Store suggestions in Details map
	result.Details["suggestions"] = suggestions
}
