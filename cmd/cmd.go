package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/ai"
	"github.com/Azure/containerization-assist/pkg/common/logger"
	"github.com/Azure/containerization-assist/pkg/common/runner"
	"github.com/Azure/containerization-assist/pkg/core/docker"
	"github.com/Azure/containerization-assist/pkg/core/kind"
	"github.com/Azure/containerization-assist/pkg/core/kubernetes"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

const (
	AZURE_OPENAI_KEY           = "AZURE_OPENAI_KEY"
	AZURE_OPENAI_ENDPOINT      = "AZURE_OPENAI_ENDPOINT"
	AZURE_OPENAI_DEPLOYMENT_ID = "AZURE_OPENAI_DEPLOYMENT_ID"
	ENV_FILE_NAME              = ".env"
)

var (
	registry            string
	dockerfileGenerator string
	generateSnapshot    bool
	snapshotCompletions bool
	generateReport      bool
	timeout             time.Duration
	maxDepth            int
	extraContext        string

	// Setup command variables
	resourceGroup      string
	location           string
	openaiResourceName string
	deploymentName     string
	modelID            string
	modelVersion       string
	targetRepo         string
	verbose            bool
	forceSetup         bool
)

var rootCmd = &cobra.Command{
	Use:   "containerization-assist",
	Short: "An AI-Powered CLI tool to containerize your app and generate Kubernetes artifacts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if verbose {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
	},
}

func getProjectRoot() string {
	execPath, err := os.Executable()
	if err != nil {
		logger.Warnf("Warning: Error getting executable path: %v", err)
		return "."
	}
	return filepath.Dir(execPath)
}

// loadEnvFile attempts to load the .env file from the project root
func loadEnvFile() {
	projectRoot := getProjectRoot()
	envFile := filepath.Join(projectRoot, ENV_FILE_NAME)

	// Check if .env file exists and load it
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			logger.Warnf("Warning: Error loading .env file: %v", err)
		}
	}
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Azure OpenAI connection",
	Long:  `The test command will test the Azure OpenAI connection based on the environment variables set and print a response.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		// Load environment variables from .env file
		loadEnvFile()

		c, err := initClients(ctx)
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}

		if err := ai.TestOpenAIConn(ctx, c.AzOpenAIClient); err != nil {
			return fmt.Errorf("error testing Azure OpenAI connection: %w", err)
		}

		return nil
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up Azure OpenAI resources and run containerization-assist",
	Long:  `The setup command will provision Azure OpenAI resources, deploy the model, and run containerization-assist to generate artifacts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot := getProjectRoot()

		// Check force-setup flag first, before loading .env file
		forceSetup, _ := cmd.Flags().GetBool("force-setup")
		envFile := filepath.Join(projectRoot, ENV_FILE_NAME)
		if forceSetup {
			logger.Info("Force setup enabled - deleting existing .env file and proceeding with fresh setup...")
			// Delete existing .env file if it exists
			if _, err := os.Stat(envFile); err == nil {
				if err := os.Remove(envFile); err != nil {
					logger.Warnf("Warning: Failed to delete existing .env file: %v", err)
				} else {
					logger.Info("  ✓ Deleted existing .env file")
				}
			}
			// Do NOT load .env file when force-setup is enabled
		} else {
			// Load any existing environment variables from .env file only if not force-setup
			loadEnvFile()
		}

		// Load configuration from environment, flags, etc.
		config, err := LoadSetupConfig(cmd, args, projectRoot)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Validate configuration
		if err := config.ValidateConfig(); err != nil {
			return err
		}

		// Print configuration
		config.PrintConfig()

		// Run the setup process
		apiKey, endpoint, deploymentID, err := RunSetup(config)
		if err != nil {
			return err
		}

		// Set environment variables for this process
		os.Setenv(AZURE_OPENAI_KEY, apiKey)
		os.Setenv(AZURE_OPENAI_ENDPOINT, endpoint)
		os.Setenv(AZURE_OPENAI_DEPLOYMENT_ID, deploymentID)

		// Update .env file
		if err := UpdateEnvFile(projectRoot, config, apiKey, endpoint, deploymentID); err != nil {
			logger.Warnf("Warning: Failed to update .env file: %v", err)
		} else {
			logger.Infof("Updated .env file at %s", filepath.Join(projectRoot, ".env"))
			logger.Infof("Azure OpenAI Key: %s", maskSecretValue(apiKey))
			logger.Infof("Azure OpenAI Endpoint: %s", endpoint)
			logger.Infof("Azure OpenAI Deployment ID: %s", deploymentID)
			logger.Infof("Target Repo: %s", config.TargetRepo)
		}

		// Setup completed successfully
		logger.Info("\n✅ Setup completed successfully!")

		// Display next steps instead of running generate automatically
		if config.TargetRepo != "" {
			logger.Infof("\nTo generate artifacts, run: containerization-assist generate %s", config.TargetRepo)
		} else {
			logger.Info("\nTo generate artifacts, run:")
			logger.Info("  containerization-assist generate <path/to/target-repo>")
		}

		return nil
	},
}

func Execute() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.ExecuteContext(context.Background())
}

func initClients(ctx context.Context) (*Clients, error) {
	// Try to load values from .env file first
	loadEnvFile()

	// Now check environment variables (which now include any from .env file)
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
		// Instead of returning an error, try to run setup automatically
		logger.Infof("Missing environment variables: %s", strings.Join(missingVars, ", "))
		logger.Info("Attempting to set up Azure OpenAI resources automatically...")

		// Run setup process
		if err := runAutoSetup(); err != nil {
			return nil, fmt.Errorf("automatic setup failed: %w\nPlease run 'containerization-assist setup' manually or provide the environment variables", err)
		}

		// After setup, reload environment variables
		apiKey = os.Getenv(AZURE_OPENAI_KEY)
		endpoint = os.Getenv(AZURE_OPENAI_ENDPOINT)
		deploymentID = os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID)
	}

	azOpenAIClient, err := ai.NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure OpenAI client: %w", err)
	}

	// After ensuring env vars are present, validate the LLM configuration
	llmConfig := ai.LLMConfig{
		Endpoint:       endpoint,
		APIKey:         apiKey,
		DeploymentID:   deploymentID,
		AzOpenAIClient: azOpenAIClient, // This client is correctly set here for validation
	}

	if err := ai.ValidateLLM(ctx, llmConfig); err != nil {
		return nil, fmt.Errorf("LLM configuration validation failed: %w", err)
	}

	cmdRunner := &runner.DefaultCommandRunner{}

	clients := &Clients{
		AzOpenAIClient: azOpenAIClient,
		Docker:         docker.NewDockerCmdRunner(cmdRunner),
		Kind:           kind.NewKindCmdRunner(cmdRunner),
		Kube:           kubernetes.NewKubeCmdRunner(cmdRunner),
	}

	return clients, nil
}

// runAutoSetup runs the setup process automatically with minimal user input
func runAutoSetup() error {
	// Load any existing environment variables from .env file
	loadEnvFile()
	projectRoot := getProjectRoot()

	// Create a temporary command to load config
	tempCmd := &cobra.Command{}
	tempCmd.Flags().String("target-repo", "", "")

	// Check if target repo is already available in environment
	envTargetRepo := os.Getenv("TARGET_REPO")
	if envTargetRepo != "" {
		tempCmd.Flags().Set("target-repo", envTargetRepo)
		logger.Infof("Using target repository from environment: %s", envTargetRepo)
	}

	// Empty args list
	var args []string

	// Load configuration with defaults
	config, err := LoadSetupConfig(tempCmd, args, projectRoot)
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// If target repo still not set, prompt the user
	if config.TargetRepo == "" {
		logger.Info("A target repository path is required for containerization.")
		fmt.Print("Enter path to the repository to containerize: ")
		var targetRepo string
		fmt.Scanln(&targetRepo)

		if targetRepo == "" {
			return fmt.Errorf("target repository is required")
		}

		// Use the normalized path utility
		normalizedPath, err := NormalizeTargetRepoPath(targetRepo)
		if err != nil {
			return err
		}
		config.TargetRepo = normalizedPath
	}

	logger.Info("Using auto-generated resource names for Azure OpenAI setup...")

	// Print configuration
	config.PrintConfig()

	// Run the setup process
	apiKey, endpoint, deploymentID, err := RunSetup(config)
	if err != nil {
		return err
	}

	// Set environment variables for this process
	os.Setenv(AZURE_OPENAI_KEY, apiKey)
	os.Setenv(AZURE_OPENAI_ENDPOINT, endpoint)
	os.Setenv(AZURE_OPENAI_DEPLOYMENT_ID, deploymentID)

	// Update .env file
	if err := UpdateEnvFile(projectRoot, config, apiKey, endpoint, deploymentID); err != nil {
		logger.Warnf("Warning: Failed to update .env file: %v", err)
	} else {
		logger.Infof("Updated .env file at %s", filepath.Join(projectRoot, ".env"))
		logger.Infof("Azure OpenAI Key: %s", maskSecretValue(apiKey))
		logger.Infof("Azure OpenAI Endpoint: %s", endpoint)
		logger.Infof("Azure OpenAI Deployment ID: %s", deploymentID)
		logger.Infof("Target Repo: %s", config.TargetRepo)
	}

	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
