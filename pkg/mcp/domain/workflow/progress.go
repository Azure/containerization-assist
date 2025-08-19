package workflow

import (
	"context"
	"fmt"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
)

// EmitStepProgress emits progress for a step
func EmitStepProgress(ctx context.Context, emitter api.ProgressEmitter, stepName string, stepIndex, totalSteps int, err error) {
	if emitter == nil {
		return
	}

	percentage := int(float64(stepIndex) / float64(totalSteps) * 100)

	var message string
	if err != nil {
		message = fmt.Sprintf("Step %s failed: %v", stepName, err)
	} else {
		message = fmt.Sprintf("Step %s completed", stepName)
	}

	// Log but don't fail on progress emission errors
	if emitErr := emitter.Emit(ctx, stepName, percentage, message); emitErr != nil {
		// TODO: Add structured logging when available
	}
}

// EmitWorkflowProgress emits overall workflow progress
func EmitWorkflowProgress(ctx context.Context, emitter api.ProgressEmitter, currentStep, totalSteps int, message string) {
	if emitter == nil {
		return
	}

	percentage := int(float64(currentStep) / float64(totalSteps) * 100)

	// Log but don't fail on progress emission errors
	if emitErr := emitter.Emit(ctx, "workflow", percentage, message); emitErr != nil {
		// TODO: Add structured logging when available
	}
}
