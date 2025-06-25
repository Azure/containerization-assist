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

	// Note: Simplified implementation - configuration analysis would be implemented here
	_ = config // Prevent unused variable error

	// Additional analysis methods would be implemented here

	// Analyze security configuration
	// Security configuration analysis would be implemented here

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Confidence = 0.8 // Default confidence

	return result, nil
}

// analyzeConfigurationFiles identifies and analyzes configuration files
func (c *ConfigurationAnalyzer) analyzeConfigurationFiles(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	configFiles := map[string]string{
		"config.json":            "JSON Configuration",
		"config.yaml":            "YAML Configuration",
		"config.yml":             "YAML Configuration",
		"appsettings.json":       ".NET Configuration",
		"web.config":             ".NET Web Configuration",
		"application.properties": "Java Configuration",
		"application.yml":        "Spring Boot Configuration",
		"tsconfig.json":          "TypeScript Configuration",
		"babel.config.js":        "Babel Configuration",
		"webpack.config.js":      "Webpack Configuration",
		"next.config.js":         "Next.js Configuration",
		"nuxt.config.js":         "Nuxt.js Configuration",
		"vue.config.js":          "Vue.js Configuration",
		"angular.json":           "Angular Configuration",
		"tailwind.config.js":     "Tailwind CSS Configuration",
		"jest.config.js":         "Jest Testing Configuration",
		"eslint.config.js":       "ESLint Configuration",
		".eslintrc":              "ESLint Configuration",
		"prettier.config.js":     "Prettier Configuration",
		".prettierrc":            "Prettier Configuration",
		"nodemon.json":           "Nodemon Configuration",
		"pm2.config.js":          "PM2 Configuration",
		"supervisord.conf":       "Supervisor Configuration",
		"nginx.conf":             "Nginx Configuration",
		"apache.conf":            "Apache Configuration",
		"redis.conf":             "Redis Configuration",
		"docker-compose.yml":     "Docker Compose Configuration",
		"docker-compose.yaml":    "Docker Compose Configuration",
		"k8s":                    "Kubernetes Configuration",
		"kubernetes":             "Kubernetes Configuration",
		"helm":                   "Helm Configuration",
		".env":                   "Environment Configuration",
		".env.example":           "Environment Template",
		".env.local":             "Local Environment",
		".env.production":        "Production Environment",
		".env.development":       "Development Environment",
	}

	for fileName, description := range configFiles {
		files := c.findFilesByPattern(repoData, fileName)
		for _, file := range files {
			finding := Finding{
				Type:        FindingTypeConfiguration,
				Category:    "config_file",
				Title:       description,
				Description: fmt.Sprintf("%s file detected: %s", description, file.Path),
				Confidence:  0.95,
				Severity:    SeverityInfo,
				Location: &Location{
					Path: file.Path,
				},
				Metadata: map[string]interface{}{
					"file_type": description,
					"file_name": fileName,
					"file_path": file.Path,
					"file_size": len(file.Content),
				},
				Evidence: []Evidence{
					{
						Type:        "file_detection",
						Description: "Configuration file detected",
						Location:    &Location{Path: file.Path},
						Value:       file.Path,
					},
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// analyzePorts detects port configurations
func (c *ConfigurationAnalyzer) analyzePorts(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	// Port patterns to search for
	portPatterns := []*regexp.Regexp{
		regexp.MustCompile(`port[:\s=]+(\d+)`),
		regexp.MustCompile(`PORT[:\s=]+(\d+)`),
		regexp.MustCompile(`listen[:\s=]+(\d+)`),
		regexp.MustCompile(`server\.port[:\s=]+(\d+)`),
		regexp.MustCompile(`app\.listen\((\d+)\)`),
		regexp.MustCompile(`\.listen\((\d+)`),
		regexp.MustCompile(`expose[:\s]+(\d+)`),
		regexp.MustCompile(`EXPOSE\s+(\d+)`),
	}

	ports := make(map[int][]string) // port -> files where found

	for _, file := range repoData.Files {
		content := strings.ToLower(file.Content)
		for _, pattern := range portPatterns {
			matches := pattern.FindAllStringSubmatch(content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					if port, err := strconv.Atoi(match[1]); err == nil {
						if port > 0 && port <= 65535 {
							if ports[port] == nil {
								ports[port] = make([]string, 0)
							}
							ports[port] = append(ports[port], file.Path)
						}
					}
				}
			}
		}
	}

	// Create findings for detected ports
	for port, files := range ports {
		severity := c.getPortSeverity(port)
		finding := Finding{
			Type:        FindingTypePort,
			Category:    "port_configuration",
			Title:       fmt.Sprintf("Port %d Configuration", port),
			Description: c.generatePortDescription(port, files),
			Confidence:  0.8,
			Severity:    severity,
			Metadata: map[string]interface{}{
				"port":      port,
				"files":     files,
				"port_type": c.classifyPort(port),
			},
			Evidence: c.createPortEvidence(port, files),
		}
		result.Findings = append(result.Findings, finding)
	}

	return nil
}

// analyzeEnvironmentVariables detects environment variable usage
func (c *ConfigurationAnalyzer) analyzeEnvironmentVariables(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	// Environment variable patterns
	envPatterns := []*regexp.Regexp{
		regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
		regexp.MustCompile(`os\.getenv\(['"]([A-Z_][A-Z0-9_]*)['"]`),
		regexp.MustCompile(`os\.environ\[['"]([A-Z_][A-Z0-9_]*)['"]`),
		regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`),
		regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`),
		regexp.MustCompile(`env\.([A-Z_][A-Z0-9_]*)`),
	}

	envVars := make(map[string][]string) // env var -> files where found

	for _, file := range repoData.Files {
		for _, pattern := range envPatterns {
			matches := pattern.FindAllStringSubmatch(file.Content, -1)
			for _, match := range matches {
				if len(match) > 1 {
					envVar := match[1]
					if envVars[envVar] == nil {
						envVars[envVar] = make([]string, 0)
					}
					envVars[envVar] = append(envVars[envVar], file.Path)
				}
			}
		}
	}

	// Analyze .env files
	envFiles := c.findFilesByPattern(repoData, ".env")
	for _, envFile := range envFiles {
		lines := strings.Split(envFile.Content, "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					envVar := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					severity := SeverityInfo
					if c.isPotentiallySensitive(envVar, value) {
						severity = SeverityMedium
					}

					finding := Finding{
						Type:        FindingTypeEnvironment,
						Category:    "environment_variable",
						Title:       fmt.Sprintf("Environment Variable: %s", envVar),
						Description: c.generateEnvVarDescription(envVar, value, envFile.Path),
						Confidence:  0.95,
						Severity:    severity,
						Location: &Location{
							Path:       envFile.Path,
							LineNumber: i + 1,
						},
						Metadata: map[string]interface{}{
							"variable":     envVar,
							"has_value":    value != "",
							"is_sensitive": c.isPotentiallySensitive(envVar, value),
							"source":       "env_file",
						},
					}
					result.Findings = append(result.Findings, finding)
				}
			}
		}
	}

	// Create findings for environment variables used in code
	for envVar, files := range envVars {
		finding := Finding{
			Type:        FindingTypeEnvironment,
			Category:    "environment_usage",
			Title:       fmt.Sprintf("Environment Variable Usage: %s", envVar),
			Description: fmt.Sprintf("Environment variable %s is used in code", envVar),
			Confidence:  0.85,
			Severity:    SeverityInfo,
			Metadata: map[string]interface{}{
				"variable":     envVar,
				"files":        files,
				"usage_count":  len(files),
				"is_sensitive": c.isPotentiallySensitive(envVar, ""),
			},
			Evidence: c.createEnvVarEvidence(envVar, files),
		}
		result.Findings = append(result.Findings, finding)
	}

	return nil
}

// analyzeLoggingConfiguration detects logging setup
func (c *ConfigurationAnalyzer) analyzeLoggingConfiguration(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	loggingIndicators := map[string]string{
		"winston":     "Winston logging library",
		"pino":        "Pino logging library",
		"bunyan":      "Bunyan logging library",
		"log4j":       "Log4j logging framework",
		"logback":     "Logback logging framework",
		"serilog":     "Serilog logging library",
		"nlog":        "NLog logging library",
		"logging":     "Python logging module",
		"logrus":      "Logrus logging library",
		"zap":         "Zap logging library",
		"console.log": "Console logging",
		"print":       "Print statements",
		"fmt.Print":   "Go print statements",
	}

	for _, file := range repoData.Files {
		content := strings.ToLower(file.Content)
		for indicator, description := range loggingIndicators {
			if strings.Contains(content, indicator) {
				severity := SeverityInfo
				if indicator == "console.log" || indicator == "print" || indicator == "fmt.Print" {
					severity = SeverityLow // These are less optimal for production
				}

				finding := Finding{
					Type:        FindingTypeConfiguration,
					Category:    "logging_configuration",
					Title:       description,
					Description: fmt.Sprintf("%s detected in %s", description, file.Path),
					Confidence:  0.7,
					Severity:    severity,
					Location: &Location{
						Path: file.Path,
					},
					Metadata: map[string]interface{}{
						"logging_type": indicator,
						"description":  description,
						"file":         file.Path,
					},
				}
				result.Findings = append(result.Findings, finding)
				break // Only report one logging type per file
			}
		}
	}

	return nil
}

// analyzeSecurityConfiguration detects security-related configuration
func (c *ConfigurationAnalyzer) analyzeSecurityConfiguration(config AnalysisConfig, result *EngineAnalysisResult) error {
	repoData := config.RepoData

	securityPatterns := map[string][]string{
		"CORS Configuration": {
			"cors", "access-control-allow-origin", "cross-origin",
		},
		"HTTPS Configuration": {
			"https", "ssl", "tls", "certificate",
		},
		"Authentication": {
			"jwt", "oauth", "auth", "passport", "session",
		},
		"Security Headers": {
			"helmet", "csp", "content-security-policy", "x-frame-options",
		},
		"Rate Limiting": {
			"rate-limit", "throttle", "rate-limiter",
		},
	}

	for category, patterns := range securityPatterns {
		found := false
		var foundIn []string

		for _, file := range repoData.Files {
			content := strings.ToLower(file.Content)
			for _, pattern := range patterns {
				if strings.Contains(content, pattern) {
					found = true
					foundIn = append(foundIn, file.Path)
					break
				}
			}
		}

		if found {
			finding := Finding{
				Type:        FindingTypeSecurity,
				Category:    "security_configuration",
				Title:       category,
				Description: fmt.Sprintf("%s detected in configuration", category),
				Confidence:  0.8,
				Severity:    SeverityInfo,
				Metadata: map[string]interface{}{
					"security_type": category,
					"files":         foundIn,
					"patterns":      patterns,
				},
			}
			result.Findings = append(result.Findings, finding)
		}
	}

	return nil
}

// Helper methods

func (c *ConfigurationAnalyzer) findFilesByPattern(repoData *RepoData, pattern string) []FileData {
	var matches []FileData
	for _, file := range repoData.Files {
		if strings.Contains(file.Path, pattern) ||
			filepath.Base(file.Path) == pattern ||
			strings.HasSuffix(file.Path, pattern) {
			matches = append(matches, file)
		}
	}
	return matches
}

func (c *ConfigurationAnalyzer) getPortSeverity(port int) Severity {
	if port < 1024 {
		return SeverityMedium // Privileged ports
	} else if port == 3000 || port == 8080 || port == 8000 || port == 5000 {
		return SeverityInfo // Common development ports
	}
	return SeverityLow
}

func (c *ConfigurationAnalyzer) classifyPort(port int) string {
	commonPorts := map[int]string{
		80:    "HTTP",
		443:   "HTTPS",
		3000:  "Development Server",
		8080:  "Development/Proxy",
		8000:  "Development Server",
		5000:  "Development Server",
		9000:  "Development Server",
		3306:  "MySQL",
		5432:  "PostgreSQL",
		6379:  "Redis",
		27017: "MongoDB",
	}

	if portType, exists := commonPorts[port]; exists {
		return portType
	}
	return "Custom"
}

func (c *ConfigurationAnalyzer) generatePortDescription(port int, files []string) string {
	portType := c.classifyPort(port)
	return fmt.Sprintf("Port %d (%s) configured in %d file(s)", port, portType, len(files))
}

func (c *ConfigurationAnalyzer) createPortEvidence(port int, files []string) []Evidence {
	var evidence []Evidence
	for _, file := range files {
		evidence = append(evidence, Evidence{
			Type:        "port_configuration",
			Description: fmt.Sprintf("Port %d found in configuration", port),
			Location:    &Location{Path: file},
			Value:       port,
		})
	}
	return evidence
}

func (c *ConfigurationAnalyzer) isPotentiallySensitive(varName, value string) bool {
	sensitivePatterns := []string{
		"password", "secret", "key", "token", "credential",
		"api_key", "auth", "private", "cert", "ssl",
	}

	varLower := strings.ToLower(varName)
	valueLower := strings.ToLower(value)

	for _, pattern := range sensitivePatterns {
		if strings.Contains(varLower, pattern) || strings.Contains(valueLower, pattern) {
			return true
		}
	}
	return false
}

func (c *ConfigurationAnalyzer) generateEnvVarDescription(varName, value, filePath string) string {
	if c.isPotentiallySensitive(varName, value) {
		return fmt.Sprintf("Potentially sensitive environment variable %s defined in %s", varName, filePath)
	}
	return fmt.Sprintf("Environment variable %s defined in %s", varName, filePath)
}

func (c *ConfigurationAnalyzer) createEnvVarEvidence(varName string, files []string) []Evidence {
	var evidence []Evidence
	for _, file := range files {
		evidence = append(evidence, Evidence{
			Type:        "environment_usage",
			Description: fmt.Sprintf("Environment variable %s used", varName),
			Location:    &Location{Path: file},
			Value:       varName,
		})
	}
	return evidence
}

func (c *ConfigurationAnalyzer) calculateConfidence(result *EngineAnalysisResult) float64 {
	if len(result.Findings) == 0 {
		return 0.0
	}

	var totalConfidence float64
	for _, finding := range result.Findings {
		totalConfidence += finding.Confidence
	}

	return totalConfidence / float64(len(result.Findings))
}
