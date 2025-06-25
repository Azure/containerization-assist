package main

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

// Generator handles code generation
type Generator struct {
	inputDir  string
	outputDir string
	verbose   bool
	logger    *log.Logger
}

// NewGenerator creates a new code generator
func NewGenerator(inputDir, outputDir string, verbose bool) *Generator {
	return &Generator{
		inputDir:  inputDir,
		outputDir: outputDir,
		verbose:   verbose,
		logger:    log.New(os.Stdout, "[generator] ", log.LstdFlags),
	}
}

// GenerateAdapters generates adapter code for each tool
func (g *Generator) GenerateAdapters(tools []ToolInfo) error {
	tmpl := template.Must(template.New("adapter").Parse(adapterTemplate))

	for _, tool := range tools {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, tool); err != nil {
			return fmt.Errorf("failed to execute adapter template for %s: %w", tool.Name, err)
		}

		// Format the generated code
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			if g.verbose {
				g.logger.Printf("Warning: Failed to format adapter for %s: %v", tool.Name, err)
				g.logger.Printf("Generated code:\n%s", buf.String())
			}
			formatted = buf.Bytes()
		}

		// Write to file
		fileName := fmt.Sprintf("%s_adapter.go", tool.Name)
		filePath := filepath.Join(g.outputDir, "adapters", fileName)
		if err := os.WriteFile(filePath, formatted, 0644); err != nil {
			return fmt.Errorf("failed to write adapter file for %s: %w", tool.Name, err)
		}

		if g.verbose {
			g.logger.Printf("Generated adapter: %s", filePath)
		}
	}

	return nil
}

// GenerateConverters generates converter functions
func (g *Generator) GenerateConverters(tools []ToolInfo) error {
	tmpl := template.Must(template.New("converters").Parse(convertersTemplate))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tools); err != nil {
		return fmt.Errorf("failed to execute converters template: %w", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		if g.verbose {
			g.logger.Printf("Warning: Failed to format converters: %v", err)
		}
		formatted = buf.Bytes()
	}

	// Write to file
	filePath := filepath.Join(g.outputDir, "converters.go")
	if err := os.WriteFile(filePath, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write converters file: %w", err)
	}

	if g.verbose {
		g.logger.Printf("Generated converters: %s", filePath)
	}

	return nil
}

// GenerateRegistry generates the tool registry
func (g *Generator) GenerateRegistry(tools []ToolInfo) error {
	tmpl := template.Must(template.New("registry").Parse(registryTemplate))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tools); err != nil {
		return fmt.Errorf("failed to execute registry template: %w", err)
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		if g.verbose {
			g.logger.Printf("Warning: Failed to format registry: %v", err)
		}
		formatted = buf.Bytes()
	}

	// Write to file
	filePath := filepath.Join(g.outputDir, "registry.go")
	if err := os.WriteFile(filePath, formatted, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	if g.verbose {
		g.logger.Printf("Generated registry: %s", filePath)
	}

	return nil
}
