package transport

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioErrorHandler_HandleToolError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	tests := []struct {
		name     string
		err      error
		toolName string
		wantType string
	}{
		{
			name:     "simple error",
			err:      errors.New("test error"),
			toolName: "test_tool",
			wantType: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := handler.HandleToolError(ctx, tt.toolName, tt.err)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify the error response structure
			if resultMap, ok := result.(map[string]interface{}); ok {
				if errorField, ok := resultMap["error"].(map[string]interface{}); ok {
					assert.Contains(t, errorField["message"], tt.err.Error())
				}
			} else {
				t.Errorf("Expected map[string]interface{} but got %T", result)
			}
		})
	}
}

func TestStdioErrorHandler_HandleGenericError(t *testing.T) {
	logger := zerolog.Nop()
	handler := NewStdioErrorHandler(logger)

	// Create a simple error
	genericErr := fmt.Errorf("test error")

	result := handler.handleGenericError(genericErr, "build_image")

	// Verify result contains formatted error
	assert.NotNil(t, result)
	if resultMap, ok := result.(map[string]interface{}); ok {
		if errorField, ok := resultMap["error"].(map[string]interface{}); ok {
			assert.Contains(t, errorField["message"], "test error")
		}
	}
}
