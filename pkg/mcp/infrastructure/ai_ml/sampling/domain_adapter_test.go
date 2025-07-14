package sampling

import (
	"context"
	"log/slog"
	"os"
	"testing"

	domain "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainAdapter_Basic(t *testing.T) {
	// Create a basic client and wrap it with the domain adapter
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	// Verify adapter implements UnifiedSampler
	var _ domain.UnifiedSampler = adapter

	t.Log("Domain adapter successfully implements all required interfaces")
}

func TestDomainAdapter_Sample_NoMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()
	req := domain.Request{
		Prompt:    "Test prompt",
		MaxTokens: 100,
	}

	_, err := adapter.Sample(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestDomainAdapter_AnalyzeDockerfile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()
	content := "FROM node:16\nCOPY . .\nRUN npm install\nCMD npm start"

	result, err := adapter.AnalyzeDockerfile(ctx, content)
	// Without MCP server context, should get retry exhaustion error
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "max retry attempts")
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestDomainAdapter_AnalyzeKubernetesManifest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()
	content := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: test:latest
        ports:
        - containerPort: 8080
`

	result, err := adapter.AnalyzeKubernetesManifest(ctx, content)
	// Without MCP server context, should get retry exhaustion error
	require.Error(t, err)
	require.Nil(t, result)
	assert.Contains(t, err.Error(), "max retry attempts")
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestDomainAdapter_RequestConversion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()

	// Test request with advanced parameters
	topP := float32(0.9)
	seed := 42
	req := domain.Request{
		Prompt:      "Test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Advanced: &domain.AdvancedParams{
			TopP:          &topP,
			Seed:          &seed,
			StopSequences: []string{"END"},
		},
	}

	// This should handle the conversion gracefully even without MCP server
	_, err := adapter.Sample(ctx, req)
	assert.Error(t, err) // Expected due to no MCP server
	assert.Contains(t, err.Error(), "no MCP server in context")
}

func TestDomainAdapter_AdvancedParametersConversion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()

	tests := []struct {
		name     string
		advanced *domain.AdvancedParams
		expected func(t *testing.T, err error)
	}{
		{
			name: "all_advanced_parameters",
			advanced: &domain.AdvancedParams{
				TopP:             ptr(float32(0.9)),
				FrequencyPenalty: ptr(float32(0.5)),
				PresencePenalty:  ptr(float32(0.3)),
				StopSequences:    []string{"END", "STOP"},
				Seed:             ptr(42),
				LogitBias:        map[string]float32{"hello": 0.5, "world": -0.5},
			},
			expected: func(t *testing.T, err error) {
				assert.Error(t, err) // Expected due to no MCP server
				assert.Contains(t, err.Error(), "no MCP server in context")
			},
		},
		{
			name: "partial_advanced_parameters",
			advanced: &domain.AdvancedParams{
				TopP:          ptr(float32(0.8)),
				StopSequences: []string{"DONE"},
			},
			expected: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no MCP server in context")
			},
		},
		{
			name:     "no_advanced_parameters",
			advanced: nil,
			expected: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no MCP server in context")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := domain.Request{
				Prompt:      "Test prompt for " + tt.name,
				MaxTokens:   150,
				Temperature: 0.6,
				Advanced:    tt.advanced,
			}

			_, err := adapter.Sample(ctx, req)
			tt.expected(t, err)
		})
	}
}

func TestDomainAdapter_StreamingWithAdvancedParameters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	client := NewClient(logger)
	adapter := NewDomainAdapter(client)

	ctx := context.Background()

	req := domain.Request{
		Prompt:      "Streaming test prompt",
		MaxTokens:   200,
		Temperature: 0.5,
		Advanced: &domain.AdvancedParams{
			TopP:             ptr(float32(0.95)),
			FrequencyPenalty: ptr(float32(0.2)),
			StopSequences:    []string{"STREAM_END"},
			Seed:             ptr(123),
		},
	}

	_, err := adapter.Stream(ctx, req)
	assert.Error(t, err) // Expected due to no MCP server
	assert.Contains(t, err.Error(), "no MCP server in context")
}

// Helper function to create pointer to generic type
func ptr[T any](v T) *T {
	return &v
}
