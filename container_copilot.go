package main

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
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

func main() {
	// Get environment variables
	apiKey := os.Getenv("AZURE_OPENAI_KEY")
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	deploymentID := "o3-mini-2"

	if apiKey == "" || endpoint == "" {
		fmt.Println("Error: AZURE_OPENAI_KEY and AZURE_OPENAI_ENDPOINT environment variables must be set")
		os.Exit(1)
	}

	// Create a client with KeyCredential
	keyCredential := azcore.NewKeyCredential(apiKey)
	client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
	if err != nil {
		fmt.Printf("Error creating Azure OpenAI client: %v\n", err)
		os.Exit(1)
	}

	maxIterations := 5
	dockerfilePath := "./Dockerfile"
	manifestPath := "../../../manifests" // Default directory containing manifests
	fileStructurePath := "repo_structure_json.txt"

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

	initialState := &PipelineState{
		RepoFileTree:   repoStructure,
		K8sManifests:   make(map[string]*K8sManifest),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[string]interface{}),
	}

	err = InitializeDefaultPathManifests(initialState) // Initialize K8sManifests with default path
	if err != nil {
		fmt.Printf("Failed to initialize manifests: %v\n", err)
		return
	}

	err = initializeDockerFileState(initialState, dockerfilePath)
	if err != nil {
		fmt.Printf("Failed to initialize Dockerfile state: %v\n", err)
		return
	}

	if err := iterateDockerfileBuildWithPrevErrors(client, deploymentID, initalState); err != nil {
		fmt.Printf("Error in dockerfile iteration process: %v\n", err)
		os.Exit(1)
	}

	if err := iterateMultipleManifestsDeploy(client, deploymentID, manifestPath, fileStructurePath, maxIterations); err != nil {
		fmt.Printf("Error in Kubernetes deployment process: %v", err)
		os.Exit(1)
	}

	//Update the dockerfile and manifests with the final successful versions stored in the pipeline state

}
