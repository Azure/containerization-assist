package main

import (
	"fmt"
	"os"
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
	} else {
		fmt.Println("\n❌ Container deployment pipeline did not complete successfully after maximum iterations.")
		fmt.Println("No files were updated. Please review the logs for more information.")
	}
}

func main() {
	// Get environment variables
	apiKey := os.Getenv("AZURE_OPENAI_KEY")
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	deploymentID := "o3-mini-2"

	if apiKey == "" || endpoint == "" {
		fmt.Println("Error: AZURE_OPENAI_KEY and AZURE_OPENAI_ENDPOINT environment variables must be set")
		os.Exit(1)
	}

	client, err := NewAzOpenAIClient(endpoint, apiKey, deploymentID)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "generate":

			maxIterations := 5
			dockerfilePath := "./Dockerfile"

			// Get current working dir for file structure path
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Println("Error getting current directory:", err)
				return
			}
			repoStructure, err := readFileTree(cwd)
			if err != nil {
				fmt.Printf("failed to get file tree: %s", err.Error())
				os.Exit(1)
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
				fmt.Printf("Failed to initialize manifests: %v\n", err)
				return
			}

			err = initializeDockerFileState(state, dockerfilePath)
			if err != nil {
				fmt.Printf("Failed to initialize Dockerfile state: %v\n", err)
				return
			}

			// loop through until max iterations or success
			for state.IterationCount < maxIterations && !state.Success {
				if err := iterateDockerfileBuild(client, state); err != nil {
					fmt.Printf("Error in dockerfile iteration process: %v\n", err)
					continue
				}

				if err := iterateMultipleManifestsDeploy(client, maxIterations, state); err != nil {
					fmt.Printf("Error in Kubernetes deployment process: %v", err)
					os.Exit(1)
				}
			}

			// Update the dockerfile and manifests with the final successful versions
			updateSuccessfulFiles(state)

		default:
			// Default behavior - test Azure OpenAI
			testResponse, err := client.GetChatCompletion("Hello Azure OpenAI! Tell me this is working in one short sentence.")
			if err != nil {
				fmt.Printf("Error getting chat completions: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Azure OpenAI Test:")
			fmt.Printf("Response: %s\n", testResponse)
		}
	}
}
