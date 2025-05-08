package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/runner"
	llmvalidator "github.com/Azure/container-copilot/pkg/utils"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
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
	timeout             time.Duration

	// Setup command variables
	resourceGroup      string
	location           string
	openaiResourceName string
	deploymentName     string
	modelID            string
	modelVersion       string
	targetRepo         string

	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "container-copilot",
	Short: "An AI-Powered CLI tool to containerize your app and generate Kubernetes artifacts",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
	},
}

// loadEnvFile attempts to load the .env file from the project root
func loadEnvFile() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		projectRoot := filepath.Dir(filepath.Dir(file))
		envFile := filepath.Join(projectRoot, ".env")

		// Check if .env file exists and load it
		if _, err := os.Stat(envFile); err == nil {
			if err := godotenv.Load(envFile); err != nil {
				logger.Warnf("Warning: Error loading .env file: %v\n", err)
			}
		}
	}
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Dockerfile and Kubernetes manifests",
	Long:  `The generate command will add Dockerfile and Kubernetes manifests to your project based on the project structure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		// Try to load .env file first to get environment variables
		loadEnvFile()

		// Determine target directory from multiple sources in order of priority:
		// 1. Command line argument
		// 2. --target-repo flag
		// 3. CCP_TARGET_REPO environment variable (which would include values from .env)
		// 4. Interactive prompt

		var targetDir string

		// Check command line argument first
		if len(args) > 0 {
			targetDir = args[0]
			// Set it in the environment so auto-setup can find it later
			os.Setenv("CCP_TARGET_REPO", targetDir)
		} else {
			// Check flag
			targetFlag, _ := cmd.Flags().GetString("target-repo")
			if targetFlag != "" {
				targetDir = targetFlag
				// Set it in the environment so auto-setup can find it later
				os.Setenv("CCP_TARGET_REPO", targetDir)
			} else {
				// Check environment variable (includes .env file)
				targetDir = os.Getenv("CCP_TARGET_REPO")

				// If still no target, prompt the user
				if targetDir == "" {
					// No target directory provided - inform the user and accept input
					logger.Warn("No target repository specified. The target repository is the directory containing the application you want to containerize.")
					logger.Info("Example: container-copilot generate ./my-app")

					// Ask if they want to provide a target directory now
					logger.Info("Would you like to specify a target repository now? (y/n): ")
					var response string
					fmt.Scanln(&response)

					if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
						fmt.Print("Enter path to the repository to containerize: ")
						fmt.Scanln(&targetDir)

						if targetDir == "" {
							return fmt.Errorf("target repository is required")
						}
						// Set it in the environment so auto-setup can find it later
						os.Setenv("CCP_TARGET_REPO", targetDir)
					} else {
						return fmt.Errorf("target repository is required")
					}
				} else {
					logger.Infof("Using target repository from environment: %s\n", targetDir)
				}
			}
		}

		// Check if Azure OpenAI environment variables are set
		if os.Getenv(AZURE_OPENAI_KEY) == "" ||
			os.Getenv(AZURE_OPENAI_ENDPOINT) == "" ||
			os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID) == "" {
			logger.Error("Azure OpenAI configuration not found. Starting automatic setup process...")
		}

		// Lets check if the Key, Endpoint and deployment are actually valid
		// Validate the LLM configuration
		llmConfig := llmvalidator.LLMConfig{
			Endpoint:     os.Getenv(AZURE_OPENAI_ENDPOINT), // "https://xxx.openai.azure.com",
			APIKey:       os.Getenv(AZURE_OPENAI_KEY),
			DeploymentID: os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID),
		}

		if err := llmvalidator.ValidateLLM(llmConfig); err != nil {
			logger.Errorf("LLM config is invalid: %v\n", err)
		} else {
			logger.Infof("LLM config validated successfully.")
		}
		// Convert targetDir to absolute path for consistent behavior
		if targetDir != "" {
			normalizedPath, err := NormalizeTargetRepoPath(targetDir)
			if err != nil {
				return err
			}
			targetDir = normalizedPath
		}

		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := generate(ctx, targetDir, registry, dockerfileGenerator == "draft", generateSnapshot, c); err != nil {
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
		ctx := cmd.Context()
		// Load environment variables from .env file
		loadEnvFile()

		c, err := initClients()
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}
		if err := c.TestOpenAIConn(ctx); err != nil {
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
		// Load any existing environment variables from .env file
		loadEnvFile()

		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("failed to determine source file location")
		}

		// Get the project root directory
		projectRoot := filepath.Dir(filepath.Dir(file))

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
			logger.Warnf("Warning: Failed to update .env file: %v\n", err)
		} else {
			logger.Infof("Updated .env file at %s\n", filepath.Join(projectRoot, ".env"))
			logger.Infof("Azure OpenAI Key: %s\n", maskSecretValue(apiKey))
			logger.Infof("Azure OpenAI Endpoint: %s\n", endpoint)
			logger.Infof("Azure OpenAI Deployment ID: %s\n", deploymentID)
			logger.Infof("Target Repo: %s\n", config.TargetRepo)
		}

		// Setup completed successfully
		logger.Info("\n✅ Setup completed successfully!")

		// Display next steps instead of running generate automatically
		if config.TargetRepo != "" {
			logger.Infof("\nTo generate artifacts, run:\n  container-copilot generate %s\n", config.TargetRepo)
		} else {
			logger.Info("\nTo generate artifacts, run:")
			logger.Info("  container-copilot generate <path/to/target-repo>")
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

func initClients() (*clients.Clients, error) {
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
		logger.Infof("Missing environment variables: %s\n", strings.Join(missingVars, ", "))
		logger.Info("Attempting to set up Azure OpenAI resources automatically...")

		// Run setup process
		if err := runAutoSetup(); err != nil {
			return nil, fmt.Errorf("automatic setup failed: %w\nPlease run 'container-copilot setup' manually or provide the environment variables", err)
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

	cmdRunner := &runner.DefaultCommandRunner{}

	clients := &clients.Clients{
		AzOpenAIClient: azOpenAIClient,
		Docker:         docker.NewDockerCmdRunner(cmdRunner),
		Kind:           kind.NewKindCmdRunner(cmdRunner),
		Kube:           k8s.NewKubeCmdRunner(cmdRunner),
	}

	return clients, nil
}

// runAutoSetup runs the setup process automatically with minimal user input
func runAutoSetup() error {
	// Load any existing environment variables from .env file
	loadEnvFile()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to determine source file location")
	}

	// Get the project root directory
	projectRoot := filepath.Dir(filepath.Dir(file))

	// Create a temporary command to load config
	tempCmd := &cobra.Command{}
	tempCmd.Flags().String("target-repo", "", "")

	// Check if target repo is already available in environment
	envTargetRepo := os.Getenv("CCP_TARGET_REPO")
	if envTargetRepo != "" {
		tempCmd.Flags().Set("target-repo", envTargetRepo)
		logger.Infof("Using target repository from environment: %s\n", envTargetRepo)
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
		logger.Warnf("Warning: Failed to update .env file: %v\n", err)
	} else {
		logger.Infof("Updated .env file at %s\n", filepath.Join(projectRoot, ".env"))
		logger.Infof("Azure OpenAI Key: %s\n", maskSecretValue(apiKey))
		logger.Infof("Azure OpenAI Endpoint: %s\n", endpoint)
		logger.Infof("Azure OpenAI Deployment ID: %s\n", deploymentID)
		logger.Infof("Target Repo: %s\n", config.TargetRepo)
	}

	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
