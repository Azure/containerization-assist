package templates

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

//go:embed workflows stages pipelines components
var templateFS embed.FS

// LoadTemplate loads a template from the embedded filesystem
func LoadTemplate(path string) (string, error) {
	data, err := templateFS.ReadFile(path)
	if err != nil {
		return "", errors.NewError().Messagef("loading template %s", path).Cause(err).WithLocation().Build()
	}
	return string(data), nil
}

// ListTemplates returns all available template paths
func ListTemplates() ([]string, error) {
	var templates []string

	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			templates = append(templates, path)
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().Message("walking template directory").Cause(err).WithLocation().Build()
	}

	return templates, nil
}

// LoadTemplatesByCategory loads all templates in a category
func LoadTemplatesByCategory(category string) (map[string]string, error) {
	templates := make(map[string]string)

	dirPath := category
	entries, err := templateFS.ReadDir(dirPath)
	if err != nil {
		return nil, errors.NewError().Messagef("reading category %s", category).Cause(err).WithLocation().Build()
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			path := filepath.Join(dirPath, entry.Name())
			content, err := LoadTemplate(path)
			if err != nil {
				return nil, err
			}

			// Use filename without extension as key
			key := strings.TrimSuffix(entry.Name(), ".yaml")
			templates[key] = content
		}
	}

	return templates, nil
}

// TemplateExists checks if a template exists
func TemplateExists(path string) bool {
	_, err := templateFS.ReadFile(path)
	return err == nil
}

// GetTemplateCategories returns all template categories
func GetTemplateCategories() []string {
	return []string{"workflows", "stages", "pipelines", "components"}
}
