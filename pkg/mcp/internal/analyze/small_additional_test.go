package analyze

import (
	"testing"

	"github.com/rs/zerolog"
)

// Simple test to add a bit more coverage
func TestOrchestrator_RegisterEngineVariations(t *testing.T) {
	logger := zerolog.Nop()
	orchestrator := NewAnalysisOrchestrator(logger)

	// Test registering multiple engines
	engines := []*MockAnalysisEngine{
		{
			name:         "engine-1",
			capabilities: []string{"cap1"},
			applicable:   true,
		},
		{
			name:         "engine-2",
			capabilities: []string{"cap2", "cap3"},
			applicable:   false,
		},
		{
			name:         "engine-3",
			capabilities: []string{},
			applicable:   true,
		},
	}

	for _, engine := range engines {
		orchestrator.RegisterEngine(engine)
	}

	// This just exercises the registration code paths
}

// Test isURL and validateLocalPath to get more coverage
func TestURL_ValidationUtilities(t *testing.T) {
	logger := zerolog.Nop()
	cloner := NewCloner(logger)

	// Test isURL indirectly through validateCloneOptions
	options1 := CloneOptions{
		RepoURL:   "https://github.com/example/repo.git",
		Branch:    "main",
		TargetDir: "/tmp/test",
		SessionID: "test",
	}

	err := cloner.validateCloneOptions(options1)
	if err != nil {
		t.Errorf("Valid URL options should not return error, got: %v", err)
	}

	// Test with non-URL (should also be valid as local path)
	options2 := CloneOptions{
		RepoURL:   "/local/path/to/repo",
		Branch:    "main",
		TargetDir: "/tmp/test",
		SessionID: "test",
	}

	err = cloner.validateCloneOptions(options2)
	if err != nil {
		t.Errorf("Valid local path options should not return error, got: %v", err)
	}

	// Test with empty URL (should fail)
	options3 := CloneOptions{
		RepoURL:   "",
		Branch:    "main",
		TargetDir: "/tmp/test",
		SessionID: "test",
	}

	err = cloner.validateCloneOptions(options3)
	if err == nil {
		t.Error("Empty URL should return validation error")
	}
}
