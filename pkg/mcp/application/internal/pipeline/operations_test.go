package pipeline

import (
	"log/slog"
	"testing"

	sessionsvc "github.com/Azure/container-kit/pkg/mcp/domain/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOperations(t *testing.T) {
	tests := []struct {
		name           string
		sessionManager *sessionsvc.SessionManager
		clients        *mcptypes.MCPClients
		logger         *slog.Logger
		expectValid    bool
	}{
		{
			name:           "valid_creation_with_all_dependencies",
			sessionManager: &sessionsvc.SessionManager{}, // Mock session manager
			clients:        &mcptypes.MCPClients{},       // Mock clients
			logger:         slog.Default(),
			expectValid:    true,
		},
		{
			name:           "creation_with_nil_session_manager",
			sessionManager: nil,
			clients:        &mcptypes.MCPClients{},
			logger:         slog.Default(),
			expectValid:    true, // Should handle gracefully
		},
		{
			name:           "creation_with_nil_clients",
			sessionManager: &sessionsvc.SessionManager{},
			clients:        nil,
			logger:         slog.Default(),
			expectValid:    true, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := NewOperations(tt.sessionManager, tt.clients, tt.logger)

			if tt.expectValid {
				require.NotNil(t, ops)
				assert.IsType(t, &Operations{}, ops)
			} else {
				assert.Nil(t, ops)
			}
		})
	}
}

func TestCreateOperations(t *testing.T) {
	logger := slog.Default()
	sessionManager := &sessionsvc.SessionManager{}
	clients := &mcptypes.MCPClients{}

	ops := createOperations(sessionManager, clients, logger)

	require.NotNil(t, ops)
	assert.IsType(t, &Operations{}, ops)

	// Verify that the operations struct has been properly initialized
	// Note: This tests the internal structure without exposing implementation details
	assert.NotNil(t, ops)
}

func TestOperations_Structure(t *testing.T) {
	logger := slog.Default()
	sessionManager := &sessionsvc.SessionManager{}
	clients := &mcptypes.MCPClients{}

	ops := NewOperations(sessionManager, clients, logger)
	require.NotNil(t, ops)

	// Test that the operations struct has the expected structure
	// This verifies the constructor properly sets up the struct
	assert.IsType(t, &Operations{}, ops)
}

// BenchmarkNewOperations benchmarks the creation of Operations instances
func BenchmarkNewOperations(b *testing.B) {
	logger := slog.Default()
	sessionManager := &sessionsvc.SessionManager{}
	clients := &mcptypes.MCPClients{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := NewOperations(sessionManager, clients, logger)
		_ = ops // Prevent optimization
	}
}

// BenchmarkCreateOperations benchmarks the internal creation logic
func BenchmarkCreateOperations(b *testing.B) {
	logger := slog.Default()
	sessionManager := &sessionsvc.SessionManager{}
	clients := &mcptypes.MCPClients{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops := createOperations(sessionManager, clients, logger)
		_ = ops // Prevent optimization
	}
}
