package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/clients"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/filetree"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

func generate(targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, c *clients.Clients) error {

	kindClusterName, err := c.GetKindCluster()
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	fmt.Printf("Using kind cluster: %s\n", kindClusterName)

	maxIterations := 5
	dockerfilePath := filepath.Join(targetDir, "Dockerfile")

	state := &pipeline.PipelineState{
		K8sObjects:     make(map[string]*k8s.K8sObject),
		Success:        false,
		IterationCount: 0,
		Metadata:       make(map[string]interface{}),
		ImageName:      "app", // TODO: clean up app naming into state
		RegistryURL:    registry,
	}
	fmt.Printf("validating connection to registry %s\n", registry)
	err = docker.ValidateRegistryReachable(state.RegistryURL)
	if err != nil {
		return fmt.Errorf("reaching registry %s: %w\n", state.RegistryURL, err)
	}

	if enableDraftDockerfile {
		fmt.Printf("Generating Dockerfile in %s\n", targetDir)
		draftTemplateName, err := docker.GetDockerfileTemplateName(c.AzOpenAIClient, targetDir)
		if err != nil {
			return fmt.Errorf("getting Dockerfile template name: %w", err)
		}

		fmt.Printf("Using Dockerfile template: %s\n", draftTemplateName)
		err = docker.GenerateDockerfileWithDraft(draftTemplateName, targetDir)
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
	err = docker.GenerateDeploymentFilesWithDraft(targetDir, registryAndImage)
	if err != nil {
		return fmt.Errorf("generating deployment files: %w", err)
	}

	//Add RepoFileTree to state after Dockerfile and Manifests are generated
	repoStructure, err := filetree.ReadFileTree(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get file tree: %w", err)
	}
	state.RepoFileTree = repoStructure

	err = state.InitializeManifests(targetDir) // Initialize K8sManifests with default path
	if err != nil {
		return fmt.Errorf("failed to initialize manifests: %w", err)
	}

	err = state.InitializeDockerFileState(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize Dockerfile state: %w", err)
	}

	errors := []string{}
	for i := 0; i < maxIterations && !state.Success; i++ {
		if err := pipeline.IterateDockerfileBuild(maxIterations, state, targetDir, generateSnapshot, c); err != nil {
			errors = append(errors, fmt.Sprintf("error in Dockerfile iteration process: %v", err))
			break
		}

		fmt.Printf("pushing image %s\n", registryAndImage)
		err = c.PushDockerImage(registryAndImage)
		if err != nil {
			return fmt.Errorf("pushing image %s: %w\n", registryAndImage, err)
		}

		if err := pipeline.IterateMultipleManifestsDeploy(maxIterations, state, targetDir, generateSnapshot, c); err != nil {
			errors = append(errors, fmt.Sprintf("error in Kubernetes deployment process: %v", err))
		}
	}

	if !state.Success {
		fmt.Println("\n❌ Container deployment pipeline did not complete successfully after maximum iterations.")
		fmt.Println("No files were updated. Please review the logs for more information.")
		return fmt.Errorf("errors encountered during iteration:\n%s", strings.Join(errors, "\n"))
	}

	// Update the dockerfile and manifests with the final successful versions
	pipeline.UpdateSuccessfulFiles(state)
	return nil
}
