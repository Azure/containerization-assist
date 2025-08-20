package steps

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

func init() {
	Register(NewResolveBaseImagesStep())
}

// ResolveBaseImagesStep implements base image resolution using repository analysis
type ResolveBaseImagesStep struct{}

// NewResolveBaseImagesStep creates a new resolve base images step
func NewResolveBaseImagesStep() workflow.Step {
	return &ResolveBaseImagesStep{}
}

// Name returns the step name
func (s *ResolveBaseImagesStep) Name() string {
	return "resolve_base_images"
}

// Execute resolves recommended base images based on repository analysis
func (s *ResolveBaseImagesStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
	state.Logger.Info("Step: Resolving recommended base images")

	if state.AnalyzeResult == nil {
		return nil, fmt.Errorf("analyze result is required for base image resolution")
	}

	builder, runtime, err := s.resolveImages(state.AnalyzeResult)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base images: %w", err)
	}

	return &workflow.StepResult{
		Success: true,
		Data: map[string]any{
			"builder": builder,
			"runtime": runtime,
		},
	}, nil
}

// ImageConfig represents configuration for language base images
type ImageConfig struct {
	Name           string   // Human-readable name
	Language       string   // Target language (java, python, node, etc.)
	FrameworkKeys  []string // Framework keywords to match
	DependencyKeys []string // Dependency keywords to match
	BuilderImage   string   // Builder image template (uses {version})
	RuntimeImage   string   // Runtime image template (uses {version})
	Priority       int      // Lower number = higher priority
	DefaultVersion string   // Default version if not detected
}

// resolveImages resolves base images based on detected language
func (s *ResolveBaseImagesStep) resolveImages(result *workflow.AnalyzeResult) (string, string, error) {
	language := strings.ToLower(result.Language)
	
	switch language {
	case "java":
		return s.resolveLanguageImages(result, s.getJavaConfigs())
	default:
		return "", "", fmt.Errorf("unsupported language: %s", language)
	}
}

// resolveLanguageImages resolves base images using provided configurations
func (s *ResolveBaseImagesStep) resolveLanguageImages(result *workflow.AnalyzeResult, configs []ImageConfig) (string, string, error) {
	language := strings.ToLower(result.Language)
	framework := strings.ToLower(result.Framework)
	
	// Extract dependencies from metadata
	var dependencies []string
	if deps, ok := result.Metadata["dependencies"].([]string); ok {
		dependencies = deps
	}
	
	// Sort configs by priority (lower number = higher priority)
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Priority < configs[j].Priority
	})
	
	// Find the best matching configuration
	for _, config := range configs {
		if s.matchesConfig(framework, dependencies, config) {
			version := s.getLanguageVersion(result, config.DefaultVersion)
			builder := strings.ReplaceAll(config.BuilderImage, "{version}", version)
			runtime := strings.ReplaceAll(config.RuntimeImage, "{version}", version)
			return builder, runtime, nil
		}
	}
	
	// No specific config matched, use generic default for the language
	return s.getDefaultImages(language, result)
}

// getJavaConfigs returns Java-specific image configurations
func (s *ResolveBaseImagesStep) getJavaConfigs() []ImageConfig {
	return []ImageConfig{
		{
			Name:           "Servlet/Tomcat",
			Language:       "java",
			FrameworkKeys:  []string{"servlet"},
			DependencyKeys: []string{"servlet", "javax.servlet", "jakarta.servlet"},
			BuilderImage:   "openjdk:{version}-jdk-slim",
			RuntimeImage:   "tomcat:10.1-jre{version}",
			Priority:       1,
			DefaultVersion: "21",
		},
		{
			Name:           "WildFly",
			Language:       "java",
			FrameworkKeys:  []string{"wildfly"},
			DependencyKeys: []string{"wildfly"},
			BuilderImage:   "openjdk:{version}-jdk-slim",
			RuntimeImage:   "quay.io/wildfly/wildfly:30.0.0.Final-jdk{version}",
			Priority:       1,
			DefaultVersion: "21",
		},
		{
			Name:           "Maven",
			Language:       "java",
			FrameworkKeys:  []string{"maven"},
			DependencyKeys: []string{},
			BuilderImage:   "maven:3.9-openjdk-{version}-slim",
			RuntimeImage:   "openjdk:{version}-jre-slim",
			Priority:       2,
			DefaultVersion: "21",
		},
		{
			Name:           "Gradle",
			Language:       "java",
			FrameworkKeys:  []string{"gradle"},
			DependencyKeys: []string{},
			BuilderImage:   "gradle:8.8-jdk{version}-alpine",
			RuntimeImage:   "openjdk:{version}-jre-alpine",
			Priority:       2,
			DefaultVersion: "21",
		},
		{
			Name:           "Spring Boot",
			Language:       "java",
			FrameworkKeys:  []string{"spring"},
			DependencyKeys: []string{"spring-boot"},
			BuilderImage:   "openjdk:{version}-jdk-slim",
			RuntimeImage:   "openjdk:{version}-jre-slim",
			Priority:       3,
			DefaultVersion: "21",
		},
	}
}

// getDefaultImages returns default images for a language when no specific config matches
func (s *ResolveBaseImagesStep) getDefaultImages(language string, result *workflow.AnalyzeResult) (string, string, error) {
	switch language {
	case "java":
		version := s.getLanguageVersion(result, "21")
		return fmt.Sprintf("openjdk:%s-jdk-slim", version), fmt.Sprintf("openjdk:%s-jre-slim", version), nil
	default:
		return "alpine:latest", "alpine:latest", nil
	}
}

// matchesConfig checks if the framework and dependencies match the given configuration
func (s *ResolveBaseImagesStep) matchesConfig(framework string, dependencies []string, config ImageConfig) bool {
	// Validate config has matching criteria
	if len(config.FrameworkKeys) == 0 && len(config.DependencyKeys) == 0 {
		return false
	}
	
	// Check framework matches (case-insensitive)
	for _, key := range config.FrameworkKeys {
		if strings.Contains(framework, strings.ToLower(key)) {
			return true
		}
	}
	
	// Check dependency matches (case-insensitive)
	for _, key := range config.DependencyKeys {
		lowerKey := strings.ToLower(key)
		for _, dep := range dependencies {
			if strings.Contains(strings.ToLower(dep), lowerKey) {
				return true
			}
		}
	}
	
	return false
}



// getLanguageVersion gets the language version from repository analysis or uses default
func (s *ResolveBaseImagesStep) getLanguageVersion(result *workflow.AnalyzeResult, defaultVersion string) string {
	// The repository analysis results are stored in the Metadata field
	if version, ok := result.Metadata["language_version"].(string); ok && version != "" {
		return version
	}
	
	return defaultVersion
}