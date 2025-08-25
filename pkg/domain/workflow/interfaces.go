package workflow

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowOrchestrator handles the execution of containerization workflows
type WorkflowOrchestrator interface {
	Execute(ctx context.Context, req *mcp.CallToolRequest, args *ContainerizeAndDeployArgs) (*ContainerizeAndDeployResult, error)
}

// StepProvider manages workflow steps
type StepProvider interface {
	GetStep(name string) (Step, error)
	ListSteps() []string
}
