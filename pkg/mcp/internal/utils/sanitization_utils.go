package utils

import (
	"regexp"
	"strings"
)

// SanitizationUtils provides centralized sanitization functions
// This consolidates duplicate sanitization functions found across:
// - pkg/mcp/internal/orchestration/tool_registry.go (sanitizeInvopopSchema)
// - pkg/mcp/internal/runtime/registry.go (sanitizeInvopopSchema)
// - pkg/mcp/internal/session/workflow_provider.go (sanitizeForK8s)
// - pkg/mcp/internal/deploy/secrets_handler.go (sanitizeSecretKey)

// SanitizeForKubernetes sanitizes a string to be valid for Kubernetes resource names
// Consolidates sanitizeForK8s function from workflow_provider.go
func SanitizeForKubernetes(input string) string {
	if input == "" {
		return ""
	}

	// Convert to lowercase
	result := strings.ToLower(input)

	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	result = reg.ReplaceAllString(result, "-")

	// Remove leading/trailing hyphens
	result = strings.Trim(result, "-")

	// Ensure it starts with alphanumeric character
	if len(result) > 0 && !regexp.MustCompile(`^[a-z0-9]`).MatchString(result) {
		result = "x" + result
	}

	// Limit length to 63 characters (Kubernetes limit)
	if len(result) > 63 {
		result = result[:63]
	}

	// Ensure it ends with alphanumeric character
	result = strings.TrimRight(result, "-")
	if result == "" {
		result = "default"
	}

	return result
}

// SanitizeSecretKey sanitizes environment variable names for secrets
// Consolidates sanitizeSecretKey function from secrets_handler.go
func SanitizeSecretKey(envName string) string {
	if envName == "" {
		return ""
	}

	// Convert to uppercase
	result := strings.ToUpper(envName)

	// Replace non-alphanumeric characters with underscores
	reg := regexp.MustCompile(`[^A-Z0-9_]+`)
	result = reg.ReplaceAllString(result, "_")

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	// Ensure it starts with letter or underscore
	if len(result) > 0 && !regexp.MustCompile(`^[A-Z_]`).MatchString(result) {
		result = "ENV_" + result
	}

	// Limit reasonable length
	if len(result) > 100 {
		result = result[:100]
	}

	if result == "" {
		result = "DEFAULT_ENV"
	}

	return result
}

// JSONSchemaDefinition represents a JSON schema structure
type JSONSchemaDefinition struct {
	Type                 string                           `json:"type,omitempty"`
	Format               string                           `json:"format,omitempty"`
	Description          string                           `json:"description,omitempty"`
	Default              interface{}                      `json:"default,omitempty"`
	Enum                 []interface{}                    `json:"enum,omitempty"`
	Properties           map[string]*JSONSchemaDefinition `json:"properties,omitempty"`
	Items                *JSONSchemaDefinition            `json:"items,omitempty"`
	Required             []string                         `json:"required,omitempty"`
	AdditionalProperties interface{}                      `json:"additionalProperties,omitempty"`
	Definitions          map[string]*JSONSchemaDefinition `json:"definitions,omitempty"`
}

// SanitizeTypedJSONSchema provides type-safe JSON schema sanitization
func SanitizeTypedJSONSchema(schema *JSONSchemaDefinition) *JSONSchemaDefinition {
	if schema == nil {
		return &JSONSchemaDefinition{}
	}

	// Fix array types that need items
	if schema.Type == "array" && schema.Items == nil {
		schema.Items = &JSONSchemaDefinition{Type: "string"}
	}

	// Recursively sanitize properties
	if schema.Properties != nil {
		for _, propSchema := range schema.Properties {
			SanitizeTypedJSONSchema(propSchema)
		}
	}

	// Recursively sanitize items
	if schema.Items != nil {
		SanitizeTypedJSONSchema(schema.Items)
	}

	// Recursively sanitize definitions
	if schema.Definitions != nil {
		for _, defSchema := range schema.Definitions {
			SanitizeTypedJSONSchema(defSchema)
		}
	}

	return schema
}

// sanitizeSchemaRecursive applies schema sanitization recursively
func sanitizeSchemaRecursive(schema map[string]interface{}) {
	if schema == nil {
		return
	}

	// Fix array types that need items
	if schemaType, ok := schema["type"].(string); ok && schemaType == "array" {
		if _, hasItems := schema["items"]; !hasItems {
			// Default to string items if not specified
			schema["items"] = map[string]interface{}{
				"type": "string",
			}
		}
	}

	// Remove GitHub Copilot incompatible fields
	RemoveCopilotIncompatible(schema)

	// Recursively process properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, propSchema := range properties {
			if propMap, ok := propSchema.(map[string]interface{}); ok {
				sanitizeSchemaRecursive(propMap)
			}
		}
	}

	// Recursively process items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		sanitizeSchemaRecursive(items)
	}

	// Recursively process definitions
	if definitions, ok := schema["definitions"].(map[string]interface{}); ok {
		for _, defSchema := range definitions {
			if defMap, ok := defSchema.(map[string]interface{}); ok {
				sanitizeSchemaRecursive(defMap)
			}
		}
	}
}

// SanitizeDockerImageName sanitizes a string to be a valid Docker image name
func SanitizeDockerImageName(imageName string) string {
	if imageName == "" {
		return ""
	}

	// Convert to lowercase
	result := strings.ToLower(imageName)

	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9._/-]+`)
	result = reg.ReplaceAllString(result, "-")

	// Remove leading/trailing special characters
	result = strings.Trim(result, "-._/")

	// Ensure it doesn't start with special characters
	if len(result) > 0 && regexp.MustCompile(`^[._/-]`).MatchString(result) {
		result = "img-" + result
	}

	if result == "" {
		result = "default-image"
	}

	return result
}

// SanitizeFilename sanitizes a string to be a valid filename
func SanitizeFilename(filename string) string {
	if filename == "" {
		return ""
	}

	// Remove or replace invalid filename characters
	reg := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f\x7f]+`)
	result := reg.ReplaceAllString(filename, "_")

	// Trim spaces and dots from beginning and end
	result = strings.Trim(result, " .")

	// Limit length
	if len(result) > 255 {
		result = result[:255]
	}

	if result == "" {
		result = "file"
	}

	return result
}

// SanitizeIdentifier sanitizes a string to be a valid programming identifier
func SanitizeIdentifier(identifier string) string {
	if identifier == "" {
		return ""
	}

	// Replace non-alphanumeric characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	result := reg.ReplaceAllString(identifier, "_")

	// Ensure it starts with letter or underscore
	if len(result) > 0 && regexp.MustCompile(`^[0-9]`).MatchString(result) {
		result = "_" + result
	}

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	if result == "" {
		result = "identifier"
	}

	return result
}

// SanitizeHTML removes or escapes HTML characters
func SanitizeHTML(input string) string {
	if input == "" {
		return ""
	}

	// Simple HTML character escaping
	replacements := map[string]string{
		"&":  "&amp;",
		"<":  "&lt;",
		">":  "&gt;",
		"\"": "&quot;",
		"'":  "&#x27;",
	}

	result := input
	for char, escape := range replacements {
		result = strings.ReplaceAll(result, char, escape)
	}

	return result
}

// SanitizeSQL removes or escapes SQL injection characters
func SanitizeSQL(input string) string {
	if input == "" {
		return ""
	}

	// Remove common SQL injection patterns
	dangerous := []string{
		"'", "\"", ";", "--", "/*", "*/", "xp_", "sp_",
		"DROP", "DELETE", "INSERT", "UPDATE", "CREATE", "ALTER",
		"EXEC", "EXECUTE", "UNION", "SELECT", "FROM", "WHERE",
	}

	result := input
	for _, pattern := range dangerous {
		result = strings.ReplaceAll(result, pattern, "")
		result = strings.ReplaceAll(result, strings.ToLower(pattern), "")
		result = strings.ReplaceAll(result, strings.ToUpper(pattern), "")
	}

	return strings.TrimSpace(result)
}

// SanitizeLogMessage sanitizes log messages to prevent log injection
func SanitizeLogMessage(message string) string {
	if message == "" {
		return ""
	}

	// Remove carriage returns and newlines to prevent log injection
	result := strings.ReplaceAll(message, "\r", "")
	result = strings.ReplaceAll(result, "\n", " ")

	// Remove null bytes
	result = strings.ReplaceAll(result, "\x00", "")

	// Limit length to prevent log spam
	if len(result) > 1000 {
		result = result[:1000] + "..."
	}

	return result
}

// SanitizeStringMap provides type-safe sanitization for string maps
func SanitizeStringMap(data map[string]string) map[string]string {
	if data == nil {
		return make(map[string]string)
	}

	sanitized := make(map[string]string)
	for k, v := range data {
		sanitized[k] = SanitizeLogMessage(v)
	}
	return sanitized
}

// SanitizeStringSlice provides type-safe sanitization for string slices
func SanitizeStringSlice(data []string) []string {
	if data == nil {
		return []string{}
	}

	sanitized := make([]string, len(data))
	for i, v := range data {
		sanitized[i] = SanitizeLogMessage(v)
	}
	return sanitized
}

// SanitizeStringInterfaceMap provides controlled sanitization for mixed-type maps
// Use this only when interface{} is absolutely necessary
func SanitizeStringInterfaceMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}

	sanitized := make(map[string]interface{})
	for k, v := range data {
		switch val := v.(type) {
		case string:
			sanitized[k] = SanitizeLogMessage(val)
		case []string:
			sanitized[k] = SanitizeStringSlice(val)
		case map[string]string:
			sanitized[k] = SanitizeStringMap(val)
		default:
			// For other types, just pass through
			// Consider adding specific handlers as needed
			sanitized[k] = val
		}
	}
	return sanitized
}

// RemoveCopilotIncompatible removes JSON schema fields that are incompatible with GitHub Copilot
// This includes fields like additionalProperties, patternProperties, allOf, anyOf, oneOf
func RemoveCopilotIncompatible(schema map[string]interface{}) {
	if schema == nil {
		return
	}

	// Remove fields that GitHub Copilot doesn't handle well
	incompatibleFields := []string{
		"additionalProperties",
		"patternProperties",
		"allOf",
		"anyOf",
		"oneOf",
		"not",
		"$ref",
		"$schema",
		"$id",
		"$comment",
		"const",
		"contentMediaType",
		"contentEncoding",
		"if",
		"then",
		"else",
		"dependentSchemas",
		"dependentRequired",
		"unevaluatedProperties",
		"unevaluatedItems",
		"propertyNames",
		"minProperties",
		"maxProperties",
	}

	for _, field := range incompatibleFields {
		delete(schema, field)
	}

	// Recursively process nested schemas
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				RemoveCopilotIncompatible(propMap)
			}
		}
	}

	// Process items for array schemas
	if items, ok := schema["items"].(map[string]interface{}); ok {
		RemoveCopilotIncompatible(items)
	}

	// Process definitions
	if definitions, ok := schema["definitions"].(map[string]interface{}); ok {
		for _, def := range definitions {
			if defMap, ok := def.(map[string]interface{}); ok {
				RemoveCopilotIncompatible(defMap)
			}
		}
	}
}

// RemoveSensitiveData removes potentially sensitive data from strings
func RemoveSensitiveData(input string) string {
	if input == "" {
		return ""
	}

	// Patterns for common sensitive data
	patterns := []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		// API keys, tokens
		{regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*["\']?([a-zA-Z0-9]{20,})["\']?`), "$1=***"},
		// Credit card numbers
		{regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`), "****-****-****-****"},
		// Email addresses (partial masking)
		{regexp.MustCompile(`([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`), "$1@***"},
		// IP addresses (partial masking)
		{regexp.MustCompile(`\b(\d{1,3}\.)(\d{1,3}\.)(\d{1,3}\.)(\d{1,3})\b`), "$1$2$3***"},
	}

	result := input
	for _, pattern := range patterns {
		result = pattern.regex.ReplaceAllString(result, pattern.replacement)
	}

	return result
}
