package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
		for name, object := range state.K8sObjects {
			if object.isSuccessfullyDeployed && object.ManifestPath != "" {
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

func (c *Clients) generate(targetDir string, registry string, enableDraftDockerfile bool) error {

	kindClusterName, err := c.getKindCluster()
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	fmt.Printf("Using kind cluster: %s\n", kindClusterName)

	maxIterations := 5
	dockerfilePath := filepath.Join(targetDir, "Dockerfile")

	state := &PipelineState{
		K8sObjects:     make(map[string]*K8sObject),
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

	if enableDraftDockerfile {
		fmt.Printf("Generating Dockerfile in %s\n", targetDir)
		draftTemplateName, err := getDockerfileTemplateName(c.AzOpenAIClient, targetDir)
		if err != nil {
			return fmt.Errorf("getting Dockerfile template name: %w", err)
		}

		fmt.Printf("Using Dockerfile template: %s\n", draftTemplateName)
		err = generateDockerfileWithDraft(draftTemplateName, targetDir)
		if err != nil {
			return fmt.Errorf("generating Dockerfile: %w", err)
		}
	} else {
		fmt.Printf("Writing blank starter Dockerfile in %s\n", targetDir)
		fmt.Printf("writing empty dockerfile\n")
		err = os.WriteFile(filepath.Join(targetDir, "Dockerfile"), []byte{}, fs.ModePerm)
		if err != nil {
			return fmt.Errorf("writing blank Dockerfile: %w", err)
		}
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
	pipelineStateHistory := []PipelineState{}
	for i := 0; i < maxIterations && !state.Success; i++ {
		if err := c.iterateDockerfileBuild(maxIterations, state, targetDir, &pipelineStateHistory); err != nil {
			errors = append(errors, fmt.Sprintf("error in Dockerfile iteration process: %v", err))
			break
		}

		fmt.Printf("pushing image %s\n", registryAndImage)
		err = c.pushDockerImage(registryAndImage)
		if err != nil {
			return fmt.Errorf("pushing image %s: %w\n", registryAndImage, err)
		}

		if err := c.iterateMultipleManifestsDeploy(maxIterations, state, &pipelineStateHistory); err != nil {
			errors = append(errors, fmt.Sprintf("error in Kubernetes deployment process: %v", err))
		}
	}

	// can make this optional with a flag later
	if err := writeIterationSnapshot(pipelineStateHistory, targetDir); err != nil {
		return fmt.Errorf("writing iteration snapshot: %w", err)
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

func (c *Clients) testOpenAIConn() error {
	testResponse, err := c.AzOpenAIClient.GetChatCompletion("Hello Azure OpenAI! Tell me this is working in one short sentence.")
	if err != nil {
		return fmt.Errorf("failed to get chat completion: %w", err)
	}

	fmt.Println("Azure OpenAI Test")
	fmt.Printf("Response: %s\n", testResponse)
	return nil
}
