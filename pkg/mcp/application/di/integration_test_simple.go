package di

import (
	"testing"
)

func TestDIContainerSimple(t *testing.T) {
	// Test that the Wire container can be successfully initialized
	container, err := InitializeContainer()
	if err != nil {
		t.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Verify all services are properly injected
	if container.ToolRegistry == nil {
		t.Fatal("ToolRegistry not injected")
	}

	if container.SessionStore == nil {
		t.Fatal("SessionStore not injected")
	}

	if container.SessionState == nil {
		t.Fatal("SessionState not injected")
	}

	if container.BuildExecutor == nil {
		t.Fatal("BuildExecutor not injected")
	}

	if container.WorkflowExecutor == nil {
		t.Fatal("WorkflowExecutor not injected")
	}

	if container.Scanner == nil {
		t.Fatal("Scanner not injected")
	}

	if container.ConfigValidator == nil {
		t.Fatal("ConfigValidator not injected")
	}

	if container.ErrorReporter == nil {
		t.Fatal("ErrorReporter not injected")
	}
}
