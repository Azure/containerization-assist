package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/internal/common/utils"
	"github.com/rs/zerolog"
)

// SecretsHandler handles secret detection and management
type SecretsHandler struct {
	secretScanner   *utils.SecretScanner
	secretGenerator *kubernetes.SecretGenerator
	logger          zerolog.Logger
}

// NewSecretsHandler creates a new secrets handler
func NewSecretsHandler(logger zerolog.Logger) *SecretsHandler {
	return &SecretsHandler{
		secretScanner:   utils.NewSecretScanner(),
		secretGenerator: kubernetes.NewSecretGenerator(logger),
		logger:          logger.With().Str("component", "secrets_handler").Logger(),
	}
}

// ScanForSecrets scans environment variables for potential secrets
func (h *SecretsHandler) ScanForSecrets(environment []SecretValue) ([]SecretInfo, error) {
	h.logger.Info().Int("env_count", len(environment)).Msg("Scanning for secrets in environment variables")

	var secrets []SecretInfo

	for _, env := range environment {
		if env.Value == "" {
			continue
		}

		// Check if this is a potential secret
		if h.isPotentialSecret(env.Name, env.Value) {
			secretInfo := h.analyzeSecret(env.Name, env.Value)
			secrets = append(secrets, secretInfo)

			h.logger.Info().
				Str("name", env.Name).
				Str("type", secretInfo.Type).
				Float64("confidence", secretInfo.Confidence).
				Msg("Detected potential secret")
		}
	}

	h.logger.Info().Int("secrets_found", len(secrets)).Msg("Secret scanning completed")
	return secrets, nil
}

// GenerateSecretManifests generates Kubernetes Secret manifests
func (h *SecretsHandler) GenerateSecretManifests(secrets []SecretInfo, namespace string) ([]ManifestFile, error) {
	if len(secrets) == 0 {
		return nil, nil
	}

	h.logger.Info().
		Int("secrets_count", len(secrets)).
		Str("namespace", namespace).
		Msg("Generating secret manifests")

	// Group secrets by their secret name
	secretGroups := h.groupSecretsByName(secrets)
	var manifests []ManifestFile

	for secretName, secretInfos := range secretGroups {
		manifest, err := h.generateSecretManifest(secretName, secretInfos, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to generate secret %s: %w", secretName, err)
		}
		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

// ExternalizeSecrets updates environment variables to reference Kubernetes secrets
func (h *SecretsHandler) ExternalizeSecrets(environment []SecretValue, secrets []SecretInfo) ([]SecretValue, error) {
	h.logger.Info().
		Int("env_count", len(environment)).
		Int("secrets_count", len(secrets)).
		Msg("Externalizing secrets")

	// Create a map for quick lookup
	secretMap := make(map[string]SecretInfo)
	for _, secret := range secrets {
		secretMap[secret.Name] = secret
	}

	// Update environment variables
	var updated []SecretValue
	for _, env := range environment {
		if secretInfo, isSecret := secretMap[env.Name]; isSecret && secretInfo.IsSecret {
			// Replace with secret reference
			updated = append(updated, SecretValue{
				Name: env.Name,
				Value: fmt.Sprintf("$(SECRET_%s_%s)",
					strings.ToUpper(secretInfo.SecretName),
					strings.ToUpper(secretInfo.SecretKey)),
			})
		} else {
			// Keep as-is
			updated = append(updated, env)
		}
	}

	return updated, nil
}

// isPotentialSecret checks if a variable might be a secret
func (h *SecretsHandler) isPotentialSecret(name, value string) bool {
	// Check by name patterns
	nameLower := strings.ToLower(name)
	secretNamePatterns := []string{
		"password", "passwd", "pwd", "secret", "key", "token", "api",
		"auth", "credential", "private", "cert", "connection", "conn_str",
	}

	for _, pattern := range secretNamePatterns {
		if strings.Contains(nameLower, pattern) {
			return true
		}
	}

	// Check by value patterns
	return h.looksLikeSecret(value)
}

// looksLikeSecret analyzes if a value looks like a secret
func (h *SecretsHandler) looksLikeSecret(value string) bool {
	// Skip very short values
	if len(value) < 8 {
		return false
	}

	// Common secret patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^[A-Za-z0-9+/]{20,}={0,2}$`),                   // Base64
		regexp.MustCompile(`^[a-fA-F0-9]{32,}$`),                           // Hex (MD5, SHA, etc)
		regexp.MustCompile(`^(mongodb|postgres|mysql|redis)://`),           // Connection strings
		regexp.MustCompile(`^(sk|pk|tok)_[a-zA-Z0-9]{20,}$`),               // API keys
		regexp.MustCompile(`^[A-Z0-9_]{20,}$`),                             // AWS-style keys
		regexp.MustCompile(`^-----BEGIN (RSA |PRIVATE |PUBLIC )?KEY-----`), // PEM keys
	}

	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}

	// High entropy check (simplified)
	if h.hasHighEntropy(value) {
		return true
	}

	return false
}

// hasHighEntropy performs a simplified entropy check
func (h *SecretsHandler) hasHighEntropy(value string) bool {
	// Very simplified entropy check
	// In production, use proper Shannon entropy calculation
	uniqueChars := make(map[rune]bool)
	for _, char := range value {
		uniqueChars[char] = true
	}

	// If the ratio of unique characters to length is high, it might be random
	ratio := float64(len(uniqueChars)) / float64(len(value))
	return ratio > 0.7 && len(value) > 16
}

// analyzeSecret provides detailed analysis of a potential secret
func (h *SecretsHandler) analyzeSecret(name, value string) SecretInfo {
	info := SecretInfo{
		Name:       name,
		Value:      value,
		IsSecret:   true,
		SecretName: h.generateSecretName(name),
		SecretKey:  h.sanitizeSecretKey(name),
	}

	// Determine secret type and confidence
	nameLower := strings.ToLower(name)

	switch {
	case strings.Contains(nameLower, "password") || strings.Contains(nameLower, "passwd"):
		info.Type = "password"
		info.Confidence = 0.95
		info.Reason = "Variable name contains 'password'"
		info.IsSensitive = true

	case strings.Contains(nameLower, "api_key") || strings.Contains(nameLower, "apikey"):
		info.Type = "api_key"
		info.Confidence = 0.9
		info.Reason = "Variable name indicates API key"
		info.IsSensitive = true

	case strings.Contains(nameLower, "token"):
		info.Type = "token"
		info.Confidence = 0.9
		info.Reason = "Variable name contains 'token'"
		info.IsSensitive = true

	case strings.Contains(nameLower, "connection") || strings.Contains(nameLower, "conn_str"):
		info.Type = "connection_string"
		info.Confidence = 0.85
		info.Reason = "Variable name indicates connection string"
		info.IsSensitive = true

	case strings.Contains(nameLower, "cert") || strings.Contains(nameLower, "certificate"):
		info.Type = "certificate"
		info.Confidence = 0.9
		info.Reason = "Variable name indicates certificate"
		info.IsSensitive = true

	case h.looksLikeSecret(value):
		info.Type = "generic_secret"
		info.Confidence = 0.7
		info.Reason = "Value has high entropy or matches secret pattern"
		info.IsSensitive = true

	default:
		info.Type = "unknown"
		info.Confidence = 0.5
		info.Reason = "Potential sensitive data"
		info.IsSensitive = false
	}

	// Set pattern if detected
	if pattern := h.detectPattern(value); pattern != "" {
		info.Pattern = pattern
		info.Confidence = commonUtils.MinFloat(info.Confidence+0.1, 1.0)
	}

	return info
}

// detectPattern identifies the pattern of a secret value
func (h *SecretsHandler) detectPattern(value string) string {
	switch {
	case regexp.MustCompile(`^[A-Za-z0-9+/]{20,}={0,2}$`).MatchString(value):
		return "base64"
	case regexp.MustCompile(`^[a-fA-F0-9]{32}$`).MatchString(value):
		return "md5"
	case regexp.MustCompile(`^[a-fA-F0-9]{40}$`).MatchString(value):
		return "sha1"
	case regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(value):
		return "sha256"
	case strings.HasPrefix(value, "mongodb://") || strings.HasPrefix(value, "postgres://"):
		return "connection_string"
	case regexp.MustCompile(`^-----BEGIN`).MatchString(value):
		return "pem_key"
	default:
		return ""
	}
}

// generateSecretName generates a Kubernetes-compliant secret name
func (h *SecretsHandler) generateSecretName(envName string) string {
	// Convert to lowercase and replace invalid characters
	name := strings.ToLower(envName)
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	// Add suffix to indicate it's a secret
	if !strings.HasSuffix(name, "-secret") {
		name = name + "-secret"
	}

	// Ensure it's not too long (Kubernetes limit is 253 characters)
	if len(name) > 253 {
		name = name[:253]
	}

	return name
}

// sanitizeSecretKey creates a valid secret key from environment variable name
func (h *SecretsHandler) sanitizeSecretKey(envName string) string {
	// Keep original case but replace invalid characters
	key := regexp.MustCompile(`[^a-zA-Z0-9_.-]`).ReplaceAllString(envName, "_")
	return key
}

// groupSecretsByName groups secrets by their Kubernetes secret name
func (h *SecretsHandler) groupSecretsByName(secrets []SecretInfo) map[string][]SecretInfo {
	groups := make(map[string][]SecretInfo)
	for _, secret := range secrets {
		groups[secret.SecretName] = append(groups[secret.SecretName], secret)
	}
	return groups
}

// generateSecretManifest generates a single Kubernetes Secret manifest
func (h *SecretsHandler) generateSecretManifest(secretName string, secrets []SecretInfo, namespace string) (ManifestFile, error) {
	// Build secret data
	secretData := make(map[string][]byte)
	for _, secret := range secrets {
		secretData[secret.SecretKey] = []byte(secret.Value)
	}

	// Generate manifest using the secret generator
	options := kubernetes.SecretOptions{
		Name:      secretName,
		Namespace: namespace,
		Data:      secretData,
		Type:      "Opaque",
	}

	result, err := h.secretGenerator.GenerateSecret(context.Background(), options)
	if err != nil {
		return ManifestFile{}, fmt.Errorf("failed to generate secret: %w", err)
	}

	// Create secret info message
	var infoBuilder strings.Builder
	infoBuilder.WriteString(fmt.Sprintf("Secret '%s' contains %d key(s): ", secretName, len(secrets)))
	var keys []string
	for _, secret := range secrets {
		keys = append(keys, fmt.Sprintf("%s (%s)", secret.SecretKey, secret.Type))
	}
	infoBuilder.WriteString(strings.Join(keys, ", "))

	// Serialize the secret to YAML
	var content strings.Builder
	content.WriteString(fmt.Sprintf("apiVersion: %s\n", result.Secret.APIVersion))
	content.WriteString(fmt.Sprintf("kind: %s\n", result.Secret.Kind))
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: %s\n", result.Secret.Metadata.Name))
	if result.Secret.Metadata.Namespace != "" {
		content.WriteString(fmt.Sprintf("  namespace: %s\n", result.Secret.Metadata.Namespace))
	}
	content.WriteString(fmt.Sprintf("type: %s\n", result.Secret.Type))
	content.WriteString("data:\n")
	for key, value := range result.Secret.Data {
		content.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	}

	return ManifestFile{
		Kind:       "Secret",
		Name:       secretName,
		Content:    content.String(),
		FilePath:   filepath.Join("manifests", fmt.Sprintf("%s.yaml", secretName)),
		IsSecret:   true,
		SecretInfo: infoBuilder.String(),
	}, nil
}
