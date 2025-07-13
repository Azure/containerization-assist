package wire

import (
	"log/slog"
	"os"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

func TestInitializeDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	deps, err := InitializeDependencies(logger)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies: %v", err)
	}

	if deps == nil {
		t.Fatal("Dependencies is nil")
	}

	// Test core components
	if deps.Logger == nil {
		t.Error("Logger is nil")
	}
	if deps.SessionManager == nil {
		t.Error("SessionManager is nil")
	}
	if deps.ResourceStore == nil {
		t.Error("ResourceStore is nil")
	}

	// Test Phase 2 components
	if deps.DockerService == nil {
		t.Error("DockerService is nil")
	}
	if deps.KubernetesService == nil {
		t.Error("KubernetesService is nil")
	}
	if deps.SecurityService == nil {
		t.Error("SecurityService is nil")
	}
	if deps.TrivyScanner == nil {
		t.Error("TrivyScanner is nil")
	}
	if deps.GrypeScanner == nil {
		t.Error("GrypeScanner is nil")
	}
	if deps.UnifiedSecurityScanner == nil {
		t.Error("UnifiedSecurityScanner is nil")
	}

	t.Logf("Successfully initialized %d infrastructure components", 6)
}

func TestInitializeDependenciesWithConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := workflow.DefaultServerConfig()

	deps, err := InitializeDependenciesWithConfig(logger, config)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies with config: %v", err)
	}

	if deps == nil {
		t.Fatal("Dependencies is nil")
	}

	// Test config is properly injected
	if deps.Config.WorkspaceDir == "" {
		t.Error("Config.WorkspaceDir is empty")
	}

	// Test Phase 2 components with custom config
	if deps.DockerClient == nil {
		t.Error("DockerClient is nil")
	}
	if deps.CommandRunner == nil {
		t.Error("CommandRunner is nil")
	}

	t.Logf("Successfully initialized dependencies with custom config")
}

func TestPhase3WorkflowSteps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	deps, err := InitializeDependencies(logger)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies: %v", err)
	}

	// Test Phase 3 Workflow Steps
	if deps.AnalysisStep == nil {
		t.Error("AnalysisStep is nil")
	}
	if deps.DockerfileStep == nil {
		t.Error("DockerfileStep is nil")
	}
	if deps.BuildStep == nil {
		t.Error("BuildStep is nil")
	}
	if deps.ManifestStep == nil {
		t.Error("ManifestStep is nil")
	}
	if deps.DeploymentStep == nil {
		t.Error("DeploymentStep is nil")
	}
	if deps.VerificationStep == nil {
		t.Error("VerificationStep is nil")
	}

	// Test Phase 3 Orchestration
	if deps.ContainerizationOrchestrator == nil {
		t.Error("ContainerizationOrchestrator is nil")
	}
	if deps.WorkflowCoordinator == nil {
		t.Error("WorkflowCoordinator is nil")
	}
	if deps.StepPipeline == nil {
		t.Error("StepPipeline is nil")
	}

	t.Logf("Successfully initialized %d workflow steps and %d orchestration components",
		6, 3)
}

func TestPhase4ApplicationServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Initialize dependencies
	deps, err := InitializeDependencies(logger)
	if err != nil {
		t.Fatalf("Failed to initialize dependencies: %v", err)
	}

	if deps == nil {
		t.Fatal("Dependencies is nil")
	}

	// Validate Phase 4 CLI components
	if deps.CLIConfig == nil {
		t.Error("CLIConfig is nil")
	}
	if deps.CLIApplication == nil {
		t.Error("CLIApplication is nil")
	}
	if deps.FlagParser == nil {
		t.Error("FlagParser is nil")
	}

	// Validate Phase 4 transport components
	if deps.MCPServerConfig == nil {
		t.Error("MCPServerConfig is nil")
	}
	if deps.MCPTransport == nil {
		t.Error("MCPTransport is nil")
	}
	if deps.HTTPTransport == nil {
		t.Error("HTTPTransport is nil")
	}
	if deps.ServerRegistrar == nil {
		t.Error("ServerRegistrar is nil")
	}

	// Validate Phase 4 application server components
	if deps.MCPServer == nil {
		t.Error("MCPServer is nil")
	}

	// Test ApplicationServer initialization
	appServer, err := InitializeApplicationServer(logger, deps)
	if err != nil {
		t.Fatalf("Failed to initialize ApplicationServer: %v", err)
	}
	if appServer == nil {
		t.Fatal("ApplicationServer is nil")
	}

	// Validate ApplicationServer components
	if appServer.mcpServer == nil {
		t.Error("ApplicationServer.mcpServer is nil")
	}
	if appServer.serverRegistrar == nil {
		t.Error("ApplicationServer.serverRegistrar is nil")
	}
	if appServer.transportManager == nil {
		t.Error("ApplicationServer.transportManager is nil")
	}
	if appServer.appDeps == nil {
		t.Error("ApplicationServer.appDeps is nil")
	}

	// Test ApplicationServer methods
	if err := appServer.Initialize(); err != nil {
		t.Errorf("ApplicationServer.Initialize() failed: %v", err)
	}

	// Test GetMCPServer method
	mcpServerWrapper := appServer.GetMCPServer()
	if mcpServerWrapper == nil {
		t.Error("GetMCPServer() returned nil")
	}

	t.Logf("Successfully initialized ApplicationServer with 3 CLI, 4 transport, and 1 server component")
}
