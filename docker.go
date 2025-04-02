package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// buildDockerfile attempts to build the Docker image and returns any error output
func buildDockerfile(dockerfilePath string,registryName string) (bool, string) {
	// Get the directory containing the Dockerfile to use as build context
	dockerfileDir := filepath.Dir(dockerfilePath)

	registryPrefix := ""
	if registryName != ""{
     registryPrefix = registryName + "/"
	}
	fmt.Println("building dockerfile at dir ",dockerfileDir)
	// Run Docker build with explicit context path
	// Use the absolute path for the dockerfile and specify the context directory
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", registryPrefix+"tomcat-hello-world-workflow:latest", dockerfileDir)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		fmt.Println("Docker build failed with error:", err)
		return false, outputStr
	}

	return true, outputStr
}

func analyzeDockerfile(client *azopenai.Client, deploymentID string, input FileAnalysisInput) (*FileAnalysisResult, error) {
	// Create prompt for analyzing the Dockerfile
	promptText := fmt.Sprintf(`Analyze the following Dockerfile for errors and suggest fixes:
Dockerfile:
%s
`, input.Content)

	// Add error information if provided and not empty
	if input.ErrorMessages != "" {
		promptText += fmt.Sprintf(`
Errors encountered when running this Dockerfile:
%s
`, input.ErrorMessages)
	} else {
		promptText += `
No error messages were provided. Please check for potential issues in the Dockerfile.
`
	}

	// Add repository file information if provided
	if input.RepoFileTree != "" {
		promptText += fmt.Sprintf(`
Repository files structure:
%s
`, input.RepoFileTree)
	}

	promptText += `
Please:
1. Identify any issues in the Dockerfile
2. Provide a fixed version of the Dockerfile
3. Explain what changes were made and why

Output the fixed Dockerfile between <<<DOCKERFILE>>> tags.`

	resp, err := client.GetChatCompletions(
		context.Background(),
		azopenai.ChatCompletionsOptions{
			DeploymentName: to.Ptr(deploymentID),
			Messages: []azopenai.ChatRequestMessageClassification{
				&azopenai.ChatRequestUserMessage{
					Content: azopenai.NewChatRequestUserMessageContent(promptText),
				},
			},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		content := *resp.Choices[0].Message.Content

		// Extract the fixed Dockerfile from between the tags
		re := regexp.MustCompile(`<<<DOCKERFILE>>>([\s\S]*?)<<<DOCKERFILE>>>`)
		matches := re.FindStringSubmatch(content)

		fixedContent := ""
		if len(matches) > 1 {
			// Found the dockerfile between tags
			fixedContent = strings.TrimSpace(matches[1])
		} else {
			// If tags aren't found, try to extract the content intelligently
			// Look for multi-line dockerfile content after FROM
			fromRe := regexp.MustCompile(`(?m)^FROM[\s\S]*?$`)
			if fromMatches := fromRe.FindString(content); fromMatches != "" {
				// Simple heuristic: Consider everything from the first FROM as the dockerfile
				fixedContent = fromMatches
			} else {
				// Fallback: use the entire content
				fixedContent = content
			}
		}

		return &FileAnalysisResult{
			FixedContent: fixedContent,
			Analysis:     content,
		}, nil
	}

	return nil, fmt.Errorf("no response from AI model")
}


func checkDockerInstalled() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker executable not found in PATH. Please install Docker or ensure it's available in your PATH")
	}
	return nil
}


// iterateDockerfileBuild attempts to iteratively fix and build the Dockerfile
func iterateDockerfileBuild(client *azopenai.Client, deploymentID string, dockerfilePath string, repoStructure string, maxIterations int) error {
	fmt.Printf("Starting Dockerfile build iteration process for: %s\n", dockerfilePath)

	// Check if Docker is installed before starting the iteration process
	if err := checkDockerInstalled(); err != nil {
		return err
	}

	// Read the original Dockerfile
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("error reading Dockerfile: %v", err)
	}

	currentDockerfile := string(dockerfileContent)

	for i := range maxIterations {
		fmt.Printf("\n=== Iteration %d of %d ===\n", i+1, maxIterations)

		// Try to build
		registry := ""
		success, buildOutput := buildDockerfile(dockerfilePath,registry)
		if success {
			fmt.Println("🎉 Docker build succeeded!")
			fmt.Println("Successful Dockerfile: \n", currentDockerfile)

			//Temp code for pushing to kind registry
			if registry == ""{
				return fmt.Errorf("no registry provided, unable to push")
			}
			cmd := exec.Command("docker", "push", registry+"/tomcat-hello-world-workflow:latest")
			output, err := cmd.CombinedOutput()
			outputStr := string(output)
			fmt.Println("Output: ", outputStr)

			if err != nil {
				fmt.Println("Registry push failed with error:", err)
				return fmt.Errorf("error pushing to registry: %v", err)
			}

			return nil
		}

		fmt.Println("Docker build failed. Using AI to fix issues...")

		// Prepare input for AI analysis
		input := FileAnalysisInput{
			Content:       currentDockerfile,
			ErrorMessages: buildOutput,
			RepoFileTree:  string(repoStructure),
			FilePath:      dockerfilePath,
		}

		// Get AI to fix the Dockerfile - call analyzeDockerfile directly
		result, err := analyzeDockerfile(client, deploymentID, input)
		if err != nil {
			return fmt.Errorf("error in AI analysis: %v", err)
		}

		// Update the Dockerfile
		currentDockerfile = result.FixedContent
		fmt.Println("AI suggested fixes:")
		fmt.Println(result.Analysis)

		// Write the fixed Dockerfile
		if err := os.WriteFile(dockerfilePath, []byte(currentDockerfile), 0644); err != nil {
			return fmt.Errorf("error writing fixed Dockerfile: %v", err)
		}

		fmt.Printf("Updated Dockerfile written. Attempting build again...\n")
		time.Sleep(1 * time.Second) // Small delay for readability
	}

	return fmt.Errorf("failed to fix Dockerfile after %d iterations", maxIterations)
}

// findKubernetesManifests finds all kubernetes manifest files (YAML/YML) at the given path
// Path can be either a directory or a single file
func findKubernetesManifests(path string) ([]string, error) {
	var manifestPaths []string

	// Check if the input is a directory or a file
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error accessing path %s: %v", path, err)
	}

	if fileInfo.IsDir() {
		// It's a directory - find all YAML files
		fmt.Printf("Looking for Kubernetes manifest files in directory: %s\n", path)

		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) {
				manifestPaths = append(manifestPaths, filePath)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking manifest directory: %v", err)
		}
	} else {
		// It's a single file
		fmt.Printf("Using single manifest file: %s\n", path)

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			manifestPaths = append(manifestPaths, path)
		} else {
			return nil, fmt.Errorf("file %s is not a YAML/YML file", path)
		}
	}

	return manifestPaths, nil
}

func dockerDaemonIsRunning()bool{
	return true
}
