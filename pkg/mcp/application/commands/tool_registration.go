package commands

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Global service dependencies that will be injected later
var (
	globalServices     services.ServiceContainer
	globalServiceMutex sync.RWMutex
)

// SetGlobalServices sets the global service container for tool creation
func SetGlobalServices(container services.ServiceContainer) {
	globalServiceMutex.Lock()
	defer globalServiceMutex.Unlock()
	globalServices = container
}

// getServices retrieves the global service container
func getServices() (services.ServiceContainer, error) {
	globalServiceMutex.RLock()
	defer globalServiceMutex.RUnlock()

	if globalServices == nil {
		return nil, errors.NewError().
			Message("service container not initialized").
			WithLocation().
			Build()
	}
	return globalServices, nil
}

// LazyAnalyzeTool wraps ConsolidatedAnalyzeCommand for lazy initialization
type LazyAnalyzeTool struct {
	once     sync.Once
	instance *ConsolidatedAnalyzeCommand
	err      error
}

func (t *LazyAnalyzeTool) Name() string {
	return "analyze_repository"
}

func (t *LazyAnalyzeTool) Description() string {
	return "Analyze repository for containerization opportunities"
}

func (t *LazyAnalyzeTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		// Create analysis engine if available
		var analysisEngine *analysis.Engine
		if engineProvider, ok := services.(interface{ AnalysisEngine() *analysis.Engine }); ok {
			analysisEngine = engineProvider.AnalysisEngine()
		}

		t.instance = NewConsolidatedAnalyzeCommand(
			services.SessionStore(),
			services.SessionState(),
			services.FileAccessService(),
			logger,
			analysisEngine,
		)
	})

	if t.err != nil {
		return api.ToolOutput{}, t.err
	}

	return t.instance.Execute(ctx, input)
}

func (t *LazyAnalyzeTool) Schema() api.ToolSchema {
	// Return the schema without needing the actual instance
	return api.ToolSchema{
		Name:        "analyze_repository",
		Description: "Analyze repository for containerization opportunities",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repository_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the repository to analyze",
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"description": "Output format (json, yaml, text)",
					"enum":        []string{"json", "yaml", "text"},
					"default":     "json",
				},
				"deep_scan": map[string]interface{}{
					"type":        "boolean",
					"description": "Perform deep analysis including dependencies",
					"default":     false,
				},
			},
			"required": []string{"repository_path"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyBuildTool wraps ConsolidatedBuildCommand for lazy initialization
type LazyBuildTool struct {
	once     sync.Once
	instance *ConsolidatedBuildCommand
	err      error
}

func (t *LazyBuildTool) Name() string {
	return "build_image"
}

func (t *LazyBuildTool) Description() string {
	return "Build Docker images with AI-powered optimization"
}

func (t *LazyBuildTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		t.instance = NewConsolidatedBuildCommand(
			services.SessionStore(),
			services.SessionState(),
			nil, // buildExecutor - will be populated by the command if needed
			logger,
		)
	})

	if t.err != nil {
		return api.ToolOutput{}, t.err
	}

	return t.instance.Execute(ctx, input)
}

func (t *LazyBuildTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "build_image",
		Description: "Build Docker images with AI-powered optimization",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the Dockerfile",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name for the built image",
				},
				"build_context": map[string]interface{}{
					"type":        "string",
					"description": "Build context directory",
					"default":     ".",
				},
			},
			"required": []string{"dockerfile_path", "image_name"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyDeployTool wraps ConsolidatedDeployCommand for lazy initialization
type LazyDeployTool struct {
	once     sync.Once
	instance *ConsolidatedDeployCommand
	err      error
}

func (t *LazyDeployTool) Name() string {
	return "generate_manifests"
}

func (t *LazyDeployTool) Description() string {
	return "Deploy containers to Kubernetes with manifest generation"
}

func (t *LazyDeployTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		t.instance = NewConsolidatedDeployCommand(
			services.SessionStore(),
			services.SessionState(),
			nil, // kubeManager - will be populated by the command if needed
			logger,
		)
	})

	if t.err != nil {
		return api.ToolOutput{}, t.err
	}

	return t.instance.Execute(ctx, input)
}

func (t *LazyDeployTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "generate_manifests",
		Description: "Deploy containers to Kubernetes with manifest generation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Docker image to deploy",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
					"default":     "default",
				},
			},
			"required": []string{"image_name"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyScanTool wraps ConsolidatedScanCommand for lazy initialization
type LazyScanTool struct {
	once     sync.Once
	instance *ConsolidatedScanCommand
	err      error
}

func (t *LazyScanTool) Name() string {
	return "scan_image"
}

func (t *LazyScanTool) Description() string {
	return "Scan container images for security vulnerabilities"
}

func (t *LazyScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		t.instance = NewConsolidatedScanCommand(
			services.SessionStore(),
			services.SessionState(),
			nil, // scanner - will be populated by the command if needed
			logger,
		)
	})

	if t.err != nil {
		return api.ToolOutput{}, t.err
	}

	return t.instance.Execute(ctx, input)
}

func (t *LazyScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "scan_image",
		Description: "Scan container images for security vulnerabilities",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Docker image to scan",
				},
				"scanner": map[string]interface{}{
					"type":        "string",
					"description": "Scanner to use (trivy, grype)",
					"enum":        []string{"trivy", "grype"},
					"default":     "trivy",
				},
			},
			"required": []string{"image_name"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyPushTool wraps ConsolidatedPushTool for lazy initialization
type LazyPushTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyPushTool) Name() string {
	return "push_image"
}

func (t *LazyPushTool) Description() string {
	return "Push Docker images to container registries"
}

func (t *LazyPushTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Simple implementation for push_image
	imageName, ok := input.Data["image_name"].(string)
	if !ok || imageName == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "image_name is required",
		}, nil
	}

	imageTag, _ := input.Data["image_tag"].(string)
	if imageTag == "" {
		imageTag = "latest"
	}

	registry, _ := input.Data["registry"].(string)
	if registry == "" {
		registry = "docker.io"
	}

	// Get services for real Docker client access
	services, err := getServices()
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "service container not available",
		}, nil
	}

	// Build full image name
	var fullImageName string
	if registry == "docker.io" {
		fullImageName = fmt.Sprintf("%s:%s", imageName, imageTag)
	} else {
		fullImageName = fmt.Sprintf("%s/%s:%s", registry, imageName, imageTag)
	}

	startTime := time.Now()

	// If services are available, try to get BuildExecutor; otherwise fallback to simulation
	if buildExecutor := services.BuildExecutor(); buildExecutor != nil {
		// Try to push the image using real BuildExecutor
		pushOptions := docker.PushOptions{} // Empty options for now
		pushResult, pushErr := buildExecutor.QuickPush(ctx, fullImageName, pushOptions)
		if pushErr != nil {
			// Return error details but still provide helpful information
			return api.ToolOutput{
				Success: false,
				Error:   fmt.Sprintf("Docker push failed: %v", pushErr),
				Data: map[string]interface{}{
					"image_name":      imageName,
					"image_tag":       imageTag,
					"registry":        registry,
					"full_image_name": fullImageName,
					"error_details":   pushErr.Error(),
					"session_id":      input.SessionID,
					"attempted_at":    startTime.Format(time.RFC3339),
				},
			}, nil
		}

		// Push succeeded
		return api.ToolOutput{
			Success: true,
			Data: map[string]interface{}{
				"image_name":      imageName,
				"image_tag":       imageTag,
				"registry":        registry,
				"full_image_name": fullImageName,
				"push_result":     pushResult,
				"pushed_at":       time.Now().Format(time.RFC3339),
				"duration_ms":     time.Since(startTime).Milliseconds(),
				"session_id":      input.SessionID,
			},
		}, nil
	}

	// Fallback: Enhanced simulation with better validation
	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"image_name":      imageName,
			"image_tag":       imageTag,
			"registry":        registry,
			"full_image_name": fullImageName,
			"status":          "simulated",
			"message":         "Push operation simulated (Docker client not available)",
			"pushed_at":       time.Now().Format(time.RFC3339),
			"duration_ms":     50, // Simulate quick push
			"session_id":      input.SessionID,
		},
	}, nil
}

func (t *LazyPushTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "push_image",
		Description: "Push Docker images to container registries",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the Docker image to push",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag of the Docker image",
					"default":     "latest",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Container registry URL",
					"default":     "docker.io",
				},
			},
			"required": []string{"image_name"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyGenerateDockerfileTool wraps ConsolidatedDockerfileCommand for lazy initialization
type LazyGenerateDockerfileTool struct {
	once     sync.Once
	instance *ConsolidatedDockerfileCommand
	err      error
}

func (t *LazyGenerateDockerfileTool) Name() string {
	return "generate_dockerfile"
}

func (t *LazyGenerateDockerfileTool) Description() string {
	return "Generate optimized Dockerfile based on language and framework"
}

func (t *LazyGenerateDockerfileTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		t.instance = NewConsolidatedDockerfileCommand(
			services.SessionStore(),
			services.SessionState(),
			services.FileAccessService(),
			logger,
		)
	})

	if t.err != nil {
		return api.ToolOutput{}, t.err
	}

	return t.instance.Execute(ctx, input)
}

func (t *LazyGenerateDockerfileTool) Schema() api.ToolSchema {
	// Delegate to the actual instance schema to avoid duplication
	t.once.Do(func() {
		services, err := getServices()
		if err != nil {
			t.err = err
			return
		}

		var logger *slog.Logger
		if loggerProvider, ok := services.(interface{ Logger() *slog.Logger }); ok {
			logger = loggerProvider.Logger()
		} else {
			logger = slog.Default()
		}

		t.instance = NewConsolidatedDockerfileCommand(
			services.SessionStore(),
			services.SessionState(),
			services.FileAccessService(),
			logger,
		)
	})

	if t.instance != nil {
		return t.instance.Schema()
	}

	// Fallback schema if instance not available
	return api.ToolSchema{
		Name:        "generate_dockerfile",
		Description: "Generate optimized Dockerfile based on language and framework",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Programming language (go, javascript, typescript, python, java, csharp)",
					"enum":        []string{"go", "javascript", "typescript", "python", "java", "csharp"},
				},
				"framework": map[string]interface{}{
					"type":        "string",
					"description": "Framework (express, next, react, flask, django, fastapi, spring)",
				},
				"port": map[string]interface{}{
					"type":        "integer",
					"description": "Port to expose",
					"minimum":     1,
					"maximum":     65535,
				},
				"multi_stage": map[string]interface{}{
					"type":        "boolean",
					"description": "Use multi-stage build",
					"default":     true,
				},
			},
			"required": []string{"language"},
		},
		Category: "containerization",
		Version:  "1.0.0",
	}
}

// LazyPingTool wraps ConsolidatedPingTool for lazy initialization
type LazyPingTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyPingTool) Name() string {
	return "ping"
}

func (t *LazyPingTool) Description() string {
	return "Test server connectivity and response time"
}

func (t *LazyPingTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Get services to check if server is properly initialized
	services, err := getServices()
	serverHealthy := err == nil && services != nil

	// Extract message parameter
	message, _ := input.Data["message"].(string)
	if message == "" {
		message = "pong"
	}

	// Calculate actual response time
	responseTime := time.Since(startTime).Milliseconds()

	status := "ok"
	if !serverHealthy {
		status = "degraded"
	}

	data := map[string]interface{}{
		"status":           status,
		"message":          message,
		"timestamp":        time.Now().Format(time.RFC3339),
		"session_id":       input.SessionID,
		"response_time_ms": responseTime,
		"server_healthy":   serverHealthy,
	}

	// Add service information if available
	if serverHealthy {
		data["services_available"] = true

		// Check if specific services are available
		data["session_store_available"] = services.SessionStore() != nil
		data["tool_registry_available"] = services.ToolRegistry() != nil
	} else {
		data["services_available"] = false
		data["error"] = "service container not initialized"
	}

	return api.ToolOutput{
		Success: true,
		Data:    data,
	}, nil
}

func (t *LazyPingTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "ping",
		Description: "Test server connectivity and response time",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Optional message to echo",
				},
			},
		},
		Category: "diagnostic",
		Version:  "1.0.0",
	}
}

// LazyServerStatusTool wraps ConsolidatedServerStatusTool for lazy initialization
type LazyServerStatusTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyServerStatusTool) Name() string {
	return "server_status"
}

func (t *LazyServerStatusTool) Description() string {
	return "Get server status and health information"
}

func (t *LazyServerStatusTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Get services for real metrics
	services, err := getServices()
	serverHealthy := err == nil && services != nil

	detailed, _ := input.Data["detailed"].(bool)

	// Get real memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Base server status
	status := "healthy"
	if !serverHealthy {
		status = "degraded"
	}

	data := map[string]interface{}{
		"status":     status,
		"version":    "1.0.0",
		"timestamp":  time.Now().Format(time.RFC3339),
		"session_id": input.SessionID,
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
	}

	// Add memory information
	data["memory"] = map[string]interface{}{
		"alloc_mb":     float64(memStats.Alloc) / 1024 / 1024,
		"sys_mb":       float64(memStats.Sys) / 1024 / 1024,
		"gc_cycles":    memStats.NumGC,
		"heap_objects": memStats.HeapObjects,
	}

	// Try to get real session count
	activeSessionCount := 0
	totalToolCount := 9 // Known from our registration

	if serverHealthy {
		data["services_healthy"] = true

		// Get session count if SessionStore is available
		sessionStore := services.SessionStore()
		if sessionStore != nil {
			if sessions, sessionErr := sessionStore.List(ctx); sessionErr == nil {
				activeSessionCount = len(sessions)
				data["sessions_from_store"] = true
			}
		}

		// Get tool count if ToolRegistry is available
		toolRegistry := services.ToolRegistry()
		if toolRegistry != nil {
			if tools := toolRegistry.List(); len(tools) > 0 {
				totalToolCount = len(tools)
				data["tools_from_registry"] = true
			}
		}
	} else {
		data["services_healthy"] = false
		data["error"] = "service container not available"
	}

	data["active_sessions"] = activeSessionCount
	data["total_tools"] = totalToolCount

	// Add detailed metrics if requested
	if detailed {
		data["detailed_memory"] = map[string]interface{}{
			"heap_alloc_mb":     float64(memStats.HeapAlloc) / 1024 / 1024,
			"heap_sys_mb":       float64(memStats.HeapSys) / 1024 / 1024,
			"heap_idle_mb":      float64(memStats.HeapIdle) / 1024 / 1024,
			"heap_inuse_mb":     float64(memStats.HeapInuse) / 1024 / 1024,
			"stack_inuse_mb":    float64(memStats.StackInuse) / 1024 / 1024,
			"next_gc_mb":        float64(memStats.NextGC) / 1024 / 1024,
			"last_gc_time":      time.Unix(0, int64(memStats.LastGC)).Format(time.RFC3339),
			"gc_pause_total_ms": float64(memStats.PauseTotalNs) / 1000000,
		}

		// Add OS-level stats
		data["runtime"] = map[string]interface{}{
			"os":        runtime.GOOS,
			"arch":      runtime.GOARCH,
			"num_cpu":   runtime.NumCPU,
			"cgo_calls": runtime.NumCgoCall(),
		}
	}

	return api.ToolOutput{
		Success: true,
		Data:    data,
	}, nil
}

func (t *LazyServerStatusTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "server_status",
		Description: "Get server status and health information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"detailed": map[string]interface{}{
					"type":        "boolean",
					"description": "Include detailed metrics",
					"default":     false,
				},
			},
		},
		Category: "diagnostic",
		Version:  "1.0.0",
	}
}

// LazyListSessionsTool wraps ConsolidatedListSessionsTool for lazy initialization
type LazyListSessionsTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyListSessionsTool) Name() string {
	return "list_sessions"
}

func (t *LazyListSessionsTool) Description() string {
	return "List all active sessions"
}

func (t *LazyListSessionsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Get services for real SessionStore access
	services, err := getServices()
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "service container not available",
		}, nil
	}

	// Extract limit parameter
	limit, _ := input.Data["limit"].(float64)
	if limit == 0 {
		limit = 10
	}

	// Try to get SessionStore from services
	if sessionStore := services.SessionStore(); sessionStore != nil {
		// Query real sessions from SessionStore
		allSessions, err := sessionStore.List(ctx)
		if err != nil {
			return api.ToolOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to list sessions: %v", err),
			}, nil
		}

		// Convert sessions to output format
		sessions := make([]map[string]interface{}, 0, len(allSessions))
		activeCount := 0
		idleCount := 0

		for i, session := range allSessions {
			if i >= int(limit) {
				break
			}

			// Determine session status based on last activity
			status := "active"
			if time.Since(session.UpdatedAt) > 30*time.Minute {
				status = "idle"
				idleCount++
			} else {
				activeCount++
			}

			sessionData := map[string]interface{}{
				"id":          session.ID,
				"status":      status,
				"created_at":  session.CreatedAt.Format(time.RFC3339),
				"last_active": session.UpdatedAt.Format(time.RFC3339),
			}

			// Add tool count if available in metadata
			if toolCount, ok := session.Metadata["tool_count"]; ok {
				sessionData["tool_count"] = toolCount
			} else {
				sessionData["tool_count"] = 0
			}

			sessions = append(sessions, sessionData)
		}

		return api.ToolOutput{
			Success: true,
			Data: map[string]interface{}{
				"sessions":     sessions,
				"total_count":  len(allSessions),
				"active_count": activeCount,
				"idle_count":   idleCount,
				"timestamp":    time.Now().Format(time.RFC3339),
				"session_id":   input.SessionID,
				"source":       "real_session_store",
			},
		}, nil
	}

	// Fallback: Enhanced mock data if SessionStore not available
	sessions := []map[string]interface{}{
		{
			"id":          input.SessionID,
			"status":      "active",
			"created_at":  time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			"last_active": time.Now().Format(time.RFC3339),
			"tool_count":  1,
		},
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"sessions":     sessions,
			"total_count":  len(sessions),
			"active_count": 1,
			"idle_count":   0,
			"timestamp":    time.Now().Format(time.RFC3339),
			"session_id":   input.SessionID,
			"source":       "simulated",
			"message":      "Session store not available, showing current session only",
		},
	}, nil
}

func (t *LazyListSessionsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "list_sessions",
		Description: "List all active sessions",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of sessions to return",
					"default":     10,
				},
			},
		},
		Category: "session",
		Version:  "1.0.0",
	}
}

// LazyReadFileTool implements file reading functionality
type LazyReadFileTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyReadFileTool) Name() string {
	return "read_file"
}

func (t *LazyReadFileTool) Description() string {
	return "Read file contents within the session workspace"
}

func (t *LazyReadFileTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract parameters
	filePath, ok := input.Data["path"].(string)
	if !ok || filePath == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "path parameter is required",
		}, nil
	}

	// Get services
	services, err := getServices()
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "service container not available",
		}, nil
	}

	// Get file access service
	fileAccess := services.FileAccessService()
	if fileAccess == nil {
		return api.ToolOutput{
			Success: false,
			Error:   "file access service not available",
		}, nil
	}

	// Read file
	content, err := fileAccess.ReadFile(ctx, input.SessionID, filePath)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"path":       filePath,
			"content":    content,
			"size":       len(content),
			"session_id": input.SessionID,
			"timestamp":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *LazyReadFileTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "read_file",
		Description: "Read file contents within the session workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file within the workspace",
				},
			},
			"required": []string{"path"},
		},
		Category: "file_access",
		Version:  "1.0.0",
	}
}

// LazyListDirectoryTool implements directory listing functionality
type LazyListDirectoryTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *LazyListDirectoryTool) Description() string {
	return "List files and directories within the session workspace"
}

func (t *LazyListDirectoryTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract parameters
	dirPath, _ := input.Data["path"].(string)
	if dirPath == "" {
		dirPath = "." // Default to workspace root
	}

	// Get services
	services, err := getServices()
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "service container not available",
		}, nil
	}

	// Get file access service
	fileAccess := services.FileAccessService()
	if fileAccess == nil {
		return api.ToolOutput{
			Success: false,
			Error:   "file access service not available",
		}, nil
	}

	// List directory
	files, err := fileAccess.ListDirectory(ctx, input.SessionID, dirPath)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert to output format
	fileList := make([]map[string]interface{}, len(files))
	for i, f := range files {
		fileList[i] = map[string]interface{}{
			"name":     f.Name,
			"path":     f.Path,
			"size":     f.Size,
			"is_dir":   f.IsDir,
			"modified": f.ModTime.Format(time.RFC3339),
			"mode":     f.Mode,
		}
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"path":       dirPath,
			"files":      fileList,
			"count":      len(files),
			"session_id": input.SessionID,
			"timestamp":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *LazyListDirectoryTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "list_directory",
		Description: "List files and directories within the session workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the directory within the workspace (defaults to root)",
					"default":     ".",
				},
			},
		},
		Category: "file_access",
		Version:  "1.0.0",
	}
}

// LazyFileExistsTool implements file existence checking functionality
type LazyFileExistsTool struct {
	once     sync.Once
	instance api.Tool
	err      error
}

func (t *LazyFileExistsTool) Name() string {
	return "file_exists"
}

func (t *LazyFileExistsTool) Description() string {
	return "Check if a file or directory exists within the session workspace"
}

func (t *LazyFileExistsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Extract parameters
	filePath, ok := input.Data["path"].(string)
	if !ok || filePath == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "path parameter is required",
		}, nil
	}

	// Get services
	services, err := getServices()
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   "service container not available",
		}, nil
	}

	// Get file access service
	fileAccess := services.FileAccessService()
	if fileAccess == nil {
		return api.ToolOutput{
			Success: false,
			Error:   "file access service not available",
		}, nil
	}

	// Check file existence
	exists, err := fileAccess.FileExists(ctx, input.SessionID, filePath)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"path":       filePath,
			"exists":     exists,
			"session_id": input.SessionID,
			"timestamp":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *LazyFileExistsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "file_exists",
		Description: "Check if a file or directory exists within the session workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path to the file or directory within the workspace",
				},
			},
			"required": []string{"path"},
		},
		Category: "file_access",
		Version:  "1.0.0",
	}
}

// Auto-registration using init()
func init() {
	// Register core containerization tools
	registry.RegisterTool("analyze_repository", func() (api.Tool, error) {
		return &LazyAnalyzeTool{}, nil
	})

	registry.RegisterTool("generate_dockerfile", func() (api.Tool, error) {
		return &LazyGenerateDockerfileTool{}, nil
	})

	registry.RegisterTool("build_image", func() (api.Tool, error) {
		return &LazyBuildTool{}, nil
	})

	registry.RegisterTool("push_image", func() (api.Tool, error) {
		return &LazyPushTool{}, nil
	})

	registry.RegisterTool("generate_manifests", func() (api.Tool, error) {
		return &LazyDeployTool{}, nil
	})

	registry.RegisterTool("scan_image", func() (api.Tool, error) {
		return &LazyScanTool{}, nil
	})

	// Register session management tools
	registry.RegisterTool("list_sessions", func() (api.Tool, error) {
		return &LazyListSessionsTool{}, nil
	})

	// Register diagnostic tools
	registry.RegisterTool("ping", func() (api.Tool, error) {
		return &LazyPingTool{}, nil
	})

	registry.RegisterTool("server_status", func() (api.Tool, error) {
		return &LazyServerStatusTool{}, nil
	})

	// Register file access tools
	registry.RegisterTool("read_file", func() (api.Tool, error) {
		return &LazyReadFileTool{}, nil
	})

	registry.RegisterTool("list_directory", func() (api.Tool, error) {
		return &LazyListDirectoryTool{}, nil
	})

	registry.RegisterTool("file_exists", func() (api.Tool, error) {
		return &LazyFileExistsTool{}, nil
	})
}
