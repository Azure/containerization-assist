package retry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

// DockerFixProvider provides fixes for Docker-related issues
type DockerFixProvider struct {
	name string
}

// NewDockerFixProvider creates a new Docker fix provider
func NewDockerFixProvider() *DockerFixProvider {
	return &DockerFixProvider{name: "docker"}
}

func (dfp *DockerFixProvider) Name() string {
	return dfp.name
}

func (dfp *DockerFixProvider) GetFixStrategies(_ context.Context, err error, context map[string]interface{}) ([]FixStrategy, error) {
	strategies := make([]FixStrategy, 0)
	errMsg := strings.ToLower(err.Error())

	// Dockerfile syntax fixes
	if strings.Contains(errMsg, "dockerfile") && strings.Contains(errMsg, "syntax") {
		strategies = append(strategies, FixStrategy{
			Type:        "dockerfile",
			Name:        "Fix Dockerfile Syntax",
			Description: "Automatically fix common Dockerfile syntax errors",
			Priority:    1,
			Automated:   true,
			Parameters: map[string]interface{}{
				"dockerfile_path": context["dockerfile_path"],
				"error_line":      extractLineNumber(errMsg),
			},
		})
	}

	// Base image not found
	if strings.Contains(errMsg, "image not found") || strings.Contains(errMsg, "pull access denied") {
		strategies = append(strategies, FixStrategy{
			Type:        "docker",
			Name:        "Fix Base Image",
			Description: "Update base image to a valid alternative",
			Priority:    2,
			Automated:   true,
			Parameters: map[string]interface{}{
				"suggested_images": []string{"ubuntu:20.04", "alpine:latest", "node:16-alpine"},
			},
		})
	}

	// Port already in use
	if strings.Contains(errMsg, "port") && strings.Contains(errMsg, "already in use") {
		strategies = append(strategies, FixStrategy{
			Type:        "docker",
			Name:        "Change Port",
			Description: "Use an alternative port for the container",
			Priority:    3,
			Automated:   true,
			Parameters: map[string]interface{}{
				"current_port":      extractPort(errMsg),
				"alternative_ports": []int{8080, 8081, 8082, 3000, 3001},
			},
		})
	}

	return strategies, nil
}

func (dfp *DockerFixProvider) ApplyFix(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error {
	switch strategy.Type {
	case "dockerfile":
		return dfp.fixDockerfileSyntax(ctx, strategy, context)
	case "docker":
		if strategy.Name == "Fix Base Image" {
			return dfp.fixBaseImage(ctx, strategy, context)
		} else if strategy.Name == "Change Port" {
			return dfp.fixPortConflict(ctx, strategy, context)
		}
	}
	return errors.NewError().
		Code(errors.CodeNotImplemented).
		Type(errors.ErrTypeValidation).
		Severity(errors.SeverityMedium).
		Message("unsupported fix strategy").
		Context("module", "retry/fix-provider").
		Context("component", "DockerFixProvider").
		Context("strategy_name", strategy.Name).
		Suggestion("Use a supported fix strategy like 'Fix Syntax', 'Change Base Image', or 'Change Port'").
		WithLocation().
		Build()
}

func (dfp *DockerFixProvider) fixDockerfileSyntax(_ context.Context, strategy FixStrategy, _ map[string]interface{}) error {
	dockerfilePath, ok := strategy.Parameters["dockerfile_path"].(string)
	if !ok || dockerfilePath == "" {
		return errors.NewError().
			Code(errors.CodeMissingParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("dockerfile path not provided").
			Context("module", "retry/fix-provider").
			Context("component", "DockerFixProvider").
			Context("method", "fixDockerfileSyntax").
			Context("strategy_name", strategy.Name).
			Suggestion("Provide a valid dockerfile_path parameter in the strategy").
			WithLocation().
			Build()
	}

	// Read the Dockerfile
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to read Dockerfile")
	}

	// Apply common fixes
	fixed := string(content)
	fixed = strings.ReplaceAll(fixed, "COPY . .", "COPY . /app")
	fixed = strings.ReplaceAll(fixed, "RUN apt-get update", "RUN apt-get update && apt-get install -y")
	fixed = regexp.MustCompile(`EXPOSE\s+(\d+)\s+(\d+)`).ReplaceAllString(fixed, "EXPOSE $1\nEXPOSE $2")

	// Write the fixed Dockerfile
	if err := os.WriteFile(dockerfilePath, []byte(fixed), 0600); err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to write fixed Dockerfile")
	}

	return nil
}

func (dfp *DockerFixProvider) fixBaseImage(_ context.Context, strategy FixStrategy, context map[string]interface{}) error {
	dockerfilePath, ok := context["dockerfile_path"].(string)
	if !ok || dockerfilePath == "" {
		return errors.Validation("retry/fix-provider", "dockerfile path not provided")
	}

	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to read Dockerfile")
	}

	// Replace with a suggested image
	suggestedImages := strategy.Parameters["suggested_images"].([]string)
	if len(suggestedImages) == 0 {
		return errors.Internal("retry/fix-provider", "no suggested images provided")
	}

	fixed := regexp.MustCompile(`FROM\s+\S+`).ReplaceAllString(string(content), "FROM "+suggestedImages[0])

	if err := os.WriteFile(dockerfilePath, []byte(fixed), 0600); err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to write fixed Dockerfile")
	}

	return nil
}

func (dfp *DockerFixProvider) fixPortConflict(_ context.Context, strategy FixStrategy, context map[string]interface{}) error {
	// This would update docker-compose.yml or runtime configuration
	// For now, just record the suggested port change
	alternativePorts := strategy.Parameters["alternative_ports"].([]int)
	if len(alternativePorts) > 0 {
		context["suggested_port"] = alternativePorts[0]
	}
	return nil
}

// ConfigFixProvider provides fixes for configuration issues
type ConfigFixProvider struct {
	name string
}

func NewConfigFixProvider() *ConfigFixProvider {
	return &ConfigFixProvider{name: "config"}
}

func (cfp *ConfigFixProvider) Name() string {
	return cfp.name
}

func (cfp *ConfigFixProvider) GetFixStrategies(_ context.Context, err error, context map[string]interface{}) ([]FixStrategy, error) {
	strategies := make([]FixStrategy, 0)
	errMsg := strings.ToLower(err.Error())

	// Missing configuration file
	if strings.Contains(errMsg, "not found") && (strings.Contains(errMsg, "config") || strings.Contains(errMsg, ".json") || strings.Contains(errMsg, ".yaml")) {
		strategies = append(strategies, FixStrategy{
			Type:        "config",
			Name:        "Create Default Config",
			Description: "Create a default configuration file",
			Priority:    1,
			Automated:   true,
			Parameters: map[string]interface{}{
				"config_path": extractFilePath(errMsg),
				"config_type": extractConfigType(errMsg),
			},
		})
	}

	// Invalid configuration format
	if strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "invalid format") {
		strategies = append(strategies, FixStrategy{
			Type:        "config",
			Name:        "Fix Config Format",
			Description: "Repair configuration file format",
			Priority:    2,
			Automated:   true,
			Parameters: map[string]interface{}{
				"config_path": context["config_path"],
			},
		})
	}

	return strategies, nil
}

func (cfp *ConfigFixProvider) ApplyFix(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error {
	switch strategy.Name {
	case "Create Default Config":
		return cfp.createDefaultConfig(ctx, strategy, context)
	case "Fix Config Format":
		return cfp.fixConfigFormat(ctx, strategy, context)
	}
	return errors.Internal("retry/fix-provider", "unsupported config fix strategy")
}

func (cfp *ConfigFixProvider) createDefaultConfig(_ context.Context, strategy FixStrategy, _ map[string]interface{}) error {
	configPath, ok := strategy.Parameters["config_path"].(string)
	if !ok || configPath == "" {
		return errors.Validation("retry/fix-provider", "config path not provided")
	}

	configType, _ := strategy.Parameters["config_type"].(string)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to create config directory")
	}

	// Create default configuration based on type
	var defaultContent string
	switch configType {
	case "json":
		defaultContent = `{
  "version": "1.0",
  "settings": {
    "enabled": true,
    "timeout": 30
  }
}`
	case "yaml":
		defaultContent = `version: "1.0"
settings:
  enabled: true
  timeout: 30
`
	default:
		defaultContent = "# Default configuration\nenabled=true\ntimeout=30\n"
	}

	if err := os.WriteFile(configPath, []byte(defaultContent), 0600); err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to write default config")
	}

	return nil
}

func (cfp *ConfigFixProvider) fixConfigFormat(_ context.Context, strategy FixStrategy, _ map[string]interface{}) error {
	configPath, ok := strategy.Parameters["config_path"].(string)
	if !ok || configPath == "" {
		return errors.Validation("retry/fix-provider", "config path not provided")
	}

	// Read and attempt to fix common JSON/YAML issues
	content, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to read config file")
	}

	fixed := string(content)

	// Fix common JSON issues
	if strings.HasSuffix(configPath, ".json") {
		fixed = strings.ReplaceAll(fixed, ",}", "}")
		fixed = strings.ReplaceAll(fixed, ",]", "]")
		// Remove trailing commas
		fixed = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(fixed, "$1")
	}

	// Fix common YAML issues
	if strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml") {
		// Fix indentation issues (basic)
		lines := strings.Split(fixed, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "\t") {
				lines[i] = "  " + strings.TrimPrefix(line, "\t")
			}
		}
		fixed = strings.Join(lines, "\n")
	}

	if err := os.WriteFile(configPath, []byte(fixed), 0600); err != nil {
		return errors.Wrap(err, "retry/fix-provider", "failed to write fixed config")
	}

	return nil
}

// DependencyFixProvider provides fixes for dependency issues
type DependencyFixProvider struct {
	name string
}

func NewDependencyFixProvider() *DependencyFixProvider {
	return &DependencyFixProvider{name: "dependency"}
}

func (dep *DependencyFixProvider) Name() string {
	return dep.name
}

func (dep *DependencyFixProvider) GetFixStrategies(_ context.Context, err error, _ map[string]interface{}) ([]FixStrategy, error) {
	strategies := make([]FixStrategy, 0)
	errMsg := strings.ToLower(err.Error())

	// Command not found
	if strings.Contains(errMsg, "command not found") || strings.Contains(errMsg, "not found") {
		command := extractCommand(errMsg)
		strategies = append(strategies, FixStrategy{
			Type:        "dependency",
			Name:        "Install Missing Command",
			Description: fmt.Sprintf("Install missing command: %s", command),
			Priority:    1,
			Automated:   true,
			Parameters: map[string]interface{}{
				"command":             command,
				"package_suggestions": getSuggestedPackages(command),
			},
		})
	}

	return strategies, nil
}

func (dep *DependencyFixProvider) ApplyFix(ctx context.Context, strategy FixStrategy, context map[string]interface{}) error {
	if strategy.Name == "Install Missing Command" {
		return dep.installMissingCommand(ctx, strategy, context)
	}
	return errors.Internal("retry/fix-provider", "unsupported dependency fix strategy")
}

func (dep *DependencyFixProvider) installMissingCommand(_ context.Context, strategy FixStrategy, context map[string]interface{}) error {
	command, ok := strategy.Parameters["command"].(string)
	if !ok || command == "" {
		return errors.Validation("retry/fix-provider", "command not specified")
	}

	suggestions, _ := strategy.Parameters["package_suggestions"].([]string)
	if len(suggestions) == 0 {
		return errors.Internal("retry/fix-provider", "no package suggestions available")
	}

	// Record the suggestion for manual installation
	// In a real implementation, this might trigger package installation
	context["install_suggestion"] = suggestions[0]
	context["install_command"] = fmt.Sprintf("apt-get install -y %s", suggestions[0])

	return nil
}

// Helper functions for extracting information from error messages
func extractLineNumber(errMsg string) int {
	re := regexp.MustCompile(`line\s+(\d+)`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		if num := parseInt(matches[1]); num > 0 {
			return num
		}
	}
	return 0
}

func extractPort(errMsg string) int {
	re := regexp.MustCompile(`port\s+(\d+)`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		if num := parseInt(matches[1]); num > 0 {
			return num
		}
	}
	return 0
}

func extractFilePath(errMsg string) string {
	// Look for file paths in error messages
	re := regexp.MustCompile(`([^\s]+\.(json|yaml|yml|conf|config))`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractConfigType(errMsg string) string {
	if strings.Contains(errMsg, ".json") {
		return "json"
	}
	if strings.Contains(errMsg, ".yaml") || strings.Contains(errMsg, ".yml") {
		return "yaml"
	}
	return "config"
}

func extractCommand(errMsg string) string {
	re := regexp.MustCompile(`command not found:\s*([^\s]+)`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`([^\s]+):\s*not found`)
	matches = re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func getSuggestedPackages(command string) []string {
	suggestions := map[string][]string{
		"git":     {"git"},
		"docker":  {"docker.io", "docker-ce"},
		"kubectl": {"kubectl"},
		"node":    {"nodejs"},
		"npm":     {"npm"},
		"python":  {"python3"},
		"pip":     {"python3-pip"},
		"curl":    {"curl"},
		"wget":    {"wget"},
		"make":    {"build-essential"},
		"gcc":     {"build-essential"},
	}

	if packages, exists := suggestions[command]; exists {
		return packages
	}
	return []string{command}
}

func parseInt(s string) int {
	var result int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			break
		}
	}
	return result
}
