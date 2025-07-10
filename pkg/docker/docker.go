package docker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Azure/container-kit/pkg/ai"
	"github.com/Azure/container-kit/templates"
)

type Dockerfile struct {
	Content                 string
	Path                    string
	BuildErrors             string
	PreviousAttemptsSummary string
}

const (
	dockerTemplatePrompt = `
You are selecting a Dockerfile template for a project.

Available Dockerfile templates:
%s

Project repository structure:
%s

First, analyze the project to determine how it should be built.
Based on project, select the most appropriate Dockerfile template name from the list.
Return only the exact template name from the list without any other text, explanation or formatting.
`

	// DockerfileRunningErrors is used to create a summary of Docker build failures
	DockerfileRunningErrors = `
You're helping analyze repeated build failures while trying to generate a working Dockerfile.
Here is a summary of previous errors and attempted fixes:
%s
Here is the most recent build error:
%s
Your task is to maintain a concise and clear summary of what has been attempted so far.
Summarize:
- What caused the most recent failure
- What changes were made in the last attempt
- Why those changes didn't work
You are not fixing the Dockerfile directly. However, if there is a clear pattern of incorrect assumptions or a flawed strategy, you may briefly point it out to guide the next iteration.
Keep the tone neutral and factual, but feel free to raise a flag if something needs to change.
`
)

// Use LLM to select the dockerfile template name from the list of available templates
func GetDockerfileTemplateName(ctx context.Context, client ai.LLMClient, projectDir string, repoStructure string) (string, ai.TokenUsage, error) {
	dockerfileTemplateNames, err := listEmbeddedSubdirNames("dockerfiles")
	if err != nil {
		return "", ai.TokenUsage{}, fmt.Errorf("failed to list dockerfile template names: %w", err)
	}

	promptText := fmt.Sprintf(dockerTemplatePrompt, strings.Join(dockerfileTemplateNames, "\n"), repoStructure)

	content, tokenUsage, err := client.GetChatCompletion(ctx, promptText)
	if err != nil {
		return "", tokenUsage, err
	}

	templateName := strings.TrimSpace(content)
	if !slices.Contains(dockerfileTemplateNames, templateName) {
		return "", tokenUsage, fmt.Errorf("invalid template name: %s", templateName)
	}

	return templateName, tokenUsage, nil
}

func listEmbeddedSubdirNames(path string) ([]string, error) {
	entries, err := templates.Templates.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading embedded dir %q: %w", path, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

func WriteDockerfileFromTemplate(templateName, targetDir string) error {
	embeddedBasePath := path.Join("dockerfiles", templateName)
	filesToCopy := []string{"Dockerfile", ".dockerignore"}
	for _, filename := range filesToCopy {
		embeddedPath := path.Join(embeddedBasePath, filename)
		data, err := templates.Templates.ReadFile(embeddedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("reading embedded file %q: %w", embeddedPath, err)
		}
		destPath := filepath.Join(targetDir, filename)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("writing file %q: %w", destPath, err)
		}
	}
	return nil
}
