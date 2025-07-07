package validators

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// BuildValidator provides validation specific to build operations
type BuildValidator struct {
	unified *UnifiedValidator
}

// NewBuildValidator creates a new build validator
func NewBuildValidator() *BuildValidator {
	return &BuildValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateBuildPrerequisites validates that all prerequisites for building are met
func (bv *BuildValidator) ValidateBuildPrerequisites(ctx context.Context, dockerfilePath, buildContext string) error {
	vctx := NewValidateContext(ctx)

	// Validate Dockerfile exists
	if err := bv.unified.FileSystem.ValidateDockerfileExists(dockerfilePath); err != nil {
		vctx.AddError(err)
	}

	// Validate build context exists
	if err := bv.unified.FileSystem.ValidateDirectoryExists(buildContext); err != nil {
		vctx.AddError(err)
	}

	// Validate Docker is available
	if err := bv.unified.System.ValidateDockerAvailable(); err != nil {
		vctx.AddError(err)
	}

	return vctx.GetFirstError()
}

// ValidateBuildArgs validates build arguments
func (bv *BuildValidator) ValidateBuildArgs(sessionID, image, dockerfile, context string) error {
	if err := bv.unified.Input.ValidateSessionID(sessionID); err != nil {
		return err
	}

	if err := bv.unified.Input.ValidateImageName(image); err != nil {
		return err
	}

	if dockerfile != "" {
		if err := bv.unified.FileSystem.ValidateFileExists(dockerfile); err != nil {
			return err
		}
	}

	if context != "" {
		if err := bv.unified.FileSystem.ValidateDirectoryExists(context); err != nil {
			return err
		}
	}

	return nil
}

// ValidateImagePushPrerequisites validates prerequisites for pushing an image
func (bv *BuildValidator) ValidateImagePushPrerequisites(image, registry string) error {
	if err := bv.unified.Input.ValidateImageName(image); err != nil {
		return err
	}

	if registry != "" && !strings.Contains(image, registry) {
		return errors.Validationf("build", "image %s does not match registry %s", image, registry)
	}

	return nil
}

// GeneratePushTroubleshootingTips provides troubleshooting tips for push failures
func (bv *BuildValidator) GeneratePushTroubleshootingTips(err error, registryURL string) []string {
	tips := []string{}
	errorMsg := err.Error()

	if strings.Contains(errorMsg, "authentication required") ||
		strings.Contains(errorMsg, "unauthorized") {
		tips = append(tips, "Authentication failed. Please ensure you are logged in to the registry:")
		tips = append(tips, "  docker login "+registryURL)
		tips = append(tips, "Or configure your credentials using:")
		tips = append(tips, "  az acr login --name <registry-name>  # for Azure Container Registry")
	}

	if strings.Contains(errorMsg, "denied") ||
		strings.Contains(errorMsg, "permission") {
		tips = append(tips, "Permission denied. Please check:")
		tips = append(tips, "  1. You have push permissions to the repository")
		tips = append(tips, "  2. The repository exists and is accessible")
		tips = append(tips, "  3. Your authentication token has not expired")
	}

	if strings.Contains(errorMsg, "network") ||
		strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "connection") {
		tips = append(tips, "Network issues detected. Please check:")
		tips = append(tips, "  1. Your internet connection is stable")
		tips = append(tips, "  2. The registry URL is correct: "+registryURL)
		tips = append(tips, "  3. No firewall is blocking the connection")
		tips = append(tips, "  4. Try again in a few moments")
	}

	if strings.Contains(errorMsg, "name invalid") ||
		strings.Contains(errorMsg, "repository name") {
		tips = append(tips, "Repository name issues. Please check:")
		tips = append(tips, "  1. Repository name follows naming conventions")
		tips = append(tips, "  2. No uppercase letters or invalid characters")
		tips = append(tips, "  3. Format should be: registry.com/namespace/repository:tag")
	}

	if strings.Contains(errorMsg, "blob upload") ||
		strings.Contains(errorMsg, "layer") {
		tips = append(tips, "Layer upload issues. Try:")
		tips = append(tips, "  1. docker system prune  # to clean up space")
		tips = append(tips, "  2. Retry the push operation")
		tips = append(tips, "  3. Check available disk space")
	}

	if len(tips) == 0 {
		tips = append(tips, "General troubleshooting steps:")
		tips = append(tips, "  1. Ensure Docker is running and accessible")
		tips = append(tips, "  2. Verify the image exists locally: docker images")
		tips = append(tips, "  3. Check registry connectivity: docker info")
		tips = append(tips, "  4. Review Docker daemon logs for more details")
	}

	return tips
}

// SecurityValidator provides security-related validation for builds
type SecurityValidator struct {
	unified *UnifiedValidator
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateDockerfileSecurity performs basic security checks on Dockerfile
func (sv *SecurityValidator) ValidateDockerfileSecurity(dockerfilePath string) ([]string, error) {
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "security", "failed to read Dockerfile for security validation")
	}

	warnings := []string{}
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		lineNum := i + 1

		// Check for running as root
		if strings.Contains(strings.ToUpper(line), "USER ROOT") {
			warnings = append(warnings,
				fmt.Sprintf("Line %d: Running as root user is not recommended for security", lineNum))
		}

		// Check for exposed sensitive ports
		if strings.HasPrefix(strings.ToUpper(line), "EXPOSE") {
			if strings.Contains(line, "22") { // SSH
				warnings = append(warnings,
					fmt.Sprintf("Line %d: Exposing SSH port (22) may be a security risk", lineNum))
			}
			if strings.Contains(line, "3389") { // RDP
				warnings = append(warnings,
					fmt.Sprintf("Line %d: Exposing RDP port (3389) may be a security risk", lineNum))
			}
		}

		// Check for hardcoded secrets (basic patterns)
		secretPatterns := []string{"password=", "secret=", "key=", "token="}
		for _, pattern := range secretPatterns {
			if strings.Contains(strings.ToLower(line), pattern) {
				warnings = append(warnings,
					fmt.Sprintf("Line %d: Possible hardcoded secret detected", lineNum))
				break
			}
		}
	}

	return warnings, nil
}

// ValidateBuildContext checks build context for security issues
func (sv *SecurityValidator) ValidateBuildContext(contextPath string) ([]string, error) {
	warnings := []string{}

	// Check for .dockerignore file
	dockerignorePath := contextPath + "/.dockerignore"
	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		warnings = append(warnings, "No .dockerignore file found - this may include unintended files in build context")
	}

	// Check for sensitive files in build context
	sensitiveFiles := []string{".env", ".secret", "id_rsa", "id_dsa", "private.key"}
	for _, file := range sensitiveFiles {
		filepath := contextPath + "/" + file
		if _, err := os.Stat(filepath); err == nil {
			warnings = append(warnings, fmt.Sprintf("Sensitive file '%s' found in build context", file))
		}
	}

	return warnings, nil
}

// DockerfileValidator provides comprehensive Dockerfile validation
type DockerfileValidator struct {
	unified *UnifiedValidator
}

// NewDockerfileValidator creates a new Dockerfile validator
func NewDockerfileValidator() *DockerfileValidator {
	return &DockerfileValidator{
		unified: NewUnifiedValidator(),
	}
}

// ValidateDockerfile performs comprehensive validation of Dockerfile content
func (dv *DockerfileValidator) ValidateDockerfile(dockerfileContent string) *types.BuildValidationResult {
	result := types.NewBuildResult()
	if result.Metadata.Context == nil {
		result.Metadata.Context = make(map[string]string)
	}
	result.Metadata.ValidatorName = "unified-dockerfile-validator"
	result.Metadata.ValidatorVersion = "2.0.0"

	// Basic validation
	if strings.TrimSpace(dockerfileContent) == "" {
		result.Valid = false
		emptyError := &types.ValidationError{
			Code:     "DOCKERFILE_EMPTY",
			Message:  "Dockerfile is empty",
			Severity: types.SeverityHigh,
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
				dv.validateInstruction(currentInstructionName, completeInstruction, currentInstructionStart, result)

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
				dv.validateInstruction(instruction, trimmed, lineNum, result)
			}
		}
	}

	// Handle case where file ends with a line continuation
	if currentInstruction.Len() > 0 {
		completeInstruction := currentInstruction.String()
		dv.validateInstruction(currentInstructionName, completeInstruction, currentInstructionStart, result)
	}

	// Validate overall structure
	dv.validateStructure(instructions, result)

	// Add general suggestions
	dv.addGeneralSuggestions(dockerfileContent, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// CheckDockerInstallation verifies Docker is installed and accessible
func (dv *DockerfileValidator) CheckDockerInstallation() error {
	// Check if docker executable exists
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.NewError().
			Message("docker executable not found in PATH. Please install Docker").
			WithLocation().
			Build()
	}

	// Check if docker daemon is accessible
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		return errors.NewError().
			Message("docker daemon is not running or not accessible. Please start Docker").
			WithLocation().
			Build()
	}

	return nil
}

// validateInstruction validates individual Dockerfile instructions
func (dv *DockerfileValidator) validateInstruction(instruction, line string, lineNum int, result *types.BuildValidationResult) {
	switch instruction {
	case "FROM":
		dv.validateFromInstruction(line, lineNum, result)
	case "RUN":
		dv.validateRunInstruction(line, lineNum, result)
	case "COPY", "ADD":
		dv.validateCopyAddInstruction(instruction, line, lineNum, result)
	case "EXPOSE":
		dv.validateExposeInstruction(line, lineNum, result)
	case "USER":
		dv.validateUserInstruction(line, lineNum, result)
	case "WORKDIR":
		dv.validateWorkdirInstruction(line, lineNum, result)
	case "CMD", "ENTRYPOINT":
		dv.validateCmdEntrypointInstruction(instruction, line, lineNum, result)
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
			unknownError := &types.ValidationError{
				Code:     "UNKNOWN_INSTRUCTION",
				Message:  fmt.Sprintf("Unknown instruction: %s", instruction),
				Field:    "instruction",
				Severity: types.SeverityHigh,
				Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			}
			result.Errors = append(result.Errors, *unknownError)
		}
	}
}

// validateFromInstruction validates FROM instructions
func (dv *DockerfileValidator) validateFromInstruction(line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		fromError := &types.ValidationError{
			Code:     "FROM_MISSING_IMAGE",
			Message:  "FROM instruction requires an image name",
			Field:    "FROM",
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *fromError)
		return
	}

	imageName := parts[1]

	// Check for latest tag usage
	if strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":") {
		latestWarning := &types.ValidationWarning{
			Code:       "FROM_LATEST_TAG",
			Message:    "Using 'latest' tag or no tag is not recommended for production",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use specific version tags for reproducible builds",
		}
		result.Warnings = append(result.Warnings, *latestWarning)
	}
}

// validateRunInstruction validates RUN instructions
func (dv *DockerfileValidator) validateRunInstruction(line string, lineNum int, result *types.BuildValidationResult) {
	// Check for apt-get without update
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
		aptWarning := &types.ValidationWarning{
			Code:       "RUN_APT_UPDATE",
			Message:    "apt-get install should be preceded by apt-get update",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Combine 'apt-get update && apt-get install' in a single RUN instruction",
		}
		result.Warnings = append(result.Warnings, *aptWarning)
	}

	// Check for package manager cache cleanup
	if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
		cacheWarning := &types.ValidationWarning{
			Code:       "RUN_CACHE_CLEANUP",
			Message:    "Consider cleaning package manager cache to reduce image size",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Add '&& rm -rf /var/lib/apt/lists/*' to clean up after apt-get",
		}
		result.Warnings = append(result.Warnings, *cacheWarning)
	}
}

// validateCopyAddInstruction validates COPY and ADD instructions
func (dv *DockerfileValidator) validateCopyAddInstruction(instruction, line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		copyError := &types.ValidationError{
			Code:     fmt.Sprintf("%s_MISSING_ARGS", instruction),
			Message:  fmt.Sprintf("%s instruction requires source and destination", instruction),
			Field:    instruction,
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *copyError)
		return
	}

	// Warn about ADD vs COPY
	if instruction == "ADD" && !strings.Contains(line, "http") && !strings.HasSuffix(parts[1], ".tar") {
		addWarning := &types.ValidationWarning{
			Code:       "ADD_VS_COPY",
			Message:    "COPY is preferred over ADD for simple file copying",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use COPY instead of ADD unless you need URL download or tar extraction",
		}
		result.Warnings = append(result.Warnings, *addWarning)
	}
}

// validateExposeInstruction validates EXPOSE instructions
func (dv *DockerfileValidator) validateExposeInstruction(line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		exposeError := &types.ValidationError{
			Code:     "EXPOSE_MISSING_PORT",
			Message:  "EXPOSE instruction requires a port number",
			Field:    "EXPOSE",
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *exposeError)
		return
	}

	// Basic port validation
	portRegex := regexp.MustCompile(`^\d+(/tcp|/udp)?$`)
	for _, port := range parts[1:] {
		if !portRegex.MatchString(port) {
			portError := &types.ValidationError{
				Code:     "EXPOSE_INVALID_PORT",
				Message:  fmt.Sprintf("Invalid port format: %s", port),
				Field:    "EXPOSE",
				Severity: types.SeverityHigh,
				Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum), "port": port},
			}
			result.Errors = append(result.Errors, *portError)
		}
	}
}

// validateUserInstruction validates USER instructions
func (dv *DockerfileValidator) validateUserInstruction(line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		userError := &types.ValidationError{
			Code:     "USER_MISSING_NAME",
			Message:  "USER instruction requires a username or UID",
			Field:    "USER",
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *userError)
		return
	}

	user := parts[1]
	if user == "root" {
		rootWarning := &types.ValidationWarning{
			Code:       "USER_ROOT_SECURITY",
			Message:    "Running as root user is a security risk",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Create and use a non-root user for better security",
		}
		result.Warnings = append(result.Warnings, *rootWarning)
	}
}

// validateWorkdirInstruction validates WORKDIR instructions
func (dv *DockerfileValidator) validateWorkdirInstruction(line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		workdirError := &types.ValidationError{
			Code:     "WORKDIR_MISSING_PATH",
			Message:  "WORKDIR instruction requires a directory path",
			Field:    "WORKDIR",
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *workdirError)
		return
	}

	workdir := parts[1]
	if !strings.HasPrefix(workdir, "/") {
		pathWarning := &types.ValidationWarning{
			Code:       "WORKDIR_RELATIVE_PATH",
			Message:    "WORKDIR should use absolute paths",
			Context:    map[string]string{"line": fmt.Sprintf("%d", lineNum)},
			Suggestion: "Use absolute paths starting with '/' for WORKDIR",
		}
		result.Warnings = append(result.Warnings, *pathWarning)
	}
}

// validateCmdEntrypointInstruction validates CMD and ENTRYPOINT instructions
func (dv *DockerfileValidator) validateCmdEntrypointInstruction(instruction, line string, lineNum int, result *types.BuildValidationResult) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		cmdError := &types.ValidationError{
			Code:     fmt.Sprintf("%s_MISSING_COMMAND", instruction),
			Message:  fmt.Sprintf("%s instruction requires a command", instruction),
			Field:    instruction,
			Severity: types.SeverityHigh,
			Context:  map[string]string{"line": fmt.Sprintf("%d", lineNum)},
		}
		result.Errors = append(result.Errors, *cmdError)
	}
}

// validateStructure validates overall Dockerfile structure
func (dv *DockerfileValidator) validateStructure(instructions []string, result *types.BuildValidationResult) {
	if len(instructions) == 0 {
		emptyError := &types.ValidationError{
			Code:     "DOCKERFILE_NO_INSTRUCTIONS",
			Message:  "Dockerfile contains no instructions",
			Field:    "structure",
			Severity: types.SeverityHigh,
			Context:  map[string]string{},
		}
		result.Errors = append(result.Errors, *emptyError)
		return
	}

	// Must start with FROM
	if instructions[0] != "FROM" {
		fromError := &types.ValidationError{
			Code:     "DOCKERFILE_NO_FROM",
			Message:  "Dockerfile must start with FROM instruction",
			Field:    "structure",
			Severity: types.SeverityHigh,
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
		cmdWarning := &types.ValidationWarning{
			Code:       "MULTIPLE_CMD",
			Message:    "Multiple CMD instructions found, only the last one will be effective",
			Context:    map[string]string{"count": fmt.Sprintf("%d", cmdCount)},
			Suggestion: "Use only one CMD instruction",
		}
		result.Warnings = append(result.Warnings, *cmdWarning)
	}

	if entrypointCount > 1 {
		entrypointWarning := &types.ValidationWarning{
			Code:       "MULTIPLE_ENTRYPOINT",
			Message:    "Multiple ENTRYPOINT instructions found, only the last one will be effective",
			Context:    map[string]string{"count": fmt.Sprintf("%d", entrypointCount)},
			Suggestion: "Use only one ENTRYPOINT instruction",
		}
		result.Warnings = append(result.Warnings, *entrypointWarning)
	}

	// Warn about ENTRYPOINT + CMD combination (informational) - only if exactly one of each
	if entrypointCount == 1 && cmdCount == 1 {
		comboWarning := &types.ValidationWarning{
			Code:       "ENTRYPOINT_CMD_COMBO",
			Message:    "Using both ENTRYPOINT and CMD - CMD will be passed as arguments to ENTRYPOINT",
			Context:    map[string]string{},
			Suggestion: "Ensure this is the intended behavior",
		}
		result.Warnings = append(result.Warnings, *comboWarning)
	}
}

// addGeneralSuggestions adds general best practice suggestions
func (dv *DockerfileValidator) addGeneralSuggestions(dockerfileContent string, result *types.BuildValidationResult) {
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
