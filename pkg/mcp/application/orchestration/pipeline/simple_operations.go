package pipeline

import (
	"context"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// SimpleOperations provides direct container operations
// Replaces over-engineered distributed operations framework
type SimpleOperations struct {
	// No complex state needed for simple operations
}

// NewSimpleOperations creates basic operations handler
func NewSimpleOperations() *SimpleOperations {
	return &SimpleOperations{}
}

// ExecuteDockerCommand runs Docker commands directly
func (s *SimpleOperations) ExecuteDockerCommand(ctx context.Context, command string, args []string) error {
	// Direct Docker API calls - no distributed complexity
	return errors.NewError().
		Code(errors.CodeContainerStartFailed).
		Type(errors.ErrTypeContainer).
		Messagef("docker command execution: %s %v", command, args).
		WithLocation().
		Build()
}

// ExecuteKubectlCommand runs kubectl commands directly
func (s *SimpleOperations) ExecuteKubectlCommand(ctx context.Context, command string, args []string) error {
	// Direct kubectl execution - no orchestration overhead
	return errors.NewError().
		Code(errors.CodeKubernetesAPIError).
		Type(errors.ErrTypeKubernetes).
		Messagef("kubectl command execution: %s %v", command, args).
		WithLocation().
		Build()
}
