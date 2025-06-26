package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		inputDir  = flag.String("input", "pkg/mcp/internal/tools", "Input directory containing tool definitions")
		outputDir = flag.String("output", "pkg/mcp/internal/orchestration/dispatch/generated", "Output directory for generated code")
		verbose   = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create subdirectories
	adaptersDir := filepath.Join(*outputDir, "adapters")
	if err := os.MkdirAll(adaptersDir, 0755); err != nil {
		return fmt.Errorf("failed to create adapters directory: %w", err)
	}

	// Initialize generator
	gen := &Generator{
		inputDir:  *inputDir,
		outputDir: *outputDir,
		verbose:   *verbose,
		logger:    log.New(os.Stdout, "[generator] ", log.LstdFlags),
	}

	// Analyze tools
	fmt.Println("Analyzing tools...")
	tools, err := gen.AnalyzeTools()
	if err != nil {
		return fmt.Errorf("failed to analyze tools: %w", err)
	}
	fmt.Printf("Found %d tools\n", len(tools))

	// Generate code for each tool
	fmt.Println("Generating tool adapters...")
	if err := gen.GenerateAdapters(tools); err != nil {
		return fmt.Errorf("failed to generate adapters: %w", err)
	}

	// Generate converters
	fmt.Println("Generating converters...")
	if err := gen.GenerateConverters(tools); err != nil {
		return fmt.Errorf("failed to generate converters: %w", err)
	}

	// Generate registry
	fmt.Println("Generating registry...")
	if err := gen.GenerateRegistry(tools); err != nil {
		return fmt.Errorf("failed to generate registry: %w", err)
	}

	fmt.Println("Code generation completed successfully!")
	return nil
}
