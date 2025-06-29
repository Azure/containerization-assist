package deploy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/templates"
	"gopkg.in/yaml.v3"
)

// writeIngressTemplate writes the ingress template to the workspace
func (t *GenerateManifestsTool) writeIngressTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/ingress.yaml")
	if err != nil {
		return mcp.NewRichError("INGRESS_TEMPLATE_READ_FAILED", fmt.Sprintf("reading embedded ingress template: %v", err), types.ErrTypeSystem)
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "ingress.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return mcp.NewRichError("INGRESS_TEMPLATE_WRITE_FAILED", fmt.Sprintf("writing ingress template: %v", err), types.ErrTypeSystem)
	}

	return nil
}

// writeNetworkPolicyTemplate writes the networkpolicy template to the workspace
func (t *GenerateManifestsTool) writeNetworkPolicyTemplate(workspaceDir string) error {
	// Import the templates package to access embedded files
	data, err := templates.Templates.ReadFile("manifests/manifest-basic/networkpolicy.yaml")
	if err != nil {
		return mcp.NewRichError("NETWORKPOLICY_TEMPLATE_READ_FAILED", fmt.Sprintf("reading embedded networkpolicy template: %v", err), types.ErrTypeSystem)
	}

	manifestPath := filepath.Join(workspaceDir, "manifests")
	destPath := filepath.Join(manifestPath, "networkpolicy.yaml")

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return mcp.NewRichError("NETWORKPOLICY_TEMPLATE_WRITE_FAILED", fmt.Sprintf("writing networkpolicy template: %v", err), types.ErrTypeSystem)
	}

	return nil
}

// addBinaryDataToConfigMap adds binary data to an existing ConfigMap manifest
func (t *GenerateManifestsTool) addBinaryDataToConfigMap(configMapPath string, binaryData map[string][]byte) error {
	content, err := os.ReadFile(configMapPath)
	if err != nil {
		return mcp.NewRichError("CONFIGMAP_READ_FAILED", fmt.Sprintf("reading configmap manifest: %v", err), "filesystem_error")
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		return mcp.NewRichError("CONFIGMAP_PARSE_FAILED", fmt.Sprintf("parsing configmap YAML: %v", err), "validation_error")
	}

	// Add binaryData section
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
		return mcp.NewRichError("CONFIGMAP_MARSHAL_FAILED", fmt.Sprintf("marshaling updated configmap YAML: %v", err), "validation_error")
	}

	if err := os.WriteFile(configMapPath, updatedContent, 0644); err != nil {
		return mcp.NewRichError("CONFIGMAP_WRITE_FAILED", fmt.Sprintf("writing updated configmap manifest: %v", err), "filesystem_error")
	}

	return nil
}

// generateRegistrySecret generates Docker registry pull secrets
func (t *GenerateManifestsTool) generateRegistrySecret(secretPath string, args GenerateManifestsArgs) error {
	secrets := []map[string]interface{}{}

	for i, regSecret := range args.RegistrySecrets {
		appName := "app"
		if args.AppName != "" {
			appName = args.AppName
		}
		secretName := fmt.Sprintf("%s-regcred-%d", appName, i+1)

		// Create Docker config JSON
		dockerConfig := map[string]interface{}{
			"auths": map[string]interface{}{
				regSecret.Registry: map[string]interface{}{
					"username": regSecret.Username,
					"password": regSecret.Password,
					"email":    regSecret.Email,
				},
			},
		}

		dockerConfigJSON, err := json.Marshal(dockerConfig)
		if err != nil {
			return mcp.NewRichError("DOCKER_CONFIG_MARSHAL_FAILED", fmt.Sprintf("marshaling docker config: %v", err), "validation_error")
		}

		secret := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      secretName,
				"namespace": args.Namespace,
			},
			"type": "kubernetes.io/dockerconfigjson",
			"data": map[string]interface{}{
				".dockerconfigjson": base64.StdEncoding.EncodeToString(dockerConfigJSON),
			},
		}

		secrets = append(secrets, secret)
	}

	// Write secrets to file
	if len(secrets) > 0 {
		for i, secret := range secrets {
			data, err := yaml.Marshal(secret)
			if err != nil {
				return mcp.NewRichError("SECRET_MARSHAL_FAILED", fmt.Sprintf("marshaling secret: %v", err), "validation_error")
			}

			// For multiple secrets, create separate files
			filename := secretPath
			if i > 0 {
				dir := filepath.Dir(secretPath)
				filename = filepath.Join(dir, fmt.Sprintf("secret-regcred-%d.yaml", i+1))
			}

			if err := os.WriteFile(filename, data, 0644); err != nil {
				return mcp.NewRichError("SECRET_WRITE_FAILED", fmt.Sprintf("writing secret file: %v", err), "filesystem_error")
			}
		}
	}

	return nil
}

// addPullSecretToDeployment adds imagePullSecrets to deployment spec
func (t *GenerateManifestsTool) addPullSecretToDeployment(deploymentPath, secretName string) error {
	// Read deployment file
	content, err := os.ReadFile(deploymentPath)
	if err != nil {
		return mcp.NewRichError("DEPLOYMENT_READ_FAILED", fmt.Sprintf("reading deployment: %v", err), "filesystem_error")
	}

	// Parse YAML
	var deployment map[string]interface{}
	if err := yaml.Unmarshal(content, &deployment); err != nil {
		return mcp.NewRichError("DEPLOYMENT_PARSE_FAILED", fmt.Sprintf("parsing deployment YAML: %v", err), "validation_error")
	}

	// Navigate to spec.template.spec
	spec, ok := deployment["spec"].(map[string]interface{})
	if !ok {
		return mcp.NewRichError("DEPLOYMENT_SPEC_MISSING", "deployment missing spec field", "validation_error")
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return mcp.NewRichError("DEPLOYMENT_TEMPLATE_MISSING", "deployment spec missing template field", "validation_error")
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return mcp.NewRichError("DEPLOYMENT_TEMPLATE_SPEC_MISSING", "deployment template missing spec field", "validation_error")
	}

	// Add imagePullSecrets
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
		return mcp.NewRichError("DEPLOYMENT_MARSHAL_FAILED", fmt.Sprintf("marshaling updated deployment: %v", err), "validation_error")
	}

	if err := os.WriteFile(deploymentPath, updatedContent, 0644); err != nil {
		return mcp.NewRichError("DEPLOYMENT_WRITE_FAILED", fmt.Sprintf("writing updated deployment: %v", err), "filesystem_error")
	}

	return nil
}
