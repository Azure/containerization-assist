package workflow

import (
	"errors"
	"fmt"
	"testing"
)

func TestWorkflowError(t *testing.T) {
	baseErr := fmt.Errorf("connection timeout")

	t.Run("Error message formatting", func(t *testing.T) {
		err := NewWorkflowError("build", 1, baseErr)
		expected := "step 'build' failed: connection timeout"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("Error message with retry attempt", func(t *testing.T) {
		err := NewWorkflowError("build", 3, baseErr)
		expected := "step 'build' failed on attempt 3: connection timeout"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("Unwrap returns wrapped error", func(t *testing.T) {
		err := NewWorkflowError("deploy", 1, baseErr)
		unwrapped := err.Unwrap()
		if unwrapped != baseErr {
			t.Errorf("Expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})

	t.Run("errors.Is works with wrapped error", func(t *testing.T) {
		specificErr := fmt.Errorf("specific error")
		err := NewWorkflowError("analyze", 2, specificErr)

		// This should work because WorkflowError implements Unwrap
		if !errors.Is(err, specificErr) {
			t.Errorf("errors.Is should identify the wrapped error")
		}
	})

	t.Run("errors.As works with WorkflowError", func(t *testing.T) {
		err := NewWorkflowError("scan", 1, baseErr)

		var workflowErr *WorkflowError
		if !errors.As(err, &workflowErr) {
			t.Errorf("errors.As should work with WorkflowError")
		}

		if workflowErr.Step != "scan" {
			t.Errorf("Expected step to be 'scan', got %s", workflowErr.Step)
		}
	})
}
