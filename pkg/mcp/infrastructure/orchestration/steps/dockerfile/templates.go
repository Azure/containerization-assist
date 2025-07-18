// Package dockerfile provides Dockerfile templates for different programming languages
package dockerfile

// GetTemplate returns the Dockerfile template for a given language
func GetTemplate(language string) (string, bool) {
	switch language {
	case "go":
		return GoTemplate, true
	case "java":
		return JavaTemplate, true
	case "javascript", "typescript", "node":
		return NodeTemplate, true
	case "python":
		return PythonTemplate, true
	case "rust":
		return RustTemplate, true
	case "php":
		return PHPTemplate, true
	case "generic":
		return GenericTemplate, true
	default:
		return "", false
	}
}

// GetAllTemplates returns a map of all available templates
func GetAllTemplates() map[string]string {
	return map[string]string{
		"go":      GoTemplate,
		"java":    JavaTemplate,
		"node":    NodeTemplate,
		"python":  PythonTemplate,
		"rust":    RustTemplate,
		"php":     PHPTemplate,
		"generic": GenericTemplate,
	}
}
