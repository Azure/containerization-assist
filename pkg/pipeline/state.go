package pipeline

import (
	"fmt"
	"os"

	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
)

// PipelineState holds state across steps and iterations
type PipelineState struct {
	RepoFileTree   string
	Dockerfile     docker.Dockerfile
	RegistryURL    string
	ImageName      string
	K8sObjects     map[string]*k8s.K8sObject
	Success        bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

// FileAnalysisInput represents the common input structure for file analysis
type FileAnalysisInput struct {
	Content       string `json:"content"` // Plain text content of the file
	ErrorMessages string `json:"error_messages,omitempty"`
	RepoFileTree  string `json:"repo_files,omitempty"` // String representation of the file tree
	FilePath      string `json:"file_path,omitempty"`  // Path to the original file
}

// FileAnalysisResult represents the common analysis result
type FileAnalysisResult struct {
	FixedContent string `json:"fixed_content"`
	Analysis     string `json:"analysis"`
}

// InitializeManifests populates the K8sManifests field in PipelineState with manifests found in the specified path
// If path is empty, the default manifest path will be used
func (s *PipelineState) InitializeManifests(path string) error {
	k8sObjects, err := k8s.FindK8sObjects(path)
	if err != nil {
		return fmt.Errorf("failed to find manifests: %w", err)
	}
	if len(k8sObjects) == 0 {
		return fmt.Errorf("no Kubernetes deployment files found in %s", path)
	}
	fmt.Printf("Found %d Kubernetes objects from %s\n", len(k8sObjects), path)
	for _, obj := range k8sObjects {
		fmt.Printf("  '%s' kind: %s source: %s\n", obj.Metadata.Name, obj.Kind, obj.ManifestPath)
	}

	s.K8sObjects = make(map[string]*k8s.K8sObject)
	for i := range k8sObjects {
		obj := k8sObjects[i]
		objKey := fmt.Sprintf("%s-%s", obj.Kind, obj.Metadata.Name)
		s.K8sObjects[objKey] = &obj
	}

	return nil
}

// updateSuccessfulFiles writes the successful Dockerfile and manifests from the pipeline state to disk
func UpdateSuccessfulFiles(state *PipelineState) {
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
