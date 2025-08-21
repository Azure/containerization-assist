package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

const defaultJavaVersion = "21"

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

// resolveImages resolves base images based on detected language
func (s *ResolveBaseImagesStep) resolveImages(result *workflow.AnalyzeResult) (string, string, error) {
	language := strings.ToLower(result.Language)

	switch language {
	case "java":
		return s.resolveJavaImages(result)
	default:
		return "", "", fmt.Errorf("unsupported language: %s", language)
	}
}

// resolveJavaImages resolves Java-specific images based on framework and dependencies
func (s *ResolveBaseImagesStep) resolveJavaImages(result *workflow.AnalyzeResult) (string, string, error) {
	framework := strings.ToLower(result.Framework)
	version := s.getLanguageVersion(result)
	buildTool := s.detectBuildTool(result)
	dependencies := s.extractDependencies(result)
	buildImage := s.getBuildImage(buildTool, version)

	// Pre-compute framework/dependency checks to avoid redundant operations
	hasWildfly := s.contains(framework, []string{"wildfly", "jboss"}) || s.containsAny(dependencies, []string{"org.wildfly", "org.jboss", "wildfly", "jboss"})
	hasServlet := s.contains(framework, []string{"tomcat", "servlet"}) || s.containsAny(dependencies, []string{"javax.servlet", "jakarta.servlet", "tomcat", "org.apache.tomcat"})
	hasJetty := s.contains(framework, []string{"jetty"}) || s.containsAny(dependencies, []string{"org.eclipse.jetty", "jetty"})
	hasSpring := s.contains(framework, []string{"spring", "embedded-tomcat", "embedded-jetty", "embedded-undertow"}) || s.containsAny(dependencies, []string{"spring-boot", "spring-boot-starter"})
	hasQuarkus := s.contains(framework, []string{"quarkus"}) || s.containsAny(dependencies, []string{"io.quarkus", "quarkus"})
	hasMicronaut := s.contains(framework, []string{"micronaut"}) || s.containsAny(dependencies, []string{"io.micronaut", "micronaut"})
	hasWar := s.contains(framework, []string{"maven-war", "gradle-war", "servlet"})

	// Application Server Detection (Highest Priority)
	if hasWildfly {
		return buildImage, fmt.Sprintf("quay.io/wildfly/wildfly:%s-jdk%s", defaultWildflyVersion, version), nil
	}
	if hasServlet || hasWar {
		return buildImage, fmt.Sprintf("tomcat:10.1-jre%s", version), nil
	}
	if hasJetty {
		return buildImage, fmt.Sprintf("jetty:11-jre%s-alpine", version), nil
	}

	// Framework Detection (Medium Priority) - grouped by common runtime
	if hasSpring || hasMicronaut {
		return buildImage, fmt.Sprintf("openjdk:%s-jre-slim", version), nil
	}
	if hasQuarkus {
		return buildImage, "registry.access.redhat.com/ubi8/openjdk-" + version + "-runtime:latest", nil
	}

	// Default Java runtime
	return buildImage, fmt.Sprintf("openjdk:%s-jre-slim", version), nil
}

// Helper functions
func (s *ResolveBaseImagesStep) getLanguageVersion(result *workflow.AnalyzeResult) string {
	if result.LanguageVersion != "" {
		return result.LanguageVersion
	}
	return defaultJavaVersion
}

func (s *ResolveBaseImagesStep) detectBuildTool(result *workflow.AnalyzeResult) string {
	framework := strings.ToLower(result.Framework)

	if strings.Contains(framework, "gradle") {
		return "gradle"
	}
	if strings.Contains(framework, "maven") {
		return "maven"
	}

	// Check dependency manager
	if result.Dependencies != nil {
		for _, dep := range result.Dependencies {
			switch strings.ToLower(dep.Manager) {
			case "gradle":
				return "gradle"
			case "maven":
				return "maven"
			}
		}
	}

	return "maven" // Default
}

func (s *ResolveBaseImagesStep) getBuildImage(buildTool, version string) string {
	switch buildTool {
	case "gradle":
		return fmt.Sprintf("gradle:8.10-jdk%s", version)
	case "maven":
		return fmt.Sprintf("maven:%s-eclipse-temurin-%s", defaultMavenVersion, version)
	default:
		return fmt.Sprintf("openjdk:%s-jdk-slim", version)
	}
}

func (s *ResolveBaseImagesStep) extractDependencies(result *workflow.AnalyzeResult) []string {
	var dependencies []string
	if result.Dependencies != nil {
		for _, dep := range result.Dependencies {
			dependencies = append(dependencies, strings.ToLower(dep.Name))
		}
	}
	return dependencies
}

func (s *ResolveBaseImagesStep) contains(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func (s *ResolveBaseImagesStep) containsAny(slice []string, keywords []string) bool {
	for _, item := range slice {
		for _, keyword := range keywords {
			if strings.Contains(item, keyword) {
				return true
			}
		}
	}
	return false
}
