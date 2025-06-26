package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/templates"
	"github.com/rs/zerolog"
)

// TemplateEngine provides mechanical Dockerfile template operations
type TemplateEngine struct {
	logger zerolog.Logger
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine(logger zerolog.Logger) *TemplateEngine {
	return &TemplateEngine{
		logger: logger.With().Str("component", "docker_template_engine").Logger(),
	}
}

// TemplateInfo contains information about a Dockerfile template
type TemplateInfo struct {
	Name        string   `json:"name"`
	Language    string   `json:"language,omitempty"`
	Framework   string   `json:"framework,omitempty"`
	Description string   `json:"description,omitempty"`
	Files       []string `json:"files"`
}

// GenerateResult contains the result of Dockerfile generation
type GenerateResult struct {
	Success      bool                   `json:"success"`
	Template     string                 `json:"template"`
	Dockerfile   string                 `json:"dockerfile"`
	DockerIgnore string                 `json:"dockerignore,omitempty"`
	Suggestions  []string               `json:"suggestions"`
	Context      map[string]interface{} `json:"context"`
	Error        *GenerateError         `json:"error,omitempty"`
}

// GenerateError provides detailed error information
type GenerateError struct {
	Type     string                 `json:"type"`
	Message  string                 `json:"message"`
	Template string                 `json:"template,omitempty"`
	Context  map[string]interface{} `json:"context"`
}

// ListAvailableTemplates returns all available Dockerfile templates
func (te *TemplateEngine) ListAvailableTemplates() ([]TemplateInfo, error) {
	templateNames, err := te.listEmbeddedSubdirNames("dockerfiles")
	if err != nil {
		return nil, fmt.Errorf("failed to list dockerfile templates: %w", err)
	}

	templates := make([]TemplateInfo, 0, len(templateNames))
	for _, name := range templateNames {
		info, err := te.getTemplateInfo(name)
		if err != nil {
			te.logger.Warn().Err(err).Str("template", name).Msg("Failed to get template info")
			continue
		}
		templates = append(templates, info)
	}

	return templates, nil
}

// GenerateFromTemplate generates a Dockerfile from a specific template
func (te *TemplateEngine) GenerateFromTemplate(templateName string, targetDir string) (*GenerateResult, error) {
	result := &GenerateResult{
		Template:    templateName,
		Suggestions: make([]string, 0),
		Context:     make(map[string]interface{}),
	}

	te.logger.Info().Str("template", templateName).Str("target_dir", targetDir).Msg("Generating Dockerfile from template")

	// Validate template exists
	templates, err := te.ListAvailableTemplates()
	if err != nil {
		result.Error = &GenerateError{
			Type:    "template_list_error",
			Message: fmt.Sprintf("Failed to list templates: %v", err),
			Context: map[string]interface{}{
				"target_dir": targetDir,
			},
		}
		return result, nil
	}

	templateExists := false
	var templateInfo TemplateInfo
	for _, tmpl := range templates {
		if tmpl.Name == templateName {
			templateExists = true
			templateInfo = tmpl
			break
		}
	}

	if !templateExists {
		availableNames := make([]string, len(templates))
		for i, tmpl := range templates {
			availableNames[i] = tmpl.Name
		}

		result.Error = &GenerateError{
			Type:     "template_not_found",
			Message:  fmt.Sprintf("Template '%s' not found", templateName),
			Template: templateName,
			Context: map[string]interface{}{
				"available_templates": availableNames,
				"target_dir":          targetDir,
			},
		}
		return result, nil
	}

	// Validate target directory
	if err := te.validateTargetDirectory(targetDir); err != nil {
		result.Error = &GenerateError{
			Type:     "target_dir_error",
			Message:  err.Error(),
			Template: templateName,
			Context: map[string]interface{}{
				"target_dir": targetDir,
			},
		}
		return result, nil
	}

	// Generate files from template
	dockerfileContent, dockerignoreContent, err := te.writeFilesFromTemplate(templateName, targetDir)
	if err != nil {
		result.Error = &GenerateError{
			Type:     "template_generation_error",
			Message:  fmt.Sprintf("Failed to generate from template: %v", err),
			Template: templateName,
			Context: map[string]interface{}{
				"target_dir":     targetDir,
				"template_files": templateInfo.Files,
			},
		}
		return result, nil
	}

	// Success
	result.Success = true
	result.Dockerfile = dockerfileContent
	result.DockerIgnore = dockerignoreContent
	result.Context = map[string]interface{}{
		"target_dir":     targetDir,
		"template_files": templateInfo.Files,
		"language":       templateInfo.Language,
		"framework":      templateInfo.Framework,
	}

	// Add suggestions based on template
	result.Suggestions = te.generateSuggestions(templateInfo, targetDir)

	te.logger.Info().
		Str("template", templateName).
		Int("dockerfile_size", len(dockerfileContent)).
		Msg("Successfully generated Dockerfile from template")

	return result, nil
}

// SuggestTemplate suggests the best template based on simple heuristics
// This replaces AI template selection with rule-based logic
func (te *TemplateEngine) SuggestTemplate(language string, framework string, dependencies []string, configFiles []string) (string, []string, error) {
	_ = make([]string, 0) // suggestions currently unused, but keeping for future use

	// Simple rule-based template selection
	switch strings.ToLower(language) {
	case "javascript", "typescript", "node":
		if te.containsAny(dependencies, []string{"next", "nextjs"}) {
			return "nextjs", []string{"Optimized for Next.js applications", "Includes production build optimization"}, nil
		}
		if te.containsAny(dependencies, []string{"react"}) {
			return "react", []string{"Optimized for React applications", "Includes build and serve stages"}, nil
		}
		return "nodejs", []string{"General Node.js template", "Good for Express and other Node.js apps"}, nil

	case "python":
		if te.containsAny(dependencies, []string{"flask"}) {
			return "python-flask", []string{"Optimized for Flask applications", "Includes Python best practices"}, nil
		}
		if te.containsAny(dependencies, []string{"django"}) {
			return "python-django", []string{"Optimized for Django applications", "Includes static file handling"}, nil
		}
		if te.containsAny(configFiles, []string{"requirements.txt", "pyproject.toml", "poetry.lock"}) {
			return "python", []string{"General Python template", "Supports pip, poetry, and conda"}, nil
		}
		return "python", []string{"General Python template"}, nil

	case "java":
		// Check for Java web applications first (Tomcat, JSP, Servlets)
		if te.containsAny(configFiles, []string{"web.xml"}) ||
			te.containsAny(dependencies, []string{"javax.servlet", "jakarta.servlet", "tomcat"}) ||
			te.hasFileExtension(configFiles, ".jsp") ||
			te.containsAny(configFiles, []string{"WEB-INF"}) {
			return "dockerfile-java-tomcat", []string{"Optimized for Java web applications", "Includes Tomcat server", "Supports WAR deployment"}, nil
		}

		if te.containsAny(configFiles, []string{"pom.xml"}) {
			return "dockerfile-maven", []string{"Optimized for Maven projects", "Multi-stage build for efficiency"}, nil
		}
		if te.containsAny(configFiles, []string{"build.gradle", "build.gradle.kts"}) {
			return "dockerfile-gradle", []string{"Optimized for Gradle projects", "Multi-stage build for efficiency"}, nil
		}
		return "dockerfile-maven", []string{"Default Java Maven template"}, nil

	case "go", "golang":
		return "golang", []string{"Optimized for Go applications", "Multi-stage build for minimal image size"}, nil

	case "rust":
		return "rust", []string{"Optimized for Rust applications", "Multi-stage build for minimal image size"}, nil

	case "php":
		return "php", []string{"Optimized for PHP applications", "Includes Apache and common extensions"}, nil

	case "ruby":
		return "ruby", []string{"Optimized for Ruby applications", "Includes Rails support"}, nil

	case "c#", "csharp", "dotnet":
		return "dotnet", []string{"Optimized for .NET applications", "Multi-stage build for efficiency"}, nil

	default:
		// Try to detect based on config files
		if te.containsAny(configFiles, []string{"package.json"}) {
			return "nodejs", []string{"Detected Node.js project from package.json"}, nil
		}
		if te.containsAny(configFiles, []string{"requirements.txt", "setup.py", "pyproject.toml"}) {
			return "python", []string{"Detected Python project from dependency files"}, nil
		}
		if te.containsAny(configFiles, []string{"pom.xml"}) {
			return "java-maven", []string{"Detected Java Maven project"}, nil
		}
		if te.containsAny(configFiles, []string{"go.mod"}) {
			return "golang", []string{"Detected Go project from go.mod"}, nil
		}

		return "alpine", []string{
			"Using generic Alpine Linux template",
			"Consider specifying your language/framework for better optimization",
			"You may need to customize this template for your specific needs",
		}, nil
	}
}

// Helper methods

func (te *TemplateEngine) listEmbeddedSubdirNames(path string) ([]string, error) {
	entries, err := templates.Templates.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading embedded dir %q: %w", path, err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

func (te *TemplateEngine) getTemplateInfo(templateName string) (TemplateInfo, error) {
	basePath := filepath.Join("dockerfiles", templateName)

	info := TemplateInfo{
		Name:  templateName,
		Files: make([]string, 0),
	}

	// Check what files exist in this template
	filesToCheck := []string{"Dockerfile", ".dockerignore"}
	for _, filename := range filesToCheck {
		embeddedPath := filepath.Join(basePath, filename)
		if _, err := templates.Templates.ReadFile(embeddedPath); err == nil {
			info.Files = append(info.Files, filename)
		}
	}

	// Infer language/framework from template name
	parts := strings.Split(templateName, "-")
	if len(parts) > 0 {
		info.Language = parts[0]
	}
	if len(parts) > 1 {
		info.Framework = parts[1]
	}

	return info, nil
}

func (te *TemplateEngine) validateTargetDirectory(targetDir string) error {
	if targetDir == "" {
		return fmt.Errorf("target directory is required")
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		return fmt.Errorf("target directory does not exist: %s", targetDir)
	}

	if !info.IsDir() {
		return fmt.Errorf("target path is not a directory: %s", targetDir)
	}

	return nil
}

func (te *TemplateEngine) writeFilesFromTemplate(templateName, targetDir string) (string, string, error) {
	basePath := filepath.Join("dockerfiles", templateName)
	filesToCopy := []string{"Dockerfile", ".dockerignore"}

	var dockerfileContent, dockerignoreContent string

	for _, filename := range filesToCopy {
		embeddedPath := filepath.Join(basePath, filename)
		data, err := templates.Templates.ReadFile(embeddedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", "", fmt.Errorf("reading embedded file %q: %w", embeddedPath, err)
		}

		destPath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return "", "", fmt.Errorf("writing file %q: %w", destPath, err)
		}

		// Store content for response
		if filename == "Dockerfile" {
			dockerfileContent = string(data)
		} else if filename == ".dockerignore" {
			dockerignoreContent = string(data)
		}
	}

	return dockerfileContent, dockerignoreContent, nil
}

func (te *TemplateEngine) generateSuggestions(templateInfo TemplateInfo, targetDir string) []string {
	suggestions := make([]string, 0)

	suggestions = append(suggestions, "Review the generated Dockerfile for your specific needs")

	if templateInfo.Language != "" {
		suggestions = append(suggestions, fmt.Sprintf("Template optimized for %s applications", templateInfo.Language))
	}

	if templateInfo.Framework != "" {
		suggestions = append(suggestions, fmt.Sprintf("Includes %s-specific optimizations", templateInfo.Framework))
	}

	suggestions = append(suggestions, "Consider adding health checks if your application supports them")
	suggestions = append(suggestions, "Verify the exposed port matches your application")
	suggestions = append(suggestions, "Test the build locally before deploying")

	return suggestions
}

func (te *TemplateEngine) containsAny(slice []string, items []string) bool {
	for _, item := range items {
		for _, s := range slice {
			if strings.Contains(strings.ToLower(s), strings.ToLower(item)) {
				return true
			}
		}
	}
	return false
}

func (te *TemplateEngine) hasFileExtension(files []string, extension string) bool {
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file), strings.ToLower(extension)) {
			return true
		}
	}
	return false
}
