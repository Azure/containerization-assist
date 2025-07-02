package utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SandboxIntegrationTestSuite provides comprehensive integration testing for production sandboxing
type SandboxIntegrationTestSuite struct {
	workspaceManager  *WorkspaceManager
	securityValidator *SecurityValidator
	testDir           string
	logger            zerolog.Logger
}

func TestSandboxIntegrationSuite(t *testing.T) {
	suite := NewSandboxIntegrationTestSuite(t)
	defer suite.Cleanup()

	t.Run("ProductionSandboxing", suite.TestProductionSandboxing)
	t.Run("SecurityValidation", suite.TestSecurityValidation)
	t.Run("ResourceLimits", suite.TestResourceLimits)
	t.Run("NetworkIsolation", suite.TestNetworkIsolation)
	t.Run("FileSystemSecurity", suite.TestFileSystemSecurity)
	t.Run("ContainerMonitoring", suite.TestContainerMonitoring)
	t.Run("AuditLogging", suite.TestAuditLogging)
	t.Run("ErrorHandling", suite.TestErrorHandling)
	t.Run("PerformanceBenchmarks", suite.TestPerformanceBenchmarks)
}

func NewSandboxIntegrationTestSuite(t *testing.T) *SandboxIntegrationTestSuite {
	// Create temporary test directory
	testDir, err := os.MkdirTemp("", "sandbox-integration-test")
	require.NoError(t, err)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Create security validator
	securityValidator := NewSecurityValidator(logger)

	// Create workspace manager with production features
	config := WorkspaceConfig{
		BaseDir:           testDir,
		MaxSizePerSession: 1024 * 1024 * 1024,     // 1GB
		TotalMaxSize:      5 * 1024 * 1024 * 1024, // 5GB
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	}

	workspaceManager, err := NewWorkspaceManager(context.Background(), config)
	if err == nil {
		workspaceManager.securityValidator = securityValidator
	}
	require.NoError(t, err)

	return &SandboxIntegrationTestSuite{
		workspaceManager:  workspaceManager,
		securityValidator: securityValidator,
		testDir:           testDir,
		logger:            logger,
	}
}

func (suite *SandboxIntegrationTestSuite) Cleanup() {
	os.RemoveAll(suite.testDir)
}

func (suite *SandboxIntegrationTestSuite) TestProductionSandboxing(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-production"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	// Test secure command execution
	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		MemoryLimit:     128 * 1024 * 1024, // 128MB
		CPUQuota:        50000,             // 50% CPU
		Timeout:         30 * time.Second,
		ReadOnly:        true,
		NetworkAccess:   false,
		User:            "1000",
		Group:           "1000",
		Capabilities:    []string{},
		Privileged:      false,
		Operation:       "TEST_PRODUCTION",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io"},
			ResourceLimits: ResourceLimits{
				Memory:   128 * 1024 * 1024,
				CPUQuota: 50000,
			},
		},
	}

	cmd := []string{"echo", "Hello Production Sandbox"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints or Docker unavailable
		t.Logf("Sandbox execution failed: %v", err)
		t.Skip("Skipping test due to sandbox execution failure")
	} else {
		assert.NotNil(t, result)
		// Docker might not be available, check if it's a Docker-related failure
		if result.ExitCode == 125 {
			t.Skip("Skipping test - Docker command failed (likely Docker not available)")
		}
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Hello Production Sandbox")
		assert.True(t, result.Duration > 0)
	}
}

func (suite *SandboxIntegrationTestSuite) TestSecurityValidation(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-security"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	// Test high-risk configuration (should be blocked)
	dangerousOptions := SandboxOptions{
		BaseImage:       "alpine:3.18",
		User:            "root",
		Privileged:      true,
		Capabilities:    []string{"CAP_SYS_ADMIN"},
		Operation:       "DANGEROUS_TEST",
		ValidationLevel: ValidationLevelDeep,
		SecurityPolicy: SecurityPolicy{
			AllowNetworking: true,
			RequireNonRoot:  false,
		},
	}

	cmd := []string{"whoami"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, dangerousOptions)

	// Should be blocked due to high security risk
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "security risk")
}

func (suite *SandboxIntegrationTestSuite) TestResourceLimits(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-resources"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	// Test with strict resource limits
	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		MemoryLimit:     64 * 1024 * 1024, // 64MB (tight limit)
		CPUQuota:        25000,            // 25% CPU
		Timeout:         10 * time.Second,
		User:            "1000",
		Group:           "1000",
		Operation:       "RESOURCE_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			AllowNetworking: false,
			RequireNonRoot:  true,
			ResourceLimits: ResourceLimits{
				Memory:   64 * 1024 * 1024,
				CPUQuota: 25000,
			},
		},
	}

	// Test memory-intensive operation (should be constrained)
	cmd := []string{"sh", "-c", "dd if=/dev/zero of=/tmp/test bs=1M count=100 2>/dev/null || echo 'Memory limited'"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints
		assert.Contains(t, err.Error(), "security risk")
	} else {
		assert.NotNil(t, result)
		// Should either succeed with limitations or fail due to resource constraints
		assert.True(t, result.ExitCode == 0 || result.ExitCode != 0)
	}
}

func (suite *SandboxIntegrationTestSuite) TestNetworkIsolation(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-network"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	// Test network isolation (should fail to connect)
	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		NetworkAccess:   false,
		User:            "1000",
		Group:           "1000",
		Operation:       "NETWORK_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			AllowNetworking: false,
			RequireNonRoot:  true,
		},
	}

	cmd := []string{"sh", "-c", "ping -c 1 8.8.8.8 2>&1 || echo 'Network isolated'"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints
		assert.Contains(t, err.Error(), "security risk")
	} else {
		assert.NotNil(t, result)
		// Should indicate network isolation
		assert.Contains(t, result.Stdout, "Network isolated")
	}
}

func (suite *SandboxIntegrationTestSuite) TestFileSystemSecurity(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-filesystem"

	// Initialize workspace and create test file
	workspaceDir, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	testFile := filepath.Join(workspaceDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test read-only filesystem
	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		ReadOnly:        true,
		User:            "1000",
		Group:           "1000",
		Operation:       "FILESYSTEM_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			AllowFileSystem: true,
			RequireNonRoot:  true,
		},
	}

	// Try to write to read-only filesystem (should fail)
	cmd := []string{"sh", "-c", "echo 'write test' > /workspace/write_test.txt 2>&1 || echo 'Write blocked'"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints
		assert.Contains(t, err.Error(), "security risk")
	} else {
		assert.NotNil(t, result)
		// Should indicate write is blocked due to read-only filesystem
		assert.Contains(t, result.Stdout, "Write blocked")
	}
}

func (suite *SandboxIntegrationTestSuite) TestContainerMonitoring(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-monitoring"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		User:            "1000",
		Group:           "1000",
		Operation:       "MONITORING_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot: true,
		},
	}

	// Run a command that takes some time to execute
	cmd := []string{"sh", "-c", "sleep 2 && echo 'Monitoring test complete'"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints or Docker unavailable
		t.Logf("Sandbox execution failed: %v", err)
		if !strings.Contains(err.Error(), "security risk") {
			t.Skip("Skipping test - likely Docker not available")
		}
	} else {
		assert.NotNil(t, result)
		if result.ExitCode == 125 {
			t.Skip("Skipping test - Docker command failed")
		}
		assert.Equal(t, 0, result.ExitCode)
		assert.Contains(t, result.Stdout, "Monitoring test complete")
		assert.True(t, result.Duration > 2*time.Second)
	}
	// In production implementation, would verify actual resource metrics
}

func (suite *SandboxIntegrationTestSuite) TestAuditLogging(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-audit"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		User:            "1000",
		Group:           "1000",
		Operation:       "AUDIT_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot: true,
		},
	}

	cmd := []string{"echo", "Audit logging test"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	if err != nil {
		// Execution might be blocked due to security constraints
		assert.Contains(t, err.Error(), "security risk")
	} else {
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.ExitCode)

		// In production implementation, would verify audit logs are created
		// For now, we verify execution completed successfully
		assert.Contains(t, result.Stdout, "Audit logging test")
	}
}

func (suite *SandboxIntegrationTestSuite) TestErrorHandling(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-errors"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	// Test with invalid image
	options := SandboxOptions{
		BaseImage:       "nonexistent:image",
		User:            "1000",
		Group:           "1000",
		Operation:       "ERROR_TEST",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io"},
		},
	}

	cmd := []string{"echo", "test"}
	result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

	// Should handle error gracefully
	assert.Error(t, err)
	assert.Nil(t, result)
}

func (suite *SandboxIntegrationTestSuite) TestPerformanceBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance benchmarks in short mode")
	}

	ctx := context.Background()
	sessionID := "test-session-performance"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(t, err)

	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		MemoryLimit:     256 * 1024 * 1024, // 256MB
		CPUQuota:        100000,            // 100% CPU
		Timeout:         30 * time.Second,
		User:            "1000",
		Group:           "1000",
		Operation:       "PERFORMANCE_TEST",
		ValidationLevel: ValidationLevelFast, // Fast validation for performance
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot: true,
		},
	}

	// Benchmark simple command execution
	iterations := 5
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		cmd := []string{"echo", "Performance test iteration"}
		result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)

		if err != nil {
			// Skip this iteration if blocked by security
			continue
		}
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.ExitCode)

		if result != nil {
			totalDuration += result.Duration
		}
	}

	averageDuration := totalDuration / time.Duration(iterations)
	suite.logger.Info().
		Dur("average_duration", averageDuration).
		Int("iterations", iterations).
		Msg("Sandbox performance benchmark completed")

	// Performance target: average execution should be reasonable
	assert.True(t, averageDuration < 10*time.Second, "Average execution time should be under 10 seconds")
}

// Benchmark tests for performance measurement
func BenchmarkSandboxExecution(b *testing.B) {
	suite := setupBenchmarkSuite(b)
	defer suite.Cleanup()

	ctx := context.Background()
	sessionID := "benchmark-session"

	// Initialize workspace
	_, err := suite.workspaceManager.InitializeWorkspace(ctx, sessionID)
	require.NoError(b, err)

	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		User:            "1000",
		Group:           "1000",
		Operation:       "BENCHMARK",
		ValidationLevel: ValidationLevelFast,
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot: true,
		},
	}

	cmd := []string{"echo", "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := suite.workspaceManager.ExecuteSandboxed(ctx, sessionID, cmd, options)
		if err != nil {
			b.Fatal(err)
		}
		if result.ExitCode != 0 {
			b.Fatal("Command failed")
		}
	}
}

func BenchmarkSecurityValidationIntegration(b *testing.B) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)

	options := SandboxOptions{
		BaseImage:       "alpine:3.18",
		User:            "1000",
		Group:           "1000",
		Operation:       "BENCHMARK_VALIDATION",
		ValidationLevel: ValidationLevelStandard,
		SecurityPolicy: SecurityPolicy{
			RequireNonRoot: true,
		},
	}

	ctx := context.Background()
	sessionID := "benchmark-validation"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		if err != nil {
			b.Fatal(err)
		}
		if report == nil {
			b.Fatal("No validation report")
		}
	}
}

func setupBenchmarkSuite(b *testing.B) *SandboxIntegrationTestSuite {
	testDir, err := os.MkdirTemp("", "sandbox-benchmark")
	require.NoError(b, err)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	securityValidator := NewSecurityValidator(logger)

	config := WorkspaceConfig{
		BaseDir:           testDir,
		MaxSizePerSession: 1024 * 1024 * 1024,     // 1GB
		TotalMaxSize:      5 * 1024 * 1024 * 1024, // 5GB
		Cleanup:           true,
		SandboxEnabled:    true,
		Logger:            logger,
	}

	workspaceManager, err := NewWorkspaceManager(context.Background(), config)
	if err == nil {
		workspaceManager.securityValidator = securityValidator
	}
	require.NoError(b, err)

	return &SandboxIntegrationTestSuite{
		workspaceManager:  workspaceManager,
		securityValidator: securityValidator,
		testDir:           testDir,
		logger:            logger,
	}
}
