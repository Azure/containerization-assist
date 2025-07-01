package build

// ToolAnalyzer provides analysis capabilities for atomic tools
type ToolAnalyzer interface {
	// AnalyzeBuildFailure analyzes build operation failures
	AnalyzeBuildFailure(sessionID, imageName string) error
	// AnalyzePushFailure analyzes push operation failures
	AnalyzePushFailure(imageRef, sessionID string) error
	// AnalyzePullFailure analyzes pull operation failures
	AnalyzePullFailure(imageRef, sessionID string) error
	// AnalyzeTagFailure analyzes tag operation failures
	AnalyzeTagFailure(sourceImage, targetImage, sessionID string) error
}

// DefaultToolAnalyzer provides a default implementation
type DefaultToolAnalyzer struct {
	toolName string
}

// NewDefaultToolAnalyzer creates a new default tool analyzer
func NewDefaultToolAnalyzer(toolName string) *DefaultToolAnalyzer {
	return &DefaultToolAnalyzer{
		toolName: toolName,
	}
}

// AnalyzeBuildFailure analyzes build failures
func (a *DefaultToolAnalyzer) AnalyzeBuildFailure(sessionID, imageName string) error {
	// Default implementation - could be enhanced with actual analysis
	return nil
}

// AnalyzePushFailure analyzes push failures
func (a *DefaultToolAnalyzer) AnalyzePushFailure(imageRef, sessionID string) error {
	// Default implementation - could be enhanced with actual analysis
	return nil
}

// AnalyzePullFailure analyzes pull failures
func (a *DefaultToolAnalyzer) AnalyzePullFailure(imageRef, sessionID string) error {
	// Default implementation - could be enhanced with actual analysis
	return nil
}

// AnalyzeTagFailure analyzes tag failures
func (a *DefaultToolAnalyzer) AnalyzeTagFailure(sourceImage, targetImage, sessionID string) error {
	// Default implementation - could be enhanced with actual analysis
	return nil
}
