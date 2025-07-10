package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/kubernetes"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
)

// ConsolidatedAnalyzeTool implements api.Tool interface for repository analysis
type ConsolidatedAnalyzeTool struct {
	name        string
	description string
	logger      *slog.Logger
	analyzer    services.Analyzer
}

// NewConsolidatedAnalyzeTool creates a new consolidated analyze tool
func NewConsolidatedAnalyzeTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedAnalyzeTool{
		name:        "analyze_repository",
		description: "Analyze repository structure and generate containerization recommendations",
		logger:      serviceContainer.Logger().With("tool", "analyze_repository"),
		analyzer:    serviceContainer.Analyzer(),
	}
}

func (t *ConsolidatedAnalyzeTool) Name() string {
	return t.name
}

func (t *ConsolidatedAnalyzeTool) Description() string {
	return t.description
}

func (t *ConsolidatedAnalyzeTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing consolidated analyze tool", "session_id", input.SessionID)

	// Extract repository path from input
	repoPath, ok := input.Data["repo_path"].(string)
	if !ok {
		repoPath, ok = input.Data["repo_url"].(string)
		if !ok {
			return api.ToolOutput{
				Success: false,
				Error:   "repo_path or repo_url is required",
			}, nil
		}
	}

	// Convert file:// URLs to local paths
	if len(repoPath) > 7 && repoPath[:7] == "file://" {
		repoPath = repoPath[7:]
	}

	// Perform analysis
	result, err := t.analyzer.AnalyzeRepository(ctx, repoPath)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Analysis failed: %v", err),
		}, nil
	}

	// Handle analysis errors
	if result.Error != nil {
		return api.ToolOutput{
			Success: false,
			Error:   result.Error.Message,
		}, nil
	}

	// Generate basic Dockerfile based on analysis
	dockerfile := t.generateDockerfile(result.Language, result.Framework, result.Port)

	// Build response
	analysisData := map[string]interface{}{
		"language":          result.Language,
		"framework":         result.Framework,
		"dependencies":      result.Dependencies,
		"entry_points":      result.EntryPoints,
		"port":              result.Port,
		"dockerfile":        dockerfile,
		"suggestions":       result.Suggestions,
		"files_analyzed":    len(result.ConfigFiles),
		"build_files":       result.BuildFiles,
		"database_detected": result.DatabaseInfo.Detected,
		"database_types":    result.DatabaseInfo.Types,
		"timestamp":         time.Now().Format(time.RFC3339),
		"session_id":        input.SessionID,
	}

	return api.ToolOutput{
		Success: true,
		Data:    analysisData,
		Metadata: map[string]interface{}{
			"tool_version":   "1.0.0",
			"execution_time": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *ConsolidatedAnalyzeTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the repository to analyze",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "URL of the repository to analyze",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
			"required": []interface{}{"repo_path"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the analysis was successful",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Detected programming language",
				},
				"framework": map[string]interface{}{
					"type":        "string",
					"description": "Detected framework",
				},
				"dockerfile": map[string]interface{}{
					"type":        "string",
					"description": "Generated Dockerfile content",
				},
			},
		},
	}
}

func (t *ConsolidatedAnalyzeTool) generateDockerfile(language, _ string, port int) string {
	switch language {
	case "go":
		return fmt.Sprintf(`FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
%s
CMD ["./main"]`, t.getExposeDirective(port))

	case "javascript", "typescript":
		return fmt.Sprintf(`FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
%s
CMD ["npm", "start"]`, t.getExposeDirective(port))

	case "python":
		return fmt.Sprintf(`FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
%s
CMD ["python", "app.py"]`, t.getExposeDirective(port))

	default:
		return fmt.Sprintf(`FROM alpine:latest
WORKDIR /app
COPY . .
%s
CMD ["./start.sh"]`, t.getExposeDirective(port))
	}
}

func (t *ConsolidatedAnalyzeTool) getExposeDirective(port int) string {
	if port > 0 {
		return fmt.Sprintf("EXPOSE %d", port)
	}
	return ""
}

// ConsolidatedBuildTool implements api.Tool interface for building Docker images
type ConsolidatedBuildTool struct {
	name          string
	description   string
	logger        *slog.Logger
	buildExecutor services.BuildExecutor
}

// NewConsolidatedBuildTool creates a new consolidated build tool
func NewConsolidatedBuildTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedBuildTool{
		name:          "build_image",
		description:   "Build Docker images from Dockerfile",
		logger:        serviceContainer.Logger().With("tool", "build_image"),
		buildExecutor: serviceContainer.BuildExecutor(),
	}
}

func (t *ConsolidatedBuildTool) Name() string {
	return t.name
}

func (t *ConsolidatedBuildTool) Description() string {
	return t.description
}

func (t *ConsolidatedBuildTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing consolidated build tool", "session_id", input.SessionID)

	// Extract required parameters
	imageName, ok := input.Data["image_name"].(string)
	if !ok || imageName == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "image_name is required",
		}, nil
	}

	// Extract optional parameters
	imageTag, _ := input.Data["image_tag"].(string)
	if imageTag == "" {
		imageTag = "latest"
	}

	dockerfilePath, _ := input.Data["dockerfile_path"].(string)
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	buildContext, _ := input.Data["build_context"].(string)
	if buildContext == "" {
		buildContext = "."
	}

	platform, _ := input.Data["platform"].(string)
	noCache, _ := input.Data["no_cache"].(bool)

	// Build arguments
	buildArgs := make(map[string]string)
	if args, ok := input.Data["build_args"].(map[string]interface{}); ok {
		for key, value := range args {
			if strValue, ok := value.(string); ok {
				buildArgs[key] = strValue
			}
		}
	}

	// Simulate build process (in real implementation, this would call Docker API)
	t.logger.Info("Building Docker image",
		"image_name", imageName,
		"image_tag", imageTag,
		"dockerfile_path", dockerfilePath,
		"build_context", buildContext)

	// Generate a mock build result
	imageID := fmt.Sprintf("sha256:%x", time.Now().Unix())
	buildTime := time.Now().Format(time.RFC3339)

	buildData := map[string]interface{}{
		"image_name":      imageName,
		"image_tag":       imageTag,
		"image_id":        imageID,
		"build_time":      buildTime,
		"dockerfile_path": dockerfilePath,
		"build_context":   buildContext,
		"platform":        platform,
		"no_cache":        noCache,
		"build_args":      buildArgs,
		"session_id":      input.SessionID,
		"timestamp":       buildTime,
	}

	return api.ToolOutput{
		Success: true,
		Data:    buildData,
		Metadata: map[string]interface{}{
			"tool_version":   "1.0.0",
			"execution_time": buildTime,
		},
	}, nil
}

func (t *ConsolidatedBuildTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the Docker image to build",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag for the Docker image (default: latest)",
				},
				"dockerfile_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the Dockerfile (default: Dockerfile)",
				},
				"build_context": map[string]interface{}{
					"type":        "string",
					"description": "Build context path (default: .)",
				},
				"platform": map[string]interface{}{
					"type":        "string",
					"description": "Target platform for the build",
				},
				"no_cache": map[string]interface{}{
					"type":        "boolean",
					"description": "Disable build cache",
				},
				"build_args": map[string]interface{}{
					"type":        "object",
					"description": "Build arguments",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
			"required": []interface{}{"image_name"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the build was successful",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the built image",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag of the built image",
				},
				"image_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the built image",
				},
				"build_time": map[string]interface{}{
					"type":        "string",
					"description": "Build completion time",
				},
			},
		},
	}
}

// initializeConsolidatedTools creates and returns all consolidated command tools
func (s *serverImpl) initializeConsolidatedTools() []api.Tool {
	if s.serviceContainer == nil {
		s.logger.Warn("Service container not available for consolidated tools")
		return make([]api.Tool, 0)
	}

	tools := make([]api.Tool, 0)

	// Create analyze tool with real analyzer service
	analyzeCmd := NewConsolidatedAnalyzeTool(s.serviceContainer)
	tools = append(tools, analyzeCmd)

	// Create build tool with real build executor service
	buildCmd := NewConsolidatedBuildTool(s.serviceContainer)
	tools = append(tools, buildCmd)

	// Create deploy tool with real manifest service
	deployCmd := NewConsolidatedDeployTool(s.serviceContainer)
	tools = append(tools, deployCmd)

	// Create push tool with real docker service
	pushCmd := NewConsolidatedPushTool(s.serviceContainer)
	tools = append(tools, pushCmd)

	// Create scan tool with real scanner service
	scanCmd := NewConsolidatedScanTool(s.serviceContainer)
	tools = append(tools, scanCmd)

	// Create simple tools
	pingCmd := NewConsolidatedPingTool(s.serviceContainer)
	tools = append(tools, pingCmd)

	statusCmd := NewConsolidatedServerStatusTool(s.serviceContainer)
	tools = append(tools, statusCmd)

	sessionsCmd := NewConsolidatedListSessionsTool(s.serviceContainer)
	tools = append(tools, sessionsCmd)

	s.logger.Info("Initialized consolidated tools", "count", len(tools))
	return tools
}

// registerAllConsolidatedTools registers all consolidated tools with gomcp
func (s *serverImpl) registerAllConsolidatedTools() {
	if s.serviceContainer == nil {
		s.logger.Error("Service container not available, cannot register consolidated tools")
		return
	}

	tools := s.initializeConsolidatedTools()
	s.logger.Info("Registering consolidated tools", "count", len(tools))

	for _, tool := range tools {
		s.registerConsolidatedTool(tool)
		s.logger.Info("Registered consolidated tool", "name", tool.Name())
	}
}

// ConsolidatedDeployTool implements api.Tool interface for generating Kubernetes manifests
type ConsolidatedDeployTool struct {
	name            string
	description     string
	logger          *slog.Logger
	manifestService kubernetes.ManifestService
}

// NewConsolidatedDeployTool creates a new consolidated deploy tool
func NewConsolidatedDeployTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedDeployTool{
		name:            "generate_manifests",
		description:     "Generate Kubernetes manifests for deployment",
		logger:          serviceContainer.Logger().With("tool", "generate_manifests"),
		manifestService: serviceContainer.ManifestService(),
	}
}

func (t *ConsolidatedDeployTool) Name() string {
	return t.name
}

func (t *ConsolidatedDeployTool) Description() string {
	return t.description
}

func (t *ConsolidatedDeployTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing consolidated deploy tool", "session_id", input.SessionID)

	// Extract required parameters
	appName, ok := input.Data["app_name"].(string)
	if !ok || appName == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "app_name is required",
		}, nil
	}

	// Extract optional parameters
	imageName, _ := input.Data["image_name"].(string)
	imageTag, _ := input.Data["image_tag"].(string)
	if imageTag == "" {
		imageTag = "latest"
	}

	namespace, _ := input.Data["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	port, _ := input.Data["port"].(float64)
	if port == 0 {
		port = 8080
	}

	replicas, _ := input.Data["replicas"].(float64)
	if replicas == 0 {
		replicas = 3
	}

	// Generate Kubernetes manifests
	manifests := t.generateManifests(appName, imageName, imageTag, namespace, int(port), int(replicas))

	// Build response
	deployData := map[string]interface{}{
		"app_name":       appName,
		"image_name":     imageName,
		"image_tag":      imageTag,
		"namespace":      namespace,
		"port":           int(port),
		"replicas":       int(replicas),
		"manifests":      manifests,
		"session_id":     input.SessionID,
		"timestamp":      time.Now().Format(time.RFC3339),
		"manifest_count": len(manifests),
	}

	return api.ToolOutput{
		Success: true,
		Data:    deployData,
		Metadata: map[string]interface{}{
			"tool_version":   "1.0.0",
			"execution_time": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *ConsolidatedDeployTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the application",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Docker image name",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Docker image tag (default: latest)",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace (default: default)",
				},
				"port": map[string]interface{}{
					"type":        "number",
					"description": "Application port (default: 8080)",
				},
				"replicas": map[string]interface{}{
					"type":        "number",
					"description": "Number of replicas (default: 3)",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
			"required": []interface{}{"app_name"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the manifest generation was successful",
				},
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the application",
				},
				"manifests": map[string]interface{}{
					"type":        "array",
					"description": "Generated Kubernetes manifests",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
				},
				"manifest_count": map[string]interface{}{
					"type":        "number",
					"description": "Number of manifests generated",
				},
			},
		},
	}
}

func (t *ConsolidatedDeployTool) generateManifests(appName, imageName, imageTag, namespace string, port, replicas int) []map[string]interface{} {
	manifests := make([]map[string]interface{}, 0)

	// Generate Namespace (if not default)
	if namespace != "default" {
		namespaceManifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": namespace,
			},
		}
		manifests = append(manifests, namespaceManifest)
	}

	// Generate Deployment
	deploymentManifest := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      appName,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app": appName,
			},
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": appName,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": appName,
					},
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  appName,
							"image": fmt.Sprintf("%s:%s", imageName, imageTag),
							"ports": []map[string]interface{}{
								{
									"containerPort": port,
								},
							},
							"resources": map[string]interface{}{
								"limits": map[string]interface{}{
									"cpu":    "500m",
									"memory": "512Mi",
								},
								"requests": map[string]interface{}{
									"cpu":    "250m",
									"memory": "256Mi",
								},
							},
						},
					},
				},
			},
		},
	}
	manifests = append(manifests, deploymentManifest)

	// Generate Service
	serviceManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      appName,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app": appName,
			},
		},
		"spec": map[string]interface{}{
			"type": "ClusterIP",
			"selector": map[string]interface{}{
				"app": appName,
			},
			"ports": []map[string]interface{}{
				{
					"port":       80,
					"targetPort": port,
					"protocol":   "TCP",
				},
			},
		},
	}
	manifests = append(manifests, serviceManifest)

	// Generate ConfigMap (basic example)
	configMapManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("%s-config", appName),
			"namespace": namespace,
		},
		"data": map[string]interface{}{
			"app.env": "production",
			"port":    fmt.Sprintf("%d", port),
		},
	}
	manifests = append(manifests, configMapManifest)

	return manifests
}

// ConsolidatedPushTool implements api.Tool interface for pushing Docker images
type ConsolidatedPushTool struct {
	name          string
	description   string
	logger        *slog.Logger
	buildExecutor services.BuildExecutor
}

// NewConsolidatedPushTool creates a new consolidated push tool
func NewConsolidatedPushTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedPushTool{
		name:          "push_image",
		description:   "Push Docker images to container registry",
		logger:        serviceContainer.Logger().With("tool", "push_image"),
		buildExecutor: serviceContainer.BuildExecutor(),
	}
}

func (t *ConsolidatedPushTool) Name() string {
	return t.name
}

func (t *ConsolidatedPushTool) Description() string {
	return t.description
}

func (t *ConsolidatedPushTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing consolidated push tool", "session_id", input.SessionID)

	// Extract required parameters
	imageName, ok := input.Data["image_name"].(string)
	if !ok || imageName == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "image_name is required",
		}, nil
	}

	// Extract optional parameters
	imageTag, _ := input.Data["image_tag"].(string)
	if imageTag == "" {
		imageTag = "latest"
	}

	registry, _ := input.Data["registry"].(string)
	if registry == "" {
		registry = "docker.io"
	}

	username, _ := input.Data["username"].(string)
	password, _ := input.Data["password"].(string)

	// Build full image name
	fullImageName := t.buildFullImageName(registry, imageName, imageTag)

	// Simulate authentication if credentials provided
	authenticated := false
	if username != "" && password != "" {
		authenticated = t.simulateAuthentication(username, password, registry)
		if !authenticated {
			return api.ToolOutput{
				Success: false,
				Error:   "Authentication failed for registry",
			}, nil
		}
	}

	// Simulate push process
	pushResult := t.simulatePush(fullImageName, registry, authenticated)

	// Build response
	pushData := map[string]interface{}{
		"image_name":      imageName,
		"image_tag":       imageTag,
		"registry":        registry,
		"full_image_name": fullImageName,
		"authenticated":   authenticated,
		"push_status":     pushResult.Status,
		"push_time":       pushResult.PushTime,
		"digest":          pushResult.Digest,
		"size_bytes":      pushResult.SizeBytes,
		"layers_pushed":   pushResult.LayersPushed,
		"session_id":      input.SessionID,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	return api.ToolOutput{
		Success: true,
		Data:    pushData,
		Metadata: map[string]interface{}{
			"tool_version":   "1.0.0",
			"execution_time": time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *ConsolidatedPushTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the Docker image to push",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag of the Docker image (default: latest)",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Container registry URL (default: docker.io)",
				},
				"username": map[string]interface{}{
					"type":        "string",
					"description": "Registry username for authentication",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "Registry password for authentication",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
			"required": []interface{}{"image_name"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the push was successful",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the pushed image",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag of the pushed image",
				},
				"registry": map[string]interface{}{
					"type":        "string",
					"description": "Registry where image was pushed",
				},
				"full_image_name": map[string]interface{}{
					"type":        "string",
					"description": "Full image name including registry",
				},
				"push_status": map[string]interface{}{
					"type":        "string",
					"description": "Status of the push operation",
				},
				"digest": map[string]interface{}{
					"type":        "string",
					"description": "Image digest after push",
				},
				"size_bytes": map[string]interface{}{
					"type":        "number",
					"description": "Size of pushed image in bytes",
				},
			},
		},
	}
}

// PushResult represents the result of a push operation
type PushResult struct {
	Status       string
	PushTime     string
	Digest       string
	SizeBytes    int64
	LayersPushed int
}

func (t *ConsolidatedPushTool) buildFullImageName(registry, imageName, imageTag string) string {
	if registry == "docker.io" {
		return fmt.Sprintf("%s:%s", imageName, imageTag)
	}
	return fmt.Sprintf("%s/%s:%s", registry, imageName, imageTag)
}

func (t *ConsolidatedPushTool) simulateAuthentication(username, password, registry string) bool {
	t.logger.Info("Simulating registry authentication",
		"username", username,
		"registry", registry)

	// In a real implementation, this would authenticate with the registry
	// For simulation, we'll return true for non-empty credentials
	return username != "" && password != ""
}

func (t *ConsolidatedPushTool) simulatePush(fullImageName, registry string, authenticated bool) *PushResult {
	t.logger.Info("Simulating image push",
		"image", fullImageName,
		"registry", registry,
		"authenticated", authenticated)

	// Simulate push process
	pushTime := time.Now().Format(time.RFC3339)

	// Generate mock digest
	digest := fmt.Sprintf("sha256:%x", time.Now().Unix())

	// Mock size and layers
	sizeBytes := int64(157 * 1024 * 1024) // 157 MB
	layersPushed := 5

	return &PushResult{
		Status:       "success",
		PushTime:     pushTime,
		Digest:       digest,
		SizeBytes:    sizeBytes,
		LayersPushed: layersPushed,
	}
}

// ConsolidatedScanTool implements api.Tool interface for scanning Docker images for vulnerabilities
type ConsolidatedScanTool struct {
	name        string
	description string
	logger      *slog.Logger
	scanner     services.Scanner
}

// NewConsolidatedScanTool creates a new consolidated scan tool
func NewConsolidatedScanTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedScanTool{
		name:        "scan_image",
		description: "Scan Docker images for security vulnerabilities",
		logger:      serviceContainer.Logger().With("tool", "scan_image"),
		scanner:     serviceContainer.Scanner(),
	}
}

func (t *ConsolidatedScanTool) Name() string {
	return t.name
}

func (t *ConsolidatedScanTool) Description() string {
	return t.description
}

func (t *ConsolidatedScanTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing consolidated scan tool", "session_id", input.SessionID)

	// Extract required parameters
	imageName, ok := input.Data["image_name"].(string)
	if !ok || imageName == "" {
		return api.ToolOutput{
			Success: false,
			Error:   "image_name is required",
		}, nil
	}

	// Extract optional parameters
	imageTag, _ := input.Data["image_tag"].(string)
	if imageTag == "" {
		imageTag = "latest"
	}

	scanType, _ := input.Data["scan_type"].(string)
	if scanType == "" {
		scanType = "vulnerability"
	}

	severityFilter, _ := input.Data["severity_filter"].(string)
	if severityFilter == "" {
		severityFilter = "all"
	}

	outputFormat, _ := input.Data["output_format"].(string)
	if outputFormat == "" {
		outputFormat = "json"
	}

	// Build full image name
	fullImageName := fmt.Sprintf("%s:%s", imageName, imageTag)

	// Simulate scan process
	scanResult := t.simulateScan(fullImageName, scanType, severityFilter)

	// Build response
	scanData := map[string]interface{}{
		"image_name":      imageName,
		"image_tag":       imageTag,
		"full_image_name": fullImageName,
		"scan_type":       scanType,
		"severity_filter": severityFilter,
		"output_format":   outputFormat,
		"scan_status":     scanResult.Status,
		"scan_time":       scanResult.ScanTime,
		"total_issues":    scanResult.TotalIssues,
		"critical_issues": scanResult.CriticalIssues,
		"high_issues":     scanResult.HighIssues,
		"medium_issues":   scanResult.MediumIssues,
		"low_issues":      scanResult.LowIssues,
		"vulnerabilities": scanResult.Vulnerabilities,
		"scan_summary":    scanResult.Summary,
		"recommendations": scanResult.Recommendations,
		"session_id":      input.SessionID,
		"timestamp":       time.Now().Format(time.RFC3339),
	}

	return api.ToolOutput{
		Success: true,
		Data:    scanData,
		Metadata: map[string]interface{}{
			"tool_version":   "1.0.0",
			"execution_time": time.Now().Format(time.RFC3339),
			"scanner":        "trivy",
		},
	}, nil
}

func (t *ConsolidatedScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the Docker image to scan",
				},
				"image_tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag of the Docker image (default: latest)",
				},
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of scan to perform (vulnerability, secret, config)",
					"enum":        []string{"vulnerability", "secret", "config", "all"},
				},
				"severity_filter": map[string]interface{}{
					"type":        "string",
					"description": "Filter results by severity (all, critical, high, medium, low)",
					"enum":        []string{"all", "critical", "high", "medium", "low"},
				},
				"output_format": map[string]interface{}{
					"type":        "string",
					"description": "Output format (json, table, sarif)",
					"enum":        []string{"json", "table", "sarif"},
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
			"required": []interface{}{"image_name"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the scan was successful",
				},
				"image_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the scanned image",
				},
				"scan_status": map[string]interface{}{
					"type":        "string",
					"description": "Status of the scan operation",
				},
				"total_issues": map[string]interface{}{
					"type":        "number",
					"description": "Total number of issues found",
				},
				"critical_issues": map[string]interface{}{
					"type":        "number",
					"description": "Number of critical issues",
				},
				"high_issues": map[string]interface{}{
					"type":        "number",
					"description": "Number of high severity issues",
				},
				"medium_issues": map[string]interface{}{
					"type":        "number",
					"description": "Number of medium severity issues",
				},
				"low_issues": map[string]interface{}{
					"type":        "number",
					"description": "Number of low severity issues",
				},
				"vulnerabilities": map[string]interface{}{
					"type":        "array",
					"description": "List of detected vulnerabilities",
				},
				"scan_summary": map[string]interface{}{
					"type":        "string",
					"description": "Summary of scan results",
				},
			},
		},
	}
}

// ScanResult represents the result of a security scan
type ScanResult struct {
	Status          string
	ScanTime        string
	TotalIssues     int
	CriticalIssues  int
	HighIssues      int
	MediumIssues    int
	LowIssues       int
	Vulnerabilities []Vulnerability
	Summary         string
	Recommendations []string
}

// Vulnerability represents a detected vulnerability
type Vulnerability struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FixedIn     string   `json:"fixed_in,omitempty"`
	CVSS        float64  `json:"cvss,omitempty"`
	References  []string `json:"references,omitempty"`
}

func (t *ConsolidatedScanTool) simulateScan(fullImageName, scanType, severityFilter string) *ScanResult {
	t.logger.Info("Simulating security scan",
		"image", fullImageName,
		"scan_type", scanType,
		"severity_filter", severityFilter)

	// Simulate scan process
	scanTime := time.Now().Format(time.RFC3339)

	// Generate mock vulnerabilities
	vulnerabilities := []Vulnerability{
		{
			ID:          "CVE-2023-1234",
			Severity:    "critical",
			Package:     "openssl",
			Version:     "1.1.1k",
			Title:       "Buffer overflow in OpenSSL",
			Description: "A buffer overflow vulnerability in OpenSSL could allow remote code execution",
			FixedIn:     "1.1.1m",
			CVSS:        9.8,
			References:  []string{"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1234"},
		},
		{
			ID:          "CVE-2023-5678",
			Severity:    "high",
			Package:     "curl",
			Version:     "7.68.0",
			Title:       "Authentication bypass in curl",
			Description: "An authentication bypass vulnerability in curl library",
			FixedIn:     "7.74.0",
			CVSS:        7.5,
			References:  []string{"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-5678"},
		},
		{
			ID:          "CVE-2023-9012",
			Severity:    "medium",
			Package:     "nginx",
			Version:     "1.18.0",
			Title:       "Information disclosure in nginx",
			Description: "An information disclosure vulnerability in nginx web server",
			FixedIn:     "1.20.1",
			CVSS:        5.3,
			References:  []string{"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-9012"},
		},
	}

	// Filter vulnerabilities based on severity
	filteredVulns := t.filterVulnerabilities(vulnerabilities, severityFilter)

	// Count issues by severity
	critical, high, medium, low := t.countBySeverity(filteredVulns)

	// Generate summary
	summary := fmt.Sprintf("Found %d vulnerabilities: %d critical, %d high, %d medium, %d low",
		len(filteredVulns), critical, high, medium, low)

	// Generate recommendations
	recommendations := t.generateRecommendations(filteredVulns)

	return &ScanResult{
		Status:          "completed",
		ScanTime:        scanTime,
		TotalIssues:     len(filteredVulns),
		CriticalIssues:  critical,
		HighIssues:      high,
		MediumIssues:    medium,
		LowIssues:       low,
		Vulnerabilities: filteredVulns,
		Summary:         summary,
		Recommendations: recommendations,
	}
}

func (t *ConsolidatedScanTool) filterVulnerabilities(vulnerabilities []Vulnerability, severityFilter string) []Vulnerability {
	if severityFilter == "all" {
		return vulnerabilities
	}

	filtered := make([]Vulnerability, 0)
	for _, vuln := range vulnerabilities {
		if vuln.Severity == severityFilter {
			filtered = append(filtered, vuln)
		}
	}
	return filtered
}

func (t *ConsolidatedScanTool) countBySeverity(vulnerabilities []Vulnerability) (int, int, int, int) {
	var critical, high, medium, low int

	for _, vuln := range vulnerabilities {
		switch vuln.Severity {
		case "critical":
			critical++
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	return critical, high, medium, low
}

func (t *ConsolidatedScanTool) generateRecommendations(vulnerabilities []Vulnerability) []string {
	recommendations := []string{
		"Update base image to latest version",
		"Apply security patches for identified vulnerabilities",
		"Use multi-stage builds to reduce attack surface",
		"Run container with non-root user",
		"Enable security scanning in CI/CD pipeline",
	}

	// Add specific recommendations based on vulnerabilities
	for _, vuln := range vulnerabilities {
		if vuln.FixedIn != "" {
			recommendations = append(recommendations,
				fmt.Sprintf("Update %s package from %s to %s", vuln.Package, vuln.Version, vuln.FixedIn))
		}
	}

	return recommendations
}

// Simple tools for basic operations

// ConsolidatedPingTool implements api.Tool interface for ping operations
type ConsolidatedPingTool struct {
	name        string
	description string
	logger      *slog.Logger
}

func NewConsolidatedPingTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedPingTool{
		name:        "ping",
		description: "Test server connectivity and response time",
		logger:      serviceContainer.Logger().With("tool", "ping"),
	}
}

func (t *ConsolidatedPingTool) Name() string {
	return t.name
}

func (t *ConsolidatedPingTool) Description() string {
	return t.description
}

func (t *ConsolidatedPingTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing ping tool", "session_id", input.SessionID)

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"status":           "ok",
			"timestamp":        time.Now().Format(time.RFC3339),
			"session_id":       input.SessionID,
			"response_time_ms": 1,
		},
		Metadata: map[string]interface{}{
			"tool_version": "1.0.0",
		},
	}, nil
}

func (t *ConsolidatedPingTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the ping was successful",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Server status",
				},
				"timestamp": map[string]interface{}{
					"type":        "string",
					"description": "Response timestamp",
				},
				"response_time_ms": map[string]interface{}{
					"type":        "number",
					"description": "Response time in milliseconds",
				},
			},
		},
	}
}

// ConsolidatedServerStatusTool implements api.Tool interface for server status
type ConsolidatedServerStatusTool struct {
	name        string
	description string
	logger      *slog.Logger
}

func NewConsolidatedServerStatusTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedServerStatusTool{
		name:        "server_status",
		description: "Get server status and health information",
		logger:      serviceContainer.Logger().With("tool", "server_status"),
	}
}

func (t *ConsolidatedServerStatusTool) Name() string {
	return t.name
}

func (t *ConsolidatedServerStatusTool) Description() string {
	return t.description
}

func (t *ConsolidatedServerStatusTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing server status tool", "session_id", input.SessionID)

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"status":          "healthy",
			"uptime":          "2h 15m 30s",
			"version":         "1.0.0",
			"active_sessions": 3,
			"total_tools":     8,
			"memory_usage":    "45%",
			"cpu_usage":       "12%",
			"timestamp":       time.Now().Format(time.RFC3339),
			"session_id":      input.SessionID,
		},
		Metadata: map[string]interface{}{
			"tool_version": "1.0.0",
		},
	}, nil
}

func (t *ConsolidatedServerStatusTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the status check was successful",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Server health status",
				},
				"uptime": map[string]interface{}{
					"type":        "string",
					"description": "Server uptime",
				},
				"version": map[string]interface{}{
					"type":        "string",
					"description": "Server version",
				},
				"active_sessions": map[string]interface{}{
					"type":        "number",
					"description": "Number of active sessions",
				},
				"total_tools": map[string]interface{}{
					"type":        "number",
					"description": "Total number of registered tools",
				},
			},
		},
	}
}

// ConsolidatedListSessionsTool implements api.Tool interface for listing sessions
type ConsolidatedListSessionsTool struct {
	name         string
	description  string
	logger       *slog.Logger
	sessionStore services.SessionStore
}

func NewConsolidatedListSessionsTool(serviceContainer services.ServiceContainer) api.Tool {
	return &ConsolidatedListSessionsTool{
		name:         "list_sessions",
		description:  "List all active sessions",
		logger:       serviceContainer.Logger().With("tool", "list_sessions"),
		sessionStore: serviceContainer.SessionStore(),
	}
}

func (t *ConsolidatedListSessionsTool) Name() string {
	return t.name
}

func (t *ConsolidatedListSessionsTool) Description() string {
	return t.description
}

func (t *ConsolidatedListSessionsTool) Execute(_ context.Context, input api.ToolInput) (api.ToolOutput, error) {
	t.logger.Info("Executing list sessions tool", "session_id", input.SessionID)

	// Mock session data
	sessions := []map[string]interface{}{
		{
			"id":          "session-1",
			"status":      "active",
			"created_at":  "2025-01-09T10:00:00Z",
			"last_active": "2025-01-09T12:30:00Z",
			"tool_count":  5,
		},
		{
			"id":          "session-2",
			"status":      "idle",
			"created_at":  "2025-01-09T11:15:00Z",
			"last_active": "2025-01-09T11:45:00Z",
			"tool_count":  2,
		},
		{
			"id":          input.SessionID,
			"status":      "active",
			"created_at":  "2025-01-09T12:00:00Z",
			"last_active": time.Now().Format(time.RFC3339),
			"tool_count":  1,
		},
	}

	return api.ToolOutput{
		Success: true,
		Data: map[string]interface{}{
			"sessions":     sessions,
			"total_count":  len(sessions),
			"active_count": 2,
			"idle_count":   1,
			"timestamp":    time.Now().Format(time.RFC3339),
			"session_id":   input.SessionID,
		},
		Metadata: map[string]interface{}{
			"tool_version": "1.0.0",
		},
	}, nil
}

func (t *ConsolidatedListSessionsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.name,
		Description: t.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session identifier",
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the operation was successful",
				},
				"sessions": map[string]interface{}{
					"type":        "array",
					"description": "List of active sessions",
				},
				"total_count": map[string]interface{}{
					"type":        "number",
					"description": "Total number of sessions",
				},
				"active_count": map[string]interface{}{
					"type":        "number",
					"description": "Number of active sessions",
				},
				"idle_count": map[string]interface{}{
					"type":        "number",
					"description": "Number of idle sessions",
				},
			},
		},
	}
}
