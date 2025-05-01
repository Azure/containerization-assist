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

const (
	repoStructurePrompt = `
You are analyzing a code repository with the goal of generating or refining Dockerfiles.

Your task is to provide two things:

1. A **structured summary** of the repository that includes:
   - A 1-sentence description of the overall architecture (e.g., single-service web app, multi-module Maven project, Spring microservices architecture, etc.).
   - A breakdown of the **project layout**, including:
     • Primary source folders (e.g., src/main/java, src/main/webapp)
     • Configuration files (e.g., pom.xml, web.xml, hibernate.cfg.xml)
     • Support for containerization (e.g., Dockerfile, docker-compose.yml, .dockerignore, manifests/)
     • Development and CI tooling (e.g., .devcontainer, GitHub workflows, shell scripts)
   - Highlight how these files relate to **build, packaging, and deployment**.

2. A **ranked list of the top 10 files or directories** that are most relevant for creating or modifying Dockerfiles.
   - For each item, include:
     • The path (or name) of the file or folder
     • A short sentence on why it matters when building a container image

Your goal is to provide context so that someone writing a Dockerfile understands which files influence:
   - how the app is built (e.g., Maven, Gradle, npm),
   - how the app runs (e.g., Java JAR/WAR, Node server, Python entrypoint),
   - and how it fits into a containerized system (e.g., Dockerfile layout, orchestration, CI).

Do not guess or hallucinate structure—base your output strictly on what is present in the following repo tree:

Repo structure:
%s
`
)

func generate(targetDir string, registry string, enableDraftDockerfile bool, generateSnapshot bool, c *clients.Clients) error {

	kindClusterName, err := c.GetKindCluster()
	if err != nil {
		return fmt.Errorf("failed to get kind cluster: %w", err)
	}
	fmt.Printf("Using kind cluster: %s\n", kindClusterName)

	maxIterations := 5
	maxFullLoopIterations := 2
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
		templateName, err := docker.GetDockerfileTemplateName(c.AzOpenAIClient, targetDir)
		if err != nil {
			return fmt.Errorf("getting Dockerfile template name: %w", err)
		}

		fmt.Printf("Using Dockerfile template: %s\n", templateName)
		err = docker.WriteDockerfileFromTemplate(templateName, targetDir)
		if err != nil {
			return fmt.Errorf("writing Dockerfile from template: %w", err)
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

	//Call chat completion to analyze the repo structure and provide the top 10 relevant files useful for dockerfile creation
	repoStructureSummary, err := c.AzOpenAIClient.GetChatCompletionWithFormat(repoStructurePrompt, repoStructure)
	fmt.Printf("Repo structure summary: %s\n", repoStructureSummary)

	repoStructure = repoStructureSummary + "\n" + repoStructure
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
	for i := 0; i < maxFullLoopIterations && !state.Success; i++ {
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
