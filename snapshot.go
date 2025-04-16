package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeIterationSnapshot(stateHistory []PipelineState, targetDir string) error {

	for i := 0; i < len(stateHistory); i++ {
		state := stateHistory[i]
		snapDir := filepath.Join(targetDir, ".snapshots", fmt.Sprintf("iteration_%d", i+1))
		if err := os.MkdirAll(snapDir, 0755); err != nil {
			return fmt.Errorf("creating snapshot directory: %w", err)
		}

		meta := map[string]interface{}{
			"iteration":       state.IterationCount,
			"success":         state.Success,
			"metadata":        state.Metadata,
			"registry_url":    state.RegistryURL,
			"image_name":      state.ImageName,
			"docker_errors":   state.Dockerfile.BuildErrors,
			"manifest_errors": FormatManifestErrors(&state),
		}

		metaJson, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(snapDir, "metadata.json"), metaJson, 0644)

		if state.Dockerfile.Content != "" {
			dockerPath := filepath.Join(snapDir, "Dockerfile")
			if err := os.WriteFile(dockerPath, []byte(state.Dockerfile.Content), 0644); err != nil {
				return fmt.Errorf("writing Dockerfile snapshot: %w", err)
			}
		}

		manifestDir := filepath.Join(snapDir, "manifests")
		os.MkdirAll(manifestDir, 0755)

		for name, obj := range state.K8sObjects {
			if obj.Content == nil {
				continue
			}
			path := filepath.Join(manifestDir, name+".yaml")
			if err := os.WriteFile(path, []byte(obj.Content), 0644); err != nil {
				return fmt.Errorf("writing manifest snapshot: %w", err)
			}
		}
	}

	return nil
}

func DeepCopy(ps *PipelineState) PipelineState {
	clone := *ps

	clone.Dockerfile = Dockerfile{
		Content:     ps.Dockerfile.Content,
		Path:        ps.Dockerfile.Path,
		BuildErrors: ps.Dockerfile.BuildErrors,
	}

	clone.K8sObjects = make(map[string]*K8sObject, len(ps.K8sObjects))
	for k, v := range ps.K8sObjects {
		if v != nil {
			objCopy := *v
			if v.Content != nil {
				objCopy.Content = make([]byte, len(v.Content))
				copy(objCopy.Content, v.Content)
			}
			clone.K8sObjects[k] = &objCopy
		}
	}

	clone.Metadata = make(map[string]interface{}, len(ps.Metadata))
	for k, v := range ps.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}
