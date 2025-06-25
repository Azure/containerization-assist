package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// SecretScanner detects sensitive values in environment variables
type SecretScanner struct {
	// Patterns that indicate sensitive data
	sensitivePatterns []*regexp.Regexp

	// Common secret management solutions
	secretManagers []SecretManager
}

// SecretManager represents a secret management solution
type SecretManager struct {
	Name        string
	Description string
	Example     string
}

// SensitiveEnvVar represents a detected sensitive environment variable
type SensitiveEnvVar struct {
	Name          string
	Value         string
	Pattern       string
	Redacted      string
	SuggestedName string // Suggested secret name
}

// SecretExternalizationPlan represents a plan to externalize secrets
type SecretExternalizationPlan struct {
	DetectedSecrets  []SensitiveEnvVar
	PreferredManager string
	SecretReferences map[string]SecretReference
	ConfigMapEntries map[string]string
}

// SecretReference represents a reference to an external secret
type SecretReference struct {
	SecretName string
	SecretKey  string
	EnvVarName string
}

// NewSecretScanner creates a new secret scanner
func NewSecretScanner() *SecretScanner {
	return &SecretScanner{
		sensitivePatterns: []*regexp.Regexp{
			// Password patterns
			regexp.MustCompile(`(?i)^.*_?PASSWORD(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?PASSWD(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?PWD(_.*)?$`),

			// Token patterns
			regexp.MustCompile(`(?i)^.*_?TOKEN(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?API_?KEY(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?SECRET(_.*)?$`),

			// Authentication patterns
			regexp.MustCompile(`(?i)^.*_?AUTH(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?CREDENTIAL(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?ACCESS_?KEY(_.*)?$`),

			// Database patterns
			regexp.MustCompile(`(?i)^DB_.*$`),
			regexp.MustCompile(`(?i)^DATABASE_.*$`),
			regexp.MustCompile(`(?i)^.*_?CONNECTION_?STRING(_.*)?$`),

			// Certificate patterns
			regexp.MustCompile(`(?i)^.*_?CERT(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?CERTIFICATE(_.*)?$`),
			regexp.MustCompile(`(?i)^.*_?PRIVATE_?KEY(_.*)?$`),

			// Cloud provider patterns
			regexp.MustCompile(`(?i)^AWS_.*$`),
			regexp.MustCompile(`(?i)^AZURE_.*$`),
			regexp.MustCompile(`(?i)^GCP_.*$`),
			regexp.MustCompile(`(?i)^GOOGLE_.*$`),
		},
		secretManagers: []SecretManager{
			{
				Name:        "kubernetes-secrets",
				Description: "Native Kubernetes Secrets (base64 encoded)",
				Example:     "kubectl create secret generic app-secrets --from-literal=DB_PASSWORD=xxx",
			},
			{
				Name:        "sealed-secrets",
				Description: "Bitnami Sealed Secrets (encrypted secrets that can be stored in Git)",
				Example:     "kubeseal --format=yaml < secret.yaml > sealed-secret.yaml",
			},
			{
				Name:        types.ExternalSecretsLabel,
				Description: "External Secrets Operator (sync secrets from external systems)",
				Example:     "Syncs from AWS Secrets Manager, HashiCorp Vault, Azure Key Vault, etc.",
			},
			{
				Name:        "vault",
				Description: "HashiCorp Vault with Kubernetes auth",
				Example:     "vault kv put secret/app/config password=xxx",
			},
		},
	}
}

// ScanEnvironment scans environment variables for sensitive data
func (ss *SecretScanner) ScanEnvironment(envVars map[string]string) []SensitiveEnvVar {
	var sensitiveVars []SensitiveEnvVar

	for name, value := range envVars {
		for _, pattern := range ss.sensitivePatterns {
			if pattern.MatchString(name) {
				sensitiveVars = append(sensitiveVars, SensitiveEnvVar{
					Name:          name,
					Value:         value,
					Pattern:       pattern.String(),
					Redacted:      ss.redactValue(value),
					SuggestedName: ss.suggestSecretName(name),
				})
				break // Only match once per variable
			}
		}
	}

	return sensitiveVars
}

// ScanContent scans text content for sensitive patterns
func (ss *SecretScanner) ScanContent(content string) []SensitiveEnvVar {
	var sensitiveVars []SensitiveEnvVar

	// Simple pattern matching for key=value or key: value patterns
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Look for key=value or key: value patterns
		var key, value string

		// Environment variable style (KEY=value)
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key = strings.TrimSpace(parts[0])
				value = strings.TrimSpace(parts[1])
			}
		}

		// YAML/JSON style (key: value)
		if strings.Contains(line, ":") && !strings.Contains(line, "=") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key = strings.TrimSpace(parts[0])
				value = strings.TrimSpace(parts[1])

				// Remove quotes from YAML/JSON values
				value = strings.Trim(value, `"'`)
			}
		}

		if key != "" && value != "" {
			// Check if key matches sensitive patterns
			for _, pattern := range ss.sensitivePatterns {
				if pattern.MatchString(key) {
					sensitiveVars = append(sensitiveVars, SensitiveEnvVar{
						Name:          key,
						Value:         value,
						Pattern:       pattern.String(),
						Redacted:      ss.redactValue(value),
						SuggestedName: ss.suggestSecretName(key),
					})
					break // Only match once per key
				}
			}
		}
	}

	return sensitiveVars
}

// CreateExternalizationPlan creates a plan to externalize secrets
func (ss *SecretScanner) CreateExternalizationPlan(envVars map[string]string, preferredManager string) *SecretExternalizationPlan {
	plan := &SecretExternalizationPlan{
		DetectedSecrets:  ss.ScanEnvironment(envVars),
		PreferredManager: preferredManager,
		SecretReferences: make(map[string]SecretReference),
		ConfigMapEntries: make(map[string]string),
	}

	// Separate secrets from non-secrets
	for name, value := range envVars {
		isSecret := false
		for _, secret := range plan.DetectedSecrets {
			if secret.Name == name {
				isSecret = true
				// Create secret reference
				plan.SecretReferences[name] = SecretReference{
					SecretName: secret.SuggestedName,
					SecretKey:  strings.ToLower(name),
					EnvVarName: name,
				}
				break
			}
		}

		if !isSecret {
			// Non-sensitive values go to ConfigMap
			plan.ConfigMapEntries[name] = value
		}
	}

	return plan
}

// GetSecretManagers returns available secret management solutions
func (ss *SecretScanner) GetSecretManagers() []SecretManager {
	return ss.secretManagers
}

// GetRecommendedManager returns the recommended secret manager based on context
func (ss *SecretScanner) GetRecommendedManager(hasGitOps bool, cloudProvider string) string {
	if hasGitOps {
		return "sealed-secrets" // Safe for Git storage
	}

	switch cloudProvider {
	case "aws":
		return types.ExternalSecretsLabel // Can sync from AWS Secrets Manager
	case "azure":
		return types.ExternalSecretsLabel // Can sync from Azure Key Vault
	case "gcp":
		return types.ExternalSecretsLabel // Can sync from GCP Secret Manager
	default:
		return "kubernetes-secrets" // Default to native secrets
	}
}

// GenerateSecretManifest generates a Kubernetes Secret manifest
func (ss *SecretScanner) GenerateSecretManifest(secretName string, secrets map[string]string, namespace string) string {
	var sb strings.Builder

	sb.WriteString("apiVersion: v1\n")
	sb.WriteString("kind: Secret\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", secretName))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	sb.WriteString("type: Opaque\n")
	sb.WriteString("stringData:\n")

	for key := range secrets {
		// Generate deterministic dummy value for testing consistency
		dummyValue := ss.generateDummySecretValue(key)
		sb.WriteString(fmt.Sprintf("  %s: %s\n", strings.ToLower(key), dummyValue))
	}

	return sb.String()
}

// GenerateExternalSecretManifest generates an External Secrets manifest
func (ss *SecretScanner) GenerateExternalSecretManifest(secretName, namespace, secretStore string, mappings map[string]string) string {
	var sb strings.Builder

	sb.WriteString("apiVersion: external-secrets.io/v1beta1\n")
	sb.WriteString("kind: ExternalSecret\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", secretName))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	sb.WriteString("spec:\n")
	sb.WriteString("  secretStoreRef:\n")
	sb.WriteString(fmt.Sprintf("    name: %s\n", secretStore))
	sb.WriteString("    kind: SecretStore\n")
	sb.WriteString("  target:\n")
	sb.WriteString(fmt.Sprintf("    name: %s\n", secretName))
	sb.WriteString("  data:\n")

	for k8sKey, externalKey := range mappings {
		sb.WriteString(fmt.Sprintf("  - secretKey: %s\n", k8sKey))
		sb.WriteString("    remoteRef:\n")
		sb.WriteString(fmt.Sprintf("      key: %s\n", externalKey))
	}

	return sb.String()
}

// Helper methods

func (ss *SecretScanner) redactValue(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}

func (ss *SecretScanner) suggestSecretName(envVarName string) string {
	// Convert to lowercase and replace underscores
	name := strings.ToLower(envVarName)
	name = strings.ReplaceAll(name, "_", "-")

	// Remove common suffixes
	suffixes := []string{"-password", "-token", "-key", "-secret", "-auth"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	// Add app prefix and secrets suffix
	if !strings.Contains(name, "secret") {
		name = "app-" + name + "-secrets"
	}

	return name
}

// generateDummySecretValue creates deterministic dummy values for testing
func (ss *SecretScanner) generateDummySecretValue(key string) string {
	// Create deterministic dummy values based on key type
	lowerKey := strings.ToLower(key)

	// Return type-specific dummy values for predictable testing
	switch {
	case strings.Contains(lowerKey, "password"):
		return "dummy-password-123"
	case strings.Contains(lowerKey, "token"):
		return "dummy-token-456"
	case strings.Contains(lowerKey, "key"):
		return "dummy-key-789"
	case strings.Contains(lowerKey, "secret"):
		return "dummy-secret-abc"
	case strings.Contains(lowerKey, "cert"):
		return "dummy-certificate-def"
	case strings.Contains(lowerKey, "connection") || strings.Contains(lowerKey, "url"):
		return "dummy://user:pass@host:5432/db"
	default:
		return "dummy-value-xyz"
	}
}
