package cmd

import (
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
	defaultResourceGroup := GenerateDefaultResourceName("container-kit-rg-")
	defaultResourceName := GenerateDefaultResourceName("container-kit-ai-")
	defaultDeploymentName := GenerateDefaultResourceName("container-kit-dep-")
	defaultLocation := DetermineDefaultLocation()

	// Load the .env file
	envVars := make(map[string]string)
	envFile := filepath.Join(projectRoot, ".env")

	if _, err := os.Stat(envFile); err == nil {
		envFromFile, err := godotenv.Read(envFile)
		if err == nil {
			for k, v := range envFromFile {
				envVars[k] = v
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

	// Create/update deployment
	logger.Infof("\n→ Creating/updating deployment '%s'…", config.DeploymentName)
	deployCmd := exec.Command("az", "cognitiveservices", "account", "deployment", "create",
		"--name", config.OpenAIResourceName,
		"--resource-group", config.ResourceGroup,
		"--deployment-name", config.DeploymentName,
		"--model-name", config.ModelID,
		"--model-version", config.ModelVersion,
		"--model-format", "OpenAI",
		"--sku-name", "GlobalStandard",
		"--sku-capacity", "10",
		"--only-show-errors",
		"--output", "none")
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr
	if err := deployCmd.Run(); err != nil {
		return "", "", "", fmt.Errorf("failed to create/update deployment: %w", err)
	}
	logger.Infof("  ✓ Deployment '%s' ready", config.DeploymentName)

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

		// Container Kit variables
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
	content.WriteString("# Container-Kit environment variables\n")
	content.WriteString("# This file was generated/updated by container-kit setup\n\n")

	// Azure OpenAI variables first
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_KEY, envVars[AZURE_OPENAI_KEY]))
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_ENDPOINT, envVars[AZURE_OPENAI_ENDPOINT]))
	content.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_DEPLOYMENT_ID, envVars[AZURE_OPENAI_DEPLOYMENT_ID]))

	content.WriteString("\n# Container-Kit setup variables\n")

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
