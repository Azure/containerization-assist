package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteIterationSnapshot(state *PipelineState, targetDir string) error {

	snapDir := filepath.Join(targetDir, ".container-copilot-snapshots", fmt.Sprintf("iteration_%d", state.IterationCount))
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return fmt.Errorf("creating container-copilot-snapshot directory: %w", err)
	}

	meta := map[string]interface{}{
		"iteration":       state.IterationCount,
		"success":         state.Success,
		"metadata":        state.Metadata,
		"registry_url":    state.RegistryURL,
		"image_name":      state.ImageName,
		"docker_errors":   state.Dockerfile.BuildErrors,
		"manifest_errors": FormatManifestErrors(state),
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
	return nil
}
