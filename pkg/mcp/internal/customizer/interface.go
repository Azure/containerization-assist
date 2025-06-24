package customizer

import (
	"github.com/Azure/container-copilot/pkg/core/analysis"
)

// Customizer is the base interface for all customizers
type Customizer interface {
	// Name returns the customizer name
	Name() string

	// Validate validates the customization options
	Validate() error
}

// OptimizationStrategy represents different optimization approaches
type OptimizationStrategy string

const (
	OptimizationSize     OptimizationStrategy = "size"
	OptimizationSpeed    OptimizationStrategy = "speed"
	OptimizationSecurity OptimizationStrategy = "security"
	OptimizationDefault  OptimizationStrategy = "default"
)

// TemplateContext provides common context for template selection
type TemplateContext struct {
	Language       string
	Framework      string
	HasTests       bool
	HasDatabase    bool
	IsWebApp       bool
	HasStaticFiles bool
	Dependencies   []analysis.Dependency
}
