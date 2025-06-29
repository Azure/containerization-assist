package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	constants "github.com/Azure/container-kit/pkg/mcp/internal/types"

	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type AtomicValidateDockerfileArgs struct {
	types.BaseToolArgs

	DockerfilePath    string `json:"dockerfile_path,omitempty" description:"Path to Dockerfile (default: session workspace/Dockerfile)"`
	DockerfileContent string `json:"dockerfile_content,omitempty" description:"Dockerfile content to validate (alternative to path)"`

	UseHadolint       bool     `json:"use_hadolint,omitempty" description:"Use Hadolint for advanced validation"`
	Severity          string   `json:"severity,omitempty" description:"Minimum severity to report (info, warning, error)"`
	IgnoreRules       []string `json:"ignore_rules,omitempty" description:"Hadolint rules to ignore (e.g., DL3008, DL3009)"`
	TrustedRegistries []string `json:"trusted_registries,omitempty" description:"List of trusted registries for base image validation"`

	CheckSecurity      bool `json:"check_security,omitempty" description:"Perform security-focused checks"`
	CheckOptimization  bool `json:"check_optimization,omitempty" description:"Check for image size optimization opportunities"`
	CheckBestPractices bool `json:"check_best_practices,omitempty" description:"Validate against Docker best practices"`

	IncludeSuggestions bool `json:"include_suggestions,omitempty" description:"Include remediation suggestions"`
	GenerateFixes      bool `json:"generate_fixes,omitempty" description:"Generate corrected Dockerfile"`
}

type AtomicValidateDockerfileResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult

	SessionID      string        `json:"session_id"`
	DockerfilePath string        `json:"dockerfile_path"`
	Duration       time.Duration `json:"duration"`
	ValidatorUsed  string        `json:"validator_used"` // hadolint, basic, hybrid

	IsValid         bool `json:"is_valid"`
	ValidationScore int  `json:"validation_score"` // 0-100
	TotalIssues     int  `json:"total_issues"`
	CriticalIssues  int  `json:"critical_issues"`

	Errors           []DockerfileValidationError   `json:"errors"`
	Warnings         []DockerfileValidationWarning `json:"warnings"`
	SecurityIssues   []DockerfileSecurityIssue     `json:"security_issues"`
	OptimizationTips []OptimizationTip             `json:"optimization_tips"`

	BaseImageAnalysis BaseImageAnalysis `json:"base_image_analysis"`
	LayerAnalysis     LayerAnalysis     `json:"layer_analysis"`
	SecurityAnalysis  SecurityAnalysis  `json:"security_analysis"`

	Suggestions         []string `json:"suggestions"`
	CorrectedDockerfile string   `json:"corrected_dockerfile,omitempty"`
	FixesApplied        []string `json:"fixes_applied,omitempty"`

	ValidationContext map[string]interface{} `json:"validation_context"`
}

type Recommendation struct {
	RecommendationID string   `json:"recommendation_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Priority         string   `json:"priority"`
	Type             string   `json:"type"`
	Tags             []string `json:"tags"`
	ActionType       string   `json:"action_type"`
	Effort           string   `json:"effort"`
	Impact           string   `json:"impact"`
	Confidence       int      `json:"confidence"`
	Benefits         []string `json:"benefits"`
	Risks            []string `json:"risks"`
	Urgency          string   `json:"urgency"`
}

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

type DockerfileValidationWarning struct {
	Type       string `json:"type"`
	Line       int    `json:"line"`
	Rule       string `json:"rule,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Impact     string `json:"impact,omitempty"` // performance, security, maintainability
}

type DockerfileSecurityIssue struct {
	Type          string   `json:"type"` // exposed_port, root_user, secrets, etc.
	Line          int      `json:"line"`
	Severity      string   `json:"severity"` // low, medium, high, critical
	Description   string   `json:"description"`
	Remediation   string   `json:"remediation"`
	CVEReferences []string `json:"cve_references,omitempty"`
}

type OptimizationTip struct {
	Type             string `json:"type"` // layer_consolidation, cache_optimization, etc.
	Line             int    `json:"line,omitempty"`
	Description      string `json:"description"`
	Impact           string `json:"impact"` // size_reduction, build_speed, etc.
	Suggestion       string `json:"suggestion"`
	EstimatedSavings string `json:"estimated_savings,omitempty"` // e.g., "50MB", "30% faster"
}

type BaseImageAnalysis struct {
	Image           string   `json:"image"`
	Registry        string   `json:"registry"`
	IsTrusted       bool     `json:"is_trusted"`
	IsOfficial      bool     `json:"is_official"`
	HasKnownVulns   bool     `json:"has_known_vulnerabilities"`
	Alternatives    []string `json:"alternatives,omitempty"`
	Recommendations []string `json:"recommendations"`
}

type LayerAnalysis struct {
	TotalLayers      int                 `json:"total_layers"`
	CacheableSteps   int                 `json:"cacheable_steps"`
	ProblematicSteps []ProblematicStep   `json:"problematic_steps"`
	Optimizations    []LayerOptimization `json:"optimizations"`
}

type ProblematicStep struct {
	Line        int    `json:"line"`
	Instruction string `json:"instruction"`
	Issue       string `json:"issue"`
	Impact      string `json:"impact"`
}

type LayerOptimization struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Benefit     string `json:"benefit"`
}

type SecurityAnalysis struct {
	RunsAsRoot      bool     `json:"runs_as_root"`
	ExposedPorts    []int    `json:"exposed_ports"`
	HasSecrets      bool     `json:"has_secrets"`
	UsesPackagePin  bool     `json:"uses_package_pinning"`
	SecurityScore   int      `json:"security_score"` // 0-100
	Recommendations []string `json:"recommendations"`
}

type AtomicValidateDockerfileTool struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
	analyzer        ToolAnalyzer
	fixingMixin     *build.AtomicToolFixingMixin
}

func NewAtomicValidateDockerfileTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicValidateDockerfileTool {
	toolLogger := logger.With().Str("tool", "atomic_validate_dockerfile").Logger()
	return &AtomicValidateDockerfileTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

func (t *AtomicValidateDockerfileTool) SetAnalyzer(analyzer ToolAnalyzer) {
	t.analyzer = analyzer
}

func (t *AtomicValidateDockerfileTool) SetFixingMixin(mixin *build.AtomicToolFixingMixin) {
	t.fixingMixin = mixin
}

func (t *AtomicValidateDockerfileTool) ExecuteValidation(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

func (t *AtomicValidateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	// Progress tracking removed for simplification

	ctx := context.Background()
	result, err := t.performValidation(ctx, args, nil)

	if err != nil {
		t.logger.Info().Msg("Validation failed")
		return result, nil
	} else {
		t.logger.Info().Msg("Validation completed successfully")
	}

	return result, nil
}

func (t *AtomicValidateDockerfileTool) executeWithoutProgress(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	return t.performValidation(ctx, args, nil)
}

func (t *AtomicValidateDockerfileTool) performValidation(ctx context.Context, args AtomicValidateDockerfileArgs, reporter interface{}) (*AtomicValidateDockerfileResult, error) {
	startTime := time.Now()

	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		result := &AtomicValidateDockerfileResult{
			BaseToolResponse:    types.NewBaseResponse("atomic_validate_dockerfile", args.SessionID, args.DryRun),
			BaseAIContextResult: core.NewBaseAIContextResult("validate", false, 0), // Will be updated later
			Duration:            time.Since(startTime),
		}

		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get session")
		return result, nil
	}
	session := sessionInterface.(*core.SessionState)

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("dockerfile_path", args.DockerfilePath).
		Bool("use_hadolint", args.UseHadolint).
		Msg("Starting atomic Dockerfile validation")

	result := &AtomicValidateDockerfileResult{
		BaseToolResponse:    types.NewBaseResponse("atomic_validate_dockerfile", session.SessionID, args.DryRun),
		BaseAIContextResult: core.NewBaseAIContextResult("validate", false, 0),
		ValidationContext:   make(map[string]interface{}),
	}
	var dockerfilePath string
	var dockerfileContent string

	if args.DockerfileContent != "" {
		dockerfileContent = args.DockerfileContent
		dockerfilePath = types.ValidationModeInline
	} else {
		if args.DockerfilePath != "" {
			dockerfilePath = args.DockerfilePath
		} else {
			workspaceDir := t.pipelineAdapter.GetSessionWorkspace(session.SessionID)
			dockerfilePath = filepath.Join(workspaceDir, "Dockerfile")
		}

		content, err := os.ReadFile(dockerfilePath)
		if err != nil {
			t.logger.Error().Err(err).Str("dockerfile_path", result.DockerfilePath).Msg("Failed to read Dockerfile")
			result.Duration = time.Since(startTime)
			return result, nil
		}
		dockerfileContent = string(content)
	}

	result.DockerfilePath = dockerfilePath

	// Direct validation implementation (adapter removed)
	var validationResult *coredocker.ValidationResult
	var validatorUsed string

	if args.UseHadolint {

		hadolintValidator := coredocker.NewHadolintValidator(t.logger)
		validationResult, err = hadolintValidator.ValidateWithHadolint(ctx, dockerfileContent)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Hadolint validation failed, falling back to basic validation")
			validatorUsed = "basic_fallback"
		} else {
			validatorUsed = "hadolint"
		}
	}

	if validationResult == nil {

		basicValidator := coredocker.NewValidator(t.logger)
		validationResult = basicValidator.ValidateDockerfile(dockerfileContent)
		if validatorUsed == "" {
			validatorUsed = "basic"
		}
	}

	result.ValidatorUsed = validatorUsed
	result.IsValid = validationResult.Valid

	t.processValidationResults(result, validationResult, args)

	if args.CheckSecurity || args.CheckOptimization || args.CheckBestPractices {

		t.performAdditionalAnalysis(result, dockerfileContent, args)

	}

	if args.GenerateFixes && !result.IsValid {

		correctedDockerfile, fixes := t.generateCorrectedDockerfile(dockerfileContent, validationResult)
		result.CorrectedDockerfile = correctedDockerfile
		result.FixesApplied = fixes

	}

	result.ValidationScore = t.calculateValidationScore(result)

	result.Duration = time.Since(startTime)

	result.BaseAIContextResult.IsSuccessful = result.IsValid
	result.BaseAIContextResult.Duration = result.Duration

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

func (r *AtomicValidateDockerfileResult) GenerateRecommendations() []Recommendation {
	recommendations := make([]Recommendation, 0)

	if len(r.SecurityIssues) > 0 {
		recommendations = append(recommendations, Recommendation{
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

	if len(r.Errors) > 0 {
		recommendations = append(recommendations, Recommendation{
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

	if len(r.Warnings) > 5 {
		recommendations = append(recommendations, Recommendation{
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

	if len(r.OptimizationTips) > 0 {
		recommendations = append(recommendations, Recommendation{
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

func (r *AtomicValidateDockerfileResult) CreateRemediationPlan() interface{} {
	return map[string]interface{}{
		"plan_id":     fmt.Sprintf("dockerfile-validation-%s", r.SessionID),
		"title":       "Dockerfile Validation Plan",
		"description": "Plan to address Dockerfile validation issues",
		"priority":    "medium",
	}
}

func (r *AtomicValidateDockerfileResult) GetAlternativeStrategies() interface{} {
	return []map[string]interface{}{
		{
			"strategy":    "Use validated base images",
			"description": "Switch to security-scanned base images",
		},
	}
}

func (r *AtomicValidateDockerfileResult) getRecommendedApproach() string {
	if len(r.Errors) > 0 {
		return "Fix syntax and validation errors first"
	}
	if len(r.SecurityIssues) > 0 {
		return "Address security vulnerabilities"
	}
	return "Optimize for production use"
}

func (r *AtomicValidateDockerfileResult) getNextSteps() []string {
	steps := []string{}
	if len(r.Errors) > 0 {
		steps = append(steps, "Fix validation errors")
	}
	if len(r.SecurityIssues) > 0 {
		steps = append(steps, "Address security issues")
	}
	if len(r.OptimizationTips) > 0 {
		steps = append(steps, "Apply optimization recommendations")
	}
	return steps
}

func (r *AtomicValidateDockerfileResult) getConsiderationsNote() string {
	return "Dockerfile validation completed - review recommendations"
}

func (t *AtomicValidateDockerfileTool) processValidationResults(result *AtomicValidateDockerfileResult, validationResult *coredocker.ValidationResult, args AtomicValidateDockerfileArgs) {
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

	result.Suggestions = validationResult.Suggestions

	result.TotalIssues = len(result.Errors) + len(result.Warnings) + len(result.SecurityIssues)

	if validationResult.Context != nil {
		for k, v := range validationResult.Context {
			result.ValidationContext[k] = v
		}
	}
}

func (t *AtomicValidateDockerfileTool) performAdditionalAnalysis(result *AtomicValidateDockerfileResult, dockerfileContent string, args AtomicValidateDockerfileArgs) {
	lines := strings.Split(dockerfileContent, "\n")

	result.BaseImageAnalysis = t.analyzeBaseImage(lines)

	result.LayerAnalysis = t.analyzeDockerfileLayers(lines)

	if args.CheckSecurity {
		result.SecurityAnalysis = t.performSecurityAnalysis(lines)
	}

	if args.CheckOptimization {
		result.OptimizationTips = t.generateOptimizationTips(lines, result.LayerAnalysis)
	}
}

func (t *AtomicValidateDockerfileTool) generateCorrectedDockerfile(dockerfileContent string, validationResult *coredocker.ValidationResult) (string, []string) {
	fixes := make([]string, 0)
	lines := strings.Split(dockerfileContent, "\n")
	corrected := make([]string, len(lines))
	copy(corrected, lines)

	for i, line := range corrected {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		if i == 0 && !strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			corrected = append([]string{"FROM alpine:latest"}, corrected...)
			fixes = append(fixes, "Added missing FROM instruction")
			continue
		}

		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
			corrected[i] = strings.Replace(line, "apt-get install", "apt-get update && apt-get install", 1)
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt-get update before install", lineNum))
		}

		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
			corrected[i] = line + " && rm -rf /var/lib/apt/lists/*"
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt cache cleanup", lineNum))
		}

		if i == len(lines)-1 && !containsUserInstruction(corrected) {
			corrected = append(corrected, "", "# Create non-root user", "RUN adduser -D appuser", "USER appuser")
			fixes = append(fixes, "Added non-root user for security")
		}
	}

	return strings.Join(corrected, "\n"), fixes
}

func (t *AtomicValidateDockerfileTool) calculateValidationScore(result *AtomicValidateDockerfileResult) int {
	score := 100

	score -= len(result.Errors) * 10
	score -= result.CriticalIssues * 15

	score -= len(result.SecurityIssues) * 15

	score -= len(result.Warnings) * 3

	if result.SecurityAnalysis.UsesPackagePin {
		score += 5
	}
	if !result.SecurityAnalysis.RunsAsRoot {
		score += 10
	}
	if result.SecurityAnalysis.SecurityScore > 80 {
		score += 5
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

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

				if strings.Contains(analysis.Image, "/") {
					analysis.Registry = strings.Split(analysis.Image, "/")[0]
					analysis.IsTrusted = isTrustedRegistry(analysis.Registry)
				} else {
					analysis.Registry = "docker.io"
					analysis.IsTrusted = true
				}

				analysis.IsOfficial = isOfficialImage(analysis.Image)

				if strings.Contains(analysis.Image, ":latest") || !strings.Contains(analysis.Image, ":") {
					analysis.Recommendations = append(analysis.Recommendations, "Use specific version tags instead of 'latest'")
					analysis.HasKnownVulns = true // Assume latest might have vulns
				}

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

			if !strings.Contains(trimmed, "apt-get update") && !strings.Contains(trimmed, "npm install") {
				cacheableSteps++
			}

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

		if strings.HasPrefix(upper, "USER") && !strings.Contains(trimmed, "root") {
			hasUser = true
		}

		if strings.HasPrefix(upper, "EXPOSE") {
			parts := strings.Fields(trimmed)
			for _, part := range parts[1:] {
				if port, err := strconv.Atoi(strings.TrimSuffix(part, "/tcp")); err == nil {
					analysis.ExposedPorts = append(analysis.ExposedPorts, port)
				}
			}
		}

		if strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY") {
			analysis.HasSecrets = true
			analysis.Recommendations = append(analysis.Recommendations, "Avoid hardcoding secrets in Dockerfile")
			securityScore -= 30
		}

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

	if layerAnalysis.TotalLayers > 10 {
		tips = append(tips, OptimizationTip{
			Type:             "layer_consolidation",
			Description:      "Too many layers detected",
			Impact:           "size_reduction",
			Suggestion:       "Combine related RUN commands using && to reduce layers",
			EstimatedSavings: "10-20% size reduction",
		})
	}

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

func (t *AtomicValidateDockerfileTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
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
		Examples: []core.ToolExample{
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

func (t *AtomicValidateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return fmt.Errorf("error")
	}

	if validateArgs.SessionID == "" {
		return fmt.Errorf("error")
	}

	// Must provide either path or content
	if validateArgs.DockerfilePath == "" && validateArgs.DockerfileContent == "" {
		return fmt.Errorf("error")
	}

	if validateArgs.Severity != "" {
		validSeverities := map[string]bool{
			"info": true, "warning": true, "error": true,
		}
		if !validSeverities[strings.ToLower(validateArgs.Severity)] {
			return fmt.Errorf("error")
		}
	}

	return nil
}

func (t *AtomicValidateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	validateArgs, ok := args.(AtomicValidateDockerfileArgs)
	if !ok {
		return nil, fmt.Errorf("error")
	}

	return t.ExecuteTyped(ctx, validateArgs)
}

func (t *AtomicValidateDockerfileTool) ExecuteTyped(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	return t.ExecuteValidation(ctx, args)
}
