package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/utils"
	"github.com/rs/zerolog"
)

const (
	// Performance target: <300μs P95 per request
	TargetP95Duration = 300 * time.Microsecond
)

// setupBenchmarkServer creates a minimal session manager for benchmarks
func setupBenchmarkServer(b *testing.B) *session.SessionManager {
	b.Helper()

	tmpDir := b.TempDir()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel) // Reduce noise

	sessionManager, err := session.NewSessionManager(session.SessionManagerConfig{
		WorkspaceDir:      tmpDir,
		StorePath:         filepath.Join(tmpDir, "benchmark_sessions.db"),
		MaxSessions:       10000, // High limit for benchmarks
		SessionTTL:        1 * time.Hour,
		MaxDiskPerSession: 1024 * 1024 * 100,  // 100MB per session
		TotalDiskLimit:    1024 * 1024 * 1024, // 1GB total
		Logger:            logger,
	})
	if err != nil {
		b.Fatalf("Failed to create session manager: %v", err)
	}

	b.Cleanup(func() {
		sessionManager.Stop()
	})

	return sessionManager
}

// BenchmarkSessionOperations benchmarks basic session operations
func BenchmarkSessionOperations(b *testing.B) {
	sessionManager := setupBenchmarkServer(b)

	b.Run("CreateSession", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			sessionID := fmt.Sprintf("bench-session-%d", i)
			_, err := sessionManager.GetOrCreateSession(sessionID)
			if err != nil {
				b.Fatalf("Failed to create session: %v", err)
			}
		}
	})

	b.Run("GetSession", func(b *testing.B) {
		// Pre-create a session
		testSessionID := "bench-get-session"
		_, err := sessionManager.GetOrCreateSession(testSessionID)
		if err != nil {
			b.Fatalf("Failed to create test session: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := sessionManager.GetSession(testSessionID)
			if err != nil {
				b.Fatalf("Failed to get session: %v", err)
			}
		}
	})

	b.Run("UpdateSession", func(b *testing.B) {
		// Pre-create a session
		testSessionID := "bench-update-session"
		_, err := sessionManager.GetOrCreateSession(testSessionID)
		if err != nil {
			b.Fatalf("Failed to create test session: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := sessionManager.UpdateSession(testSessionID, func(s *sessiontypes.SessionState) {
				s.RepoURL = fmt.Sprintf("https://github.com/test/repo-%d", i)
			})
			if err != nil {
				b.Fatalf("Failed to update session: %v", err)
			}
		}
	})
}

// BenchmarkUtilOperations benchmarks utility operations
func BenchmarkUtilOperations(b *testing.B) {
	b.Run("LogBufferOperations", func(b *testing.B) {
		// Initialize log capture
		utils.InitializeLogCapture(1000)
		logBuffer := utils.GetGlobalLogBuffer()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			entry := utils.LogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Test log entry %d", i),
			}
			logBuffer.Add(entry)
		}
	})

	b.Run("LogBufferRead", func(b *testing.B) {
		// Initialize log capture with test data
		utils.InitializeLogCapture(1000)
		logBuffer := utils.GetGlobalLogBuffer()

		// Pre-populate with data
		for i := 0; i < 100; i++ {
			entry := utils.LogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   fmt.Sprintf("Test log entry %d", i),
			}
			logBuffer.Add(entry)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = logBuffer.Size()
		}
	})
}

// BenchmarkSchemaProcessing benchmarks schema processing utilities
func BenchmarkSchemaProcessing(b *testing.B) {
	b.Run("RemoveCopilotIncompatible", func(b *testing.B) {
		testSchema := map[string]any{
			"$schema":               "https://json-schema.org/draft/2020-12/schema",
			"$id":                   "test",
			"$dynamicRef":           "#meta",
			"unevaluatedProperties": false,
			"properties": map[string]any{
				"sessionId": map[string]any{
					"type": "string",
				},
			},
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Clone the map for each iteration
			schemaCopy := make(map[string]any)
			for k, v := range testSchema {
				schemaCopy[k] = v
			}
			removeCopilotIncompatible(schemaCopy)
		}
	})
}

// BenchmarkConcurrentSessionOperations tests concurrent session access
func BenchmarkConcurrentSessionOperations(b *testing.B) {
	sessionManager := setupBenchmarkServer(b)

	// Pre-create a session
	testSessionID := "concurrent-bench-session"
	_, err := sessionManager.GetOrCreateSession(testSessionID)
	if err != nil {
		b.Fatalf("Failed to create test session: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := sessionManager.GetSession(testSessionID)
			if err != nil {
				b.Fatalf("Concurrent session access failed: %v", err)
			}
		}
	})
}

// reportP95Performance calculates and reports P95 latency
func reportP95Performance(b *testing.B, durations []time.Duration) {
	if len(durations) == 0 {
		return
	}

	// Sort durations for percentile calculation
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p95Index := int(float64(len(durations)) * 0.95)
	if p95Index >= len(durations) {
		p95Index = len(durations) - 1
	}

	p95Duration := durations[p95Index]

	b.Logf("P95 latency: %v (target: %v)", p95Duration, TargetP95Duration)

	if p95Duration > TargetP95Duration {
		b.Logf("WARNING: P95 latency %v exceeds target %v", p95Duration, TargetP95Duration)
	}
}

// BenchmarkPerformanceTracking demonstrates latency tracking
func BenchmarkPerformanceTracking(b *testing.B) {
	sessionManager := setupBenchmarkServer(b)

	// Create test sessions
	for i := 0; i < 10; i++ {
		_, err := sessionManager.GetOrCreateSession(fmt.Sprintf("latency-bench-session-%d", i))
		if err != nil {
			b.Fatalf("Failed to create test session: %v", err)
		}
	}

	var durations []time.Duration

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()

		// Simple session get operation
		_, err := sessionManager.GetSession("latency-bench-session-0")
		if err != nil {
			b.Fatalf("GetSession failed: %v", err)
		}

		duration := time.Since(start)
		durations = append(durations, duration)
	}

	b.StopTimer()
	reportP95Performance(b, durations)
}
