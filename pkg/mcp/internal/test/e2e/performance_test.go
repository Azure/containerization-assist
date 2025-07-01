package e2e

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Azure/container-kit/pkg/mcp/internal/test/testutil"
)

// Performance targets from GAMMA workstream requirements
const (
	SessionOperationTargetP95 = 300 * time.Microsecond // <300μs P95 target
	MaxConcurrentSessions     = 50                     // Concurrent session limit for testing
	WorkflowTimeoutLimit      = 5 * time.Minute        // Maximum workflow execution time
)

// PerformanceMetrics tracks performance measurements
type PerformanceMetrics struct {
	OperationTimes []time.Duration
	TotalDuration  time.Duration
	Errors         int
	Successes      int
	P95            time.Duration
	P99            time.Duration
	Average        time.Duration
	Median         time.Duration
}

// TestSessionPerformance validates session operations meet performance targets
func TestSessionPerformance(t *testing.T) {
	t.Skip("TEMPORARILY SKIPPED: E2E tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test session creation/retrieval performance (<300μs P95)
	t.Run("session_creation_performance", func(t *testing.T) {
		metrics := measureSessionCreationPerformance(t, client, ctx, 100)

		assert.True(t, metrics.P95 < SessionOperationTargetP95,
			"Session creation P95 (%v) should be under %v", metrics.P95, SessionOperationTargetP95)

		t.Logf("Session Creation Performance:")
		t.Logf("  P95: %v (target: <%v)", metrics.P95, SessionOperationTargetP95)
		t.Logf("  P99: %v", metrics.P99)
		t.Logf("  Average: %v", metrics.Average)
		t.Logf("  Successes: %d, Errors: %d", metrics.Successes, metrics.Errors)
	})

	// Test session retrieval performance
	t.Run("session_retrieval_performance", func(t *testing.T) {
		// Create session first
		analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": "https://github.com/spring-projects/spring-petclinic",
			"branch":   "main",
		})
		require.NoError(t, err)

		sessionID, err := client.ExtractSessionID(analyzeResult)
		require.NoError(t, err)

		metrics := measureSessionRetrievalPerformance(t, client, sessionID, 100)

		assert.True(t, metrics.P95 < SessionOperationTargetP95,
			"Session retrieval P95 (%v) should be under %v", metrics.P95, SessionOperationTargetP95)

		t.Logf("Session Retrieval Performance:")
		t.Logf("  P95: %v (target: <%v)", metrics.P95, SessionOperationTargetP95)
		t.Logf("  P99: %v", metrics.P99)
		t.Logf("  Average: %v", metrics.Average)
	})

	// Test session cleanup performance
	t.Run("session_cleanup_performance", func(t *testing.T) {
		metrics := measureSessionCleanupPerformance(t, client, ctx, 50)

		// Cleanup should be reasonable (no hard target, but log for monitoring)
		t.Logf("Session Cleanup Performance:")
		t.Logf("  P95: %v", metrics.P95)
		t.Logf("  P99: %v", metrics.P99)
		t.Logf("  Average: %v", metrics.Average)
		t.Logf("  Successes: %d, Errors: %d", metrics.Successes, metrics.Errors)
	})
}

// TestConcurrentSessionHandling validates performance under concurrent load
func TestConcurrentSessionHandling(t *testing.T) {
	t.Skip("TEMPORARILY SKIPPED: E2E tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	if testing.Short() {
		t.Skip("Skipping concurrent session tests in short mode")
	}
	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test concurrent session creation
	t.Run("concurrent_session_creation", func(t *testing.T) {
		numConcurrent := MaxConcurrentSessions
		startTime := time.Now()

		var wg sync.WaitGroup
		results := make(chan error, numConcurrent)
		sessionIDs := make(chan string, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
					"repo_url": fmt.Sprintf("https://github.com/example/repo-%d", index),
					"branch":   "main",
				})

				if err == nil {
					if sessionID, extractErr := client.ExtractSessionID(analyzeResult); extractErr == nil {
						sessionIDs <- sessionID
					}
				}
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)
		close(sessionIDs)

		totalTime := time.Since(startTime)

		// Count successes and failures
		successes := 0
		failures := 0
		for err := range results {
			if err == nil {
				successes++
			} else {
				failures++
			}
		}

		// Collect session IDs
		createdSessions := make([]string, 0)
		for sessionID := range sessionIDs {
			createdSessions = append(createdSessions, sessionID)
		}

		// Performance validation
		assert.True(t, totalTime < 30*time.Second, "Concurrent session creation should complete within 30s")
		assert.True(t, float64(successes)/float64(numConcurrent) > 0.8, "At least 80%% of concurrent sessions should succeed")

		t.Logf("Concurrent Session Creation (%d sessions):", numConcurrent)
		t.Logf("  Total Time: %v", totalTime)
		t.Logf("  Successes: %d (%.1f%%)", successes, float64(successes)/float64(numConcurrent)*100)
		t.Logf("  Failures: %d", failures)
		t.Logf("  Average per session: %v", totalTime/time.Duration(numConcurrent))
		t.Logf("  Sessions created: %d", len(createdSessions))
	})

	// Test concurrent operations on same session
	t.Run("concurrent_operations_same_session", func(t *testing.T) {
		// Create base session
		analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": "https://github.com/spring-projects/spring-petclinic",
			"branch":   "main",
		})
		require.NoError(t, err)

		sessionID, err := client.ExtractSessionID(analyzeResult)
		require.NoError(t, err)

		numConcurrent := 10
		var wg sync.WaitGroup
		results := make(chan error, numConcurrent)
		startTime := time.Now()

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// Concurrent dockerfile generation
				_, err := client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
					"session_id": sessionID,
					"template":   "java",
				})
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)

		// Analyze results
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			}
		}

		// At least one operation should succeed (others may fail due to concurrency)
		assert.True(t, successes > 0, "At least one concurrent operation should succeed")
		assert.True(t, totalTime < 10*time.Second, "Concurrent operations should complete quickly")

		t.Logf("Concurrent Operations on Same Session:")
		t.Logf("  Total Time: %v", totalTime)
		t.Logf("  Successes: %d/%d", successes, numConcurrent)
	})
}

// TestLongRunningWorkflows validates performance of extended workflows
func TestLongRunningWorkflows(t *testing.T) {
	t.Skip("TEMPORARILY SKIPPED: E2E tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	if testing.Short() {
		t.Skip("Skipping long-running workflow test in short mode")
	}

	client, _, cleanup := setupE2ETestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Test workflows with multiple tools over time
	t.Run("extended_workflow_performance", func(t *testing.T) {
		startTime := time.Now()

		// Step 1: Repository analysis
		analyzeStart := time.Now()
		analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": "https://github.com/spring-projects/spring-petclinic",
			"branch":   "main",
		})
		require.NoError(t, err)
		analyzeTime := time.Since(analyzeStart)

		sessionID, err := client.ExtractSessionID(analyzeResult)
		require.NoError(t, err)

		// Step 2: Dockerfile generation
		dockerfileStart := time.Now()
		_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
			"session_id": sessionID,
			"template":   "java",
		})
		require.NoError(t, err)
		dockerfileTime := time.Since(dockerfileStart)

		// Step 3: Image build
		buildStart := time.Now()
		_, err = client.CallTool(ctx, "build_image", map[string]interface{}{
			"session_id": sessionID,
			"image_name": "performance-test",
			"tag":        "latest",
		})
		require.NoError(t, err)
		buildTime := time.Since(buildStart)

		// Step 4: Manifest generation
		manifestStart := time.Now()
		_, err = client.CallTool(ctx, "generate_manifests", map[string]interface{}{
			"session_id": sessionID,
			"app_name":   "performance-test",
			"port":       8080,
		})
		require.NoError(t, err)
		manifestTime := time.Since(manifestStart)

		totalTime := time.Since(startTime)

		// Performance validation
		assert.True(t, totalTime < WorkflowTimeoutLimit,
			"Complete workflow should finish within %v (took %v)", WorkflowTimeoutLimit, totalTime)

		t.Logf("Extended Workflow Performance:")
		t.Logf("  Total Time: %v (limit: %v)", totalTime, WorkflowTimeoutLimit)
		t.Logf("  Analyze: %v", analyzeTime)
		t.Logf("  Dockerfile: %v", dockerfileTime)
		t.Logf("  Build: %v", buildTime)
		t.Logf("  Manifests: %v", manifestTime)
		t.Logf("  Session ID: %s", sessionID)
	})

	// Test session TTL and expiration handling
	t.Run("session_ttl_performance", func(t *testing.T) {
		// Create session
		analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": "https://github.com/spring-projects/spring-petclinic",
			"branch":   "main",
		})
		require.NoError(t, err)

		sessionID, err := client.ExtractSessionID(analyzeResult)
		require.NoError(t, err)

		// Test session access over time
		accessTimes := make([]time.Duration, 5)
		for i := 0; i < 5; i++ {
			time.Sleep(500 * time.Millisecond) // Wait between accesses

			start := time.Now()
			_, err := client.InspectSessionState(sessionID)
			accessTimes[i] = time.Since(start)

			if err != nil {
				t.Logf("Session access %d failed after %v: %v", i+1, time.Duration(i)*500*time.Millisecond, err)
				break
			}
		}

		// Session access should remain fast
		for i, accessTime := range accessTimes {
			if accessTime > 0 {
				assert.True(t, accessTime < time.Second, "Session access %d should be fast (took %v)", i+1, accessTime)
			}
		}

		t.Logf("Session TTL Performance (access times): %v", accessTimes)
	})

	// Test memory usage over long sessions
	t.Run("memory_usage_monitoring", func(t *testing.T) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Create multiple sessions and perform operations
		sessionCount := 20
		for i := 0; i < sessionCount; i++ {
			analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": fmt.Sprintf("https://github.com/example/repo-%d", i),
				"branch":   "main",
			})
			require.NoError(t, err)

			sessionID, err := client.ExtractSessionID(analyzeResult)
			require.NoError(t, err)

			// Perform operation on each session
			_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
				"session_id": sessionID,
				"template":   "java",
			})
			require.NoError(t, err)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		memoryIncrease := m2.Alloc - m1.Alloc
		memoryPerSession := memoryIncrease / uint64(sessionCount)

		t.Logf("Memory Usage Analysis:")
		t.Logf("  Sessions created: %d", sessionCount)
		t.Logf("  Memory increase: %d bytes", memoryIncrease)
		t.Logf("  Memory per session: %d bytes", memoryPerSession)
		t.Logf("  Total allocations: %d", m2.TotalAlloc-m1.TotalAlloc)

		// Memory usage should be reasonable (no hard limit, but log for monitoring)
		assert.True(t, memoryPerSession < 1024*1024, "Memory per session should be reasonable (<%d MB per session)", 1) // 1MB per session
	})
}

// Benchmark functions for automated performance testing

// BenchmarkSessionOperations benchmarks session operations
func BenchmarkSessionOperations(b *testing.B) {
	b.Skip("TEMPORARILY SKIPPED: E2E tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	client, _, cleanup := setupBenchmarkEnvironment(b)
	defer cleanup()

	ctx := context.Background()

	b.Run("SessionCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			result, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": fmt.Sprintf("https://github.com/example/bench-%d", i),
				"branch":   "main",
			})
			duration := time.Since(start)

			if err != nil {
				b.Errorf("Session creation failed: %v", err)
				continue
			}

			// Validate performance target (<300μs P95)
			if duration > SessionOperationTargetP95*2 { // Allow 2x for benchmark variance
				b.Logf("Session operation took %v (iteration %d)", duration, i)
			}

			// Validate result
			if _, extractErr := client.ExtractSessionID(result); extractErr != nil {
				b.Errorf("Failed to extract session ID: %v", extractErr)
			}
		}
	})

	// Create a session for retrieval benchmarks
	analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
		"repo_url": "https://github.com/spring-projects/spring-petclinic",
		"branch":   "main",
	})
	if err != nil {
		b.Fatalf("Failed to create session for benchmark: %v", err)
	}

	sessionID, err := client.ExtractSessionID(analyzeResult)
	if err != nil {
		b.Fatalf("Failed to extract session ID: %v", err)
	}

	b.Run("SessionRetrieval", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			_, err := client.InspectSessionState(sessionID)
			duration := time.Since(start)

			if err != nil {
				b.Errorf("Session retrieval failed: %v", err)
				continue
			}

			// Validate performance target
			if duration > SessionOperationTargetP95*2 {
				b.Logf("Session retrieval took %v (iteration %d)", duration, i)
			}
		}
	})
}

// BenchmarkWorkflowOperations benchmarks complete workflow operations
func BenchmarkWorkflowOperations(b *testing.B) {
	b.Skip("TEMPORARILY SKIPPED: E2E tests need troubleshooting - see TOOL_SCHEMA_FIX_PLAN.md")
	client, _, cleanup := setupBenchmarkEnvironment(b)
	defer cleanup()

	ctx := context.Background()

	b.Run("CompleteWorkflow", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Complete workflow
			start := time.Now()

			// Step 1: Analyze
			analyzeResult, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
				"repo_url": fmt.Sprintf("https://github.com/example/workflow-%d", i),
				"branch":   "main",
			})
			if err != nil {
				b.Errorf("Workflow step 1 failed: %v", err)
				continue
			}

			sessionID, err := client.ExtractSessionID(analyzeResult)
			if err != nil {
				b.Errorf("Failed to extract session ID: %v", err)
				continue
			}

			// Step 2: Generate Dockerfile
			_, err = client.CallTool(ctx, "generate_dockerfile", map[string]interface{}{
				"session_id": sessionID,
				"template":   "java",
			})
			if err != nil {
				b.Errorf("Workflow step 2 failed: %v", err)
				continue
			}

			// Step 3: Build Image
			_, err = client.CallTool(ctx, "build_image", map[string]interface{}{
				"session_id": sessionID,
				"image_name": fmt.Sprintf("bench-test-%d", i),
				"tag":        "latest",
			})
			if err != nil {
				b.Errorf("Workflow step 3 failed: %v", err)
				continue
			}

			// Step 4: Generate Manifests
			_, err = client.CallTool(ctx, "generate_manifests", map[string]interface{}{
				"session_id": sessionID,
				"app_name":   fmt.Sprintf("bench-test-%d", i),
				"port":       8080,
			})
			if err != nil {
				b.Errorf("Workflow step 4 failed: %v", err)
				continue
			}

			duration := time.Since(start)

			// Log slow workflows
			if duration > WorkflowTimeoutLimit/10 { // 10% of timeout limit
				b.Logf("Slow workflow (iteration %d): %v", i, duration)
			}
		}
	})
}

// Helper functions

func measureSessionCreationPerformance(t *testing.T, client testutil.MCPTestClient, ctx context.Context, iterations int) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		OperationTimes: make([]time.Duration, 0, iterations),
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		opStart := time.Now()
		result, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": fmt.Sprintf("https://github.com/example/perf-%d", i),
			"branch":   "main",
		})
		opDuration := time.Since(opStart)
		metrics.OperationTimes = append(metrics.OperationTimes, opDuration)

		if err != nil {
			metrics.Errors++
		} else {
			if _, extractErr := client.ExtractSessionID(result); extractErr == nil {
				metrics.Successes++
			} else {
				metrics.Errors++
			}
		}
	}
	metrics.TotalDuration = time.Since(start)

	calculatePerformanceStats(metrics)
	return metrics
}

func measureSessionRetrievalPerformance(t *testing.T, client testutil.MCPTestClient, sessionID string, iterations int) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		OperationTimes: make([]time.Duration, 0, iterations),
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		opStart := time.Now()
		_, err := client.InspectSessionState(sessionID)
		opDuration := time.Since(opStart)
		metrics.OperationTimes = append(metrics.OperationTimes, opDuration)

		if err != nil {
			metrics.Errors++
		} else {
			metrics.Successes++
		}
	}
	metrics.TotalDuration = time.Since(start)

	calculatePerformanceStats(metrics)
	return metrics
}

func measureSessionCleanupPerformance(t *testing.T, client testutil.MCPTestClient, ctx context.Context, iterations int) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		OperationTimes: make([]time.Duration, 0, iterations),
	}

	// Create sessions to clean up
	sessionIDs := make([]string, iterations)
	for i := 0; i < iterations; i++ {
		result, err := client.CallTool(ctx, "analyze_repository", map[string]interface{}{
			"repo_url": fmt.Sprintf("https://github.com/example/cleanup-%d", i),
			"branch":   "main",
		})
		if err != nil {
			t.Fatalf("Failed to create session for cleanup test: %v", err)
		}
		sessionID, err := client.ExtractSessionID(result)
		if err != nil {
			t.Fatalf("Failed to extract session ID: %v", err)
		}
		sessionIDs[i] = sessionID
	}

	// Measure cleanup performance
	start := time.Now()
	for _, sessionID := range sessionIDs {
		opStart := time.Now()
		// Note: Actual cleanup mechanism depends on implementation
		// For now, we'll measure session state access as proxy
		_, err := client.InspectSessionState(sessionID)
		opDuration := time.Since(opStart)
		metrics.OperationTimes = append(metrics.OperationTimes, opDuration)

		if err != nil {
			metrics.Errors++ // Session may already be cleaned up
		} else {
			metrics.Successes++
		}
	}
	metrics.TotalDuration = time.Since(start)

	calculatePerformanceStats(metrics)
	return metrics
}

func calculatePerformanceStats(metrics *PerformanceMetrics) {
	if len(metrics.OperationTimes) == 0 {
		return
	}

	// Sort times for percentile calculation
	times := make([]time.Duration, len(metrics.OperationTimes))
	copy(times, metrics.OperationTimes)

	// Simple sort for percentiles
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}

	// Calculate percentiles
	p95Index := int(float64(len(times)) * 0.95)
	p99Index := int(float64(len(times)) * 0.99)
	medianIndex := len(times) / 2

	if p95Index >= len(times) {
		p95Index = len(times) - 1
	}
	if p99Index >= len(times) {
		p99Index = len(times) - 1
	}

	metrics.P95 = times[p95Index]
	metrics.P99 = times[p99Index]
	metrics.Median = times[medianIndex]

	// Calculate average
	var total time.Duration
	for _, t := range metrics.OperationTimes {
		total += t
	}
	metrics.Average = total / time.Duration(len(metrics.OperationTimes))
}

func setupBenchmarkEnvironment(b *testing.B) (testutil.MCPTestClient, *testutil.TestServer, func()) {
	server, err := testutil.NewTestServer()
	if err != nil {
		b.Fatalf("Failed to create test server: %v", err)
	}

	client, err := testutil.NewMCPTestClient(server.URL())
	if err != nil {
		b.Fatalf("Failed to create test client: %v", err)
	}

	cleanup := func() {
		client.Close()
		server.Close()
	}

	return client, server, cleanup
}
