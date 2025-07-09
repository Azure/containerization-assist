package services

// TemplateLoader provides an interface for loading templates
type TemplateLoader interface {
	// LoadTemplate loads a template by name
	LoadTemplate(name string) (string, error)

	// ListTemplates lists all available templates
	ListTemplates() ([]string, error)
}
