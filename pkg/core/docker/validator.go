package docker

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// Validator provides mechanical Dockerfile validation operations
type Validator struct {
	logger zerolog.Logger
}

// NewValidator creates a new Dockerfile validator
func NewValidator(logger zerolog.Logger) *Validator {
	return &Validator{
		logger: logger.With().Str("component", "dockerfile_validator").Logger(),
	}
}

// ValidationResult contains the result of Dockerfile validation
type ValidationResult struct {
	Valid       bool                   `json:"valid"`
	Errors      []ValidationError      `json:"errors"`
	Warnings    []ValidationWarning    `json:"warnings"`
	Suggestions []string               `json:"suggestions"`
	Context     map[string]interface{} `json:"context"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Type        string `json:"type"` // "syntax", "instruction", "security", "best_practice"
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	Message     string `json:"message"`
	Instruction string `json:"instruction,omitempty"`
	Severity    string `json:"severity"` // "error", "warning", "info"
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Type        string `json:"type"`
	Line        int    `json:"line"`
	Message     string `json:"message"`
	Instruction string `json:"instruction,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ValidateDockerfile performs comprehensive validation of Dockerfile content
func (v *Validator) ValidateDockerfile(dockerfileContent string) *ValidationResult {
	result := &ValidationResult{
		Valid:       true,
		Errors:      make([]ValidationError, 0),
		Warnings:    make([]ValidationWarning, 0),
		Suggestions: make([]string, 0),
		Context:     make(map[string]interface{}),
	}

	v.logger.Debug().Msg("Starting Dockerfile validation")

	// Basic validation
	if strings.TrimSpace(dockerfileContent) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "content",
			Line:     0,
			Message:  "Dockerfile is empty",
			Severity: "error",
		})
		return result
	}

	lines := strings.Split(dockerfileContent, "\n")
	result.Context["line_count"] = len(lines)
	result.Context["total_size"] = len(dockerfileContent)

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

	v.logger.Debug().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Dockerfile validation completed")

	return result
}

// CheckDockerInstallation verifies Docker is installed and accessible
func (v *Validator) CheckDockerInstallation() error {
	// Check if docker executable exists
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker")
	}

	// Check if docker daemon is accessible
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running or not accessible. Please start Docker")
	}

	return nil
}

// validateInstruction validates individual Dockerfile instructions
func (v *Validator) validateInstruction(instruction, line string, lineNum int, result *ValidationResult) {
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
			result.Errors = append(result.Errors, ValidationError{
				Type:        "instruction",
				Line:        lineNum,
				Message:     fmt.Sprintf("Unknown instruction: %s", instruction),
				Instruction: instruction,
				Severity:    "error",
			})
		}
	}
}

// validateFromInstruction validates FROM instructions
func (v *Validator) validateFromInstruction(line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     "FROM instruction requires an image name",
			Instruction: "FROM",
			Severity:    "error",
		})
		return
	}

	imageName := parts[1]

	// Check for latest tag usage
	if strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "best_practice",
			Line:        lineNum,
			Message:     "Using 'latest' tag or no tag is not recommended for production",
			Instruction: "FROM",
			Suggestion:  "Use specific version tags for reproducible builds",
		})
	}
}

// validateRunInstruction validates RUN instructions
func (v *Validator) validateRunInstruction(line string, lineNum int, result *ValidationResult) {
	// Check for apt-get without update
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "best_practice",
			Line:        lineNum,
			Message:     "apt-get install should be preceded by apt-get update",
			Instruction: "RUN",
			Suggestion:  "Combine 'apt-get update && apt-get install' in a single RUN instruction",
		})
	}

	// Check for package manager cache cleanup
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "best_practice",
			Line:        lineNum,
			Message:     "Consider cleaning package manager cache to reduce image size",
			Instruction: "RUN",
			Suggestion:  "Add '&& rm -rf /var/lib/apt/lists/*' to clean up after apt-get",
		})
	}
}

// validateCopyAddInstruction validates COPY and ADD instructions
func (v *Validator) validateCopyAddInstruction(instruction, line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     fmt.Sprintf("%s instruction requires source and destination", instruction),
			Instruction: instruction,
			Severity:    "error",
		})
		return
	}

	// Warn about ADD vs COPY
	if instruction == "ADD" && !strings.Contains(line, "http") && !strings.HasSuffix(parts[1], ".tar") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "best_practice",
			Line:        lineNum,
			Message:     "COPY is preferred over ADD for simple file copying",
			Instruction: "ADD",
			Suggestion:  "Use COPY instead of ADD unless you need URL download or tar extraction",
		})
	}
}

// validateExposeInstruction validates EXPOSE instructions
func (v *Validator) validateExposeInstruction(line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     "EXPOSE instruction requires a port number",
			Instruction: "EXPOSE",
			Severity:    "error",
		})
		return
	}

	// Basic port validation
	portRegex := regexp.MustCompile(`^\d+(/tcp|/udp)?$`)
	for _, port := range parts[1:] {
		if !portRegex.MatchString(port) {
			result.Errors = append(result.Errors, ValidationError{
				Type:        "syntax",
				Line:        lineNum,
				Message:     fmt.Sprintf("Invalid port format: %s", port),
				Instruction: "EXPOSE",
				Severity:    "error",
			})
		}
	}
}

// validateUserInstruction validates USER instructions
func (v *Validator) validateUserInstruction(line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     "USER instruction requires a username or UID",
			Instruction: "USER",
			Severity:    "error",
		})
		return
	}

	user := parts[1]
	if user == "root" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "security",
			Line:        lineNum,
			Message:     "Running as root user is a security risk",
			Instruction: "USER",
			Suggestion:  "Create and use a non-root user for better security",
		})
	}
}

// validateWorkdirInstruction validates WORKDIR instructions
func (v *Validator) validateWorkdirInstruction(line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     "WORKDIR instruction requires a directory path",
			Instruction: "WORKDIR",
			Severity:    "error",
		})
		return
	}

	workdir := parts[1]
	if !strings.HasPrefix(workdir, "/") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:        "best_practice",
			Line:        lineNum,
			Message:     "WORKDIR should use absolute paths",
			Instruction: "WORKDIR",
			Suggestion:  "Use absolute paths starting with '/' for WORKDIR",
		})
	}
}

// validateCmdEntrypointInstruction validates CMD and ENTRYPOINT instructions
func (v *Validator) validateCmdEntrypointInstruction(instruction, line string, lineNum int, result *ValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Type:        "syntax",
			Line:        lineNum,
			Message:     fmt.Sprintf("%s instruction requires a command", instruction),
			Instruction: instruction,
			Severity:    "error",
		})
	}
}

// validateStructure validates overall Dockerfile structure
func (v *Validator) validateStructure(instructions []string, result *ValidationResult) {
	if len(instructions) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Type:     "structure",
			Line:     0,
			Message:  "Dockerfile contains no instructions",
			Severity: "error",
		})
		return
	}

	// Must start with FROM
	if instructions[0] != "FROM" {
		result.Errors = append(result.Errors, ValidationError{
			Type:     "structure",
			Line:     1,
			Message:  "Dockerfile must start with FROM instruction",
			Severity: "error",
		})
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
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:       "structure",
			Line:       0,
			Message:    "Multiple CMD instructions found, only the last one will be effective",
			Suggestion: "Use only one CMD instruction",
		})
	}

	if entrypointCount > 1 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:       "structure",
			Line:       0,
			Message:    "Multiple ENTRYPOINT instructions found, only the last one will be effective",
			Suggestion: "Use only one ENTRYPOINT instruction",
		})
	}

	// Warn about ENTRYPOINT + CMD combination (informational) - only if exactly one of each
	if entrypointCount == 1 && cmdCount == 1 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:       "structure",
			Line:       0,
			Message:    "Using both ENTRYPOINT and CMD - CMD will be passed as arguments to ENTRYPOINT",
			Suggestion: "Ensure this is the intended behavior",
		})
	}
}

// addGeneralSuggestions adds general best practice suggestions
func (v *Validator) addGeneralSuggestions(dockerfileContent string, result *ValidationResult) {
	// Check for health check
	if !strings.Contains(dockerfileContent, "HEALTHCHECK") {
		result.Suggestions = append(result.Suggestions, "Consider adding a HEALTHCHECK instruction for better container monitoring")
	}

	// Check for .dockerignore reference
	result.Suggestions = append(result.Suggestions, "Ensure you have a .dockerignore file to exclude unnecessary files")

	// Check for multi-stage build opportunities
	if strings.Count(dockerfileContent, "FROM") == 1 && (strings.Contains(dockerfileContent, "npm install") || strings.Contains(dockerfileContent, "go build") || strings.Contains(dockerfileContent, "mvn package")) {
		result.Suggestions = append(result.Suggestions, "Consider using multi-stage builds to reduce final image size")
	}

	// Security suggestions
	result.Suggestions = append(result.Suggestions, "Review file permissions and avoid running as root user")
	result.Suggestions = append(result.Suggestions, "Use specific version tags for base images to ensure reproducible builds")
}
