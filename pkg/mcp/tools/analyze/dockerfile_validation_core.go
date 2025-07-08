package analyze

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	internalcommon "github.com/Azure/container-kit/pkg/mcp/internal/common"
	validationcore "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/tools/build"

	constants "github.com/Azure/container-kit/pkg/mcp/core"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/localrivet/gomcp/server"
)

// ============================================================================
// Extended Types for Backward Compatibility
// ============================================================================

// These types extend the base types from dockerfile_types.go to maintain
// backward compatibility with the existing validation implementation.

// ExtendedValidationResult extends AtomicValidateDockerfileResult with additional fields
// needed for the core validation implementation.
type ExtendedValidationResult struct {
	AtomicValidateDockerfileResult

	// Additional fields for backward compatibility
	SessionID         string                 `json:"session_id"`
	DockerfilePath    string                 `json:"dockerfile_path"`
	Duration          time.Duration          `json:"duration"`
	ValidatorUsed     string                 `json:"validator_used"` // hadolint, basic, hybrid
	IsValid           bool                   `json:"is_valid"`
	TotalIssues       int                    `json:"total_issues"`
	CriticalIssues    int                    `json:"critical_issues"`
	FixesApplied      []string               `json:"fixes_applied,omitempty"`
	ValidationContext map[string]interface{} `json:"validation_context"`
}

// ExtendedSecurityAnalysis extends SecurityAnalysis with backward compatibility fields
type ExtendedSecurityAnalysis struct {
	SecurityAnalysis

	// Backward compatibility fields
	RunsAsRoot     bool `json:"runs_as_root"`
	HasSecrets     bool `json:"has_secrets"`
	UsesPackagePin bool `json:"uses_package_pinning"`
	SecurityScore  int  `json:"security_score"` // 0-100
}

// ExtendedBaseImageAnalysis extends BaseImageAnalysis with backward compatibility fields
type ExtendedBaseImageAnalysis struct {
	BaseImageAnalysis

	// Backward compatibility fields
	Registry      string   `json:"registry"`
	IsTrusted     bool     `json:"is_trusted"`
	IsOfficial    bool     `json:"is_official"`
	HasKnownVulns bool     `json:"has_known_vulnerabilities"`
	Alternatives  []string `json:"alternatives,omitempty"`
}

// ExtendedLayerAnalysis extends LayerAnalysis with backward compatibility fields
type ExtendedLayerAnalysis struct {
	LayerAnalysis

	// Backward compatibility fields
	CacheableSteps   int                 `json:"cacheable_steps"`
	ProblematicSteps []ProblematicStep   `json:"problematic_steps"`
	Optimizations    []LayerOptimization `json:"optimizations"`
}

// ProblematicStep represents a problematic layer configuration
type ProblematicStep struct {
	Line        int    `json:"line"`
	Instruction string `json:"instruction"`
	Issue       string `json:"issue"`
	Impact      string `json:"impact"`
}

// LayerOptimization represents a layer optimization opportunity
type LayerOptimization struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Benefit     string `json:"benefit"`
}

// ExtendedRecommendation extends Recommendation with additional fields
type ExtendedRecommendation struct {
	Recommendation

	// Additional fields for backward compatibility
	RecommendationID string   `json:"recommendation_id"`
	Type             string   `json:"type"`
	Tags             []string `json:"tags"`
	ActionType       string   `json:"action_type"`
	Confidence       int      `json:"confidence"`
	Benefits         []string `json:"benefits"`
	Risks            []string `json:"risks"`
	Urgency          string   `json:"urgency"`
}

// ============================================================================
// Core Validation Tool
// ============================================================================

// AtomicValidateDockerfileTool provides comprehensive Dockerfile validation
// with support for multiple validators, security analysis, and optimization recommendations.
type AtomicValidateDockerfileTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionStore    services.SessionStore // Focused service interface
	sessionState    services.SessionState // Focused service interface
	logger          *slog.Logger
	analyzer        internalcommon.FailureAnalyzer
	fixingMixin     *build.AtomicToolFixingMixin
}

// NewAtomicValidateDockerfileTool creates a new instance of the validation tool using focused service interfaces.
func NewAtomicValidateDockerfileTool(adapter core.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicValidateDockerfileTool {
	toolLogger := logger.With("tool", "atomic_validate_dockerfile")
	return createAtomicValidateDockerfileTool(adapter, sessionStore, sessionState, toolLogger)
}

// NewAtomicValidateDockerfileToolWithServices creates a new instance of the validation tool using service container
func NewAtomicValidateDockerfileToolWithServices(adapter core.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicValidateDockerfileTool {
	toolLogger := logger.With("tool", "atomic_validate_dockerfile")

	// Use focused services directly - no wrapper needed!
	return createAtomicValidateDockerfileTool(adapter, serviceContainer.SessionStore(), serviceContainer.SessionState(), toolLogger)
}

// createAtomicValidateDockerfileTool is the common creation logic
func createAtomicValidateDockerfileTool(adapter core.TypedPipelineOperations, sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicValidateDockerfileTool {
	return &AtomicValidateDockerfileTool{
		pipelineAdapter: adapter,
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		logger:          logger,
	}
}

// SetAnalyzer sets the failure analyzer for enhanced error reporting.
func (t *AtomicValidateDockerfileTool) SetAnalyzer(analyzer internalcommon.FailureAnalyzer) {
	t.analyzer = analyzer
}

// SetFixingMixin sets the fixing mixin for automatic issue resolution.
func (t *AtomicValidateDockerfileTool) SetFixingMixin(mixin *build.AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

// ============================================================================
// Main Validation Entry Points
// ============================================================================

// ExecuteValidation performs Dockerfile validation with the specified configuration.
func (t *AtomicValidateDockerfileTool) ExecuteValidation(ctx context.Context, args AtomicValidateDockerfileArgs) (*ExtendedValidationResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext performs validation with MCP server context support.
func (t *AtomicValidateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args AtomicValidateDockerfileArgs) (*ExtendedValidationResult, error) {
	// Progress tracking removed for simplification
	ctx := context.Background()
	result, err := t.performValidation(ctx, args, nil)

	if err != nil {
		t.logger.Info("Validation failed")
		return result, nil
	} else {
		t.logger.Info("Validation completed successfully")
	}

	return result, nil
}

// executeWithoutProgress executes validation without progress reporting.
func (t *AtomicValidateDockerfileTool) executeWithoutProgress(ctx context.Context, args AtomicValidateDockerfileArgs) (*ExtendedValidationResult, error) {
	return t.performValidation(ctx, args, nil)
}

// ============================================================================
// Core Validation Logic
// ============================================================================

// performValidation orchestrates the entire validation process.
// This is the main validation orchestrator that coordinates all validation activities.
func (t *AtomicValidateDockerfileTool) performValidation(ctx context.Context, args AtomicValidateDockerfileArgs, reporter interface{}) (*ExtendedValidationResult, error) {
	startTime := time.Now()

	// Get session using focused service interface
	sessionData, err := t.sessionStore.Get(ctx, args.SessionID)
	if err != nil {
		result := &ExtendedValidationResult{
			AtomicValidateDockerfileResult: AtomicValidateDockerfileResult{
				BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
				BaseAIContextResult: mcptypes.NewBaseAIContextResult("validate", false, 0), // Will be updated later
			},
			SessionID: args.SessionID,
			Duration:  time.Since(startTime),
		}

		t.logger.Error("Failed to get session", "error", err, "session_id", args.SessionID)
		return result, nil
	}

	// Convert api.Session to core.SessionState for compatibility
	session := &mcptypes.SessionState{
		SessionID: sessionData.ID,
		Metadata:  sessionData.Metadata,
	}

	t.logger.Info("Starting atomic Dockerfile validation",
		"session_id", session.SessionID,
		"dockerfile_path", args.DockerfilePath,
		"use_hadolint", args.UseHadolint)

	// Initialize result structure
	result := &ExtendedValidationResult{
		AtomicValidateDockerfileResult: AtomicValidateDockerfileResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("validate", false, 0),
		},
		SessionID:         session.SessionID,
		ValidationContext: make(map[string]interface{}),
	}

	// Resolve Dockerfile content
	dockerfilePath, dockerfileContent, err := t.resolveDockerfileContent(args, session.SessionID)
	if err != nil {
		t.logger.Error("Failed to resolve Dockerfile content", "error", err, "dockerfile_path", dockerfilePath)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.DockerfilePath = dockerfilePath

	// Perform core validation
	validationResult, validatorUsed, err := t.performCoreValidation(ctx, dockerfileContent, args)
	if err != nil {
		t.logger.Error("Core validation failed", "error", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.ValidatorUsed = validatorUsed
	result.IsValid = validationResult.Valid

	// Process validation results
	t.processValidationResults(result, validationResult, args)

	// Perform additional analysis if requested
	if args.CheckSecurity || args.CheckOptimization || args.CheckBestPractices {
		t.performAdditionalAnalysis(result, dockerfileContent, args)
	}

	// Generate fixes if requested
	if args.GenerateFixes && !result.IsValid {
		correctedDockerfile, fixes := t.generateCorrectedDockerfile(dockerfileContent, validationResult)
		result.CorrectedDockerfile = correctedDockerfile
		result.FixesApplied = fixes
	}

	// Calculate final validation score
	result.ValidationScore = float64(t.calculateValidationScore(result))

	// Finalize result
	result.Duration = time.Since(startTime)
	result.BaseAIContextResult.IsSuccessful = result.IsValid
	result.BaseAIContextResult.Duration = result.Duration

	t.logger.Info("Dockerfile validation completed",
		"session_id", session.SessionID,
		"validator", validatorUsed,
		"is_valid", result.IsValid,
		"total_issues", result.TotalIssues,
		"validation_score", result.ValidationScore,
		"duration", result.Duration)

	return result, nil
}

// ============================================================================
// Session Management and File I/O
// ============================================================================

// resolveDockerfileContent handles both path-based and content-based input.
func (t *AtomicValidateDockerfileTool) resolveDockerfileContent(args AtomicValidateDockerfileArgs, sessionID string) (string, string, error) {
	var dockerfilePath string
	var dockerfileContent string

	if args.DockerfileContent != "" {
		// Content-based validation
		dockerfileContent = args.DockerfileContent
		dockerfilePath = types.ValidationModeInline
	} else {
		// Path-based validation
		if args.DockerfilePath != "" {
			dockerfilePath = args.DockerfilePath
		} else {
			workspaceDir := t.pipelineAdapter.GetSessionWorkspace(sessionID)
			dockerfilePath = filepath.Join(workspaceDir, "Dockerfile")
		}

		content, err := os.ReadFile(dockerfilePath)
		if err != nil {
			return dockerfilePath, "", errors.NewError().Message("failed to read Dockerfile at " + dockerfilePath).Cause(err).WithLocation().Build()
		}
		dockerfileContent = string(content)
	}

	return dockerfilePath, dockerfileContent, nil
}

// ============================================================================
// Core Validation Execution
// ============================================================================

// performCoreValidation executes the primary validation logic using the appropriate validator.
func (t *AtomicValidateDockerfileTool) performCoreValidation(ctx context.Context, dockerfileContent string, args AtomicValidateDockerfileArgs) (*types.BuildValidationResult, string, error) {
	var validationResult *types.BuildValidationResult
	var validatorUsed string
	var err error

	// Try Hadolint validation if requested
	if args.UseHadolint {
		hadolintValidator := coredocker.NewHadolintValidator(t.logger)
		validationResult, err = hadolintValidator.ValidateWithHadolint(ctx, dockerfileContent)
		if err != nil {
			t.logger.Warn("Hadolint validation failed, falling back to basic validation", "error", err)
			validatorUsed = "basic_fallback"
		} else {
			validatorUsed = "hadolint"
		}
	}

	// Fall back to basic validation if Hadolint failed or wasn't requested
	if validationResult == nil {
		basicValidator := coredocker.NewValidator(t.logger)
		validationResult = basicValidator.ValidateDockerfile(dockerfileContent)
		if validatorUsed == "" {
			validatorUsed = "basic"
		}
	}

	return validationResult, validatorUsed, nil
}

// ============================================================================
// Result Processing and Analysis
// ============================================================================

// processValidationResults converts core validation results to the result structure.
func (t *AtomicValidateDockerfileTool) processValidationResults(result *ExtendedValidationResult, validationResult *types.BuildValidationResult, _ AtomicValidateDockerfileArgs) {
	// Process validation errors
	for _, err := range validationResult.Errors {
		// Extract line and column from context if available
		var line, column int
		if lineStr, ok := err.Context["line"]; ok {
			if parsedLine, err := strconv.Atoi(lineStr); err == nil {
				line = parsedLine
			}
		}
		if colStr, ok := err.Context["column"]; ok {
			if parsedCol, err := strconv.Atoi(colStr); err == nil {
				column = parsedCol
			}
		}

		instruction := err.Field

		dockerfileErr := DockerfileError{
			Line:        line,
			Column:      column,
			Rule:        err.Code, // Use error code as rule
			Message:     err.Message,
			Instruction: instruction, // Use field as instruction
			Severity:    string(err.Severity),
			Fix:         "", // TODO: Add fix generation logic
		}

		// Categorize security issues separately
		if strings.Contains(strings.ToLower(err.Code), "security") || strings.Contains(strings.ToLower(err.Message), "security") {
			result.SecurityIssues = append(result.SecurityIssues, DockerfileSecurityIssue{
				Line:        line,
				Type:        "security",
				Severity:    string(err.Severity),
				Description: err.Message,
			})
			result.CriticalIssues++
		} else {
			result.Errors = append(result.Errors, dockerfileErr)
			if err.Severity == validationcore.SeverityHigh || err.Severity == validationcore.SeverityCritical {
				result.CriticalIssues++
			}
		}
	}

	// Process validation warnings
	for _, warn := range validationResult.Warnings {
		suggestion := warn.Suggestion

		// Extract line and column from context if available
		var line, column int
		if lineStr, ok := warn.Context["line"]; ok {
			if parsedLine, err := strconv.Atoi(lineStr); err == nil {
				line = parsedLine
			}
		}
		if colStr, ok := warn.Context["column"]; ok {
			if parsedCol, err := strconv.Atoi(colStr); err == nil {
				column = parsedCol
			}
		}

		instruction := warn.Field

		result.Warnings = append(result.Warnings, DockerfileWarning{
			Line:        line,
			Column:      column,
			Rule:        warn.Code,
			Message:     warn.Message,
			Severity:    "warning",
			Instruction: instruction,
			Suggestion:  suggestion,
		})
	}

	// Convert suggestions to DockerfileSuggestion format from Details map
	if suggestions, ok := validationResult.Details["suggestions"].([]string); ok {
		for _, suggestion := range suggestions {
			result.Suggestions = append(result.Suggestions, DockerfileSuggestion{
				Category:    "general",
				Message:     suggestion,
				Improvement: suggestion,
				Impact:      "improvement",
			})
		}
	}

	// Calculate totals
	result.TotalIssues = len(result.Errors) + len(result.Warnings) + len(result.SecurityIssues)

	// Add validation context
	if validationResult.Metadata.Context != nil {
		for k, v := range validationResult.Metadata.Context {
			result.ValidationContext[k] = v
		}
	}
}

// performAdditionalAnalysis performs extended analysis beyond basic validation.
func (t *AtomicValidateDockerfileTool) performAdditionalAnalysis(result *ExtendedValidationResult, dockerfileContent string, args AtomicValidateDockerfileArgs) {
	lines := strings.Split(dockerfileContent, "\n")

	// Analyze base image - convert Extended to Base for assignment
	extBaseAnalysis := t.analyzeBaseImage(lines)
	result.BaseImageAnalysis = &extBaseAnalysis.BaseImageAnalysis

	// Analyze layer structure - convert Extended to Base for assignment
	extLayerAnalysis := t.analyzeDockerfileLayers(lines)
	result.LayerAnalysis = &extLayerAnalysis.LayerAnalysis

	// Perform security analysis - convert Extended to Base for assignment
	if args.CheckSecurity {
		extSecurityAnalysis := t.performSecurityAnalysis(lines)
		result.SecurityAnalysis = &extSecurityAnalysis.SecurityAnalysis
	}

	// Generate optimization recommendations
	if args.CheckOptimization {
		result.OptimizationTips = t.generateOptimizationTips(lines, extLayerAnalysis)
	}
}

// ============================================================================
// Tool Interface Implementation
// ============================================================================

func (t *AtomicValidateDockerfileTool) GetName() string {
	return "atomic_validate_dockerfile"
}

func (t *AtomicValidateDockerfileTool) GetDescription() string {
	return "Validates Dockerfiles against best practices, security standards, and optimization guidelines"
}

func (t *AtomicValidateDockerfileTool) GetVersion() string {
	return constants.AtomicToolVersion
}

func (t *AtomicValidateDockerfileTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

func (t *AtomicValidateDockerfileTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:        "atomic_validate_dockerfile",
		Description: "Validates Dockerfiles against best practices, security standards, and optimization guidelines with automatic fix generation",
		Version:     "1.0.0",
		Category:    api.ToolCategory("validation"),
		Tags:        []string{"validation", "dockerfile", "security"},
		Status:      api.ToolStatus("active"),
		Dependencies: []string{
			"session_manager",
			"docker_access",
			"file_system_access",
			"hadolint_optional",
		},
		Capabilities: []string{
			"dockerfile_validation",
			"syntax_checking",
			"security_analysis",
			"best_practices_validation",
			"optimization_analysis",
			"fix_generation",
			"hadolint_integration",
			"base_image_analysis",
			"layer_optimization",
		},
		Requirements: []string{
			"valid_session_id",
		},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

func (t *AtomicValidateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	// First validate with tags
	if err := validationcore.ValidateTaggedStruct(args); err != nil {
		return err
	}

	// Additional custom validation for mutual exclusivity
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return errors.NewError().Messagef("invalid argument type for dockerfile validation").Build()
	}

	// Must provide either path or content
	if validateArgs.DockerfilePath == "" && validateArgs.DockerfileContent == "" {
		return errors.NewError().Messagef("either dockerfile_path or dockerfile_content is required").Build()
	}

	return nil
}

func (t *AtomicValidateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return nil, errors.NewError().Messagef("invalid argument type for typed dockerfile validation").Build()
	}

	return t.ExecuteTyped(ctx, validateArgs)
}

func (t *AtomicValidateDockerfileTool) ExecuteTyped(ctx context.Context, args AtomicValidateDockerfileArgs) (*ExtendedValidationResult, error) {
	return t.ExecuteValidation(ctx, args)
}
