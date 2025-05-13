package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
)

// WriteIterationSnapshot creates a snapshot of the current pipeline iteration.
// The function accepts a variadic parameter `pipelines`, which is a list of Pipeline objects.
// Each pipeline can contribute its errors to the snapshot, which are included in the metadata.
func WriteIterationSnapshot(state *PipelineState, targetDir string, pipelines ...PipelineStage) error {
	snapDir := filepath.Join(targetDir, ".container-copilot-snapshots", fmt.Sprintf("iteration_%d", state.IterationCount))
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("creating container-copilot-snapshot directory: %w", err)
	}

	// Collect errors from all pipelines
	pipelineErrors := make(map[string]string)

	// Each pipeline can contribute its errors
	for _, p := range pipelines {
		if p == nil {
			continue
		}

		// Use the pipeline base type name as a key prefix
		typeName := reflect.TypeOf(p).Elem().Name()
		key := fmt.Sprintf("%s_errors", typeName)
		pipelineErrors[key] = p.GetErrors(state)
	}

	// Build metadata including errors
	meta := map[string]interface{}{
		"iteration":    state.IterationCount,
		"success":      state.Success,
		"metadata":     state.Metadata,
		"registry_url": state.RegistryURL,
		"image_name":   state.ImageName,
		"errors":       pipelineErrors,
	}

	// For backward compatibility, also include specific error fields
	// if we can identify Docker and Manifest pipelines
	for k, v := range pipelineErrors {
		if strings.Contains(k, "DockerPipeline") {
			meta["docker_errors"] = v
		} else if strings.Contains(k, "ManifestPipeline") {
			meta["manifest_errors"] = v
		}
	}

	metaJson, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata to JSON: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "metadata.json"), metaJson, 0644); err != nil {
		return fmt.Errorf("writing metadata.json: %w", err)
	}

	if state.Dockerfile.Content != "" {
		dockerPath := filepath.Join(snapDir, "Dockerfile")
		if err := os.WriteFile(dockerPath, []byte(state.Dockerfile.Content), 0644); err != nil {
			return fmt.Errorf("writing Dockerfile snapshot: %w", err)
		}
	}

	manifestDir := filepath.Join(snapDir, "manifests")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	for name, obj := range state.K8sObjects {
		if obj.Content == nil {
			continue
		}
		path := filepath.Join(manifestDir, name+".yaml")
		if err := os.WriteFile(path, []byte(obj.Content), 0644); err != nil {
			return fmt.Errorf("writing manifest snapshot: %w", err)
		}
	}

	logger.Infof("Snapshot for iteration %d saved to %s\n", state.IterationCount, snapDir)
	return nil
}
