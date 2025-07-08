package build

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/core"
)

// wrapAddWarning is a helper to add warnings to BuildValidationResult
func wrapAddWarning(result *BuildValidationResult, warn *core.DockerfileValidationWarning) {
	// Build suggestion string
	suggestion := ""
	if warn.Error != nil && warn.Error.Line > 0 {
		suggestion = fmt.Sprintf("Line %d", warn.Error.Line)
		if warn.Error.Column > 0 {
			suggestion += fmt.Sprintf(", Column %d", warn.Error.Column)
		}
		if warn.Error.Rule != "" {
			suggestion += fmt.Sprintf(" (Rule: %s)", warn.Error.Rule)
		}
	}

	field := ""
	if warn.Error != nil && warn.Error.Rule != "" {
		field = warn.Error.Rule
	}

	result.AddWarning(field, warn.Message, warn.Code, nil, suggestion)
}
