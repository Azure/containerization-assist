package dispatch

import (
	"context"
	"fmt"
)

// Example: How to adapt an existing tool to the new type-safe system

// AnalyzeRepositoryArgs represents arguments for the analyze repository tool
type AnalyzeRepositoryArgs struct {
	SessionID   string            `json:"session_id"`
	RepoURL     string            `json:"repo_url"`
	Branch      string            `json:"branch,omitempty"`
	Depth       int               `json:"depth,omitempty"`
	ExtraParams map[string]string `json:"extra_params,omitempty"`
}

// Validate implements ToolArgs interface
func (a *AnalyzeRepositoryArgs) Validate() error {
	if a.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if a.RepoURL == "" {
		return fmt.Errorf("repo_url is required")
	}
	return nil
}

// AnalyzeRepositoryResult represents the result of repository analysis
type AnalyzeRepositoryResult struct {
	Success         bool                   `json:"success"`
	Error           error                  `json:"error,omitempty"`
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	PackageManager  string                 `json:"package_manager"`
	Dependencies    []string               `json:"dependencies"`
	Recommendations map[string]interface{} `json:"recommendations"`
}

// IsSuccess implements ToolResult interface
func (r *AnalyzeRepositoryResult) IsSuccess() bool {
	return r.Success
}

// GetError implements ToolResult interface
func (r *AnalyzeRepositoryResult) GetError() error {
	return r.Error
}

// AnalyzeRepositoryToolAdapter adapts the existing tool to the new interface
type AnalyzeRepositoryToolAdapter struct {
	// This would wrap the existing AtomicAnalyzeRepositoryTool
	// For now, we'll simulate the behavior
}

// Execute implements Tool interface
func (t *AnalyzeRepositoryToolAdapter) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assert to our specific args type
	toolArgs, ok := args.(*AnalyzeRepositoryArgs)
	if !ok {
		return nil, fmt.Errorf("invalid argument type: expected *AnalyzeRepositoryArgs")
	}

	// In real implementation, we would use toolArgs to analyze the repository
	// For this example, we'll just verify the args and return simulated results
	_ = toolArgs.SessionID // Would be used to track session
	_ = toolArgs.RepoURL   // Would be used to clone/analyze repo

	// Simulate tool execution
	result := &AnalyzeRepositoryResult{
		Success:        true,
		Language:       "go",
		Framework:      "gin",
		PackageManager: "go mod",
		Dependencies:   []string{"github.com/gin-gonic/gin", "github.com/rs/zerolog"},
		Recommendations: map[string]interface{}{
			"dockerfile": "Use multi-stage build",
			"security":   "Enable dependency scanning",
		},
	}

	return result, nil
}

// GetMetadata implements Tool interface
func (t *AnalyzeRepositoryToolAdapter) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:         "analyze_repository_atomic",
		Description:  "Analyzes repository structure and dependencies",
		Version:      "1.0.0",
		Category:     "analysis",
		Dependencies: []string{},
		Capabilities: []string{"language_detection", "framework_analysis", "dependency_scanning"},
		Requirements: []string{"repository_access"},
		Parameters: map[string]string{
			"session_id": "string (required)",
			"repo_url":   "string (required)",
			"branch":     "string (optional)",
			"depth":      "int (optional)",
		},
		Examples: []ToolExample{
			{
				Name:        "Basic Repository Analysis",
				Description: "Analyze a GitHub repository",
				Input: map[string]interface{}{
					"session_id": "example-session",
					"repo_url":   "https://github.com/example/app",
				},
				Output: map[string]interface{}{
					"language":        "go",
					"framework":       "gin",
					"package_manager": "go mod",
				},
			},
		},
	}
}

// ConvertAnalyzeRepositoryArgs is the generated argument converter
func ConvertAnalyzeRepositoryArgs(args map[string]interface{}) (ToolArgs, error) {
	result := &AnalyzeRepositoryArgs{}

	// Extract session_id
	if v, ok := args["session_id"]; ok {
		if str, ok := v.(string); ok {
			result.SessionID = str
		} else {
			return nil, fmt.Errorf("session_id must be a string")
		}
	}

	// Extract repo_url
	if v, ok := args["repo_url"]; ok {
		if str, ok := v.(string); ok {
			result.RepoURL = str
		} else {
			return nil, fmt.Errorf("repo_url must be a string")
		}
	}

	// Extract optional branch
	if v, ok := args["branch"]; ok {
		if str, ok := v.(string); ok {
			result.Branch = str
		}
	}

	// Extract optional depth
	if v, ok := args["depth"]; ok {
		switch val := v.(type) {
		case int:
			result.Depth = val
		case float64:
			result.Depth = int(val)
		default:
			return nil, fmt.Errorf("depth must be a number")
		}
	}

	// Extract extra params
	if v, ok := args["extra_params"]; ok {
		if params, ok := v.(map[string]interface{}); ok {
			result.ExtraParams = make(map[string]string)
			for k, v := range params {
				result.ExtraParams[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return result, nil
}

// RegisterAnalyzeRepositoryTool registers the tool with the dispatcher
func RegisterAnalyzeRepositoryTool(dispatcher *ToolDispatcher) error {
	return dispatcher.RegisterTool(
		"analyze_repository_atomic",
		func() Tool { return &AnalyzeRepositoryToolAdapter{} },
		ConvertAnalyzeRepositoryArgs,
	)
}
