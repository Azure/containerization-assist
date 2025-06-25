package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/build"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	constants "github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	ai_context "github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicValidateDockerfileArgs defines arguments for atomic Dockerfile validation
type AtomicValidateDockerfileArgs struct {
	types.BaseToolArgs

	// Validation targets
	DockerfilePath    string `json:"dockerfile_path,omitempty" description:"Path to Dockerfile (default: session workspace/Dockerfile)"`
	DockerfileContent string `json:"dockerfile_content,omitempty" description:"Dockerfile content to validate (alternative to path)"`

	// Validation options
	UseHadolint       bool     `json:"use_hadolint,omitempty" description:"Use Hadolint for advanced validation"`
	Severity          string   `json:"severity,omitempty" description:"Minimum severity to report (info, warning, error)"`
	IgnoreRules       []string `json:"ignore_rules,omitempty" description:"Hadolint rules to ignore (e.g., DL3008, DL3009)"`
	TrustedRegistries []string `json:"trusted_registries,omitempty" description:"List of trusted registries for base image validation"`

	// Analysis options
	CheckSecurity      bool `json:"check_security,omitempty" description:"Perform security-focused checks"`
	CheckOptimization  bool `json:"check_optimization,omitempty" description:"Check for image size optimization opportunities"`
	CheckBestPractices bool `json:"check_best_practices,omitempty" description:"Validate against Docker best practices"`

	// Output options
	IncludeSuggestions bool `json:"include_suggestions,omitempty" description:"Include remediation suggestions"`
	GenerateFixes      bool `json:"generate_fixes,omitempty" description:"Generate corrected Dockerfile"`
}

// AtomicValidateDockerfileResult represents the result of atomic Dockerfile validation
type AtomicValidateDockerfileResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult // Embedded for AI context methods

	// Validation metadata
	SessionID      string        `json:"session_id"`
	DockerfilePath string        `json:"dockerfile_path"`
	Duration       time.Duration `json:"duration"`
	ValidatorUsed  string        `json:"validator_used"` // hadolint, basic, hybrid

	// Validation results
	IsValid         bool `json:"is_valid"`
	ValidationScore int  `json:"validation_score"` // 0-100
	TotalIssues     int  `json:"total_issues"`
	CriticalIssues  int  `json:"critical_issues"`

	// Issue breakdown
	Errors           []DockerfileValidationError   `json:"errors"`
	Warnings         []DockerfileValidationWarning `json:"warnings"`
	SecurityIssues   []DockerfileSecurityIssue     `json:"security_issues"`
	OptimizationTips []OptimizationTip             `json:"optimization_tips"`

	// Analysis results
	BaseImageAnalysis BaseImageAnalysis `json:"base_image_analysis"`
	LayerAnalysis     LayerAnalysis     `json:"layer_analysis"`
	SecurityAnalysis  SecurityAnalysis  `json:"security_analysis"`

	// Remediation
	Suggestions         []string `json:"suggestions"`
	CorrectedDockerfile string   `json:"corrected_dockerfile,omitempty"`
	FixesApplied        []string `json:"fixes_applied,omitempty"`

	// Context and debugging
	ValidationContext map[string]interface{} `json:"validation_context"`
}

// DockerfileValidationError represents a validation error with enhanced context
type DockerfileValidationError struct {
	Type          string `json:"type"` // syntax, instruction, security, best_practice
	Line          int    `json:"line"`
	Column        int    `json:"column,omitempty"`
	Rule          string `json:"rule,omitempty"` // Hadolint rule code (DL3008, etc.)
	Message       string `json:"message"`
	Instruction   string `json:"instruction,omitempty"`
	Severity      string `json:"severity"` // error, warning, info
	Fix           string `json:"fix,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

// DockerfileValidationWarning represents a validation warning
type DockerfileValidationWarning struct {
	Type       string `json:"type"`
	Line       int    `json:"line"`
	Rule       string `json:"rule,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Impact     string `json:"impact,omitempty"` // performance, security, maintainability
}

// DockerfileSecurityIssue represents a security-related issue in the Dockerfile
type DockerfileSecurityIssue struct {
	Type          string   `json:"type"` // exposed_port, root_user, secrets, etc.
	Line          int      `json:"line"`
	Severity      string   `json:"severity"` // low, medium, high, critical
	Description   string   `json:"description"`
	Remediation   string   `json:"remediation"`
	CVEReferences []string `json:"cve_references,omitempty"`
}

// OptimizationTip represents an optimization suggestion
type OptimizationTip struct {
	Type             string `json:"type"` // layer_consolidation, cache_optimization, etc.
	Line             int    `json:"line,omitempty"`
	Description      string `json:"description"`
	Impact           string `json:"impact"` // size_reduction, build_speed, etc.
	Suggestion       string `json:"suggestion"`
	EstimatedSavings string `json:"estimated_savings,omitempty"` // e.g., "50MB", "30% faster"
}

// BaseImageAnalysis provides analysis of the base image
type BaseImageAnalysis struct {
	Image           string   `json:"image"`
	Registry        string   `json:"registry"`
	IsTrusted       bool     `json:"is_trusted"`
	IsOfficial      bool     `json:"is_official"`
	HasKnownVulns   bool     `json:"has_known_vulnerabilities"`
	Alternatives    []string `json:"alternatives,omitempty"`
	Recommendations []string `json:"recommendations"`
}

// LayerAnalysis provides analysis of Dockerfile layers
type LayerAnalysis struct {
	TotalLayers      int                 `json:"total_layers"`
	CacheableSteps   int                 `json:"cacheable_steps"`
	ProblematicSteps []ProblematicStep   `json:"problematic_steps"`
	Optimizations    []LayerOptimization `json:"optimizations"`
}

// ProblematicStep represents a step that could cause issues
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

// SecurityAnalysis provides comprehensive security analysis
type SecurityAnalysis struct {
	RunsAsRoot      bool     `json:"runs_as_root"`
	ExposedPorts    []int    `json:"exposed_ports"`
	HasSecrets      bool     `json:"has_secrets"`
	UsesPackagePin  bool     `json:"uses_package_pinning"`
	SecurityScore   int      `json:"security_score"` // 0-100
	Recommendations []string `json:"recommendations"`
}

// AtomicValidateDockerfileTool implements atomic Dockerfile validation
type AtomicValidateDockerfileTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	fixingMixin     *fixing.AtomicToolFixingMixin
	// dockerfileAdapter removed - functionality integrated directly
	logger zerolog.Logger
}

// NewAtomicValidateDockerfileTool creates a new atomic Dockerfile validation tool
func NewAtomicValidateDockerfileTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicValidateDockerfileTool {
	toolLogger := logger.With().Str("tool", "atomic_validate_dockerfile").Logger()
	return &AtomicValidateDockerfileTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		fixingMixin:     nil, // Will be set via SetAnalyzer if fixing is enabled
		// dockerfileAdapter removed - functionality integrated directly
		logger: toolLogger,
	}
}

// ExecuteValidation runs the atomic Dockerfile validation
func (t *AtomicValidateDockerfileTool) ExecuteValidation(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic Dockerfile validation with GoMCP progress tracking
func (t *AtomicValidateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	// Create progress adapter for GoMCP using standard validation stages
	_ = mcptypes.NewGoMCPProgressAdapter(serverCtx, []mcptypes.LocalProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Validate", Weight: 0.80, Description: "Validating"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performValidation(ctx, args, nil)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Validation failed")
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Validation completed successfully")
	}

	return result, nil
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicValidateDockerfileTool) executeWithoutProgress(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	return t.performValidation(ctx, args, nil)
}

// performValidation performs the actual Dockerfile validation
func (t *AtomicValidateDockerfileTool) performValidation(ctx context.Context, args AtomicValidateDockerfileArgs, reporter interface{}) (*AtomicValidateDockerfileResult, error) {
	startTime := time.Now()

	// Stage 1: Initialize
	// Progress reporting removed

	// Get session
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicValidateDockerfileResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_validate_dockerfile", args.SessionID, args.DryRun),
			BaseAIContextResult: mcptypes.NewBaseAIContextResult("validate", false, 0), // Will be updated later
			Duration:            time.Since(startTime),
		}

		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, nil
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Progress reporting removed

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("dockerfile_path", args.DockerfilePath).
		Bool("use_hadolint", args.UseHadolint).
		Msg("Starting atomic Dockerfile validation")

	// Create base result
	result := &AtomicValidateDockerfileResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_validate_dockerfile", session.SessionID, args.DryRun),
		BaseAIContextResult: mcptypes.NewBaseAIContextResult("validate", false, 0), // Will be updated later
		ValidationContext:   make(map[string]interface{}),
	}

	// Stage 2: Read Dockerfile
	// Progress reporting removed

	// Determine Dockerfile path and content
	var dockerfilePath string
	var dockerfileContent string

	if args.DockerfileContent != "" {
		// Use provided content
		dockerfileContent = args.DockerfileContent
		dockerfilePath = types.ValidationModeInline
	} else {
		// Determine Dockerfile path
		if args.DockerfilePath != "" {
			dockerfilePath = args.DockerfilePath
		} else {
			// Default to session workspace
			workspaceDir := t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
			dockerfilePath = filepath.Join(workspaceDir, "Dockerfile")
		}

		// Progress reporting removed

		// Read Dockerfile content
		content, err := os.ReadFile(dockerfilePath)
		if err != nil {
			t.logger.Error().Err(err).Str("dockerfile_path", result.DockerfilePath).Msg("Failed to read Dockerfile")
			result.Duration = time.Since(startTime)
			return result, nil
		}
		dockerfileContent = string(content)
	}

	result.DockerfilePath = dockerfilePath

	// Progress reporting removed

	// Stage 3: Validate Dockerfile
	// Progress reporting removed

	// Check if we should use refactored modules
	useRefactoredModules := os.Getenv("USE_REFACTORED_DOCKERFILE") == "true"
	if useRefactoredModules {
		t.logger.Info().Msg("Using refactored Dockerfile validation modules")
		// dockerfileAdapter removed - return error for now
		return nil, types.NewRichError("FEATURE_NOT_IMPLEMENTED", "refactored Dockerfile validation not implemented without adapter", types.ErrTypeSystem)
	}

	// Perform validation using legacy code
	var validationResult *coredocker.ValidationResult
	var validatorUsed string

	if args.UseHadolint {
		// Progress reporting removed

		// Try Hadolint validation first
		hadolintValidator := coredocker.NewHadolintValidator(t.logger)
		validationResult, err = hadolintValidator.ValidateWithHadolint(ctx, dockerfileContent)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Hadolint validation failed, falling back to basic validation")
			validatorUsed = "basic_fallback"
		} else {
			validatorUsed = "hadolint"
		}
	}

	// Fall back to basic validation if Hadolint failed or wasn't requested
	if validationResult == nil {
		// Progress reporting removed

		basicValidator := coredocker.NewValidator(t.logger)
		validationResult = basicValidator.ValidateDockerfile(dockerfileContent)
		if validatorUsed == "" {
			validatorUsed = "basic"
		}
	}

	result.ValidatorUsed = validatorUsed
	result.IsValid = validationResult.Valid

	// Progress reporting removed

	// Process validation results
	t.processValidationResults(result, validationResult, args)

	// Progress reporting removed

	// Stage 4: Analyze (additional checks)
	if args.CheckSecurity || args.CheckOptimization || args.CheckBestPractices {
		// Progress reporting removed

		t.performAdditionalAnalysis(result, dockerfileContent, args)

		// Progress reporting removed
	}

	// Stage 5: Generate fixes and suggestions
	if args.GenerateFixes && !result.IsValid {
		// Progress reporting removed

		correctedDockerfile, fixes := t.generateCorrectedDockerfile(dockerfileContent, validationResult)
		result.CorrectedDockerfile = correctedDockerfile
		result.FixesApplied = fixes

		// Progress reporting removed
	}

	// Stage 6: Finalize
	// Progress reporting removed

	// Calculate validation score
	result.ValidationScore = t.calculateValidationScore(result)

	result.Duration = time.Since(startTime)

	// Update mcptypes.BaseAIContextResult with final values
	result.BaseAIContextResult.IsSuccessful = result.IsValid
	result.BaseAIContextResult.Duration = result.Duration

	// Progress reporting removed

	// Log results
	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("validator", validatorUsed).
		Bool("is_valid", result.IsValid).
		Int("total_issues", result.TotalIssues).
		Int("validation_score", result.ValidationScore).
		Dur("duration", result.Duration).
		Msg("Dockerfile validation completed")

	return result, nil
}

// AI Context Interface Implementations

// AI Context methods are now provided by embedded mcptypes.BaseAIContextResult

// GenerateRecommendations implements ai_context.Recommendable
func (r *AtomicValidateDockerfileResult) GenerateRecommendations() []ai_context.Recommendation {
	recommendations := make([]ai_context.Recommendation, 0)

	// Security recommendations
	if len(r.SecurityIssues) > 0 {
		recommendations = append(recommendations, ai_context.Recommendation{
			RecommendationID: fmt.Sprintf("security-fixes-%s", r.SessionID),
			Title:            "Address Security Issues",
			Description:      "Fix identified security vulnerabilities in Dockerfile",
			Category:         "security",
			Priority:         types.SeverityHigh,
			Type:             "fix",
			Tags:             []string{"security", "dockerfile", "vulnerabilities"},
			ActionType:       "immediate",
			Benefits:         []string{"Improved security posture", "Reduced attack surface"},
			Risks:            []string{"Build process changes", "Compatibility issues"},
			Urgency:          "immediate",
			Effort:           "medium",
			Impact:           types.SeverityHigh,
			Confidence:       95,
		})
	}

	// Error recommendations
	if len(r.Errors) > 0 {
		recommendations = append(recommendations, ai_context.Recommendation{
			RecommendationID: fmt.Sprintf("validation-errors-%s", r.SessionID),
			Title:            "Fix Validation Errors",
			Description:      "Address validation errors in Dockerfile",
			Category:         "quality",
			Priority:         types.SeverityHigh,
			Type:             "fix",
			Tags:             []string{"validation", "dockerfile", "errors"},
			ActionType:       "immediate",
			Benefits:         []string{"Valid Dockerfile", "Successful builds"},
			Risks:            []string{"None"},
			Urgency:          "immediate",
			Effort:           "low",
			Impact:           types.SeverityHigh,
			Confidence:       100,
		})
	}

	// Warning recommendations
	if len(r.Warnings) > 5 {
		recommendations = append(recommendations, ai_context.Recommendation{
			RecommendationID: fmt.Sprintf("best-practices-%s", r.SessionID),
			Title:            "Follow Docker Best Practices",
			Description:      "Implement Docker best practices for better maintainability",
			Category:         "quality",
			Priority:         types.SeverityMedium,
			Type:             "improvement",
			Tags:             []string{"best-practices", "dockerfile", "quality"},
			ActionType:       "soon",
			Benefits:         []string{"Better maintainability", "Improved performance", "Reduced image size"},
			Risks:            []string{"Build changes required"},
			Urgency:          "soon",
			Effort:           "low",
			Impact:           types.SeverityMedium,
			Confidence:       85,
		})
	}

	// Optimization recommendations
	if len(r.OptimizationTips) > 0 {
		recommendations = append(recommendations, ai_context.Recommendation{
			RecommendationID: fmt.Sprintf("optimizations-%s", r.SessionID),
			Title:            "Apply Dockerfile Optimizations",
			Description:      "Implement suggested optimizations for better performance",
			Category:         "performance",
			Priority:         types.SeverityLow,
			Type:             "optimization",
			Tags:             []string{"optimization", "dockerfile", "performance"},
			ActionType:       "when_convenient",
			Benefits:         []string{"Smaller image size", "Faster builds", "Better caching"},
			Risks:            []string{"Minimal"},
			Urgency:          "low",
			Effort:           "medium",
			Impact:           types.SeverityMedium,
			Confidence:       80,
		})
	}

	return recommendations
}

// CreateRemediationPlan implements ai_context.Recommendable
func (r *AtomicValidateDockerfileResult) CreateRemediationPlan() *ai_context.RemediationPlan {
	if r.IsValid && len(r.SecurityIssues) == 0 {
		return &ai_context.RemediationPlan{
			PlanID:          fmt.Sprintf("dockerfile-optimization-%s", r.SessionID),
			Title:           "Dockerfile Optimization",
			Description:     "Optimize Dockerfile for better performance and best practices",
			Priority:        types.SeverityLow,
			Complexity:      "simple",
			EstimatedEffort: "30 minutes",
			Steps: []ai_context.RemediationStep{
				{
					StepID:         "apply-optimizations",
					Title:          "Apply Suggested Optimizations",
					Description:    "Implement optimization suggestions",
					Action:         "Review and apply optimization recommendations",
					ExpectedResult: "Improved Dockerfile efficiency",
				},
			},
			ValidationSteps: []ai_context.ValidationStep{
				{
					StepID:         "verify-build",
					Description:    "Dockerfile builds successfully",
					Method:         "automated",
					Command:        "docker build -t test:latest .",
					ExpectedResult: "Build completes without errors",
				},
				{
					StepID:         "verify-optimizations",
					Description:    "Optimizations applied",
					Method:         "manual",
					ExpectedResult: "All suggested optimizations implemented",
				},
			},
		}
	}

	return &ai_context.RemediationPlan{
		PlanID:          fmt.Sprintf("dockerfile-fixes-%s", r.SessionID),
		Title:           "Dockerfile Issue Resolution",
		Description:     "Fix critical issues in Dockerfile",
		Priority:        types.SeverityHigh,
		Complexity:      "moderate",
		EstimatedEffort: "1 hour",
		Steps: []ai_context.RemediationStep{
			{
				StepID:         "fix-syntax",
				Title:          "Fix Syntax Errors",
				Description:    "Correct Dockerfile syntax issues",
				Action:         "Review and fix syntax errors",
				ExpectedResult: "Valid Dockerfile syntax",
			},
			{
				StepID:         "fix-security",
				Title:          "Address Security Issues",
				Description:    "Fix identified security vulnerabilities",
				Action:         "Implement security fixes",
				ExpectedResult: "Secure Dockerfile configuration",
			},
		},
		ValidationSteps: []ai_context.ValidationStep{
			{
				StepID:         "verify-validation",
				Description:    "Validation passes",
				Method:         "tool",
				ToolCall:       "validate_dockerfile",
				ExpectedResult: "No validation errors",
			},
			{
				StepID:         "verify-security",
				Description:    "No security issues",
				Method:         "automated",
				Command:        "docker scan",
				ExpectedResult: "No security issues detected",
			},
		},
	}
}

// GetAlternativeStrategies implements ai_context.Recommendable
func (r *AtomicValidateDockerfileResult) GetAlternativeStrategies() []ai_context.AlternativeStrategy {
	strategies := make([]ai_context.AlternativeStrategy, 0)

	strategies = append(strategies, ai_context.AlternativeStrategy{
		StrategyID:  "dockerfile-templates",
		Name:        "Use Dockerfile Templates",
		Description: "Start with proven Dockerfile templates",
		Complexity:  "simple",
		Timeline:    "immediate",
		Suitability: "best_for_beginners",
		Benefits:    []string{"Proven patterns", "Best practices included", "Faster development"},
		Drawbacks:   []string{"Less customization", "Template dependencies"},
		BestFor:     []string{"Standard application patterns"},
		AvoidIf:     []string{"Highly customized requirements"},
		RiskLevel:   "low",
		Confidence:  80,
	})

	return strategies
}

// GetAIContext implements *ai_context.ToolContextEnriched
// GetAIContext is now provided by embedded mcptypes.BaseAIContextResult

// EnrichWithInsights implements *ai_context.ToolContextEnriched
// EnrichWithInsights is now provided by embedded mcptypes.BaseAIContextResult
// GetMetadataForAI is now provided by embedded mcptypes.BaseAIContextResult

func (r *AtomicValidateDockerfileResult) convertStrengthsToAreas() []ai_context.AssessmentArea {
	areas := make([]ai_context.AssessmentArea, 0)
	strengths := r.GetStrengths()

	for i, strength := range strengths {
		areas = append(areas, ai_context.AssessmentArea{
			Area:        fmt.Sprintf("validation_strength_%d", i+1),
			Category:    "quality",
			Description: strength,
			Impact:      "medium",
			Evidence:    []string{strength},
			Score:       75 + (i * 5),
		})
	}

	return areas
}

func (r *AtomicValidateDockerfileResult) convertChallengesToAreas() []ai_context.AssessmentArea {
	areas := make([]ai_context.AssessmentArea, 0)
	challenges := r.GetChallenges()

	for i, challenge := range challenges {
		impact := "medium"
		if strings.Contains(strings.ToLower(challenge), "security") {
			impact = "high"
		}

		areas = append(areas, ai_context.AssessmentArea{
			Area:        fmt.Sprintf("validation_challenge_%d", i+1),
			Category:    "quality",
			Description: challenge,
			Impact:      impact,
			Evidence:    []string{challenge},
			Score:       25 + (i * 5),
		})
	}

	return areas
}

func (r *AtomicValidateDockerfileResult) extractRiskFactors() []ai_context.RiskFactor {
	risks := make([]ai_context.RiskFactor, 0)

	if !r.IsValid {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "Invalid Dockerfile syntax",
			Category:       "technical",
			Likelihood:     "high",
			Impact:         "high",
			CurrentLevel:   types.SeverityHigh,
			Mitigation:     "Fix syntax errors in Dockerfile",
			PreventionTips: []string{"Use linting tools", "Regular validation"},
		})
	}

	if len(r.SecurityIssues) > 0 {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "Security vulnerabilities in Dockerfile",
			Category:       "security",
			Likelihood:     "medium",
			Impact:         "high",
			CurrentLevel:   types.SeverityHigh,
			Mitigation:     "Address identified security issues",
			PreventionTips: []string{"Security scanning", "Best practice adherence"},
		})
	}

	if r.CriticalIssues > 0 {
		risks = append(risks, ai_context.RiskFactor{
			Risk:           "Critical validation errors",
			Category:       "technical",
			Likelihood:     "high",
			Impact:         "high",
			CurrentLevel:   types.SeverityHigh,
			Mitigation:     "Fix critical issues before proceeding",
			PreventionTips: []string{"Code review", "Automated validation"},
		})
	}

	return risks
}

func (r *AtomicValidateDockerfileResult) getRecommendedApproach() string {
	if !r.IsValid {
		return "Fix validation issues and retry"
	}

	if !r.IsValid {
		return "Fix syntax errors before proceeding"
	}

	if len(r.SecurityIssues) > 0 {
		return "Address security issues before building"
	}

	if r.CriticalIssues > 0 {
		return "Fix critical issues before building"
	}

	return "Proceed with image build - Dockerfile validated successfully"
}

func (r *AtomicValidateDockerfileResult) getNextSteps() []string {
	steps := make([]string, 0)

	if !r.IsValid {
		steps = append(steps, "Fix validation failures")
		return steps
	}

	if !r.IsValid {
		steps = append(steps, "Fix Dockerfile syntax errors")
	}

	if len(r.SecurityIssues) > 0 {
		steps = append(steps, "Address security issues")
	}

	if len(r.Warnings) > 5 {
		steps = append(steps, "Consider fixing best practice violations")
	}

	if len(r.OptimizationTips) > 0 {
		steps = append(steps, "Review optimization suggestions")
	}

	steps = append(steps, "Proceed with container image build")

	return steps
}

func (r *AtomicValidateDockerfileResult) getConsiderationsNote() string {
	considerations := make([]string, 0)

	if !r.IsValid {
		return "Validation failed - check Dockerfile and tool configuration"
	}

	if !r.IsValid {
		considerations = append(considerations, "syntax errors present")
	}
	if len(r.SecurityIssues) > 0 {
		considerations = append(considerations, "security issues detected")
	}
	if len(r.Warnings) > 3 {
		considerations = append(considerations, "many best practice violations")
	}

	if len(considerations) > 0 {
		return fmt.Sprintf("Consider: %s", strings.Join(considerations, ", "))
	}

	return "Dockerfile validation successful - ready for build"
}

// processValidationResults processes the validation results from the core validator
func (t *AtomicValidateDockerfileTool) processValidationResults(result *AtomicValidateDockerfileResult, validationResult *coredocker.ValidationResult, args AtomicValidateDockerfileArgs) {
	// Process errors
	for _, err := range validationResult.Errors {
		dockerfileErr := DockerfileValidationError{
			Type:        err.Type,
			Line:        err.Line,
			Column:      err.Column,
			Rule:        "", // Core validator doesn't provide rules
			Message:     err.Message,
			Instruction: err.Instruction,
			Severity:    err.Severity,
		}

		// Check if this is a security issue
		if err.Type == "security" || strings.Contains(strings.ToLower(err.Message), "security") {
			result.SecurityIssues = append(result.SecurityIssues, DockerfileSecurityIssue{
				Type:        err.Type,
				Line:        err.Line,
				Severity:    err.Severity,
				Description: err.Message,
				Remediation: "Review and fix the security issue",
			})
			result.CriticalIssues++
		} else {
			result.Errors = append(result.Errors, dockerfileErr)
			if err.Severity == "error" {
				result.CriticalIssues++
			}
		}
	}

	// Process warnings
	for _, warn := range validationResult.Warnings {
		result.Warnings = append(result.Warnings, DockerfileValidationWarning{
			Type:       warn.Type,
			Line:       warn.Line,
			Rule:       "", // Core validator doesn't provide rules
			Message:    warn.Message,
			Suggestion: warn.Suggestion,
			Impact:     determineImpact(warn.Type),
		})
	}

	// Add suggestions
	result.Suggestions = validationResult.Suggestions

	// Set total issues
	result.TotalIssues = len(result.Errors) + len(result.Warnings) + len(result.SecurityIssues)

	// Add validation context
	if validationResult.Context != nil {
		for k, v := range validationResult.Context {
			result.ValidationContext[k] = v
		}
	}
}

// performAdditionalAnalysis performs additional security, optimization, and best practice checks
func (t *AtomicValidateDockerfileTool) performAdditionalAnalysis(result *AtomicValidateDockerfileResult, dockerfileContent string, args AtomicValidateDockerfileArgs) {
	lines := strings.Split(dockerfileContent, "\n")

	// Base image analysis
	result.BaseImageAnalysis = t.analyzeBaseImage(lines)

	// Layer analysis
	result.LayerAnalysis = t.analyzeDockerfileLayers(lines)

	// Security analysis
	if args.CheckSecurity {
		result.SecurityAnalysis = t.performSecurityAnalysis(lines)
	}

	// Optimization tips
	if args.CheckOptimization {
		result.OptimizationTips = t.generateOptimizationTips(lines, result.LayerAnalysis)
	}
}

// generateCorrectedDockerfile generates a corrected version of the Dockerfile
func (t *AtomicValidateDockerfileTool) generateCorrectedDockerfile(dockerfileContent string, validationResult *coredocker.ValidationResult) (string, []string) {
	fixes := make([]string, 0)
	lines := strings.Split(dockerfileContent, "\n")
	corrected := make([]string, len(lines))
	copy(corrected, lines)

	// Apply automatic fixes for common issues
	for i, line := range corrected {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Fix missing FROM instruction
		if i == 0 && !strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			corrected = append([]string{"FROM alpine:latest"}, corrected...)
			fixes = append(fixes, "Added missing FROM instruction")
			continue
		}

		// Fix apt-get without update
		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
			corrected[i] = strings.Replace(line, "apt-get install", "apt-get update && apt-get install", 1)
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt-get update before install", lineNum))
		}

		// Fix missing cache cleanup for apt
		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
			corrected[i] = line + " && rm -rf /var/lib/apt/lists/*"
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt cache cleanup", lineNum))
		}

		// Fix running as root (add non-root user at the end if missing)
		if i == len(lines)-1 && !containsUserInstruction(corrected) {
			corrected = append(corrected, "", "# Create non-root user", "RUN adduser -D appuser", "USER appuser")
			fixes = append(fixes, "Added non-root user for security")
		}
	}

	return strings.Join(corrected, "\n"), fixes
}

// calculateValidationScore calculates a validation score based on various factors
func (t *AtomicValidateDockerfileTool) calculateValidationScore(result *AtomicValidateDockerfileResult) int {
	score := 100

	// Deduct points for errors
	score -= len(result.Errors) * 10
	score -= result.CriticalIssues * 15

	// Deduct points for security issues
	score -= len(result.SecurityIssues) * 15

	// Deduct points for warnings (less severe)
	score -= len(result.Warnings) * 3

	// Bonus points for following best practices
	if result.SecurityAnalysis.UsesPackagePin {
		score += 5
	}
	if !result.SecurityAnalysis.RunsAsRoot {
		score += 10
	}
	if result.SecurityAnalysis.SecurityScore > 80 {
		score += 5
	}

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// Helper functions for additional analysis

func (t *AtomicValidateDockerfileTool) analyzeBaseImage(lines []string) BaseImageAnalysis {
	analysis := BaseImageAnalysis{
		Recommendations: make([]string, 0),
		Alternatives:    make([]string, 0),
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				analysis.Image = parts[1]

				// Parse registry and check if trusted
				if strings.Contains(analysis.Image, "/") {
					analysis.Registry = strings.Split(analysis.Image, "/")[0]
					analysis.IsTrusted = isTrustedRegistry(analysis.Registry)
				} else {
					analysis.Registry = "docker.io"
					analysis.IsTrusted = true
				}

				// Check if official image
				analysis.IsOfficial = isOfficialImage(analysis.Image)

				// Check for latest tag
				if strings.Contains(analysis.Image, ":latest") || !strings.Contains(analysis.Image, ":") {
					analysis.Recommendations = append(analysis.Recommendations, "Use specific version tags instead of 'latest'")
					analysis.HasKnownVulns = true // Assume latest might have vulns
				}

				// Suggest alternatives for common images
				analysis.Alternatives = suggestAlternativeImages(analysis.Image)
			}
			break
		}
	}

	return analysis
}

func (t *AtomicValidateDockerfileTool) analyzeDockerfileLayers(lines []string) LayerAnalysis {
	analysis := LayerAnalysis{
		ProblematicSteps: make([]ProblematicStep, 0),
		Optimizations:    make([]LayerOptimization, 0),
	}

	runCommands := 0
	cacheableSteps := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "RUN") {
			runCommands++
			analysis.TotalLayers++

			// Check for cache-breaking commands
			if !strings.Contains(trimmed, "apt-get update") && !strings.Contains(trimmed, "npm install") {
				cacheableSteps++
			}

			// Check for problematic patterns
			if strings.Count(trimmed, "&&") == 0 && runCommands > 1 {
				analysis.ProblematicSteps = append(analysis.ProblematicSteps, ProblematicStep{
					Line:        i + 1,
					Instruction: "RUN",
					Issue:       "Multiple RUN commands can be combined",
					Impact:      "Larger image size due to additional layers",
				})
			}
		} else if strings.HasPrefix(strings.ToUpper(trimmed), "COPY") || strings.HasPrefix(strings.ToUpper(trimmed), "ADD") {
			analysis.TotalLayers++
			cacheableSteps++
		}
	}

	analysis.CacheableSteps = cacheableSteps

	// Suggest layer optimizations
	if runCommands > 3 {
		analysis.Optimizations = append(analysis.Optimizations, LayerOptimization{
			Type:        "layer_consolidation",
			Description: "Combine multiple RUN commands",
			Before:      "RUN cmd1\nRUN cmd2\nRUN cmd3",
			After:       "RUN cmd1 && \\\n    cmd2 && \\\n    cmd3",
			Benefit:     "Reduces image layers and size",
		})
	}

	return analysis
}

func (t *AtomicValidateDockerfileTool) performSecurityAnalysis(lines []string) SecurityAnalysis {
	analysis := SecurityAnalysis{
		ExposedPorts:    make([]int, 0),
		Recommendations: make([]string, 0),
	}

	hasUser := false
	analysis.UsesPackagePin = true // Assume true until proven otherwise
	securityScore := 100

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for USER instruction
		if strings.HasPrefix(upper, "USER") && !strings.Contains(trimmed, "root") {
			hasUser = true
		}

		// Check for exposed ports
		if strings.HasPrefix(upper, "EXPOSE") {
			parts := strings.Fields(trimmed)
			for _, part := range parts[1:] {
				if port, err := strconv.Atoi(strings.TrimSuffix(part, "/tcp")); err == nil {
					analysis.ExposedPorts = append(analysis.ExposedPorts, port)
				}
			}
		}

		// Check for secrets
		if strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY") {
			analysis.HasSecrets = true
			analysis.Recommendations = append(analysis.Recommendations, "Avoid hardcoding secrets in Dockerfile")
			securityScore -= 30
		}

		// Check for package pinning
		if strings.Contains(trimmed, "apt-get install") && !strings.Contains(trimmed, "=") {
			analysis.UsesPackagePin = false
			securityScore -= 10
		}
	}

	analysis.RunsAsRoot = !hasUser
	if analysis.RunsAsRoot {
		analysis.Recommendations = append(analysis.Recommendations, "Add a non-root user for better security")
		securityScore -= 20
	}

	analysis.SecurityScore = securityScore
	if analysis.SecurityScore < 0 {
		analysis.SecurityScore = 0
	}

	return analysis
}

func (t *AtomicValidateDockerfileTool) generateOptimizationTips(lines []string, layerAnalysis LayerAnalysis) []OptimizationTip {
	tips := make([]OptimizationTip, 0)

	// Check for layer optimization opportunities
	if layerAnalysis.TotalLayers > 10 {
		tips = append(tips, OptimizationTip{
			Type:             "layer_consolidation",
			Description:      "Too many layers detected",
			Impact:           "size_reduction",
			Suggestion:       "Combine related RUN commands using && to reduce layers",
			EstimatedSavings: "10-20% size reduction",
		})
	}

	// Check for cache optimization
	copyBeforeRun := false
	lastCopyLine := -1
	lastRunLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "COPY") {
			lastCopyLine = i
		} else if strings.HasPrefix(trimmed, "RUN") {
			lastRunLine = i
			if lastCopyLine > lastRunLine {
				copyBeforeRun = true
			}
		}
	}

	if copyBeforeRun {
		tips = append(tips, OptimizationTip{
			Type:        "cache_optimization",
			Line:        lastCopyLine + 1,
			Description: "COPY after RUN breaks Docker cache",
			Impact:      "build_speed",
			Suggestion:  "Move COPY commands before RUN commands when possible",
		})
	}

	return tips
}

// Helper utility functions

func determineImpact(warningType string) string {
	switch warningType {
	case "security":
		return "security"
	case "best_practice":
		return "maintainability"
	default:
		return "performance"
	}
}

func isTrustedRegistry(registry string) bool {
	trustedRegistries := constants.KnownRegistries

	for _, trusted := range trustedRegistries {
		if registry == trusted {
			return true
		}
	}
	return false
}

func isOfficialImage(image string) bool {
	// Official images don't have a username/organization prefix
	parts := strings.Split(image, "/")
	return len(parts) == 1 || (len(parts) == 2 && parts[0] == "library")
}

func suggestAlternativeImages(image string) []string {
	alternatives := make([]string, 0)

	baseImage := strings.Split(image, ":")[0]
	switch {
	case strings.Contains(baseImage, "ubuntu"):
		alternatives = append(alternatives, "debian:slim", "alpine:latest")
	case strings.Contains(baseImage, "debian"):
		alternatives = append(alternatives, "debian:slim", "alpine:latest")
	case strings.Contains(baseImage, "centos"):
		alternatives = append(alternatives, "rockylinux:minimal", "almalinux:minimal")
	case strings.Contains(baseImage, "node"):
		alternatives = append(alternatives, "node:alpine", "node:slim")
	}

	return alternatives
}

func containsUserInstruction(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "USER") {
			return true
		}
	}
	return false
}

// SimpleTool interface implementation

// GetName returns the tool name
func (t *AtomicValidateDockerfileTool) GetName() string {
	return "atomic_validate_dockerfile"
}

// GetDescription returns the tool description
func (t *AtomicValidateDockerfileTool) GetDescription() string {
	return "Validates Dockerfiles against best practices, security standards, and optimization guidelines"
}

// GetVersion returns the tool version
func (t *AtomicValidateDockerfileTool) GetVersion() string {
	return constants.AtomicToolVersion
}

// GetCapabilities returns the tool capabilities
func (t *AtomicValidateDockerfileTool) GetCapabilities() types.ToolCapabilities {
	return types.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

// GetMetadata returns comprehensive metadata about the tool
func (t *AtomicValidateDockerfileTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "atomic_validate_dockerfile",
		Description: "Validates Dockerfiles against best practices, security standards, and optimization guidelines with automatic fix generation",
		Version:     "1.0.0",
		Category:    "validation",
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
			"dockerfile_content_or_path",
		},
		Parameters: map[string]string{
			"session_id":           "string - Session ID for session context",
			"dockerfile_path":      "string - Path to Dockerfile (default: session workspace/Dockerfile)",
			"dockerfile_content":   "string - Dockerfile content to validate (alternative to path)",
			"use_hadolint":         "bool - Use Hadolint for advanced validation",
			"severity":             "string - Minimum severity to report (info, warning, error)",
			"ignore_rules":         "[]string - Hadolint rules to ignore (e.g., DL3008, DL3009)",
			"trusted_registries":   "[]string - List of trusted registries for base image validation",
			"check_security":       "bool - Perform security-focused checks",
			"check_optimization":   "bool - Check for image size optimization opportunities",
			"check_best_practices": "bool - Validate against Docker best practices",
			"include_suggestions":  "bool - Include remediation suggestions",
			"generate_fixes":       "bool - Generate corrected Dockerfile",
			"dry_run":              "bool - Validate without making changes",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Basic Dockerfile Validation",
				Description: "Validate a Dockerfile for syntax and basic issues",
				Input: map[string]interface{}{
					"session_id":           "session-123",
					"dockerfile_path":      "/workspace/Dockerfile",
					"check_best_practices": true,
				},
				Output: map[string]interface{}{
					"success":          true,
					"is_valid":         true,
					"validation_score": 85,
					"total_issues":     2,
					"critical_issues":  0,
					"validator_used":   "basic",
				},
			},
			{
				Name:        "Advanced Security Validation",
				Description: "Comprehensive validation with security and optimization checks",
				Input: map[string]interface{}{
					"session_id":           "session-456",
					"use_hadolint":         true,
					"check_security":       true,
					"check_optimization":   true,
					"check_best_practices": true,
					"include_suggestions":  true,
					"trusted_registries": []string{
						"docker.io",
						"gcr.io",
						"registry.access.redhat.com",
					},
				},
				Output: map[string]interface{}{
					"success":           true,
					"is_valid":          false,
					"validation_score":  45,
					"total_issues":      8,
					"critical_issues":   2,
					"security_issues":   3,
					"optimization_tips": 5,
					"validator_used":    "hadolint",
				},
			},
			{
				Name:        "Validation with Fix Generation",
				Description: "Validate Dockerfile and generate corrected version",
				Input: map[string]interface{}{
					"session_id":          "session-789",
					"dockerfile_content":  "FROM ubuntu\nRUN apt-get install -y curl\nUSER root",
					"generate_fixes":      true,
					"check_security":      true,
					"include_suggestions": true,
				},
				Output: map[string]interface{}{
					"success":              true,
					"is_valid":             false,
					"validation_score":     30,
					"total_issues":         4,
					"fixes_applied":        []string{"Added apt-get update", "Added cache cleanup", "Added non-root user"},
					"corrected_dockerfile": "FROM ubuntu:20.04\nRUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*\nRUN adduser -D appuser\nUSER appuser",
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicValidateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return types.NewValidationErrorBuilder("Invalid argument type for atomic_validate_dockerfile", "args", args).
			WithField("expected", "AtomicValidateDockerfileArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	if validateArgs.SessionID == "" {
		return types.NewValidationErrorBuilder("SessionID is required", "session_id", validateArgs.SessionID).
			WithField("field", "session_id").
			Build()
	}

	// Must provide either path or content
	if validateArgs.DockerfilePath == "" && validateArgs.DockerfileContent == "" {
		return types.NewValidationErrorBuilder("Either dockerfile_path or dockerfile_content must be provided", "dockerfile", "").
			WithField("dockerfile_path", validateArgs.DockerfilePath).
			WithField("has_content", validateArgs.DockerfileContent != "").
			Build()
	}

	// Validate severity if provided
	if validateArgs.Severity != "" {
		validSeverities := map[string]bool{
			"info": true, "warning": true, "error": true,
		}
		if !validSeverities[strings.ToLower(validateArgs.Severity)] {
			return types.NewValidationErrorBuilder("Invalid severity level", "severity", validateArgs.Severity).
				WithField("valid_values", "info, warning, error").
				Build()
		}
	}

	return nil
}

// Execute implements SimpleTool interface with generic signature
func (t *AtomicValidateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return nil, types.NewValidationErrorBuilder("Invalid argument type for atomic_validate_dockerfile", "args", args).
			WithField("expected", "AtomicValidateDockerfileArgs").
			WithField("received", fmt.Sprintf("%T", args)).
			Build()
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, validateArgs)
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicValidateDockerfileTool) ExecuteTyped(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	return t.ExecuteValidation(ctx, args)
}

// SetAnalyzer enables AI-driven fixing capabilities by providing an analyzer
func (t *AtomicValidateDockerfileTool) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	if analyzer != nil {
		t.fixingMixin = fixing.NewAtomicToolFixingMixin(analyzer, "validate_dockerfile_atomic", t.logger)
	}
}
