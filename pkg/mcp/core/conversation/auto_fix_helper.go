package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// AutoFixHelper provides automatic error recovery and fixing capabilities
type AutoFixHelper struct {
	logger *slog.Logger
	fixes  map[string]FixStrategy
}

// FixStrategy defines a function that attempts to fix an error
type FixStrategy func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error)

// NewAutoFixHelper creates a new auto-fix helper
func NewAutoFixHelper(logger *slog.Logger) *AutoFixHelper {
	helper := &AutoFixHelper{
		logger: logger,
		fixes:  make(map[string]FixStrategy),
	}

	// Register common fix strategies
	helper.registerCommonFixes()

	return helper
}

// registerCommonFixes registers common error fix strategies
func (h *AutoFixHelper) registerCommonFixes() {
	// Fix for missing Dockerfile
	h.fixes["dockerfile_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "Dockerfile not found") || strings.Contains(err.Error(), "dockerfile not found") {
			h.logger.Info("Attempting to fix missing Dockerfile error")

			// Try common Dockerfile locations
			buildArgs, ok := args.(*BuildArgs)
			if !ok {
				// Try to extract from map
				if argsMap, ok := args.(map[string]interface{}); ok {
					buildArgs = &BuildArgs{}
					if dp, ok := argsMap["dockerfile_path"].(string); ok {
						buildArgs.DockerfilePath = dp
					}
					if cp, ok := argsMap["context_path"].(string); ok {
						buildArgs.ContextPath = cp
					}
				} else {
					return nil, err
				}
			}

			// Try alternative Dockerfile names
			alternatives := []string{"dockerfile", "Dockerfile.dev", "docker/Dockerfile", ".dockerfile"}
			originalPath := buildArgs.DockerfilePath

			for _, alt := range alternatives {
				buildArgs.DockerfilePath = alt
				h.logger.Debug("Trying alternative Dockerfile path",
					slog.String("path", alt))

				// Create new tool input with updated args
				toolInput := api.ToolInput{
					Data: map[string]interface{}{
						"dockerfile_path": alt,
						"context_path":    buildArgs.ContextPath,
					},
				}

				if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
					h.logger.Info("Auto-fixed Dockerfile location",
						slog.String("original", originalPath),
						slog.String("fixed", alt))
					return result, nil
				}
			}
		}
		return nil, err
	}

	// Fix for missing build context
	h.fixes["context_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "context path does not exist") || strings.Contains(err.Error(), "context path error") {
			h.logger.Info("Attempting to fix missing build context error")

			buildArgs, ok := args.(*BuildArgs)
			if !ok {
				// Try to extract from map
				if argsMap, ok := args.(map[string]interface{}); ok {
					buildArgs = &BuildArgs{}
					if cp, ok := argsMap["context_path"].(string); ok {
						buildArgs.ContextPath = cp
					}
				} else {
					return nil, err
				}
			}

			// Try current directory as context
			originalContext := buildArgs.ContextPath
			buildArgs.ContextPath = "."

			// Create new tool input with updated args
			toolInput := api.ToolInput{
				Data: map[string]interface{}{
					"context_path": ".",
				},
			}

			if result, execErr := tool.Execute(ctx, toolInput); execErr == nil {
				h.logger.Info("Auto-fixed build context to current directory",
					slog.String("original", originalContext))
				return result, nil
			}
		}
		return nil, err
	}

	// Fix for authentication errors
	h.fixes["auth_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "authentication required") || strings.Contains(err.Error(), "unauthorized") {
			h.logger.Warn("Authentication required, cannot auto-fix",
				slog.String("error", err.Error()))

			// Return a helpful error message
			return nil, fmt.Errorf("authentication required: please ensure Docker is logged in to the registry")
		}
		return nil, err
	}

	// Fix for network errors
	h.fixes["network_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "network") || strings.Contains(err.Error(), "connection") {
			h.logger.Info("Network error detected, suggesting retry")

			// Could implement retry logic here
			return nil, fmt.Errorf("network error: %w. Please check your internet connection and try again", err)
		}
		return nil, err
	}

	// Fix for permission errors
	h.fixes["permission_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "access denied") {
			h.logger.Warn("Permission error detected",
				slog.String("error", err.Error()))

			return nil, fmt.Errorf("permission denied: ensure you have the necessary permissions to perform this operation")
		}
		return nil, err
	}

	// Fix for disk space errors
	h.fixes["disk_space_error"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
		if strings.Contains(err.Error(), "no space left on device") || strings.Contains(err.Error(), "disk full") {
			h.logger.Error("Disk space error detected",
				slog.String("error", err.Error()))

			return nil, fmt.Errorf("insufficient disk space: please free up disk space and try again. Consider running 'docker system prune'")
		}
		return nil, err
	}
}

// AttemptFix attempts to fix an error using registered strategies
func (h *AutoFixHelper) AttemptFix(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
	if err == nil {
		return nil, nil
	}

	h.logger.Debug("Attempting auto-fix",
		slog.String("error", err.Error()),
		slog.String("tool", tool.Name()))

	// Try each fix strategy
	for fixName, strategy := range h.fixes {
		h.logger.Debug("Trying fix strategy", slog.String("strategy", fixName))

		result, fixErr := strategy(ctx, tool, args, err)
		if fixErr == nil && result != nil {
			h.logger.Info("Auto-fix successful",
				slog.String("fix", fixName),
				slog.String("original_error", err.Error()))
			return result, nil
		}
	}

	// No fix worked, return original error
	h.logger.Debug("No auto-fix strategy succeeded",
		slog.String("error", err.Error()))
	return nil, err
}

// RegisterFix registers a custom fix strategy
func (h *AutoFixHelper) RegisterFix(name string, strategy FixStrategy) {
	h.fixes[name] = strategy
}

// HasFix checks if a fix strategy exists for the given name
func (h *AutoFixHelper) HasFix(name string) bool {
	_, exists := h.fixes[name]
	return exists
}

// ListFixes returns the names of all registered fix strategies
func (h *AutoFixHelper) ListFixes() []string {
	fixes := make([]string, 0, len(h.fixes))
	for name := range h.fixes {
		fixes = append(fixes, name)
	}
	return fixes
}
