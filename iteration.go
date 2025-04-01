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
	Success        bool
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
func (p *Pipeline) Execute(initialState *PipelineState) (*PipelineState, error) {
	state := initialState
	if state == nil {
		state = &PipelineState{
			Metadata:     make(map[string]interface{}),
			K8sManifests: make(map[string]*K8sManifest),
		}
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
	//Call function in draft.go to generate a Dockerfile

	//Should store dockerfile in state

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
	state.Success = true
	fmt.Println("  Built Docker image")
	return nil
}

// FILLER FUCNTIONS END

func ExampleUsage() {
	// Initialize state before pipeline execution
	initialState := &PipelineState{
		Metadata:     make(map[string]interface{}),
		K8sManifests: make(map[string]*K8sManifest),
	}

	//Call function in draft.go to generate a Dockerfile

	initialState.Dockerfile = "" // Add the generated Dockerfile here // Currently assumes just one dockerfile

	err := InitializeDefaultPathManifests(initialState) // Initialize K8sManifests with default path
	if err != nil {
		fmt.Printf("Failed to initialize manifests: %v\n", err)
		return
	}

	//At this point we have a dockerfile and a set of manifests in the state
	// We can now proceed to the iteration pipelines

	// Dockerfile Pipeline
	dockerfilePipeline := &Pipeline{
		Steps: []Step{
			EnhanceWithLLM,
			BuildDockerfile,
		},
		ShouldIterate: func(state *PipelineState) bool {
			return !state.Success // Iterate until build succeeds
		},
		MaxIterations: 3,
	}

	// K8s Manifest Pipeline - uses the state from the Dockerfile pipeline
	manifestPipeline := &Pipeline{
		Steps: []Step{
			//DeployK8sManifests, only try to deploy the manifests that were not previously succesfully deployed
			//ValidateK8sManifests,
			//EnhanceWithLLM,
		},
		ShouldIterate: func(state *PipelineState) bool {
			return !state.AllManifestsDeployed() // Iterate until all manifests are deployed //REQUIRES CHANGE sucessful deployment does not neccesarily mean container running
		},
		MaxIterations: 3,
	}

	fmt.Println("EXECUTING DOCKERFILE PIPELINE")
	dockerResult, err := dockerfilePipeline.Execute(initialState)
	if err != nil {
		fmt.Printf("Dockerfile pipeline failed: %v\n", err)
		return
	}
	fmt.Printf("Final Dockerfile after %d iterations:\n%s\n\n",
		dockerResult.IterationCount+1, dockerResult.Dockerfile)

	// Reset the iteration count before starting the manifest pipeline
	dockerResult.IterationCount = 0

	// The state from the dockerfile pipeline (dockerResult) is passed to the manifest pipeline,
	// so all changes made during the dockerfile pipeline will be available to the manifest pipeline
	fmt.Println("EXECUTING K8S MANIFEST PIPELINE")
	manifestResult, err := manifestPipeline.Execute(dockerResult)
	if err != nil {
		fmt.Printf("K8s manifest pipeline failed: %v\n", err)
		return
	}

	fmt.Printf("Processed %d Kubernetes manifests after %d iterations\n",
		len(manifestResult.K8sManifests), manifestResult.IterationCount+1)
}
