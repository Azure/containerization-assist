// Package docker provides core Docker operations extracted from the Containerization Assist pipeline.
// This package contains mechanical Docker operations without AI dependencies,
// designed to be used by atomic MCP tools that let external AI handle reasoning.
package docker

import (
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
)

// ContainerizeOptions contains all options for the containerization process
type ContainerizeOptions struct {
	// Template options
	TemplateName string
	Language     string
	Framework    string
	Dependencies []string
	ConfigFiles  []string

	// Build options
	ImageName    string
	Registry     string
	NoCache      bool
	Platform     string
	BuildArgs    map[string]string
	BuildTimeout time.Duration

	// Push options
	AutoPush    bool
	RetryCount  int
	PushTimeout time.Duration
}

// ContainerizationResult contains the complete result of containerization
type ContainerizationResult struct {
	Success bool `json:"success"`

	// Template generation results
	Template *GenerateResult `json:"template,omitempty"`

	// Validation results
	Validation *api.BuildValidationResult `json:"validation,omitempty"`

	// Build results
	Build *BuildResult `json:"build,omitempty"`

	// Push results (if requested)
	Push *RegistryPushResult `json:"push,omitempty"`

	// Overall context
	Duration time.Duration          `json:"duration"`
	Context  map[string]interface{} `json:"context"`
	Error    string                 `json:"error,omitempty"`
}

// ServiceImpl methods are implemented in service.go
