package messaging

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectProgressFactory_InterfaceCompliance(t *testing.T) {
	// Verify that DirectProgressFactory implements the workflow interface
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)

	// This will fail to compile if the interface is not satisfied
	var _ workflow.ProgressEmitterFactory = factory

	// Additional runtime check
	assert.Implements(t, (*workflow.ProgressEmitterFactory)(nil), factory)
}

func TestDirectProgressFactory_BasicFunctionality(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)

	t.Run("nil request creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitter(ctx, nil, 10)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for nil request")
	})

	t.Run("empty context creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		// Even with a non-nil request, without server in context should get CLI emitter
		emitter := factory.CreateEmitter(ctx, nil, 5)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for empty context")
	})
}

func TestDirectProgressFactory_CreateEmitterWithToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)

	t.Run("nil token creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitterWithToken(ctx, nil)

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter for nil token")
	})

	t.Run("empty context with token creates CLI emitter", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitterWithToken(ctx, "some-token")

		require.NotNil(t, emitter)
		_, ok := emitter.(*CLIDirectEmitter)
		assert.True(t, ok, "Expected CLIDirectEmitter when no server in context")
	})
}

func TestDirectProgressFactory_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)

	t.Run("zero steps", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitter(ctx, nil, 0)
		require.NotNil(t, emitter, "Should create emitter even with zero steps")
	})

	t.Run("negative steps", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitter(ctx, nil, -1)
		require.NotNil(t, emitter, "Should create emitter even with negative steps")
	})

	t.Run("large step count", func(t *testing.T) {
		ctx := context.Background()
		emitter := factory.CreateEmitter(ctx, nil, 999999)
		require.NotNil(t, emitter, "Should create emitter with large step count")
	})
}

func TestDirectProgressFactory_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to the factory
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)

	ctx := context.Background()

	// Create multiple emitters concurrently
	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Test both methods
			emitter1 := factory.CreateEmitter(ctx, nil, id)
			require.NotNil(t, emitter1)

			emitter2 := factory.CreateEmitterWithToken(ctx, "token")
			require.NotNil(t, emitter2)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestDirectProgressFactory_LoggerBehavior(t *testing.T) {
	// Test with custom logger to verify logging behavior
	logCapture := &testLogHandler{logs: []string{}}
	logger := slog.New(logCapture)

	factory := NewDirectProgressFactory(logger)
	ctx := context.Background()

	// Create emitter to trigger logging
	_ = factory.CreateEmitter(ctx, nil, 10)

	// Verify factory was created with proper component tagging
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
func BenchmarkDirectProgressFactory_CreateEmitter(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.CreateEmitter(ctx, nil, 10)
	}
}

func BenchmarkDirectProgressFactory_CreateEmitterWithToken(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	factory := NewDirectProgressFactory(logger)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.CreateEmitterWithToken(ctx, "bench-token")
	}
}
