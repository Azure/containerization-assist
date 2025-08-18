package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/filesystem"
	"github.com/Azure/containerization-assist/pkg/common/logger"
	"github.com/Azure/containerization-assist/pkg/core/docker"
	"github.com/Azure/containerization-assist/pkg/core/kubernetes"
	"github.com/Azure/containerization-assist/pkg/pipeline"
	"github.com/Azure/containerization-assist/pkg/pipeline/databasedetectionstage"
	"github.com/Azure/containerization-assist/pkg/pipeline/dockerstage"
	"github.com/Azure/containerization-assist/pkg/pipeline/manifeststage"
	"github.com/Azure/containerization-assist/pkg/pipeline/repoanalysisstage"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Dockerfile and Kubernetes manifests",
	Long:  `The generate command will add Dockerfile and Kubernetes manifests to your project based on the project structure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		if generateSnapshot {
			logger.Debugf("Running with snapshot generation and run report enabled")
		}
		// Try to load .env file first to get environment variables
		loadEnvFile()

		// Determine target directory from multiple sources in order of priority:
		// 1. Command line argument
		// 2. --target-repo flag
		// 3. TARGET_REPO environment variable (which would include values from .env)
		// 4. Interactive prompt

		var targetDir string

		// Check command line argument first
		if len(args) > 0 {
			targetDir = args[0]
			// Set it in the environment so auto-setup can find it later
			os.Setenv("TARGET_REPO", targetDir)
		} else {
			// Check flag
			targetFlag, _ := cmd.Flags().GetString("target-repo")
			if targetFlag != "" {
				targetDir = targetFlag
				// Set it in the environment so auto-setup can find it later
				os.Setenv("TARGET_REPO", targetDir)
			} else {
				// Check environment variable (includes .env file)
				targetDir = os.Getenv("TARGET_REPO")

				// If still no target, prompt the user
				if targetDir == "" {
					// No target directory provided - inform the user and accept input
					logger.Warn("No target repository specified. The target repository is the directory containing the application you want to containerize.")
					logger.Info("Example: containerization-assist generate ./my-app")

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
						os.Setenv("TARGET_REPO", targetDir)
					} else {
						return fmt.Errorf("target repository is required")
					}
				} else {
					logger.Infof("Using target repository from environment: %s", targetDir)
				}
			}
		}

		// Check if Azure OpenAI environment variables are set
		if os.Getenv(AZURE_OPENAI_KEY) == "" ||
			os.Getenv(AZURE_OPENAI_ENDPOINT) == "" ||
			os.Getenv(AZURE_OPENAI_DEPLOYMENT_ID) == "" {
			logger.Warn("Azure OpenAI configuration not found. Starting automatic setup process...")
		}

		// Convert targetDir to absolute path for consistent behavior
		if targetDir != "" {
			normalizedPath, err := NormalizeTargetRepoPath(targetDir)
			if err != nil {
				return err
			}
			targetDir = normalizedPath
		}

		// Validate that the directory exists
		info, err := os.Stat(targetDir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("targetDir %q does not exist", targetDir)
			}
			return fmt.Errorf("error checking targetDir %q: %w", targetDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("targetDir %q is not a directory", targetDir)
		}

		c, err := initClients(ctx)
		if err != nil {
			return fmt.Errorf("error initializing Azure OpenAI client: %w", err)
		}

		if err := generate(ctx, targetDir, registry, dockerfileGenerator == "draft", generateSnapshot, generateReport, c, extraContext); err != nil {
			return fmt.Errorf("error generating artifacts: %w", err)
		}

		return nil
	},
}

func generate(ctx context.Context, targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, generateReport bool, c *Clients, extraContext string) error {
	logger.Debugf("Generating artifacts in directory: %s", targetDir)
	// Check for kind cluster before starting
	kindClusterName, err := kubernetes.GetKindCluster(ctx, c.Kind, c.Docker)
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	logger.Infof("Using kind cluster: %s", kindClusterName)

	// Validate registry connection
	logger.Infof("Validating connection to registry %s", registry)
	err = docker.ValidateRegistryReachable(ctx, registry)
	if err != nil {
		return fmt.Errorf("reaching registry %s: %w", registry, err)
	}

	const defaultAppName = "test-app"
	// Derive app name from target directory
	appName := filepath.Base(targetDir)
	if appName == "." || appName == "/" {
		appName = defaultAppName // fallback to default
	}
	// Sanitize app name for Kubernetes (lowercase, alphanumeric + hyphens)
	appName = strings.ToLower(appName)
	appName = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(appName, "-")
	appName = strings.Trim(appName, "-")
	if appName == "" {
		appName = defaultAppName
	}

	// Initialize pipeline state
	state := &pipeline.PipelineState{
		K8sObjects:     make(map[string]*kubernetes.K8sObject),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[pipeline.MetadataKey]any),
		ImageName:      appName,
		RegistryURL:    registry,
		ExtraContext:   extraContext,
	}

	// Get file tree structure for context
	repoStructure, err := filesystem.GenerateJSONFileTree(targetDir, maxDepth)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %w", err)
	}
	state.RepoFileTree = repoStructure
	logger.Debugf("File tree structure:\n%s", repoStructure)

	// Common pipeline options
	options := pipeline.RunnerOptions{
		MaxIterations:             5, // Default max iterations
		CompleteLoopMaxIterations: 2, // Default max iterations for the entire loop
		GenerateSnapshot:          generateSnapshot,
		GenerateReport:            generateReport,
		TargetDirectory:           targetDir,
		SnapshotCompletions:       snapshotCompletions,
	}

	runner := pipeline.NewRunner([]*pipeline.StageConfig{
		{
			Id:   "analysis",
			Path: targetDir,
			Stage: &repoanalysisstage.RepoAnalysisStage{
				AIClient: c.AzOpenAIClient,
				Parser:   &pipeline.DefaultParser{},
			},
		},
		{
			Id:    "database-detection",
			Path:  targetDir,
			Stage: &databasedetectionstage.DatabaseDetectionStage{},
		},
		{
			Id:         "docker",
			MaxRetries: 5,
			Path:       filepath.Join(targetDir, "Dockerfile"),
			Stage: &dockerstage.DockerStage{
				AIClient:         c.AzOpenAIClient,
				UseDraftTemplate: enableDraftDockerfile,
				Parser:           &pipeline.DefaultParser{},
			},
		},
		{
			Id:         "manifest",
			MaxRetries: 5,
			OnFailGoto: "docker",
			Path:       targetDir,
			Stage: &manifeststage.ManifestStage{
				AIClient: c.AzOpenAIClient,
				Parser:   &pipeline.DefaultParser{},
			},
		},
	}, os.Stdout)
	err = runner.Run(ctx, state, options, c)
	if generateReport {
		if err := pipeline.WriteReport(ctx, state, targetDir); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
	}

	if err != nil {
		return fmt.Errorf("running pipeline: %w", err)
	}

	logger.Infof("Total Token usage: Prompt: %d, Completion: %d,  Total: %d\n", state.TokenUsage.PromptTokens, state.TokenUsage.CompletionTokens, state.TokenUsage.TotalTokens)
	return nil
}

// separated generateSnapshot and generateReport flags to make it more customer-friendly
func init() {
	generateCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "localhost:5001", "Docker registry to push the image to")
	generateCmd.PersistentFlags().StringVarP(&dockerfileGenerator, "dockerfile-generator", "", "draft", "Which generator to use for the Dockerfile, options: draft, none")
	generateCmd.PersistentFlags().BoolVarP(&generateSnapshot, "snapshot", "s", false, "Generate a snapshot of the Dockerfile and Kubernetes manifests generated in each iteration")
	generateCmd.PersistentFlags().BoolVarP(&snapshotCompletions, "snapshot-completions", "", false, "Include LLM completions in snapshots")
	generateCmd.PersistentFlags().BoolVarP(&generateReport, "report", "R", false, "Generate final run summary reports (JSON and Markdown).")
	generateCmd.PersistentFlags().StringVarP(&targetRepo, "target-repo", "t", "", "Path to the repo to containerize")
	generateCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 10*time.Minute, "Timeout duration for generating artifacts")
	generateCmd.PersistentFlags().IntVarP(&maxDepth, "max-depth", "d", 3, "Maximum depth for file tree scan of target repository. Set to -1 for entire repo.")
	generateCmd.PersistentFlags().StringVarP(&extraContext, "context", "c", "", "Extra context to pass to the AI model, e.g., 'This is a SpringBoot app'")
}
