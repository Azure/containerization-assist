package scan

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"log/slog"
)

// RemediationGenerator handles generation of remediation plans and Kubernetes secrets
type RemediationGenerator struct {
	logger *slog.Logger
}

// NewRemediationGenerator creates a new remediation generator
func NewRemediationGenerator(logger *slog.Logger) *RemediationGenerator {
	return &RemediationGenerator{
		logger: logger,
	}
}

// GenerateRemediationPlan creates a comprehensive remediation plan for found secrets
func (rg *RemediationGenerator) GenerateRemediationPlan(secrets []ScannedSecret) *SecretRemediationPlan {
	plan := &SecretRemediationPlan{
		ConfigMapEntries: make(map[string]string),
		PreferredManager: "kubernetes-secrets",
	}

	plan.ImmediateActions = []string{
		"Stop committing files with detected secrets",
		"Remove secrets from version control history if already committed",
		"Rotate any exposed credentials",
		"Review and update .gitignore to prevent future commits",
	}

	plan.MigrationSteps = []string{
		"Create Kubernetes Secret manifests for sensitive data",
		"Update Deployment manifests to reference secrets via secretKeyRef",
		"Test the application with externalized secrets",
		"Remove hardcoded secrets from source files",
		"Implement proper secret rotation procedures",
	}

	secretMap := make(map[string][]ScannedSecret)
	for _, scannedSecret := range secrets {
		key := scannedSecret.Type
		secretMap[key] = append(secretMap[key], scannedSecret)
	}

	for secretType, typeSecrets := range secretMap {
		secretName := fmt.Sprintf("app-%s-secrets", secretType)

		for i := range typeSecrets {
			keyName := fmt.Sprintf("%s-%d", secretType, i+1)

			ref := SecretReference{
				SecretName:     secretName,
				SecretKey:      keyName,
				OriginalEnvVar: fmt.Sprintf("%s_VAR", strings.ToUpper(keyName)),
				KubernetesRef:  fmt.Sprintf("secretKeyRef: {name: %s, key: %s}", secretName, keyName),
			}

			plan.SecretReferences = append(plan.SecretReferences, ref)
		}
	}

	return plan
}

// GenerateKubernetesSecrets creates Kubernetes Secret manifests for found secrets
func (rg *RemediationGenerator) GenerateKubernetesSecrets(secrets []ScannedSecret, sessionID string) ([]GeneratedSecretManifest, error) {
	rg.logger.Info("Generating Kubernetes Secret manifests",
		"secret_count", len(secrets),
		"session_id", sessionID)

	if len(secrets) == 0 {
		rg.logger.Info("No secrets found, skipping manifest generation")
		return []GeneratedSecretManifest{}, nil
	}

	var manifests []GeneratedSecretManifest

	secretsByType := make(map[string][]ScannedSecret)
	for _, secret := range secrets {
		secretType := rg.normalizeSecretType(secret.Type)
		secretsByType[secretType] = append(secretsByType[secretType], secret)
	}

	for secretType, typeSecrets := range secretsByType {
		secretName := rg.generateSecretName(secretType)

		secretData := make(map[string]string)
		var keys []string

		for i, secret := range typeSecrets {
			key := rg.generateSecretKey(secret, i)
			keys = append(keys, key)
			placeholderValue := rg.generatePlaceholderValue(secret)
			secretData[key] = placeholderValue
		}

		manifest := GeneratedSecretManifest{
			Name:     secretName,
			Content:  rg.generateSecretYAML(secretName, secretData, typeSecrets),
			FilePath: filepath.Join("k8s", fmt.Sprintf("%s.yaml", secretName)),
			Keys:     keys,
		}

		manifests = append(manifests, manifest)

		rg.logger.Info("Generated Kubernetes Secret manifest",
			"secret_name", secretName,
			"secret_type", secretType,
			"key_count", len(keys))
	}

	return manifests, nil
}

// generateSecretYAML creates a YAML manifest for a Kubernetes Secret
func (rg *RemediationGenerator) generateSecretYAML(name string, secretData map[string]string, detectedSecrets []ScannedSecret) string {
	yamlContent := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  labels:
    app: %s
    generated-by: container-kit
    secret-type: %s
  annotations:
    description: "Generated from detected secrets in source code"
    secrets-detected: "%d"
    generation-time: "%s"
type: Opaque
data:
`, name, rg.extractAppName(name), rg.extractSecretType(name), len(detectedSecrets), time.Now().UTC().Format(time.RFC3339))

	for key, value := range secretData {
		yamlContent += fmt.Sprintf("  %s: %s\n", key, value)
	}

	yamlContent += `
# Instructions:
# 1. Replace the placeholder values above with your actual base64-encoded secrets
# 2. Use 'echo -n "your-secret-value" | base64' to encode values
# 3. Apply this secret to your cluster: kubectl apply -f this-file.yaml
# 4. Reference in your deployment using:
#    env:
#    - name: SECRET_NAME
#      valueFrom:
#        secretKeyRef:
#          name: ` + name + `
#          key: <key-name>
#
# Detected secrets that should be stored here:
`

	for i, secret := range detectedSecrets {
		yamlContent += fmt.Sprintf("# %d. Found in %s:%d - Type: %s (Severity: %s)\n",
			i+1, secret.File, secret.Line, secret.Type, secret.Severity)
	}

	return yamlContent
}

// normalizeSecretType normalizes secret types for consistent naming
func (rg *RemediationGenerator) normalizeSecretType(secretType string) string {
	switch strings.ToLower(secretType) {
	case "api_key", "apikey", "api-key":
		return "api-keys"
	case "password", "pwd":
		return "passwords"
	case "token", "access_token", "auth_token":
		return "tokens"
	case "database_credential", "db_credential":
		return "database"
	case "certificate", "cert":
		return "certificates"
	case "private_key", "privatekey":
		return "private-keys"
	default:
		return "secrets"
	}
}

// generateSecretName creates a consistent name for Kubernetes secrets
func (rg *RemediationGenerator) generateSecretName(secretType string) string {
	normalizedType := strings.ReplaceAll(secretType, "_", "-")
	return fmt.Sprintf("app-%s", normalizedType)
}

// generateSecretKey creates a key name for a secret within a Kubernetes Secret
func (rg *RemediationGenerator) generateSecretKey(secret ScannedSecret, index int) string {
	secretType := strings.ToLower(secret.Type)
	secretType = strings.ReplaceAll(secretType, "_", "-")

	// Extract meaningful context from the pattern if available
	pattern := strings.ToLower(secret.Pattern)

	// Try to extract service or component name from pattern
	if strings.Contains(pattern, "database") || strings.Contains(pattern, "db") {
		return fmt.Sprintf("database-%s-%d", secretType, index+1)
	}
	if strings.Contains(pattern, "redis") {
		return fmt.Sprintf("redis-%s-%d", secretType, index+1)
	}
	if strings.Contains(pattern, "api") {
		return fmt.Sprintf("api-%s-%d", secretType, index+1)
	}
	if strings.Contains(pattern, "jwt") {
		return fmt.Sprintf("jwt-%s-%d", secretType, index+1)
	}
	if strings.Contains(pattern, "auth") {
		return fmt.Sprintf("auth-%s-%d", secretType, index+1)
	}

	// Default format
	return fmt.Sprintf("%s-%d", secretType, index+1)
}

// generatePlaceholderValue creates a base64-encoded placeholder for secrets
func (rg *RemediationGenerator) generatePlaceholderValue(secret ScannedSecret) string {
	var placeholder string

	switch strings.ToLower(secret.Type) {
	case "api_key", "apikey":
		placeholder = "your-api-key-here"
	case "password":
		placeholder = "your-secure-password"
	case "token", "access_token":
		placeholder = "your-token-value"
	case "database_credential":
		if strings.Contains(strings.ToLower(secret.Pattern), "password") {
			placeholder = "your-database-password"
		} else {
			placeholder = "your-database-credential"
		}
	case "private_key":
		placeholder = "-----BEGIN PRIVATE KEY-----\nyour-private-key-content\n-----END PRIVATE KEY-----"
	case "certificate":
		placeholder = "-----BEGIN CERTIFICATE-----\nyour-certificate-content\n-----END CERTIFICATE-----"
	default:
		placeholder = fmt.Sprintf("your-%s-value", strings.ReplaceAll(secret.Type, "_", "-"))
	}

	// Add context about the original location
	placeholder += fmt.Sprintf(" (originally from %s)", filepath.Base(secret.File))

	return base64.StdEncoding.EncodeToString([]byte(placeholder))
}

// extractAppName extracts application name from secret name
func (rg *RemediationGenerator) extractAppName(secretName string) string {
	if strings.HasPrefix(secretName, "app-") {
		parts := strings.Split(secretName, "-")
		if len(parts) > 1 {
			return "container-kit-app"
		}
	}
	return "app"
}

// extractSecretType extracts secret type from secret name
func (rg *RemediationGenerator) extractSecretType(secretName string) string {
	if strings.HasPrefix(secretName, "app-") {
		parts := strings.Split(secretName, "-")
		if len(parts) > 1 {
			return strings.Join(parts[1:], "-")
		}
	}
	return "general"
}
