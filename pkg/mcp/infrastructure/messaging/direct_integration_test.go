package messaging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestDirectProgressFactory_CreateEmitter_CLI(t *testing.T) {
	// Create factory
	factory := NewDirectProgressFactory(slog.Default())

	// Create emitter without MCP server (should get CLI emitter)
	ctx := context.Background()
	emitter := factory.CreateEmitter(ctx, nil, 10)

	// Verify we got a CLI emitter
	_, ok := emitter.(*CLIDirectEmitter)
	assert.True(t, ok, "Expected CLIDirectEmitter when no MCP server in context")

	// Test basic emit
	err := emitter.Emit(ctx, "test", 50, "Testing")
	assert.NoError(t, err)

	// Test close
	err = emitter.Close()
	assert.NoError(t, err)
}

func TestDirectProgressFactory_ImplementsInterface(t *testing.T) {
	factory := NewDirectProgressFactory(slog.Default())

	// Verify it implements the workflow interface
	var _ workflow.ProgressEmitterFactory = factory
}

func TestCLIDirectEmitter_EmitDetailed(t *testing.T) {
	emitter := NewCLIDirectEmitter(slog.Default())
	ctx := context.Background()

	tests := []struct {
		name   string
		update api.ProgressUpdate
	}{
		{
			name: "running status",
			update: api.ProgressUpdate{
				Stage:      "build",
				Percentage: 50,
				Message:    "Building",
				Status:     "running",
			},
		},
		{
			name: "failed status",
			update: api.ProgressUpdate{
				Stage:      "deploy",
				Percentage: 75,
				Message:    "Deployment failed",
				Status:     "failed",
			},
		},
		{
			name: "completed status",
			update: api.ProgressUpdate{
				Stage:      "verify",
				Percentage: 100,
				Message:    "Verification complete",
				Status:     "completed",
			},
		},
		{
			name: "warning status",
			update: api.ProgressUpdate{
				Stage:      "scan",
				Percentage: 60,
				Message:    "Vulnerabilities found",
				Status:     "warning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := emitter.EmitDetailed(ctx, tt.update)
			assert.NoError(t, err)
		})
	}
}

func TestCLIDirectEmitter_ProgressBar(t *testing.T) {
	emitter := &CLIDirectEmitter{}

	tests := []struct {
		percent  int
		expected string
	}{
		{0, "[░░░░░░░░░░░░░░░░░░░░]   0%"},
		{25, "[█████░░░░░░░░░░░░░░░]  25%"},
		{50, "[██████████░░░░░░░░░░]  50%"},
		{75, "[███████████████░░░░░]  75%"},
		{100, "[████████████████████] 100%"},
		{-10, "[░░░░░░░░░░░░░░░░░░░░]   0%"}, // Test negative
		{150, "[████████████████████] 100%"}, // Test over 100
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := emitter.formatProgressBar(tt.percent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDirectProgressFactory_WithProgressToken is commented out until we can verify
// the exact structure of mcp.CallToolRequest
// func TestDirectProgressFactory_WithProgressToken(t *testing.T) {
// 	factory := NewDirectProgressFactory(slog.Default())
// 	ctx := context.Background()
//
// 	// Create request with progress token
// 	req := &mcp.CallToolRequest{
// 		Params: mcp.CallToolParams{
// 			Meta: &mcp.CallToolMeta{
// 				ProgressToken: "test-token-123",
// 			},
// 		},
// 	}
//
// 	// Without server in context, should still get CLI emitter
// 	emitter := factory.CreateEmitter(ctx, req, 10)
// 	_, ok := emitter.(*CLIDirectEmitter)
// 	assert.True(t, ok, "Expected CLIDirectEmitter when no server in context, even with token")
// }
