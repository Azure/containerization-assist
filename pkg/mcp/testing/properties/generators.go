// Package properties provides data generators for property-based testing.
package properties

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ============================================================================
// Data Generators for Property Testing
// ============================================================================

// generateWorkflowState creates a random workflow state for testing
func (pt *PropertyTester) generateWorkflowState() *workflow.WorkflowState {
	workflowID := pt.generateWorkflowID()
	args := pt.generateContainerizeArgs()

	// Generate result based on args (to respect test mode)
	result := pt.generateWorkflowResultWithArgs(args)

	currentStep := pt.rand.Intn(11) // 0-10 steps
	totalSteps := 10

	// Create basic workflow state
	state := &workflow.WorkflowState{
		WorkflowID:  workflowID,
		Args:        args,
		Result:      result,
		CurrentStep: currentStep,
		TotalSteps:  totalSteps,
	}

	// Add optional components based on progress
	if currentStep > 0 {
		state.AnalyzeResult = pt.generateAnalyzeResult()
	}
	if currentStep > 1 {
		state.DockerfileResult = pt.generateDockerfileResult()
	}
	if currentStep > 2 {
		state.BuildResult = pt.generateBuildResult()
	}
	if currentStep > 6 {
		state.K8sResult = pt.generateK8sResult()
	}

	return state
}

// generateSagaExecution creates a random saga execution for testing
func (pt *PropertyTester) generateSagaExecution() *saga.SagaExecution {
	sagaID := pt.generateSagaID()
	workflowID := pt.generateWorkflowID()
	state := pt.generateSagaState()
	steps := pt.generateSagaSteps()
	executedSteps := pt.generateExecutedSteps(steps)
	compensatedSteps := pt.generateCompensatedSteps(executedSteps, state)

	return &saga.SagaExecution{
		ID:               sagaID,
		WorkflowID:       workflowID,
		State:            state,
		Steps:            steps,
		ExecutedSteps:    executedSteps,
		CompensatedSteps: compensatedSteps,
	}
}

// generateWorkflowID creates a random workflow ID
func (pt *PropertyTester) generateWorkflowID() string {
	return fmt.Sprintf("wf-%d-%d", time.Now().Unix(), pt.rand.Int63())
}

// generateSagaID creates a random saga ID
func (pt *PropertyTester) generateSagaID() string {
	return fmt.Sprintf("saga-%d-%d", time.Now().Unix(), pt.rand.Int63())
}

// generateContainerizeArgs creates random containerization arguments
func (pt *PropertyTester) generateContainerizeArgs() *workflow.ContainerizeAndDeployArgs {
	repoURLs := []string{
		"https://github.com/example/app",
		"https://github.com/test/service",
		"https://github.com/demo/microservice",
	}
	branches := []string{"main", "develop", "feature/test", ""}

	deployPtr := func(b bool) *bool { return &b }(pt.rand.Float64() < 0.8) // 80% chance of deploy

	return &workflow.ContainerizeAndDeployArgs{
		RepoURL:  repoURLs[pt.rand.Intn(len(repoURLs))],
		Branch:   branches[pt.rand.Intn(len(branches))],
		Scan:     pt.rand.Float64() < 0.7, // 70% chance of scanning
		Deploy:   deployPtr,
		TestMode: pt.rand.Float64() < 0.3, // 30% chance of test mode
	}
}

// generateWorkflowResultWithArgs creates a random workflow result that respects test mode
func (pt *PropertyTester) generateWorkflowResultWithArgs(args *workflow.ContainerizeAndDeployArgs) *workflow.ContainerizeAndDeployResult {
	success := pt.rand.Float64() < 0.8 // 80% success rate

	// Successful workflows should have completed all 10 steps
	// Failed workflows can fail at any step
	stepCount := 10
	if !success {
		stepCount = pt.rand.Intn(10) + 1 // 1-10 steps for failed workflows
	}

	result := &workflow.ContainerizeAndDeployResult{
		Success: success,
		Steps:   make([]workflow.WorkflowStep, stepCount),
	}

	if success {
		// Generate image ref based on test mode
		if args.TestMode {
			// In test mode, prefix with "test-"
			result.ImageRef = fmt.Sprintf("test-registry/test-%s:test-%s",
				pt.generateRandomString(8),
				pt.generateRandomString(6))
			result.Namespace = "test-namespace"
		} else {
			result.ImageRef = pt.generateImageRef()
			result.Namespace = pt.generateRandomString(8)
		}
		result.Endpoint = pt.generateEndpoint()
	} else {
		result.Error = pt.generateErrorMessage()
	}

	// Generate workflow steps
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	for i := 0; i < stepCount; i++ {
		stepName := stepNames[i%len(stepNames)]
		status := "completed"
		if i == stepCount-1 && !success {
			status = "failed"
		}

		result.Steps[i] = workflow.WorkflowStep{
			Name:     stepName,
			Status:   status,
			Duration: fmt.Sprintf("%ds", pt.rand.Intn(60)+10),
			Progress: fmt.Sprintf("%d/10", i+1),
			Message:  fmt.Sprintf("Completed %s", stepName),
		}

		if status == "failed" {
			result.Steps[i].Error = pt.generateErrorMessage()
			result.Steps[i].Retries = pt.rand.Intn(3)
		}
	}

	return result
}

// generateWorkflowResult creates a random workflow result
func (pt *PropertyTester) generateWorkflowResult() *workflow.ContainerizeAndDeployResult {
	success := pt.rand.Float64() < 0.8 // 80% success rate
	stepCount := pt.rand.Intn(11)      // 0-10 steps

	result := &workflow.ContainerizeAndDeployResult{
		Success: success,
		Steps:   make([]workflow.WorkflowStep, stepCount),
	}

	if success {
		result.ImageRef = pt.generateImageRef()
		result.Endpoint = pt.generateEndpoint()
		result.Namespace = pt.generateRandomString(8)
	} else {
		result.Error = pt.generateErrorMessage()
	}

	// Generate workflow steps
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	for i := 0; i < stepCount; i++ {
		stepName := stepNames[i%len(stepNames)]
		status := "completed"
		if !success && i == stepCount-1 {
			status = "failed"
		}

		result.Steps[i] = workflow.WorkflowStep{
			Name:     stepName,
			Status:   status,
			Duration: fmt.Sprintf("%ds", pt.rand.Intn(300)+10),
			Progress: fmt.Sprintf("%d/10", i+1),
			Message:  fmt.Sprintf("Step %s %s", stepName, status),
			Retries:  pt.rand.Intn(3),
		}

		if status == "failed" {
			result.Steps[i].Error = pt.generateErrorMessage()
		}
	}

	return result
}

// generateAnalyzeResult creates a random analysis result
func (pt *PropertyTester) generateAnalyzeResult() *workflow.AnalyzeResult {
	languages := []string{"go", "python", "node", "java", "rust"}
	frameworks := []string{"gin", "fastapi", "express", "spring", "actix"}

	return &workflow.AnalyzeResult{
		Language:        languages[pt.rand.Intn(len(languages))],
		Framework:       frameworks[pt.rand.Intn(len(frameworks))],
		Port:            []int{8080, 3000, 8000}[pt.rand.Intn(3)],
		BuildCommand:    "go build",
		StartCommand:    "./app",
		Dependencies:    pt.generateDependencies(),
		DevDependencies: pt.generateDependencies(),
		Metadata:        make(map[string]interface{}),
		RepoPath:        "/tmp/repo",
	}
}

// generateDockerfileResult creates a random Dockerfile result
func (pt *PropertyTester) generateDockerfileResult() *workflow.DockerfileResult {
	return &workflow.DockerfileResult{
		Content:     "FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build\nEXPOSE 8080\nCMD [\"./app\"]",
		Path:        "./Dockerfile",
		BaseImage:   "golang:1.21",
		Metadata:    make(map[string]interface{}),
		ExposedPort: 8080,
	}
}

// generateBuildResult creates a random build result
func (pt *PropertyTester) generateBuildResult() *workflow.BuildResult {
	return &workflow.BuildResult{
		ImageID:   pt.generateImageID(),
		ImageRef:  pt.generateImageRef(),
		ImageSize: int64(pt.rand.Intn(1000000000) + 100000000), // 100MB-1GB
		BuildTime: fmt.Sprintf("%ds", pt.rand.Intn(300)+30),
		Metadata:  make(map[string]interface{}),
	}
}

// generateK8sResult creates a random Kubernetes result
func (pt *PropertyTester) generateK8sResult() *workflow.K8sResult {
	return &workflow.K8sResult{
		Manifests:   []string{"deployment.yaml", "service.yaml"},
		Namespace:   pt.generateRandomString(8),
		ServiceName: pt.generateRandomString(10),
		Endpoint:    pt.generateEndpoint(),
		Metadata:    make(map[string]interface{}),
	}
}

// generateSagaState creates a random saga state
func (pt *PropertyTester) generateSagaState() saga.SagaState {
	states := []saga.SagaState{
		saga.SagaStateStarted,
		saga.SagaStateInProgress,
		saga.SagaStateCompleted,
		saga.SagaStateFailed,
		saga.SagaStateCompensated,
		saga.SagaStateAborted,
	}
	return states[pt.rand.Intn(len(states))]
}

// TestSagaStep is a simple implementation of SagaStep for testing
type TestSagaStep struct {
	name          string
	canCompensate bool
}

func (s *TestSagaStep) Name() string {
	return s.name
}

func (s *TestSagaStep) Execute(ctx context.Context, data map[string]interface{}) error {
	// Simulate step execution
	return nil
}

func (s *TestSagaStep) Compensate(ctx context.Context, data map[string]interface{}) error {
	// Simulate compensation
	return nil
}

func (s *TestSagaStep) CanCompensate() bool {
	return s.canCompensate
}

// generateSagaSteps creates random saga steps
func (pt *PropertyTester) generateSagaSteps() []saga.SagaStep {
	stepNames := []string{"analyze", "dockerfile", "build", "scan", "tag", "push", "manifest", "cluster", "deploy", "verify"}
	stepCount := pt.rand.Intn(len(stepNames)) + 1

	steps := make([]saga.SagaStep, stepCount)
	for i := 0; i < stepCount; i++ {
		steps[i] = &TestSagaStep{
			name:          stepNames[i],
			canCompensate: pt.rand.Float64() < 0.9, // 90% can be compensated
		}
	}

	return steps
}

// generateExecutedSteps creates executed steps based on saga steps
func (pt *PropertyTester) generateExecutedSteps(steps []saga.SagaStep) []saga.SagaStepResult {
	// Randomly execute some steps
	executeCount := pt.rand.Intn(len(steps) + 1)
	executed := make([]saga.SagaStepResult, executeCount)

	baseTime := time.Now().Add(-time.Hour)
	for i := 0; i < executeCount; i++ {
		step := steps[i]
		executed[i] = saga.SagaStepResult{
			StepName:  step.Name(),
			Success:   true,
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Duration:  time.Duration(pt.rand.Intn(60)+10) * time.Second,
			Data:      map[string]interface{}{"success": true},
		}
	}

	return executed
}

// generateCompensatedSteps creates compensated steps based on executed steps and saga state
func (pt *PropertyTester) generateCompensatedSteps(executed []saga.SagaStepResult, state saga.SagaState) []saga.SagaStepResult {
	if state != saga.SagaStateCompensated {
		return []saga.SagaStepResult{}
	}

	// Compensate in reverse order
	compensated := make([]saga.SagaStepResult, len(executed))
	baseTime := time.Now().Add(-30 * time.Minute)

	for i := len(executed) - 1; i >= 0; i-- {
		executedStep := executed[i]
		compensatedIndex := len(executed) - 1 - i
		compensated[compensatedIndex] = saga.SagaStepResult{
			StepName:  executedStep.StepName,
			Success:   true,
			Timestamp: baseTime.Add(time.Duration(compensatedIndex) * time.Minute),
			Duration:  time.Duration(pt.rand.Intn(30)+5) * time.Second,
			Data:      map[string]interface{}{"compensated": true},
		}
	}

	return compensated
}

// Helper generators

func (pt *PropertyTester) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[pt.rand.Intn(len(charset))]
	}
	return string(b)
}

func (pt *PropertyTester) generateRandomRegistry() string {
	registries := []string{
		"docker.io",
		"gcr.io",
		"quay.io",
		"registry.hub.docker.com",
	}
	return registries[pt.rand.Intn(len(registries))]
}

func (pt *PropertyTester) generateImageRef() string {
	return fmt.Sprintf("%s/%s:%s",
		pt.generateRandomRegistry(),
		pt.generateRandomString(12),
		pt.generateRandomString(8))
}

func (pt *PropertyTester) generateImageID() string {
	return fmt.Sprintf("sha256:%s", pt.generateRandomString(64))
}

func (pt *PropertyTester) generateEndpoint() string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:8080",
		pt.generateRandomString(10),
		pt.generateRandomString(8))
}

func (pt *PropertyTester) generateErrorMessage() string {
	errors := []string{
		"dockerfile build failed",
		"network timeout",
		"registry push denied",
		"kubernetes deployment failed",
		"dependency resolution error",
	}
	return errors[pt.rand.Intn(len(errors))]
}

func (pt *PropertyTester) generateDependencies() []string {
	allDeps := []string{"gin", "logrus", "viper", "cobra", "testify"}
	count := pt.rand.Intn(3) + 1
	deps := make([]string, count)
	for i := 0; i < count; i++ {
		deps[i] = allDeps[pt.rand.Intn(len(allDeps))]
	}
	return deps
}
