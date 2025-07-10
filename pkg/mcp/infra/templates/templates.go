package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

//go:embed *.tmpl Dockerfile
//go:embed manifests/*.yaml
//go:embed components/*.yaml
//go:embed workflows/*.yaml
//go:embed stages/*.yaml
//go:embed pipelines/*.yaml
var templateFS embed.FS

// LoadTemplate loads a template by name
func LoadTemplate(name string) (string, error) {
	// Try direct path first
	content, err := templateFS.ReadFile(name)
	if err == nil {
		return string(content), nil
	}

	// Try with common extensions
	extensions := []string{".yaml", ".yml", ".tmpl", ""}
	for _, ext := range extensions {
		path := name + ext
		content, err := templateFS.ReadFile(path)
		if err == nil {
			return string(content), nil
		}
	}

	// Try in subdirectories
	subdirs := []string{"manifests", "components", "workflows", "stages", "pipelines"}
	for _, dir := range subdirs {
		for _, ext := range extensions {
			path := filepath.Join(dir, name+ext)
			content, err := templateFS.ReadFile(path)
			if err == nil {
				return string(content), nil
			}
		}
	}

	return "", fmt.Errorf("template not found: %s", name)
}

// ListTemplates lists all available templates
func ListTemplates() ([]string, error) {
	var templates []string

	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && (strings.HasSuffix(path, ".yaml") ||
			strings.HasSuffix(path, ".yml") ||
			strings.HasSuffix(path, ".tmpl") ||
			filepath.Base(path) == "Dockerfile") {
			templates = append(templates, path)
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().Code(errors.CodeInternalError).Message("failed to list templates").Cause(err).Build()
	}

	return templates, nil
}
