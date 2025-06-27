package utils

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// ValidationUtils provides centralized validation functions
// This consolidates duplicate validation functions found across the codebase

// IsEmpty checks if a string is empty or contains only whitespace
// Consolidates isEmpty/isBlank functions from multiple files
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsValidURL validates if a string is a valid URL
func IsValidURL(s string) bool {
	if IsEmpty(s) {
		return false
	}

	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	// Check if scheme and host are present
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

// IsValidEmail validates if a string is a valid email address
func IsValidEmail(email string) bool {
	if IsEmpty(email) {
		return false
	}

	// Basic email regex - can be enhanced as needed
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// IsValidPort validates if a string represents a valid port number
func IsValidPort(port string) bool {
	if IsEmpty(port) {
		return false
	}

	// Port should be a number between 1 and 65535
	portRegex := regexp.MustCompile(`^([1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$`)
	return portRegex.MatchString(port)
}

// IsValidIPAddress validates if a string is a valid IP address (IPv4 or IPv6)
func IsValidIPAddress(ip string) bool {
	if IsEmpty(ip) {
		return false
	}

	// IPv4 regex
	ipv4Regex := regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
	if ipv4Regex.MatchString(ip) {
		return true
	}

	// IPv6 regex (simplified)
	ipv6Regex := regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::1$|^::$`)
	return ipv6Regex.MatchString(ip)
}

// IsValidDomainName validates if a string is a valid domain name
func IsValidDomainName(domain string) bool {
	if IsEmpty(domain) {
		return false
	}

	// Domain name regex
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain) && len(domain) <= 253
}

// IsValidDockerImageName validates Docker image name format
func IsValidDockerImageName(imageName string) bool {
	if IsEmpty(imageName) {
		return false
	}

	// Docker image name regex (simplified)
	// Allows: registry.com/namespace/image:tag
	imageRegex := regexp.MustCompile(`^([a-z0-9.-]+/)?[a-z0-9._-]+(/[a-z0-9._-]+)*(:[a-zA-Z0-9._-]+)?$`)
	return imageRegex.MatchString(imageName)
}

// IsValidKubernetesName validates Kubernetes resource name format
func IsValidKubernetesName(name string) bool {
	if IsEmpty(name) {
		return false
	}

	// Kubernetes name must be lowercase alphanumeric with hyphens
	// Must start and end with alphanumeric character
	// Maximum 63 characters
	k8sRegex := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	return k8sRegex.MatchString(name)
}

// IsValidEnvironmentVariable validates environment variable name format
func IsValidEnvironmentVariable(envVar string) bool {
	if IsEmpty(envVar) {
		return false
	}

	// Environment variable names should contain only uppercase letters, numbers, and underscores
	// Should start with a letter or underscore
	envRegex := regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	return envRegex.MatchString(envVar)
}

// IsValidSecretKey validates secret key format (used for environment variables)
func IsValidSecretKey(key string) bool {
	if IsEmpty(key) {
		return false
	}

	// Secret keys follow similar rules to environment variables but can be mixed case
	secretRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	return secretRegex.MatchString(key)
}

// IsValidJSONString validates if a string is valid JSON
func IsValidJSONString(jsonStr string) bool {
	if IsEmpty(jsonStr) {
		return false
	}

	// Try to parse as JSON - if it succeeds, it's valid
	var js interface{}
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

// ContainsUnsafeCharacters checks if a string contains potentially unsafe characters
func ContainsUnsafeCharacters(s string) bool {
	if IsEmpty(s) {
		return false
	}

	// Check for common unsafe characters that might be used in injection attacks
	unsafeChars := []string{
		"<script", "</script", "javascript:", "data:",
		"../", "..\\",
		"$(", "`",
		"||", "&&", ";", "|",
		"DROP TABLE", "SELECT *", "INSERT INTO", "DELETE FROM",
	}

	lowerStr := strings.ToLower(s)
	for _, unsafe := range unsafeChars {
		if strings.Contains(lowerStr, strings.ToLower(unsafe)) {
			return true
		}
	}

	return false
}

// IsNumeric checks if a string contains only numeric characters
func IsNumeric(s string) bool {
	if IsEmpty(s) {
		return false
	}

	numericRegex := regexp.MustCompile(`^[0-9]+$`)
	return numericRegex.MatchString(s)
}

// IsAlphanumeric checks if a string contains only alphanumeric characters
func IsAlphanumeric(s string) bool {
	if IsEmpty(s) {
		return false
	}

	alphanumericRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	return alphanumericRegex.MatchString(s)
}

// IsAlphabetic checks if a string contains only alphabetic characters
func IsAlphabetic(s string) bool {
	if IsEmpty(s) {
		return false
	}

	alphabeticRegex := regexp.MustCompile(`^[a-zA-Z]+$`)
	return alphabeticRegex.MatchString(s)
}

// ValidateStringLength checks if string length is within specified bounds
func ValidateStringLength(s string, minLength, maxLength int) bool {
	length := len(s)
	return length >= minLength && length <= maxLength
}

// SanitizeInput removes potentially unsafe characters from input
func SanitizeInput(input string) string {
	if IsEmpty(input) {
		return ""
	}

	// Remove null bytes
	sanitized := strings.ReplaceAll(input, "\x00", "")

	// Remove carriage returns to prevent CRLF injection
	sanitized = strings.ReplaceAll(sanitized, "\r", "")

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// ValidateRequired checks if all required fields are non-empty
func ValidateRequired(fields map[string]string) []string {
	var missing []string

	for fieldName, fieldValue := range fields {
		if IsEmpty(fieldValue) {
			missing = append(missing, fieldName)
		}
	}

	return missing
}
