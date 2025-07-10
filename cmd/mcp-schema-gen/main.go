package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		tool        = flag.String("tool", "", "Tool to generate (format: domain_action_object)")
		domain      = flag.String("domain", "", "Domain for the tool (e.g., security, build, deploy)")
		outputDir   = flag.String("output", ".", "Output directory for generated code")
		verbose     = flag.Bool("v", false, "Verbose output")
		genType     = flag.String("type", "boilerplate", "Generation type: boilerplate, compliance, migration")
		description = flag.String("desc", "", "Tool description")
	)
	flag.Parse()

	if *tool == "" {
		return fmt.Errorf("tool name is required (use -tool flag)")
	}

	if *domain == "" {
		return fmt.Errorf("domain is required (use -domain flag)")
	}

	// Initialize enhanced generator
	generator, err := NewEnhancedSchemaGenerator(*outputDir, *verbose)
	if err != nil {
		return fmt.Errorf("failed to initialize generator: %w", err)
	}

	// Parse tool name
	parts := strings.Split(*tool, "_")
	if len(parts) < 3 {
		return fmt.Errorf("tool name must be in format domain_action_object (e.g., security_scan_container)")
	}

	toolSpec := ToolSpec{
		ToolName:    parts[1] + parts[2], // e.g., ScanContainer
		Domain:      *domain,
		Action:      parts[1], // e.g., scan
		Object:      parts[2], // e.g., container
		Description: *description,
		Version:     "1.0.0",
		InputFields: []FieldSpec{
			{
				Name:        "session_id",
				Type:        "string",
				Required:    false,
				Description: "Session ID for correlation (auto-generated if not provided)",
			},
		},
		OutputFields: []FieldSpec{
			{
				Name:        "status",
				Type:        "string",
				Required:    true,
				Description: "Execution status",
			},
		},
	}

	// Add domain-specific fields
	switch *domain {
	case "security":
		toolSpec.InputFields = append(toolSpec.InputFields, []FieldSpec{
			{
				Name:        "image_ref",
				Type:        "string",
				Required:    true,
				Description: "Container image reference to scan",
				Validation: []ValidationRule{
					{Type: "required", Message: "image_ref is required"},
					{Type: "pattern", Value: "^[a-z0-9]+(\\.[a-z0-9]+)*\\/[a-z0-9]+:[a-z0-9]+$", Message: "must be valid image reference"},
				},
			},
			{
				Name:        "scan_type",
				Type:        "string",
				Required:    false,
				Description: "Type of security scan (vulnerabilities, secrets, etc.)",
			},
		}...)
		toolSpec.OutputFields = append(toolSpec.OutputFields, []FieldSpec{
			{
				Name:        "vulnerabilities",
				Type:        "[]Vulnerability",
				Required:    false,
				Description: "List of discovered vulnerabilities",
			},
			{
				Name:        "security_score",
				Type:        "float64",
				Required:    false,
				Description: "Overall security score",
			},
		}...)
	case "build":
		toolSpec.InputFields = append(toolSpec.InputFields, []FieldSpec{
			{
				Name:        "dockerfile_path",
				Type:        "string",
				Required:    false,
				Description: "Path to Dockerfile (default: ./Dockerfile)",
			},
			{
				Name:        "image_name",
				Type:        "string",
				Required:    true,
				Description: "Name for the built image",
				Validation: []ValidationRule{
					{Type: "required", Message: "image_name is required"},
				},
			},
		}...)
		toolSpec.OutputFields = append(toolSpec.OutputFields, []FieldSpec{
			{
				Name:        "image_id",
				Type:        "string",
				Required:    false,
				Description: "ID of the built image",
			},
			{
				Name:        "build_logs",
				Type:        "[]string",
				Required:    false,
				Description: "Build process logs",
			},
		}...)
	case "deploy":
		toolSpec.InputFields = append(toolSpec.InputFields, []FieldSpec{
			{
				Name:        "manifest_path",
				Type:        "string",
				Required:    true,
				Description: "Path to Kubernetes manifest file",
				Validation: []ValidationRule{
					{Type: "required", Message: "manifest_path is required"},
				},
			},
			{
				Name:        "namespace",
				Type:        "string",
				Required:    false,
				Description: "Kubernetes namespace (default: default)",
			},
		}...)
		toolSpec.OutputFields = append(toolSpec.OutputFields, []FieldSpec{
			{
				Name:        "deployed_resources",
				Type:        "[]DeployedResource",
				Required:    false,
				Description: "List of deployed Kubernetes resources",
			},
		}...)
	}

	// Generate based on type
	switch *genType {
	case "boilerplate":
		fmt.Printf("Generating boilerplate for %s tool in %s domain...\n", *tool, *domain)
		if err := generator.GenerateToolBoilerplate(toolSpec); err != nil {
			return fmt.Errorf("failed to generate boilerplate: %w", err)
		}
		fmt.Println("✅ Boilerplate generation completed!")

	case "compliance":
		fmt.Println("Generating interface compliance checks...")
		if err := generator.GenerateInterfaceCompliance([]ToolSpec{toolSpec}); err != nil {
			return fmt.Errorf("failed to generate compliance checks: %w", err)
		}
		fmt.Println("✅ Interface compliance generation completed!")

	case "migration":
		fmt.Printf("Generating migration script for %s...\n", *tool)
		if err := generator.GenerateMigrationScript(toolSpec.ToolName, *domain); err != nil {
			return fmt.Errorf("failed to generate migration script: %w", err)
		}
		fmt.Println("✅ Migration script generation completed!")

	default:
		return fmt.Errorf("unknown generation type: %s (valid types: boilerplate, compliance, migration)", *genType)
	}

	// Print usage instructions
	fmt.Println("\n📋 Next steps:")
	fmt.Println("1. Review generated code for correctness")
	fmt.Println("2. Implement business logic in the Execute method")
	fmt.Println("3. Add comprehensive test cases")
	fmt.Println("4. Run: go build ./pkg/mcp/internal/" + *domain + "/")
	fmt.Println("5. Run: go test ./pkg/mcp/internal/" + *domain + "/")

	return nil
}
