package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"

	"log/slog"
)

var (
	templatePath = flag.String("template", "", "Path to pipeline template")
	outputPath   = flag.String("output", "", "Output file path")
	pipelineName = flag.String("name", "", "Pipeline name")
	stageNames   = flag.String("stages", "", "Comma-separated stage names")
)

func main() {
	flag.Parse()

	if *templatePath == "" || *outputPath == "" || *pipelineName == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -template <path> -output <path> -name <name> [-stages <stages>]\n", os.Args[0])
		os.Exit(1)
	}

	generator := &PipelineGenerator{
		TemplatePath: *templatePath,
		OutputPath:   *outputPath,
		Name:         *pipelineName,
		Stages:       parseStages(*stageNames),
	}

	if err := generator.Generate(); err != nil {
		slog.Error("Failed to generate pipeline", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Pipeline %s generated successfully at %s\n", *pipelineName, *outputPath)
}

type PipelineGenerator struct {
	TemplatePath string
	OutputPath   string
	Name         string
	Stages       []string
}

func (g *PipelineGenerator) Generate() error {
	tmpl, err := template.ParseFiles(g.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	output, err := os.Create(g.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()

	data := struct {
		Name   string
		Stages []string
	}{
		Name:   g.Name,
		Stages: g.Stages,
	}

	if err := tmpl.Execute(output, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func parseStages(stages string) []string {
	if stages == "" {
		return []string{}
	}

	// Simple comma-separated parsing
	var result []string
	for _, stage := range strings.Split(stages, ",") {
		result = append(result, strings.TrimSpace(stage))
	}
	return result
}
