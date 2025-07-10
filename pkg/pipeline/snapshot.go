package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Azure/container-kit/pkg/logger"
	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ReportDirectory is the directory where the iteration snapshots will be stored along with a report of the run
const ReportDirectory = ".container-kit"

// WriteIterationSnapshot creates a snapshot of the current pipeline iteration.
// The function accepts a variadic parameter `stages`, which is a list of PipelineStage objects.
// Each stage can contribute its errors to the snapshot, which are included in the metadata.
func WriteIterationSnapshot(state *PipelineState, targetDir string, snapshotCompletions bool, stages ...PipelineStage) error {
	snapDir := filepath.Join(targetDir, ReportDirectory, fmt.Sprintf("iteration_%d", state.IterationCount))
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return mcperrors.NewError().Message("creating container-kit-snapshot directory").Cause(err).WithLocation(

		// Collect errors from all stages
		).Build()
	}

	stageErrors := make(map[string]string)

	// Each stage can contribute its errors
	for _, s := range stages {
		if s == nil {
			continue
		}

		// Use the pipeline base type name as a key prefix
		typeName := reflect.TypeOf(s).Elem().Name()
		key := fmt.Sprintf("%s_errors", typeName)
		stageErrors[key] = s.GetErrors(state)
	}

	// Build metadata including errors
	meta := map[string]interface{}{
		"iteration":    state.IterationCount,
		"success":      state.Success,
		"metadata":     state.Metadata,
		"registry_url": state.RegistryURL,
		"image_name":   state.ImageName,
		"errors":       stageErrors,
	}

	// For backward compatibility, also include specific error fields
	// if we can identify Docker and Manifest pipelines
	for k, v := range stageErrors {
		if strings.Contains(k, "DockerPipeline") {
			meta["docker_errors"] = v
		} else if strings.Contains(k, "ManifestPipeline") {
			meta["manifest_errors"] = v
		}
	}
	if snapshotCompletions && len(state.LLMCompletions) > 0 {
		meta["llm_completions"] = state.LLMCompletions

		completionsJSON, err := json.MarshalIndent(state.LLMCompletions, "", "  ")
		if err != nil {
			return mcperrors.NewError().Message("marshal LLM completions").Cause(err).WithLocation().Build()
		}

		path := filepath.Join(snapDir, "llm_completions.json")
		if err := os.WriteFile(path, completionsJSON, 0644); err != nil {
			return mcperrors.NewError().Message("write llm_completions.json").Cause(err).WithLocation().Build()
		}
	}

	metaJson, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return mcperrors.NewError().Message("marshaling metadata to JSON").Cause(err).WithLocation().Build()
	}
	if err := os.WriteFile(filepath.Join(snapDir, "metadata.json"), metaJson, 0644); err != nil {
		return mcperrors.NewError().Message("writing metadata.json").Cause(err).WithLocation().Build()
	}

	if state.Dockerfile.Content != "" {
		dockerPath := filepath.Join(snapDir, "Dockerfile")
		if err := os.WriteFile(dockerPath, []byte(state.Dockerfile.Content), 0644); err != nil {
			return mcperrors.NewError().Message("writing Dockerfile snapshot").Cause(err).WithLocation().Build()
		}
	}

	manifestDir := filepath.Join(snapDir, "manifests")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return mcperrors.NewError().Message("creating manifest directory").Cause(err).WithLocation().Build()
	}

	for name, obj := range state.K8sObjects {
		if obj.Content == nil {
			continue
		}
		path := filepath.Join(manifestDir, name+".yaml")
		if err := os.WriteFile(path, []byte(obj.Content), 0644); err != nil {
			return mcperrors.NewError().Message("writing manifest snapshot").Cause(err).WithLocation().Build()
		}
	}

	logger.Infof("Snapshot for iteration %d saved to %s\n", state.IterationCount, snapDir)
	return nil
}
