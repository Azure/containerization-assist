package utils

import (
	"os"
	"regexp"
	"strings"
	"sync"
)

// SecretMasker provides utilities for masking sensitive information in logs and outputs
type SecretMasker struct {
	patterns      map[string]*regexp.Regexp
	customSecrets map[string]bool
	envSecrets    map[string]string
	mu            sync.RWMutex
}

// NewSecretMasker creates a new secret masker with default patterns
func NewSecretMasker() *SecretMasker {
	masker := &SecretMasker{
		patterns:      make(map[string]*regexp.Regexp),
		customSecrets: make(map[string]bool),
		envSecrets:    make(map[string]string),
	}

	// Initialize with common patterns
	masker.initializeDefaultPatterns()
	masker.loadEnvironmentSecrets()

	return masker
}

// initializeDefaultPatterns sets up common regex patterns for sensitive data
func (m *SecretMasker) initializeDefaultPatterns() {
	// API keys and tokens
	m.patterns["api_key"] = regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?token)[\s=:]+["']?([a-zA-Z0-9\-_]{20,})["']?`)
	m.patterns["token"] = regexp.MustCompile(`(?i)(token|bearer|auth[_-]?token)[\s=:]+["']?([a-zA-Z0-9\-_\.]{20,})["']?`)

	// Passwords
	m.patterns["password"] = regexp.MustCompile(`(?i)(password|passwd|pwd)[\s=:]+["']?([^"'\s]{6,})["']?`)

	// AWS credentials
	m.patterns["aws_access"] = regexp.MustCompile(`(?i)(aws[_-]?access[_-]?key[_-]?id|AKIA[A-Z0-9]{16})[\s=:]+["']?([A-Z0-9]{20})["']?`)
	m.patterns["aws_secret"] = regexp.MustCompile(`(?i)(aws[_-]?secret[_-]?access[_-]?key)[\s=:]+["']?([a-zA-Z0-9/+=]{40})["']?`)

	// Azure credentials
	m.patterns["azure_key"] = regexp.MustCompile(`(?i)(azure[_-]?key|azure[_-]?secret)[\s=:]+["']?([a-zA-Z0-9\-_]{20,})["']?`)

	// Docker registry credentials
	m.patterns["docker_auth"] = regexp.MustCompile(`(?i)(docker[_-]?auth|registry[_-]?auth)[\s=:]+["']?([a-zA-Z0-9+/=]{20,})["']?`)

	// SSH private keys
	m.patterns["ssh_private"] = regexp.MustCompile(`(?m)-----BEGIN (RSA |DSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]+?-----END (RSA |DSA |EC |OPENSSH )?PRIVATE KEY-----`)

	// JWT tokens
	m.patterns["jwt"] = regexp.MustCompile(`(?i)(jwt|bearer)[\s=:]+["']?(eyJ[a-zA-Z0-9\-_]+\.eyJ[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+)["']?`)

	// GitHub tokens
	m.patterns["github"] = regexp.MustCompile(`(?i)(ghp_[a-zA-Z0-9]{36}|gho_[a-zA-Z0-9]{36}|github[_-]?token)[\s=:]+["']?([a-zA-Z0-9]{40})["']?`)

	// Connection strings
	m.patterns["connection_string"] = regexp.MustCompile(`(?i)(connection[_-]?string|conn[_-]?str|database[_-]?url|db[_-]?url)[\s=:]+["']?([^"'\s]+)["']?`)

	// Generic secrets
	m.patterns["secret"] = regexp.MustCompile(`(?i)(secret|private[_-]?key|client[_-]?secret)[\s=:]+["']?([a-zA-Z0-9\-_]{10,})["']?`)
}

// loadEnvironmentSecrets loads sensitive environment variables
func (m *SecretMasker) loadEnvironmentSecrets() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Common sensitive environment variable patterns
	sensitivePatterns := []string{
		"PASSWORD", "PASSWD", "PWD",
		"SECRET", "KEY", "TOKEN",
		"API", "AUTH",
		"CREDENTIAL", "CRED",
		"AZURE", "AWS", "GCP",
		"DOCKER", "REGISTRY",
		"DATABASE", "DB",
		"MONGO", "REDIS", "POSTGRES", "MYSQL",
	}

	// Load all environment variables and check for sensitive patterns
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Skip empty values
		if value == "" {
			continue
		}

		// Check if the key contains any sensitive pattern
		upperKey := strings.ToUpper(key)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(upperKey, pattern) {
				m.envSecrets[key] = value
				break
			}
		}
	}
}

// Mask masks sensitive information in the input string
func (m *SecretMasker) Mask(input string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	masked := input

	// Apply regex patterns
	for _, pattern := range m.patterns {
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			// Extract the actual secret value from the match
			submatches := pattern.FindStringSubmatch(match)
			if len(submatches) > 2 {
				// Replace the secret part with asterisks
				secretPart := submatches[2]
				replacement := maskValue(secretPart)
				return strings.Replace(match, secretPart, replacement, 1)
			}
			// For patterns without capture groups, mask the entire match
			return maskValue(match)
		})
	}

	// Mask environment secrets
	for _, envValue := range m.envSecrets {
		if envValue != "" && strings.Contains(masked, envValue) {
			masked = strings.ReplaceAll(masked, envValue, maskValue(envValue))
		}
	}

	// Mask custom secrets
	for secret := range m.customSecrets {
		if secret != "" && strings.Contains(masked, secret) {
			masked = strings.ReplaceAll(masked, secret, maskValue(secret))
		}
	}

	return masked
}

// AddCustomSecret adds a custom secret to be masked
func (m *SecretMasker) AddCustomSecret(secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if secret != "" {
		m.customSecrets[secret] = true
	}
}

// AddCustomPattern adds a custom regex pattern for masking
func (m *SecretMasker) AddCustomPattern(name string, pattern string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	m.patterns[name] = regex
	return nil
}

// maskValue returns a masked version of the secret value
func maskValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}

	// Show first 2 and last 2 characters for longer secrets
	if len(value) <= 8 {
		return value[:1] + strings.Repeat("*", len(value)-2) + value[len(value)-1:]
	}

	// For longer secrets, show first 3 and last 3 characters
	return value[:3] + strings.Repeat("*", len(value)-6) + value[len(value)-3:]
}

// MaskMap masks sensitive values in a map
func (m *SecretMasker) MaskMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range input {
		switch v := value.(type) {
		case string:
			result[key] = m.Mask(v)
		case map[string]interface{}:
			result[key] = m.MaskMap(v)
		case []interface{}:
			masked := make([]interface{}, len(v))
			for i, item := range v {
				if str, ok := item.(string); ok {
					masked[i] = m.Mask(str)
				} else if subMap, ok := item.(map[string]interface{}); ok {
					masked[i] = m.MaskMap(subMap)
				} else {
					masked[i] = item
				}
			}
			result[key] = masked
		default:
			result[key] = value
		}
	}

	return result
}

// Global instance for convenience
var defaultMasker = NewSecretMasker()

// Mask is a convenience function using the default masker
func Mask(input string) string {
	return defaultMasker.Mask(input)
}

// MaskMap is a convenience function using the default masker
func MaskMap(input map[string]interface{}) map[string]interface{} {
	return defaultMasker.MaskMap(input)
}

// AddCustomSecret is a convenience function using the default masker
func AddCustomSecret(secret string) {
	defaultMasker.AddCustomSecret(secret)
}

// AddCustomPattern is a convenience function using the default masker
func AddCustomPattern(name string, pattern string) error {
	return defaultMasker.AddCustomPattern(name, pattern)
}
