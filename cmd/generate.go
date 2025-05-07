package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	// Check for kind cluster before starting
	kindClusterName, err := c.GetKindCluster()
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	logger.Infof("Using kind cluster: %s\n", kindClusterName)

	// Validate registry connection
	logger.Infof("Validating connection to registry %s\n", registry)
	err = docker.ValidateRegistryReachable(registry)
	if err != nil {
		return fmt.Errorf("reaching registry %s: %w\n", registry, err)
	}

	// Initialize pipeline state
	state := &pipeline.PipelineState{
		K8sObjects:     make(map[string]*k8s.K8sObject),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[string]interface{}),
		ImageName:      "app", // TODO: clean up app naming into state
		RegistryURL:    registry,
	}

	// Get file tree structure for context
	repoStructure, err := filetree.ReadFileTree(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %w", err)
	}
	state.RepoFileTree = repoStructure

	registryAndImage := fmt.Sprintf("%s/%s", registry, state.ImageName)
	if err := docker.GenerateDeploymentFilesWithDraft(targetDir, registryAndImage); err != nil {
		return fmt.Errorf("generating deployment files: %w", err)
	}

	repoAnalysisPipeline := &repoanalysispipeline.RepoAnalysisPipeline{
		AIClient: c.AzOpenAIClient,
		Parser:   &pipeline.DefaultParser{},
	}
	dockerPipeline := &dockerpipeline.DockerPipeline{
		AIClient:         c.AzOpenAIClient,
		UseDraftTemplate: enableDraftDockerfile,
		Parser:           &pipeline.DefaultParser{},
	}
	manifestPipeline := &manifestpipeline.ManifestPipeline{
		AIClient: c.AzOpenAIClient,
		Parser:   &pipeline.DefaultParser{},
	}

	pipelinesByType := map[string]pipeline.Pipeline{
		"repoanalysis": repoAnalysisPipeline,
		"docker":       dockerPipeline,
		"manifest":     manifestPipeline,
	}

	// Create path map for each pipeline
	pathMap := map[string]string{
		"repoanalysis": targetDir,
		"docker":       filepath.Join(targetDir, "Dockerfile"),
		"manifest":     targetDir,
	}

	// Common pipeline options
	options := pipeline.RunnerOptions{
		MaxIterations:             5, // Default max iterations
		CompleteLoopMaxIterations: 2, // Default max iterations for the entire loop
		GenerateSnapshot:          generateSnapshot,
		TargetDirectory:           targetDir,
	}

	// Update execution order to include repo analysis as the first step
	execOrder := []string{"repoanalysis", "docker", "manifest"}

	runner := pipeline.NewRunner(pipelinesByType, execOrder, os.Stdout)
	return runner.Run(ctx, state, pathMap, options, c)
}
