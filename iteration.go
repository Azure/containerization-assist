//Should provide an interface to that allows for flexible iterating
//As in let's say we find it's better to first verify the dockerfile even before trying to build it

//File provide an idea for a possible iteration structure, feel free to changes/leave ideas

package main

import (
	"fmt"
)

// IterationContext holds state across steps and iterations
type IterationContext struct { //These are just examples
	Dockerfile     string
	K8sManifests   string
	BuildSuccess   bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

// Step is a function type that processes the iteration context
type Step func(ctx *IterationContext) error

// Pipeline holds a sequence of steps to execute
type Pipeline struct {
	Steps         []Step
	ShouldIterate func(ctx *IterationContext) bool
	MaxIterations int
}

// Execute runs all steps in the pipeline
func (p *Pipeline) Execute() (*IterationContext, error) {
	ctx := &IterationContext{
		Metadata: make(map[string]interface{}),
	}

	for ctx.IterationCount < p.MaxIterations {
		fmt.Printf("Starting iteration %d\n", ctx.IterationCount+1)

		for i, step := range p.Steps {
			fmt.Printf("  Executing step %d\n", i+1)
			if err := step(ctx); err != nil {
				return ctx, fmt.Errorf("step %d failed: %w", i+1, err)
			}
		}

		if !p.ShouldIterate(ctx) {
			fmt.Println("Iteration complete, no further iterations needed")
			break
		}

		ctx.IterationCount++
	}

	return ctx, nil
}

// FILLER FUNCTIONS BELOW - These don't actually do anything

func DraftDockerfile(ctx *IterationContext) error {
	fmt.Println("  Generated draft Dockerfile")
	return nil
}

func EnhanceWithLLM(ctx *IterationContext) error {
	ctx.Dockerfile += "\n# Enhanced with LLM"
	fmt.Println("  Enhanced Dockerfile with LLM")
	return nil
}

func ValidateDockerfile(ctx *IterationContext) error {
	fmt.Println("  Validated Dockerfile")
	return nil
}

// BuildDockerfile attempts to build the Docker image
func BuildDockerfile(ctx *IterationContext) error {
	ctx.BuildSuccess = true
	fmt.Println("  Built Docker image")
	return nil
}

// FILLER FUCNTIONS END

func ExampleUsage() {
	// Create a pipeline for Dockerfile generation
	dockerPipeline := &Pipeline{
		Steps: []Step{
			DraftDockerfile,
			EnhanceWithLLM,
			ValidateDockerfile,
			BuildDockerfile,
		},
		ShouldIterate: func(ctx *IterationContext) bool {
			return !ctx.BuildSuccess // Iterate until build succeeds
		},
		MaxIterations: 3,
	}

	// Execute the pipeline
	result, err := dockerPipeline.Execute()
	if err != nil {
		fmt.Printf("Pipeline failed: %v\n", err)
		return
	}

	fmt.Printf("Final Dockerfile after %d iterations:\n%s\n",
		result.IterationCount+1, result.Dockerfile)
}
