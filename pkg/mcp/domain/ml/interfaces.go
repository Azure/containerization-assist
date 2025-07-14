package ml

import (
	"context"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// ErrorPatternRecognizer defines the interface for ML-based error pattern recognition.
// This interface is implemented by infrastructure layer.
type ErrorPatternRecognizer interface {
	// RecognizePattern analyzes an error and returns pattern classification
	RecognizePattern(ctx context.Context, err error, stepContext *workflow.WorkflowState) (*ErrorClassification, error)

	// GetSimilarErrors finds similar errors from historical data
	GetSimilarErrors(ctx context.Context, err error) ([]HistoricalError, error)
}

// ErrorClassification represents the ML classification of an error
type ErrorClassification struct {
	Category    string                 `json:"category"`
	Confidence  float64                `json:"confidence"`
	Patterns    []string               `json:"patterns"`
	Suggestions []string               `json:"suggestions"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HistoricalError represents a similar error from history
type HistoricalError struct {
	Error      string   `json:"error"`
	Context    string   `json:"context"`
	Solutions  []string `json:"solutions"`
	Similarity float64  `json:"similarity"`
	Timestamp  string   `json:"timestamp"`
}

// EnhancedErrorHandler defines the interface for ML-enhanced error handling.
// This interface is implemented by infrastructure layer.
type EnhancedErrorHandler interface {
	// AnalyzeAndFix attempts to analyze and fix an error using ML
	AnalyzeAndFix(ctx context.Context, err error, state *workflow.WorkflowState) (*ErrorFix, error)

	// SuggestFixes provides fix suggestions without applying them
	SuggestFixes(ctx context.Context, err error, state *workflow.WorkflowState) ([]FixSuggestion, error)
}

// ErrorFix represents an ML-generated error fix
type ErrorFix struct {
	Applied     bool                   `json:"applied"`
	Description string                 `json:"description"`
	Changes     []string               `json:"changes"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FixSuggestion represents a suggested fix
type FixSuggestion struct {
	Description string  `json:"description"`
	Command     string  `json:"command,omitempty"`
	Confidence  float64 `json:"confidence"`
	Risk        string  `json:"risk"`
}

// StepEnhancer defines the interface for ML-based workflow step enhancement.
// This interface is implemented by infrastructure layer.
type StepEnhancer interface {
	// EnhanceStep applies ML optimizations to a workflow step
	EnhanceStep(ctx context.Context, step workflow.Step, state *workflow.WorkflowState) (workflow.Step, error)

	// OptimizeWorkflow suggests workflow optimizations
	OptimizeWorkflow(ctx context.Context, steps []workflow.Step) (*WorkflowOptimization, error)
}

// WorkflowOptimization represents ML-suggested workflow optimizations
type WorkflowOptimization struct {
	Suggestions          []OptimizationSuggestion `json:"suggestions"`
	EstimatedImprovement float64                  `json:"estimated_improvement"`
	Metadata             map[string]interface{}   `json:"metadata,omitempty"`
}

// OptimizationSuggestion represents a single optimization suggestion
type OptimizationSuggestion struct {
	StepName    string  `json:"step_name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`
}
