package cmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// SetupConfig contains all the configuration needed for the setup process
type SetupConfig struct {
	ResourceGroup       string
	Location            string
	OpenAIResourceName  string
	DeploymentName      string
	ModelID             string
	ModelVersion        string
	TargetRepo          string
	Registry            string
	DockerfileGenerator string
	GenerateSnapshot    bool
	ForceSetup          bool
}

// GenerateDefaultResourceName generates a default name for Azure resources
// with a random suffix to avoid conflicts
func GenerateDefaultResourceName(prefix string) string {
	// Generate a random number between 1000-9999
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSuffix := r.Intn(9000) + 1000

	// Return the formatted name
	return fmt.Sprintf("%s%d", prefix, randomSuffix)
}

// DetermineDefaultLocation attempts to determine a good default Azure location
// based on available regions. Falls back to "eastus" if can't determine.
func DetermineDefaultLocation() string {
	// List of preferred regions in order of preference
	preferredRegions := []string{"eastus", "westus", "westeurope", "northeurope", "southeastasia"}

	// Try to get available regions from Azure CLI
	cmd := exec.Command("az", "account", "list-locations", "--query", "[].name", "-o", "tsv")
	output, err := cmd.Output()

	// If we got a successful result
	if err == nil {
		// Parse the regions
		regions := strings.Split(string(output), "\n")

		// Look for first preferred region that is available
		for _, preferred := range preferredRegions {
			for _, region := range regions {
				if strings.TrimSpace(region) == preferred {
					return preferred
				}
			}
		}
	}

	// Default fallback
	return "eastus"
}

// NormalizeTargetRepoPath takes a target repository path and converts it to an absolute path.
// It also updates the CCP_TARGET_REPO environment variable with the normalized path.
// Returns the normalized path and any error that occurred.
func NormalizeTargetRepoPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error converting target repo path to absolute path: %w", err)
	}

	// Update environment variable
	os.Setenv("CCP_TARGET_REPO", absPath)

	return absPath, nil
}

// LoadSetupConfig loads configuration from environment, flags, and defaults
func LoadSetupConfig(cmd *cobra.Command, args []string, projectRoot string) (*SetupConfig, error) {
	// Generate default resource names
	defaultResourceGroup := GenerateDefaultResourceName("ccp-rg-")
	defaultResourceName := GenerateDefaultResourceName("ccp-ai-")
	defaultDeploymentName := GenerateDefaultResourceName("ccp-dep-")
	defaultLocation := DetermineDefaultLocation()

	// Get force-setup flag first
	forceSetup, _ := cmd.Flags().GetBool("force-setup")

	// Load the .env file only if force-setup is NOT enabled
	envVars := make(map[string]string)
	envFile := filepath.Join(projectRoot, ".env")

	if !forceSetup {
		if _, err := os.Stat(envFile); err == nil {
			envFromFile, err := godotenv.Read(envFile)
			if err == nil {
				for k, v := range envFromFile {
					envVars[k] = v
				}
			}
		}
	}

	// Get flags
	resourceGroup, _ := cmd.Flags().GetString("resource-group")
	location, _ := cmd.Flags().GetString("location")
	openaiResourceName, _ := cmd.Flags().GetString("openai-resource")
	deploymentName, _ := cmd.Flags().GetString("deployment")
	modelID, _ := cmd.Flags().GetString("model-id")
	modelVersion, _ := cmd.Flags().GetString("model-version")
	targetRepo, _ := cmd.Flags().GetString("target-repo")
	registry, _ := cmd.Flags().GetString("registry")
	dockerfileGenerator, _ := cmd.Flags().GetString("dockerfile-generator")
	generateSnapshot, _ := cmd.Flags().GetBool("snapshot")
	// forceSetup already retrieved above

	// Create config, prioritizing flag values, then .env, then env vars, then defaults
	config := &SetupConfig{
		ResourceGroup:       getFirstNonEmpty(resourceGroup, envVars["CCP_RESOURCE_GROUP"], os.Getenv("CCP_RESOURCE_GROUP"), defaultResourceGroup),
		Location:            getFirstNonEmpty(location, envVars["CCP_LOCATION"], os.Getenv("CCP_LOCATION"), defaultLocation),
		OpenAIResourceName:  getFirstNonEmpty(openaiResourceName, envVars["CCP_OPENAI_RESOURCE_NAME"], os.Getenv("CCP_OPENAI_RESOURCE_NAME"), defaultResourceName),
		DeploymentName:      getFirstNonEmpty(deploymentName, envVars["CCP_DEPLOYMENT_NAME"], os.Getenv("CCP_DEPLOYMENT_NAME"), defaultDeploymentName),
		ModelID:             getFirstNonEmpty(modelID, envVars["CCP_MODEL_ID"], os.Getenv("CCP_MODEL_ID"), "o3-mini"),
		ModelVersion:        getFirstNonEmpty(modelVersion, envVars["CCP_MODEL_VERSION"], os.Getenv("CCP_MODEL_VERSION"), "2025-01-31"),
		Registry:            getFirstNonEmpty(registry, envVars["CCP_REGISTRY"], os.Getenv("CCP_REGISTRY"), "localhost:5001"),
		DockerfileGenerator: getFirstNonEmpty(dockerfileGenerator, "", "", "draft"),
		GenerateSnapshot:    generateSnapshot,
		ForceSetup:          forceSetup,
	}

	// Handle target repo from args or env
	targetRepoPath := getFirstNonEmpty(targetRepo, envVars["CCP_TARGET_REPO"], os.Getenv("CCP_TARGET_REPO"), "")
	if targetRepoPath == "" && len(args) > 0 {
		targetRepoPath = args[0]
	}

	// Normalize the target repo path
	normalizedPath, err := NormalizeTargetRepoPath(targetRepoPath)
	if err != nil {
		return nil, err
	}
	config.TargetRepo = normalizedPath

	return config, nil
}

// ValidateConfig validates that all required configuration values are set
func (c *SetupConfig) ValidateConfig() error {
	var missing []string

	// Only the target repository is required
	if c.TargetRepo == "" {
		missing = append(missing, "target repository")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required values: %s", strings.Join(missing, ", "))
	}

	return nil
}

// PrintConfig prints the configuration values
func (c *SetupConfig) PrintConfig() {
	logger.Info("→ Configuration:\n")
	logger.Infof("  RESOURCE_GROUP:        %s", c.ResourceGroup)
	logger.Infof("  LOCATION:              %s", c.Location)
	logger.Infof("  OPENAI_RES_NAME:       %s", c.OpenAIResourceName)
	logger.Infof("  DEPLOYMENT_NAME:       %s", c.DeploymentName)
	logger.Infof("  MODEL_ID:              %s", c.ModelID)
	logger.Infof("  MODEL_VERSION:         %s", c.ModelVersion)
	logger.Infof("  TARGET_REPO:           %s", c.TargetRepo)
}

// RunSetup performs the full setup process and returns Azure OpenAI credentials
func RunSetup(config *SetupConfig) (string, string, string, error) {
	// Check prerequisites
	logger.Info("\n→ Verifying prerequisites…")
	prereqs := []string{"az", "go", "kubectl", "docker", "kind"}
	for _, prereq := range prereqs {
		if _, err := exec.LookPath(prereq); err != nil {
			return "", "", "", fmt.Errorf("prerequisite %s not found", prereq)
		}
		logger.Infof("✓ %s\n", prereq)
	}

	// Check RPM capacity across regions and determine optimal deployment settings
	optimalRegion, err := CheckRPMCapacityInRegions(config.ModelID, config.ModelVersion, config.Location)
	capacity := 10 // Use default capacity
	if err != nil {
		logger.Warnf("Failed to check RPM capacity: %v", err)
		return "", "", "", fmt.Errorf("failed to determine optimal region: %w", err)
	} else if optimalRegion != config.Location {
		logger.Infof("→ Using region '%s' instead of '%s' for better capacity", optimalRegion, config.Location)
		config.Location = optimalRegion
	}

	// Ensure resource group exists
	logger.Infof("\n→ Checking resource group '%s'…\n", config.ResourceGroup)
	rgCheckCmd := exec.Command("az", "group", "show", "--name", config.ResourceGroup)
	rgCheckCmd.Stderr = nil
	rgCheckCmd.Stdout = nil
	if err := rgCheckCmd.Run(); err != nil {
		logger.Warnf("  not found → creating in '%s'…\n", config.Location)
		rgCreateCmd := exec.Command("az", "group", "create",
			"--name", config.ResourceGroup,
			"--location", config.Location,
			"--output", "none")
		rgCreateCmd.Stdout = os.Stdout
		rgCreateCmd.Stderr = os.Stderr
		if err := rgCreateCmd.Run(); err != nil {
			return "", "", "", fmt.Errorf("failed to create resource group: %w", err)
		}
		logger.Info("  ✓ Created")
	} else {
		logger.Info("  ✓ Exists")
	}

	// Ensure OpenAI Cognitive Services account exists
	logger.Infof("\n→ Ensuring Cognitive Services account '%s' (kind=OpenAI)…\n", config.OpenAIResourceName)
	csCheckCmd := exec.Command("az", "cognitiveservices", "account", "show",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup)
	csCheckCmd.Stderr = nil
	csCheckCmd.Stdout = nil
	if err := csCheckCmd.Run(); err != nil {
		logger.Warn("  not found → creating…")
		csCreateCmd := exec.Command("az", "cognitiveservices", "account", "create",
			"--name", config.OpenAIResourceName,
			"--resource-group", config.ResourceGroup,
			"--kind", "OpenAI",
			"--sku", "S0",
			"--location", config.Location,
			"--yes",
			"--output", "none")
		csCreateCmd.Stdout = os.Stdout
		csCreateCmd.Stderr = os.Stderr
		if err := csCreateCmd.Run(); err != nil {
			return "", "", "", fmt.Errorf("failed to create Cognitive Services account: %w", err)
		}
		logger.Info("  ✓ Created account")
	} else {
		logger.Info("  ✓ Account exists")
	}

	// Fetch API key
	logger.Info("\n→ Retrieving API key and endpoint…")
	keyCmd := exec.Command("az", "cognitiveservices", "account", "keys", "list",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup,
		"--query", "key1", "-o", "tsv")
	keyOutput, err := keyCmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to retrieve API key: %w", err)
	}
	apiKey := strings.TrimSpace(string(keyOutput))
	logger.Info("  ✓ Key retrieved")

	// Fetch endpoint
	endpointCmd := exec.Command("az", "cognitiveservices", "account", "show",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup,
		"--query", "properties.endpoint", "-o", "tsv")
	endpointOutput, err := endpointCmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to retrieve endpoint: %w", err)
	}
	endpoint := strings.TrimSpace(string(endpointOutput))
	logger.Info("  ✓ Endpoint retrieved")

	// List available models
	logger.Infof("\n→ Available models on '%s':", config.OpenAIResourceName)
	modelsCmd := exec.Command("az", "cognitiveservices", "account", "list-models",
		"--resource-group", config.ResourceGroup,
		"--name", config.OpenAIResourceName,
		"--output", "table")
	modelsCmd.Stdout = os.Stdout
	modelsCmd.Stderr = os.Stderr
	if err := modelsCmd.Run(); err != nil {
		return "", "", "", fmt.Errorf("failed to list models: %w", err)
	}

	// Create/update deployment with optimal capacity
	logger.Infof("\n→ Creating/updating deployment '%s' with capacity %d…", config.DeploymentName, capacity)

	// Check if deployment already exists
	deployCheckCmd := exec.Command("az", "cognitiveservices", "account", "deployment", "show",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup,
		"--deployment-name", config.DeploymentName)
	deployCheckCmd.Stderr = nil
	deployCheckCmd.Stdout = nil
	deploymentExists := deployCheckCmd.Run() == nil

	if deploymentExists {
		if config.ForceSetup {
			logger.Infof("  Deployment '%s' exists. Force setup enabled - deleting existing deployment first...", config.DeploymentName)
			deleteCmd := exec.Command("az", "cognitiveservices", "account", "deployment", "delete",
				"--name", config.OpenAIResourceName,
				"--resource-group", config.ResourceGroup,
				"--deployment-name", config.DeploymentName,
				"--yes")
			deleteCmd.Stdout = os.Stdout
			deleteCmd.Stderr = os.Stderr
			if err := deleteCmd.Run(); err != nil {
				logger.Warnf("Warning: Failed to delete existing deployment: %v", err)
				logger.Info("  Attempting to proceed with update...")
			} else {
				logger.Info("  ✓ Existing deployment deleted")
			}
		} else {
			logger.Infof("  Deployment '%s' already exists. Use --force-setup to overwrite.", config.DeploymentName)
			logger.Info("  Attempting to update existing deployment...")
		}
	}

	deployCmd := exec.Command("az", "cognitiveservices", "account", "deployment", "create",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup,
		"--deployment-name", config.DeploymentName,
		"--model-name", config.ModelID,
		"--model-version", config.ModelVersion,
		"--model-format", "OpenAI",
		"--sku-name", "GlobalStandard",
		"--sku-capacity", fmt.Sprintf("%d", capacity),
		"--only-show-errors",
		"--output", "none")
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr
	if err := deployCmd.Run(); err != nil {
		return "", "", "", fmt.Errorf("failed to create/update deployment: %w", err)
	}
	logger.Infof("  ✓ Deployment '%s' ready with capacity %d", config.DeploymentName, capacity)

	// Setting deployment ID
	deploymentID := config.DeploymentName

	logger.Infof("\n→ Exporting AZURE_* variables…")

	return apiKey, endpoint, deploymentID, nil
}

// UpdateEnvFile updates the .env file with all the setup variables
func UpdateEnvFile(projectRoot string, config *SetupConfig, apiKey, endpoint, deploymentID string) error {
	// Create a map of all values to save
	envVars := map[string]string{
		// Azure OpenAI variables
		AZURE_OPENAI_KEY:           apiKey,
		AZURE_OPENAI_ENDPOINT:      endpoint,
		AZURE_OPENAI_DEPLOYMENT_ID: deploymentID,

		// Container Copilot variables
		"CCP_RESOURCE_GROUP":       config.ResourceGroup,
		"CCP_LOCATION":             config.Location,
		"CCP_OPENAI_RESOURCE_NAME": config.OpenAIResourceName,
		"CCP_DEPLOYMENT_NAME":      config.DeploymentName,
		"CCP_MODEL_ID":             config.ModelID,
		"CCP_MODEL_VERSION":        config.ModelVersion,
		"CCP_TARGET_REPO":          config.TargetRepo,
	}

	// Read existing env file to preserve other variables
	envFile := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envFile); err == nil {
		existingVars, err := godotenv.Read(envFile)
		if err == nil {
			// Add any existing variables that we're not explicitly setting
			for k, v := range existingVars {
				if _, exists := envVars[k]; !exists {
					envVars[k] = v
				}
			}
		}
	}

	// Write .env file content
	var content strings.Builder
	content.WriteString("# Container-Copilot environment variables\n")
	content.WriteString("# This file was generated/updated by container-copilot setup\n\n")

	// Azure OpenAI variables first
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_KEY, envVars[AZURE_OPENAI_KEY]))
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_ENDPOINT, envVars[AZURE_OPENAI_ENDPOINT]))
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_DEPLOYMENT_ID, envVars[AZURE_OPENAI_DEPLOYMENT_ID]))

	content.WriteString("\n# Container-Copilot setup variables\n")

	// All CCP_ variables
	for k, v := range envVars {
		if strings.HasPrefix(k, "CCP_") {
			content.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	content.WriteString("\n# Other variables\n")

	// All other variables
	for k, v := range envVars {
		if !strings.HasPrefix(k, "CCP_") &&
			k != AZURE_OPENAI_KEY &&
			k != AZURE_OPENAI_ENDPOINT &&
			k != AZURE_OPENAI_DEPLOYMENT_ID {
			content.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}

	// Write the file
	return os.WriteFile(envFile, []byte(content.String()), 0644)
}

func getFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func maskSecretValue(s string) string {
	if s == "" {
		return "<empty>"
	}
	return "****"
}

// ModelQuota represents the quota information for a model in a region
type ModelQuota struct {
	CurrentValue int `json:"currentValue"`
	Limit        int `json:"limit"`
	Name         struct {
		LocalizedValue string `json:"localizedValue"`
		Value          string `json:"value"`
	} `json:"name"`
	Unit string `json:"unit"`
}

// QuotaResponse represents the response from the Azure quota API
type QuotaResponse struct {
	Value []ModelQuota `json:"value"`
}

// RegionRPMInfo represents capacity information for a region
type RegionRPMInfo struct {
	Region            string
	AvailableRPM      int
	AvailableAccounts int
}

// CheckRPMCapacityInRegions checks RPM capacity for the specified model across multiple regions
// Returns the best region with sufficient capacity
func CheckRPMCapacityInRegions(modelID, modelVersion, preferredLocation string) (string, error) {
	logger.Info("\n→ Checking RPM capacity across regions...")

	// Define preferred regions to check, starting with user's preferred location
	preferredRegions := []string{
		preferredLocation, // Start with the configured location
		"westus", "westus2", "eastus2", "centralus",
		"westeurope", "northeurope", "uksouth", "francecentral",
		"southeastasia", "japaneast", "australiaeast", "canadacentral",
	}

	var regionInfo []RegionRPMInfo
	optimalRPM := 100    // Preferred RPM for early return
	minRequiredRPM := 10 // Minimum RPM needed for deployment

	subCmd := exec.Command("az", "account", "show", "--query", "id", "-o", "tsv")
	subOutput, err := subCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get subscription ID: %v", err)
	}
	subscriptionID := strings.TrimSpace(string(subOutput))

	// Check all provided regions for capacity
	for i, region := range preferredRegions {
		logger.Infof("  Checking region %d/%d: %s...", i+1, len(preferredRegions), region)
		// Get quota information for the region using REST API
		quotaURL := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.CognitiveServices/locations/%s/usages?api-version=2023-05-01", subscriptionID, region)
		quotaCmd := exec.Command("az", "rest", "--method", "GET", "--url", quotaURL)

		output, err := quotaCmd.Output()
		if err != nil {
			logger.Warnf("    Failed to check capacity in %s: %v", region, err)
			continue
		}

		var quotaResponse QuotaResponse
		if err := json.Unmarshal(output, &quotaResponse); err != nil {
			logger.Warnf("    Failed to parse quota response for %s: %v", region, err)
			continue
		}

		// Look for both Cognitive Services account quota and deployment quota
		var availableRPM int
		var availableAccounts int
		foundDeploymentQuota := false
		foundAccountQuota := false

		// Check for OpenAI account quota (this is what matters for creating OpenAI accounts)
		for _, quota := range quotaResponse.Value {
			if quota.Name.Value == "OpenAI.S0.AccountCount" {
				availableAccounts = quota.Limit - quota.CurrentValue
				logger.Infof("    Available OpenAI accounts: %d (limit: %d, current: %d)",
					availableAccounts, quota.Limit, quota.CurrentValue)
				foundAccountQuota = true
				break
			}
		}

		// Check for GlobalStandard deployment quota for the specific model
		globalStandardQuotaName := fmt.Sprintf("OpenAI.GlobalStandard.%s", modelID)
		for _, quota := range quotaResponse.Value {
			if quota.Name.Value == globalStandardQuotaName {
				availableRPM = quota.Limit - quota.CurrentValue
				logger.Infof("    Available GlobalStandard capacity: %d (limit: %d, current: %d) for %s",
					availableRPM, quota.Limit, quota.CurrentValue, quota.Name.Value)
				foundDeploymentQuota = true
				break
			}
		}

		// Debug: Log all available quotas for this region if no match found
		if !foundAccountQuota || !foundDeploymentQuota {
			logger.Debugf("    Available quotas in %s:", region)
			for _, quota := range quotaResponse.Value {
				if (strings.Contains(quota.Name.Value, "OpenAI") || strings.Contains(quota.Name.Value, "AccountCount")) && quota.Limit > 0 {
					logger.Debugf("      - %s: %d available (limit: %d, current: %d)",
						quota.Name.Value, quota.Limit-quota.CurrentValue, quota.Limit, quota.CurrentValue)
				}
			}
		}

		// Region is only suitable if it has both account quota AND deployment quota available
		if foundAccountQuota && foundDeploymentQuota && availableAccounts > 0 && availableRPM > 0 {
			regionInfo = append(regionInfo, RegionRPMInfo{
				Region:            region,
				AvailableRPM:      availableRPM,
				AvailableAccounts: availableAccounts,
			})

			// If we found a region with optimal capacity, return immediately
			if availableRPM >= optimalRPM {
				logger.Infof("  ✓ Found optimal capacity in %s (available accounts: %d, available RPM: %d)",
					region, availableAccounts, availableRPM)
				return region, nil
			}
		} else {
			if !foundAccountQuota {
				logger.Infof("    No OpenAI account quota found in %s", region)
			}
			if !foundDeploymentQuota {
				logger.Infof("    No deployment quota found for %s in %s", modelID, region)
			}
			if foundAccountQuota && availableAccounts == 0 {
				logger.Infof("    No available OpenAI account quota in %s", region)
			}
			if foundDeploymentQuota && availableRPM == 0 {
				logger.Infof("    No available deployment capacity in %s", region)
			}
		}
	}

	// If no region has sufficient capacity, fail
	if len(regionInfo) == 0 {
		return "", fmt.Errorf("no regions found with available capacity for model %s", modelID)
	}

	// Sort regions by available RPM and pick the best one
	bestRegion := regionInfo[0]
	for _, info := range regionInfo[1:] {
		if info.AvailableRPM > bestRegion.AvailableRPM {
			bestRegion = info
		}
	}

	// Fail if the best region doesn't meet minimum requirements
	if bestRegion.AvailableRPM < minRequiredRPM {
		return "", fmt.Errorf("best available region '%s' has insufficient capacity (%d RPM) - minimum required is %d RPM for model %s",
			bestRegion.Region, bestRegion.AvailableRPM, minRequiredRPM, modelID)
	}

	logger.Infof("  ✓ Best available region: %s (available accounts: %d, available RPM: %d)",
		bestRegion.Region, bestRegion.AvailableAccounts, bestRegion.AvailableRPM)
	return bestRegion.Region, nil
}

func init() {
	// Setup command flags
	setupCmd.PersistentFlags().StringVarP(&resourceGroup, "resource-group", "g", "", "Azure resource group")
	setupCmd.PersistentFlags().StringVarP(&location, "location", "l", "", "Azure region for the resource group")
	setupCmd.PersistentFlags().StringVarP(&openaiResourceName, "openai-resource", "a", "", "Azure OpenAI Cognitive Services resource name")
	setupCmd.PersistentFlags().StringVarP(&deploymentName, "deployment", "d", "", "Deployment name")
	setupCmd.PersistentFlags().StringVarP(&modelID, "model-id", "m", "o3-mini", "Model ID")
	setupCmd.PersistentFlags().StringVarP(&modelVersion, "model-version", "", "2025-01-31", "Model version")
	setupCmd.PersistentFlags().StringVarP(&targetRepo, "target-repo", "t", "", "Path to the repo to containerize")
	setupCmd.PersistentFlags().Bool("force-setup", false, "Force setup even if environment variables are already set")
}
