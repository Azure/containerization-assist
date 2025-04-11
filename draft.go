package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Azure/draft/pkg/handlers"
	"github.com/Azure/draft/pkg/templatewriter/writers"
)

const (
	manifestDeploymentTemplateName = "deployment-manifests"
	dockerTemplatePrompt           = `
You are selecting a Dockerfile template for a project.

Available Dockerfile templates:
%s

Project repository structure:
%s

First, analyze the project to determine how it should be built. 
Based on project, select the most appropriate Dockerfile template name from the list.
Return only the exact template name from the list without any other text, explanation or formatting.
`
)

func generateArtifactsWithDraft(templateName, outputDir string, variables map[string]string) error {
	writer := writers.LocalFSWriter{
		WriteMode: 0644,
	}

	template, err := handlers.GetTemplate(templateName, "", outputDir, &writer)
	if err != nil {
		return fmt.Errorf("error getting template '%s' from draft: %v\n", templateName, err)
	}
	if template == nil {
		return fmt.Errorf("template not found: %s\n", templateName)
	}

	if variables != nil {
		for k, v := range variables {
			template.Config.SetVariable(k, v)
		}
	}

	err = template.Generate()
	if err != nil {
		return fmt.Errorf("error generating files from template %s: %w", templateName, err)
	}

	return nil
}

func generateDockerfileWithDraft(dockerfileTemplateName, outputDir string) error {
	return generateArtifactsWithDraft(dockerfileTemplateName, outputDir, nil)
}

func generateDeploymentFilesWithDraft(outputDir string, registryAndImage string) error {
	// APPNAME doesn't have a default value in draft template
	fmt.Println("generating manifests with imagename ", registryAndImage)
	customVariables := map[string]string{
		"IMAGENAME": registryAndImage,
		"APPNAME":   "app", // TODO: make appname based on repo dir
	}
	return generateArtifactsWithDraft(manifestDeploymentTemplateName, outputDir, customVariables)
}

// Can be used to feed the template names to LLM to choose
func getDockerfileTemplateNamesFromDraft() []string {
	dockerfileTemplateMap := handlers.GetTemplatesByType(handlers.TemplateTypeDockerfile)
	return slices.Collect(maps.Keys(dockerfileTemplateMap))
}

// Use LLM to select the dockerfile template name from the list of available templates in draft
func getDockerfileTemplateName(client *AzOpenAIClient, projectDir string) (string, error) {
	dockerfileTemplateNames := getDockerfileTemplateNamesFromDraft()

	repoStructure, err := readFileTree(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to get file tree: %w", err)
	}

	promptText := fmt.Sprintf(dockerTemplatePrompt, strings.Join(dockerfileTemplateNames, "\n"), repoStructure)

	content, err := client.GetChatCompletion(promptText)
	if err != nil {
		return "", err
	}

	templateName := strings.TrimSpace(content)
	if !slices.Contains(dockerfileTemplateNames, templateName) {
		return "", fmt.Errorf("invalid template name: %s", templateName)
	}

	return templateName, nil
}
