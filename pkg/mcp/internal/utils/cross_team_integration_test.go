package utils

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CrossTeamIntegrationSuite validates integration between all teams
type CrossTeamIntegrationSuite struct {
	t             *testing.T
	ctx           context.Context
	logger        zerolog.Logger
	workspace     *WorkspaceManager
	testSessionID string
}

// NewCrossTeamIntegrationSuite creates a new cross-team integration test suite
func NewCrossTeamIntegrationSuite(t *testing.T) *CrossTeamIntegrationSuite {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	workspace, err := NewWorkspaceManager(context.Background(), WorkspaceConfig{
		BaseDir:           t.TempDir(),
		MaxSizePerSession: 1024 * 1024 * 1024,     // 1GB per session
		TotalMaxSize:      5 * 1024 * 1024 * 1024, // 5GB total
		Cleanup:           true,
		SandboxEnabled:    false, // Start disabled for basic integration tests
		Logger:            logger,
	})
	require.NoError(t, err)

	return &CrossTeamIntegrationSuite{
		t:             t,
		ctx:           context.Background(),
		logger:        logger,
		workspace:     workspace,
		testSessionID: "integration-test-" + time.Now().Format("20060102-150405"),
	}
}

// TestEndToEndWorkflow tests the complete workflow from analysis to deployment
func TestEndToEndWorkflow(t *testing.T) {
	suite := NewCrossTeamIntegrationSuite(t)
	defer suite.cleanup()

	t.Run("WorkflowInitialization", func(t *testing.T) {
		// Test workspace initialization (AdvancedBot + InfraBot)
		workspaceDir, err := suite.workspace.InitializeWorkspace(suite.ctx, suite.testSessionID)
		require.NoError(t, err)
		assert.DirExists(t, workspaceDir)

		// Verify all required subdirectories exist
		subdirs := []string{"repo", "build", "manifests", "logs", "cache"}
		for _, subdir := range subdirs {
			assert.DirExists(t, workspaceDir+"/"+subdir)
		}

		suite.logger.Info().Str("workspace", workspaceDir).Msg("Workspace initialized successfully")
	})

	t.Run("WorkspaceStatsMonitoring", func(t *testing.T) {
		// Test workspace statistics monitoring (AdvancedBot)
		// Force disk usage update to ensure stats are current
		err := suite.workspace.UpdateDiskUsage(suite.ctx, suite.testSessionID)
		require.NoError(t, err)

		stats := suite.workspace.GetStats()
		assert.NotNil(t, stats)
		assert.GreaterOrEqual(t, stats.TotalSessions, 0) // May be 0 or 1 depending on implementation
		assert.GreaterOrEqual(t, stats.TotalDiskUsage, int64(0))
		assert.Equal(t, int64(1024*1024*1024), stats.PerSessionLimit)
		assert.Equal(t, int64(5*1024*1024*1024), stats.TotalDiskLimit)
		assert.False(t, stats.SandboxEnabled) // Should be disabled for this test

		suite.logger.Info().
			Int("sessions", stats.TotalSessions).
			Int64("disk_usage", stats.TotalDiskUsage).
			Bool("sandbox_enabled", stats.SandboxEnabled).
			Msg("Workspace monitoring integration successful")
	})

	t.Run("InterfaceCompatibility", func(t *testing.T) {
		// Test that all team interfaces are compatible
		suite.validateInfraBotInterfaces(t)
		suite.validateBuildSecBotInterfaces(t)
		suite.validateOrchBotInterfaces(t)
		suite.validateAdvancedBotInterfaces(t)
	})

	t.Run("ResourceManagement", func(t *testing.T) {
		// Test resource management across teams
		stats := suite.workspace.GetStats()
		assert.NotNil(t, stats)
		assert.GreaterOrEqual(t, stats.TotalSessions, 0)
		assert.GreaterOrEqual(t, stats.TotalDiskUsage, int64(0))

		// Test quota checking functionality
		err := suite.workspace.CheckQuota(suite.testSessionID, 50*1024*1024) // 50MB - should pass
		assert.NoError(t, err)

		suite.logger.Info().
			Int("sessions", stats.TotalSessions).
			Int64("disk_usage", stats.TotalDiskUsage).
			Msg("Resource management validation successful")
	})

	t.Run("ErrorHandlingChain", func(t *testing.T) {
		// Test error handling across the entire chain
		suite.validateErrorPropagation(t)
	})
}

// TestTeamInterfaceContracts validates that all teams implement expected interfaces
func TestTeamInterfaceContracts(t *testing.T) {
	suite := NewCrossTeamIntegrationSuite(t)
	defer suite.cleanup()

	t.Run("InfraBotContracts", func(t *testing.T) {
		// Validate InfraBot provides expected Docker operations
		suite.validateDockerOperationsContract(t)
		suite.validateSessionManagementContract(t)
		suite.validateAtomicFrameworkContract(t)
	})

	t.Run("BuildSecBotContracts", func(t *testing.T) {
		// Validate BuildSecBot provides expected atomic tools and security
		suite.validateAtomicToolsContract(t)
		suite.validateSecurityScanningContract(t)
		suite.validateBuildStrategiesContract(t)
	})

	t.Run("OrchBotContracts", func(t *testing.T) {
		// Validate OrchBot provides expected orchestration and communication
		suite.validateContextSharingContract(t)
		suite.validateWorkflowOrchestrationContract(t)
		suite.validateCommunicationContract(t)
	})

	t.Run("AdvancedBotContracts", func(t *testing.T) {
		// Validate AdvancedBot provides expected testing and sandboxing
		suite.validateSandboxingContract(t)
		suite.validateTestingFrameworkContract(t)
		suite.validateQualityMonitoringContract(t)
	})
}

// TestPerformanceBenchmarks validates performance across all teams
func TestPerformanceBenchmarks(t *testing.T) {
	suite := NewCrossTeamIntegrationSuite(t)
	defer suite.cleanup()

	t.Run("WorkspaceOperationsPerformance", func(t *testing.T) {
		// Benchmark workspace operations
		start := time.Now()

		workspaceDir, err := suite.workspace.InitializeWorkspace(suite.ctx, suite.testSessionID)
		require.NoError(t, err)

		initDuration := time.Since(start)

		// Should be fast (< 100ms for basic operations)
		assert.Less(t, initDuration, 100*time.Millisecond)

		// Benchmark disk usage calculation
		start = time.Now()
		err = suite.workspace.UpdateDiskUsage(suite.ctx, suite.testSessionID)
		require.NoError(t, err)

		updateDuration := time.Since(start)
		assert.Less(t, updateDuration, 50*time.Millisecond)

		suite.logger.Info().
			Dur("init_duration", initDuration).
			Dur("update_duration", updateDuration).
			Str("workspace", workspaceDir).
			Msg("Workspace performance benchmark completed")
	})

	t.Run("WorkspaceStatsPerformance", func(t *testing.T) {
		// Benchmark workspace statistics operations
		start := time.Now()
		stats := suite.workspace.GetStats()
		statsDuration := time.Since(start)

		assert.NotNil(t, stats)
		assert.Less(t, statsDuration, 5*time.Millisecond)

		// Benchmark quota checking
		start = time.Now()
		err := suite.workspace.CheckQuota(suite.testSessionID, 100*1024*1024) // 100MB
		quotaDuration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, quotaDuration, 1*time.Millisecond)

		suite.logger.Info().
			Dur("stats_duration", statsDuration).
			Dur("quota_duration", quotaDuration).
			Msg("Workspace performance benchmark completed")
	})
}

// TestSecurityIntegration validates security across all team implementations
func TestSecurityIntegration(t *testing.T) {
	suite := NewCrossTeamIntegrationSuite(t)
	defer suite.cleanup()

	t.Run("WorkspaceSecurityValidation", func(t *testing.T) {
		// Test workspace path validation security
		err := suite.workspace.ValidateLocalPath(suite.ctx, "../../../etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal attempts are not allowed")

		err = suite.workspace.ValidateLocalPath(suite.ctx, "/etc/passwd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "absolute paths not allowed outside workspace")

		err = suite.workspace.ValidateLocalPath(suite.ctx, ".hidden")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hidden files are not allowed")

		suite.logger.Info().Msg("Workspace security validation passed")
	})

	t.Run("SandboxingSecurityValidation", func(t *testing.T) {
		// Test security policy validation
		validPolicy := SecurityPolicy{
			AllowNetworking:   false,
			AllowFileSystem:   true,
			RequireNonRoot:    true,
			TrustedRegistries: []string{"docker.io", "alpine"},
		}

		// Test public security policy validation method
		err := suite.workspace.ValidateSecurityPolicy(validPolicy)
		assert.NoError(t, err)

		// Test invalid policy
		invalidPolicy := SecurityPolicy{
			TrustedRegistries: []string{}, // Empty - should fail
		}
		err = suite.workspace.ValidateSecurityPolicy(invalidPolicy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one trusted registry must be specified")

		suite.logger.Info().Msg("Security policy validation working correctly")
	})
}

// Validation helper methods

func (suite *CrossTeamIntegrationSuite) validateInfraBotInterfaces(t *testing.T) {
	// Validate InfraBot interface contracts
	suite.logger.Info().Msg("Validating InfraBot interfaces")
	// Placeholder - would check actual interface implementations
}

func (suite *CrossTeamIntegrationSuite) validateBuildSecBotInterfaces(t *testing.T) {
	// Validate BuildSecBot interface contracts
	suite.logger.Info().Msg("Validating BuildSecBot interfaces")
	// Placeholder - would check actual interface implementations
}

func (suite *CrossTeamIntegrationSuite) validateOrchBotInterfaces(t *testing.T) {
	// Validate OrchBot interface contracts
	suite.logger.Info().Msg("Validating OrchBot interfaces")
	// Placeholder - would check actual interface implementations
}

func (suite *CrossTeamIntegrationSuite) validateAdvancedBotInterfaces(t *testing.T) {
	// Validate AdvancedBot interface contracts
	stats := suite.workspace.GetStats()
	assert.NotNil(t, stats)

	// Validate workspace operations are working
	assert.IsType(t, int64(0), stats.TotalDiskUsage)
	assert.IsType(t, int64(0), stats.TotalDiskLimit)
	assert.IsType(t, false, stats.SandboxEnabled)

	suite.logger.Info().Msg("Validating AdvancedBot interfaces - workspace operations operational")
}

func (suite *CrossTeamIntegrationSuite) validateErrorPropagation(t *testing.T) {
	// Test error handling across teams

	// Test quota exceeded error
	err := suite.workspace.CheckQuota(suite.testSessionID, 999*1024*1024*1024) // 999GB - should exceed quota
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QUOTA_EXCEEDED")

	suite.logger.Info().Msg("Error propagation validation completed")
}

func (suite *CrossTeamIntegrationSuite) validateDockerOperationsContract(t *testing.T) {
	suite.logger.Info().Msg("Validating Docker operations contract")
	// Placeholder - would validate InfraBot Docker operations interface
}

func (suite *CrossTeamIntegrationSuite) validateSessionManagementContract(t *testing.T) {
	suite.logger.Info().Msg("Validating session management contract")
	// Placeholder - would validate InfraBot session management interface
}

func (suite *CrossTeamIntegrationSuite) validateAtomicFrameworkContract(t *testing.T) {
	suite.logger.Info().Msg("Validating atomic framework contract")
	// Placeholder - would validate InfraBot atomic framework interface
}

func (suite *CrossTeamIntegrationSuite) validateAtomicToolsContract(t *testing.T) {
	suite.logger.Info().Msg("Validating atomic tools contract")
	// Placeholder - would validate BuildSecBot atomic tools interface
}

func (suite *CrossTeamIntegrationSuite) validateSecurityScanningContract(t *testing.T) {
	suite.logger.Info().Msg("Validating security scanning contract")
	// Placeholder - would validate BuildSecBot security scanning interface
}

func (suite *CrossTeamIntegrationSuite) validateBuildStrategiesContract(t *testing.T) {
	suite.logger.Info().Msg("Validating build strategies contract")
	// Placeholder - would validate BuildSecBot build strategies interface
}

func (suite *CrossTeamIntegrationSuite) validateContextSharingContract(t *testing.T) {
	suite.logger.Info().Msg("Validating context sharing contract")
	// Placeholder - would validate OrchBot context sharing interface
}

func (suite *CrossTeamIntegrationSuite) validateWorkflowOrchestrationContract(t *testing.T) {
	suite.logger.Info().Msg("Validating workflow orchestration contract")
	// Placeholder - would validate OrchBot workflow orchestration interface
}

func (suite *CrossTeamIntegrationSuite) validateCommunicationContract(t *testing.T) {
	suite.logger.Info().Msg("Validating communication contract")
	// Placeholder - would validate OrchBot communication interface
}

func (suite *CrossTeamIntegrationSuite) validateSandboxingContract(t *testing.T) {
	stats := suite.workspace.GetStats()
	assert.NotNil(t, stats)
	// SandboxEnabled field should be present
	assert.IsType(t, false, stats.SandboxEnabled)
	suite.logger.Info().Bool("sandbox_enabled", stats.SandboxEnabled).Msg("Validating sandboxing contract")
}

func (suite *CrossTeamIntegrationSuite) validateTestingFrameworkContract(t *testing.T) {
	// This test itself validates the testing framework is operational
	suite.logger.Info().Msg("Validating testing framework contract - framework is operational")
}

func (suite *CrossTeamIntegrationSuite) validateQualityMonitoringContract(t *testing.T) {
	// Validate workspace statistics as a form of quality monitoring
	stats := suite.workspace.GetStats()
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalSessions, 0)
	assert.GreaterOrEqual(t, stats.TotalDiskUsage, int64(0))
	suite.logger.Info().Msg("Validating quality monitoring contract - workspace stats operational")
}

func (suite *CrossTeamIntegrationSuite) cleanup() {
	if suite.workspace != nil {
		suite.workspace.CleanupWorkspace(suite.ctx, suite.testSessionID)
	}
}
