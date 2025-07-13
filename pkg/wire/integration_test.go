package wire

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// TestFullWireIntegration tests the complete Wire-based dependency injection flow
func TestFullWireIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Phase 1: Initialize Dependencies with Wire
	deps, err := InitializeDependencies(logger)
	if err != nil {
		t.Fatalf("Phase 1 failed - Failed to initialize dependencies: %v", err)
	}
	if deps == nil {
		t.Fatal("Phase 1 failed - Dependencies is nil")
	}

	// Phase 2: Validate Core Services (Docker, Kubernetes, Security)
	validatePhase2CoreServices(t, deps)

	// Phase 3: Validate Workflow Steps and Orchestration
	validatePhase3WorkflowComponents(t, deps)

	// Phase 4: Validate CLI and Transport Components + Application Server
	validatePhase4ApplicationComponents(t, deps, logger)

	t.Logf("✅ All phases validated successfully - Wire migration complete!")
}

// TestCustomConfigIntegration tests Wire with custom configuration
func TestCustomConfigIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create custom config
	config := workflow.DefaultServerConfig()
	config.WorkspaceDir = "/tmp/test-workspace"
	config.MaxSessions = 5
	config.TransportType = "http"

	// Initialize with custom config
	deps, err := InitializeDependenciesWithConfig(logger, config)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies with custom config: %v", err)
	}

	// Validate custom config is applied
	if deps.Config.WorkspaceDir != "/tmp/test-workspace" {
		t.Errorf("Expected WorkspaceDir '/tmp/test-workspace', got '%s'", deps.Config.WorkspaceDir)
	}
	if deps.Config.MaxSessions != 5 {
		t.Errorf("Expected MaxSessions 5, got %d", deps.Config.MaxSessions)
	}
	if deps.Config.TransportType != "http" {
		t.Errorf("Expected TransportType 'http', got '%s'", deps.Config.TransportType)
	}

	t.Logf("✅ Custom configuration integration validated")
}

// TestApplicationServerLifecycle tests the complete application server lifecycle
func TestApplicationServerLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Initialize dependencies
	deps, err := InitializeDependencies(logger)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies: %v", err)
	}

	// Create ApplicationServer
	appServer, err := InitializeApplicationServer(logger, deps)
	if err != nil {
		t.Fatalf("Failed to initialize ApplicationServer: %v", err)
	}

	// Test initialization
	if err := appServer.Initialize(); err != nil {
		t.Fatalf("Failed to initialize ApplicationServer: %v", err)
	}

	// Test MCP server wrapper
	mcpServer := appServer.GetMCPServer()
	if mcpServer == nil {
		t.Fatal("GetMCPServer() returned nil")
	}

	// Test lifecycle methods
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test Start (should fail quickly due to context timeout, but should not panic)
	err = mcpServer.Start(ctx)
	if err != nil && err != context.DeadlineExceeded {
		t.Logf("Start returned error (expected due to timeout): %v", err)
	}

	// Test Stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stopCancel()
	if err := mcpServer.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	t.Logf("✅ ApplicationServer lifecycle validated")
}

// validatePhase2CoreServices validates Phase 2 components
func validatePhase2CoreServices(t *testing.T, deps *Dependencies) {
	coreServices := []struct {
		name      string
		component interface{}
	}{
		{"CommandRunner", deps.CommandRunner},
		{"DockerClient", deps.DockerClient},
		{"DockerService", deps.DockerService},
		{"KubeRunner", deps.KubeRunner},
		{"KubernetesService", deps.KubernetesService},
		{"SecurityService", deps.SecurityService},
		{"TrivyScanner", deps.TrivyScanner},
		{"GrypeScanner", deps.GrypeScanner},
		{"UnifiedSecurityScanner", deps.UnifiedSecurityScanner},
	}

	for _, service := range coreServices {
		if service.component == nil {
			t.Errorf("Phase 2 validation failed: %s is nil", service.name)
		}
	}

	t.Logf("✅ Phase 2: Core Services validated (%d components)", len(coreServices))
}

// validatePhase3WorkflowComponents validates Phase 3 components
func validatePhase3WorkflowComponents(t *testing.T, deps *Dependencies) {
	workflowSteps := []struct {
		name      string
		component interface{}
	}{
		{"AnalysisStep", deps.AnalysisStep},
		{"DockerfileStep", deps.DockerfileStep},
		{"BuildStep", deps.BuildStep},
		{"ManifestStep", deps.ManifestStep},
		{"DeploymentStep", deps.DeploymentStep},
		{"VerificationStep", deps.VerificationStep},
	}

	orchestrationComponents := []struct {
		name      string
		component interface{}
	}{
		{"ContainerizationOrchestrator", deps.ContainerizationOrchestrator},
		{"WorkflowCoordinator", deps.WorkflowCoordinator},
		{"StepPipeline", deps.StepPipeline},
	}

	for _, step := range workflowSteps {
		if step.component == nil {
			t.Errorf("Phase 3 validation failed: %s is nil", step.name)
		}
	}

	for _, orchestrator := range orchestrationComponents {
		if orchestrator.component == nil {
			t.Errorf("Phase 3 validation failed: %s is nil", orchestrator.name)
		}
	}

	t.Logf("✅ Phase 3: Workflow Components validated (%d steps, %d orchestrators)",
		len(workflowSteps), len(orchestrationComponents))
}

// validatePhase4ApplicationComponents validates Phase 4 components
func validatePhase4ApplicationComponents(t *testing.T, deps *Dependencies, logger *slog.Logger) {
	cliComponents := []struct {
		name      string
		component interface{}
	}{
		{"CLIConfig", deps.CLIConfig},
		{"CLIApplication", deps.CLIApplication},
		{"FlagParser", deps.FlagParser},
	}

	transportComponents := []struct {
		name      string
		component interface{}
	}{
		{"MCPServerConfig", deps.MCPServerConfig},
		{"MCPTransport", deps.MCPTransport},
		{"HTTPTransport", deps.HTTPTransport},
		{"ServerRegistrar", deps.ServerRegistrar},
		{"MCPServer", deps.MCPServer},
	}

	for _, cli := range cliComponents {
		if cli.component == nil {
			t.Errorf("Phase 4 validation failed: %s is nil", cli.name)
		}
	}

	for _, transport := range transportComponents {
		if transport.component == nil {
			t.Errorf("Phase 4 validation failed: %s is nil", transport.name)
		}
	}

	// Test ApplicationServer creation
	appServer, err := InitializeApplicationServer(logger, deps)
	if err != nil {
		t.Errorf("Phase 4 validation failed: ApplicationServer initialization error: %v", err)
		return
	}
	if appServer == nil {
		t.Error("Phase 4 validation failed: ApplicationServer is nil")
		return
	}

	t.Logf("✅ Phase 4: Application Components validated (%d CLI, %d transport, 1 server)",
		len(cliComponents), len(transportComponents))
}
