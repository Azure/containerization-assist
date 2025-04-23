package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/filetree"
)

type Dockerfile struct {
	Content     string
	Path        string
	BuildErrors string
}

// Use LLM to select the dockerfile template name from the list of available templates
func GetDockerfileTemplateName(client *ai.AzOpenAIClient, projectDir string) (string, error) {
	dockerfileTemplateNames, err := listSubdirNames(filepath.Join("templates", "dockerfiles"))
	if err != nil {
		return "", fmt.Errorf("failed to list dockerfile template names: %w", err)
	}

	repoStructure, err := filetree.ReadFileTree(projectDir)
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

func listSubdirNames(path string) ([]string, error) {
	var dirs []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

func WriteDockerfileFromTemplate(templateName, targetDir string) error {
	templateDir := filepath.Join("templates", "dockerfiles", templateName)

	filesToCopy := []string{"Dockerfile", ".dockerignore"}

	for _, filename := range filesToCopy {
		src := filepath.Join(templateDir, filename)
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("error getting stat of file %q: %w", src, err)
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("error reading file %q: %w", src, err)
		}
		dest := filepath.Join(targetDir, filename)
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("error writing file %q: %w", dest, err)
		}
	}

	return nil
}
