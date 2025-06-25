package customizer

import (
	"github.com/Azure/container-copilot/pkg/core/analysis"
)

// OptimizationStrategy represents different optimization approaches
type OptimizationStrategy string

const (
	OptimizationSize        OptimizationStrategy = "size"
	OptimizationSpeed       OptimizationStrategy = "speed"
	OptimizationSecurity    OptimizationStrategy = "security"
	OptimizationPerformance OptimizationStrategy = "performance"
	OptimizationBalanced    OptimizationStrategy = "balanced"
)

// TemplateContext provides context for template customization
type TemplateContext struct {
	AnalysisResult *analysis.AnalysisResult
	Language       string
	Framework      string
	Port           int
	Dependencies   []string
	OptStrategy    OptimizationStrategy
	CustomValues   map[string]interface{}
	HasTests       bool
	HasDatabase    bool
	IsWebApp       bool
	HasStaticFiles bool
}
