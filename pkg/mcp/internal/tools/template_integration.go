package tools

import (
	"fmt"
	"reflect"
	"strings"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// TemplateIntegration provides template-based generation capabilities for atomic tools
type TemplateIntegration struct {
	dockerTemplateEngine *coredocker.TemplateEngine
	logger               zerolog.Logger
}

// NewTemplateIntegration creates a new template integration
func NewTemplateIntegration(logger zerolog.Logger) *TemplateIntegration {
	return &TemplateIntegration{
		dockerTemplateEngine: coredocker.NewTemplateEngine(logger),
		logger:               logger.With().Str("component", "template_integration").Logger(),
	}
}

// DockerfileTemplateContext provides enhanced template context for Dockerfile generation
type DockerfileTemplateContext struct {
	// Template selection
	SelectedTemplate    string                   `json:"selected_template"`
	TemplateInfo        *coredocker.TemplateInfo `json:"template_info"`
	SelectionMethod     string                   `json:"selection_method"` // "auto", "user", "fallback"
	SelectionConfidence float64                  `json:"selection_confidence"`

	// Available options
	AvailableTemplates []TemplateOptionInternal    `json:"available_templates"`
	AlternativeOptions []AlternativeTemplateOption `json:"alternative_options"`

	// Language/Framework detection
	DetectedLanguage     string   `json:"detected_language"`
	DetectedFramework    string   `json:"detected_framework"`
	DetectedDependencies []string `json:"detected_dependencies"`
	DetectedConfigFiles  []string `json:"detected_config_files"`

	// Template customization
	CustomizationOptions  map[string]interface{} `json:"customization_options"`
	AppliedCustomizations []string               `json:"applied_customizations"`

	// Reasoning
	SelectionReasoning []string `json:"selection_reasoning"`
	TradeOffs          []string `json:"trade_offs"`
}

// TemplateOptionInternal represents an available template with scoring for internal use
type TemplateOptionInternal struct {
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	Framework   string   `json:"framework,omitempty"`
	Description string   `json:"description"`
	MatchScore  float64  `json:"match_score"` // 0.0-1.0
	Strengths   []string `json:"strengths"`
	Limitations []string `json:"limitations"`
	BestFor     []string `json:"best_for"`
}

// AlternativeTemplateOption provides alternative template suggestions
type AlternativeTemplateOption struct {
	Template   string   `json:"template"`
	Reason     string   `json:"reason"`
	TradeOffs  []string `json:"trade_offs"`
	UseCases   []string `json:"use_cases"`
	Complexity string   `json:"complexity"` // "simple", "moderate", "complex"
	MatchScore float64  `json:"match_score"`
}

// ManifestTemplateContext provides enhanced template context for manifest generation
type ManifestTemplateContext struct {
	// Template selection
	SelectedTemplate string `json:"selected_template"`
	TemplateType     string `json:"template_type"` // "basic", "advanced", "gitops", "helm"
	SelectionMethod  string `json:"selection_method"`

	// Available options
	AvailableTemplates []ManifestTemplateOption `json:"available_templates"`

	// Application context
	ApplicationType    string `json:"application_type"`
	DeploymentStrategy string `json:"deployment_strategy"`
	ResourceProfile    string `json:"resource_profile"`

	// Customization
	CustomizationOptions map[string]interface{} `json:"customization_options"`
	GeneratedFiles       []string               `json:"generated_files"`

	// Reasoning
	SelectionReasoning []string `json:"selection_reasoning"`
	BestPractices      []string `json:"best_practices"`
}

// ManifestTemplateOption represents an available manifest template
type ManifestTemplateOption struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Components   []string `json:"components"` // deployment, service, configmap, etc.
	Features     []string `json:"features"`   // autoscaling, monitoring, etc.
	Complexity   string   `json:"complexity"`
	Requirements []string `json:"requirements"`
}

// SelectDockerfileTemplate selects the best Dockerfile template based on repository analysis
func (ti *TemplateIntegration) SelectDockerfileTemplate(repoInfo map[string]interface{}, userTemplate string) (*DockerfileTemplateContext, error) {
	context := &DockerfileTemplateContext{
		SelectionMethod:      "auto",
		CustomizationOptions: make(map[string]interface{}),
		SelectionReasoning:   make([]string, 0),
	}

	// Extract repository information
	language, _ := repoInfo["language"].(string)
	framework, _ := repoInfo["framework"].(string)

	// Extract dependencies
	var dependencies []string
	if deps, ok := repoInfo["dependencies"].([]interface{}); ok {
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

	// Extract config files
	var configFiles []string
	if files, ok := repoInfo["files"].([]interface{}); ok {
		for _, file := range files {
			if fileStr, ok := file.(string); ok {
				configFiles = append(configFiles, fileStr)
			}
		}
	}

	// Set detection results
	context.DetectedLanguage = language
	context.DetectedFramework = framework
	context.DetectedDependencies = dependencies
	context.DetectedConfigFiles = configFiles

	// Handle user-specified template
	if userTemplate != "" {
		context.SelectionMethod = "user"
		context.SelectedTemplate = ti.mapCommonTemplateNames(userTemplate)
		context.SelectionConfidence = 1.0
		context.SelectionReasoning = append(context.SelectionReasoning,
			fmt.Sprintf("User explicitly requested template: %s", userTemplate))
	} else {
		// Auto-select template
		selectedTemplate, reasons, err := ti.dockerTemplateEngine.SuggestTemplate(
			language, framework, dependencies, configFiles)
		if err != nil {
			ti.logger.Warn().Err(err).Msg("Failed to auto-select template, using fallback")
			context.SelectionMethod = "fallback"
			context.SelectedTemplate = "dockerfile-python" // Safe default
			context.SelectionConfidence = 0.3
			context.SelectionReasoning = append(context.SelectionReasoning,
				"Failed to auto-select template, using Python as fallback")
		} else {
			context.SelectedTemplate = selectedTemplate
			context.SelectionConfidence = 0.8 // Default high confidence for auto-selection
			if len(reasons) > 0 {
				context.SelectionReasoning = reasons
			} else {
				context.SelectionReasoning = ti.generateSelectionReasoning(
					language, framework, dependencies, selectedTemplate)
			}
		}
	}

	// Get template info by listing available templates
	availableTemplates, err := ti.dockerTemplateEngine.ListAvailableTemplates()
	if err == nil {
		for _, tmpl := range availableTemplates {
			if tmpl.Name == context.SelectedTemplate {
				context.TemplateInfo = &tmpl
				break
			}
		}
	}

	// Get available templates with scoring
	context.AvailableTemplates = ti.getAvailableDockerfileTemplates(language, framework, dependencies)

	// Generate alternative options
	context.AlternativeOptions = ti.generateAlternativeDockerfileOptions(
		context.SelectedTemplate, language, framework, dependencies)

	// Add trade-offs
	context.TradeOffs = ti.generateDockerfileTradeOffs(context.SelectedTemplate, language, framework)

	// Add customization options based on template
	context.CustomizationOptions = ti.generateDockerfileCustomizationOptions(
		context.SelectedTemplate, language, framework, dependencies)

	return context, nil
}

// SelectManifestTemplate selects the best manifest template based on application requirements
func (ti *TemplateIntegration) SelectManifestTemplate(args interface{}, repoInfo map[string]interface{}) (*ManifestTemplateContext, error) {
	context := &ManifestTemplateContext{
		SelectionMethod:      "auto",
		CustomizationOptions: make(map[string]interface{}),
		SelectionReasoning:   make([]string, 0),
		BestPractices:        make([]string, 0),
	}

	// Handle different argument types
	var port int
	var namespace string
	var replicas int
	var serviceType string
	var generateHelm bool
	var gitOpsReady bool
	var resourceProfile string
	var enableHPA bool
	var enableProbes bool
	var annotations map[string]string
	var labels map[string]string
	var deploymentStrategy string
	var envVars map[string]string

	// Use reflection to extract fields from any struct
	v := reflect.ValueOf(args)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, types.NewRichError(
			"INVALID_ARGUMENTS",
			fmt.Sprintf("args must be a struct, got %T", args),
			"validation_error",
		)
	}

	// Helper function to safely get field values
	getFieldValue := func(fieldName string, defaultVal interface{}) interface{} {
		field := v.FieldByName(fieldName)
		if !field.IsValid() || !field.CanInterface() {
			return defaultVal
		}
		return field.Interface()
	}

	// Extract values with fallbacks
	if portVal := getFieldValue("Port", 8080); portVal != nil {
		if p, ok := portVal.(int); ok {
			port = p
		}
	}
	if nsVal := getFieldValue("Namespace", ""); nsVal != nil {
		if ns, ok := nsVal.(string); ok {
			namespace = ns
		}
	}
	if repVal := getFieldValue("Replicas", 1); repVal != nil {
		if r, ok := repVal.(int); ok {
			replicas = r
		}
	}
	if stVal := getFieldValue("ServiceType", "ClusterIP"); stVal != nil {
		if st, ok := stVal.(string); ok {
			serviceType = st
		}
	}
	if ghVal := getFieldValue("GenerateHelm", false); ghVal != nil {
		if gh, ok := ghVal.(bool); ok {
			generateHelm = gh
		}
	}
	// Also try HelmTemplate field for compatibility
	if htVal := getFieldValue("HelmTemplate", false); htVal != nil {
		if ht, ok := htVal.(bool); ok {
			generateHelm = generateHelm || ht
		}
	}
	if grVal := getFieldValue("GitOpsReady", false); grVal != nil {
		if gr, ok := grVal.(bool); ok {
			gitOpsReady = gr
		}
	}
	if envVal := getFieldValue("Environment", make(map[string]string)); envVal != nil {
		if env, ok := envVal.(map[string]string); ok {
			envVars = env
		}
	}

	// Set defaults for missing fields
	resourceProfile = ""
	enableHPA = false
	enableProbes = false
	annotations = nil
	labels = nil
	deploymentStrategy = ""

	// Create a simplified args structure for the helper methods
	manifestArgs := &manifestTemplateArgs{
		Namespace:          namespace,
		Replicas:           replicas,
		ServiceType:        serviceType,
		ResourceProfile:    resourceProfile,
		EnableHPA:          enableHPA,
		EnableProbes:       enableProbes,
		GenerateHelm:       generateHelm,
		DeploymentStrategy: deploymentStrategy,
		EnvVars:            envVars,
	}

	// Determine application type
	context.ApplicationType = ti.determineApplicationType(repoInfo, port)
	context.DeploymentStrategy = ti.determineDeploymentStrategy(manifestArgs)
	context.ResourceProfile = resourceProfile

	// Select template type based on requirements
	if generateHelm {
		context.SelectedTemplate = "helm-chart"
		context.TemplateType = "helm"
		context.SelectionReasoning = append(context.SelectionReasoning,
			"Helm chart generation requested by user")
	} else if gitOpsReady {
		context.SelectedTemplate = "gitops-manifests"
		context.TemplateType = "gitops"
		context.SelectionReasoning = append(context.SelectionReasoning,
			"GitOps-ready manifests requested for better deployment practices")
	} else {
		context.SelectedTemplate = "manifest-basic"
		context.TemplateType = "basic"
		context.SelectionReasoning = append(context.SelectionReasoning,
			"Using basic manifests for straightforward deployment")
	}

	// Get available templates
	context.AvailableTemplates = ti.getAvailableManifestTemplates()

	// Add customization options
	context.CustomizationOptions = map[string]interface{}{
		"namespace":        namespace,
		"replicas":         replicas,
		"service_type":     serviceType,
		"port":             port,
		"resource_profile": resourceProfile,
		"enable_hpa":       enableHPA,
		"enable_probes":    enableProbes,
		"annotations":      annotations,
		"labels":           labels,
	}

	// Add best practices
	context.BestPractices = ti.generateManifestBestPractices(context.TemplateType, manifestArgs)

	// List files that will be generated
	context.GeneratedFiles = ti.listGeneratedManifestFiles(context.TemplateType, manifestArgs)

	return context, nil
}

// manifestTemplateArgs is a simplified structure for manifest template selection
type manifestTemplateArgs struct {
	Namespace          string
	Replicas           int
	ServiceType        string
	ResourceProfile    string
	EnableHPA          bool
	EnableProbes       bool
	GenerateHelm       bool
	DeploymentStrategy string
	EnvVars            map[string]string
}

// Helper methods

func (ti *TemplateIntegration) mapCommonTemplateNames(template string) string {
	// Map common language names to actual template names
	mappings := map[string]string{
		"python":     "dockerfile-python",
		"go":         "dockerfile-go",
		"golang":     "dockerfile-go",
		"javascript": "dockerfile-javascript",
		"js":         "dockerfile-javascript",
		"node":       "dockerfile-javascript",
		"nodejs":     "dockerfile-javascript",
		"typescript": "dockerfile-javascript",
		"ts":         "dockerfile-javascript",
		"java":       "dockerfile-java",
		"csharp":     "dockerfile-csharp",
		"c#":         "dockerfile-csharp",
		"dotnet":     "dockerfile-csharp",
		"ruby":       "dockerfile-ruby",
		"php":        "dockerfile-php",
		"rust":       "dockerfile-rust",
		"swift":      "dockerfile-swift",
	}

	if mapped, ok := mappings[strings.ToLower(template)]; ok {
		return mapped
	}

	// If it already starts with "dockerfile-", return as-is
	if strings.HasPrefix(template, "dockerfile-") {
		return template
	}

	// Otherwise, prepend "dockerfile-"
	return "dockerfile-" + template
}

func (ti *TemplateIntegration) generateSelectionReasoning(language, framework string, dependencies []string, selectedTemplate string) []string {
	reasoning := []string{
		fmt.Sprintf("Detected %s as the primary language", language),
	}

	if framework != "" {
		reasoning = append(reasoning, fmt.Sprintf("Detected %s framework", framework))
	}

	if len(dependencies) > 0 {
		reasoning = append(reasoning, fmt.Sprintf("Found %d dependencies", len(dependencies)))
	}

	reasoning = append(reasoning, fmt.Sprintf("Selected %s as the best match", selectedTemplate))

	return reasoning
}

func (ti *TemplateIntegration) getAvailableDockerfileTemplates(language, framework string, dependencies []string) []TemplateOptionInternal {
	// Get all available templates
	templates, err := ti.dockerTemplateEngine.ListAvailableTemplates()
	if err != nil {
		ti.logger.Error().Err(err).Msg("Failed to list dockerfile templates")
		return []TemplateOptionInternal{}
	}

	options := make([]TemplateOptionInternal, 0, len(templates))
	for _, tmpl := range templates {
		option := TemplateOptionInternal{
			Name:        tmpl.Name,
			Language:    tmpl.Language,
			Framework:   tmpl.Framework,
			Description: tmpl.Description,
			MatchScore:  ti.calculateTemplateMatchScore(tmpl.Name, language, framework, dependencies),
			Strengths:   ti.getTemplateStrengths(tmpl.Name),
			Limitations: ti.getTemplateLimitations(tmpl.Name),
			BestFor:     ti.getTemplateBestFor(tmpl.Name),
		}
		options = append(options, option)
	}

	return options
}

func (ti *TemplateIntegration) calculateTemplateMatchScore(templateName, language, framework string, dependencies []string) float64 {
	score := 0.0

	// Extract template language from name
	templateLang := strings.TrimPrefix(templateName, "dockerfile-")

	// Language match
	if strings.ToLower(language) == templateLang {
		score += 0.6
	} else if ti.areLanguagesRelated(language, templateLang) {
		score += 0.3
	}

	// Framework match
	if framework != "" && strings.Contains(templateName, strings.ToLower(framework)) {
		score += 0.3
	}

	// Dependency match
	depScore := 0.0
	for _, dep := range dependencies {
		if ti.isTemplateCompatibleWithDependency(templateName, dep) {
			depScore += 0.1
		}
	}
	score += minFloat64(depScore, 0.1) // Cap dependency score

	return minFloat64(score, 1.0)
}

func (ti *TemplateIntegration) areLanguagesRelated(lang1, lang2 string) bool {
	related := map[string][]string{
		"javascript": {"typescript", "node", "nodejs"},
		"typescript": {"javascript", "node", "nodejs"},
		"java":       {"gradle", "maven", "gradlew"},
	}

	lang1Lower := strings.ToLower(lang1)
	lang2Lower := strings.ToLower(lang2)

	if relatives, ok := related[lang1Lower]; ok {
		for _, rel := range relatives {
			if rel == lang2Lower {
				return true
			}
		}
	}

	return false
}

func (ti *TemplateIntegration) isTemplateCompatibleWithDependency(templateName, dependency string) bool {
	// Check if template is designed for specific dependencies
	compatMap := map[string][]string{
		"dockerfile-maven":    {"maven", "junit", "spring"},
		"dockerfile-gradle":   {"gradle", "spring", "junit"},
		"dockerfile-gomodule": {"go.mod", "gin", "echo", "fiber"},
	}

	if deps, ok := compatMap[templateName]; ok {
		depLower := strings.ToLower(dependency)
		for _, compat := range deps {
			if strings.Contains(depLower, compat) {
				return true
			}
		}
	}

	return false
}

func (ti *TemplateIntegration) getTemplateStrengths(templateName string) []string {
	strengths := map[string][]string{
		"dockerfile-python": {
			"Optimized for Python applications",
			"Includes pip caching for faster builds",
			"Multi-stage build for smaller images",
		},
		"dockerfile-javascript": {
			"Optimized for Node.js applications",
			"npm/yarn caching for faster builds",
			"Production-ready with NODE_ENV",
		},
		"dockerfile-go": {
			"Multi-stage build with scratch base",
			"Minimal final image size",
			"Static binary compilation",
		},
		"dockerfile-java": {
			"JVM optimization",
			"Memory configuration options",
			"JAR file handling",
		},
	}

	if s, ok := strengths[templateName]; ok {
		return s
	}

	return []string{"Standard containerization approach", "Based on Azure Draft best practices"}
}

func (ti *TemplateIntegration) getTemplateLimitations(templateName string) []string {
	limitations := map[string][]string{
		"dockerfile-python": {
			"May need adjustment for complex dependencies",
			"Default to pip, may need poetry/pipenv changes",
		},
		"dockerfile-javascript": {
			"Assumes npm, may need yarn/pnpm adjustments",
			"May need modifications for monorepos",
		},
		"dockerfile-go": {
			"Requires go.mod for dependency management",
			"CGO disabled by default",
		},
		"dockerfile-java": {
			"May need JVM tuning for production",
			"Default heap settings may not be optimal",
		},
	}

	if l, ok := limitations[templateName]; ok {
		return l
	}

	return []string{"May require customization for specific use cases"}
}

func (ti *TemplateIntegration) getTemplateBestFor(templateName string) []string {
	bestFor := map[string][]string{
		"dockerfile-python": {
			"Web applications (Django, Flask, FastAPI)",
			"Data science and ML workloads",
			"API services",
		},
		"dockerfile-javascript": {
			"Node.js web applications",
			"React/Vue/Angular frontend apps",
			"Express/NestJS APIs",
		},
		"dockerfile-go": {
			"Microservices",
			"CLI tools",
			"High-performance APIs",
		},
		"dockerfile-java": {
			"Spring Boot applications",
			"Enterprise services",
			"Long-running applications",
		},
	}

	if b, ok := bestFor[templateName]; ok {
		return b
	}

	return []string{"General containerization needs"}
}

func (ti *TemplateIntegration) generateAlternativeDockerfileOptions(selectedTemplate, language, framework string, dependencies []string) []AlternativeTemplateOption {
	alternatives := []AlternativeTemplateOption{}

	// Suggest multi-stage optimization
	if !strings.Contains(selectedTemplate, "multi") {
		alternatives = append(alternatives, AlternativeTemplateOption{
			Template:   "custom-multistage",
			Reason:     "Optimize image size with multi-stage build",
			TradeOffs:  []string{"Smaller image size", "More complex Dockerfile", "Longer initial build"},
			UseCases:   []string{"Production deployments", "Bandwidth-constrained environments"},
			Complexity: "moderate",
			MatchScore: 0.8,
		})
	}

	// Suggest distroless for supported languages
	if language == "Go" || language == "Java" || language == "Python" {
		alternatives = append(alternatives, AlternativeTemplateOption{
			Template:   "custom-distroless",
			Reason:     "Maximum security with distroless base image",
			TradeOffs:  []string{"Enhanced security", "Minimal attack surface", "No shell access"},
			UseCases:   []string{"High-security environments", "Production services"},
			Complexity: "complex",
			MatchScore: 0.7,
		})
	}

	// Suggest Alpine variants
	if !strings.Contains(selectedTemplate, "alpine") {
		alternatives = append(alternatives, AlternativeTemplateOption{
			Template:   selectedTemplate + "-alpine",
			Reason:     "Smaller image size with Alpine Linux",
			TradeOffs:  []string{"Smaller size", "Potential compatibility issues", "Different package manager"},
			UseCases:   []string{"Size-constrained deployments", "Edge computing"},
			Complexity: "moderate",
			MatchScore: 0.6,
		})
	}

	return alternatives
}

func (ti *TemplateIntegration) generateDockerfileTradeOffs(template, language, framework string) []string {
	tradeOffs := []string{}

	// General trade-offs
	tradeOffs = append(tradeOffs, "Template provides standardized approach vs custom optimization")

	// Language-specific trade-offs
	switch strings.ToLower(language) {
	case "python":
		tradeOffs = append(tradeOffs,
			"pip installation speed vs using system packages",
			"Virtual environment isolation vs global installation")
	case "javascript", "typescript":
		tradeOffs = append(tradeOffs,
			"npm ci for reproducibility vs npm install flexibility",
			"Node modules caching vs fresh installation")
	case "go":
		tradeOffs = append(tradeOffs,
			"Static binary simplicity vs CGO functionality",
			"Scratch base minimalism vs debugging capabilities")
	case "java":
		tradeOffs = append(tradeOffs,
			"JRE vs JDK in production",
			"Memory optimization vs startup time")
	}

	return tradeOffs
}

func (ti *TemplateIntegration) generateDockerfileCustomizationOptions(template, language, framework string, dependencies []string) map[string]interface{} {
	options := map[string]interface{}{
		"base_image_variant": ti.getBaseImageVariants(language),
		"optimization_level": []string{"size", "speed", "security"},
		"caching_strategy":   []string{"aggressive", "moderate", "minimal"},
		"user_configuration": map[string]interface{}{
			"run_as_root":     false,
			"create_app_user": true,
			"user_id":         1000,
		},
	}

	// Language-specific options
	switch strings.ToLower(language) {
	case "python":
		options["python_options"] = map[string]interface{}{
			"use_virtual_env": true,
			"pip_no_cache":    false,
			"compile_pyc":     true,
		}
	case "javascript", "typescript":
		options["node_options"] = map[string]interface{}{
			"npm_ci":          true,
			"production_only": true,
			"prune_dev_deps":  true,
		}
	case "go":
		options["go_options"] = map[string]interface{}{
			"cgo_enabled":  false,
			"vendor_mode":  false,
			"mod_download": true,
		}
	case "java":
		options["java_options"] = map[string]interface{}{
			"jvm_version": "17",
			"heap_size":   "512m",
			"use_jlink":   false,
		}
	}

	return options
}

func (ti *TemplateIntegration) getBaseImageVariants(language string) []string {
	variants := map[string][]string{
		"python":     {"python:3.11-slim", "python:3.11-alpine", "python:3.11-bullseye"},
		"javascript": {"node:18-alpine", "node:18-slim", "node:18-bullseye"},
		"typescript": {"node:18-alpine", "node:18-slim", "node:18-bullseye"},
		"go":         {"golang:1.21-alpine", "golang:1.21-bullseye", "scratch"},
		"java":       {"openjdk:17-slim", "openjdk:17-alpine", "amazoncorretto:17"},
		"csharp":     {"mcr.microsoft.com/dotnet/sdk:7.0", "mcr.microsoft.com/dotnet/aspnet:7.0"},
		"ruby":       {"ruby:3.2-slim", "ruby:3.2-alpine"},
		"php":        {"php:8.2-fpm-alpine", "php:8.2-apache"},
	}

	if v, ok := variants[strings.ToLower(language)]; ok {
		return v
	}

	return []string{"alpine:latest", "ubuntu:22.04", "debian:bullseye-slim"}
}

func (ti *TemplateIntegration) determineApplicationType(repoInfo map[string]interface{}, port int) string {
	// Check for web application indicators
	if port > 0 && port != 22 && port != 3306 && port != 5432 {
		return "web"
	}

	// Check framework
	if framework, ok := repoInfo["framework"].(string); ok {
		switch strings.ToLower(framework) {
		case "express", "django", "flask", "spring", "rails", "laravel":
			return "web"
		case "cli", "console":
			return "cli"
		}
	}

	// Check for API indicators
	if deps, ok := repoInfo["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			depStr := fmt.Sprintf("%v", dep)
			if strings.Contains(depStr, "fastapi") || strings.Contains(depStr, "graphql") {
				return "api"
			}
		}
	}

	// Default to service
	return "service"
}

func (ti *TemplateIntegration) determineDeploymentStrategy(args *manifestTemplateArgs) string {
	if args.DeploymentStrategy != "" {
		return args.DeploymentStrategy
	}

	// Determine based on configuration
	if args.EnableHPA {
		return "scalable"
	}

	if args.Replicas > 1 {
		return "replicated"
	}

	return "simple"
}

func (ti *TemplateIntegration) getAvailableManifestTemplates() []ManifestTemplateOption {
	return []ManifestTemplateOption{
		{
			Name:         "manifest-basic",
			Type:         "basic",
			Description:  "Basic Kubernetes manifests for simple deployments",
			Components:   []string{"deployment", "service", "configmap"},
			Features:     []string{"basic networking", "environment variables"},
			Complexity:   "simple",
			Requirements: []string{"Kubernetes 1.19+"},
		},
		{
			Name:         "manifest-advanced",
			Type:         "advanced",
			Description:  "Advanced manifests with production features",
			Components:   []string{"deployment", "service", "configmap", "secret", "ingress", "hpa"},
			Features:     []string{"autoscaling", "ingress", "probes", "resource limits"},
			Complexity:   "moderate",
			Requirements: []string{"Kubernetes 1.21+", "metrics-server for HPA"},
		},
		{
			Name:         "gitops-manifests",
			Type:         "gitops",
			Description:  "GitOps-ready manifests with Kustomize support",
			Components:   []string{"base/", "overlays/", "kustomization.yaml"},
			Features:     []string{"multi-environment", "kustomize patches", "sealed secrets"},
			Complexity:   "complex",
			Requirements: []string{"Kubernetes 1.21+", "Kustomize", "GitOps operator"},
		},
		{
			Name:         "helm-chart",
			Type:         "helm",
			Description:  "Helm chart for flexible deployments",
			Components:   []string{"Chart.yaml", "values.yaml", "templates/"},
			Features:     []string{"parameterization", "dependencies", "hooks", "tests"},
			Complexity:   "complex",
			Requirements: []string{"Helm 3.0+", "Kubernetes 1.19+"},
		},
	}
}

func (ti *TemplateIntegration) generateManifestBestPractices(templateType string, args *manifestTemplateArgs) []string {
	practices := []string{
		"Use resource requests and limits for predictable performance",
		"Implement health checks (liveness and readiness probes)",
		"Use ConfigMaps for configuration and Secrets for sensitive data",
		"Label resources consistently for organization and selection",
		"Set security context to run as non-root user",
	}

	// Template-specific practices
	switch templateType {
	case "basic":
		practices = append(practices,
			"Consider upgrading to advanced templates for production",
			"Add horizontal pod autoscaling for variable load")
	case "advanced":
		practices = append(practices,
			"Configure HPA thresholds based on load testing",
			"Use PodDisruptionBudget for high availability")
	case "gitops":
		practices = append(practices,
			"Structure overlays by environment (dev, staging, prod)",
			"Use Kustomize patches for environment-specific changes",
			"Implement sealed secrets for secure GitOps workflows")
	case "helm":
		practices = append(practices,
			"Keep values.yaml well-documented",
			"Use named templates for repeated configurations",
			"Implement chart tests for validation")
	}

	// Conditional practices
	if args.ServiceType == "LoadBalancer" {
		practices = append(practices, "Consider using Ingress instead of LoadBalancer for cost efficiency")
	}

	if args.Replicas > 3 {
		practices = append(practices, "Use PodAntiAffinity to spread pods across nodes")
	}

	return practices
}

func (ti *TemplateIntegration) listGeneratedManifestFiles(templateType string, args *manifestTemplateArgs) []string {
	files := []string{}

	switch templateType {
	case "basic":
		files = []string{
			"deployment.yaml",
			"service.yaml",
		}
		if len(args.EnvVars) > 0 {
			files = append(files, "configmap.yaml")
		}
	case "advanced":
		files = []string{
			"deployment.yaml",
			"service.yaml",
			"configmap.yaml",
			"secret.yaml",
		}
		if args.ServiceType == "ClusterIP" || args.ServiceType == "NodePort" {
			files = append(files, "ingress.yaml")
		}
		if args.EnableHPA {
			files = append(files, "hpa.yaml")
		}
	case "gitops":
		files = []string{
			"base/deployment.yaml",
			"base/service.yaml",
			"base/configmap.yaml",
			"base/kustomization.yaml",
			"overlays/dev/kustomization.yaml",
			"overlays/dev/patch-deployment.yaml",
			"overlays/prod/kustomization.yaml",
			"overlays/prod/patch-deployment.yaml",
		}
	case "helm":
		files = []string{
			"Chart.yaml",
			"values.yaml",
			"values-dev.yaml",
			"values-prod.yaml",
			"templates/deployment.yaml",
			"templates/service.yaml",
			"templates/configmap.yaml",
			"templates/secret.yaml",
			"templates/ingress.yaml",
			"templates/hpa.yaml",
			"templates/_helpers.tpl",
			"templates/NOTES.txt",
		}
	}

	return files
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
