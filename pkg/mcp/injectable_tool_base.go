package mcp

import (
	"fmt"

	"github.com/rs/zerolog"
)

// =============================================================================
// Injectable Tool Interfaces
// =============================================================================

// InjectableTool extends the basic Tool interface with dependency injection support
type InjectableTool interface {
	Tool
	InjectableClientProvider

	// Validation
	ValidateClients(requiredClients []string) error
}

// InjectableToolFactory creates tools with dependency injection support
type InjectableToolFactory[T InjectableTool] interface {
	Create(clientFactory ClientFactory) T
	GetRequiredClients() []string
	GetMetadata() ToolMetadata
}

// InjectableToolBase provides a base implementation for tools that need dependency injection
type InjectableToolBase struct {
	BaseInjectableClients
	logger zerolog.Logger
	name   string
}

// NewInjectableToolBase creates a new injectable tool base
func NewInjectableToolBase(name string, logger zerolog.Logger) *InjectableToolBase {
	return &InjectableToolBase{
		logger: logger.With().Str("tool", name).Logger(),
		name:   name,
	}
}

// GetLogger returns the tool logger
func (t *InjectableToolBase) GetLogger() zerolog.Logger {
	return t.logger
}

// GetName returns the tool name
func (t *InjectableToolBase) GetName() string {
	return t.name
}

// ValidateClients ensures all required clients are available
func (t *InjectableToolBase) ValidateClients(requiredClients []string) error {
	if t.clientFactory == nil {
		return fmt.Errorf("client factory not injected")
	}

	for _, clientType := range requiredClients {
		switch clientType {
		case "docker":
			if client := t.clientFactory.CreateDockerClient(); client == nil {
				return fmt.Errorf("docker client not available")
			}
		case "k8s", "kubernetes":
			if client := t.clientFactory.CreateK8sClient(); client == nil {
				return fmt.Errorf("kubernetes client not available")
			}
		case "kind":
			if client := t.clientFactory.CreateKindClient(); client == nil {
				return fmt.Errorf("kind client not available")
			}
		case "ai":
			if client := t.clientFactory.CreateAIClient(); client == nil {
				return fmt.Errorf("ai client not available")
			}
		default:
			t.logger.Warn().Str("client_type", clientType).Msg("Unknown client type for validation")
		}
	}

	return nil
}
