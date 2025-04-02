package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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


func generate(client *azopenai.Client,deploymentID string, dir string) error{
	fmt.Println("Generating dockerfile")
	maxIterations := 5
	dockerfilePath := filepath.Join(dir,"./Dockerfile")

	// Get current working dir for file structure path
	fmt.Println("reading repo file tree at ",dir)
	repoStructure, err := readFileTree(dir)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %s",err.Error())
	}

	if !dockerDaemonIsRunning(){
		return fmt.Errorf("docker daemon not detected")
	}

	if err := iterateDockerfileBuild(client, deploymentID, dockerfilePath, repoStructure, maxIterations); err != nil {
		return fmt.Errorf("Error in dockerfile iteration process: %v\n", err)
	}

	manifestPath := filepath.Join(dir,"manifests") // Default directory containing manifests

	if err := iterateMultipleManifestsDeploy(client, deploymentID, manifestPath, repoStructure, maxIterations); err != nil {
		return fmt.Errorf("Error in Kubernetes deployment process: %v", err)
	}
	return nil
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

	// Check command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "generate":
			repoDir := "."
			if len( os.Args) > 2{
				fmt.Printf("targeting repo at %s",repoDir)
				repoDir = os.Args[2]
			}
			generate(client,deploymentID,repoDir)
		default:
			// Default behavior - test Azure OpenAI
			resp, err := client.GetChatCompletions(
				context.Background(),
				azopenai.ChatCompletionsOptions{
					DeploymentName: to.Ptr(deploymentID),
					Messages: []azopenai.ChatRequestMessageClassification{
						&azopenai.ChatRequestUserMessage{
							Content: azopenai.NewChatRequestUserMessageContent("Hello Azure OpenAI! Tell me this is working in one short sentence."),
						},
					},
				},
				nil,
				)
			if err != nil {
				fmt.Printf("Error getting chat completions: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Azure OpenAI Test:")
			if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
				fmt.Printf("Response: %s\n", *resp.Choices[0].Message.Content)
			}
		}
		return
	}

	// If no arguments provided, print usage
	fmt.Println("Usage:")
	fmt.Println("  container_copilot                          - Test Azure OpenAI connection")
	fmt.Println("  container_copilot generate [path-to-repository-root] [max-iterations] - Iteratively generate starter container artifacts with AI")
}
