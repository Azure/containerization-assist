package analyze

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/services"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/localrivet/gomcp/server"
)

// GenerateDockerfileArgs represents arguments for Dockerfile generation
type GenerateDockerfileArgs struct {
	SessionID          string                 `json:"session_id"`
	BaseImage          string                 `json:"base_image,omitempty"`
	Template           string                 `json:"template,omitempty"`
	Optimization       string                 `json:"optimization,omitempty"`
	IncludeHealthCheck bool                   `json:"include_health_check,omitempty"`
	Platform           string                 `json:"platform,omitempty"`
	DryRun             bool                   `json:"dry_run,omitempty"`
	BuildArgs          map[string]string      `json:"build_args,omitempty"`
	RepoPath           string                 `json:"repo_path,omitempty"`
	Language           string                 `json:"language,omitempty"`
	Framework          string                 `json:"framework,omitempty"`
	Port               int                    `json:"port,omitempty"`
	BuildCommand       string                 `json:"build_command,omitempty"`
	RunCommand         string                 `json:"run_command,omitempty"`
	Dependencies       []string               `json:"dependencies,omitempty"`
	Customizations     map[string]interface{} `json:"customizations,omitempty"`
}

// GenerateDockerfileResult represents the result of Dockerfile generation
type GenerateDockerfileResult struct {
	SessionID         string               `json:"session_id"`
	Template          string               `json:"template"`
	Content           string               `json:"content"`
	FilePath          string               `json:"file_path"`
	DockerfilePath    string               `json:"dockerfile_path"`
	BaseImage         string               `json:"base_image"`
	ExposedPorts      []int                `json:"exposed_ports"`
	BuildSteps        []string             `json:"build_steps"`
	HealthCheck       string               `json:"health_check"`
	Message           string               `json:"message"`
	TemplateSelection interface{}          `json:"template_selection"`
	OptimizationHints *OptimizationContext `json:"optimization_hints"`
	Validation        interface{}          `json:"validation"`
}

// AtomicGenerateDockerfileTool handles Dockerfile generation
type AtomicGenerateDockerfileTool struct {
	logger              *slog.Logger
	sessionStore        services.SessionStore // Focused service interface
	sessionState        services.SessionState // Focused service interface
	templateSelector    *TemplateSelector
	optimizer           *DockerfileOptimizer
	templateIntegration *TemplateIntegration
}

// NewAtomicGenerateDockerfileTool creates a new Dockerfile generation tool using focused service interfaces
func NewAtomicGenerateDockerfileTool(sessionStore services.SessionStore, sessionState services.SessionState, logger *slog.Logger) *AtomicGenerateDockerfileTool {
	return &AtomicGenerateDockerfileTool{
		logger:              logger,
		sessionStore:        sessionStore,
		sessionState:        sessionState,
		templateSelector:    NewTemplateSelector(logger),
		optimizer:           NewDockerfileOptimizer(logger),
		templateIntegration: NewTemplateIntegration(logger),
	}
}

// NewAtomicGenerateDockerfileToolWithServices creates a new Dockerfile generation tool using service container
func NewAtomicGenerateDockerfileToolWithServices(serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicGenerateDockerfileTool {
	// Use focused services directly - no wrapper needed!
	return &AtomicGenerateDockerfileTool{
		logger:              logger,
		sessionStore:        serviceContainer.SessionStore(),
		sessionState:        serviceContainer.SessionState(),
		templateSelector:    NewTemplateSelector(logger),
		optimizer:           NewDockerfileOptimizer(logger),
		templateIntegration: NewTemplateIntegration(logger),
	}
}

// ExecuteWithContext executes the tool with the provided arguments
func (t *AtomicGenerateDockerfileTool) ExecuteWithContext(ctx *server.Context, args *GenerateDockerfileArgs) (*GenerateDockerfileResult, error) {
	// Get session state
	sessionState, err := t.getSessionState(*args)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session state").Cause(err).WithLocation(

		// Select template
		).Build()
	}

	templateName := t.selectTemplateFromSession(*args, sessionState)

	// Prepare response
	response := &GenerateDockerfileResult{
		SessionID: args.SessionID,
		Template:  templateName,
	}

	// Handle dry run
	if args.DryRun {
		return t.handleDryRun(templateName, *args, sessionState, response)
	}

	// Generate Dockerfile content
	if err := t.generateDockerfileContent(templateName, *args, sessionState, response); err != nil {
		return nil, err
	}

	return response, nil
}

// getSessionState retrieves the session state
func (t *AtomicGenerateDockerfileTool) getSessionState(args GenerateDockerfileArgs) (map[string]interface{}, error) {
	if args.SessionID == "" {
		return nil, errors.NewError().Messagef("session_id is required").WithLocation().Build()
	}

	// Get session using focused service interface
	sessionData, err := t.sessionStore.Get(context.Background(), args.SessionID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get session").Cause(err).WithLocation().Build()
	}

	sessionMap := make(map[string]interface{})
	sessionMap["session_id"] = args.SessionID
	// Extract work_dir and repository_analysis from metadata if available
	if workDir, ok := sessionData.Metadata["workspace_dir"]; ok {
		sessionMap["work_dir"] = workDir
	}
	if repoAnalysis, ok := sessionData.Metadata["repository_analysis"]; ok {
		sessionMap["repository_analysis"] = repoAnalysis
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
		t.logger.Warn("No repository analysis available, using generic template")
		return "generic"
	}

	template, err := t.templateSelector.SelectTemplate(repoAnalysis)
	if err != nil {
		t.logger.Error("Failed to select template, using generic", "error", err)
		return "generic"
	}

	return template
}

// handleDryRun handles dry run mode
func (t *AtomicGenerateDockerfileTool) handleDryRun(templateName string, args GenerateDockerfileArgs, session map[string]interface{}, response *GenerateDockerfileResult) (*GenerateDockerfileResult, error) {
	repoAnalysis, _ := session["repository_analysis"].(map[string]interface{})
	content, err := t.previewDockerfile(templateName, args, repoAnalysis)
	if err != nil {
		return nil, errors.NewError().Message("failed to preview Dockerfile").Cause(err).WithLocation().Build()
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
		return errors.NewError().Message("failed to generate Dockerfile").Cause(err).WithLocation().Build()
	}

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
			t.logger.Warn("Dockerfile validation found issues",
				"errors", len(validationResult.Errors),
				"warnings", len(validationResult.Warnings))
		}
	}
}

// previewDockerfile generates a preview of the Dockerfile
func (t *AtomicGenerateDockerfileTool) previewDockerfile(templateName string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error) {
	// Get template content
	content, err := t.templateIntegration.GetTemplateContent(templateName)
	if err != nil {
		return "", errors.NewError().Message("failed to get template").Cause(err).WithLocation().Build()
	}

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
		return "", errors.NewError().Message("failed to write Dockerfile").Cause(err).WithLocation().Build()
	}

	t.logger.Info("Dockerfile generated successfully",
		"path", dockerfilePath,
		"template", templateName)

	return content, nil
}

// GetMetadata returns tool metadata
func (t *AtomicGenerateDockerfileTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "generate_dockerfile",
		Description:  "Intelligently generates a Dockerfile based on repository analysis stored in the session",
		Category:     api.ToolCategory("containerization"),
		Tags:         []string{"dockerfile", "generation", "containerization"},
		Status:       api.ToolStatus("active"),
		Version:      "2.0.0",
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// Validate validates the tool arguments
func (t *AtomicGenerateDockerfileTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
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
				return nil, errors.NewError().Message("failed to convert arguments").Cause(err).Build()
			}
		} else {
			return nil, errors.NewError().Messagef("invalid argument type").WithLocation(

			// Execute
			).Build()
		}
	}

	return t.ExecuteWithContext(&server.Context{}, &typedArgs)
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
