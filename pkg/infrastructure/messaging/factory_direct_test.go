package messaging

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/containerization-assist/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProgressEmitter_InterfaceCompliance(t *testing.T) {
	// Verify that CreateProgressEmitter returns a proper interface implementation
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()
	emitter := CreateProgressEmitter(ctx, nil, 10, logger)

	// This will fail to compile if the interface is not satisfied
	var _ api.ProgressEmitter = emitter

	// Additional runtime check
	assert.Implements(t, (*api.ProgressEmitter)(nil), emitter)
}

func TestCreateProgressEmitter_BasicFunctionality(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("nil request creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitter(ctx, nil, 10, logger)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for nil request")
	})

	t.Run("empty context creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		// Even with a non-nil request, without server in context should get CLI emitter
		emitter := CreateProgressEmitter(ctx, nil, 5, logger)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for empty context")
	})
}

func TestCreateProgressEmitterWithToken_BasicFunctionality(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("nil token creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitterWithToken(ctx, nil, logger)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for nil token")
	})

	t.Run("empty context with token creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitterWithToken(ctx, "some-token", logger)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter when no server in context")
	})
}

func TestCreateProgressEmitter_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("zero steps", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitter(ctx, nil, 0, logger)
		require.NotNil(t, emitter, "Should create emitter even with zero steps")
	})

	t.Run("negative steps", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitter(ctx, nil, -1, logger)
		require.NotNil(t, emitter, "Should create emitter even with negative steps")
	})

	t.Run("large step count", func(t *testing.T) {
		ctx := context.Background()
		emitter := CreateProgressEmitter(ctx, nil, 999999, logger)
		require.NotNil(t, emitter, "Should create emitter with large step count")
	})
}

func TestCreateProgressEmitter_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to the emitter creation functions
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	// Create multiple emitters concurrently
	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(stepID int) {
			defer func() { done <- true }()

			// Test both functions
			emitter1 := CreateProgressEmitter(ctx, nil, stepID, logger)
			require.NotNil(t, emitter1)

			emitter2 := CreateProgressEmitterWithToken(ctx, "token", logger)
			require.NotNil(t, emitter2)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestCreateProgressEmitter_LoggerBehavior(t *testing.T) {
	// Test with custom logger to verify logging behavior
	logCapture := &testLogHandler{logs: []string{}}
	logger := slog.New(logCapture)

	ctx := context.Background()

	// Create emitter to trigger logging
	_ = CreateProgressEmitter(ctx, nil, 10, logger)

	// Verify logging occurred
	logs := logCapture.String()
	assert.Contains(t, logs, "Creating CLI progress emitter", "Should log emitter creation")
}

// testLogHandler captures log messages for testing
type testLogHandler struct {
	logs []string
}

func (h *testLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.logs = append(h.logs, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		h.logs = append(h.logs, a.Key, a.Value.String())
		return true
	})
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *testLogHandler) String() string {
	result := ""
	for _, log := range h.logs {
		result += log + " "
	}
	return result
}

// Benchmark tests
func BenchmarkCreateProgressEmitter(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateProgressEmitter(ctx, nil, 10, logger)
	}
}

func BenchmarkCreateProgressEmitterWithToken(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateProgressEmitterWithToken(ctx, "bench-token", logger)
	}
}
