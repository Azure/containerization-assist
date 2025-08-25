package steps

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/containerization-assist/pkg/infrastructure/azure"
)

// AzureContainerAppsResult contains deployment configuration for Azure
type AzureContainerAppsResult struct {
	ResourceGroup   string                 `json:"resource_group"`
	AppName         string                 `json:"app_name"`
	EnvironmentName string                 `json:"environment_name"`
	Location        string                 `json:"location"`
	Manifests       map[string]interface{} `json:"manifests"`
	AppURL          string                 `json:"app_url,omitempty"`
	FQDN            string                 `json:"fqdn,omitempty"`
	DeployedAt      time.Time              `json:"deployed_at"`
	OutputFormat    string                 `json:"output_format"` // "bicep" or "arm"
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// GenerateAzureContainerAppsManifests creates Azure Container Apps manifests
func GenerateAzureContainerAppsManifests(
	buildResult *BuildResult,
	appName, resourceGroup, location, environmentName string,
	port int,
	repoPath, registryURL string,
	outputFormat string, // "bicep" or "arm"
	logger *slog.Logger,
) (*AzureContainerAppsResult, error) {

	if buildResult == nil {
		return nil, fmt.Errorf("build result is required")
	}

	// Set defaults
	if appName == "" {
		appName = buildResult.ImageName
	}

	if resourceGroup == "" {
		resourceGroup = "containerized-apps-rg"
	}

	if location == "" {
		location = "eastus"
	}

	if environmentName == "" {
		environmentName = "containerized-apps-env"
	}

	if port <= 0 {
		port = 8080 // Default port
	}

	if outputFormat == "" {
		outputFormat = "bicep"
	}

	if registryURL == "" {
		registryURL = "myregistry.azurecr.io" // Default Azure Container Registry
	}

	// Create image reference using the provided registry URL
	imageRef := fmt.Sprintf("%s/%s:%s", registryURL, buildResult.ImageName, buildResult.ImageTag)

	// Create manifests directory in the repository path (persistent)
	manifestDir := filepath.Join(repoPath, "azure-manifests")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create manifests directory: %w", err)
	}

	// Configure manifest options
	manifestOptions := azure.AzureContainerAppsManifestOptions{
		Template:           outputFormat,
		AppName:            appName,
		ResourceGroup:      resourceGroup,
		Location:           location,
		EnvironmentName:    environmentName,
		ImageRef:           imageRef,
		Port:               port,
		Replicas:           1,
		MinReplicas:        1,
		MaxReplicas:        10,
		OutputDir:          manifestDir,
		IncludeEnvironment: true,
		IncludeIngress:     true,
		EnableDapr:         false,
		ManagedIdentity:    true,
		Labels: map[string]string{
			"app":        appName,
			"managed-by": "containerization-assist",
		},
		Resources: &azure.ContainerResources{
			CPU:    0.5,
			Memory: "1.0Gi",
		},
	}

	// Create manifest service
	manifestService := azure.NewAzureContainerAppsManifestService(logger)

	// Generate manifests
	ctx := context.Background()
	manifestResult, err := manifestService.GenerateManifests(ctx, manifestOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Azure Container Apps manifests: %v", err)
	}

	if !manifestResult.Success {
		return nil, fmt.Errorf("Azure manifest generation unsuccessful: %v", manifestResult.Error)
	}

	// Convert manifest content to interface map for compatibility
	manifests := map[string]interface{}{
		"path":      manifestResult.ManifestPath,
		"manifests": manifestResult.Manifests,
		"template":  manifestResult.Template,
		"outputDir": manifestResult.OutputDir,
	}

	// Add individual manifest paths for easy access
	for _, manifest := range manifestResult.Manifests {
		manifests[manifest.Name] = manifest.Path
	}

	if logger != nil {
		logger.Info("Azure Container Apps manifests generated successfully",
			slog.String("app_name", appName),
			slog.String("resource_group", resourceGroup),
			slog.String("location", location),
			slog.String("environment", environmentName),
			slog.String("format", outputFormat),
			slog.String("output_dir", manifestDir),
		)
	}

	return &AzureContainerAppsResult{
		ResourceGroup:   resourceGroup,
		AppName:         appName,
		EnvironmentName: environmentName,
		Location:        location,
		Manifests:       manifests,
		DeployedAt:      time.Now(),
		OutputFormat:    outputFormat,
		Metadata: map[string]interface{}{
			"imageRef":     imageRef,
			"port":         port,
			"manifestsDir": manifestDir,
		},
	}, nil
}

// ValidateAzureContainerAppsManifests validates Azure Container Apps manifests
func ValidateAzureContainerAppsManifests(
	ctx context.Context,
	manifestPath string,
	outputFormat string,
	strictMode bool,
	logger *slog.Logger,
) (*azure.ValidationResult, error) {

	if manifestPath == "" {
		return nil, fmt.Errorf("manifest path is required")
	}

	if outputFormat == "" {
		// Try to detect format from file extension
		if filepath.Ext(manifestPath) == ".bicep" {
			outputFormat = "bicep"
		} else if filepath.Ext(manifestPath) == ".json" {
			outputFormat = "arm"
		} else {
			return nil, fmt.Errorf("cannot detect manifest format from file extension")
		}
	}

	if logger != nil {
		logger.Info("Validating Azure Container Apps manifest",
			slog.String("path", manifestPath),
			slog.String("format", outputFormat),
			slog.Bool("strict_mode", strictMode),
		)
	}

	// Validate the manifest
	result, err := azure.ValidateAzureManifests(
		ctx,
		manifestPath,
		outputFormat,
		strictMode,
		logger,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to validate Azure manifest: %v", err)
	}

	if logger != nil {
		if result.Valid {
			logger.Info("Azure Container Apps manifest validation successful",
				slog.String("path", manifestPath),
				slog.Int("warnings", len(result.Warnings)),
			)
		} else {
			logger.Error("Azure Container Apps manifest validation failed",
				slog.String("path", manifestPath),
				slog.Int("errors", len(result.Errors)),
				slog.Int("warnings", len(result.Warnings)),
			)
		}
	}

	return result, nil
}
