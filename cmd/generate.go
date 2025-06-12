package cmd

import (
	"context"
	"encoding/json"
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
	"github.com/Azure/container-copilot/pkg/pipeline/databasedetectionstage"
	"github.com/Azure/container-copilot/pkg/pipeline/dockerstage"
	"github.com/Azure/container-copilot/pkg/pipeline/manifeststage"
	"github.com/Azure/container-copilot/pkg/pipeline/repoanalysisstage"
)

func generate(ctx context.Context, targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, c *clients.Clients, extraContext string) error {
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
		TargetDirectory:           targetDir,
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
	if generateSnapshot {
		report := NewReport(ctx, state, targetDir)
		reportJSON, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			logger.Warnf("Error marshalling stage history: %v", err)
		}
		reportFile := filepath.Join(targetDir, pipeline.ReportDirectory, "run_report.json")
		logger.Debugf("Writing stage history to %s", reportFile)
		if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
			logger.Errorf("Error writing stage history to file: %v", err)
		}
	}

	if err != nil {
		return fmt.Errorf("running pipeline: %w", err)
	}

	logger.Infof("Total Token usage: Prompt: %d, Completion: %d,  Total: %d\n", state.TokenUsage.PromptTokens, state.TokenUsage.CompletionTokens, state.TokenUsage.TotalTokens)
	return nil
}

func NewReport(ctx context.Context, state *pipeline.PipelineState, targetDir string) *RunReport {
	outcome := RunOutcomeSuccess
	// if deadline exceeded or canceled, set outcome to timeout
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		outcome = RunOutcomeTimeout
	}
	if !state.Success {
		outcome = RunOutcomeFailure
	}
	return &RunReport{
		IterationCount: state.IterationCount,
		Outcome:        outcome,
		StageHistory:   state.StageHistory,
	}
}

type RunOutcome string

const (
	RunOutcomeSuccess RunOutcome = "success"
	RunOutcomeFailure RunOutcome = "failure"
	RunOutcomeTimeout RunOutcome = "timeout"
)

type RunReport struct {
	IterationCount int                   `json:"iteration_count"`
	Outcome        RunOutcome            `json:"outcome"`
	StageHistory   []pipeline.StageVisit `json:"stage_history"`
}

func init() {
	generateCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "localhost:5001", "Docker registry to push the image to")
	generateCmd.PersistentFlags().StringVarP(&dockerfileGenerator, "dockerfile-generator", "", "draft", "Which generator to use for the Dockerfile, options: draft, none")
	generateCmd.PersistentFlags().BoolVarP(&generateSnapshot, "snapshot", "s", false, "Generate a snapshot of the Dockerfile and Kubernetes manifests generated in each iteration")
	generateCmd.PersistentFlags().StringVarP(&targetRepo, "target-repo", "t", "", "Path to the repo to containerize")
	generateCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 10*time.Minute, "Timeout duration for generating artifacts")
	generateCmd.PersistentFlags().IntVarP(&maxDepth, "max-depth", "d", 3, "Maximum depth for file tree scan of target repository. Set to -1 for entire repo.")
	generateCmd.PersistentFlags().StringVarP(&extraContext, "context", "c", "", "Extra context to pass to the AI model, e.g., 'This is a SpringBoot app'")
}
