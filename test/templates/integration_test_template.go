package PACKAGE_NAME

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// IntegrationTestSuite groups related integration tests
type IntegrationTestSuite struct {
	suite.Suite
	
	// Test fixtures
	tempDir     string
	testServer  *TestServer
	testClient  *TestClient
	cleanup     []func()
}

// SetupSuite runs once before all tests in the suite
func (s *IntegrationTestSuite) SetupSuite() {
	var err error
	
	// Create temporary directory for test files
	s.tempDir, err = os.MkdirTemp("", "container-kit-integration-*")
	s.Require().NoError(err)
	s.addCleanup(func() { os.RemoveAll(s.tempDir) })
	
	// Start test server
	s.testServer, err = NewTestServer(&TestServerConfig{
		Port:    0, // Use random available port
		DataDir: filepath.Join(s.tempDir, "data"),
	})
	s.Require().NoError(err)
	
	err = s.testServer.Start()
	s.Require().NoError(err)
	s.addCleanup(func() { s.testServer.Stop() })
	
	// Create test client
	s.testClient = NewTestClient(s.testServer.URL())
	s.addCleanup(func() { s.testClient.Close() })
	
	// Wait for server to be ready
	s.waitForServerReady()
}

// TearDownSuite runs once after all tests in the suite
func (s *IntegrationTestSuite) TearDownSuite() {
	// Run cleanup functions in reverse order
	for i := len(s.cleanup) - 1; i >= 0; i-- {
		s.cleanup[i]()
	}
}

// SetupTest runs before each individual test
func (s *IntegrationTestSuite) SetupTest() {
	// Reset server state
	err := s.testServer.Reset()
	s.Require().NoError(err)
}

// TearDownTest runs after each individual test
func (s *IntegrationTestSuite) TearDownTest() {
	// Clean up test-specific resources if needed
}

// TestFullWorkflow tests the complete end-to-end workflow
func (s *IntegrationTestSuite) TestFullWorkflow() {
	ctx := context.Background()
	
	// Step 1: Initialize session
	session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
		Name: "integration-test-session",
		Metadata: map[string]string{
			"test": "true",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(session)
	s.Assert().NotEmpty(session.ID)
	
	// Step 2: Execute tool
	result, err := s.testClient.ExecuteTool(ctx, &ExecuteToolRequest{
		SessionID: session.ID,
		ToolName:  "test-tool",
		Arguments: map[string]interface{}{
			"input": "test-data",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Assert().Equal("success", result.Status)
	
	// Step 3: Verify side effects
	sessions, err := s.testClient.ListSessions(ctx, &ListSessionsRequest{})
	s.Require().NoError(err)
	s.Assert().Len(sessions, 1)
	s.Assert().Equal(session.ID, sessions[0].ID)
	
	// Step 4: Cleanup
	err = s.testClient.DeleteSession(ctx, session.ID)
	s.Require().NoError(err)
}

// TestConcurrentOperations tests concurrent access scenarios
func (s *IntegrationTestSuite) TestConcurrentOperations() {
	ctx := context.Background()
	
	// Create multiple sessions concurrently
	const numSessions = 10
	sessionChan := make(chan *Session, numSessions)
	errorChan := make(chan error, numSessions)
	
	for i := 0; i < numSessions; i++ {
		go func(index int) {
			session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
				Name: fmt.Sprintf("concurrent-session-%d", index),
			})
			if err != nil {
				errorChan <- err
				return
			}
			sessionChan <- session
		}(i)
	}
	
	// Collect results
	var sessions []*Session
	for i := 0; i < numSessions; i++ {
		select {
		case session := <-sessionChan:
			sessions = append(sessions, session)
		case err := <-errorChan:
			s.Require().NoError(err)
		case <-time.After(30 * time.Second):
			s.Fail("timeout waiting for concurrent operations")
		}
	}
	
	s.Assert().Len(sessions, numSessions)
	
	// Verify all sessions are unique
	sessionIDs := make(map[string]bool)
	for _, session := range sessions {
		s.Assert().False(sessionIDs[session.ID], "duplicate session ID: %s", session.ID)
		sessionIDs[session.ID] = true
	}
}

// TestErrorHandling tests error scenarios
func (s *IntegrationTestSuite) TestErrorHandling() {
	ctx := context.Background()
	
	// Test invalid tool execution
	_, err := s.testClient.ExecuteTool(ctx, &ExecuteToolRequest{
		SessionID: "non-existent-session",
		ToolName:  "test-tool",
		Arguments: map[string]interface{}{},
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "session not found")
	
	// Test invalid tool name
	session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
		Name: "error-test-session",
	})
	s.Require().NoError(err)
	
	_, err = s.testClient.ExecuteTool(ctx, &ExecuteToolRequest{
		SessionID: session.ID,
		ToolName:  "non-existent-tool",
		Arguments: map[string]interface{}{},
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "tool not found")
}

// TestLongRunningOperations tests operations with timeouts
func (s *IntegrationTestSuite) TestLongRunningOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
		Name: "long-running-test",
	})
	s.Require().NoError(err)
	
	// Execute a tool that takes some time
	start := time.Now()
	result, err := s.testClient.ExecuteTool(ctx, &ExecuteToolRequest{
		SessionID: session.ID,
		ToolName:  "slow-tool",
		Arguments: map[string]interface{}{
			"delay": "2s",
		},
	})
	duration := time.Since(start)
	
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Assert().GreaterOrEqual(duration, 2*time.Second)
	s.Assert().Equal("success", result.Status)
}

// TestDataPersistence tests data persistence across server restarts
func (s *IntegrationTestSuite) TestDataPersistence() {
	ctx := context.Background()
	
	// Create session and data
	session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
		Name: "persistence-test",
		Metadata: map[string]string{
			"persistent": "true",
		},
	})
	s.Require().NoError(err)
	originalSessionID := session.ID
	
	// Restart server to test persistence
	s.testServer.Stop()
	err = s.testServer.Start()
	s.Require().NoError(err)
	s.waitForServerReady()
	
	// Verify session still exists
	sessions, err := s.testClient.ListSessions(ctx, &ListSessionsRequest{})
	s.Require().NoError(err)
	
	found := false
	for _, session := range sessions {
		if session.ID == originalSessionID {
			found = true
			s.Assert().Equal("persistence-test", session.Name)
			s.Assert().Equal("true", session.Metadata["persistent"])
			break
		}
	}
	s.Assert().True(found, "session not found after server restart")
}

// Helper methods

func (s *IntegrationTestSuite) addCleanup(fn func()) {
	s.cleanup = append(s.cleanup, fn)
}

func (s *IntegrationTestSuite) waitForServerReady() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	for {
		select {
		case <-ctx.Done():
			s.Require().Fail("timeout waiting for server to be ready")
		default:
			if s.testServer.IsReady() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Run the integration test suite
func TestIntegrationSuite(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}
	
	// Check for required environment or dependencies
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("integration tests disabled by environment variable")
	}
	
	suite.Run(t, new(IntegrationTestSuite))
}

// Table-driven integration test example
func (s *IntegrationTestSuite) TestToolExecution() {
	tests := []struct {
		name        string
		toolName    string
		arguments   map[string]interface{}
		expectError bool
		expectStatus string
	}{
		{
			name:     "valid tool execution",
			toolName: "test-tool",
			arguments: map[string]interface{}{
				"input": "valid-data",
			},
			expectError:  false,
			expectStatus: "success",
		},
		{
			name:     "tool with validation error",
			toolName: "validation-tool",
			arguments: map[string]interface{}{
				"invalid": "data",
			},
			expectError: true,
		},
		{
			name:     "tool with processing error",
			toolName: "error-tool",
			arguments: map[string]interface{}{
				"trigger": "error",
			},
			expectError: true,
		},
	}
	
	ctx := context.Background()
	session, err := s.testClient.CreateSession(ctx, &CreateSessionRequest{
		Name: "tool-execution-test",
	})
	s.Require().NoError(err)
	
	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := s.testClient.ExecuteTool(ctx, &ExecuteToolRequest{
				SessionID: session.ID,
				ToolName:  tt.toolName,
				Arguments: tt.arguments,
			})
			
			if tt.expectError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				s.Assert().Equal(tt.expectStatus, result.Status)
			}
		})
	}
}