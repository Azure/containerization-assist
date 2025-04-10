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
	RegistryURL    string
	ImageName      string
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

func (c *AzOpenAIClient) generate(targetDir string, registry string) error {
	kindClusterName, err := getKindCluster()
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	fmt.Printf("Using kind cluster: %s\n", kindClusterName)

	maxIterations := 5
	dockerfilePath := filepath.Join(targetDir, "Dockerfile")

	state := &PipelineState{
		K8sManifests:   make(map[string]*K8sManifest),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[string]interface{}),
		ImageName:      "app", // TODO: clean up app naming into state
		RegistryURL:    registry,
	}
	fmt.Printf("validating connection to registry %s\n", registry)
	err = validateRegistryReachable(state.RegistryURL)
	if err != nil {
		return fmt.Errorf("reaching registry %s: %w\n", state.RegistryURL, err)
	}

	fmt.Printf("Generating Dockerfile in %s\n", targetDir)
	draftTemplateName, err := getDockerfileTemplateName(c, targetDir)
	if err != nil {
		return fmt.Errorf("getting Dockerfile template name: %w", err)
	}

	fmt.Printf("Using Dockerfile template: %s\n", draftTemplateName)
	err = generateDockerfileWithDraft(draftTemplateName, targetDir)
	if err != nil {
		return fmt.Errorf("generating Dockerfile: %w", err)
	}

	fmt.Printf("Generating Kubernetes manifests in %s\n", targetDir)
	registryAndImage := fmt.Sprintf("%s/%s", registry, "app")
	err = generateDeploymentFilesWithDraft(targetDir, registryAndImage)
	if err != nil {
		return fmt.Errorf("generating deployment files: %w", err)
	}

	//Add RepoFileTree to state after Dockerfile and Manifests are generated
	repoStructure, err := readFileTree(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %w", err)
	}
	state.RepoFileTree = repoStructure

	err = InitializeManifests(state, targetDir) // Initialize K8sManifests with default path
	if err != nil {
		return fmt.Errorf("failed to initialize manifests: %w", err)
	}

	err = initializeDockerFileState(state, dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize Dockerfile state: %w", err)
	}

	errors := []string{}
	for i := 0; i < maxIterations && !state.Success; i++ {
		if err := iterateDockerfileBuild(c, maxIterations, state, targetDir); err != nil {
			errors = append(errors, fmt.Sprintf("error in Dockerfile iteration process: %v", err))
			break
		}

		fmt.Printf("pushing image %s\n", registryAndImage)
		err = pushDockerImage(registryAndImage)
		if err != nil {
			return fmt.Errorf("pushing image %s: %w\n", registryAndImage, err)
		}

		if err := iterateMultipleManifestsDeploy(c, maxIterations, state); err != nil {
			errors = append(errors, fmt.Sprintf("error in Kubernetes deployment process: %v", err))
		}

	}

	if !state.Success {
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
