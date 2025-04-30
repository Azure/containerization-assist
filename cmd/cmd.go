package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/runner"
	"github.com/spf13/cobra"
)

const (
	AZURE_OPENAI_KEY           = "AZURE_OPENAI_KEY"
	AZURE_OPENAI_ENDPOINT      = "AZURE_OPENAI_ENDPOINT"
	AZURE_OPENAI_DEPLOYMENT_ID = "AZURE_OPENAI_DEPLOYMENT_ID"
)

var (
	registry            string
	dockerfileGenerator string
	generateSnapshot    bool

	// Setup command variables
	resourceGroup      string
	location           string
	openaiResourceName string
	deploymentName     string
	modelID            string
	modelVersion       string
	targetRepo         string
)

var rootCmd = &cobra.Command{
	Use:   "container-copilot",
	Short: "An AI-Powered CLI tool to containerize your app and generate Kubernetes artifacts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Dockerfile and Kubernetes manifests",
	Long:  `The generate command will add Dockerfile and Kubernetes manifests to your project based on the project structure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("error getting current directory: %w", err)
		}
		if len(args) > 0 {
			targetDir = args[0]
		}

		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := generate(targetDir, registry, dockerfileGenerator == "draft", generateSnapshot, c); err != nil {
			return fmt.Errorf("error generating artifacts: %w", err)
		}

		return nil
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Azure OpenAI connection",
	Long:  `The test command will test the Azure OpenAI connection based on the environment variables set and print a response.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := c.TestOpenAIConn(); err != nil {
			return fmt.Errorf("error testing Azure OpenAI connection: %w", err)
		}

		return nil
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up Azure OpenAI resources and run container-copilot",
	Long:  `The setup command will provision Azure OpenAI resources, deploy the model, and run container-copilot to generate artifacts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("failed to determine source file location")
		}

		// The script should be at <repo root>/hack/run-container-copilot.sh
		projectRoot := filepath.Dir(filepath.Dir(file)) // cmd.go -> cmd -> project root
		scriptPath := filepath.Join(projectRoot, "hack", "run-container-copilot.sh")

		// Verify script exists
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return fmt.Errorf("script not found at %s: %w", scriptPath, err)
		}

		// Check for existing .env file and load its content
		envFile := filepath.Join(projectRoot, ".env")
		existingEnvVars := make(map[string]string)

		// Load existing .env file if it exists
		if _, err := os.Stat(envFile); err == nil {
			content, err := os.ReadFile(envFile)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						existingEnvVars[parts[0]] = parts[1]
					}
				}
			}
		}

		// Set environment variables from command line args
		if resourceGroup != "" {
			os.Setenv("CCP_RESOURCE_GROUP", resourceGroup)
			existingEnvVars["CCP_RESOURCE_GROUP"] = resourceGroup
		}
		if location != "" {
			os.Setenv("CCP_LOCATION", location)
			existingEnvVars["CCP_LOCATION"] = location
		}
		if openaiResourceName != "" {
			os.Setenv("CCP_OPENAI_RESOURCE_NAME", openaiResourceName)
			existingEnvVars["CCP_OPENAI_RESOURCE_NAME"] = openaiResourceName
		}
		if deploymentName != "" {
			os.Setenv("CCP_DEPLOYMENT_NAME", deploymentName)
			existingEnvVars["CCP_DEPLOYMENT_NAME"] = deploymentName
		}
		if modelID != "" {
			os.Setenv("CCP_MODEL_ID", modelID)
			existingEnvVars["CCP_MODEL_ID"] = modelID
		}
		if modelVersion != "" {
			os.Setenv("CCP_MODEL_VERSION", modelVersion)
			existingEnvVars["CCP_MODEL_VERSION"] = modelVersion
		}
		if targetRepo != "" {
			os.Setenv("CCP_TARGET_REPO", targetRepo)
			existingEnvVars["CCP_TARGET_REPO"] = targetRepo
		}

		// Build the command to run the setup part of the script
		setupCmd := exec.Command("bash", scriptPath, "--setup-only")
		setupCmd.Stdout = os.Stdout
		setupCmd.Stderr = os.Stderr

		// Run the command
		fmt.Println("Setting up Azure OpenAI resources...")
		if err := setupCmd.Run(); err != nil {
			return fmt.Errorf("error running setup script: %w", err)
		}


		fmt.Println("Retrieving Azure OpenAI configuration...")

		// Get the resource name and resource group from the environment or flags
		resName := openaiResourceName
		if resName == "" {
			resName = existingEnvVars["CCP_OPENAI_RESOURCE_NAME"]
		}
		resGroup := resourceGroup
		if resGroup == "" {
			resGroup = existingEnvVars["CCP_RESOURCE_GROUP"]
		}
		deployName := deploymentName
		if deployName == "" {
			deployName = existingEnvVars["CCP_DEPLOYMENT_NAME"]
		}

		// Verify we have the necessary values
		if resName == "" || resGroup == "" || deployName == "" {
			return fmt.Errorf("missing required values for Azure resources. Check CCP_OPENAI_RESOURCE_NAME, CCP_RESOURCE_GROUP, and CCP_DEPLOYMENT_NAME in .env file or provide them via flags")
		}

		// Get the API key using Azure CLI
		keyCmd := exec.Command("az", "cognitiveservices", "account", "keys", "list",
			"--name", resName,
			"--resource-group", resGroup,
			"--query", "key1", "-o", "tsv")
		keyOutput, err := keyCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to retrieve API key: %w", err)
		}
		newApiKey := strings.TrimSpace(string(keyOutput))

		// Get the endpoint using Azure CLI
		endpointCmd := exec.Command("az", "cognitiveservices", "account", "show",
			"--name", resName,
			"--resource-group", resGroup,
			"--query", "properties.endpoint", "-o", "tsv")
		endpointOutput, err := endpointCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to retrieve endpoint: %w", err)
		}
		newEndpoint := strings.TrimSpace(string(endpointOutput))

		// Set the deployment ID
		newDeploymentID := deployName

		// Update environment variables
		os.Setenv(AZURE_OPENAI_KEY, newApiKey)
		os.Setenv(AZURE_OPENAI_ENDPOINT, newEndpoint)
		os.Setenv(AZURE_OPENAI_DEPLOYMENT_ID, newDeploymentID)

		// Update the environment variables map
		existingEnvVars[AZURE_OPENAI_KEY] = newApiKey
		existingEnvVars[AZURE_OPENAI_ENDPOINT] = newEndpoint
		existingEnvVars[AZURE_OPENAI_DEPLOYMENT_ID] = newDeploymentID

		// Write updated environment variables to .env file
		var envContent strings.Builder
		envContent.WriteString("# Container-Copilot environment variables\n")
		envContent.WriteString("# This file was generated/updated by container-copilot setup\n\n")

		// Write Azure variables first
		if val, ok := existingEnvVars[AZURE_OPENAI_KEY]; ok {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_KEY, val))
		}
		if val, ok := existingEnvVars[AZURE_OPENAI_ENDPOINT]; ok {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_ENDPOINT, val))
		}
		if val, ok := existingEnvVars[AZURE_OPENAI_DEPLOYMENT_ID]; ok {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", AZURE_OPENAI_DEPLOYMENT_ID, val))
		}

		envContent.WriteString("\n# Container-Copilot setup variables\n")

		// Write all CCP_ variables
		for key, val := range existingEnvVars {
			if strings.HasPrefix(key, "CCP_") {
				envContent.WriteString(fmt.Sprintf("%s=%s\n", key, val))
			}
		}

		// Write any other variables that were already in the file
		for key, val := range existingEnvVars {
			if !strings.HasPrefix(key, "CCP_") &&
				key != AZURE_OPENAI_KEY &&
				key != AZURE_OPENAI_ENDPOINT &&
				key != AZURE_OPENAI_DEPLOYMENT_ID {
				envContent.WriteString(fmt.Sprintf("%s=%s\n", key, val))
			}
		}

		// Write the file
		if err := os.WriteFile(envFile, []byte(envContent.String()), 0644); err != nil {
			fmt.Printf("Warning: Failed to update .env file: %v\n", err)
		} else {
			fmt.Printf("Updated .env file at %s\n", envFile)

			// Debug output of important values
			fmt.Printf("Azure OpenAI Key: %s\n", maskSecret(existingEnvVars[AZURE_OPENAI_KEY]))
			fmt.Printf("Azure OpenAI Endpoint: %s\n", existingEnvVars[AZURE_OPENAI_ENDPOINT])
			fmt.Printf("Azure OpenAI Deployment ID: %s\n", existingEnvVars[AZURE_OPENAI_DEPLOYMENT_ID])
			fmt.Printf("Target Repo: %s\n", existingEnvVars["CCP_TARGET_REPO"])
		}

		// Now that the resources are set up, continue with the generate flow
		// Get the target directory from the target repo variable
		targetDir := targetRepo
		if targetDir == "" {
			targetDir = existingEnvVars["CCP_TARGET_REPO"] // Check the loaded .env vars
		}
		if targetDir == "" {
			targetDir = os.Getenv("CCP_TARGET_REPO") // Check env as last resort
		}
		if targetDir == "" && len(args) > 0 {
			targetDir = args[0]
		}

		if targetDir == "" {
			return fmt.Errorf("no target repository specified, please provide it with --target-repo or CCP_TARGET_REPO")
		}

		// Initialize clients (this will use the AZURE_* environment variables set by the script)
		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}

		// Generate the artifacts
		fmt.Printf("Generating artifacts for %s...\n", targetDir)
		if err := generate(targetDir, registry, dockerfileGenerator == "draft", generateSnapshot, c); err != nil {
			return fmt.Errorf("error generating artifacts: %w", err)
		}

		return nil
	},
}

func Execute() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.Execute()
}

func initClients() (*clients.Clients, error) {

	// read from .env

	apiKey := os.Getenv(AZURE_OPENAI_KEY)
	endpoint := os.Getenv(AZURE_OPENAI_ENDPOINT)
	deploymentID := os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID)

	var missingVars []string
	if apiKey == "" {
		missingVars = append(missingVars, AZURE_OPENAI_KEY)
	}
	if endpoint == "" {
		missingVars = append(missingVars, AZURE_OPENAI_ENDPOINT)
	}
	if deploymentID == "" {
		missingVars = append(missingVars, AZURE_OPENAI_DEPLOYMENT_ID)
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing environment variables: %s", strings.Join(missingVars, ", "))
	}

	azOpenAIClient, err := ai.NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %w", err)
	}

	cmdRunner := &runner.DefaultCommandRunner{}

	clients := &clients.Clients{
		AzOpenAIClient: azOpenAIClient,
		Docker:         docker.NewDockerCmdRunner(cmdRunner),
		Kind:           kind.NewKindCmdRunner(cmdRunner),
		Kube:           k8s.NewKubeCmdRunner(cmdRunner),
	}

	return clients, nil
}

func init() {
	generateCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "localhost:5001", "Docker registry to push the image to")
	generateCmd.PersistentFlags().StringVarP(&dockerfileGenerator, "dockerfile-generator", "", "draft", "Which generator to use for the Dockerfile, options: draft, none")
	generateCmd.PersistentFlags().BoolVarP(&generateSnapshot, "snapshot", "s", false, "Generate a snapshot of the Dockerfile and Kubernetes manifests generated in each iteration")

	// Setup command flags
	setupCmd.PersistentFlags().StringVarP(&resourceGroup, "resource-group", "g", "", "Azure resource group")
	setupCmd.PersistentFlags().StringVarP(&location, "location", "l", "", "Azure region for the resource group")
	setupCmd.PersistentFlags().StringVarP(&openaiResourceName, "openai-resource", "a", "", "Azure OpenAI Cognitive Services resource name")
	setupCmd.PersistentFlags().StringVarP(&deploymentName, "deployment", "d", "", "Deployment name")
	setupCmd.PersistentFlags().StringVarP(&modelID, "model-id", "m", "o3-mini", "Model ID")
	setupCmd.PersistentFlags().StringVarP(&modelVersion, "model-version", "v", "2025-01-31", "Model version")
	setupCmd.PersistentFlags().StringVarP(&targetRepo, "target-repo", "t", "", "Path to the repo to containerize")
	setupCmd.PersistentFlags().Bool("force-setup", false, "Force setup even if environment variables are already set")
}

// Add a helper function to mask secrets for debug output
func maskSecret(s string) string {
	if s == "" {
		return "<empty>"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
