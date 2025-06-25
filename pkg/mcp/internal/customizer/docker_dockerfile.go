package customizer

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// DockerfileCustomizer handles Dockerfile customization
type DockerfileCustomizer struct {
	logger zerolog.Logger
}

// NewDockerfileCustomizer creates a new Dockerfile customizer
func NewDockerfileCustomizer(logger zerolog.Logger) *DockerfileCustomizer {
	return &DockerfileCustomizer{
		logger: logger.With().Str("customizer", "dockerfile").Logger(),
	}
}

// DockerfileCustomizationOptions contains options for customizing a Dockerfile
type DockerfileCustomizationOptions struct {
	BaseImage          string
	IncludeHealthCheck bool
	Optimization       customizer.OptimizationStrategy
	BuildArgs          map[string]string
	Platform           string
	TemplateContext    *customizer.TemplateContext
}

// CustomizeDockerfile applies customizations to a Dockerfile
func (c *DockerfileCustomizer) CustomizeDockerfile(content string, opts DockerfileCustomizationOptions) string {
	// Override base image if specified
	if opts.BaseImage != "" {
		content = c.replaceBaseImage(content, opts.BaseImage)
	}

	// Add health check if requested
	if opts.IncludeHealthCheck && !strings.Contains(content, "HEALTHCHECK") {
		language := ""
		framework := ""
		if opts.TemplateContext != nil {
			language = opts.TemplateContext.Language
			framework = opts.TemplateContext.Framework
		}
		healthCheck := c.generateHealthCheck(language, framework)
		content = strings.TrimRight(content, "\n") + "\n\n" + healthCheck + "\n"
	}

	// Apply optimization hints
	if opts.Optimization != "" {
		optimizer := NewOptimizer(c.logger)
		content = optimizer.ApplyOptimization(content, opts.Optimization, opts.TemplateContext)
	}

	// Add build args
	if len(opts.BuildArgs) > 0 {
		content = c.addBuildArgs(content, opts.BuildArgs)
	}

	// Add platform if specified
	if opts.Platform != "" {
		content = fmt.Sprintf("# syntax=docker/dockerfile:1\n# platform=%s\n%s", opts.Platform, content)
	}

	return content
}

// replaceBaseImage replaces the base image in a Dockerfile
func (c *DockerfileCustomizer) replaceBaseImage(content, newBaseImage string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM ") {
			// Replace the first FROM instruction
			lines[i] = fmt.Sprintf("FROM %s", newBaseImage)
			c.logger.Debug().
				Str("base_image", newBaseImage).
				Msg("Replaced base image")
			break
		}
	}
	return strings.Join(lines, "\n")
}

// generateHealthCheck generates appropriate health check based on language/framework
func (c *DockerfileCustomizer) generateHealthCheck(language, framework string) string {
	hc := NewHealthCheckGenerator(c.logger)
	return hc.Generate(language, framework)
}

// addBuildArgs adds build arguments to a Dockerfile
func (c *DockerfileCustomizer) addBuildArgs(content string, buildArgs map[string]string) string {
	buildArgsSection := "\n# Build arguments\n"
	for key, value := range buildArgs {
		buildArgsSection += fmt.Sprintf("ARG %s=%s\n", key, value)
	}

	// Insert after FROM instruction
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "FROM ") {
			lines[i] = line + buildArgsSection
			c.logger.Debug().
				Int("arg_count", len(buildArgs)).
				Msg("Added build arguments")
			break
		}
	}
	return strings.Join(lines, "\n")
}
