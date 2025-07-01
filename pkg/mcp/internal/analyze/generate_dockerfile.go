package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"

	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicGenerateDockerfileTool handles Dockerfile generation
type AtomicGenerateDockerfileTool struct {
	logger              zerolog.Logger
	sessionManager      core.ToolSessionManager
	templateSelector    *TemplateSelector
	optimizer           *DockerfileOptimizer
	templateIntegration *TemplateIntegration
}

// NewAtomicGenerateDockerfileTool creates a new Dockerfile generation tool
func NewAtomicGenerateDockerfileTool(sessionManager core.ToolSessionManager, logger zerolog.Logger) *AtomicGenerateDockerfileTool {
	return &AtomicGenerateDockerfileTool{
		logger:              logger,
		sessionManager:      sessionManager,
		templateSelector:    NewTemplateSelector(logger),
		optimizer:           NewDockerfileOptimizer(logger),
		templateIntegration: NewTemplateIntegration(logger),
	}
}

// ExecuteTyped executes the tool with typed arguments
func (t *AtomicGenerateDockerfileTool) ExecuteTyped(ctx context.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	// Get session state
	sessionState, err := t.getSessionState(args)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Select template
	templateName := t.selectTemplateFromSession(args, sessionState)

	// Prepare response
	response := &GenerateDockerfileResult{
		SessionID: args.SessionID,
		Template:  templateName,
	}

	// Handle dry run
	if args.DryRun {
		return t.handleDryRun(templateName, args, sessionState, response)
	}

	// Generate Dockerfile content
	if err := t.generateDockerfileContent(templateName, args, sessionState, response); err != nil {
		return nil, err
	}

	return response, nil
}

// getSessionState retrieves the session state
func (t *AtomicGenerateDockerfileTool) getSessionState(args GenerateDockerfileArgs) (map[string]interface{}, error) {
	if args.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sessionInterface, err := t.sessionManager.GetSession(args.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Convert to map for simplicity
	sessionMap := make(map[string]interface{})
	sessionMap["session_id"] = args.SessionID

	// Extract workspace directory and repository analysis from session
	if sessionState, ok := sessionInterface.(*session.SessionState); ok {
		sessionMap["work_dir"] = sessionState.WorkspaceDir
		sessionMap["repository_analysis"] = sessionState.RepoAnalysis
	} else {
		// Fallback for other session types
		sessionMap["work_dir"] = "/tmp"
		sessionMap["repository_analysis"] = make(map[string]interface{})
	}

	return sessionMap, nil
}

// selectTemplateFromSession selects the appropriate template based on session analysis
func (t *AtomicGenerateDockerfileTool) selectTemplateFromSession(args GenerateDockerfileArgs, session map[string]interface{}) string {
	// Use provided template if specified
	if args.Template != "" {
		return t.templateSelector.MapCommonTemplateNames(args.Template)
	}

	// Auto-select based on repository analysis
	return t.autoSelectTemplate(session)
}

// autoSelectTemplate automatically selects a template based on analysis
func (t *AtomicGenerateDockerfileTool) autoSelectTemplate(session map[string]interface{}) string {
	repoAnalysis, ok := session["repository_analysis"].(map[string]interface{})
	if !ok || repoAnalysis == nil {
		t.logger.Warn().Msg("No repository analysis available, using generic template")
		return "generic"
	}

	template, err := t.templateSelector.SelectTemplate(repoAnalysis)
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to select template, using generic")
		return "generic"
	}

	return template
}

// handleDryRun handles dry run mode
func (t *AtomicGenerateDockerfileTool) handleDryRun(templateName string, args GenerateDockerfileArgs, session map[string]interface{}, response *GenerateDockerfileResult) (*GenerateDockerfileResult, error) {
	repoAnalysis, _ := session["repository_analysis"].(map[string]interface{})
	content, err := t.previewDockerfile(templateName, args, repoAnalysis)
	if err != nil {
		return nil, fmt.Errorf("failed to preview Dockerfile: %w", err)
	}

	response.Content = content
	response.BaseImage = t.optimizer.ExtractBaseImage(content)
	response.ExposedPorts = t.optimizer.ExtractExposedPorts(content)
	response.BuildSteps = t.optimizer.ExtractBuildSteps(content)
	response.HealthCheck = t.optimizer.ExtractHealthCheck(content)
	response.Message = "Dockerfile preview generated (dry run mode)"

	// Generate rich context
	t.generateRichContext(repoAnalysis, content, args, response)

	return response, nil
}

// generateDockerfileContent generates and writes the Dockerfile
func (t *AtomicGenerateDockerfileTool) generateDockerfileContent(templateName string, args GenerateDockerfileArgs, session map[string]interface{}, response *GenerateDockerfileResult) error {
	// Determine Dockerfile path
	workDir, _ := session["work_dir"].(string)
	if workDir == "" {
		workDir = "/tmp"
	}
	dockerfilePath := filepath.Join(workDir, "Dockerfile")

	// Generate content
	repoAnalysis, _ := session["repository_analysis"].(map[string]interface{})
	content, err := t.generateDockerfile(templateName, dockerfilePath, args, repoAnalysis)
	if err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Populate response
	response.Content = content
	response.FilePath = dockerfilePath
	response.DockerfilePath = dockerfilePath
	response.BaseImage = t.optimizer.ExtractBaseImage(content)
	response.ExposedPorts = t.optimizer.ExtractExposedPorts(content)
	response.BuildSteps = t.optimizer.ExtractBuildSteps(content)
	response.HealthCheck = t.optimizer.ExtractHealthCheck(content)
	response.Message = fmt.Sprintf("Dockerfile generated successfully at %s", dockerfilePath)

	// Generate rich context
	t.generateRichContext(repoAnalysis, content, args, response)

	// Perform validation
	t.performValidation(context.Background(), content, args, response)

	return nil
}

// generateRichContext generates rich context for AI reasoning
func (t *AtomicGenerateDockerfileTool) generateRichContext(repositoryData map[string]interface{}, content string, args GenerateDockerfileArgs, response *GenerateDockerfileResult) {
	// Extract useful information
	language := ""
	framework := ""
	if lang, ok := repositoryData["primary_language"].(string); ok {
		language = lang
	}
	if fw, ok := repositoryData["framework"].(string); ok {
		framework = fw
	}

	dependencies := t.extractDependencies(repositoryData)
	configFiles := t.extractConfigFiles(repositoryData)

	// Generate template selection context
	response.TemplateSelection = t.templateSelector.GenerateTemplateSelectionContext(
		language, framework, dependencies, configFiles,
	)

	// Generate optimization context
	response.OptimizationHints = t.optimizer.GenerateOptimizationContext(content, args)
}

// extractDependencies extracts dependencies from repository data
func (t *AtomicGenerateDockerfileTool) extractDependencies(repositoryData map[string]interface{}) []string {
	var deps []string

	if depMap, ok := repositoryData["dependencies"].(map[string]interface{}); ok {
		for dep := range depMap {
			deps = append(deps, dep)
		}
	}

	return deps
}

// extractConfigFiles extracts configuration files from repository data
func (t *AtomicGenerateDockerfileTool) extractConfigFiles(repositoryData map[string]interface{}) []string {
	var files []string

	if fileList, ok := repositoryData["config_files"].([]interface{}); ok {
		for _, f := range fileList {
			if file, ok := f.(string); ok {
				files = append(files, file)
			}
		}
	}

	return files
}

// performValidation performs Dockerfile validation
func (t *AtomicGenerateDockerfileTool) performValidation(ctx context.Context, content string, args GenerateDockerfileArgs, response *GenerateDockerfileResult) {
	validationResult := t.optimizer.ValidateDockerfile(ctx, content)

	if validationResult != nil {
		response.Validation = validationResult

		if !validationResult.Valid {
			t.logger.Warn().
				Int("errors", len(validationResult.Errors)).
				Int("warnings", len(validationResult.Warnings)).
				Msg("Dockerfile validation found issues")
		}
	}
}

// ExecuteWithContext executes the tool with server context
func (t *AtomicGenerateDockerfileTool) ExecuteWithContext(serverCtx *server.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	return t.ExecuteTyped(context.Background(), args)
}

// previewDockerfile generates a preview of the Dockerfile
func (t *AtomicGenerateDockerfileTool) previewDockerfile(templateName string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Get template content
	content, err := t.templateIntegration.GetTemplateContent(templateName)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	// Get base image recommendation
	language := ""
	framework := ""
	if lang, ok := repoAnalysis["primary_language"].(string); ok {
		language = lang
	}
	if fw, ok := repoAnalysis["framework"].(string); ok {
		framework = fw
	}

	// Override base image if not provided
	if args.BaseImage == "" {
		args.BaseImage = t.templateSelector.GetRecommendedBaseImage(language, framework)
	}

	// Apply template
	content = t.templateIntegration.ApplyTemplate(content, map[string]string{
		"BASE_IMAGE": args.BaseImage,
		"LANGUAGE":   language,
		"FRAMEWORK":  framework,
	})

	// Apply customizations
	content = t.optimizer.ApplyCustomizations(content, args, repoAnalysis)

	// Add health check if requested
	if args.IncludeHealthCheck {
		content += "\n\n" + t.optimizer.GenerateHealthCheck()
	}

	return content, nil
}

// generateDockerfile generates and writes the Dockerfile
func (t *AtomicGenerateDockerfileTool) generateDockerfile(templateName, dockerfilePath string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Generate content
	content, err := t.previewDockerfile(templateName, args, repoAnalysis)
	if err != nil {
		return "", err
	}

	// Write to file
	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	t.logger.Info().
		Str("path", dockerfilePath).
		Str("template", templateName).
		Msg("Dockerfile generated successfully")

	return content, nil
}

// GetMetadata returns tool metadata
func (t *AtomicGenerateDockerfileTool) GetMetadata() core.ToolMetadata {
	return core.ToolMetadata{
		Name:        "generate_dockerfile",
		Description: "Intelligently generates a Dockerfile based on repository analysis stored in the session",
		Category:    "containerization",
		Version:     "2.0.0",
		Parameters: map[string]string{
			"session_id": "required",
			"template":   "optional",
		},
		Examples: []core.ToolExample{
			{
				Name:        "basic",
				Description: "Generate Dockerfile with auto-detection",
				Input:       map[string]interface{}{"session_id": "session-123"},
				Output:      map[string]interface{}{"message": "Dockerfile generated successfully"},
			},
		},
	}
}

// Validate validates the tool arguments
func (t *AtomicGenerateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	typedArgs, ok := args.(GenerateDockerfileArgs)
	if !ok {
		// Try to convert from map
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			typedArgs, err = convertToGenerateDockerfileArgs(mapArgs)
			if err != nil {
				return fmt.Errorf("invalid argument format: %w", err)
			}
		} else {
			return fmt.Errorf("invalid argument type: expected GenerateDockerfileArgs or map[string]interface{}")
		}
	}

	if typedArgs.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	// Validate template if provided
	if typedArgs.Template != "" {
		validTemplates := []string{"go", "node", "python", "java", "rust", "php", "ruby", "dotnet", "generic", "golang", "nodejs", "js"}
		found := false
		for _, valid := range validTemplates {
			if strings.EqualFold(typedArgs.Template, valid) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid template: %s", typedArgs.Template)
		}
	}

	return nil
}

// Execute executes the tool with generic arguments
func (t *AtomicGenerateDockerfileTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Convert args
	typedArgs, ok := args.(GenerateDockerfileArgs)
	if !ok {
		if mapArgs, ok := args.(map[string]interface{}); ok {
			var err error
			typedArgs, err = convertToGenerateDockerfileArgs(mapArgs)
			if err != nil {
				return nil, fmt.Errorf("failed to convert arguments: %w", err)
			}
		} else {
			return nil, fmt.Errorf("invalid argument type")
		}
	}

	// Execute
	return t.ExecuteTyped(ctx, typedArgs)
}

// convertToGenerateDockerfileArgs converts map arguments to typed arguments
func convertToGenerateDockerfileArgs(args map[string]interface{}) (GenerateDockerfileArgs, error) {
	result := GenerateDockerfileArgs{}

	if v, ok := args["session_id"].(string); ok {
		result.SessionID = v
	}
	if v, ok := args["base_image"].(string); ok {
		result.BaseImage = v
	}
	if v, ok := args["template"].(string); ok {
		result.Template = v
	}
	if v, ok := args["optimization"].(string); ok {
		result.Optimization = v
	}
	if v, ok := args["include_health_check"].(bool); ok {
		result.IncludeHealthCheck = v
	}
	if v, ok := args["platform"].(string); ok {
		result.Platform = v
	}
	if v, ok := args["dry_run"].(bool); ok {
		result.DryRun = v
	}

	// Handle build args
	if v, ok := args["build_args"].(map[string]interface{}); ok {
		result.BuildArgs = make(map[string]string)
		for k, val := range v {
			if strVal, ok := val.(string); ok {
				result.BuildArgs[k] = strVal
			}
		}
	}

	return result, nil
}
