package validators

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/utils"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// DockerfileValidator validates Dockerfile content and structure
type DockerfileValidator struct {
	*BaseValidatorImpl
	securityChecks bool
	syntaxChecks   bool
	bestPractices  bool
}

// NewDockerfileValidator creates a new Dockerfile validator
func NewDockerfileValidator() *DockerfileValidator {
	return &DockerfileValidator{
		BaseValidatorImpl: NewBaseValidator("dockerfile", "1.0.0", []string{"dockerfile", "docker", "string"}),
		securityChecks:    true,
		syntaxChecks:      true,
		bestPractices:     true,
	}
}

// WithSecurityChecks enables or disables security checks
func (d *DockerfileValidator) WithSecurityChecks(enabled bool) *DockerfileValidator {
	d.securityChecks = enabled
	return d
}

// WithSyntaxChecks enables or disables syntax checks
func (d *DockerfileValidator) WithSyntaxChecks(enabled bool) *DockerfileValidator {
	d.syntaxChecks = enabled
	return d
}

// WithBestPractices enables or disables best practice checks
func (d *DockerfileValidator) WithBestPractices(enabled bool) *DockerfileValidator {
	d.bestPractices = enabled
	return d
}

// Validate validates Dockerfile content
func (d *DockerfileValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()

	result := &core.NonGenericResult{
		Valid:    true,
		Errors:   make([]*core.Error, 0),
		Warnings: make([]*core.Warning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    d.Name,
			ValidatorVersion: d.Version,
			RulesApplied:     make([]string, 0),
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Convert data to string
	content, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_DATA_TYPE",
			"Expected string content for Dockerfile validation",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		result.Duration = time.Since(startTime)
		return result
	}

	if strings.TrimSpace(content) == "" {
		result.AddError(core.NewError(
			"EMPTY_DOCKERFILE",
			"Dockerfile content cannot be empty",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		result.Duration = time.Since(startTime)
		return result
	}

	lines := strings.Split(content, "\n")

	// Perform syntax checks
	if d.syntaxChecks && !options.ShouldSkipRule("syntax") {
		d.validateSyntax(lines, result)
		result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "syntax")
	}

	// Perform security checks
	if d.securityChecks && !options.ShouldSkipRule("security") {
		d.validateSecurity(lines, result)
		result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "security")
	}

	// Perform best practice checks
	if d.bestPractices && !options.ShouldSkipRule("best-practices") {
		d.validateBestPractices(lines, result)
		result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "best-practices")
	}

	result.Duration = time.Since(startTime)
	return result
}

// validateSyntax performs syntax validation
func (d *DockerfileValidator) validateSyntax(lines []string, result *core.NonGenericResult) {
	hasFrom := false
	validInstructions := map[string]bool{
		"FROM": true, "RUN": true, "CMD": true, "LABEL": true, "EXPOSE": true,
		"ENV": true, "ADD": true, "COPY": true, "ENTRYPOINT": true, "VOLUME": true,
		"USER": true, "WORKDIR": true, "ARG": true, "ONBUILD": true, "STOPSIGNAL": true,
		"HEALTHCHECK": true, "SHELL": true,
	}

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for valid instruction
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			continue
		}

		instruction := strings.ToUpper(parts[0])

		if !validInstructions[instruction] {
			result.AddLineError(lineNum,
				"Invalid or unrecognized instruction: "+instruction,
				"syntax-invalid-instruction")
			continue
		}

		// Check for FROM instruction
		if instruction == "FROM" {
			hasFrom = true
			if len(parts) < 2 {
				result.AddLineError(lineNum,
					"FROM instruction requires a base image",
					"syntax-from-missing-image")
			}
		}

		// Check for instruction-specific syntax
		d.validateInstructionSyntax(instruction, parts, lineNum, result)
	}

	if !hasFrom {
		err := core.NewError(
			"MISSING_FROM",
			"Dockerfile must start with a FROM instruction",
			core.ErrTypeSyntax,
			core.SeverityHigh,
		)
		err.Suggestions = append(err.Suggestions, "Add 'FROM <base-image>' as the first instruction")
		result.AddError(err)
	}
}

// validateInstructionSyntax validates specific instruction syntax
func (d *DockerfileValidator) validateInstructionSyntax(instruction string, parts []string, lineNum int, result *core.NonGenericResult) {
	switch instruction {
	case "EXPOSE":
		if len(parts) < 2 {
			result.AddLineError(lineNum,
				"EXPOSE instruction requires at least one port",
				"syntax-expose-missing-port")
		}
		for i := 1; i < len(parts); i++ {
			port := parts[i]
			if !regexp.MustCompile(`^\d+(/tcp|/udp)?$`).MatchString(port) {
				result.AddLineError(lineNum,
					"Invalid port format: "+port+" (expected: number[/protocol])",
					"syntax-expose-invalid-port")
			}
		}

	case "ENV":
		if len(parts) < 3 {
			result.AddLineError(lineNum,
				"ENV instruction requires variable name and value",
				"syntax-env-missing-parts")
		}

	case "COPY", "ADD":
		if len(parts) < 3 {
			result.AddLineError(lineNum,
				instruction+" instruction requires source and destination",
				"syntax-"+strings.ToLower(instruction)+"-missing-parts")
		}

	case "USER":
		if len(parts) < 2 {
			result.AddLineError(lineNum,
				"USER instruction requires a username or UID",
				"syntax-user-missing-value")
		}

	case "WORKDIR":
		if len(parts) < 2 {
			result.AddLineError(lineNum,
				"WORKDIR instruction requires a directory path",
				"syntax-workdir-missing-path")
		}
	}
}

// validateSecurity performs security validation
func (d *DockerfileValidator) validateSecurity(lines []string, result *core.NonGenericResult) {
	lastUser := ""
	hasHealthcheck := false

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for root user
		if strings.HasPrefix(upper, "USER") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				user := parts[1]
				lastUser = user
				if utils.IsRootUser(user) {
					result.AddLineError(lineNum,
						"Running as root user is a security risk",
						"security-root-user")
					result.AddSuggestion("Create and use a non-root user")
				}
			}
		}

		// Check for HEALTHCHECK
		if strings.HasPrefix(upper, "HEALTHCHECK") {
			hasHealthcheck = true
		}

		// Perform comprehensive security checks using shared utilities
		securityResults := utils.PerformDockerfileSecurityChecks(trimmed, lineNum)
		for _, secResult := range securityResults {
			if secResult.HasIssue {
				result.AddLineError(secResult.LineNumber, secResult.Message, secResult.Code)
				result.AddSuggestion(secResult.Suggestion)
			}
		}
	}

	// Check if container runs as root (default)
	if utils.IsRootUser(lastUser) {
		err := core.NewError(
			"DEFAULT_ROOT_USER",
			"Container will run as root user by default",
			core.ErrTypeSecurity,
			core.SeverityMedium,
		)
		err.Suggestions = append(err.Suggestions, "Add USER instruction with non-root user")
		result.AddError(err)
	}

	// Check for missing healthcheck
	if !hasHealthcheck {
		warning := core.NewWarning(
			"MISSING_HEALTHCHECK",
			"No HEALTHCHECK instruction found",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Add HEALTHCHECK instruction to monitor container health")
		result.AddWarning(warning)
	}
}

// validateBestPractices performs best practice validation
func (d *DockerfileValidator) validateBestPractices(lines []string, result *core.NonGenericResult) {
	hasEntrypoint := false
	hasCmd := false

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check FROM instruction
		if strings.HasPrefix(upper, "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				image := parts[1]
				if utils.HasLatestTag(image) {
					warning := core.NewWarning(
						"FROM_LATEST_TAG",
						"Using 'latest' tag or no tag is not recommended",
					)
					warning.Error.WithLine(lineNum).
						WithSuggestion("Use specific version tags for reproducible builds")
					result.AddWarning(warning)
				}

				// Check for minimal base images
				if d.isMinimalBaseImage(image) {
					result.AddSuggestion("Consider using even smaller base images like scratch or distroless")
				} else if !d.isRecommendedBaseImage(image) {
					warning := core.NewWarning(
						"NON_MINIMAL_BASE",
						"Consider using minimal base image for smaller attack surface",
					)
					warning.Error.WithLine(lineNum).
						WithSuggestion("Use alpine, distroless, or scratch base images")
					result.AddWarning(warning)
				}
			}
		}

		// Check for ADD vs COPY
		if strings.HasPrefix(upper, "ADD") {
			warning := core.NewWarning(
				"USE_COPY_INSTEAD_ADD",
				"COPY is preferred over ADD unless you need ADD's additional features",
			)
			warning.Error.WithLine(lineNum).
				WithSuggestion("Use COPY for simple file copying")
			result.AddWarning(warning)
		}

		// Check for ENTRYPOINT and CMD
		if strings.HasPrefix(upper, "ENTRYPOINT") {
			hasEntrypoint = true
		}
		if strings.HasPrefix(upper, "CMD") {
			hasCmd = true
		}

		// Check for package manager cache cleanup
		if d.needsCacheCleanup(trimmed) {
			warning := core.NewWarning(
				"MISSING_CACHE_CLEANUP",
				"Package manager cache should be cleaned to reduce image size",
			)
			warning.Error.WithLine(lineNum).
				WithSuggestion("Add package manager cache cleanup commands")
			result.AddWarning(warning)
		}
	}

	// Check for missing execution instructions
	if !hasEntrypoint && !hasCmd {
		warning := core.NewWarning(
			"MISSING_EXECUTION_INSTRUCTION",
			"No ENTRYPOINT or CMD instruction found",
		)
		warning.Error.Suggestions = append(warning.Error.Suggestions, "Add ENTRYPOINT or CMD instruction to define container behavior")
		result.AddWarning(warning)
	}
}

// Helper methods for security and best practice checks

func (d *DockerfileValidator) containsHardcodedSecrets(line string) bool {
	secretPatterns := []string{
		`(?i)(password|pwd|passwd)\s*=\s*['"][^'"]+['"]`,
		`(?i)(api[_-]?key|apikey)\s*=\s*['"][^'"]+['"]`,
		`(?i)(secret|token)\s*=\s*['"][^'"]+['"]`,
		`(?i)(private[_-]?key)\s*=\s*['"][^'"]+['"]`,
	}

	for _, pattern := range secretPatterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			// Exclude if it's using environment variables or build args
			if !strings.Contains(line, "${") && !strings.Contains(line, "$(") {
				return true
			}
		}
	}
	return false
}

func (d *DockerfileValidator) containsInsecureDownload(line string) bool {
	if !strings.Contains(strings.ToLower(line), "http://") {
		return false
	}

	// Check if it's in a RUN, ADD, or COPY instruction that downloads
	upper := strings.ToUpper(strings.TrimSpace(line))
	return strings.HasPrefix(upper, "RUN") ||
		strings.HasPrefix(upper, "ADD") ||
		(strings.HasPrefix(upper, "COPY") && strings.Contains(line, "http://"))
}

func (d *DockerfileValidator) installsSSH(line string) bool {
	sshPackages := []string{"openssh-server", "ssh-server", "sshd"}
	lower := strings.ToLower(line)

	if !strings.Contains(lower, "install") && !strings.Contains(lower, "add") {
		return false
	}

	for _, pkg := range sshPackages {
		if strings.Contains(lower, pkg) {
			return true
		}
	}
	return false
}

func (d *DockerfileValidator) installsSudo(line string) bool {
	lower := strings.ToLower(line)
	return (strings.Contains(lower, "install") || strings.Contains(lower, "add")) &&
		strings.Contains(lower, "sudo")
}

func (d *DockerfileValidator) isMinimalBaseImage(image string) bool {
	minimalImages := []string{"alpine", "scratch", "distroless", "busybox"}
	lower := strings.ToLower(image)

	for _, minimal := range minimalImages {
		if strings.Contains(lower, minimal) {
			return true
		}
	}
	return false
}

func (d *DockerfileValidator) isRecommendedBaseImage(image string) bool {
	recommendedPatterns := []string{"alpine", "slim", "minimal", "distroless", "scratch"}
	lower := strings.ToLower(image)

	for _, pattern := range recommendedPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func (d *DockerfileValidator) needsCacheCleanup(line string) bool {
	lower := strings.ToLower(line)

	// Check for package installation without cleanup
	packageManagers := []struct {
		install string
		cleanup string
	}{
		{"apt-get install", "apt-get clean"},
		{"yum install", "yum clean"},
		{"dnf install", "dnf clean"},
		{"apk add", "rm -rf /var/cache/apk"},
	}

	for _, pm := range packageManagers {
		if strings.Contains(lower, pm.install) && !strings.Contains(lower, pm.cleanup) {
			return true
		}
	}
	return false
}

// DockerImageValidator validates Docker image names
type DockerImageValidator struct {
	*BaseValidatorImpl
}

// NewDockerImageValidator creates a new Docker image name validator
func NewDockerImageValidator() *DockerImageValidator {
	return &DockerImageValidator{
		BaseValidatorImpl: NewBaseValidator("docker-image", "1.0.0", []string{"string", "docker-image"}),
	}
}

// Validate validates Docker image name
func (d *DockerImageValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := d.BaseValidatorImpl.Validate(ctx, data, options)

	imageName, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_DATA_TYPE",
			"Expected string for Docker image name validation",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return result
	}

	if err := utils.ValidateDockerImageName(imageName, "image"); err != nil {
		result.AddError(err)
	}

	return result
}

// ============================================================================
// Type-Safe Validation Methods (applying Kubernetes validator pattern)
// ============================================================================

// DockerfileData represents type-safe Dockerfile validation data
type DockerfileData struct {
	Content      string                  `json:"content"`
	Path         string                  `json:"path,omitempty"`
	BaseImage    string                  `json:"base_image,omitempty"`
	Instructions []DockerfileInstruction `json:"instructions,omitempty"`
	Raw          interface{}             `json:"raw,omitempty"` // Keep raw data for backward compatibility
}

// DockerfileInstruction represents a single Dockerfile instruction
type DockerfileInstruction struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Line    int      `json:"line"`
}

// ValidateTyped validates Dockerfile data with type safety
func (d *DockerfileValidator) ValidateTyped(ctx context.Context, dockerfileData DockerfileData, options *core.ValidationOptions) *core.NonGenericResult {
	startTime := time.Now()
	result := d.BaseValidatorImpl.Validate(ctx, dockerfileData, options)

	// Use the typed data for validation
	d.validateDockerfileData(dockerfileData, result, options)

	result.Duration = time.Since(startTime)
	return result
}

// ConvertToDockerfileData converts interface{} to typed DockerfileData
func ConvertToDockerfileData(data interface{}) (DockerfileData, error) {
	dockerfileData := DockerfileData{
		Raw: data, // Keep raw data for backward compatibility
	}

	switch v := data.(type) {
	case string:
		dockerfileData.Content = v
		dockerfileData.Instructions = parseDockerfileInstructions(v)
	case map[string]interface{}:
		if content, ok := v["content"].(string); ok {
			dockerfileData.Content = content
			dockerfileData.Instructions = parseDockerfileInstructions(content)
		}
		if path, ok := v["path"].(string); ok {
			dockerfileData.Path = path
		}
	case DockerfileData:
		return v, nil
	default:
		return dockerfileData, errors.NewError().Messagef("unsupported data type for Dockerfile validation: %T", data).Build()
	}

	return dockerfileData, nil
}

// parseDockerfileInstructions parses Dockerfile content into typed instructions
func parseDockerfileInstructions(content string) []DockerfileInstruction {
	var instructions []DockerfileInstruction
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) > 0 {
			instruction := DockerfileInstruction{
				Command: strings.ToUpper(parts[0]),
				Args:    parts[1:],
				Line:    i + 1,
			}
			instructions = append(instructions, instruction)
		}
	}

	return instructions
}

// validateDockerfileData validates typed Dockerfile data
func (d *DockerfileValidator) validateDockerfileData(data DockerfileData, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Validate using the existing string-based validation by parsing lines
	if data.Content != "" {
		lines := strings.Split(data.Content, "\n")

		// Perform syntax checks
		if d.syntaxChecks && !options.ShouldSkipRule("syntax") {
			d.validateSyntax(lines, result)
			result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "syntax")
		}

		// Perform security checks
		if d.securityChecks && !options.ShouldSkipRule("security") {
			d.validateSecurity(lines, result)
			result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "security")
		}

		// Perform best practice checks
		if d.bestPractices && !options.ShouldSkipRule("best-practices") {
			d.validateBestPractices(lines, result)
			result.Metadata.RulesApplied = append(result.Metadata.RulesApplied, "best-practices")
		}
	}

	// Additional typed validations can be added here
	if len(data.Instructions) == 0 && data.Content != "" {
		result.AddError(core.NewError(
			"DOCKERFILE_NO_INSTRUCTIONS",
			"Dockerfile has no valid instructions",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
	}
}
