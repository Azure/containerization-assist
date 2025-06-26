package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// GenerateDockerfileArgs defines the arguments for the generate_dockerfile tool
type GenerateDockerfileArgs struct {
	types.BaseToolArgs
	BaseImage          string            `json:"base_image,omitempty" description:"Override detected base image"`
	Template           string            `json:"template,omitempty" jsonschema:"enum=go,node,python,java,rust,php,ruby,dotnet,golang" description:"Use specific template (go, node, python, etc.)"`
	Optimization       string            `json:"optimization,omitempty" jsonschema:"enum=size,speed,security,balanced" description:"Optimization level (size, speed, security)"`
	IncludeHealthCheck bool              `json:"include_health_check,omitempty" description:"Add health check to Dockerfile"`
	BuildArgs          map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	Platform           string            `json:"platform,omitempty" jsonschema:"enum=linux/amd64,linux/arm64,linux/arm/v7" description:"Target platform (e.g., linux/amd64)"`
}

// GenerateDockerfileResult defines the response for the generate_dockerfile tool
type GenerateDockerfileResult struct {
	types.BaseToolResponse
	Content      string                       `json:"content"`
	BaseImage    string                       `json:"base_image"`
	ExposedPorts []int                        `json:"exposed_ports"`
	HealthCheck  string                       `json:"health_check,omitempty"`
	BuildSteps   []string                     `json:"build_steps"`
	Template     string                       `json:"template_used"`
	FilePath     string                       `json:"file_path"`
	Validation   *coredocker.ValidationResult `json:"validation,omitempty"`
	Message      string                       `json:"message,omitempty"`

	// Rich context for AI decision making
	TemplateSelection *TemplateSelectionContext `json:"template_selection,omitempty"`
	OptimizationHints *OptimizationContext      `json:"optimization_hints,omitempty"`
}

// TemplateSelectionContext provides rich context for AI template selection
type TemplateSelectionContext struct {
	DetectedLanguage    string                `json:"detected_language"`
	DetectedFramework   string                `json:"detected_framework"`
	AvailableTemplates  []TemplateOption      `json:"available_templates"`
	RecommendedTemplate string                `json:"recommended_template"`
	SelectionReasoning  []string              `json:"selection_reasoning"`
	AlternativeOptions  []AlternativeTemplate `json:"alternative_options"`
}

// TemplateOption describes an available template
type TemplateOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BestFor     []string `json:"best_for"`
	Limitations []string `json:"limitations"`
	MatchScore  int      `json:"match_score"` // 0-100
}

// AlternativeTemplate suggests alternatives with trade-offs
type AlternativeTemplate struct {
	Template  string   `json:"template"`
	Reason    string   `json:"reason"`
	TradeOffs []string `json:"trade_offs"`
	UseCases  []string `json:"use_cases"`
}

// OptimizationContext provides optimization guidance for AI
type OptimizationContext struct {
	CurrentSize       string               `json:"current_size,omitempty"`
	OptimizationGoals []string             `json:"optimization_goals"`
	SuggestedChanges  []OptimizationChange `json:"suggested_changes"`
	SecurityConcerns  []SecurityConcern    `json:"security_concerns"`
	BestPractices     []string             `json:"best_practices"`
}

// OptimizationChange describes a potential optimization
type OptimizationChange struct {
	Type        string `json:"type"` // "size", "security", "performance"
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Example     string `json:"example,omitempty"`
}

// SecurityConcern describes a security issue
type SecurityConcern struct {
	Issue      string `json:"issue"`
	Severity   string `json:"severity"` // "high", "medium", "low"
	Suggestion string `json:"suggestion"`
	Reference  string `json:"reference,omitempty"`
}

// GenerateDockerfileTool implements Dockerfile generation functionality
type GenerateDockerfileTool struct {
	logger              zerolog.Logger
	validator           *coredocker.Validator
	hadolintValidator   *coredocker.HadolintValidator
	sessionManager      mcptypes.ToolSessionManager
	templateIntegration *TemplateIntegration
}

// NewGenerateDockerfileTool creates a new instance of GenerateDockerfileTool
func NewGenerateDockerfileTool(sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *GenerateDockerfileTool {
	return &GenerateDockerfileTool{
		logger:              logger,
		validator:           coredocker.NewValidator(logger),
		hadolintValidator:   coredocker.NewHadolintValidator(logger),
		sessionManager:      sessionManager,
		templateIntegration: NewTemplateIntegration(logger),
	}
}

// Execute generates a Dockerfile based on repository analysis and user preferences
func (t *GenerateDockerfileTool) ExecuteTyped(ctx context.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	// Create base response
	response := &GenerateDockerfileResult{
		BaseToolResponse: types.NewBaseResponse("generate_dockerfile", args.SessionID, args.DryRun),
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", args.Template).
		Str("optimization", args.Optimization).
		Bool("dry_run", args.DryRun).
		Msg("Starting Dockerfile generation")

	// Get session to access repository analysis
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("failed to get session %s: %v", args.SessionID, err), "session_error")
	}

	// Type assert to concrete session type
	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, types.NewRichError("INTERNAL_ERROR", "session type assertion failed", "type_error")
	}

	// Select template based on repository analysis or user override
	templateName := args.Template
	if templateName == "" {
		// Use repository analysis to auto-select template
		var repositoryData map[string]interface{}
		if session.ScanSummary != nil {
			repositoryData = sessiontypes.ConvertScanSummaryToRepositoryInfo(session.ScanSummary)
		}
		if repositoryData != nil && len(repositoryData) > 0 {
			selectedTemplate, err := t.selectTemplate(repositoryData)
			if err != nil {
				t.logger.Warn().Err(err).Msg("Failed to auto-select template, using generic dockerfile-python template")
				templateName = "dockerfile-python" // Generic fallback that exists
			} else {
				templateName = selectedTemplate
			}
		} else {
			t.logger.Warn().Msg("No repository analysis found, using generic dockerfile-python template")
			templateName = "dockerfile-python" // Generic fallback that exists
		}
	} else {
		// If user provided a template name, map common language names to actual template names
		templateName = t.mapCommonTemplateNames(templateName)
	}

	t.logger.Info().Str("template", templateName).Msg("Selected Dockerfile template")

	// Handle dry-run mode
	if args.DryRun {
		var repositoryData map[string]interface{}
		if session.ScanSummary != nil {
			repositoryData = sessiontypes.ConvertScanSummaryToRepositoryInfo(session.ScanSummary)
		}
		content, err := t.previewDockerfile(templateName, args, repositoryData)
		if err != nil {
			return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to preview Dockerfile: %v", err), "generation_error")
		}

		response.Content = content
		response.Template = templateName
		response.BuildSteps = t.extractBuildSteps(content)
		response.ExposedPorts = t.extractExposedPorts(content)
		response.BaseImage = t.extractBaseImage(content)

		return response, nil
	}

	// For actual generation, we'll need a target directory
	// For now, use current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to get current directory: %v", err), "filesystem_error")
	}

	dockerfilePath := filepath.Join(cwd, "Dockerfile")
	repositoryData := make(map[string]interface{})
	if session.ScanSummary != nil {
		repositoryData = sessiontypes.ConvertScanSummaryToRepositoryInfo(session.ScanSummary)
	}
	content, err := t.generateDockerfile(templateName, dockerfilePath, args, repositoryData)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to generate Dockerfile: %v", err), "generation_error")
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

	// Generate rich context for AI decision making
	// repositoryData already created above

	if repositoryData != nil && len(repositoryData) > 0 {
		// Extract analysis data for context generation
		language, _ := repositoryData["language"].(string)   //nolint:errcheck // Used for context
		framework, _ := repositoryData["framework"].(string) //nolint:errcheck // Used for context

		// Extract dependencies
		var dependencies []string
		if deps, ok := repositoryData["dependencies"].([]string); ok {
			dependencies = deps
		} else if deps, ok := repositoryData["dependencies"].([]interface{}); ok {
			for _, dep := range deps {
				if depStr, ok := dep.(string); ok {
					dependencies = append(dependencies, depStr)
				}
			}
		}

		// Extract config files
		var configFiles []string
		if files, ok := repositoryData["files"].([]string); ok {
			configFiles = files
		} else if files, ok := repositoryData["files"].([]interface{}); ok {
			for _, file := range files {
				if fileStr, ok := file.(string); ok {
					configFiles = append(configFiles, fileStr)
				}
			}
		}

		// Generate template selection context
		response.TemplateSelection = t.generateTemplateSelectionContext(language, framework, dependencies, configFiles)
	}

	// Generate optimization context
	response.OptimizationHints = t.generateOptimizationContext(content, args)

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

			// Don't fail completely, but add warning to response
			response.Message = fmt.Sprintf(
				"Dockerfile generated but has %d critical validation errors. Please review and fix before building.",
				criticalErrors)
		}
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", templateName).
		Str("file_path", dockerfilePath).
		Bool("validation_passed", validationResult == nil || validationResult.Valid).
		Msg("Successfully generated Dockerfile")

	return response, nil
}

// ExecuteWithContext runs the Dockerfile generation with GoMCP progress tracking
func (t *GenerateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	// Create progress adapter for GoMCP using standard generation stages
	_ = mcptypes.NewGoMCPProgressAdapter(serverCtx, []mcptypes.LocalProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Generate", Weight: 0.80, Description: "Generating"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Progress adapter removed - execute the core logic directly
	t.logger.Info().Msg("Initializing Dockerfile generation")

	// Execute the core logic
	result, err := t.ExecuteTyped(context.Background(), args)

	if err != nil {
		t.logger.Info().Msg("Dockerfile generation failed")
		return result, nil // Return nil result since this tool returns error directly
	} else {
		t.logger.Info().Msg("Dockerfile generation completed successfully")
	}

	return result, nil
}

// selectTemplate automatically selects the best template based on repository analysis
func (t *GenerateDockerfileTool) selectTemplate(repoAnalysis map[string]interface{}) (string, error) {
	// Extract language from analysis
	language, ok := repoAnalysis["language"].(string)
	if !ok {
		return "", types.NewRichError("INVALID_ARGUMENTS", "no language detected in repository analysis", "missing_language")
	}

	// Extract config files and dependencies for template engine
	var configFiles []string
	var dependencies []string

	// Extract files from analysis
	if files, ok := repoAnalysis["files"].([]interface{}); ok {
		for _, file := range files {
			if fileStr, ok := file.(string); ok {
				configFiles = append(configFiles, fileStr)
			}
		}
	}

	// Extract dependencies from analysis (handle both string slice and dependency objects)
	if deps, ok := repoAnalysis["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			switch d := dep.(type) {
			case string:
				dependencies = append(dependencies, d)
			case map[string]interface{}:
				// Handle dependency objects with Name field
				if name, ok := d["Name"].(string); ok {
					dependencies = append(dependencies, name)
				}
			}
		}
	}

	// Extract framework from analysis
	framework := ""
	if fw, ok := repoAnalysis["framework"].(string); ok {
		framework = fw
	}

	// Use the enhanced core template engine for selection
	templateEngine := coredocker.NewTemplateEngine(t.logger)
	templateName, _, err := templateEngine.SuggestTemplate(language, framework, dependencies, configFiles)
	if err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("template selection failed: %v", err), "template_error")
	}

	// Log the selected template for debugging
	t.logger.Info().
		Str("language", language).
		Str("framework", framework).
		Str("selected_template", templateName).
		Msg("Template selected by engine")

	return templateName, nil
}

// getRecommendedBaseImage returns the recommended base image for a language/framework combination
func (t *GenerateDockerfileTool) getRecommendedBaseImage(language, framework string) string {
	// Base image lookup table with optimized images for each language
	baseImageMap := map[string]map[string]string{
		"Go": {
			"default":    "golang:1.21-alpine",
			"gin":        "golang:1.21-alpine",
			"echo":       "golang:1.21-alpine",
			"fiber":      "golang:1.21-alpine",
			"gorilla":    "golang:1.21-alpine",
			"chi":        "golang:1.21-alpine",
			"production": "gcr.io/distroless/static:nonroot", // For multi-stage builds
		},
		"JavaScript": {
			"default":    "node:18-alpine",
			"express":    "node:18-alpine",
			"nestjs":     "node:18-alpine",
			"react":      "node:18-alpine",
			"vue":        "node:18-alpine",
			"angular":    "node:18-alpine",
			"next":       "node:18-alpine",
			"nuxt":       "node:18-alpine",
			"production": "node:18-alpine",
		},
		"TypeScript": {
			"default":    "node:18-alpine",
			"express":    "node:18-alpine",
			"nestjs":     "node:18-alpine",
			"react":      "node:18-alpine",
			"vue":        "node:18-alpine",
			"angular":    "node:18-alpine",
			"next":       "node:18-alpine",
			"production": "node:18-alpine",
		},
		"Python": {
			"default":    "python:3.11-slim",
			"django":     "python:3.11-slim",
			"flask":      "python:3.11-slim",
			"fastapi":    "python:3.11-slim",
			"tornado":    "python:3.11-slim",
			"pyramid":    "python:3.11-slim",
			"production": "python:3.11-slim",
		},
		"Java": {
			"default":     "openjdk:17-jre-slim",
			"maven":       "maven:3.9-openjdk-17-slim",
			"gradle":      "gradle:8-jdk17-alpine",
			"spring":      "openjdk:17-jre-slim",
			"spring-boot": "openjdk:17-jre-slim",
			"production":  "eclipse-temurin:17-jre-alpine",
		},
		"C#": {
			"default":    "mcr.microsoft.com/dotnet/aspnet:7.0",
			"aspnet":     "mcr.microsoft.com/dotnet/aspnet:7.0",
			"console":    "mcr.microsoft.com/dotnet/runtime:7.0",
			"production": "mcr.microsoft.com/dotnet/aspnet:7.0-alpine",
		},
		"Ruby": {
			"default":    "ruby:3.2-alpine",
			"rails":      "ruby:3.2-alpine",
			"sinatra":    "ruby:3.2-alpine",
			"production": "ruby:3.2-alpine",
		},
		"PHP": {
			"default":    "php:8.2-fpm-alpine",
			"laravel":    "php:8.2-fpm-alpine",
			"symfony":    "php:8.2-fpm-alpine",
			"wordpress":  "wordpress:6-php8.2-fpm-alpine",
			"production": "php:8.2-fpm-alpine",
		},
		"Rust": {
			"default":    "rust:1.75-alpine",
			"actix":      "rust:1.75-alpine",
			"rocket":     "rust:1.75-alpine",
			"production": "gcr.io/distroless/cc:nonroot", // For multi-stage builds
		},
		"Swift": {
			"default":    "swift:5.9-jammy",
			"vapor":      "swift:5.9-jammy",
			"production": "swift:5.9-jammy-slim",
		},
	}

	// Get language-specific images
	languageImages, exists := baseImageMap[language]
	if !exists {
		// Return a generic Linux base for unknown languages
		return "ubuntu:22.04"
	}

	// Try to find framework-specific image
	if framework != "" {
		if image, exists := languageImages[strings.ToLower(framework)]; exists {
			return image
		}
	}

	// Fall back to default for the language
	if defaultImage, exists := languageImages["default"]; exists {
		return defaultImage
	}

	// Ultimate fallback
	return "ubuntu:22.04"
}

// previewDockerfile generates a preview of the Dockerfile without writing to disk
func (t *GenerateDockerfileTool) previewDockerfile(templateName string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Use the core template engine to generate preview
	templateEngine := coredocker.NewTemplateEngine(t.logger)

	// Create a temporary directory for preview
	tempDir, err := os.MkdirTemp("", "dockerfile-preview-*")
	if err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create temp directory: %v", err), "filesystem_error")
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			// Log but don't fail - temp dir cleanup is not critical
			t.logger.Warn().Err(err).Str("temp_dir", tempDir).Msg("Failed to remove temp directory")
		}
	}()

	// Generate from template
	result, err := templateEngine.GenerateFromTemplate(templateName, tempDir)
	if err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to generate from template: %v", err), "template_error")
	}

	if !result.Success {
		if result.Error != nil {
			return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("template generation failed: %s - %s", result.Error.Type, result.Error.Message), "template_error")
		}
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", "template generation failed with unknown error", "template_error")
	}

	// Apply customizations
	dockerfileContent := result.Dockerfile
	dockerfileContent = t.applyCustomizations(dockerfileContent, args, repoAnalysis)

	return dockerfileContent, nil
}

// generateDockerfile creates the actual Dockerfile
func (t *GenerateDockerfileTool) generateDockerfile(templateName, dockerfilePath string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Use the core template engine for better error handling
	targetDir := filepath.Dir(dockerfilePath)
	templateEngine := coredocker.NewTemplateEngine(t.logger)

	// Generate from template using the core engine
	result, err := templateEngine.GenerateFromTemplate(templateName, targetDir)
	if err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to generate from template: %v", err), "template_error")
	}

	// Check if generation was successful
	if !result.Success {
		if result.Error != nil {
			return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("template generation failed: %s - %s", result.Error.Type, result.Error.Message), "template_error")
		}
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", "template generation failed with unknown error", "template_error")
	}

	// Read the generated content to ensure it was written
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to read generated Dockerfile: %v", err), "file_error")
	}

	// Apply customizations
	dockerfileContent := string(content)
	dockerfileContent = t.applyCustomizations(dockerfileContent, args, repoAnalysis)

	// Write the customized content back
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0o644); err != nil {
		return "", types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to write customized Dockerfile: %v", err), "file_error")
	}

	return dockerfileContent, nil
}

// applyCustomizations applies user-specified customizations to the Dockerfile
func (t *GenerateDockerfileTool) applyCustomizations(content string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) string {
	lines := strings.Split(content, "\n")
	var result []string

	// Apply base image override or use recommended base image from lookup table
	baseImageToUse := args.BaseImage
	if baseImageToUse == "" && repoAnalysis != nil {
		// Use base image lookup table to get recommended image
		language, _ := repoAnalysis["language"].(string)   //nolint:errcheck // Has defaults
		framework, _ := repoAnalysis["framework"].(string) //nolint:errcheck // Has defaults
		recommendedImage := t.getRecommendedBaseImage(language, framework)
		baseImageToUse = recommendedImage

		t.logger.Info().
			Str("language", language).
			Str("framework", framework).
			Str("recommended_image", recommendedImage).
			Msg("Using recommended base image from lookup table")
	}

	if baseImageToUse != "" {
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "FROM ") {
				lines[i] = fmt.Sprintf("FROM %s", baseImageToUse)
				break
			}
		}
	}

	// Add health check if requested
	if args.IncludeHealthCheck {
		healthCheck := t.generateHealthCheck()
		if healthCheck != "" {
			// Insert health check before CMD instruction
			for i, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "CMD ") || strings.HasPrefix(strings.TrimSpace(line), "ENTRYPOINT ") {
					// Insert health check before CMD/ENTRYPOINT
					result = append(result, lines[:i]...)
					result = append(result, "", healthCheck)
					result = append(result, lines[i:]...)
					return strings.Join(result, "\n")
				}
			}
			// If no CMD/ENTRYPOINT found, add at the end
			lines = append(lines, "", healthCheck)
		}
	}

	// Apply optimization-specific changes
	switch args.Optimization {
	case "size":
		lines = t.applySizeOptimizations(lines)
	case "security":
		lines = t.applySecurityOptimizations(lines)
	}

	return strings.Join(lines, "\n")
}

// generateHealthCheck creates a health check instruction
func (t *GenerateDockerfileTool) generateHealthCheck() string {
	// For now, use a simple health check - could be enhanced based on language/framework
	port := 80 // default port
	return fmt.Sprintf("HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:%d/health || exit 1", port)
}

// applySizeOptimizations applies Docker best practices for smaller images
func (t *GenerateDockerfileTool) applySizeOptimizations(lines []string) []string {
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Combine RUN commands where possible and add cleanup
		if strings.HasPrefix(trimmed, "RUN ") {
			if strings.Contains(trimmed, "apt-get") || strings.Contains(trimmed, "apk") {
				// Add cleanup for package managers
				if strings.Contains(trimmed, "apt-get") && !strings.Contains(trimmed, "rm -rf /var/lib/apt/lists/*") {
					line += " && rm -rf /var/lib/apt/lists/*"
				} else if strings.Contains(trimmed, "apk") && !strings.Contains(trimmed, "--no-cache") {
					line = strings.Replace(line, "apk add", "apk add --no-cache", 1)
				}
			}
		}

		result = append(result, line)
	}

	return result
}

// applySecurityOptimizations applies security best practices
func (t *GenerateDockerfileTool) applySecurityOptimizations(lines []string) []string {
	var result []string
	addedUser := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Add non-root user before CMD/ENTRYPOINT
		if !addedUser && (strings.HasPrefix(trimmed, "CMD ") || strings.HasPrefix(trimmed, "ENTRYPOINT ")) {
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
			result = append(result, "")
			addedUser = true
		}

		result = append(result, line)

		// If this is the last line and we haven't added a user, add it
		if i == len(lines)-1 && !addedUser {
			result = append(result, "")
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
		}
	}

	return result
}

// extractBuildSteps extracts the build steps from Dockerfile content
func (t *GenerateDockerfileTool) extractBuildSteps(content string) []string {
	var steps []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Extract major build instructions
		if strings.HasPrefix(trimmed, "FROM ") ||
			strings.HasPrefix(trimmed, "RUN ") ||
			strings.HasPrefix(trimmed, "COPY ") ||
			strings.HasPrefix(trimmed, "ADD ") ||
			strings.HasPrefix(trimmed, "WORKDIR ") {
			steps = append(steps, trimmed)
		}
	}

	return steps
}

// extractExposedPorts extracts exposed ports from Dockerfile content
func (t *GenerateDockerfileTool) extractExposedPorts(content string) []int {
	var ports []int
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "EXPOSE ") {
			portStr := strings.TrimPrefix(trimmed, "EXPOSE ")
			portStr = strings.TrimSpace(portStr)

			// Simple port extraction (could be enhanced for complex cases)
			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
				ports = append(ports, port)
			}
		}
	}

	return ports
}

// extractBaseImage extracts the base image from Dockerfile content
func (t *GenerateDockerfileTool) extractBaseImage(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "FROM ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return ""
}

// extractHealthCheck extracts health check instruction from Dockerfile content
func (t *GenerateDockerfileTool) extractHealthCheck(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "HEALTHCHECK ") {
			return trimmed
		}
	}

	return ""
}

// mapCommonTemplateNames maps common language/framework names to actual template directory names
func (t *GenerateDockerfileTool) mapCommonTemplateNames(name string) string {
	// Map common names to actual template names
	templateMap := map[string]string{
		"java":        "dockerfile-maven",       // Default Java to Maven
		"java-web":    "dockerfile-java-tomcat", // Java web apps
		"java-tomcat": "dockerfile-java-tomcat",
		"java-jboss":  "dockerfile-java-jboss",
		"maven":       "dockerfile-maven",
		"gradle":      "dockerfile-gradle",
		"gradlew":     "dockerfile-gradlew",
		"go":          "dockerfile-go",
		"golang":      "dockerfile-go",
		"go-module":   "dockerfile-gomodule",
		"gomod":       "dockerfile-gomodule",
		"node":        "dockerfile-javascript",
		"nodejs":      "dockerfile-javascript",
		"javascript":  "dockerfile-javascript",
		"js":          "dockerfile-javascript",
		"python":      "dockerfile-python",
		"py":          "dockerfile-python",
		"ruby":        "dockerfile-ruby",
		"rb":          "dockerfile-ruby",
		"php":         "dockerfile-php",
		"csharp":      "dockerfile-csharp",
		"c#":          "dockerfile-csharp",
		"dotnet":      "dockerfile-csharp",
		"rust":        "dockerfile-rust",
		"swift":       "dockerfile-swift",
		"clojure":     "dockerfile-clojure",
		"erlang":      "dockerfile-erlang",
	}

	// Check if we have a mapping
	if mapped, exists := templateMap[strings.ToLower(name)]; exists {
		t.logger.Info().
			Str("input", name).
			Str("mapped", mapped).
			Msg("Mapped template name")
		return mapped
	}

	// If it starts with "dockerfile-", assume it's already a full template name
	if strings.HasPrefix(name, "dockerfile-") {
		return name
	}

	// Otherwise, return as-is and let the template engine handle validation
	return name
}

// validateDockerfile validates the generated Dockerfile content
func (t *GenerateDockerfileTool) validateDockerfile(ctx context.Context, content string) *coredocker.ValidationResult {
	// First try Hadolint if available
	if t.hadolintValidator.CheckHadolintInstalled() {
		t.logger.Info().Msg("Running Hadolint validation on generated Dockerfile")
		result, err := t.hadolintValidator.ValidateWithHadolint(ctx, content)
		if err == nil {
			return result
		}
		t.logger.Warn().Err(err).Msg("Hadolint validation failed, falling back to basic validation")
	} else {
		t.logger.Info().Msg("Hadolint not installed, using basic validation")
	}

	// Fall back to basic validation
	return t.validator.ValidateDockerfile(content)
}

// generateTemplateSelectionContext creates rich context for AI template selection
func (t *GenerateDockerfileTool) generateTemplateSelectionContext(language, framework string, dependencies, configFiles []string) *TemplateSelectionContext {
	ctx := &TemplateSelectionContext{
		DetectedLanguage:   language,
		DetectedFramework:  framework,
		AvailableTemplates: make([]TemplateOption, 0),
		SelectionReasoning: make([]string, 0),
		AlternativeOptions: make([]AlternativeTemplate, 0),
	}

	// Generate template options with rich metadata
	templateOptions := t.getTemplateOptions(language, framework, dependencies, configFiles)
	ctx.AvailableTemplates = templateOptions

	// Find best match
	var bestTemplate TemplateOption
	bestScore := 0
	for _, tmpl := range templateOptions {
		if tmpl.MatchScore > bestScore {
			bestScore = tmpl.MatchScore
			bestTemplate = tmpl
		}
	}

	if bestScore > 0 {
		ctx.RecommendedTemplate = bestTemplate.Name
		ctx.SelectionReasoning = append(ctx.SelectionReasoning,
			fmt.Sprintf("Template '%s' has the highest match score (%d/100) for %s/%s projects",
				bestTemplate.Name, bestScore, language, framework))
	}

	// Add alternative recommendations
	ctx.AlternativeOptions = t.getAlternativeTemplates(language, framework, dependencies)

	return ctx
}

// getTemplateOptions returns available templates with metadata
func (t *GenerateDockerfileTool) getTemplateOptions(language, framework string, dependencies, configFiles []string) []TemplateOption {
	options := []TemplateOption{
		// Java templates
		{
			Name:        "dockerfile-maven",
			Description: "Multi-stage Maven build with dependency caching",
			BestFor:     []string{"Maven projects", "Spring Boot", "Enterprise Java"},
			Limitations: []string{"Requires pom.xml", "Not suitable for Gradle projects"},
			MatchScore:  t.calculateMatchScore("maven", language, framework, configFiles),
		},
		{
			Name:        "dockerfile-gradle",
			Description: "Multi-stage Gradle build with wrapper support",
			BestFor:     []string{"Gradle projects", "Android backend", "Kotlin services"},
			Limitations: []string{"Requires build.gradle", "Larger build cache"},
			MatchScore:  t.calculateMatchScore("gradle", language, framework, configFiles),
		},
		{
			Name:        "dockerfile-java-tomcat",
			Description: "Tomcat-based deployment for WAR files",
			BestFor:     []string{"Java web applications", "JSP projects", "Servlet-based apps"},
			Limitations: []string{"Heavier base image", "Requires WAR packaging"},
			MatchScore:  t.calculateMatchScore("tomcat", language, framework, configFiles),
		},
		// Node.js templates
		{
			Name:        "dockerfile-javascript",
			Description: "Node.js with npm/yarn optimization",
			BestFor:     []string{"Express apps", "React SSR", "Node.js APIs"},
			Limitations: []string{"Single-stage build", "No TypeScript compilation"},
			MatchScore:  t.calculateMatchScore("javascript", language, framework, configFiles),
		},
		// Python templates
		{
			Name:        "dockerfile-python",
			Description: "Python with pip/poetry support",
			BestFor:     []string{"Django", "Flask", "FastAPI", "Data science apps"},
			Limitations: []string{"May need additional system dependencies"},
			MatchScore:  t.calculateMatchScore("python", language, framework, configFiles),
		},
		// Go templates
		{
			Name:        "dockerfile-go",
			Description: "Go build without modules",
			BestFor:     []string{"Simple Go applications", "GOPATH-based projects"},
			Limitations: []string{"No module support", "Deprecated approach"},
			MatchScore:  t.calculateMatchScore("go", language, framework, configFiles),
		},
		{
			Name:        "dockerfile-gomodule",
			Description: "Modern Go with module support",
			BestFor:     []string{"Go 1.11+ projects", "Microservices", "CLI tools"},
			Limitations: []string{"Requires go.mod file"},
			MatchScore:  t.calculateMatchScore("gomodule", language, framework, configFiles),
		},
	}

	// Sort by match score
	for i := 0; i < len(options)-1; i++ {
		for j := i + 1; j < len(options); j++ {
			if options[j].MatchScore > options[i].MatchScore {
				options[i], options[j] = options[j], options[i]
			}
		}
	}

	return options
}

// calculateMatchScore calculates how well a template matches the project
func (t *GenerateDockerfileTool) calculateMatchScore(templateType, language, framework string, configFiles []string) int {
	score := 0

	// Language match
	switch templateType {
	case "maven", "gradle", types.AppServerTomcat:
		if strings.ToLower(language) == "java" {
			score += 40
		}
	case "javascript":
		if strings.ToLower(language) == "javascript" || strings.ToLower(language) == "typescript" {
			score += 40
		}
	case "python":
		if strings.ToLower(language) == "python" {
			score += 40
		}
	case "go", "gomodule":
		if strings.ToLower(language) == "go" {
			score += 40
		}
	}

	// Config file match
	for _, file := range configFiles {
		switch templateType {
		case "maven":
			if strings.Contains(file, "pom.xml") {
				score += 40
			}
		case "gradle":
			if strings.Contains(file, "build.gradle") {
				score += 40
			}
		case "tomcat":
			if strings.Contains(file, "web.xml") || strings.Contains(file, ".jsp") {
				score += 30
			}
		case "javascript":
			if strings.Contains(file, "package.json") {
				score += 40
			}
		case "python":
			if strings.Contains(file, "requirements.txt") || strings.Contains(file, "pyproject.toml") {
				score += 40
			}
		case "gomodule":
			if strings.Contains(file, "go.mod") {
				score += 40
			}
		}
	}

	// Framework match
	if framework != "" {
		switch templateType {
		case "maven":
			if strings.Contains(strings.ToLower(framework), "spring") {
				score += 20
			}
		case "tomcat":
			if strings.Contains(strings.ToLower(framework), "servlet") {
				score += 20
			}
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// getAlternativeTemplates suggests alternatives with trade-offs
func (t *GenerateDockerfileTool) getAlternativeTemplates(language, framework string, dependencies []string) []AlternativeTemplate {
	alternatives := make([]AlternativeTemplate, 0)

	if strings.ToLower(language) == "java" {
		// Suggest distroless for security-focused deployments
		alternatives = append(alternatives, AlternativeTemplate{
			Template: "custom-distroless",
			Reason:   "Maximum security with minimal attack surface",
			TradeOffs: []string{
				"No shell access for debugging",
				"Requires careful dependency management",
				"May complicate troubleshooting",
			},
			UseCases: []string{
				"Production deployments",
				"Security-critical applications",
				"Compliance requirements",
			},
		})

		// Suggest JLink for size optimization
		alternatives = append(alternatives, AlternativeTemplate{
			Template: "custom-jlink",
			Reason:   "Minimal JRE with only required modules",
			TradeOffs: []string{
				"Requires Java 9+",
				"More complex build process",
				"Module dependency analysis needed",
			},
			UseCases: []string{
				"Microservices",
				"Size-constrained environments",
				"Serverless deployments",
			},
		})
	}

	return alternatives
}

// generateOptimizationContext creates optimization hints for the AI
func (t *GenerateDockerfileTool) generateOptimizationContext(content string, args GenerateDockerfileArgs) *OptimizationContext {
	ctx := &OptimizationContext{
		OptimizationGoals: make([]string, 0),
		SuggestedChanges:  make([]OptimizationChange, 0),
		SecurityConcerns:  make([]SecurityConcern, 0),
		BestPractices:     make([]string, 0),
	}

	// Analyze the Dockerfile content
	lines := strings.Split(content, "\n")

	// Check for security issues
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Running as root
		if strings.HasPrefix(trimmed, "USER") && strings.Contains(trimmed, "root") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "Container runs as root user",
				Severity:   "high",
				Suggestion: "Add a non-root user and switch to it before the entrypoint",
				Reference:  "CIS Docker Benchmark 4.1",
			})
		}

		// Exposed SSH port
		if strings.HasPrefix(trimmed, "EXPOSE") && strings.Contains(trimmed, "22") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "SSH port exposed",
				Severity:   "medium",
				Suggestion: "Avoid SSH in containers; use kubectl exec or docker exec instead",
				Reference:  "Container security best practices",
			})
		}

		// Using latest tags
		if strings.HasPrefix(trimmed, "FROM") && strings.Contains(trimmed, ":latest") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "Using :latest tag",
				Severity:   "medium",
				Suggestion: "Pin to specific version for reproducible builds",
				Reference:  "Docker best practices",
			})
		}
	}

	// Add optimization suggestions based on the optimization parameter
	switch args.Optimization {
	case "size":
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Minimize image size")
		ctx.SuggestedChanges = append(ctx.SuggestedChanges,
			OptimizationChange{
				Type:        "size",
				Description: "Use multi-stage builds to reduce final image size",
				Impact:      "Can reduce image size by 50-90%",
				Example:     "Copy only runtime artifacts from build stage",
			},
			OptimizationChange{
				Type:        "size",
				Description: "Use Alpine-based images where possible",
				Impact:      "Alpine images are ~5MB vs ~100MB for Ubuntu",
				Example:     "FROM node:18-alpine instead of FROM node:18",
			},
		)
	case "security":
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Maximize security")
		ctx.SuggestedChanges = append(ctx.SuggestedChanges,
			OptimizationChange{
				Type:        "security",
				Description: "Use distroless or minimal base images",
				Impact:      "Reduces attack surface by removing shell and package managers",
				Example:     "FROM gcr.io/distroless/java:11",
			},
			OptimizationChange{
				Type:        "security",
				Description: "Run as non-root user",
				Impact:      "Prevents privilege escalation attacks",
				Example:     "USER 1000:1000",
			},
		)
	case "speed":
		ctx.OptimizationGoals = append(ctx.OptimizationGoals, "Optimize build speed")
		ctx.SuggestedChanges = append(ctx.SuggestedChanges,
			OptimizationChange{
				Type:        "performance",
				Description: "Order Dockerfile commands for better layer caching",
				Impact:      "Reduces rebuild time by 60-80%",
				Example:     "COPY package*.json first, then RUN npm install, then COPY source",
			},
		)
	}

	// Add general best practices
	ctx.BestPractices = append(ctx.BestPractices,
		"Use .dockerignore to exclude unnecessary files",
		"Combine RUN commands to reduce layers",
		"Clean up package manager caches in the same RUN command",
		"Use COPY instead of ADD unless you need auto-extraction",
		"Set WORKDIR instead of using cd commands",
		"Use exec form for CMD and ENTRYPOINT for proper signal handling",
	)

	return ctx
}

// Unified Interface Implementation
// These methods implement the mcptypes.Tool interface for unified tool handling

// GetMetadata returns comprehensive tool metadata
func (t *GenerateDockerfileTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "generate_dockerfile_atomic",
		Description: "Generates optimized Dockerfiles based on repository analysis with language-specific templates and best practices",
		Version:     "1.0.0",
		Category:    "containerization",
		Dependencies: []string{
			"session_manager",
			"repository_analysis",
		},
		Capabilities: []string{
			"dockerfile_generation",
			"language_detection",
			"template_selection",
			"optimization_recommendations",
			"multi_stage_builds",
			"security_hardening",
		},
		Requirements: []string{
			"valid_session_id",
			"analyzed_repository",
		},
		Parameters: map[string]string{
			"session_id":     "string - Session ID for session context",
			"language":       "string - Programming language (optional, auto-detected if not provided)",
			"framework":      "string - Framework name (optional, auto-detected if not provided)",
			"optimization":   "string - Optimization focus: size, security, speed (default: balanced)",
			"use_multistage": "bool - Use multi-stage builds for optimization (default: true)",
			"base_image":     "string - Custom base image (optional, uses language defaults)",
			"port":           "int - Application port (default: language-specific)",
			"dry_run":        "bool - Generate preview without creating files",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Auto-detected Node.js Application",
				Description: "Generate Dockerfile for a Node.js application with auto-detection",
				Input: map[string]interface{}{
					"session_id": "session-123",
				},
				Output: map[string]interface{}{
					"success":         true,
					"language":        "javascript",
					"framework":       "node",
					"dockerfile_path": "/workspace/Dockerfile",
					"optimization":    "balanced",
				},
			},
			{
				Name:        "Python Flask with Size Optimization",
				Description: "Generate optimized Dockerfile for Python Flask application",
				Input: map[string]interface{}{
					"session_id":   "session-456",
					"language":     "python",
					"framework":    "flask",
					"optimization": "size",
					"port":         5000,
				},
				Output: map[string]interface{}{
					"success":      true,
					"language":     "python",
					"framework":    "flask",
					"optimization": "size",
					"multistage":   true,
					"base_image":   "python:3.11-alpine",
				},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *GenerateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	dockerfileArgs, ok := args.(GenerateDockerfileArgs)
	if !ok {
		// Try to convert from map if it's not already typed
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			dockerfileArgs, err = convertToGenerateDockerfileArgs(mapArgs)
			if err != nil {
				return types.NewRichError("CONVERSION_ERROR", fmt.Sprintf("failed to convert arguments: %v", err), types.ErrTypeValidation)
			}
		} else {
			return types.NewRichError("INVALID_ARGUMENTS", "invalid argument type for generate_dockerfile_atomic", types.ErrTypeValidation)
		}
	}

	if dockerfileArgs.SessionID == "" {
		return types.NewRichError("MISSING_REQUIRED_FIELD", "session_id is required", types.ErrTypeValidation)
	}

	// Validate optimization type if provided
	if dockerfileArgs.Optimization != "" {
		validOptimizations := []string{"size", "security", "speed", "balanced"}
		valid := false
		for _, opt := range validOptimizations {
			if dockerfileArgs.Optimization == opt {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewRichError("INVALID_OPTIMIZATION", fmt.Sprintf("optimization must be one of: %v, got: %s", validOptimizations, dockerfileArgs.Optimization), types.ErrTypeValidation)
		}
	}

	return nil
}

// Execute implements the generic Tool interface
func (t *GenerateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Handle both typed and untyped arguments
	var dockerfileArgs GenerateDockerfileArgs
	var err error

	switch a := args.(type) {
	case GenerateDockerfileArgs:
		dockerfileArgs = a
	case map[string]interface{}:
		dockerfileArgs, err = convertToGenerateDockerfileArgs(a)
		if err != nil {
			return nil, types.NewRichError("CONVERSION_ERROR", fmt.Sprintf("failed to convert arguments: %v", err), types.ErrTypeValidation)
		}
	default:
		return nil, types.NewRichError("INVALID_ARGUMENTS", "invalid argument type for generate_dockerfile_atomic", types.ErrTypeValidation)
	}

	// Call the typed ExecuteTyped method
	return t.ExecuteTyped(ctx, dockerfileArgs)
}

// convertToGenerateDockerfileArgs converts untyped map to typed GenerateDockerfileArgs
func convertToGenerateDockerfileArgs(args map[string]interface{}) (GenerateDockerfileArgs, error) {
	result := GenerateDockerfileArgs{}

	if sessionID, ok := args["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if dryRun, ok := args["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}
	if template, ok := args["template"].(string); ok {
		result.Template = template
	}
	if optimization, ok := args["optimization"].(string); ok {
		result.Optimization = optimization
	}
	if baseImage, ok := args["base_image"].(string); ok {
		result.BaseImage = baseImage
	}
	if includeHealthCheck, ok := args["include_health_check"].(bool); ok {
		result.IncludeHealthCheck = includeHealthCheck
	}
	if platform, ok := args["platform"].(string); ok {
		result.Platform = platform
	}
	if buildArgs, ok := args["build_args"].(map[string]interface{}); ok {
		result.BuildArgs = make(map[string]string)
		for k, v := range buildArgs {
			if strVal, ok := v.(string); ok {
				result.BuildArgs[k] = strVal
			}
		}
	}

	return result, nil
}
