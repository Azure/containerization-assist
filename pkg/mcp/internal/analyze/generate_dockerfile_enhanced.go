package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// GenerateDockerfileEnhancedTool implements enhanced Dockerfile generation with template integration
type GenerateDockerfileEnhancedTool struct {
	logger              zerolog.Logger
	validator           *coredocker.Validator
	hadolintValidator   *coredocker.HadolintValidator
	sessionManager      mcptypes.ToolSessionManager
	templateIntegration *TemplateIntegration
	templateEngine      *coredocker.TemplateEngine
}

// NewGenerateDockerfileEnhancedTool creates a new instance of GenerateDockerfileEnhancedTool
func NewGenerateDockerfileEnhancedTool(sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *GenerateDockerfileEnhancedTool {
	return &GenerateDockerfileEnhancedTool{
		logger:              logger,
		validator:           coredocker.NewValidator(logger),
		hadolintValidator:   coredocker.NewHadolintValidator(logger),
		sessionManager:      sessionManager,
		templateIntegration: NewTemplateIntegration(logger),
		templateEngine:      coredocker.NewTemplateEngine(logger),
	}
}

// ExecuteTyped generates a Dockerfile based on repository analysis with enhanced template integration
func (t *GenerateDockerfileEnhancedTool) ExecuteTyped(ctx context.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	// Create base response
	response := &GenerateDockerfileResult{
		BaseToolResponse: types.NewBaseResponse("generate_dockerfile", args.SessionID, args.DryRun),
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", args.Template).
		Str("optimization", args.Optimization).
		Bool("dry_run", args.DryRun).
		Msg("Starting enhanced Dockerfile generation")

	// Get session to access repository analysis
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, types.NewRichError("SESSION_ACCESS_FAILED", "failed to get session "+args.SessionID+": "+err.Error(), types.ErrTypeSession)
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, types.NewRichError("INTERNAL_ERROR", "session type assertion failed", "type_error")
	}

	// Use template integration for enhanced template selection
	// Use structured ScanSummary
	var repositoryData map[string]interface{}
	if session.ScanSummary != nil {
		repositoryData = sessiontypes.ConvertScanSummaryToRepositoryInfo(session.ScanSummary)
	}

	templateContext, err := t.templateIntegration.SelectDockerfileTemplate(
		repositoryData,
		args.Template,
	)
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to select template")
		return nil, types.NewRichError("TEMPLATE_SELECTION_FAILED", "template selection failed: "+err.Error(), types.ErrTypeSystem)
	}

	templateName := templateContext.SelectedTemplate

	// Set template selection context in response
	response.TemplateSelection = &TemplateSelectionContext{
		DetectedLanguage:    templateContext.DetectedLanguage,
		DetectedFramework:   templateContext.DetectedFramework,
		AvailableTemplates:  t.convertTemplateOptions(templateContext.AvailableTemplates),
		RecommendedTemplate: templateContext.SelectedTemplate,
		SelectionReasoning:  templateContext.SelectionReasoning,
		AlternativeOptions:  t.convertAlternativeOptions(templateContext.AlternativeOptions),
	}

	t.logger.Info().
		Str("template", templateName).
		Str("method", templateContext.SelectionMethod).
		Float64("confidence", templateContext.SelectionConfidence).
		Msg("Selected Dockerfile template")

	// Handle dry-run mode
	if args.DryRun {
		content, err := t.previewDockerfile(templateName, args, templateContext)
		if err != nil {
			return nil, types.NewRichError("DOCKERFILE_PREVIEW_FAILED", "failed to preview Dockerfile: "+err.Error(), types.ErrTypeBuild)
		}

		response.Content = content
		response.Template = templateName
		response.BuildSteps = t.extractBuildSteps(content)
		response.ExposedPorts = t.extractExposedPorts(content)
		response.BaseImage = t.extractBaseImage(content)

		// Add optimization hints
		response.OptimizationHints = t.generateOptimizationContext(content, args, templateContext)

		return response, nil
	}

	// For actual generation, use workspace directory
	workspaceDir := filepath.Join(os.TempDir(), "container-kit-workspace", session.SessionID)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, types.NewRichError("WORKSPACE_CREATION_FAILED", "failed to create workspace: "+err.Error(), types.ErrTypeSystem)
	}

	// Generate Dockerfile using template engine
	generateResult, err := t.templateEngine.GenerateFromTemplate(templateName, workspaceDir)
	if err != nil {
		return nil, types.NewRichError("DOCKERFILE_GENERATION_FAILED", "failed to generate Dockerfile: "+err.Error(), types.ErrTypeBuild)
	}

	if !generateResult.Success {
		return nil, types.NewRichError("DOCKERFILE_GENERATION_FAILED", "Dockerfile generation failed: "+generateResult.Error.Message, types.ErrTypeBuild)
	}

	// Apply customizations based on args and template context
	content := t.applyCustomizations(generateResult.Dockerfile, args, templateContext)

	// Write the customized Dockerfile
	dockerfilePath := filepath.Join(workspaceDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		return nil, types.NewRichError("DOCKERFILE_WRITE_FAILED", "failed to write Dockerfile: "+err.Error(), types.ErrTypeSystem)
	}

	// Also write .dockerignore if provided
	if generateResult.DockerIgnore != "" {
		dockerignorePath := filepath.Join(workspaceDir, ".dockerignore")
		if err := os.WriteFile(dockerignorePath, []byte(generateResult.DockerIgnore), 0644); err != nil {
			t.logger.Warn().Err(err).Msg("Failed to write .dockerignore")
		}
	}

	// Populate response
	response.Content = content
	response.Template = templateName
	response.FilePath = dockerfilePath
	response.BuildSteps = t.extractBuildSteps(content)
	response.ExposedPorts = t.extractExposedPorts(content)
	response.BaseImage = t.extractBaseImage(content)

	if args.IncludeHealthCheck {
		response.HealthCheck = t.extractHealthCheck(content)
	}

	// Generate optimization context
	response.OptimizationHints = t.generateOptimizationContext(content, args, templateContext)

	// Validate the generated Dockerfile
	validationResult := t.validateDockerfile(ctx, content)
	response.Validation = validationResult

	// Check if validation failed with critical errors
	if validationResult != nil && !validationResult.Valid {
		criticalErrors := 0
		for _, err := range validationResult.Errors {
			if err.Severity == "error" {
				criticalErrors++
			}
		}

		if criticalErrors > 0 {
			t.logger.Error().
				Int("critical_errors", criticalErrors).
				Msg("Dockerfile validation failed with critical errors")

			response.Message = fmt.Sprintf(
				"Dockerfile generated but has %d critical validation errors. Please review and fix before building.",
				criticalErrors)
		}
	}

	// Update session state with generated Dockerfile info
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["dockerfile_template"] = templateName
	session.Metadata["dockerfile_path"] = dockerfilePath
	session.Metadata["dockerfile_generated"] = true

	if err := t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
	}); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", templateName).
		Str("file_path", dockerfilePath).
		Bool("validation_passed", validationResult == nil || validationResult.Valid).
		Msg("Successfully generated Dockerfile with enhanced template integration")

	return response, nil
}

// Helper methods

func (t *GenerateDockerfileEnhancedTool) convertTemplateOptions(options []TemplateOptionInternal) []TemplateOption {
	result := make([]TemplateOption, len(options))
	for i, opt := range options {
		result[i] = TemplateOption{
			Name:        opt.Name,
			Description: opt.Description,
			BestFor:     opt.BestFor,
			Limitations: opt.Limitations,
			MatchScore:  int(opt.MatchScore * 100), // Convert float to int percentage
		}
	}
	return result
}

func (t *GenerateDockerfileEnhancedTool) convertAlternativeOptions(options []AlternativeTemplateOption) []AlternativeTemplate {
	result := make([]AlternativeTemplate, len(options))
	for i, opt := range options {
		result[i] = AlternativeTemplate{
			Template:  opt.Template,
			Reason:    opt.Reason,
			TradeOffs: opt.TradeOffs,
			UseCases:  opt.UseCases,
		}
	}
	return result
}

func (t *GenerateDockerfileEnhancedTool) previewDockerfile(templateName string, args GenerateDockerfileArgs, context *DockerfileTemplateContext) (string, error) {
	// Generate a preview without actually writing files
	preview := fmt.Sprintf(`# Dockerfile generated from template: %s
# Language: %s
# Framework: %s
# Selection Method: %s
# Confidence: %.2f

# This is a preview - actual content will be generated from the template
# Template provides optimized configuration for %s applications

`, templateName, context.DetectedLanguage, context.DetectedFramework,
		context.SelectionMethod, context.SelectionConfidence, context.DetectedLanguage)

	// Add optimization hints
	if args.Optimization != "" {
		preview += fmt.Sprintf("# Optimization: %s\n", args.Optimization)
	}

	// Add base image override
	if args.BaseImage != "" {
		preview += fmt.Sprintf("# Base image override: %s\n", args.BaseImage)
	}

	return preview, nil
}

func (t *GenerateDockerfileEnhancedTool) applyCustomizations(content string, args GenerateDockerfileArgs, context *DockerfileTemplateContext) string {
	// Apply user-requested customizations to the template-generated Dockerfile

	// Override base image if specified
	if args.BaseImage != "" {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM ") {
				// Replace the first FROM instruction
				lines[i] = fmt.Sprintf("FROM %s", args.BaseImage)
				break
			}
		}
		content = strings.Join(lines, "\n")
	}

	// Add health check if requested
	if args.IncludeHealthCheck && !strings.Contains(content, "HEALTHCHECK") {
		healthCheck := t.generateHealthCheck(context.DetectedLanguage, context.DetectedFramework)
		content = strings.TrimRight(content, "\n") + "\n\n" + healthCheck + "\n"
	}

	// Apply optimization hints
	if args.Optimization != "" {
		content = t.applyOptimization(content, args.Optimization, context)
	}

	// Add build args
	if len(args.BuildArgs) > 0 {
		buildArgsSection := "\n# Build arguments\n"
		for key, value := range args.BuildArgs {
			buildArgsSection += fmt.Sprintf("ARG %s=%s\n", key, value)
		}
		// Insert after FROM instruction
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM ") {
				lines[i] = line + buildArgsSection
				break
			}
		}
		content = strings.Join(lines, "\n")
	}

	// Add platform if specified
	if args.Platform != "" {
		content = fmt.Sprintf("# syntax=docker/dockerfile:1\n# platform=%s\n%s", args.Platform, content)
	}

	return content
}

func (t *GenerateDockerfileEnhancedTool) generateHealthCheck(language, framework string) string {
	// Generate appropriate health check based on language/framework
	switch strings.ToLower(language) {
	case "javascript", "typescript":
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD node -e \"require('http').get('http://localhost:' + (process.env.PORT || 3000) + '/health', (res) => process.exit(res.statusCode === 200 ? 0 : 1))\""
	case "python":
		if strings.Contains(strings.ToLower(framework), "django") {
			return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD python -c \"import urllib.request; urllib.request.urlopen('http://localhost:8000/health')\""
		}
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD python -c \"import urllib.request; urllib.request.urlopen('http://localhost:5000/health')\""
	case "go":
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1"
	case "java":
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 \\\n  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/actuator/health || exit 1"
	default:
		return "HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD wget --no-verbose --tries=1 --spider http://localhost/ || exit 1"
	}
}

func (t *GenerateDockerfileEnhancedTool) applyOptimization(content, optimization string, context *DockerfileTemplateContext) string {
	// Apply optimization strategies
	switch optimization {
	case "size":
		// Add size optimization comments and suggestions
		sizeHints := "\n# Size optimization applied:\n"
		sizeHints += "# - Using minimal base images where possible\n"
		sizeHints += "# - Combining RUN commands to reduce layers\n"
		sizeHints += "# - Cleaning package manager caches\n"
		sizeHints += "# - Removing unnecessary build dependencies\n"
		return sizeHints + content

	case "security":
		// Add security hardening
		securityHints := "\n# Security hardening applied:\n"
		securityHints += "# - Running as non-root user\n"
		securityHints += "# - Using specific version tags\n"
		securityHints += "# - Minimal attack surface\n"

		// Ensure non-root user
		if !strings.Contains(content, "USER ") {
			content += "\n# Run as non-root user\nRUN adduser -D -u 1001 appuser\nUSER appuser\n"
		}
		return securityHints + content

	case "speed":
		// Add build speed optimization
		speedHints := "\n# Build speed optimization applied:\n"
		speedHints += "# - Leveraging build cache effectively\n"
		speedHints += "# - Ordering commands by change frequency\n"
		speedHints += "# - Using cache mounts for package managers\n"
		return speedHints + content

	default:
		return content
	}
}

func (t *GenerateDockerfileEnhancedTool) generateOptimizationContext(content string, args GenerateDockerfileArgs, context *DockerfileTemplateContext) *OptimizationContext {
	ctx := &OptimizationContext{
		OptimizationGoals: []string{},
		SuggestedChanges:  []OptimizationChange{},
		SecurityConcerns:  []SecurityConcern{},
		BestPractices:     []string{},
	}

	// Analyze current Dockerfile
	lines := strings.Split(content, "\n")
	runCount := 0
	hasUser := false
	hasHealthcheck := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "RUN ") {
			runCount++
		}
		if strings.HasPrefix(upper, "USER ") {
			hasUser = true
		}
		if strings.HasPrefix(upper, "HEALTHCHECK ") {
			hasHealthcheck = true
		}
	}

	// Set optimization goals based on args
	if args.Optimization == "size" {
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Minimize image size")
	} else if args.Optimization == "security" {
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Maximize security posture")
	} else if args.Optimization == "speed" {
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Optimize build speed")
	}

	// Suggest layer optimization if many RUN commands
	if runCount > 5 {
		ctx.SuggestedChanges = append(ctx.SuggestedChanges, OptimizationChange{
			Type:        "size",
			Description: "Combine multiple RUN commands to reduce layers",
			Impact:      "Smaller image size, fewer layers",
			Example:     "RUN apt-get update && apt-get install -y pkg1 pkg2 && rm -rf /var/lib/apt/lists/*",
		})
	}

	// Security concerns
	if !hasUser {
		ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
			Issue:      "Container runs as root user",
			Severity:   "high",
			Suggestion: "Add a non-root user and switch to it",
			Reference:  "CIS Docker Benchmark 4.1",
		})
	}

	// Health check recommendation
	if !hasHealthcheck && !args.IncludeHealthCheck {
		ctx.SuggestedChanges = append(ctx.SuggestedChanges, OptimizationChange{
			Type:        "reliability",
			Description: "Add HEALTHCHECK instruction",
			Impact:      "Better container health monitoring",
			Example:     "HEALTHCHECK CMD wget --spider http://localhost/health || exit 1",
		})
	}

	// Best practices based on template
	ctx.BestPractices = append(ctx.BestPractices,
		"Pin base image versions for reproducibility",
		"Order Dockerfile commands from least to most frequently changing",
		"Use .dockerignore to exclude unnecessary files",
		"Leverage multi-stage builds for smaller production images",
	)

	// Add template-specific customization hints
	if customOpts, ok := context.CustomizationOptions["optimization_hints"].([]string); ok {
		ctx.BestPractices = append(ctx.BestPractices, customOpts...)
	}

	return ctx
}

func (t *GenerateDockerfileEnhancedTool) extractBuildSteps(content string) []string {
	steps := []string{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "RUN ") {
			steps = append(steps, strings.TrimPrefix(trimmed, "RUN "))
		}
	}

	return steps
}

func (t *GenerateDockerfileEnhancedTool) extractExposedPorts(content string) []int {
	ports := []int{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "EXPOSE ") {
			portStr := strings.TrimPrefix(strings.ToUpper(trimmed), "EXPOSE ")
			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
				ports = append(ports, port)
			}
		}
	}

	return ports
}

func (t *GenerateDockerfileEnhancedTool) extractBaseImage(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			return strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(trimmed), "FROM "))
		}
	}

	return ""
}

func (t *GenerateDockerfileEnhancedTool) extractHealthCheck(content string) string {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "HEALTHCHECK ") {
			// Handle multi-line health checks
			healthCheck := trimmed
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if strings.HasSuffix(trimmed, "\\") {
					healthCheck += " " + nextLine
				} else {
					break
				}
			}
			return healthCheck
		}
	}

	return ""
}

func (t *GenerateDockerfileEnhancedTool) validateDockerfile(ctx context.Context, content string) *coredocker.ValidationResult {
	// Use the validator's ValidateDockerfile method
	return t.validator.ValidateDockerfile(content)
}

// Execute implements the unified Tool interface
func (t *GenerateDockerfileEnhancedTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Convert generic args to typed args
	var dockerArgs GenerateDockerfileArgs

	switch a := args.(type) {
	case GenerateDockerfileArgs:
		dockerArgs = a
	case map[string]interface{}:
		// Convert from map to struct using JSON marshaling
		jsonData, err := json.Marshal(a)
		if err != nil {
			return nil, types.NewRichError("INVALID_ARGUMENTS", "Failed to marshal arguments", "validation_error")
		}
		if err = json.Unmarshal(jsonData, &dockerArgs); err != nil {
			return nil, types.NewRichError("INVALID_ARGUMENTS", "Invalid argument structure for generate_dockerfile", "validation_error")
		}
	default:
		return nil, types.NewRichError("INVALID_ARGUMENTS", "Invalid argument type for generate_dockerfile", "validation_error")
	}

	// Call the typed execute method
	return t.ExecuteTyped(ctx, dockerArgs)
}

// Validate implements the unified Tool interface
func (t *GenerateDockerfileEnhancedTool) Validate(ctx context.Context, args interface{}) error {
	var dockerArgs GenerateDockerfileArgs

	switch a := args.(type) {
	case GenerateDockerfileArgs:
		dockerArgs = a
	case map[string]interface{}:
		// Convert from map to struct using JSON marshaling
		jsonData, err := json.Marshal(a)
		if err != nil {
			return types.NewRichError("INVALID_ARGUMENTS", "Failed to marshal arguments", "validation_error")
		}
		if err = json.Unmarshal(jsonData, &dockerArgs); err != nil {
			return types.NewRichError("INVALID_ARGUMENTS", "Invalid argument structure for generate_dockerfile", "validation_error")
		}
	default:
		return types.NewRichError("INVALID_ARGUMENTS", "Invalid argument type for generate_dockerfile", "validation_error")
	}

	// Validate required fields
	if dockerArgs.SessionID == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "session_id is required", "validation_error")
	}

	return nil
}

// GetMetadata implements the unified Tool interface
func (t *GenerateDockerfileEnhancedTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "generate_dockerfile_enhanced",
		Description:  "Generates optimized Dockerfiles using advanced template integration and best practices",
		Version:      "2.0.0",
		Category:     "build",
		Dependencies: []string{"analyze_repository"},
		Capabilities: []string{
			"template_selection",
			"multi_stage_builds",
			"optimization_strategies",
			"security_scanning",
			"hadolint_validation",
			"best_practices_enforcement",
			"custom_template_support",
		},
		Requirements: []string{
			"repository_analysis",
			"filesystem_access",
		},
		Parameters: map[string]string{
			"session_id":           "Required session identifier",
			"analysis":             "Repository analysis result (optional, will fetch from session)",
			"template":             "Template name (e.g., 'node', 'python', 'custom')",
			"optimization":         "Optimization level: 'size', 'security', 'speed', 'balanced'",
			"include_health_check": "Include HEALTHCHECK instruction",
			"multi_stage":          "Use multi-stage build pattern",
			"custom_template":      "Path to custom Dockerfile template",
			"template_vars":        "Variables for custom template",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Generate with Template",
				Description: "Generate Dockerfile using a specific template",
				Input: map[string]interface{}{
					"session_id":   "build-session",
					"template":     "node",
					"optimization": "balanced",
					"multi_stage":  true,
				},
				Output: map[string]interface{}{
					"dockerfile_path": "/workspace/session/Dockerfile",
					"template_used":   "node-multi-stage",
					"optimization":    "balanced",
				},
			},
			{
				Name:        "Generate with Custom Template",
				Description: "Generate using custom template with variables",
				Input: map[string]interface{}{
					"session_id":      "build-session",
					"custom_template": "/templates/custom.dockerfile",
					"template_vars": map[string]string{
						"NODE_VERSION": "18",
						"APP_PORT":     "3000",
					},
				},
				Output: map[string]interface{}{
					"dockerfile_path": "/workspace/session/Dockerfile",
					"template_used":   "custom",
				},
			},
		},
	}
}
