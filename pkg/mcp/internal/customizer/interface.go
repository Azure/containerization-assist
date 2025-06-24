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
	OptimizationSize         OptimizationStrategy = "size"
	OptimizationSpeed        OptimizationStrategy = "speed"
	OptimizationSecurity     OptimizationStrategy = "security"
	OptimizationPerformance  OptimizationStrategy = "performance"
	OptimizationBalanced     OptimizationStrategy = "balanced"
)

// TemplateContext provides context for template customization
type TemplateContext struct {
	AnalysisResult   *analysis.AnalysisResult
	Language         string
	Framework        string
	Port             int
	Dependencies     []string
	OptStrategy      OptimizationStrategy
	CustomValues     map[string]interface{}
	HasTests         bool
	HasDatabase      bool
	IsWebApp         bool
	HasStaticFiles   bool
}