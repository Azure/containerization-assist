package build

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/rs/zerolog"
)

// SyntaxValidator handles Dockerfile syntax validation
type SyntaxValidator struct {
	logger   zerolog.Logger
	hadolint *docker.HadolintValidator
	basic    *docker.Validator
}

// NewSyntaxValidator creates a new syntax validator
func NewSyntaxValidator(logger zerolog.Logger) *SyntaxValidator {
	return &SyntaxValidator{
		logger:   logger.With().Str("component", "syntax_validator").Logger(),
		hadolint: docker.NewHadolintValidator(logger),
		basic:    docker.NewValidator(logger),
	}
}

// Validate performs syntax validation on Dockerfile content
func (v *SyntaxValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().
		Bool("use_hadolint", options.UseHadolint).
		Str("severity", options.Severity).
		Msg("Starting Dockerfile syntax validation")
	var coreResult *docker.ValidationResult
	var err error
	if options.UseHadolint {
		// Try Hadolint validation first
		coreResult, err = v.hadolint.ValidateWithHadolint(nil, content)
		if err != nil {
			v.logger.Warn().Err(err).Msg("Hadolint validation failed, falling back to basic validation")
			coreResult = v.basic.ValidateDockerfile(content)
		}
	} else {
		// Use basic validation
		coreResult = v.basic.ValidateDockerfile(content)
	}
	// Convert core result to our result type
	result := ConvertCoreResult(coreResult)
	// Apply severity filtering if specified
	if options.Severity != "" {
		v.filterBySeverity(result, options.Severity)
	}
	// Apply rule filtering if specified
	if len(options.IgnoreRules) > 0 {
		v.filterByRules(result, options.IgnoreRules)
	}
	// Add syntax-specific checks
	v.performSyntaxChecks(content, result)
	return result, nil
}

// Analyze provides syntax-specific analysis
func (v *SyntaxValidator) Analyze(lines []string, context ValidationContext) interface{} {
	analysis := SyntaxAnalysis{
		ValidInstructions:   0,
		InvalidInstructions: 0,
		DeprecatedUsage:     make([]string, 0),
		MultiStageInfo:      MultiStageInfo{Stages: make([]StageInfo, 0)},
	}
	currentStage := -1
	instructionCount := make(map[string]int)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		upper := strings.ToUpper(trimmed)
		instruction := strings.Fields(upper)[0]
		// Track valid instructions
		if isValidInstruction(instruction) {
			analysis.ValidInstructions++
			instructionCount[instruction]++
			// Track multi-stage builds
			if strings.HasPrefix(upper, "FROM") {
				currentStage++
				stageName := extractStageName(trimmed)
				if stageName == "" {
					stageName = fmt.Sprintf("stage_%d", currentStage)
				}
				analysis.MultiStageInfo.Stages = append(analysis.MultiStageInfo.Stages, StageInfo{
					Name:      stageName,
					StartLine: i + 1,
					BaseImage: extractBaseImage(trimmed),
				})
			}
		} else {
			analysis.InvalidInstructions++
		}
		// Check for deprecated usage
		if deprecated := checkDeprecatedSyntax(trimmed); deprecated != "" {
			analysis.DeprecatedUsage = append(analysis.DeprecatedUsage,
				fmt.Sprintf("Line %d: %s", i+1, deprecated))
		}
	}
	analysis.MultiStageInfo.TotalStages = len(analysis.MultiStageInfo.Stages)
	analysis.InstructionUsage = instructionCount
	return analysis
}

// SyntaxAnalysis contains syntax analysis results
type SyntaxAnalysis struct {
	ValidInstructions   int
	InvalidInstructions int
	DeprecatedUsage     []string
	MultiStageInfo      MultiStageInfo
	InstructionUsage    map[string]int
}

// MultiStageInfo contains multi-stage build information
type MultiStageInfo struct {
	TotalStages int
	Stages      []StageInfo
}

// StageInfo contains information about a build stage
type StageInfo struct {
	Name      string
	StartLine int
	BaseImage string
}

// performSyntaxChecks performs additional syntax validation
func (v *SyntaxValidator) performSyntaxChecks(content string, result *ValidationResult) {
	lines := strings.Split(content, "\n")
	// Check for missing FROM instruction
	hasFrom := false
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "FROM") {
			hasFrom = true
			break
		}
	}
	if !hasFrom {
		result.Errors = append(result.Errors, ValidationError{
			Line:    1,
			Column:  0,
			Message: "Missing FROM instruction",
			Rule:    "syntax",
		})
	}
	// Check for instruction case consistency
	v.checkInstructionCase(lines, result)
	// Check for line continuation issues
	v.checkLineContinuation(lines, result)
}

// filterBySeverity filters validation results by minimum severity (simplified)
func (v *SyntaxValidator) filterBySeverity(result *ValidationResult, minSeverity string) {
	// Since ValidationError no longer has Severity field, this is now a no-op
	// In a future version, severity could be determined by Rule field or other means
	v.logger.Debug().
		Str("min_severity", minSeverity).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Msg("Severity filtering currently not supported")
}

// filterByRules filters out issues matching ignored rules
func (v *SyntaxValidator) filterByRules(result *ValidationResult, ignoreRules []string) {
	// Create rule map for quick lookup
	ignoreMap := make(map[string]bool)
	for _, rule := range ignoreRules {
		ignoreMap[rule] = true
	}
	// Filter errors
	filteredErrors := make([]ValidationError, 0)
	for _, err := range result.Errors {
		if err.Rule == "" || !ignoreMap[err.Rule] {
			filteredErrors = append(filteredErrors, err)
		}
	}
	result.Errors = filteredErrors
	// Filter warnings
	filteredWarnings := make([]ValidationWarning, 0)
	for _, warn := range result.Warnings {
		if warn.Rule == "" || !ignoreMap[warn.Rule] {
			filteredWarnings = append(filteredWarnings, warn)
		}
	}
	result.Warnings = filteredWarnings
	// Update counts (TotalIssues field no longer exists, but we can log the count)
	v.logger.Debug().
		Int("total_issues", len(result.Errors)+len(result.Warnings)).
		Msg("Filtered validation results")
}

// checkInstructionCase checks for inconsistent instruction casing
func (v *SyntaxValidator) checkInstructionCase(lines []string, result *ValidationResult) {
	upperCount := 0
	lowerCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) > 0 {
			instruction := parts[0]
			if isValidInstruction(strings.ToUpper(instruction)) {
				if instruction == strings.ToUpper(instruction) {
					upperCount++
				} else if instruction == strings.ToLower(instruction) {
					lowerCount++
				}
			}
		}
	}
	if upperCount > 0 && lowerCount > 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    0,
			Column:  0,
			Message: "Inconsistent instruction casing detected. Use consistent casing for Dockerfile instructions (preferably uppercase)",
			Rule:    "style",
		})
	}
}

// checkLineContinuation checks for line continuation issues
func (v *SyntaxValidator) checkLineContinuation(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for backslash not at end of line
		if strings.Contains(trimmed, "\\") && !strings.HasSuffix(trimmed, "\\") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Backslash should be at the end of the line for continuation. Move backslash to the end of the line",
				Rule:    "syntax",
			})
		}
		// Check for trailing whitespace after backslash
		if strings.HasSuffix(line, "\\ ") || strings.HasSuffix(line, "\\\t") {
			result.Errors = append(result.Errors, ValidationError{
				Line:    i + 1,
				Column:  0,
				Message: "Trailing whitespace after line continuation backslash",
				Rule:    "syntax",
			})
		}
	}
}

// Helper functions
func getSeverityLevel(severity string) int {
	switch strings.ToLower(severity) {
	case "info":
		return 1
	case "warning":
		return 2
	case "error":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}
func isValidInstruction(instruction string) bool {
	validInstructions := []string{
		"FROM", "RUN", "CMD", "LABEL", "MAINTAINER", "EXPOSE",
		"ENV", "ADD", "COPY", "ENTRYPOINT", "VOLUME", "USER",
		"WORKDIR", "ARG", "ONBUILD", "STOPSIGNAL", "HEALTHCHECK",
		"SHELL",
	}
	for _, valid := range validInstructions {
		if instruction == valid {
			return true
		}
	}
	return false
}
func extractStageName(fromLine string) string {
	parts := strings.Fields(fromLine)
	for i, part := range parts {
		if strings.ToUpper(part) == "AS" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
func extractBaseImage(fromLine string) string {
	parts := strings.Fields(fromLine)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
func checkDeprecatedSyntax(line string) string {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "MAINTAINER") {
		return "MAINTAINER is deprecated, use LABEL maintainer=\"...\" instead"
	}
	// Add more deprecated syntax checks as needed
	return ""
}
