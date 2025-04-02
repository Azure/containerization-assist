package main

import (
	"context"
	"fmt"
	"os"

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
		case "iterate-dockerfile-build":
			maxIterations := 5
			dockerfilePath := "./Dockerfile"

			// Allow custom dockerfile path
			if len(os.Args) > 2 {
				dockerfilePath = os.Args[2]
			}

			// Allow custom max iterations
			if len(os.Args) > 4 {
				fmt.Sscanf(os.Args[4], "%d", &maxIterations)
			}

			// Get current working dir for file structure path
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Println("Error getting current directory:", err)
				return
			}
			repoStructure, err := readFileTree(cwd)
			if err != nil {
				fmt.Printf("failed to get file tree: %s",err.Error())
				os.Exit(1)
			}

			if err := iterateDockerfileBuild(client, deploymentID, dockerfilePath, repoStructure, maxIterations); err != nil {
				fmt.Printf("Error in dockerfile iteration process: %v\n", err)
				os.Exit(1)
			}

		case "iterate-kubernetes-deploy":
			maxIterations := 5
			manifestPath := "../../../manifests" // Default directory containing manifests
			fileStructurePath := "repo_structure_json.txt"

			// Allow custom manifest path (can be a directory or file)
			if len(os.Args) > 2 {
				manifestPath = os.Args[2]
			}

			// Allow file structure path
			if len(os.Args) > 3 {
				fileStructurePath = os.Args[3]
			}

			// Allow custom max iterations
			if len(os.Args) > 4 {
				fmt.Sscanf(os.Args[4], "%d", &maxIterations)
			}

			if err := iterateMultipleManifestsDeploy(client, deploymentID, manifestPath, fileStructurePath, maxIterations); err != nil {
				fmt.Printf("Error in Kubernetes deployment process: %v", err)
				os.Exit(1)
			}

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
	fmt.Println("  go run container_copilot.go                          - Test Azure OpenAI connection")
	fmt.Println("  go run container_copilot.go iterate-dockerfile-build [dockerfile-path] [file-structure-path] [max-iterations] - Iteratively build and fix a Dockerfile")
	fmt.Println("  go run container_copilot.go iterate-kubernetes-deploy [manifest-path-or-dir] [file-structure-path] [max-iterations] - Iteratively deploy and fix Kubernetes manifest(s)")
}
