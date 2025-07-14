// Package ai_ml provides unified dependency injection for AI/ML services
package ai_ml

import (
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/ml"
	"github.com/google/wire"
)

// AIMLProviders provides all AI/ML domain dependencies
var AIMLProviders = wire.NewSet(
	// Machine learning services - using existing constructors
	ml.NewErrorPatternRecognizer,
	ml.NewEnhancedErrorHandler,
	ml.NewStepEnhancer,

	// Note: sampling.Client and prompts.Manager are provided in wiring
	// since they need config conversion

	// Interface bindings would go here if needed
)
