package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/clients"
	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/filetree"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/logger"
	"github.com/Azure/container-kit/pkg/pipeline"
	"github.com/Azure/container-kit/pkg/pipeline/databasedetectionstage"
	"github.com/Azure/container-kit/pkg/pipeline/dockerstage"
	"github.com/Azure/container-kit/pkg/pipeline/manifeststage"
	"github.com/Azure/container-kit/pkg/pipeline/repoanalysisstage"
)

func generate(ctx context.Context, targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, generateReport bool, c *clients.Clients, extraContext string) error {
	logger.Debugf("Generating artifacts in directory: %s", targetDir)
	// Check for kind cluster before starting
	kindClusterName, err := c.GetKindCluster(ctx)
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

	// Derive app name from target directory
	appName := filepath.Base(targetDir)
	if appName == "." || appName == "/" {
		appName = "app" // fallback to default
	}
	// Sanitize app name for Kubernetes (lowercase, alphanumeric + hyphens)
	appName = strings.ToLower(appName)
	appName = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(appName, "-")
	appName = strings.Trim(appName, "-")
	if appName == "" {
		appName = "app"
	}

	// Initialize pipeline state
	state := &pipeline.PipelineState{
		K8sObjects:     make(map[string]*k8s.K8sObject),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[pipeline.MetadataKey]any),
		ImageName:      appName,
		RegistryURL:    registry,
		ExtraContext:   extraContext,
	}

	// Get file tree structure for context
	repoStructure, err := filetree.ReadFileTree(targetDir, maxDepth)
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
