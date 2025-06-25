package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corek8s "github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
)

// convertDetectedSecrets converts utils.SensitiveEnvVar to DetectedSecret
func (t *AtomicGenerateManifestsTool) convertDetectedSecrets(secrets []utils.SensitiveEnvVar) []DetectedSecret {
	var detected []DetectedSecret
	for _, s := range secrets {
		detected = append(detected, DetectedSecret{
			Name:          s.Name,
			RedactedValue: s.Redacted,
			SuggestedRef:  s.SuggestedName,
			Pattern:       s.Pattern,
		})
	}
	return detected
}

// createSecretsPlan creates a plan for handling detected secrets
func (t *AtomicGenerateManifestsTool) createSecretsPlan(args AtomicGenerateManifestsArgs, detectedSecrets []utils.SensitiveEnvVar) *SecretsPlan {
	plan := &SecretsPlan{
		SecretReferences: make(map[string]SecretRef),
		ConfigMapEntries: make(map[string]string),
		Instructions:     []string{},
	}

	// Determine secret manager
	if args.SecretManager != "" {
		plan.SecretManager = args.SecretManager
	} else {
		plan.SecretManager = t.secretScanner.GetRecommendedManager(args.GitOpsReady, "")
	}

	// Determine strategy
	switch args.SecretHandling {
	case "inline":
		plan.Strategy = "inline"
		plan.Instructions = append(plan.Instructions,
			"⚠️ Secrets are included inline in manifests (NOT RECOMMENDED for production)",
			"Consider using 'auto' or 'prompt' mode for better security")
	case "prompt":
		plan.Strategy = "prompt"
		plan.Instructions = append(plan.Instructions,
			"Please create the following secrets before deployment:")
	case types.ResourceModeAuto, "":
		plan.Strategy = types.ResourceModeAuto
		plan.Instructions = append(plan.Instructions,
			fmt.Sprintf("Secrets will be externalized using %s", plan.SecretManager))
	}

	// Create references for each detected secret
	secretGroups := make(map[string][]string)
	for _, secret := range detectedSecrets {
		secretName := secret.SuggestedName
		plan.SecretReferences[secret.Name] = SecretRef{
			Name: secretName,
			Key:  strings.ToLower(secret.Name),
			Env:  secret.Name,
		}
		secretGroups[secretName] = append(secretGroups[secretName], secret.Name)
	}

	// Add instructions for each secret group
	for secretName, vars := range secretGroups {
		switch plan.SecretManager {
		case "kubernetes-secrets":
			plan.Instructions = append(plan.Instructions,
				fmt.Sprintf("kubectl create secret generic %s --from-literal=%s=<value> -n %s",
					secretName, strings.Join(vars, "=<value> --from-literal="), args.Namespace))
		case "sealed-secrets":
			plan.Instructions = append(plan.Instructions,
				fmt.Sprintf("Create sealed secret '%s' with keys: %s", secretName, strings.Join(vars, ", ")))
		case "external-secrets":
			plan.Instructions = append(plan.Instructions,
				fmt.Sprintf("Configure external secret '%s' to sync keys: %s", secretName, strings.Join(vars, ", ")))
		}
	}

	// Non-sensitive vars go to ConfigMap
	for name, value := range args.Environment {
		isSecret := false
		for _, secret := range detectedSecrets {
			if secret.Name == name {
				isSecret = true
				break
			}
		}
		if !isSecret {
			plan.ConfigMapEntries[name] = value
		}
	}

	return plan
}

// applySecretsPlan applies the secrets plan to the arguments
func (t *AtomicGenerateManifestsTool) applySecretsPlan(args AtomicGenerateManifestsArgs, plan *SecretsPlan) AtomicGenerateManifestsArgs {
	// Remove sensitive values from environment
	newEnv := make(map[string]string)
	for name, value := range args.Environment {
		if _, isSecret := plan.SecretReferences[name]; !isSecret {
			newEnv[name] = value
		}
	}
	args.Environment = newEnv
	return args
}

// generateSecretManifests creates secret manifest files
func (t *AtomicGenerateManifestsTool) generateSecretManifests(sessionID string, result *AtomicGenerateManifestsResult) []GeneratedManifest {
	var manifests []GeneratedManifest

	// Group secrets by secret name
	secretGroups := make(map[string]map[string]string)
	for _, ref := range result.SecretsPlan.SecretReferences {
		if secretGroups[ref.Name] == nil {
			secretGroups[ref.Name] = make(map[string]string)
		}
		secretGroups[ref.Name][ref.Key] = t.generateDummySecretValue(ref.Key)
	}

	// Ensure output directory exists
	outputDir := filepath.Join(t.pipelineAdapter.GetSessionWorkspace(sessionID), "manifests", "secrets")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.logger.Error().Err(err).Msg("Failed to create secrets output directory")
		return manifests
	}

	// Generate manifest for each secret group
	for secretName, secrets := range secretGroups {
		var kind string

		switch result.SecretsPlan.SecretManager {
		case "sealed-secrets":
			// For sealed secrets, generate a template
			kind = "SealedSecret"
			templateContent := t.generateSealedSecretTemplate(secretName, result.Namespace, secrets)
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-sealed.yaml", secretName))
			if err := os.WriteFile(outputPath, []byte(templateContent), 0o644); err != nil {
				t.logger.Error().Err(err).Str("path", outputPath).Msg("Failed to save sealed secret template")
			}
		case "external-secrets":
			kind = "ExternalSecret"
			externalSecretContent := t.secretScanner.GenerateExternalSecretManifest(
				secretName, result.Namespace, "default-secret-store", secrets)
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-external.yaml", secretName))
			if err := os.WriteFile(outputPath, []byte(externalSecretContent), 0o644); err != nil {
				t.logger.Error().Err(err).Str("path", outputPath).Msg("Failed to save external secret")
			}
		default:
			// Use our new SecretGenerator for Kubernetes secrets
			kind = "Secret"

			// Convert map[string]string to map[string]string for StringData
			secretOptions := corek8s.SecretOptions{
				Name:       secretName,
				Namespace:  result.Namespace,
				Type:       corek8s.SecretTypeOpaque,
				StringData: secrets,
				Labels: map[string]string{
					"app":                            result.AppName,
					"kubernetes.azure.com/generator": "container-kit-mcp",
				},
			}

			// Generate the secret using our SecretGenerator
			ctx := context.Background()
			secretResult, err := t.secretGenerator.GenerateSecret(ctx, secretOptions)
			if err != nil {
				t.logger.Error().Err(err).Str("secret_name", secretName).Msg("Failed to generate secret")
				continue
			}

			if !secretResult.Success {
				t.logger.Error().
					Str("secret_name", secretName).
					Str("error_type", secretResult.Error.Type).
					Str("error_message", secretResult.Error.Message).
					Msg("Secret generation failed")
				continue
			}

			// Save secret to file
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", secretName))
			if err := t.secretGenerator.SaveSecretToFile(secretResult.Secret, outputPath); err != nil {
				t.logger.Error().Err(err).Str("path", outputPath).Msg("Failed to save secret to file")
				// Still use the generated content even if saving failed
			} else {
				t.logger.Info().Str("path", outputPath).Msg("Secret saved to file")
			}

			// We don't need manifestContent for kubernetes secrets since we save directly to file
		}

		relativePath := filepath.Join("secrets", fmt.Sprintf("%s.yaml", secretName))
		manifests = append(manifests, GeneratedManifest{
			Name:    secretName,
			Kind:    kind,
			Path:    relativePath,
			Purpose: fmt.Sprintf("Externalized secrets for %s", secretName),
		})

		t.logger.Info().
			Str("secret_name", secretName).
			Str("kind", kind).
			Str("path", relativePath).
			Msg("Generated secret manifest")
	}

	return manifests
}

// generateSealedSecretTemplate creates a template for Sealed Secrets
func (t *AtomicGenerateManifestsTool) generateSealedSecretTemplate(name, namespace string, secrets map[string]string) string {
	var sb strings.Builder

	sb.WriteString("# This is a template for a Sealed Secret\n")
	sb.WriteString("# Generate the actual sealed secret with:\n")
	sb.WriteString(fmt.Sprintf("# kubectl create secret generic %s \\\n", name))
	for key := range secrets {
		sb.WriteString(fmt.Sprintf("#   --from-literal=%s=<value> \\\n", key))
	}
	sb.WriteString(fmt.Sprintf("#   -n %s --dry-run=client -o yaml | \\\n", namespace))
	sb.WriteString("#   kubeseal --format=yaml > sealed-secret.yaml\n\n")

	sb.WriteString("apiVersion: bitnami.com/v1alpha1\n")
	sb.WriteString("kind: SealedSecret\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", name))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	sb.WriteString("spec:\n")
	sb.WriteString("  encryptedData:\n")
	for key := range secrets {
		sb.WriteString(fmt.Sprintf("    %s: <ENCRYPTED_VALUE>\n", key))
	}

	return sb.String()
}

// generateDummySecretValue creates deterministic dummy values for testing
func (t *AtomicGenerateManifestsTool) generateDummySecretValue(key string) string {
	// Create deterministic dummy values based on key type for consistent testing
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
