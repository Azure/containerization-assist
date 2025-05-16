package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/filetree"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/Azure/container-copilot/pkg/pipeline/dockerpipeline"
	"github.com/Azure/container-copilot/pkg/pipeline/manifestpipeline"
	"github.com/Azure/container-copilot/pkg/pipeline/repoanalysispipeline"
)

func generate(ctx context.Context, targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, c *clients.Clients) error {
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

	// Initialize pipeline state
	state := &pipeline.PipelineState{
		K8sObjects:     make(map[string]*k8s.K8sObject),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[pipeline.MetadataKey]any),
		ImageName:      "app", // TODO: clean up app naming into state
		RegistryURL:    registry,
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
		TargetDirectory:           targetDir,
	}

	runner := pipeline.NewRunner([]*pipeline.StageConfig{
		{
			Id:   "analysis",
			Path: targetDir,
			Stage: &repoanalysispipeline.RepoAnalysisStage{
				AIClient: c.AzOpenAIClient,
				Parser:   &pipeline.DefaultParser{},
			},
		},
		{
			Id:         "docker",
			MaxRetries: 5,
			Path:       filepath.Join(targetDir, "Dockerfile"),
			Stage: &dockerpipeline.DockerStage{
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
			Stage: &manifestpipeline.ManifestStage{
				AIClient: c.AzOpenAIClient,
				Parser:   &pipeline.DefaultParser{},
			},
		},
	}, os.Stdout)
	err = runner.Run(ctx, state, options, c)
	if err != nil {
		return err
	}

	logger.Infof("Total Token usage: Prompt: %d, Completion: %d,  Total: %d\n", state.TokenUsage.PromptTokens, state.TokenUsage.CompletionTokens, state.TokenUsage.TotalTokens)
	return nil
}

func init() {
	generateCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "localhost:5001", "Docker registry to push the image to")
	generateCmd.PersistentFlags().StringVarP(&dockerfileGenerator, "dockerfile-generator", "", "draft", "Which generator to use for the Dockerfile, options: draft, none")
	generateCmd.PersistentFlags().BoolVarP(&generateSnapshot, "snapshot", "s", false, "Generate a snapshot of the Dockerfile and Kubernetes manifests generated in each iteration")
	generateCmd.PersistentFlags().StringVarP(&targetRepo, "target-repo", "t", "", "Path to the repo to containerize")
	generateCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 10*time.Minute, "Timeout duration for generating artifacts")
	generateCmd.PersistentFlags().IntVarP(&maxDepth, "max-depth", "d", 3, "Maximum depth for file tree scan of target repository. Set to -1 for entire repo.")
}
