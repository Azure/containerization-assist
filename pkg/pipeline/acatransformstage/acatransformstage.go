package acatransformstage

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/azureaca"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// Ensure interface compliance
var _ pipeline.PipelineStage = &ACATransformStage{}

// ACATransformStage converts an exported Azure Container App definition into
// Kubernetes manifests and stores them on PipelineState.
// It assumes Dockerfile generation is not needed; the existing image is reused.
type ACATransformStage struct{}

func (s *ACATransformStage) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// nothing to init
	return nil
}

func (s *ACATransformStage) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	rawPath, ok := state.Metadata[pipeline.UserACAConfigPathKey].(string)
	if !ok || rawPath == "" {
		return fmt.Errorf("ACATransformStage: metadata key %s not set", pipeline.UserACAConfigPathKey)
	}

	cfg, err := azureaca.ParseACAJSON(rawPath)
	if err != nil {
		return err
	}

	if cfg.Image == "" {
		return fmt.Errorf("ACATransformStage: ACAConfig image is empty – cannot continue")
	}

	// Create a short summary for downstream stages / LLM prompts
	summary := fmt.Sprintf("ACA spec imported from %s with %d env vars; port %d; replicas %d; ingress=%v", rawPath, len(cfg.Env), cfg.Port, cfg.Replicas, cfg.Ingress)
	state.Metadata[pipeline.ACAAnalysisSummaryKey] = summary

	objs := azureaca.GenerateK8sObjects(cfg)
	state.K8sObjects = objs

	// Write manifests immediately so they appear in snapshots and can be
	// picked up by manifeststage without extra Generate step.
	if err := k8s.WriteK8sObjectsToFiles(objs, targetDir); err != nil {
		return err
	}

	logger.Infof("Generated %d Kubernetes manifest(s) from ACA export", len(objs))
	return nil
}

func (s *ACATransformStage) GetErrors(state *pipeline.PipelineState) string {
	return "" // no iterative errors yet
}

func (s *ACATransformStage) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	// Files already written during Generate.
	return nil
}

func (s *ACATransformStage) Run(ctx context.Context, state *pipeline.PipelineState, _ interface{}, _ pipeline.RunnerOptions) error {
	// No iteration: manifeststage will handle apply/verify cycles.
	return nil
}

func (s *ACATransformStage) Deploy(ctx context.Context, state *pipeline.PipelineState, _ interface{}) error {
	// Deployment delegated to manifeststage.
	return nil
}
