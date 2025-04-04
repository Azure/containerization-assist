package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestDeployResult stores the result of a single manifest deployment
type ManifestDeployResult struct {
	Path    string
	Success bool
	Output  string
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

// K8sManifest represents a single Kubernetes manifest and its deployment status
type K8sManifest struct {
	Name             string
	Content          string
	Path             string
	isDeployed       bool
	isDeploymentType bool
	errorLog         string
	//Possibly Summary of changes
}

type Dockerfile struct {
	Content     string
	Path        string
	BuildErrors string
}

// PipelineState holds state across steps and iterations
type PipelineState struct {
	RepoFileTree   string
	Dockerfile     Dockerfile
	K8sManifests   map[string]*K8sManifest
	Success        bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

// updateSuccessfulFiles writes the successful Dockerfile and manifests from the pipeline state to disk
func updateSuccessfulFiles(state *PipelineState) {
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
		for name, manifest := range state.K8sManifests {
			if manifest.isDeployed && manifest.Path != "" {
				fmt.Printf("Writing updated manifest: %s\n", name)
				if err := os.WriteFile(manifest.Path, []byte(manifest.Content), 0644); err != nil {
					fmt.Printf("Error writing manifest %s: %v\n", name, err)
				}
			}
		}

		fmt.Println("\n🎉 Container deployment pipeline completed successfully!")
		fmt.Println("Dockerfile and manifest files have been updated with the working versions.")
	}
}

func (c *AzOpenAIClient) generate(outputDir string) error {
	maxIterations := 5
	dockerfilePath := filepath.Join(outputDir, "Dockerfile")

	repoStructure, err := readFileTree(outputDir)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %w", err)
	}

	state := &PipelineState{
		RepoFileTree:   repoStructure,
		K8sManifests:   make(map[string]*K8sManifest),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[string]interface{}),
	}

	err = InitializeDefaultPathManifests(state) // Initialize K8sManifests with default path
	if err != nil {
		return fmt.Errorf("failed to initialize manifests: %w", err)
	}

	err = initializeDockerFileState(state, dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize Dockerfile state: %w", err)
	}

	errors := []string{}
	if err := iterateDockerfileBuild(c, maxIterations, state); err != nil {
		errors = append(errors, fmt.Sprintf("error in Dockerfile iteration process: %v", err))
	}

	if err := iterateMultipleManifestsDeploy(c, maxIterations, state); err != nil {
		errors = append(errors, fmt.Sprintf("error in Kubernetes deplpoyment process: %v", err))
	}

	if len(errors) > 0 {
		fmt.Println("\n❌ Container deployment pipeline did not complete successfully after maximum iterations.")
		fmt.Println("No files were updated. Please review the logs for more information.")
		return fmt.Errorf("errors encountered during iteration:\n%s", strings.Join(errors, "\n"))
	}

	// Update the dockerfile and manifests with the final successful versions
	updateSuccessfulFiles(state)
	return nil
}

func (c *AzOpenAIClient) testOpenAIConn() error {
	testResponse, err := c.GetChatCompletion("Hello Azure OpenAI! Tell me this is working in one short sentence.")
		if err != nil {
			return fmt.Errorf("failed to get chat completion: %w", err)
		}

		fmt.Println("Azure OpenAI Test")
		fmt.Printf("Response: %s\n", testResponse)
	return nil
}

