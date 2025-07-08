package build

import (
	"strings"
	"sync"
)

// BuildFailureService provides build failure analysis without global state
type BuildFailureService struct {
	mu              sync.RWMutex
	failurePatterns map[string][]string
}

// NewBuildFailureService creates a new build failure service
func NewBuildFailureService() *BuildFailureService {
	return &BuildFailureService{
		failurePatterns: getDefaultFailurePatterns(),
	}
}

// getDefaultFailurePatterns returns the default failure pattern mappings
func getDefaultFailurePatterns() map[string][]string {
	return map[string][]string{
		"dockerfile_error":    {"dockerfile", "parse error", "unknown instruction"},
		"dependency_error":    {"dependency", "package", "module", "import"},
		"build_error":         {"build", "compile", "make"},
		"test_failure":        {"test", "spec"},
		"deployment_error":    {"deploy", "kubernetes", "kubectl"},
		"security_issue":      {"security", "vulnerability", "cve"},
		"performance_issue":   {"timeout", "memory", "cpu", "resource"},
		"resource_exhaustion": {"out of", "limit", "quota"},
	}
}

// CategorizeFailure categorizes the type of failure based on the error
func (bfs *BuildFailureService) CategorizeFailure(err error) string {
	if err == nil {
		return "unknown"
	}

	errorText := strings.ToLower(err.Error())

	bfs.mu.RLock()
	defer bfs.mu.RUnlock()

	for category, patterns := range bfs.failurePatterns {
		for _, pattern := range patterns {
			if strings.Contains(errorText, pattern) {
				return category
			}
		}
	}

	return "unknown"
}

// AddFailurePattern adds a new failure pattern for a category
func (bfs *BuildFailureService) AddFailurePattern(category, pattern string) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()

	if patterns, exists := bfs.failurePatterns[category]; exists {
		bfs.failurePatterns[category] = append(patterns, pattern)
	} else {
		bfs.failurePatterns[category] = []string{pattern}
	}
}

// SetFailurePatterns sets custom failure patterns for a category
func (bfs *BuildFailureService) SetFailurePatterns(category string, patterns []string) {
	bfs.mu.Lock()
	defer bfs.mu.Unlock()
	bfs.failurePatterns[category] = patterns
}

// GetFailurePatterns returns all failure patterns for a category
func (bfs *BuildFailureService) GetFailurePatterns(category string) []string {
	bfs.mu.RLock()
	defer bfs.mu.RUnlock()

	if patterns, exists := bfs.failurePatterns[category]; exists {
		// Return a copy to prevent external modification
		result := make([]string, len(patterns))
		copy(result, patterns)
		return result
	}

	return nil
}

// GetAllCategories returns all available failure categories
func (bfs *BuildFailureService) GetAllCategories() []string {
	bfs.mu.RLock()
	defer bfs.mu.RUnlock()

	categories := make([]string, 0, len(bfs.failurePatterns))
	for category := range bfs.failurePatterns {
		categories = append(categories, category)
	}

	return categories
}
