package analyze

import (
	"context"
	"log/slog"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ValidateDockerfile provides Dockerfile validation functionality.

// ValidateDockerfile provides a simplified interface for Dockerfile validation.
// This function serves as a backward-compatible wrapper around the core validation logic.
func ValidateDockerfile(ctx context.Context, args AtomicValidateDockerfileArgs) (*AtomicValidateDockerfileResult, error) {
	// Create a basic tool instance for core validation
	tool := createAtomicValidateDockerfileTool(nil, nil, nil, slog.Default())

	// Execute validation using the core implementation
	result, err := tool.executeWithoutProgress(ctx, args)
	if err != nil {
		return nil, err
	}

	// Convert ExtendedValidationResult back to base AtomicValidateDockerfileResult
	if result != nil {
		return &result.AtomicValidateDockerfileResult, nil
	}

	return nil, errors.NewError().Messagef("validation returned no result").Build()
}

// ValidateDockerfileArgs validates the input arguments for Dockerfile validation.
func ValidateDockerfileArgs(args AtomicValidateDockerfileArgs) error {
	if args.SessionID == "" {
		return errors.NewError().Messagef("session ID is required for dockerfile validation").Build()
	}

	// Must provide either path or content
	if args.DockerfilePath == "" && args.DockerfileContent == "" {
		return errors.NewError().Messagef("either dockerfile_path or dockerfile_content is required").Build()
	}

	if args.Severity != "" {
		validSeverities := map[string]bool{
			"info": true, "warning": true, "error": true,
		}
		if !validSeverities[strings.ToLower(args.Severity)] {
			return errors.NewError().Messagef("invalid severity level: %s, valid options are info, warning, error", args.Severity).Build()
		}
	}

	return nil
}
