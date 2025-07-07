package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ConfigurationAnalyzer analyzes configuration files and settings
type ConfigurationAnalyzer struct {
	logger zerolog.Logger
}

// NewConfigurationAnalyzer creates a new configuration analyzer
func NewConfigurationAnalyzer(logger zerolog.Logger) *ConfigurationAnalyzer {
	return &ConfigurationAnalyzer{
		logger: logger.With().Str("engine", "configuration").Logger(),
	}
}

// GetName returns the name of this engine
func (c *ConfigurationAnalyzer) GetName() string {
	return "configuration_analyzer"
}

// GetCapabilities returns what this engine can analyze
func (c *ConfigurationAnalyzer) GetCapabilities() []string {
	return []string{
		"configuration_files",
		"environment_variables",
		"port_detection",
		"secrets_detection",
		"logging_configuration",
		"monitoring_setup",
	}
}

// IsApplicable determines if this engine should run
func (c *ConfigurationAnalyzer) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	// Configuration analysis is always useful
	return true
}

// Analyze performs configuration analysis
func (c *ConfigurationAnalyzer) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	startTime := time.Now()
	result := &EngineAnalysisResult{
		Engine:   c.GetName(),
		Findings: make([]Finding, 0),
		Metadata: make(map[string]interface{}),
		Errors:   make([]error, 0),
	}

	// Analyze environment variables
	c.analyzeEnvironmentVariables(config, result)

	// Analyze port configuration
	c.analyzePortConfiguration(config, result)

	// Analyze secrets detection
	c.analyzeSecretsConfiguration(config, result)

	// Analyze logging configuration
	c.analyzeLoggingConfiguration(config, result)

	// Analyze monitoring setup
	c.analyzeMonitoringConfiguration(config, result)

	// Analyze general configuration files
	c.analyzeConfigurationFiles(config, result)

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0

	// Calculate average confidence from findings
	if len(result.Findings) > 0 {
		totalConfidence := 0.0
		for _, finding := range result.Findings {
			totalConfidence += finding.Confidence
		}
		result.Confidence = totalConfidence / float64(len(result.Findings))
	} else {
		result.Confidence = 0.6 // Default confidence when no findings
	}

	// Store metadata
	result.Metadata["files_analyzed"] = len(config.RepoData.Files)
	result.Metadata["findings_count"] = len(result.Findings)
	result.Metadata["errors_count"] = len(result.Errors)

	return result, nil
}

// analyzeEnvironmentVariables detects environment variables and their usage patterns
func (c *ConfigurationAnalyzer) analyzeEnvironmentVariables(config AnalysisConfig, result *EngineAnalysisResult) {
	envFiles := []string{".env", ".env.local", ".env.development", ".env.production", ".env.example"}

	for _, file := range config.RepoData.Files {
		for _, envFile := range envFiles {
			if strings.HasSuffix(file.Path, envFile) {
				c.analyzeEnvFile(file, result)
				break
			}
		}

		// Check for docker-compose files
		if strings.Contains(strings.ToLower(filepath.Base(file.Path)), "compose") &&
			(strings.HasSuffix(file.Path, ".yml") || strings.HasSuffix(file.Path, ".yaml")) {
			c.analyzeDockerComposeEnv(file, result)
		}
	}
}

// analyzePortConfiguration detects port configurations
func (c *ConfigurationAnalyzer) analyzePortConfiguration(config AnalysisConfig, result *EngineAnalysisResult) {
	portPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)port[\s=:]+([0-9]+)`),
		regexp.MustCompile(`(?i)listen[\s=:]+.*:([0-9]+)`),
		regexp.MustCompile(`(?i)PORT[\s=:]+([0-9]+)`),
		regexp.MustCompile(`(?i)expose[\s]+([0-9]+)`),
	}

	for _, file := range config.RepoData.Files {
		for _, pattern := range portPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					port, err := strconv.Atoi(match[1])
					if err == nil && port > 0 && port <= 65535 {
						c.addPortFinding(file, port, result)
					}
				}
			}
		}
	}
}

// analyzeSecretsConfiguration detects potential secrets in configuration
func (c *ConfigurationAnalyzer) analyzeSecretsConfiguration(config AnalysisConfig, result *EngineAnalysisResult) {
	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key|apikey)[\s=:]+["']?([a-zA-Z0-9_-]{16,})["']?`),
		regexp.MustCompile(`(?i)(secret|password|pwd)[\s=:]+["']?([^\s"']{8,})["']?`),
		regexp.MustCompile(`(?i)(token)[\s=:]+["']?([a-zA-Z0-9_-]{20,})["']?`),
		regexp.MustCompile(`(?i)(database[_-]?url|db[_-]?url)[\s=:]+["']?([^\s"']+)["']?`),
	}

	for _, file := range config.RepoData.Files {
		// Skip binary files and tests
		if c.isBinaryFile(file.Path) || strings.Contains(file.Path, "test") {
			continue
		}

		for _, pattern := range secretPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 2 {
					c.addSecretFinding(file, match[1], match[2], result)
				}
			}
		}
	}
}

// analyzeLoggingConfiguration detects logging configuration
func (c *ConfigurationAnalyzer) analyzeLoggingConfiguration(config AnalysisConfig, result *EngineAnalysisResult) {
	loggingPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(log[_-]?level|logging[_-]?level)[\s=:]+["']?(debug|info|warn|error|fatal)["']?`),
		regexp.MustCompile(`(?i)(log[_-]?format)[\s=:]+["']?(json|text|structured)["']?`),
		regexp.MustCompile(`(?i)(log[_-]?file|log[_-]?path)[\s=:]+["']?([^\s"']+)["']?`),
	}

	for _, file := range config.RepoData.Files {
		for _, pattern := range loggingPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 2 {
					c.addLoggingFinding(file, match[1], match[2], result)
				}
			}
		}
	}
}

// analyzeMonitoringConfiguration detects monitoring and metrics configuration
func (c *ConfigurationAnalyzer) analyzeMonitoringConfiguration(config AnalysisConfig, result *EngineAnalysisResult) {
	monitoringPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(metrics[_-]?port)[\s=:]+([0-9]+)`),
		regexp.MustCompile(`(?i)(health[_-]?check[_-]?path)[\s=:]+["']?([^\s"']+)["']?`),
		regexp.MustCompile(`(?i)(monitoring|tracing|jaeger|zipkin)[\s=:]+["']?(true|enabled)["']?`),
	}

	for _, file := range config.RepoData.Files {
		for _, pattern := range monitoringPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 2 {
					c.addMonitoringFinding(file, match[1], match[2], result)
				}
			}
		}
	}
}

// analyzeConfigurationFiles detects configuration files by extension and naming patterns
func (c *ConfigurationAnalyzer) analyzeConfigurationFiles(config AnalysisConfig, result *EngineAnalysisResult) {
	configFiles := []string{
		".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg",
		"config.js", "config.ts", "settings.py", "application.properties",
	}

	for _, file := range config.RepoData.Files {
		for _, configExt := range configFiles {
			if strings.HasSuffix(file.Path, configExt) || strings.Contains(strings.ToLower(filepath.Base(file.Path)), "config") {
				c.addConfigFileFinding(file, result)
				break
			}
		}
	}
}

// Helper methods for adding findings

func (c *ConfigurationAnalyzer) analyzeEnvFile(file FileData, result *EngineAnalysisResult) {
	lines := strings.Split(file.Content, "\n")
	envVarCount := 0

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		if strings.Contains(line, "=") {
			envVarCount++
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				finding := Finding{
					Type:        FindingTypeEnvironment,
					Category:    "environment_variable",
					Title:       "Environment Variable: " + key,
					Description: "Found environment variable configuration",
					Confidence:  0.9,
					Severity:    SeverityInfo,
					Location: &Location{
						Path:       file.Path,
						LineNumber: i + 1,
					},
					Metadata: map[string]interface{}{
						"key":   key,
						"value": c.maskSensitiveValue(key, value),
						"file":  file.Path,
					},
				}
				result.Findings = append(result.Findings, finding)
			}
		}
	}

	if envVarCount > 0 {
		finding := Finding{
			Type:        FindingTypeConfiguration,
			Category:    "environment_file",
			Title:       "Environment Configuration File",
			Description: fmt.Sprintf("Found environment file with %d variables", envVarCount),
			Confidence:  0.95,
			Severity:    SeverityInfo,
			Location: &Location{
				Path: file.Path,
			},
			Metadata: map[string]interface{}{
				"variable_count": envVarCount,
				"file_type":      "environment",
			},
		}
		result.Findings = append(result.Findings, finding)
	}
}

func (c *ConfigurationAnalyzer) analyzeDockerComposeEnv(file FileData, result *EngineAnalysisResult) {
	// Parse YAML to extract environment variables
	var compose map[string]interface{}
	if err := yaml.Unmarshal([]byte(file.Content), &compose); err != nil {
		return // Skip if YAML parsing fails
	}

	if services, ok := compose["services"].(map[string]interface{}); ok {
		for serviceName, serviceConfig := range services {
			if service, ok := serviceConfig.(map[string]interface{}); ok {
				if env, ok := service["environment"]; ok {
					finding := Finding{
						Type:        FindingTypeEnvironment,
						Category:    "docker_compose_environment",
						Title:       "Docker Compose Environment Variables",
						Description: fmt.Sprintf("Service '%s' has environment configuration", serviceName),
						Confidence:  0.9,
						Severity:    SeverityInfo,
						Location: &Location{
							Path: file.Path,
						},
						Metadata: map[string]interface{}{
							"service":     serviceName,
							"environment": env,
							"file_type":   "docker_compose",
						},
					}
					result.Findings = append(result.Findings, finding)
				}
			}
		}
	}
}

func (c *ConfigurationAnalyzer) addPortFinding(file FileData, port int, result *EngineAnalysisResult) {
	severity := SeverityInfo
	if port < 1024 {
		severity = SeverityMedium // Privileged ports
	}

	finding := Finding{
		Type:        FindingTypePort,
		Category:    "port_configuration",
		Title:       fmt.Sprintf("Port Configuration: %d", port),
		Description: "Found port configuration in file",
		Confidence:  0.8,
		Severity:    severity,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"port":       port,
			"file_type":  c.getFileType(file.Path),
			"privileged": port < 1024,
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (c *ConfigurationAnalyzer) addSecretFinding(file FileData, secretType, value string, result *EngineAnalysisResult) {
	// Reduce confidence for test files or example values
	confidence := 0.7
	if strings.Contains(file.Path, "test") || strings.Contains(file.Path, "example") {
		confidence = 0.4
	}
	if strings.Contains(strings.ToLower(value), "example") || strings.Contains(strings.ToLower(value), "test") {
		confidence = 0.3
	}

	finding := Finding{
		Type:        FindingTypeSecurity,
		Category:    "potential_secret",
		Title:       "Potential Secret: " + secretType,
		Description: "Found potential secret or sensitive configuration",
		Confidence:  confidence,
		Severity:    SeverityMedium,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"secret_type": secretType,
			"value":       c.maskValue(value),
			"file_type":   c.getFileType(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (c *ConfigurationAnalyzer) addLoggingFinding(file FileData, configType, value string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeConfiguration,
		Category:    "logging_configuration",
		Title:       "Logging Configuration: " + configType,
		Description: "Found logging configuration",
		Confidence:  0.85,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"config_type": configType,
			"value":       value,
			"file_type":   c.getFileType(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (c *ConfigurationAnalyzer) addMonitoringFinding(file FileData, configType, value string, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeConfiguration,
		Category:    "monitoring_configuration",
		Title:       "Monitoring Configuration: " + configType,
		Description: "Found monitoring or metrics configuration",
		Confidence:  0.8,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"config_type": configType,
			"value":       value,
			"file_type":   c.getFileType(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

func (c *ConfigurationAnalyzer) addConfigFileFinding(file FileData, result *EngineAnalysisResult) {
	finding := Finding{
		Type:        FindingTypeConfiguration,
		Category:    "configuration_file",
		Title:       "Configuration File: " + filepath.Base(file.Path),
		Description: "Found configuration file",
		Confidence:  0.75,
		Severity:    SeverityInfo,
		Location: &Location{
			Path: file.Path,
		},
		Metadata: map[string]interface{}{
			"file_size": file.Size,
			"file_type": c.getFileType(file.Path),
		},
	}
	result.Findings = append(result.Findings, finding)
}

// Helper utility methods

func (c *ConfigurationAnalyzer) isBinaryFile(path string) bool {
	binaryExts := []string{".exe", ".dll", ".so", ".dylib", ".bin", ".jar", ".class", ".pyc", ".o", ".a"}
	ext := strings.ToLower(filepath.Ext(path))
	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}
	return false
}

func (c *ConfigurationAnalyzer) getFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".ini":
		return "ini"
	case ".env":
		return "environment"
	case ".conf", ".cfg":
		return "config"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".properties":
		return "properties"
	default:
		return "unknown"
	}
}

func (c *ConfigurationAnalyzer) maskSensitiveValue(key, value string) string {
	sensitiveKeys := []string{"password", "secret", "key", "token", "credential"}
	keyLower := strings.ToLower(key)

	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(keyLower, sensitiveKey) {
			return c.maskValue(value)
		}
	}
	return value
}

func (c *ConfigurationAnalyzer) maskValue(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}
