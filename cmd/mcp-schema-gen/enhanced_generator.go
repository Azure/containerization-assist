package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// EnhancedSchemaGenerator provides advanced code generation capabilities
type EnhancedSchemaGenerator struct {
	templateEngine *template.Template
	outputDir      string
	verbose        bool
}

// NewEnhancedSchemaGenerator creates a new enhanced generator
func NewEnhancedSchemaGenerator(outputDir string, verbose bool) (*EnhancedSchemaGenerator, error) {
	// Load templates
	tmpl, err := template.New("generator").Funcs(templateFuncs()).ParseFiles("cmd/mcp-schema-gen/templates/canonical_tool.go.tmpl")
	if err != nil {
		return nil, mcperrors.NewError().Messagef("failed to load templates: %w", err).WithLocation().Build()
	}

	return &EnhancedSchemaGenerator{
		templateEngine: tmpl,
		outputDir:      outputDir,
		verbose:        verbose,
	}, nil
}

// ToolSpec defines the structure for tool generation
type ToolSpec struct {
	ToolName     string
	Domain       string
	Action       string
	Object       string
	Description  string
	InputFields  []FieldSpec
	OutputFields []FieldSpec
	Version      string
}

// FieldSpec defines input/output field specifications
type FieldSpec struct {
	Name        string
	Type        string
	Required    bool
	Description string
	Validation  []ValidationRule
}

// ValidationRule defines validation rules for fields
type ValidationRule struct {
	Type    string // required, minLength, maxLength, pattern, etc.
	Value   interface{}
	Message string
}

// GenerateToolBoilerplate generates complete boilerplate for a new tool
func (g *EnhancedSchemaGenerator) GenerateToolBoilerplate(spec ToolSpec) error {
	if g.verbose {
		fmt.Printf("Generating boilerplate for tool: %s\n", spec.ToolName)
	}

	// Enhance spec with additional data needed by template
	enhancedSpec := g.enhanceSpecForCanonicalTemplate(spec)

	// Create domain directory using the new canonical structure
	domainDir := filepath.Join(g.outputDir, "pkg/mcp/domain/containerization", spec.Domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return mcperrors.NewError().Messagef("failed to create domain directory: %w", err).WithLocation().Build()
	}

	// Generate canonical tool file
	outputPath := filepath.Join(domainDir, fmt.Sprintf("%s_tool.go", strings.ToLower(spec.ToolName)))
	if err := g.renderTemplate("canonical_tool.go.tmpl", outputPath, enhancedSpec); err != nil {
		return mcperrors.NewError().Messagef("failed to render canonical tool template: %w", err).WithLocation().Build()
	}

	return nil
}

// GenerateCanonicalTool generates a tool using the canonical tools.Tool interface
func (g *EnhancedSchemaGenerator) GenerateCanonicalTool(spec ToolSpec) error {
	if g.verbose {
		fmt.Printf("Generating canonical tool: %s\n", spec.ToolName)
	}

	// Enhance spec with additional data needed by template
	enhancedSpec := g.enhanceSpecForCanonicalTemplate(spec)

	// Create domain directory
	domainDir := filepath.Join(g.outputDir, "pkg/mcp/domain/containerization", spec.Domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return mcperrors.NewError().Messagef("failed to create domain directory: %w", err).WithLocation(

		// Generate canonical tool file
		).Build()
	}

	outputPath := filepath.Join(domainDir, fmt.Sprintf("%s_canonical_tool.go", strings.ToLower(spec.ToolName)))
	return g.renderTemplate("canonical_tool.go.tmpl", outputPath, enhancedSpec)
}

// enhanceSpecForCanonicalTemplate adds template-specific data to the tool spec
func (g *EnhancedSchemaGenerator) enhanceSpecForCanonicalTemplate(spec ToolSpec) map[string]interface{} {
	// Create enhanced input fields with GoType
	enhancedInputFields := make([]map[string]interface{}, len(spec.InputFields))
	for i, field := range spec.InputFields {
		goType := "string"
		switch field.Type {
		case "string":
			goType = "string"
		case "boolean":
			goType = "bool"
		case "array":
			goType = "[]string"
		}

		enhancedInputFields[i] = map[string]interface{}{
			"Name":        field.Name,
			"Type":        field.Type,
			"GoType":      goType,
			"Required":    field.Required,
			"Description": field.Description,
			"Validation":  field.Validation,
		}
	}

	// Create enhanced output fields with GoType
	enhancedOutputFields := make([]map[string]interface{}, len(spec.OutputFields))
	for i, field := range spec.OutputFields {
		goType := "string"
		switch field.Type {
		case "string":
			goType = "string"
		case "boolean":
			goType = "bool"
		case "array":
			goType = "[]string"
		}

		enhancedOutputFields[i] = map[string]interface{}{
			"Name":        field.Name,
			"Type":        field.Type,
			"GoType":      goType,
			"Required":    field.Required,
			"Description": field.Description,
			"Validation":  field.Validation,
		}
	}

	enhanced := map[string]interface{}{
		"ToolName":      spec.ToolName,
		"ToolNameLower": strings.ToLower(spec.ToolName),
		"Domain":        spec.Domain,
		"Description":   spec.Description,
		"Version":       spec.Version,
		"InputFields":   enhancedInputFields,
		"OutputFields":  enhancedOutputFields,
		"Tags":          []string{spec.Domain, "containerization", "automation"},
	}

	return enhanced
}

// GenerateInterfaceCompliance generates interface compliance checking code
func (g *EnhancedSchemaGenerator) GenerateInterfaceCompliance(tools []ToolSpec) error {
	if g.verbose {
		fmt.Println("Generating interface compliance checks")
	}

	complianceFile := filepath.Join(g.outputDir, "pkg/mcp/internal/validation", "interface_compliance_test.go")
	return g.renderTemplate("interface_compliance.go.tmpl", complianceFile, map[string]interface{}{
		"Tools": tools,
	})
}

// GenerateMigrationScript generates migration scripts for existing tools
func (g *EnhancedSchemaGenerator) GenerateMigrationScript(toolName, domain string) error {
	if g.verbose {
		fmt.Printf("Generating migration script for %s.%s\n", domain, toolName)
	}

	scriptFile := filepath.Join(g.outputDir, "scripts", fmt.Sprintf("migrate_%s_%s.sh", domain, toolName))
	if err := os.MkdirAll(filepath.Dir(scriptFile), 0755); err != nil {
		return mcperrors.NewError().Messagef("failed to create scripts directory: %w", err).WithLocation().Build()
	}

	return g.renderTemplate("migration_script.sh.tmpl", scriptFile, map[string]interface{}{
		"ToolName": toolName,
		"Domain":   domain,
	})
}

// renderTemplate renders a template to a file
func (g *EnhancedSchemaGenerator) renderTemplate(templateName, outputPath string, data interface{}) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := g.templateEngine.ExecuteTemplate(file, templateName, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if g.verbose {
		fmt.Printf("Generated: %s\n", outputPath)
	}

	return nil
}

// templateFuncs returns template helper functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"toLower":    strings.ToLower,
		"toUpper":    strings.ToUpper,
		"title":      strings.Title,
		"camelCase":  toCamelCase,
		"pascalCase": toPascalCase,
		"snakeCase":  toSnakeCase,
		"kebabCase":  toKebabCase,
	}
}

// Helper functions for string transformation
func toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})

	result := strings.ToLower(words[0])
	for i := 1; i < len(words); i++ {
		result += strings.Title(strings.ToLower(words[i]))
	}
	return result
}

func toPascalCase(s string) string {
	camel := toCamelCase(s)
	if len(camel) == 0 {
		return camel
	}
	return strings.ToUpper(camel[:1]) + camel[1:]
}

func toSnakeCase(s string) string {
	result := strings.ToLower(s)
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, " ", "_")
	return result
}

func toKebabCase(s string) string {
	result := strings.ToLower(s)
	result = strings.ReplaceAll(result, "_", "-")
	result = strings.ReplaceAll(result, " ", "-")
	return result
}
