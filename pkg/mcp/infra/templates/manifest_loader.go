package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

//go:embed manifests/*.yaml
var manifestFS embed.FS

// LoadManifestTemplate loads and parses a manifest template
func LoadManifestTemplate(name string) (*template.Template, error) {
	path := fmt.Sprintf("manifests/%s.yaml", name)
	data, err := manifestFS.ReadFile(path)
	if err != nil {
		return nil, errors.NewError().Message("reading template " + name).Cause(err).WithLocation().Build()
	}

	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		return nil, errors.NewError().Message("parsing template " + name).Cause(err).WithLocation().Build()
	}

	return tmpl, nil
}

// RenderManifest renders a manifest template with the given data
func RenderManifest(templateName string, data interface{}) (string, error) {
	tmpl, err := LoadManifestTemplate(templateName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.NewError().Message("executing template " + templateName).Cause(err).WithLocation().Build()
	}

	return buf.String(), nil
}

// ListManifestTemplates returns available manifest templates
func ListManifestTemplates() ([]string, error) {
	entries, err := manifestFS.ReadDir("manifests")
	if err != nil {
		return nil, err
	}

	var templates []string
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[len(entry.Name())-5:] == ".yaml" {
			templates = append(templates, entry.Name()[:len(entry.Name())-5])
		}
	}

	return templates, nil
}
