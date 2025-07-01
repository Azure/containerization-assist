package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/core"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type GenerateDockerfileArgs struct {
	types.BaseToolArgs
	BaseImage          string            `json:"base_image,omitempty" description:"Override detected base image"`
	Template           string            `json:"template,omitempty" jsonschema:"enum=go,node,python,java,rust,php,ruby,dotnet,golang" description:"Use specific template (go, node, python, etc.)"`
	Optimization       string            `json:"optimization,omitempty" jsonschema:"enum=size,speed,security,balanced" description:"Optimization level (size, speed, security)"`
	IncludeHealthCheck bool              `json:"include_health_check,omitempty" description:"Add health check to Dockerfile"`
	BuildArgs          map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	Platform           string            `json:"platform,omitempty" jsonschema:"enum=linux/amd64,linux/arm64,linux/arm/v7" description:"Target platform (e.g., linux/amd64)"`
}

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

	TemplateSelection *TemplateSelectionContext `json:"template_selection,omitempty"`
	OptimizationHints *OptimizationContext      `json:"optimization_hints,omitempty"`
}

type TemplateSelectionContext struct {
	DetectedLanguage    string                `json:"detected_language"`
	DetectedFramework   string                `json:"detected_framework"`
	AvailableTemplates  []TemplateOption      `json:"available_templates"`
	RecommendedTemplate string                `json:"recommended_template"`
	SelectionReasoning  []string              `json:"selection_reasoning"`
	AlternativeOptions  []AlternativeTemplate `json:"alternative_options"`
}

type TemplateOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BestFor     []string `json:"best_for"`
	Limitations []string `json:"limitations"`
	MatchScore  int      `json:"match_score"`
}

type AlternativeTemplate struct {
	Template  string   `json:"template"`
	Reason    string   `json:"reason"`
	TradeOffs []string `json:"trade_offs"`
	UseCases  []string `json:"use_cases"`
}

type OptimizationContext struct {
	CurrentSize       string               `json:"current_size,omitempty"`
	OptimizationGoals []string             `json:"optimization_goals"`
	SuggestedChanges  []OptimizationChange `json:"suggested_changes"`
	SecurityConcerns  []SecurityConcern    `json:"security_concerns"`
	BestPractices     []string             `json:"best_practices"`
}

type OptimizationChange struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Example     string `json:"example,omitempty"`
}

type SecurityConcern struct {
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
	Reference  string `json:"reference,omitempty"`
}

type AtomicGenerateDockerfileTool struct {
	logger              zerolog.Logger
	validator           *coredocker.Validator
	hadolintValidator   *coredocker.HadolintValidator
	sessionManager      core.ToolSessionManager
	templateIntegration *TemplateIntegration
}

func NewAtomicGenerateDockerfileTool(sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicGenerateDockerfileTool {
	return &AtomicGenerateDockerfileTool{
		logger:              logger,
		validator:           coredocker.NewValidator(logger),
		hadolintValidator:   coredocker.NewHadolintValidator(logger),
		sessionManager:      sessionManager,
		templateIntegration: NewTemplateIntegration(logger),
	}
}

func (t *AtomicGenerateDockerfileTool) ExecuteTyped(ctx context.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	response := &GenerateDockerfileResult{
		BaseToolResponse: types.NewBaseResponse("generate_dockerfile", args.SessionID, args.DryRun),
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", args.Template).
		Str("optimization", args.Optimization).
		Bool("dry_run", args.DryRun).
		Msg("Starting Dockerfile generation")

	session, err := t.getSessionState(args)
	if err != nil {
		return nil, err
	}

	templateName := t.selectTemplateFromSession(args, session)
	t.logger.Info().Str("template", templateName).Msg("Selected Dockerfile template")

	if args.DryRun {
		return t.handleDryRun(templateName, args, session, response)
	}

	if err := t.generateDockerfileContent(templateName, args, session, response); err != nil {
		return nil, err
	}

	t.performValidation(ctx, response.Content, args, response)

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", templateName).
		Str("file_path", response.FilePath).
		Bool("validation_passed", response.Validation == nil || response.Validation.Valid).
		Msg("Successfully generated Dockerfile")

	return response, nil
}

// getSessionState retrieves and validates the session state
func (t *AtomicGenerateDockerfileTool) getSessionState(args GenerateDockerfileArgs) (*core.SessionState, error) {
	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session %s: %w", args.SessionID, err)
	}

	sessionState, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return nil, fmt.Errorf("invalid session type: expected *session.SessionState, got %T", sessionInterface)
	}

	return sessionState.ToCoreSessionState(), nil
}

// selectTemplateFromSession determines template name from session analysis or user input
func (t *AtomicGenerateDockerfileTool) selectTemplateFromSession(args GenerateDockerfileArgs, session *core.SessionState) string {
	templateName := args.Template

	if templateName == "" {
		templateName = t.autoSelectTemplate(session)
	} else {
		templateName = t.mapCommonTemplateNames(templateName)
	}

	return templateName
}

// autoSelectTemplate automatically selects template based on repository analysis
func (t *AtomicGenerateDockerfileTool) autoSelectTemplate(session *core.SessionState) string {
	var repositoryData map[string]interface{}
	if session.Metadata != nil {
		if scanSummary, exists := session.Metadata["scan_summary"].(map[string]interface{}); exists {
			repositoryData = scanSummary
		}
	}

	if len(repositoryData) > 0 {
		selectedTemplate, err := t.selectTemplate(repositoryData)
		if err != nil {
			t.logger.Warn().Err(err).Msg("Failed to auto-select template, using generic dockerfile-python template")
			return "dockerfile-python"
		}
		return selectedTemplate
	}

	t.logger.Warn().Msg("No repository analysis found, using generic dockerfile-python template")
	return "dockerfile-python"
}

// handleDryRun processes dry-run mode and returns the preview response
func (t *AtomicGenerateDockerfileTool) handleDryRun(templateName string, args GenerateDockerfileArgs, session *core.SessionState, response *GenerateDockerfileResult) (*GenerateDockerfileResult, error) {
	var repositoryData map[string]interface{}
	if session.Metadata != nil {
		if scanSummary, exists := session.Metadata["scan_summary"].(map[string]interface{}); exists {
			repositoryData = scanSummary
		}
	}

	content, err := t.previewDockerfile(templateName, args, repositoryData)
	if err != nil {
		return nil, fmt.Errorf("failed to preview Dockerfile with template %s: %w", templateName, err)
	}

	response.Content = content
	response.Template = templateName
	response.BuildSteps = t.extractBuildSteps(content)
	response.ExposedPorts = t.extractExposedPorts(content)
	response.BaseImage = t.extractBaseImage(content)

	return response, nil
}

// generateDockerfileContent generates the actual Dockerfile content and metadata
func (t *AtomicGenerateDockerfileTool) generateDockerfileContent(templateName string, args GenerateDockerfileArgs, session *core.SessionState, response *GenerateDockerfileResult) error {
	// Use session workspace directory for Dockerfile
	dockerfilePath := filepath.Join(session.WorkspaceDir, "Dockerfile")
	repositoryData := make(map[string]interface{})
	if session.Metadata != nil {
		if scanSummary, exists := session.Metadata["scan_summary"].(map[string]interface{}); exists {
			repositoryData = scanSummary
		}
	}

	content, err := t.generateDockerfile(templateName, dockerfilePath, args, repositoryData)
	if err != nil {
		return fmt.Errorf("failed to generate Dockerfile with template %s at path %s: %w", templateName, dockerfilePath, err)
	}

	response.Content = content
	response.Template = templateName
	response.FilePath = dockerfilePath
	response.BuildSteps = t.extractBuildSteps(content)
	response.ExposedPorts = t.extractExposedPorts(content)
	response.BaseImage = t.extractBaseImage(content)

	if args.IncludeHealthCheck {
		response.HealthCheck = t.extractHealthCheck(content)
	}

	t.generateRichContext(repositoryData, content, args, response)

	return nil
}

func (t *AtomicGenerateDockerfileTool) generateRichContext(repositoryData map[string]interface{}, content string, args GenerateDockerfileArgs, response *GenerateDockerfileResult) {
	if len(repositoryData) > 0 {
		language, _ := repositoryData["language"].(string)
		framework, _ := repositoryData["framework"].(string)

		dependencies := t.extractDependencies(repositoryData)
		configFiles := t.extractConfigFiles(repositoryData)

		response.TemplateSelection = t.generateTemplateSelectionContext(language, framework, dependencies, configFiles)
	}

	response.OptimizationHints = t.generateOptimizationContext(content, args)
}

func (t *AtomicGenerateDockerfileTool) extractDependencies(repositoryData map[string]interface{}) []string {
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
	return dependencies
}

func (t *AtomicGenerateDockerfileTool) extractConfigFiles(repositoryData map[string]interface{}) []string {
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
	return configFiles
}

func (t *AtomicGenerateDockerfileTool) performValidation(ctx context.Context, content string, args GenerateDockerfileArgs, response *GenerateDockerfileResult) {
	validationResult := t.validateDockerfile(ctx, content)
	response.Validation = validationResult

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
}

func (t *AtomicGenerateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	t.logger.Info().Msg("Initializing Dockerfile generation")

	result, err := t.ExecuteTyped(context.Background(), args)

	if err != nil {
		t.logger.Info().Msg("Dockerfile generation failed")
		return result, nil
	} else {
		t.logger.Info().Msg("Dockerfile generation completed successfully")
	}

	return result, nil
}

func (t *AtomicGenerateDockerfileTool) selectTemplate(repoAnalysis map[string]interface{}) (string, error) {
	language, ok := repoAnalysis["language"].(string)
	if !ok {
		return "", fmt.Errorf("error")
	}

	var configFiles []string
	var dependencies []string

	if files, ok := repoAnalysis["files"].([]interface{}); ok {
		for _, file := range files {
			if fileStr, ok := file.(string); ok {
				configFiles = append(configFiles, fileStr)
			}
		}
	}

	if deps, ok := repoAnalysis["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			switch d := dep.(type) {
			case string:
				dependencies = append(dependencies, d)
			case map[string]interface{}:
				if name, ok := d["Name"].(string); ok {
					dependencies = append(dependencies, name)
				}
			}
		}
	}

	framework := ""
	if fw, ok := repoAnalysis["framework"].(string); ok {
		framework = fw
	}

	templateEngine := coredocker.NewTemplateEngine(t.logger)
	templateName, _, err := templateEngine.SuggestTemplate(language, framework, dependencies, configFiles)
	if err != nil {
		return "", fmt.Errorf("error")
	}

	t.logger.Info().
		Str("language", language).
		Str("framework", framework).
		Str("selected_template", templateName).
		Msg("Template selected by engine")

	return templateName, nil
}

func (t *AtomicGenerateDockerfileTool) getRecommendedBaseImage(language, framework string) string {
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
func (t *AtomicGenerateDockerfileTool) previewDockerfile(templateName string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Use the core template engine to generate preview
	templateEngine := coredocker.NewTemplateEngine(t.logger)

	// Create a temporary directory for preview
	tempDir, err := os.MkdirTemp("", "dockerfile-preview-*")
	if err != nil {
		return "", fmt.Errorf("error")
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
		return "", fmt.Errorf("error")
	}

	if !result.Success {
		if result.Error != nil {
			return "", fmt.Errorf("error")
		}
		return "", fmt.Errorf("error")
	}

	dockerfileContent := result.Dockerfile
	dockerfileContent = t.applyCustomizations(dockerfileContent, args, repoAnalysis)

	return dockerfileContent, nil
}

func (t *AtomicGenerateDockerfileTool) generateDockerfile(templateName, dockerfilePath string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	targetDir := filepath.Dir(dockerfilePath)
	templateEngine := coredocker.NewTemplateEngine(t.logger)

	result, err := templateEngine.GenerateFromTemplate(templateName, targetDir)
	if err != nil {
		return "", fmt.Errorf("error")
	}

	if !result.Success {
		if result.Error != nil {
			return "", fmt.Errorf("error")
		}
		return "", fmt.Errorf("error")
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("error")
	}

	dockerfileContent := string(content)
	dockerfileContent = t.applyCustomizations(dockerfileContent, args, repoAnalysis)

	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0o644); err != nil {
		return "", fmt.Errorf("error")
	}

	return dockerfileContent, nil
}

func (t *AtomicGenerateDockerfileTool) applyCustomizations(content string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) string {
	lines := strings.Split(content, "\n")
	var result []string

	baseImageToUse := args.BaseImage
	if baseImageToUse == "" && repoAnalysis != nil {
		language, _ := repoAnalysis["language"].(string)
		framework, _ := repoAnalysis["framework"].(string)
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

	if args.IncludeHealthCheck {
		healthCheck := t.generateHealthCheck()
		if healthCheck != "" {
			for i, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "CMD ") || strings.HasPrefix(strings.TrimSpace(line), "ENTRYPOINT ") {
					result = append(result, lines[:i]...)
					result = append(result, "", healthCheck)
					result = append(result, lines[i:]...)
					return strings.Join(result, "\n")
				}
			}
			lines = append(lines, "", healthCheck)
		}
	}

	switch args.Optimization {
	case "size":
		lines = t.applySizeOptimizations(lines)
	case "security":
		lines = t.applySecurityOptimizations(lines)
	}

	return strings.Join(lines, "\n")
}

func (t *AtomicGenerateDockerfileTool) generateHealthCheck() string {
	port := 80
	return fmt.Sprintf("HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD curl -f http://localhost:%d/health || exit 1", port)
}

func (t *AtomicGenerateDockerfileTool) applySizeOptimizations(lines []string) []string {
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "RUN ") {
			if strings.Contains(trimmed, "apt-get") || strings.Contains(trimmed, "apk") {
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

func (t *AtomicGenerateDockerfileTool) applySecurityOptimizations(lines []string) []string {
	var result []string
	addedUser := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !addedUser && (strings.HasPrefix(trimmed, "CMD ") || strings.HasPrefix(trimmed, "ENTRYPOINT ")) {
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
			result = append(result, "")
			addedUser = true
		}

		result = append(result, line)

		if i == len(lines)-1 && !addedUser {
			result = append(result, "")
			result = append(result, "# Create non-root user")
			result = append(result, "RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup")
			result = append(result, "USER appuser")
		}
	}

	return result
}

func (t *AtomicGenerateDockerfileTool) extractBuildSteps(content string) []string {
	var steps []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

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

func (t *AtomicGenerateDockerfileTool) extractExposedPorts(content string) []int {
	var ports []int
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "EXPOSE ") {
			portStr := strings.TrimPrefix(trimmed, "EXPOSE ")
			portStr = strings.TrimSpace(portStr)

			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
				ports = append(ports, port)
			}
		}
	}

	return ports
}

func (t *AtomicGenerateDockerfileTool) extractBaseImage(content string) string {
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

func (t *AtomicGenerateDockerfileTool) extractHealthCheck(content string) string {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "HEALTHCHECK ") {
			return trimmed
		}
	}

	return ""
}

func (t *AtomicGenerateDockerfileTool) mapCommonTemplateNames(name string) string {
	templateMap := map[string]string{
		"java":        "dockerfile-maven",
		"java-web":    "dockerfile-java-tomcat",
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

	if mapped, exists := templateMap[strings.ToLower(name)]; exists {
		t.logger.Info().
			Str("input", name).
			Str("mapped", mapped).
			Msg("Mapped template name")
		return mapped
	}

	if strings.HasPrefix(name, "dockerfile-") {
		return name
	}

	return name
}

func (t *AtomicGenerateDockerfileTool) validateDockerfile(ctx context.Context, content string) *coredocker.ValidationResult {
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

	return t.validator.ValidateDockerfile(content)
}

func (t *AtomicGenerateDockerfileTool) generateTemplateSelectionContext(language, framework string, dependencies, configFiles []string) *TemplateSelectionContext {
	ctx := &TemplateSelectionContext{
		DetectedLanguage:   language,
		DetectedFramework:  framework,
		AvailableTemplates: make([]TemplateOption, 0),
		SelectionReasoning: make([]string, 0),
		AlternativeOptions: make([]AlternativeTemplate, 0),
	}

	templateOptions := t.getTemplateOptions(language, framework, dependencies, configFiles)
	ctx.AvailableTemplates = templateOptions

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

	ctx.AlternativeOptions = t.getAlternativeTemplates(language, framework, dependencies)

	return ctx
}

func (t *AtomicGenerateDockerfileTool) getTemplateOptions(language, framework string, dependencies, configFiles []string) []TemplateOption {
	options := []TemplateOption{
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
		{
			Name:        "dockerfile-javascript",
			Description: "Node.js with npm/yarn optimization",
			BestFor:     []string{"Express apps", "React SSR", "Node.js APIs"},
			Limitations: []string{"Single-stage build", "No TypeScript compilation"},
			MatchScore:  t.calculateMatchScore("javascript", language, framework, configFiles),
		},
		{
			Name:        "dockerfile-python",
			Description: "Python with pip/poetry support",
			BestFor:     []string{"Django", "Flask", "FastAPI", "Data science apps"},
			Limitations: []string{"May need additional system dependencies"},
			MatchScore:  t.calculateMatchScore("python", language, framework, configFiles),
		},
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

	for i := 0; i < len(options)-1; i++ {
		for j := i + 1; j < len(options); j++ {
			if options[j].MatchScore > options[i].MatchScore {
				options[i], options[j] = options[j], options[i]
			}
		}
	}

	return options
}

func (t *AtomicGenerateDockerfileTool) calculateMatchScore(templateType, language, framework string, configFiles []string) int {
	score := t.scoreLanguageMatch(templateType, language)
	score += t.scoreConfigFileMatch(templateType, configFiles)
	score += t.scoreFrameworkMatch(templateType, framework)

	if score > 100 {
		score = 100
	}

	return score
}

func (t *AtomicGenerateDockerfileTool) scoreLanguageMatch(templateType, language string) int {
	langLower := strings.ToLower(language)

	switch templateType {
	case "maven", "gradle", types.AppServerTomcat:
		if langLower == "java" {
			return 40
		}
	case "javascript":
		if langLower == "javascript" || langLower == "typescript" {
			return 40
		}
	case "python":
		if langLower == "python" {
			return 40
		}
	case "go", "gomodule":
		if langLower == "go" {
			return 40
		}
	}

	return 0
}

func (t *AtomicGenerateDockerfileTool) scoreConfigFileMatch(templateType string, configFiles []string) int {
	score := 0

	for _, file := range configFiles {
		score += t.getConfigFileScore(templateType, file)
	}

	return score
}

func (t *AtomicGenerateDockerfileTool) getConfigFileScore(templateType, file string) int {
	switch templateType {
	case "maven":
		if strings.Contains(file, "pom.xml") {
			return 40
		}
	case "gradle":
		if strings.Contains(file, "build.gradle") {
			return 40
		}
	case "tomcat":
		if strings.Contains(file, "web.xml") || strings.Contains(file, ".jsp") {
			return 30
		}
	case "javascript":
		if strings.Contains(file, "package.json") {
			return 40
		}
	case "python":
		if strings.Contains(file, "requirements.txt") || strings.Contains(file, "pyproject.toml") {
			return 40
		}
	case "gomodule":
		if strings.Contains(file, "go.mod") {
			return 40
		}
	}

	return 0
}

func (t *AtomicGenerateDockerfileTool) scoreFrameworkMatch(templateType, framework string) int {
	if framework == "" {
		return 0
	}

	frameworkLower := strings.ToLower(framework)

	switch templateType {
	case "maven":
		if strings.Contains(frameworkLower, "spring") {
			return 20
		}
	case "tomcat":
		if strings.Contains(frameworkLower, "servlet") {
			return 20
		}
	}

	return 0
}

func (t *AtomicGenerateDockerfileTool) getAlternativeTemplates(language, framework string, dependencies []string) []AlternativeTemplate {
	alternatives := make([]AlternativeTemplate, 0)

	if strings.ToLower(language) == "java" {
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

func (t *AtomicGenerateDockerfileTool) generateOptimizationContext(content string, args GenerateDockerfileArgs) *OptimizationContext {
	ctx := &OptimizationContext{
		OptimizationGoals: make([]string, 0),
		SuggestedChanges:  make([]OptimizationChange, 0),
		SecurityConcerns:  make([]SecurityConcern, 0),
		BestPractices:     make([]string, 0),
	}

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "USER") && strings.Contains(trimmed, "root") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "Container runs as root user",
				Severity:   "high",
				Suggestion: "Add a non-root user and switch to it before the entrypoint",
				Reference:  "CIS Docker Benchmark 4.1",
			})
		}

		if strings.HasPrefix(trimmed, "EXPOSE") && strings.Contains(trimmed, "22") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "SSH port exposed",
				Severity:   "medium",
				Suggestion: "Avoid SSH in containers; use kubectl exec or docker exec instead",
				Reference:  "Container security best practices",
			})
		}

		if strings.HasPrefix(trimmed, "FROM") && strings.Contains(trimmed, ":latest") {
			ctx.SecurityConcerns = append(ctx.SecurityConcerns, SecurityConcern{
				Issue:      "Using :latest tag",
				Severity:   "medium",
				Suggestion: "Pin to specific version for reproducible builds",
				Reference:  "Docker best practices",
			})
		}
	}

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

func (t *AtomicGenerateDockerfileTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
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
		Examples: []core.ToolExample{
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

func (t *AtomicGenerateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	dockerfileArgs, ok := args.(GenerateDockerfileArgs)
	if !ok {
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			dockerfileArgs, err = convertToGenerateDockerfileArgs(mapArgs)
			if err != nil {
				return fmt.Errorf("failed to convert arguments to GenerateDockerfileArgs: %w", err)
			}
		} else {
			return fmt.Errorf("invalid argument type: expected GenerateDockerfileArgs or map[string]interface{}, got %T", args)
		}
	}

	// Session ID is now optional - will be auto-generated if empty

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
			return fmt.Errorf("invalid optimization parameter '%s': must be one of [size, security, speed, balanced]", dockerfileArgs.Optimization)
		}
	}

	return nil
}

func (t *AtomicGenerateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	var dockerfileArgs GenerateDockerfileArgs
	var err error

	switch a := args.(type) {
	case GenerateDockerfileArgs:
		dockerfileArgs = a
	case map[string]interface{}:
		dockerfileArgs, err = convertToGenerateDockerfileArgs(a)
		if err != nil {
			return nil, fmt.Errorf("error")
		}
	default:
		return nil, fmt.Errorf("error")
	}

	return t.ExecuteTyped(ctx, dockerfileArgs)
}

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
