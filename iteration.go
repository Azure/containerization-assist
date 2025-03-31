//Should provide an interface to that allows for flexible iterating
//As in let's say we find it's better to first verify the dockerfile even before trying to build it

//File provide an idea for a possible iteration structure, feel free to changes/leave ideas

package main

import (
	"fmt"
)

// K8sManifest represents a single Kubernetes manifest and its deployment status
type K8sManifest struct {
	Name             string
	Content          string
	isDeployed       bool
	isDeploymentType bool
	//Possibly Summary of changes
}

// PipelineState holds state across steps and iterations
type PipelineState struct {
	Dockerfile     string
	K8sManifests   map[string]*K8sManifest
	BuildSuccess   bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

// Step is a function type that processes the pipeline state
type Step func(state *PipelineState) error

// Pipeline holds a sequence of steps to execute
type Pipeline struct {
	Steps         []Step
	ShouldIterate func(state *PipelineState) bool
	MaxIterations int
}

// Execute runs all steps in the pipeline
func (p *Pipeline) Execute() (*PipelineState, error) {
	state := &PipelineState{
		Metadata:     make(map[string]interface{}),
		K8sManifests: make(map[string]*K8sManifest),
	}

	for state.IterationCount < p.MaxIterations {
		fmt.Printf("Starting iteration %d\n", state.IterationCount+1)

		for i, step := range p.Steps {
			fmt.Printf("  Executing step %d\n", i+1)
			if err := step(state); err != nil {
				return state, fmt.Errorf("step %d failed: %w", i+1, err)
			}
		}

		if !p.ShouldIterate(state) {
			fmt.Println("Iteration complete, no further iterations needed")
			break
		}

		state.IterationCount++
	}

	return state, nil
}

// Helper method to check if all manifests are successfully deployed
func (s *PipelineState) AllManifestsDeployed() bool {
	if len(s.K8sManifests) == 0 {
		return false
	}

	for _, manifest := range s.K8sManifests {
		if !manifest.isDeployed {
			return false
		}
	}
	return true
}

// FILLER FUNCTIONS BELOW - These don't actually do anything

func DraftDockerfile(state *PipelineState) error {
	fmt.Println("  Generated draft Dockerfile")
	return nil
}

func EnhanceWithLLM(state *PipelineState) error {
	state.Dockerfile += "\n# Enhanced with LLM"
	fmt.Println("  Enhanced Dockerfile with LLM")
	return nil
}

func ValidateDockerfile(state *PipelineState) error {
	fmt.Println("  Validated Dockerfile")
	return nil
}

// BuildDockerfile attempts to build the Docker image
func BuildDockerfile(state *PipelineState) error {
	state.BuildSuccess = true
	fmt.Println("  Built Docker image")
	return nil
}

// FILLER FUCNTIONS END

func ExampleUsage() {
	examplePipeline := &Pipeline{
		Steps: []Step{
			InitializeDefaultPathManifests,
			DraftDockerfile,
			EnhanceWithLLM,
			ValidateDockerfile,
			BuildDockerfile,
		},
		ShouldIterate: func(state *PipelineState) bool {
			return !state.BuildSuccess // Iterate until build succeeds
		},
		MaxIterations: 3,
	}

	// Execute the pipeline
	result, err := examplePipeline.Execute()
	if err != nil {
		fmt.Printf("Pipeline failed: %v\n", err)
		return
	}

	fmt.Printf("Final Dockerfile after %d iterations:\n%s\n",
		result.IterationCount+1, result.Dockerfile)
}
