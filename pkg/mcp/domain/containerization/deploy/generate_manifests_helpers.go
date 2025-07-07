package deploy

import (
	"encoding/base64"
	"os"
	"path/filepath"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/templates"
	"gopkg.in/yaml.v3"
)

// writeIngressTemplate writes the ingress template to the workspace
func (t *GenerateManifestsTool) writeIngressTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/ingress.yaml")
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "ingress.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	return nil
}

func (t *GenerateManifestsTool) writeNetworkPolicyTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/networkpolicy.yaml")
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "networkpolicy.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	return nil
}

func (t *GenerateManifestsTool) addBinaryDataToConfigMap(configMapPath string, binaryData map[string][]byte) error {
	content, err := os.ReadFile(configMapPath)
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	if len(binaryData) > 0 {
		binaryDataMap := make(map[string]interface{})
		for key, data := range binaryData {
			// Kubernetes expects base64 encoded binary data
			binaryDataMap[key] = base64.StdEncoding.EncodeToString(data)
		}
		configMap["binaryData"] = binaryDataMap
	}

	// Write back the updated manifest
	updatedContent, err := yaml.Marshal(configMap)
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	if err := os.WriteFile(configMapPath, updatedContent, 0644); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	return nil
}

func (t *GenerateManifestsTool) generateRegistrySecret(secretPath string, args GenerateManifestsArgs) error {
	// RegistrySecrets field doesn't exist in GenerateManifestsArgs, so we skip registry secret generation
	// This could be added later if needed
	t.logger.Debug("Registry secrets not supported in current GenerateManifestsArgs structure")
	return nil

	// Note: The following code is commented out until RegistrySecrets field is added to GenerateManifestsArgs
}

// addPullSecretToDeployment adds imagePullSecrets to deployment spec
func (t *GenerateManifestsTool) addPullSecretToDeployment(deploymentPath, secretName string) error {
	// Read deployment file
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	var deployment map[string]interface{}
	if err := yaml.Unmarshal(content, &deployment); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	spec, ok := deployment["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	imagePullSecrets := []map[string]interface{}{
		{"name": secretName},
	}

	// Check if imagePullSecrets already exists
	if existing, ok := templateSpec["imagePullSecrets"].([]interface{}); ok {
		for _, secret := range existing {
			if secretMap, ok := secret.(map[string]interface{}); ok {
				imagePullSecrets = append(imagePullSecrets, secretMap)
			}
		}
	}

	templateSpec["imagePullSecrets"] = imagePullSecrets

	// Write back to file
	updatedContent, err := yaml.Marshal(deployment)
	if err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	if err := os.WriteFile(deploymentPath, updatedContent, 0644); err != nil {
		return errors.NewError().Messagef("error").WithLocation().Build()
	}

	return nil
}
