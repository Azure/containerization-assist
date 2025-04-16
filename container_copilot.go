package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/filetree"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/runner"
)

// updateSuccessfulFiles writes the successful Dockerfile and manifests from the pipeline state to disk
func updateSuccessfulFiles(state *runner.PipelineState) {
	if state.Success {
		// Write final Dockerfile
		if state.Dockerfile.Path != "" && state.Dockerfile.Content != "" {
			fmt.Printf("Writing final Dockerfile to %s\n", state.Dockerfile.Path)
			if err := os.WriteFile(state.Dockerfile.Path, []byte(state.Dockerfile.Content), 0644); err != nil {
				fmt.Printf("Error writing Dockerfile: %v\n", err)
			}
		}

		// Write final manifests
		fmt.Println("Writing final Kubernetes manifest files...")
		for name, object := range state.K8sObjects {
			if object.IsSuccessfullyDeployed && object.ManifestPath != "" {
				fmt.Printf("Writing updated manifest: %s\n", name)
				// assumes single object per file so we can write the whole content
				if err := os.WriteFile(object.ManifestPath, []byte(object.Content), 0644); err != nil {
					fmt.Printf("Error writing manifest %s: %v\n", name, err)
				}
			}
		}

		fmt.Println("\n🎉 Container deployment pipeline completed successfully!")
		fmt.Println("Dockerfile and manifest files have been updated with the working versions.")
	}
}
