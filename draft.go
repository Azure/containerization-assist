package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/draft/pkg/handlers"
	"github.com/Azure/draft/pkg/templatewriter/writers"
	"maps"
	"slices"
	"strings"
)

const (
	manifestDeploymentTemplateName = "deployment-manifests"
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

func generateDeploymentFilesWithDraft(outputDir string) error {
	// APPNAME doesn't have a default value in draft template
	customVariables := map[string]string{
		"APPNAME": "app",
	}
	return generateArtifactsWithDraft(manifestDeploymentTemplateName, outputDir, customVariables)
}

// Can be used to feed the template names to LLM to choose
func getDockerfileTemplateNamesFromDraft() []string {
	dockerfileTemplateMap := handlers.GetTemplatesByType(handlers.TemplateTypeDockerfile)
	return slices.Collect(maps.Keys(dockerfileTemplateMap))
}

// Use LLM to select the dockerfile template name from the list of available templates in draft
func getDockerfileTemplateName(client *azopenai.Client, deploymentID, projectDir string) (string, error) {
	dockerfileTemplateNames := getDockerfileTemplateNamesFromDraft()

	repoStructure, err := readFileTree(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to get file tree: %w", err)
	}

	promptText := fmt.Sprintf(`
You are selecting a Dockerfile template for a project.

Available Dockerfile templates:
%s

Project repository structure:
%s

Based on the files in this project, select the most appropriate Dockerfile template name from the list.
Return only the exact template name from the list without any other text, explanation or formatting.
`, strings.Join(dockerfileTemplateNames, "\n"), repoStructure)

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
		return "", err
	}

	var templateName string

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		content := *resp.Choices[0].Message.Content
		// Extract the template name from the response
		templateName = strings.TrimSpace(content)
		if !slices.Contains(dockerfileTemplateNames, templateName) {
			return "", fmt.Errorf("invalid template name: %s", templateName)
		}
	}
	return templateName, nil
}
