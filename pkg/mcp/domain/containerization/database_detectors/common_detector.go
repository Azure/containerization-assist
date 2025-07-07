package database_detectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DetectDockerComposeServices detects services in docker-compose files
func DetectDockerComposeServices(repoPath string, serviceNames []string) ([]DockerService, error) {
	var services []DockerService

	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, composeFile := range composeFiles {
		composePath := filepath.Join(repoPath, composeFile)
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(composePath)
		if err != nil {
			continue
		}

		var compose DockerCompose
		if err := yaml.Unmarshal(content, &compose); err != nil {
			// Try to parse as string-based search if YAML parsing fails
			services = append(services, detectServicesFromText(string(content), serviceNames, composePath)...)
			continue
		}

		for serviceName, service := range compose.Services {
			if containsAny(serviceName, serviceNames) || containsAny(service.Image, serviceNames) {
				dockerService := DockerService{
					Name:        serviceName,
					Image:       service.Image,
					Ports:       service.Ports,
					Environment: service.Environment,
					Volumes:     service.Volumes,
				}
				services = append(services, dockerService)
			}
		}
	}

	return services, nil
}

// DetectEnvironmentVariables detects environment variables with specific prefixes
func DetectEnvironmentVariables(repoPath string, varPrefixes []string) ([]EnvironmentVar, error) {
	var envVars []EnvironmentVar

	envFiles := []string{
		".env",
		".env.local",
		".env.development",
		".env.production",
		".env.example",
	}

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Check for .env files
		for _, envFile := range envFiles {
			if strings.HasSuffix(path, envFile) {
				vars, err := parseEnvFile(path, varPrefixes)
				if err == nil {
					envVars = append(envVars, vars...)
				}
				break
			}
		}

		// Check for environment variables in docker-compose files
		if strings.Contains(strings.ToLower(info.Name()), "compose") &&
			(strings.HasSuffix(info.Name(), ".yml") || strings.HasSuffix(info.Name(), ".yaml")) {
			vars, err := parseDockerComposeEnvVars(path, varPrefixes)
			if err == nil {
				envVars = append(envVars, vars...)
			}
		}

		return nil
	})

	return envVars, err
}

// DetectConfigFiles detects configuration files matching patterns
func DetectConfigFiles(repoPath string, patterns []string) ([]ConfigFile, error) {
	var configFiles []ConfigFile

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if info.IsDir() {
			return nil
		}

		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				continue
			}

			if matched {
				configFile := ConfigFile{
					Path: path,
					Type: detectConfigType(info.Name()),
				}

				// Try to parse basic settings if it's a recognized format
				if settings, err := parseConfigFile(path); err == nil {
					configFile.Settings = settings
				}

				configFiles = append(configFiles, configFile)
				break
			}
		}

		return nil
	})

	return configFiles, err
}

// Helper types for Docker Compose parsing
type DockerCompose struct {
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image       string            `yaml:"image"`
	Ports       []string          `yaml:"ports"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []string          `yaml:"volumes"`
}

// Helper functions

func containsAny(str string, needles []string) bool {
	strLower := strings.ToLower(str)
	for _, needle := range needles {
		if strings.Contains(strLower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func detectServicesFromText(content string, serviceNames []string, source string) []DockerService {
	var services []DockerService

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "image:") {
			for _, serviceName := range serviceNames {
				if strings.Contains(strings.ToLower(line), strings.ToLower(serviceName)) {
					// Extract image name
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						image := strings.TrimSpace(strings.Join(parts[1:], ":"))
						services = append(services, DockerService{
							Name:  serviceName,
							Image: image,
						})
					}
					break
				}
			}
		}
	}

	return services
}

func parseEnvFile(path string, prefixes []string) ([]EnvironmentVar, error) {
	var envVars []EnvironmentVar

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check for KEY=VALUE format
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				for _, prefix := range prefixes {
					if strings.HasPrefix(strings.ToUpper(key), strings.ToUpper(prefix)) {
						envVars = append(envVars, EnvironmentVar{
							Name:   key,
							Value:  value,
							Source: path,
						})
						break
					}
				}
			}
		}
	}

	return envVars, scanner.Err()
}

func parseDockerComposeEnvVars(path string, prefixes []string) ([]EnvironmentVar, error) {
	var envVars []EnvironmentVar

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use regex to find environment variables in docker-compose files
	envRegex := regexp.MustCompile(`(?i)(?:environment|env):\s*\n(?:\s*-?\s*([A-Z_][A-Z0-9_]*)\s*[=:]\s*([^\n]+))+`)
	matches := envRegex.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])

			for _, prefix := range prefixes {
				if strings.HasPrefix(strings.ToUpper(key), strings.ToUpper(prefix)) {
					envVars = append(envVars, EnvironmentVar{
						Name:   key,
						Value:  value,
						Source: path,
					})
					break
				}
			}
		}
	}

	return envVars, nil
}

func detectConfigType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.ToLower(filename)

	switch ext {
	case ".conf":
		return "config"
	case ".cfg":
		return "config"
	case ".ini":
		return "ini"
	case ".yml", ".yaml":
		return "yaml"
	case ".json":
		return "json"
	case ".env":
		return "env"
	default:
		if strings.Contains(name, "compose") {
			return "docker-compose"
		}
		return "unknown"
	}
}

func parseConfigFile(path string) (map[string]string, error) {
	settings := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || line == "" {
			continue
		}

		// Look for key-value pairs
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				settings[key] = value
			}
		}
	}

	return settings, scanner.Err()
}

// ValidateDetection validates a database detection with confidence scoring
func ValidateDetection(db DetectedDatabase) float64 {
	confidence := 0.0

	// Base confidence from evidence sources
	for _, source := range db.EvidenceSources {
		switch {
		case strings.Contains(source, "config"):
			confidence += 0.4
		case strings.Contains(source, "docker"):
			confidence += 0.3
		case strings.Contains(source, "env"):
			confidence += 0.2
		case strings.Contains(source, "code"):
			confidence += 0.1
		}
	}

	// Boost confidence if version is detected
	if db.Version != "" && db.Version != "unknown" {
		confidence += 0.1
	}

	// Boost confidence if connection info is available
	if db.ConnectionInfo.Host != "" || db.ConnectionInfo.Port > 0 {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// ExtractVersion extracts version from text using common patterns
func ExtractVersion(text string, dbNames []string) string {
	for _, dbName := range dbNames {
		// Pattern: database:version, database-version, database version
		patterns := []string{
			fmt.Sprintf(`(?i)%s[:\-\s]+v?(\d+\.\d+(?:\.\d+)?)`, dbName),
			fmt.Sprintf(`(?i)%s.*?(\d+\.\d+(?:\.\d+)?)`, dbName),
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(text)
			if len(matches) > 1 {
				version := matches[1]
				// Validate version format
				if matched, _ := regexp.MatchString(`^\d+\.\d+(\.\d+)?$`, version); matched {
					return version
				}
			}
		}
	}

	return "unknown"
}
