package docker

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/draft/pkg/handlers"
	"github.com/Azure/draft/pkg/templatewriter/writers"
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

func GenerateDockerfileWithDraft(dockerfileTemplateName, outputDir string) error {
	return generateArtifactsWithDraft(dockerfileTemplateName, outputDir, nil)
}

func GenerateDeploymentFilesWithDraft(outputDir string, registryAndImage string) error {
	// APPNAME doesn't have a default value in draft template
	logger.Infof("generating manifests with imagename %s", registryAndImage)
	customVariables := map[string]string{
		"IMAGENAME": registryAndImage,
		"APPNAME":   "app", // TODO: make appname based on repo dir
	}
	return generateArtifactsWithDraft(manifestDeploymentTemplateName, outputDir, customVariables)
}
