// Package integration provides integration tests for the Containerization Assist MCP server
package integration_test

import (
	"os"
	"testing"
)

// TestMain sets up the integration test environment
func TestMain(m *testing.M) {
	// Set up any global test configuration
	os.Setenv("CONTAINER_KIT_TEST_MODE", "true")

	// Run tests
	code := m.Run()

	// Clean up
	os.Exit(code)
}

// TestIntegrationSuite verifies that integration tests are properly configured
func TestIntegrationSuite(t *testing.T) {
	t.Log("Containerization Assist MCP Server Integration Test Suite")

	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Verify test environment
	if os.Getenv("CONTAINER_KIT_TEST_MODE") != "true" {
		t.Fatal("Test environment not properly configured")
	}

	t.Log("Integration test suite is ready. Run individual test files for specific functionality.")
}
