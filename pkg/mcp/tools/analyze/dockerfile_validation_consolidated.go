package analyze

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// Register consolidated Dockerfile validation tools
func init() {
	core.RegisterTool("validate_dockerfile", func() api.Tool {
		return &ConsolidatedValidateDockerfileTool{}
	})
	core.RegisterTool("validate_dockerfile_analysis", func() api.Tool {
		return &DockerfileAnalysisWrapper{}
	})
}

// Unified input schema for Dockerfile validation
type ValidateDockerfileInput struct {
	// Core parameters
	SessionID         string `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	DockerfilePath    string `json:"dockerfile_path,omitempty" validate:"omitempty,filepath" description:"Path to Dockerfile"`
	DockerfileContent string `json:"dockerfile_content,omitempty" description:"Raw Dockerfile content"`

	// Validation options
	Severity           string `json:"severity,omitempty" validate:"omitempty,oneof=info warning error" description:"Minimum severity level (info, warning, error)"`
	CheckSecurity      bool   `json:"check_security,omitempty" description:"Include security checks"`
	CheckBestPractices bool   `json:"check_best_practices,omitempty" description:"Include best practices checks"`
	RuleSet            string `json:"rule_set,omitempty" validate:"omitempty,oneof=basic strict enterprise" description:"Validation rule set"`

	// Advanced options
	UseHadolint bool     `json:"use_hadolint,omitempty" description:"Use hadolint for validation"`
	CustomRules []string `json:"custom_rules,omitempty" description:"Custom validation rules"`
	DryRun      bool     `json:"dry_run,omitempty" description:"Preview validation without executing"`

	// Output options
	IncludeAnalysis bool `json:"include_analysis,omitempty" description:"Include detailed analysis"`
	IncludeFixes    bool `json:"include_fixes,omitempty" description:"Include suggested fixes"`
}

// Validate implements validation using tag-based validation
func (v ValidateDockerfileInput) Validate() error {
	// Must provide either path or content
	if v.DockerfilePath == "" && v.DockerfileContent == "" {
		return errors.NewError().Message("either dockerfile_path or dockerfile_content is required").Build()
	}
	return validation.ValidateTaggedStruct(v)
}

// Unified output schema for Dockerfile validation
type ValidateDockerfileOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`

	// Validation results
	IsValid         bool `json:"is_valid"`
	ValidationScore int  `json:"validation_score"` // 0-100
	TotalIssues     int  `json:"total_issues"`
	CriticalIssues  int  `json:"critical_issues"`

	// Issue details
	Errors         []ValidationIssue   `json:"errors,omitempty"`
	Warnings       []ValidationIssue   `json:"warnings,omitempty"`
	InfoMessages   []ValidationIssue   `json:"info_messages,omitempty"`
	SecurityIssues []api.SecurityIssue `json:"security_issues,omitempty"`

	// Analysis results (optional)
	SecurityAnalysis *SecurityAnalysis      `json:"security_analysis,omitempty"`
	BestPractices    *BestPracticesAnalysis `json:"best_practices,omitempty"`
	SuggestedFixes   []SuggestedFix         `json:"suggested_fixes,omitempty"`

	// Metadata
	ValidatorUsed  string        `json:"validator_used"`
	DockerfilePath string        `json:"dockerfile_path,omitempty"`
	Duration       time.Duration `json:"duration"`
	LinesAnalyzed  int           `json:"lines_analyzed"`
	RuleSet        string        `json:"rule_set"`
}

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// SecurityAnalysis is defined in dockerfile_types.go

// BestPracticesAnalysis contains best practices analysis
type BestPracticesAnalysis struct {
	Score            int      `json:"score"` // 0-100
	UsesMultiStage   bool     `json:"uses_multi_stage"`
	OptimizesLayers  bool     `json:"optimizes_layers"`
	HasHealthCheck   bool     `json:"has_health_check"`
	UsesDockerignore bool     `json:"uses_dockerignore"`
	PinsBaseImage    bool     `json:"pins_base_image"`
	Recommendations  []string `json:"recommendations,omitempty"`
}

// SuggestedFix represents a suggested fix for a validation issue
type SuggestedFix struct {
	Line        int    `json:"line"`
	Rule        string `json:"rule"`
	Description string `json:"description"`
	OldContent  string `json:"old_content"`
	NewContent  string `json:"new_content"`
	Confidence  int    `json:"confidence"` // 0-100
}

// ConsolidatedValidateDockerfileTool - Core Dockerfile validation tool
type ConsolidatedValidateDockerfileTool struct {
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	configValidator services.ConfigValidator
	logger          *slog.Logger

	// Validation engines
	basicValidator    *BasicDockerfileValidator
	hadolintValidator *HadolintValidator
	securityValidator *SecurityValidator
}

// NewConsolidatedValidateDockerfileTool creates a new consolidated Dockerfile validation tool
func NewConsolidatedValidateDockerfileTool(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *ConsolidatedValidateDockerfileTool {
	toolLogger := logger.With("tool", "validate_dockerfile_consolidated")

	return &ConsolidatedValidateDockerfileTool{
		sessionStore:      serviceContainer.SessionStore(),
		sessionState:      serviceContainer.SessionState(),
		configValidator:   serviceContainer.ConfigValidator(),
		logger:            toolLogger,
		basicValidator:    NewBasicDockerfileValidator(toolLogger),
		hadolintValidator: NewHadolintValidator(toolLogger),
		securityValidator: NewSecurityValidator(toolLogger),
	}
}

// Execute implements api.Tool interface
func (t *ConsolidatedValidateDockerfileTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	validationInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := validationInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Execute validation
	result, err := t.executeValidation(ctx, validationInput, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Validation failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeValidation performs the actual Dockerfile validation
func (t *ConsolidatedValidateDockerfileTool) executeValidation(
	ctx context.Context,
	input *ValidateDockerfileInput,
	startTime time.Time,
) (*ValidateDockerfileOutput, error) {
	result := &ValidateDockerfileOutput{
		Success:        false,
		SessionID:      input.SessionID,
		DockerfilePath: input.DockerfilePath,
		RuleSet:        input.RuleSet,
	}

	// Get Dockerfile content
	content, err := t.getDockerfileContent(input)
	if err != nil {
		return result, err
	}

	// Count lines for metadata
	result.LinesAnalyzed = strings.Count(content, "\n") + 1

	// Initialize session if needed
	if input.SessionID != "" && t.sessionStore != nil {
		if err := t.sessionStore.Create(ctx, &api.Session{
			ID:       input.SessionID,
			Metadata: map[string]interface{}{"tool": "validate_dockerfile", "dockerfile_path": input.DockerfilePath},
		}); err != nil {
			t.logger.Warn("Failed to create session", "error", err)
		}
	}

	// Choose validator based on input
	validator := t.chooseValidator(input)
	result.ValidatorUsed = validator

	// Perform validation
	var validationResult *ValidationResult
	switch validator {
	case "hadolint":
		validationResult, err = t.hadolintValidator.Validate(ctx, content, input)
	case "security":
		validationResult, err = t.securityValidator.Validate(ctx, content, input)
	default:
		validationResult, err = t.basicValidator.Validate(ctx, content, input)
	}

	if err != nil {
		return result, err
	}

	// Convert validation result to output format
	t.convertValidationResult(validationResult, result)

	// Calculate overall validation score
	result.ValidationScore = t.calculateValidationScore(result)
	result.IsValid = result.ValidationScore >= 70 && result.CriticalIssues == 0

	// Include optional analysis
	if input.IncludeAnalysis {
		result.SecurityAnalysis = t.performSecurityAnalysis(content)
		result.BestPractices = t.performBestPracticesAnalysis(content)
	}

	// Include suggested fixes
	if input.IncludeFixes {
		result.SuggestedFixes = t.generateSuggestedFixes(validationResult)
	}

	// Set timing and success
	result.Duration = time.Since(startTime)
	result.Success = true

	t.logger.Info("Dockerfile validation completed",
		"dockerfile_path", input.DockerfilePath,
		"validator", validator,
		"is_valid", result.IsValid,
		"score", result.ValidationScore,
		"issues", result.TotalIssues,
		"duration", result.Duration)

	return result, nil
}

// DockerfileAnalysisWrapper - Analysis wrapper for detailed reporting
type DockerfileAnalysisWrapper struct {
	consolidatedTool *ConsolidatedValidateDockerfileTool
	logger           *slog.Logger
}

// NewDockerfileAnalysisWrapper creates a new analysis wrapper
func NewDockerfileAnalysisWrapper(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *DockerfileAnalysisWrapper {
	return &DockerfileAnalysisWrapper{
		consolidatedTool: NewConsolidatedValidateDockerfileTool(serviceContainer, logger),
		logger:           logger.With("tool", "dockerfile_analysis_wrapper"),
	}
}

// Execute implements api.Tool interface for analysis wrapper
func (w *DockerfileAnalysisWrapper) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Parse input and ensure analysis is enabled
	validationInput, err := w.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Force enable analysis and fixes
	validationInput.IncludeAnalysis = true
	validationInput.IncludeFixes = true
	validationInput.CheckSecurity = true
	validationInput.CheckBestPractices = true

	// Use consolidated tool with enhanced input
	return w.consolidatedTool.Execute(ctx, api.ToolInput{
		Arguments: validationInput,
	})
}

// Implement api.Tool interface methods

func (t *ConsolidatedValidateDockerfileTool) Name() string {
	return "validate_dockerfile"
}

func (t *ConsolidatedValidateDockerfileTool) Description() string {
	return "Comprehensive Dockerfile validation with security checks, best practices analysis, and suggested fixes"
}

func (t *ConsolidatedValidateDockerfileTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "validate_dockerfile",
		Description: "Comprehensive Dockerfile validation with security checks, best practices analysis, and suggested fixes",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to Dockerfile",
				},
				"dockerfile_content": map[string]interface{}{
					"type":        "string",
					"description": "Raw Dockerfile content",
				},
				"check_security": map[string]interface{}{
					"type":        "boolean",
					"description": "Include security checks",
				},
				"check_best_practices": map[string]interface{}{
					"type":        "boolean",
					"description": "Include best practices checks",
				},
				"include_fixes": map[string]interface{}{
					"type":        "boolean",
					"description": "Include suggested fixes",
				},
			},
			"oneOf": []interface{}{
				map[string]interface{}{
					"required": []string{"dockerfile_path"},
				},
				map[string]interface{}{
					"required": []string{"dockerfile_content"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether validation was successful",
				},
				"is_valid": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether Dockerfile is valid",
				},
				"validation_score": map[string]interface{}{
					"type":        "integer",
					"description": "Overall validation score (0-100)",
				},
				"total_issues": map[string]interface{}{
					"type":        "integer",
					"description": "Total number of issues found",
				},
			},
		},
	}
}

func (w *DockerfileAnalysisWrapper) Name() string {
	return "validate_dockerfile_analysis"
}

func (w *DockerfileAnalysisWrapper) Description() string {
	return "Comprehensive Dockerfile analysis with detailed reporting, security analysis, and best practices recommendations"
}

func (w *DockerfileAnalysisWrapper) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "validate_dockerfile_analysis",
		Description: "Comprehensive Dockerfile analysis with detailed reporting, security analysis, and best practices recommendations",
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to Dockerfile",
				},
				"dockerfile_content": map[string]interface{}{
					"type":        "string",
					"description": "Raw Dockerfile content",
				},
			},
			"oneOf": []interface{}{
				map[string]interface{}{
					"required": []string{"dockerfile_path"},
				},
				map[string]interface{}{
					"required": []string{"dockerfile_content"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether analysis was successful",
				},
				"is_valid": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether Dockerfile is valid",
				},
				"validation_score": map[string]interface{}{
					"type":        "integer",
					"description": "Overall validation score (0-100)",
				},
				"security_analysis": map[string]interface{}{
					"type":        "object",
					"description": "Security analysis results",
				},
				"best_practices": map[string]interface{}{
					"type":        "object",
					"description": "Best practices analysis results",
				},
				"suggested_fixes": map[string]interface{}{
					"type":        "array",
					"description": "Suggested fixes for issues",
				},
			},
		},
	}
}

// Helper methods

func (t *ConsolidatedValidateDockerfileTool) parseInput(input api.ToolInput) (*ValidateDockerfileInput, error) {
	result := &ValidateDockerfileInput{}

	switch v := input.Arguments.(type) {
	case map[string]interface{}:
		if dockerfilePath, ok := v["dockerfile_path"].(string); ok {
			result.DockerfilePath = dockerfilePath
		}
		if dockerfileContent, ok := v["dockerfile_content"].(string); ok {
			result.DockerfileContent = dockerfileContent
		}
		if sessionID, ok := v["session_id"].(string); ok {
			result.SessionID = sessionID
		}
		if severity, ok := v["severity"].(string); ok {
			result.Severity = severity
		}
		if checkSecurity, ok := v["check_security"].(bool); ok {
			result.CheckSecurity = checkSecurity
		}
		if checkBestPractices, ok := v["check_best_practices"].(bool); ok {
			result.CheckBestPractices = checkBestPractices
		}
		if useHadolint, ok := v["use_hadolint"].(bool); ok {
			result.UseHadolint = useHadolint
		}
		if includeAnalysis, ok := v["include_analysis"].(bool); ok {
			result.IncludeAnalysis = includeAnalysis
		}
		if includeFixes, ok := v["include_fixes"].(bool); ok {
			result.IncludeFixes = includeFixes
		}
		if ruleSet, ok := v["rule_set"].(string); ok {
			result.RuleSet = ruleSet
		}
		if dryRun, ok := v["dry_run"].(bool); ok {
			result.DryRun = dryRun
		}
	case *ValidateDockerfileInput:
		result = v
	default:
		return nil, errors.NewError().Message("invalid input format").Build()
	}

	return result, nil
}

func (w *DockerfileAnalysisWrapper) parseInput(input api.ToolInput) (*ValidateDockerfileInput, error) {
	result := &ValidateDockerfileInput{}

	switch v := input.Arguments.(type) {
	case map[string]interface{}:
		if dockerfilePath, ok := v["dockerfile_path"].(string); ok {
			result.DockerfilePath = dockerfilePath
		}
		if dockerfileContent, ok := v["dockerfile_content"].(string); ok {
			result.DockerfileContent = dockerfileContent
		}
		if sessionID, ok := v["session_id"].(string); ok {
			result.SessionID = sessionID
		}
	default:
		return nil, errors.NewError().Message("invalid input format").Build()
	}

	return result, nil
}

func (t *ConsolidatedValidateDockerfileTool) getDockerfileContent(input *ValidateDockerfileInput) (string, error) {
	if input.DockerfileContent != "" {
		return input.DockerfileContent, nil
	}

	if input.DockerfilePath != "" {
		content, err := os.ReadFile(input.DockerfilePath)
		if err != nil {
			return "", errors.NewError().
				Message("Failed to read Dockerfile").
				Cause(err).
				Context("dockerfile_path", input.DockerfilePath).
				Build()
		}
		return string(content), nil
	}

	return "", errors.NewError().Message("no Dockerfile content provided").Build()
}

func (t *ConsolidatedValidateDockerfileTool) chooseValidator(input *ValidateDockerfileInput) string {
	if input.UseHadolint {
		return "hadolint"
	}
	if input.CheckSecurity {
		return "security"
	}
	return "basic"
}

func (t *ConsolidatedValidateDockerfileTool) calculateValidationScore(result *ValidateDockerfileOutput) int {
	score := 100

	// Deduct points for issues
	score -= len(result.Errors) * 10
	score -= len(result.Warnings) * 3
	score -= len(result.SecurityIssues) * 15
	score -= result.CriticalIssues * 20

	// Add bonus points for good practices
	if result.SecurityAnalysis != nil {
		if !result.SecurityAnalysis.RunsAsRoot {
			score += 5
		}
		if result.SecurityAnalysis.UsesPackagePin {
			score += 5
		}
		if result.SecurityAnalysis.UsesTLSCorrectly {
			score += 5
		}
	}

	if result.BestPractices != nil {
		if result.BestPractices.UsesMultiStage {
			score += 5
		}
		if result.BestPractices.HasHealthCheck {
			score += 5
		}
		if result.BestPractices.PinsBaseImage {
			score += 5
		}
	}

	// Ensure score is within valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// Placeholder implementations for validators (would be implemented separately)

type ValidationResult struct {
	Errors         []ValidationIssue
	Warnings       []ValidationIssue
	InfoMessages   []ValidationIssue
	SecurityIssues []api.SecurityIssue
}

type BasicDockerfileValidator struct {
	logger *slog.Logger
}

func NewBasicDockerfileValidator(logger *slog.Logger) *BasicDockerfileValidator {
	return &BasicDockerfileValidator{logger: logger}
}

func (v *BasicDockerfileValidator) Validate(ctx context.Context, content string, input *ValidateDockerfileInput) (*ValidationResult, error) {
	// Placeholder implementation
	return &ValidationResult{
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}, nil
}

type HadolintValidator struct {
	logger *slog.Logger
}

func NewHadolintValidator(logger *slog.Logger) *HadolintValidator {
	return &HadolintValidator{logger: logger}
}

func (v *HadolintValidator) Validate(ctx context.Context, content string, input *ValidateDockerfileInput) (*ValidationResult, error) {
	// Placeholder implementation
	return &ValidationResult{
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}, nil
}

type SecurityValidator struct {
	logger *slog.Logger
}

func NewSecurityValidator(logger *slog.Logger) *SecurityValidator {
	return &SecurityValidator{logger: logger}
}

func (v *SecurityValidator) Validate(ctx context.Context, content string, input *ValidateDockerfileInput) (*ValidationResult, error) {
	// Placeholder implementation
	return &ValidationResult{
		Errors:         []ValidationIssue{},
		Warnings:       []ValidationIssue{},
		SecurityIssues: []api.SecurityIssue{},
	}, nil
}

func (t *ConsolidatedValidateDockerfileTool) convertValidationResult(validationResult *ValidationResult, result *ValidateDockerfileOutput) {
	result.Errors = validationResult.Errors
	result.Warnings = validationResult.Warnings
	result.InfoMessages = validationResult.InfoMessages
	result.SecurityIssues = validationResult.SecurityIssues

	result.TotalIssues = len(result.Errors) + len(result.Warnings) + len(result.SecurityIssues)

	// Count critical issues
	for _, issue := range result.Errors {
		if issue.Severity == "error" {
			result.CriticalIssues++
		}
	}
	for _, issue := range result.SecurityIssues {
		if issue.Severity == "critical" {
			result.CriticalIssues++
		}
	}
}

func (t *ConsolidatedValidateDockerfileTool) performSecurityAnalysis(content string) *SecurityAnalysis {
	// Placeholder implementation
	return &SecurityAnalysis{
		SecurityScore:    80,
		RunsAsRoot:       strings.Contains(content, "USER root"),
		UsesPackagePin:   strings.Contains(content, "="),
		ExposesSecrets:   false,
		UsesTLSCorrectly: true,
		SecurityRisks:    []string{},
		Recommendations:  []string{"Use non-root user", "Pin package versions"},
	}
}

func (t *ConsolidatedValidateDockerfileTool) performBestPracticesAnalysis(content string) *BestPracticesAnalysis {
	// Placeholder implementation
	return &BestPracticesAnalysis{
		Score:            85,
		UsesMultiStage:   strings.Contains(content, "FROM") && strings.Count(content, "FROM") > 1,
		OptimizesLayers:  true,
		HasHealthCheck:   strings.Contains(content, "HEALTHCHECK"),
		UsesDockerignore: false, // Would need to check filesystem
		PinsBaseImage:    strings.Contains(content, ":") && !strings.Contains(content, ":latest"),
		Recommendations:  []string{"Add .dockerignore file", "Use HEALTHCHECK instruction"},
	}
}

func (t *ConsolidatedValidateDockerfileTool) generateSuggestedFixes(validationResult *ValidationResult) []SuggestedFix {
	var fixes []SuggestedFix

	// Generate fixes based on validation results
	for _, issue := range validationResult.Errors {
		if issue.Fix != "" {
			fixes = append(fixes, SuggestedFix{
				Line:        issue.Line,
				Rule:        issue.Rule,
				Description: fmt.Sprintf("Fix: %s", issue.Message),
				OldContent:  "", // Would need to extract from content
				NewContent:  issue.Fix,
				Confidence:  80,
			})
		}
	}

	return fixes
}
