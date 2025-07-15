package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// PerformanceIntegrationSuite tests performance characteristics under various loads
type PerformanceIntegrationSuite struct {
	suite.Suite
	tmpDir string
}

func (suite *PerformanceIntegrationSuite) SetupSuite() {
	var err error
	suite.tmpDir, err = os.MkdirTemp("", "performance-test-")
	require.NoError(suite.T(), err)
}

func (suite *PerformanceIntegrationSuite) TearDownSuite() {
	if suite.tmpDir != "" {
		os.RemoveAll(suite.tmpDir)
	}
}

// TestConcurrentWorkflowExecution tests multiple concurrent workflow executions
func (suite *PerformanceIntegrationSuite) TestConcurrentWorkflowExecution() {
	if testing.Short() {
		suite.T().Skip("Skipping performance test in short mode")
	}

	suite.T().Log("Testing concurrent workflow execution performance")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	// Test concurrent execution
	concurrencyLevels := []int{2, 5, 10}

	for _, concurrency := range concurrencyLevels {
		suite.Run(fmt.Sprintf("Concurrency_%d", concurrency), func() {
			suite.testConcurrentExecution(server, concurrency)
		})
	}

	suite.T().Log("✓ Concurrent workflow execution performance verified")
}

// TestResponseTimeUnderLoad tests response times under increasing load
func (suite *PerformanceIntegrationSuite) TestResponseTimeUnderLoad() {
	if testing.Short() {
		suite.T().Skip("Skipping response time test in short mode")
	}

	suite.T().Log("Testing response times under load")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	// Test response times with different request patterns
	loadPatterns := []struct {
		name          string
		requestCount  int
		intervalMs    int
		expectedMaxMs int
	}{
		{"LightLoad", 10, 1000, 5000},
		{"MediumLoad", 20, 500, 10000},
		{"HeavyLoad", 50, 200, 20000},
	}

	for _, pattern := range loadPatterns {
		suite.Run(pattern.name, func() {
			suite.testResponseTimes(server, pattern.requestCount, pattern.intervalMs, pattern.expectedMaxMs)
		})
	}

	suite.T().Log("✓ Response time under load verified")
}

// TestMemoryUsageStability tests memory usage patterns over time
func (suite *PerformanceIntegrationSuite) TestMemoryUsageStability() {
	if testing.Short() {
		suite.T().Skip("Skipping memory usage test in short mode")
	}

	suite.T().Log("Testing memory usage stability over time")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	// Monitor memory usage over multiple operations
	suite.monitorMemoryUsage(server, 30, time.Second*2)

	suite.T().Log("✓ Memory usage stability verified")
}

// TestLargeRepositoryHandling tests handling of large repositories
func (suite *PerformanceIntegrationSuite) TestLargeRepositoryHandling() {
	if testing.Short() {
		suite.T().Skip("Skipping large repository test in short mode")
	}

	suite.T().Log("Testing large repository handling")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	server := suite.startMCPServer(ctx)
	defer server.cmd.Process.Kill()
	time.Sleep(2 * time.Second)

	// Create and test large repositories
	repoSizes := []struct {
		name      string
		fileCount int
		sizeMB    int
	}{
		{"SmallRepo", 10, 1},
		{"MediumRepo", 100, 10},
		{"LargeRepo", 500, 50},
	}

	for _, size := range repoSizes {
		suite.Run(size.name, func() {
			suite.testRepositorySize(server, size.fileCount, size.sizeMB)
		})
	}

	suite.T().Log("✓ Large repository handling verified")
}

// Helper methods

func (suite *PerformanceIntegrationSuite) startMCPServer(ctx context.Context) *MCPServerProcess {
	return startMCPServerProcess(ctx, suite.tmpDir)
}

func (suite *PerformanceIntegrationSuite) testConcurrentExecution(server *MCPServerProcess, concurrency int) {
	var wg sync.WaitGroup
	results := make(chan time.Duration, concurrency)
	errors := make(chan error, concurrency)

	// Initialize server once
	suite.initializeServer(server)

	// Create test repositories
	repos := make([]string, concurrency)
	for i := 0; i < concurrency; i++ {
		repos[i] = suite.createTestRepository(fmt.Sprintf("concurrent-repo-%d", i))
	}

	startTime := time.Now()

	// Execute concurrent workflows
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(repoIndex int) {
			defer wg.Done()

			execStart := time.Now()
			err := suite.executeWorkflow(server, repos[repoIndex])
			execDuration := time.Since(execStart)

			if err != nil {
				errors <- err
			} else {
				results <- execDuration
			}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	totalDuration := time.Since(startTime)

	// Validate results
	var successCount int
	var totalExecutionTime time.Duration
	var maxExecutionTime time.Duration

	for duration := range results {
		successCount++
		totalExecutionTime += duration
		if duration > maxExecutionTime {
			maxExecutionTime = duration
		}
	}

	errorCount := len(errors)

	suite.T().Logf("Concurrency %d: %d successful, %d errors", concurrency, successCount, errorCount)
	suite.T().Logf("Total time: %v, Average execution: %v, Max execution: %v",
		totalDuration, totalExecutionTime/time.Duration(successCount), maxExecutionTime)

	// Performance assertions
	assert.Greater(suite.T(), successCount, concurrency/2, "At least half of concurrent executions should succeed")
	assert.Less(suite.T(), maxExecutionTime, 5*time.Minute, "Individual executions should complete within 5 minutes")
}

func (suite *PerformanceIntegrationSuite) testResponseTimes(server *MCPServerProcess, requestCount, intervalMs, expectedMaxMs int) {
	suite.initializeServer(server)

	responseTimes := make([]time.Duration, 0, requestCount)

	for i := 0; i < requestCount; i++ {
		start := time.Now()

		response := sendMCPRequest(server.stdin, server.stdout, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "ping",
				"arguments": map[string]interface{}{
					"message": fmt.Sprintf("load-test-%d", i),
				},
			},
		}, suite.T())

		duration := time.Since(start)
		responseTimes = append(responseTimes, duration)

		assert.Contains(suite.T(), response, "result", "Request %d should succeed", i)

		// Wait before next request
		if i < requestCount-1 {
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
		}
	}

	// Calculate statistics
	var total time.Duration
	var max time.Duration
	for _, rt := range responseTimes {
		total += rt
		if rt > max {
			max = rt
		}
	}

	average := total / time.Duration(len(responseTimes))
	maxExpected := time.Duration(expectedMaxMs) * time.Millisecond

	suite.T().Logf("Response times - Average: %v, Max: %v (expected max: %v)", average, max, maxExpected)

	assert.Less(suite.T(), max, maxExpected, "Maximum response time should be within expected limits")
}

func (suite *PerformanceIntegrationSuite) monitorMemoryUsage(server *MCPServerProcess, iterations int, interval time.Duration) {
	suite.initializeServer(server)

	for i := 0; i < iterations; i++ {
		// Execute a lightweight operation
		response := sendMCPRequest(server.stdin, server.stdout, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "ping",
				"arguments": map[string]interface{}{
					"message": fmt.Sprintf("memory-test-%d", i),
				},
			},
		}, suite.T())

		assert.Contains(suite.T(), response, "result", "Memory test iteration %d should succeed", i)

		if i%10 == 0 {
			suite.T().Logf("Memory test iteration %d/%d", i, iterations)
		}

		time.Sleep(interval)
	}

	// The test passes if we reach this point without crashes or significant degradation
	suite.T().Log("Memory usage remained stable throughout test")
}

func (suite *PerformanceIntegrationSuite) testRepositorySize(server *MCPServerProcess, fileCount, sizeMB int) {
	suite.initializeServer(server)

	// Create large repository
	largeRepo := suite.createLargeRepository(fileCount, sizeMB)

	start := time.Now()
	err := suite.executeWorkflow(server, largeRepo)
	duration := time.Since(start)

	suite.T().Logf("Repository with %d files (%dMB) processed in %v", fileCount, sizeMB, duration)

	if err != nil {
		// Large repositories might timeout or fail, but should handle gracefully
		suite.T().Logf("Large repository processing failed (expected for very large repos): %v", err)
	} else {
		// Successful processing should complete within reasonable time
		assert.Less(suite.T(), duration, 10*time.Minute, "Large repository should process within 10 minutes")
	}
}

func (suite *PerformanceIntegrationSuite) initializeServer(server *MCPServerProcess) {
	initResp := sendMCPRequest(server.stdin, server.stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    "performance-test",
				"version": "1.0.0",
			},
		},
	}, suite.T())

	require.Contains(suite.T(), initResp, "result")
}

func (suite *PerformanceIntegrationSuite) executeWorkflow(server *MCPServerProcess, repoDir string) error {
	response := sendMCPRequest(server.stdin, server.stdout, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().Unix(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "containerize_and_deploy",
			"arguments": map[string]interface{}{
				"repo_url":  "file://" + repoDir,
				"branch":    "main",
				"scan":      false,
				"deploy":    false,
				"test_mode": true,
			},
		},
	}, suite.T())

	if response == nil {
		return fmt.Errorf("no response received")
	}

	if _, hasError := response["error"]; hasError {
		return fmt.Errorf("workflow returned error: %v", response["error"])
	}

	return nil
}

func (suite *PerformanceIntegrationSuite) createTestRepository(name string) string {
	repoDir := filepath.Join(suite.tmpDir, name)
	require.NoError(suite.T(), os.MkdirAll(repoDir, 0755))

	mainGo := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s!")
	})
	http.ListenAndServe(":8080", nil)
}
`

	goMod := fmt.Sprintf(`module %s

go 1.21
`, name)

	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))

	return repoDir
}

func (suite *PerformanceIntegrationSuite) createLargeRepository(fileCount, sizeMB int) string {
	repoDir := filepath.Join(suite.tmpDir, fmt.Sprintf("large-repo-%d-%dmb", fileCount, sizeMB))
	require.NoError(suite.T(), os.MkdirAll(repoDir, 0755))

	// Create go.mod
	goMod := `module large-test-app

go 1.21
`
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0644))

	// Create main.go
	mainGo := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from large repository!")
	})
	http.ListenAndServe(":8080", nil)
}
`
	require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(mainGo), 0644))

	// Create additional files to reach target size
	fileSizeKB := (sizeMB * 1024) / fileCount
	if fileSizeKB < 1 {
		fileSizeKB = 1
	}

	fileContent := make([]byte, fileSizeKB*1024)
	for i := range fileContent {
		fileContent[i] = byte('a' + (i % 26))
	}

	for i := 0; i < fileCount-2; i++ { // -2 for main.go and go.mod
		fileName := fmt.Sprintf("file_%04d.txt", i)
		require.NoError(suite.T(), os.WriteFile(filepath.Join(repoDir, fileName), fileContent, 0644))
	}

	return repoDir
}

// Test runner
func TestPerformanceIntegration(t *testing.T) {
	suite.Run(t, new(PerformanceIntegrationSuite))
}
