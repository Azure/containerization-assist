// Package azure provides core Azure Container Apps operations
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
)

// AzureContainerAppsManifestService provides Azure Container Apps manifest operations
type AzureContainerAppsManifestService interface {
	GenerateManifests(ctx context.Context, options AzureContainerAppsManifestOptions) (*AzureContainerAppsManifestResult, error)
	ValidateManifests(ctx context.Context, manifests []string, manifestType string) (*api.ManifestValidationResult, error)
	GetAvailableTemplates() ([]string, error)
}

// manifestService implements the AzureContainerAppsManifestService interface
type manifestService struct {
	logger *slog.Logger
}

// NewAzureContainerAppsManifestService creates a new Azure Container Apps manifest service
func NewAzureContainerAppsManifestService(logger *slog.Logger) AzureContainerAppsManifestService {
	return &manifestService{
		logger: logger,
	}
}

// GenerateManifests generates Azure Container Apps manifests from templates
func (s *manifestService) GenerateManifests(_ context.Context, options AzureContainerAppsManifestOptions) (*AzureContainerAppsManifestResult, error) {
	startTime := time.Now()

	result := &AzureContainerAppsManifestResult{
		Template:  options.Template,
		OutputDir: options.OutputDir,
		Context:   make(map[string]interface{}),
		Manifests: make([]GeneratedAzureManifest, 0),
	}

	// Validate inputs
	if err := s.validateGenerateInputs(options); err != nil {
		result.Error = &AzureManifestError{
			Type:    "validation_error",
			Message: err.Error(),
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Set defaults
	if options.MinReplicas == 0 {
		options.MinReplicas = 1
	}
	if options.MaxReplicas == 0 {
		options.MaxReplicas = 10
	}
	if options.Resources == nil {
		options.Resources = &ContainerResources{
			CPU:    0.5,
			Memory: "1.0Gi",
		}
	}

	// Create output directory
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		result.Error = &AzureManifestError{
			Type:    "directory_error",
			Message: fmt.Sprintf("Failed to create output directory: %v", err),
			Path:    options.OutputDir,
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Generate manifests based on template type
	var manifests []GeneratedAzureManifest
	var err error

	switch strings.ToLower(options.Template) {
	case "bicep":
		manifests, err = s.generateBicepManifests(options)
	case "arm":
		manifests, err = s.generateARMManifests(options)
	default:
		err = fmt.Errorf("unsupported template type: %s (must be 'bicep' or 'arm')", options.Template)
	}

	if err != nil {
		result.Error = &AzureManifestError{
			Type:    "generation_error",
			Message: fmt.Sprintf("Failed to generate manifests: %v", err),
			Context: map[string]interface{}{
				"options": options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.Manifests = manifests
	result.Success = true
	result.Duration = time.Since(startTime)

	// Set manifest path to first manifest or output directory
	if len(manifests) > 0 {
		result.ManifestPath = manifests[0].Path
	} else {
		result.ManifestPath = options.OutputDir
	}

	return result, nil
}

// ValidateManifests validates Azure Container Apps manifests
func (s *manifestService) ValidateManifests(_ context.Context, manifests []string, manifestType string) (*api.ManifestValidationResult, error) {
	result := &api.ManifestValidationResult{
		ValidationResult: api.ValidationResult{
			Valid:    true,
			Errors:   make([]api.ValidationError, 0),
			Warnings: make([]api.ValidationWarning, 0),
			Metadata: make(map[string]interface{}),
		},
	}

	for _, manifest := range manifests {
		var err error
		switch strings.ToLower(manifestType) {
		case "bicep":
			err = s.validateBicepManifest(manifest)
		case "arm":
			err = s.validateARMManifest(manifest)
		default:
			err = fmt.Errorf("unknown manifest type: %s", manifestType)
		}

		if err != nil {
			result.Valid = false
			validationErr := api.ValidationError{
				Message: err.Error(),
				Field:   "manifest",
				Code:    "AZURE_MANIFEST_VALIDATION_ERROR",
			}
			result.Errors = append(result.Errors, validationErr)
		}
	}

	return result, nil
}

// GetAvailableTemplates returns available manifest templates
func (s *manifestService) GetAvailableTemplates() ([]string, error) {
	templates := []string{
		"bicep",
		"arm",
	}
	return templates, nil
}

// Helper methods

func (s *manifestService) validateGenerateInputs(options AzureContainerAppsManifestOptions) error {
	if options.AppName == "" {
		return fmt.Errorf("app name is required")
	}
	if options.ImageRef == "" {
		return fmt.Errorf("image reference is required")
	}
	if options.ResourceGroup == "" {
		return fmt.Errorf("resource group is required")
	}
	if options.Location == "" {
		return fmt.Errorf("location is required")
	}
	if options.EnvironmentName == "" {
		return fmt.Errorf("environment name is required")
	}
	if options.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	return nil
}

func (s *manifestService) generateBicepManifests(options AzureContainerAppsManifestOptions) ([]GeneratedAzureManifest, error) {
	manifests := make([]GeneratedAzureManifest, 0)

	// Generate main bicep file
	mainContent := s.generateBicepMainTemplate(options)
	mainPath := filepath.Join(options.OutputDir, "main.bicep")

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write main bicep manifest: %v", err)
	}

	manifests = append(manifests, GeneratedAzureManifest{
		Name:    "main.bicep",
		Type:    "bicep",
		Path:    mainPath,
		Content: mainContent,
		Size:    len(mainContent),
		Valid:   true,
	})

	// Generate parameters file
	paramsContent := s.generateBicepParametersFile(options)
	paramsPath := filepath.Join(options.OutputDir, "main.parameters.json")

	if err := os.WriteFile(paramsPath, []byte(paramsContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write parameters file: %v", err)
	}

	manifests = append(manifests, GeneratedAzureManifest{
		Name:    "main.parameters.json",
		Type:    "json",
		Path:    paramsPath,
		Content: paramsContent,
		Size:    len(paramsContent),
		Valid:   true,
	})

	return manifests, nil
}

func (s *manifestService) generateBicepMainTemplate(options AzureContainerAppsManifestOptions) string {
	var template strings.Builder

	// Parameters
	template.WriteString("// Azure Container Apps Bicep Template\n")
	template.WriteString("// Generated by Container Kit\n\n")

	template.WriteString("param location string = resourceGroup().location\n")
	template.WriteString(fmt.Sprintf("param appName string = '%s'\n", options.AppName))
	template.WriteString(fmt.Sprintf("param imageName string = '%s'\n", options.ImageRef))
	template.WriteString(fmt.Sprintf("param containerPort int = %d\n", options.Port))
	template.WriteString(fmt.Sprintf("param environmentName string = '%s'\n", options.EnvironmentName))

	if options.CustomDomain != "" {
		template.WriteString(fmt.Sprintf("param customDomain string = '%s'\n", options.CustomDomain))
	}

	template.WriteString("\n")

	// Environment resource (if needed)
	if options.IncludeEnvironment {
		template.WriteString("// Container Apps Environment\n")
		template.WriteString("resource environment 'Microsoft.App/managedEnvironments@2024-03-01' = {\n")
		template.WriteString("  name: environmentName\n")
		template.WriteString("  location: location\n")
		template.WriteString("  properties: {\n")
		template.WriteString("    appLogsConfiguration: {\n")
		template.WriteString("      destination: 'azure-monitor'\n")
		template.WriteString("    }\n")
		template.WriteString("  }\n")
		template.WriteString("}\n\n")
	}

	// Container App resource
	template.WriteString("// Container App\n")
	template.WriteString("resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {\n")
	template.WriteString("  name: appName\n")
	template.WriteString("  location: location\n")

	if options.ManagedIdentity {
		template.WriteString("  identity: {\n")
		template.WriteString("    type: 'SystemAssigned'\n")
		template.WriteString("  }\n")
	}

	template.WriteString("  properties: {\n")

	// Reference environment
	if options.IncludeEnvironment {
		template.WriteString("    managedEnvironmentId: environment.id\n")
	} else {
		template.WriteString("    managedEnvironmentId: resourceId('Microsoft.App/managedEnvironments', environmentName)\n")
	}

	// Configuration section
	template.WriteString("    configuration: {\n")

	// Ingress configuration
	if options.IncludeIngress {
		template.WriteString("      ingress: {\n")
		template.WriteString("        external: true\n")
		template.WriteString(fmt.Sprintf("        targetPort: %d\n", options.Port))
		template.WriteString("        transport: 'auto'\n")

		if options.CustomDomain != "" {
			template.WriteString("        customDomains: [\n")
			template.WriteString("          {\n")
			template.WriteString(fmt.Sprintf("            name: '%s'\n", options.CustomDomain))
			template.WriteString("            bindingType: 'SniEnabled'\n")
			template.WriteString("          }\n")
			template.WriteString("        ]\n")
		}

		template.WriteString("        traffic: [\n")
		template.WriteString("          {\n")
		template.WriteString("            latestRevision: true\n")
		template.WriteString("            weight: 100\n")
		template.WriteString("          }\n")
		template.WriteString("        ]\n")
		template.WriteString("      }\n")
	}

	// Dapr configuration
	if options.EnableDapr {
		template.WriteString("      dapr: {\n")
		template.WriteString("        enabled: true\n")
		if options.DaprAppId != "" {
			template.WriteString(fmt.Sprintf("        appId: '%s'\n", options.DaprAppId))
		}
		if options.DaprAppPort > 0 {
			template.WriteString(fmt.Sprintf("        appPort: %d\n", options.DaprAppPort))
		}
		template.WriteString("        appProtocol: 'http'\n")
		template.WriteString("      }\n")
	}

	template.WriteString("    }\n")

	// Template section
	template.WriteString("    template: {\n")

	// Containers
	template.WriteString("      containers: [\n")
	template.WriteString("        {\n")
	template.WriteString("          name: appName\n")
	template.WriteString("          image: imageName\n")

	// Resources
	template.WriteString("          resources: {\n")
	template.WriteString(fmt.Sprintf("            cpu: %g\n", options.Resources.CPU))
	template.WriteString(fmt.Sprintf("            memory: '%s'\n", options.Resources.Memory))
	template.WriteString("          }\n")

	// Environment variables
	if len(options.EnvironmentVariables) > 0 {
		template.WriteString("          env: [\n")
		for key, value := range options.EnvironmentVariables {
			template.WriteString("            {\n")
			template.WriteString(fmt.Sprintf("              name: '%s'\n", key))
			template.WriteString(fmt.Sprintf("              value: '%s'\n", value))
			template.WriteString("            }\n")
		}
		template.WriteString("          ]\n")
	}

	template.WriteString("        }\n")
	template.WriteString("      ]\n")

	// Scale configuration
	template.WriteString("      scale: {\n")
	template.WriteString(fmt.Sprintf("        minReplicas: %d\n", options.MinReplicas))
	template.WriteString(fmt.Sprintf("        maxReplicas: %d\n", options.MaxReplicas))
	template.WriteString("      }\n")

	template.WriteString("    }\n")
	template.WriteString("  }\n")

	// Tags
	if len(options.Labels) > 0 {
		template.WriteString("  tags: {\n")
		for key, value := range options.Labels {
			template.WriteString(fmt.Sprintf("    '%s': '%s'\n", key, value))
		}
		template.WriteString("  }\n")
	}

	template.WriteString("}\n\n")

	// Outputs
	template.WriteString("// Outputs\n")
	template.WriteString("output fqdn string = containerApp.properties.configuration.ingress.fqdn\n")
	template.WriteString("output appName string = containerApp.name\n")

	return template.String()
}

func (s *manifestService) generateBicepParametersFile(options AzureContainerAppsManifestOptions) string {
	params := map[string]interface{}{
		"$schema":        "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#",
		"contentVersion": "1.0.0.0",
		"parameters": map[string]interface{}{
			"appName": map[string]interface{}{
				"value": options.AppName,
			},
			"imageName": map[string]interface{}{
				"value": options.ImageRef,
			},
			"containerPort": map[string]interface{}{
				"value": options.Port,
			},
			"environmentName": map[string]interface{}{
				"value": options.EnvironmentName,
			},
		},
	}

	if options.CustomDomain != "" {
		params["parameters"].(map[string]interface{})["customDomain"] = map[string]interface{}{
			"value": options.CustomDomain,
		}
	}

	jsonBytes, _ := json.MarshalIndent(params, "", "  ")
	return string(jsonBytes)
}

func (s *manifestService) generateARMManifests(options AzureContainerAppsManifestOptions) ([]GeneratedAzureManifest, error) {
	manifests := make([]GeneratedAzureManifest, 0)

	// Generate ARM template
	armContent := s.generateARMTemplate(options)
	armPath := filepath.Join(options.OutputDir, "azuredeploy.json")

	if err := os.WriteFile(armPath, []byte(armContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write ARM template: %v", err)
	}

	manifests = append(manifests, GeneratedAzureManifest{
		Name:    "azuredeploy.json",
		Type:    "arm",
		Path:    armPath,
		Content: armContent,
		Size:    len(armContent),
		Valid:   true,
	})

	// Generate parameters file
	paramsContent := s.generateARMParametersFile(options)
	paramsPath := filepath.Join(options.OutputDir, "azuredeploy.parameters.json")

	if err := os.WriteFile(paramsPath, []byte(paramsContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write ARM parameters file: %v", err)
	}

	manifests = append(manifests, GeneratedAzureManifest{
		Name:    "azuredeploy.parameters.json",
		Type:    "json",
		Path:    paramsPath,
		Content: paramsContent,
		Size:    len(paramsContent),
		Valid:   true,
	})

	return manifests, nil
}

func (s *manifestService) generateARMTemplate(options AzureContainerAppsManifestOptions) string {
	// Build resources array
	resources := make([]interface{}, 0)

	// Add environment if needed
	if options.IncludeEnvironment {
		envResource := map[string]interface{}{
			"type":       "Microsoft.App/managedEnvironments",
			"apiVersion": "2024-03-01",
			"name":       "[parameters('environmentName')]",
			"location":   "[parameters('location')]",
			"properties": map[string]interface{}{
				"appLogsConfiguration": map[string]interface{}{
					"destination": "azure-monitor",
				},
			},
		}
		resources = append(resources, envResource)
	}

	// Build container app resource
	containerAppResource := map[string]interface{}{
		"type":       "Microsoft.App/containerApps",
		"apiVersion": "2024-03-01",
		"name":       "[parameters('appName')]",
		"location":   "[parameters('location')]",
	}

	// Add dependency on environment if we're creating it
	if options.IncludeEnvironment {
		containerAppResource["dependsOn"] = []string{
			"[resourceId('Microsoft.App/managedEnvironments', parameters('environmentName'))]",
		}
	}

	// Add managed identity if requested
	if options.ManagedIdentity {
		containerAppResource["identity"] = map[string]interface{}{
			"type": "SystemAssigned",
		}
	}

	// Build properties
	properties := map[string]interface{}{
		"managedEnvironmentId": "[resourceId('Microsoft.App/managedEnvironments', parameters('environmentName'))]",
	}

	// Configuration
	configuration := map[string]interface{}{}

	// Ingress
	if options.IncludeIngress {
		ingress := map[string]interface{}{
			"external":   true,
			"targetPort": "[parameters('containerPort')]",
			"transport":  "auto",
			"traffic": []map[string]interface{}{
				{
					"latestRevision": true,
					"weight":         100,
				},
			},
		}

		if options.CustomDomain != "" {
			ingress["customDomains"] = []map[string]interface{}{
				{
					"name":        "[parameters('customDomain')]",
					"bindingType": "SniEnabled",
				},
			}
		}

		configuration["ingress"] = ingress
	}

	// Dapr
	if options.EnableDapr {
		dapr := map[string]interface{}{
			"enabled":     true,
			"appProtocol": "http",
		}
		if options.DaprAppId != "" {
			dapr["appId"] = options.DaprAppId
		}
		if options.DaprAppPort > 0 {
			dapr["appPort"] = options.DaprAppPort
		}
		configuration["dapr"] = dapr
	}

	properties["configuration"] = configuration

	// Template
	containerTemplate := map[string]interface{}{
		"containers": []map[string]interface{}{
			{
				"name":  "[parameters('appName')]",
				"image": "[parameters('imageName')]",
				"resources": map[string]interface{}{
					"cpu":    options.Resources.CPU,
					"memory": options.Resources.Memory,
				},
			},
		},
		"scale": map[string]interface{}{
			"minReplicas": options.MinReplicas,
			"maxReplicas": options.MaxReplicas,
		},
	}

	// Add environment variables if present
	if len(options.EnvironmentVariables) > 0 {
		envVars := make([]map[string]interface{}, 0)
		for key, value := range options.EnvironmentVariables {
			envVars = append(envVars, map[string]interface{}{
				"name":  key,
				"value": value,
			})
		}
		containerTemplate["containers"].([]map[string]interface{})[0]["env"] = envVars
	}

	properties["template"] = containerTemplate
	containerAppResource["properties"] = properties

	// Add tags if present
	if len(options.Labels) > 0 {
		containerAppResource["tags"] = options.Labels
	}

	resources = append(resources, containerAppResource)

	// Build complete template
	template := map[string]interface{}{
		"$schema":        "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
		"contentVersion": "1.0.0.0",
		"parameters": map[string]interface{}{
			"location": map[string]interface{}{
				"type":         "string",
				"defaultValue": "[resourceGroup().location]",
				"metadata": map[string]interface{}{
					"description": "Location for all resources",
				},
			},
			"appName": map[string]interface{}{
				"type": "string",
				"metadata": map[string]interface{}{
					"description": "Name of the container app",
				},
			},
			"imageName": map[string]interface{}{
				"type": "string",
				"metadata": map[string]interface{}{
					"description": "Container image to deploy",
				},
			},
			"containerPort": map[string]interface{}{
				"type":         "int",
				"defaultValue": options.Port,
				"metadata": map[string]interface{}{
					"description": "Port exposed by the container",
				},
			},
			"environmentName": map[string]interface{}{
				"type": "string",
				"metadata": map[string]interface{}{
					"description": "Name of the Container Apps environment",
				},
			},
		},
		"resources": resources,
		"outputs": map[string]interface{}{
			"fqdn": map[string]interface{}{
				"type":  "string",
				"value": "[reference(resourceId('Microsoft.App/containerApps', parameters('appName'))).configuration.ingress.fqdn]",
			},
			"appName": map[string]interface{}{
				"type":  "string",
				"value": "[parameters('appName')]",
			},
		},
	}

	// Add custom domain parameter if needed
	if options.CustomDomain != "" {
		template["parameters"].(map[string]interface{})["customDomain"] = map[string]interface{}{
			"type": "string",
			"metadata": map[string]interface{}{
				"description": "Custom domain for the app",
			},
		}
	}

	jsonBytes, _ := json.MarshalIndent(template, "", "  ")
	return string(jsonBytes)
}

func (s *manifestService) generateARMParametersFile(options AzureContainerAppsManifestOptions) string {
	// Same as Bicep parameters, they use the same format
	return s.generateBicepParametersFile(options)
}

func (s *manifestService) validateBicepManifest(manifestContent string) error {
	// Basic validation - check for required Bicep elements
	if !strings.Contains(manifestContent, "resource") {
		return fmt.Errorf("no resource definitions found in Bicep template")
	}
	if !strings.Contains(manifestContent, "Microsoft.App/containerApps") {
		return fmt.Errorf("no Container Apps resource found in Bicep template")
	}
	return nil
}

func (s *manifestService) validateARMManifest(manifestContent string) error {
	// Parse JSON to validate structure
	var template map[string]interface{}
	if err := json.Unmarshal([]byte(manifestContent), &template); err != nil {
		return fmt.Errorf("invalid JSON in ARM template: %v", err)
	}

	// Check for required fields
	if _, ok := template["$schema"]; !ok {
		return fmt.Errorf("missing $schema in ARM template")
	}
	if _, ok := template["resources"]; !ok {
		return fmt.Errorf("missing resources section in ARM template")
	}

	return nil
}
