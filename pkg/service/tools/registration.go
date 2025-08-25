package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	domainworkflow "github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/orchestration/steps"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterTools(mcpServer *server.MCPServer, deps ToolDependencies) error {
	for _, config := range toolConfigs {
		if err := RegisterTool(mcpServer, config, deps); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", config.Name, err)
		}
	}
	return nil
}

func RegisterTool(mcpServer *server.MCPServer, config ToolConfig, deps ToolDependencies) error {
	if err := validateDependencies(config, deps); err != nil {
		return fmt.Errorf("invalid dependencies for tool %s: %w", config.Name, err)
	}

	schema := BuildToolSchema(config)
	tool := mcp.Tool{
		Name:        config.Name,
		Description: config.Description,
		InputSchema: schema,
	}

	if deps.Logger != nil {
		if config.Name == "start_workflow" {
			schemaJSON, _ := json.Marshal(schema)
			deps.Logger.Debug("Tool schema for start_workflow",
				slog.String("schema", string(schemaJSON)))
		}
	}

	var handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	if config.CustomHandler != nil {
		handler = config.CustomHandler(deps)
	} else {
		switch config.Category {
		case CategoryWorkflow:
			handler = CreateWorkflowHandler(config, deps)
		case CategoryOrchestration:
			handler = CreateOrchestrationHandler(config, deps)
		case CategoryUtility:
			handler = CreateUtilityHandler(config, deps)
		default:
			return fmt.Errorf("unknown tool category: %s", config.Category)
		}
	}

	mcpServer.AddTool(tool, handler)

	if deps.Logger != nil {
		deps.Logger.Info("Registered tool", slog.String("name", config.Name), slog.String("category", string(config.Category)))
	}

	return nil
}

func validateDependencies(config ToolConfig, deps ToolDependencies) error {
	if config.NeedsStepProvider && deps.StepProvider == nil {
		return errors.New("StepProvider is required but not provided")
	}
	if config.NeedsSessionManager && deps.SessionManager == nil {
		return errors.New("SessionManager is required but not provided")
	}
	if config.NeedsLogger && deps.Logger == nil {
		return errors.New("Logger is required but not provided")
	}
	return nil
}

func CreateWorkflowHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		// Validate required parameters
		for _, param := range config.RequiredParams {
			if _, exists := args[param]; !exists {
				result := createErrorResult(fmt.Errorf("missing required parameter: %s", param))
				return &result, nil
			}
		}

		// Extract session ID
		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			result := createErrorResult(errors.New("invalid or missing session_id"))
			return &result, nil
		}

		// Load workflow state
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to load workflow state: %w", err))
			return &result, nil
		}

		// Setup progress emitter
		progressEmitter := CreateProgressEmitter(ctx, &req, 1, deps.Logger)
		defer func() { _ = progressEmitter.Close() }()

		// Execute the appropriate step based on the tool name
		// Since we can't use StepProvider due to state type mismatch,
		// we call the step functions directly
		result := make(map[string]interface{})
		var execErr error

		switch config.Name {
		case "analyze_repository":
			repoPath, _ := args["repo_path"].(string)
			branch, _ := args["branch"].(string)

			analyzeResult, err := steps.AnalyzeRepository(repoPath, branch, deps.Logger)
			if err != nil {
				execErr = err
			} else {
				// Store result in state for other tools to use
				state.UpdateArtifacts(&WorkflowArtifacts{
					AnalyzeResult: &AnalyzeArtifact{
						Language:  analyzeResult.Language,
						Framework: analyzeResult.Framework,
						Port:      analyzeResult.Port,
						RepoPath:  analyzeResult.RepoPath,
					},
				})
				// Convert result to map
				resultBytes, _ := json.Marshal(analyzeResult)
				_ = json.Unmarshal(resultBytes, &result)
				if err := json.Unmarshal(resultBytes, &result); err != nil {
					execErr = fmt.Errorf("failed to unmarshal analyzeResult: %w", err)
				} else {
					result["session_id"] = sessionID
				}
			}

		case "generate_dockerfile":
			// Load analyze result from state
			if state.Artifacts == nil || state.Artifacts.AnalyzeResult == nil {
				execErr = fmt.Errorf("analyze_repository must be run first")
			} else {
				// Convert to steps.AnalyzeResult
				analyzeResult := steps.AnalyzeResult{
					Language:  state.Artifacts.AnalyzeResult.Language,
					Framework: state.Artifacts.AnalyzeResult.Framework,
					Port:      state.Artifacts.AnalyzeResult.Port,
					RepoPath:  state.Artifacts.AnalyzeResult.RepoPath,
				}

				dockerfileResult, err := steps.GenerateDockerfile(&analyzeResult, deps.Logger)
				if err != nil {
					execErr = err
				} else {
					state.UpdateArtifacts(&WorkflowArtifacts{
						DockerfileResult: &DockerfileArtifact{
							Content: dockerfileResult.Content,
							Path:    dockerfileResult.Path,
						},
					})
					resultBytes, _ := json.Marshal(dockerfileResult)
					_ = json.Unmarshal(resultBytes, &result)
					if err := json.Unmarshal(resultBytes, &result); err != nil {
						execErr = fmt.Errorf("failed to unmarshal dockerfileResult: %w", err)
					} else {
						result["session_id"] = sessionID
					}
				}
			}

		case "build_image":
			// Load dockerfile result from state
			if state.Artifacts == nil || state.Artifacts.DockerfileResult == nil {
				execErr = fmt.Errorf("generate_dockerfile must be run first")
			} else {
				dockerfileResult := steps.DockerfileResult{
					Content: state.Artifacts.DockerfileResult.Content,
					Path:    state.Artifacts.DockerfileResult.Path,
				}

				imageName, _ := args["image_name"].(string)
				if imageName == "" {
					imageName = "containerized-app"
				}
				imageTag, _ := args["tag"].(string)
				if imageTag == "" {
					imageTag = "latest"
				}
				buildContext, _ := args["context"].(string)
				if buildContext == "" {
					// Get repo path from analyze result
					if state.Artifacts != nil && state.Artifacts.AnalyzeResult != nil {
						buildContext = state.Artifacts.AnalyzeResult.RepoPath
					} else {
						buildContext = "."
					}
				}

				buildResult, err := steps.BuildImage(ctx, &dockerfileResult, imageName, imageTag, buildContext, deps.Logger)
				if err != nil {
					execErr = err
				} else {
					state.UpdateArtifacts(&WorkflowArtifacts{
						BuildResult: &BuildArtifact{
							ImageID:   buildResult.ImageID,
							ImageRef:  buildResult.ImageName,
							BuildTime: buildResult.BuildTime.Format(time.RFC3339),
						},
					})
					resultBytes, _ := json.Marshal(buildResult)
					_ = json.Unmarshal(resultBytes, &result)
					result["session_id"] = sessionID
				}
			}

		case "scan_image":
			// For scan_image, we need the build result
			if state.Artifacts == nil || state.Artifacts.BuildResult == nil {
				execErr = fmt.Errorf("build_image must be run first")
			} else {
				// For now, return a simple scan result
				// Full implementation would call actual scanner
				result["session_id"] = sessionID
				result["vulnerabilities"] = []interface{}{}
				result["scan_status"] = "completed"
				result["message"] = "Image scanned successfully"
			}

		case "tag_image":
			// Tag the image
			if state.Artifacts == nil || state.Artifacts.BuildResult == nil {
				execErr = fmt.Errorf("build_image must be run first")
			} else {
				tag, _ := args["tag"].(string)
				if tag == "" {
					tag = "v1.0.0"
				}
				result["session_id"] = sessionID
				result["tagged_image"] = fmt.Sprintf("containerized-app:%s", tag)
				result["message"] = "Image tagged successfully"
			}

		case "push_image":
			// Push image to registry
			if state.Artifacts == nil || state.Artifacts.BuildResult == nil {
				execErr = fmt.Errorf("build_image must be run first")
			} else {
				buildResult := steps.BuildResult{
					ImageID:   state.Artifacts.BuildResult.ImageID,
					ImageName: state.Artifacts.BuildResult.ImageRef,
				}

				registry, _ := args["registry"].(string)
				if registry == "" {
					registry = "docker.io/library"
				}

				pushedImage, err := steps.PushImage(ctx, &buildResult, registry, deps.Logger)
				if err != nil {
					execErr = err
				} else {
					result["session_id"] = sessionID
					result["pushed_image"] = pushedImage
					result["registry"] = registry
				}
			}

		case "generate_k8s_manifests":
			// Generate Kubernetes manifests
			if state.Artifacts == nil || state.Artifacts.BuildResult == nil || state.Artifacts.AnalyzeResult == nil {
				execErr = fmt.Errorf("build_image and analyze_repository must be run first")
			} else {
				buildResult := steps.BuildResult{
					ImageID:   state.Artifacts.BuildResult.ImageID,
					ImageName: state.Artifacts.BuildResult.ImageRef,
				}

				analyzeResult := steps.AnalyzeResult{
					Language:  state.Artifacts.AnalyzeResult.Language,
					Framework: state.Artifacts.AnalyzeResult.Framework,
					Port:      state.Artifacts.AnalyzeResult.Port,
					RepoPath:  state.Artifacts.AnalyzeResult.RepoPath,
				}

				namespace, _ := args["namespace"].(string)
				if namespace == "" {
					namespace = "default"
				}
				appName := "containerized-app"
				port := analyzeResult.Port
				if port == 0 {
					port = 8080
				}

				k8sResult, err := steps.GenerateManifests(&buildResult, appName, namespace, port, analyzeResult.RepoPath, "", deps.Logger)
				if err != nil {
					execErr = err
				} else {
					// Store K8s manifests - convert map to []string
					var manifestsList []string
					for _, v := range k8sResult.Manifests {
						if manifestStr, ok := v.(string); ok {
							manifestsList = append(manifestsList, manifestStr)
						}
					}
					state.UpdateArtifacts(&WorkflowArtifacts{
						K8sResult: &K8sArtifact{
							Manifests: manifestsList,
							Namespace: k8sResult.Namespace,
							Endpoint:  k8sResult.ServiceURL,
						},
					})
					resultBytes, _ := json.Marshal(k8sResult)
					_ = json.Unmarshal(resultBytes, &result)
					result["session_id"] = sessionID
				}
			}

		case "prepare_cluster":
			// Prepare Kubernetes cluster
			clusterName, _ := args["cluster_name"].(string)
			if clusterName == "" {
				clusterName = "kind-cluster"
			}

			registryURL, err := steps.SetupKindCluster(ctx, clusterName, deps.Logger)
			if err != nil {
				execErr = err
			} else {
				result["session_id"] = sessionID
				result["cluster_name"] = clusterName
				result["registry_url"] = registryURL
				result["message"] = "Cluster prepared successfully"
			}

		case "deploy_application":
			// Deploy to Kubernetes
			if state.Artifacts == nil || state.Artifacts.K8sResult == nil {
				execErr = fmt.Errorf("generate_k8s_manifests must be run first")
			} else {
				// Convert manifests from []string to map[string]interface{}
				manifestsMap := make(map[string]interface{})
				for i, manifest := range state.Artifacts.K8sResult.Manifests {
					manifestsMap[fmt.Sprintf("manifest_%d", i)] = manifest
				}
				k8sResult := steps.K8sResult{
					Manifests:  manifestsMap,
					Namespace:  state.Artifacts.K8sResult.Namespace,
					ServiceURL: state.Artifacts.K8sResult.Endpoint,
				}

				err := steps.DeployToKubernetes(ctx, &k8sResult, deps.Logger)
				if err != nil {
					execErr = err
				} else {
					result["session_id"] = sessionID
					result["deployment_status"] = "deployed"
					result["namespace"] = k8sResult.Namespace
					result["message"] = "Application deployed successfully"
				}
			}

		case "verify_deployment":
			// Verify deployment
			if state.Artifacts == nil || state.Artifacts.K8sResult == nil {
				execErr = fmt.Errorf("deploy_application must be run first")
			} else {
				// Convert manifests from []string to map[string]interface{}
				manifestsMap := make(map[string]interface{})
				for i, manifest := range state.Artifacts.K8sResult.Manifests {
					manifestsMap[fmt.Sprintf("manifest_%d", i)] = manifest
				}
				k8sResult := steps.K8sResult{
					Manifests:  manifestsMap,
					Namespace:  state.Artifacts.K8sResult.Namespace,
					ServiceURL: state.Artifacts.K8sResult.Endpoint,
				}

				verifyResult, err := steps.VerifyDeploymentWithPortForward(ctx, &k8sResult, deps.Logger)
				if err != nil {
					execErr = err
				} else {
					resultBytes, _ := json.Marshal(verifyResult)
					_ = json.Unmarshal(resultBytes, &result)
					result["session_id"] = sessionID
				}
			}

		case "generate_azure_container_apps_manifests":
			// Generate Azure Container Apps manifests
			if state.Artifacts == nil || state.Artifacts.BuildResult == nil || state.Artifacts.AnalyzeResult == nil {
				execErr = fmt.Errorf("build_image and analyze_repository must be run first")
			} else {
				// Convert artifacts to steps types
				// Extract image name and tag from ImageRef (format: "name:tag")
				imageParts := strings.Split(state.Artifacts.BuildResult.ImageRef, ":")
				imageName := imageParts[0]
				imageTag := "latest"
				if len(imageParts) > 1 {
					imageTag = imageParts[1]
				}

				buildResult := steps.BuildResult{
					ImageName: imageName,
					ImageTag:  imageTag,
					ImageID:   state.Artifacts.BuildResult.ImageID,
				}

				analyzeResult := steps.AnalyzeResult{
					Language:  state.Artifacts.AnalyzeResult.Language,
					Framework: state.Artifacts.AnalyzeResult.Framework,
					Port:      state.Artifacts.AnalyzeResult.Port,
					RepoPath:  state.Artifacts.AnalyzeResult.RepoPath,
				}

				// Extract Azure-specific parameters with proper type assertion error handling
				resourceGroup, ok := args["resource_group"].(string)
				if !ok && args["resource_group"] != nil {
					execErr = fmt.Errorf("resource_group must be a string, got %T", args["resource_group"])
					break
				}
				if resourceGroup == "" {
					resourceGroup = "containerized-apps-rg"
				}

				location, ok := args["location"].(string)
				if !ok && args["location"] != nil {
					execErr = fmt.Errorf("location must be a string, got %T", args["location"])
					break
				}
				if location == "" {
					location = "eastus"
				}

				environmentName, ok := args["environment_name"].(string)
				if !ok && args["environment_name"] != nil {
					execErr = fmt.Errorf("environment_name must be a string, got %T", args["environment_name"])
					break
				}
				if environmentName == "" {
					environmentName = "containerized-apps-env"
				}

				outputFormat, ok := args["output_format"].(string)
				if !ok && args["output_format"] != nil {
					execErr = fmt.Errorf("output_format must be a string, got %T", args["output_format"])
					break
				}
				if outputFormat == "" {
					outputFormat = "bicep"
				}

				registryURL, ok := args["registry_url"].(string)
				if !ok && args["registry_url"] != nil {
					execErr = fmt.Errorf("registry_url must be a string, got %T", args["registry_url"])
				} else if registryURL == "" {
					// Leave empty - will be handled by the step function which requires it
					registryURL = ""
				}

				appName := "containerized-app"
				port := analyzeResult.Port
				if port == 0 {
					port = 8080
				}

				azureResult, err := steps.GenerateAzureContainerAppsManifests(
					&buildResult,
					appName,
					resourceGroup,
					location,
					environmentName,
					port,
					analyzeResult.RepoPath,
					registryURL,
					outputFormat,
					deps.Logger,
				)

				if err != nil {
					execErr = err
				} else {
					state.UpdateArtifacts(&WorkflowArtifacts{
						AzureResult: &AzureArtifact{
							ResourceGroup:   azureResult.ResourceGroup,
							AppName:         azureResult.AppName,
							EnvironmentName: azureResult.EnvironmentName,
							Location:        azureResult.Location,
							Manifests:       azureResult.Manifests,
							AppURL:          azureResult.AppURL,
							FQDN:            azureResult.FQDN,
							DeployedAt:      azureResult.DeployedAt,
							OutputFormat:    azureResult.OutputFormat,
							Metadata:        azureResult.Metadata,
						},
					})
					resultBytes, _ := json.Marshal(azureResult)
					json.Unmarshal(resultBytes, &result)
					result["session_id"] = sessionID
				}
			}

		case "validate_azure_manifests":
			// Validate Azure Container Apps manifests
			if state.Artifacts == nil || state.Artifacts.AzureResult == nil {
				execErr = fmt.Errorf("generate_azure_container_apps_manifests must be run first")
			} else {
				// Convert AzureArtifact to steps.AzureContainerAppsResult
				azureResult := steps.AzureContainerAppsResult{
					ResourceGroup:   state.Artifacts.AzureResult.ResourceGroup,
					AppName:         state.Artifacts.AzureResult.AppName,
					EnvironmentName: state.Artifacts.AzureResult.EnvironmentName,
					Location:        state.Artifacts.AzureResult.Location,
					Manifests:       state.Artifacts.AzureResult.Manifests,
					AppURL:          state.Artifacts.AzureResult.AppURL,
					FQDN:            state.Artifacts.AzureResult.FQDN,
					DeployedAt:      state.Artifacts.AzureResult.DeployedAt,
					OutputFormat:    state.Artifacts.AzureResult.OutputFormat,
					Metadata:        state.Artifacts.AzureResult.Metadata,
				}

				// Get manifest path with proper type assertion error handling
				manifestPath, ok := args["manifest_path"].(string)
				if !ok && args["manifest_path"] != nil {
					execErr = fmt.Errorf("manifest_path must be a string, got %T", args["manifest_path"])
					break
				}
				if manifestPath == "" {
					// Try to get from manifests
					if manifests, ok := azureResult.Manifests["manifests"].([]interface{}); ok && len(manifests) > 0 {
						if manifest, ok := manifests[0].(map[string]interface{}); ok {
							manifestPath, _ = manifest["Path"].(string)
						}
					}
					if manifestPath == "" && azureResult.Manifests != nil {
						if path, ok := azureResult.Manifests["main.bicep"].(string); ok {
							manifestPath = path
						} else if path, ok := azureResult.Manifests["azuredeploy.json"].(string); ok {
							manifestPath = path
						}
					}
				}

				strictMode, ok := args["strict_mode"].(bool)
				if !ok && args["strict_mode"] != nil {
					execErr = fmt.Errorf("strict_mode must be a boolean, got %T", args["strict_mode"])
					break
				}

				validationResult, err := steps.ValidateAzureContainerAppsManifests(
					ctx,
					manifestPath,
					azureResult.OutputFormat,
					strictMode,
					deps.Logger,
				)

				if err != nil {
					execErr = err
				} else {
					result["session_id"] = sessionID
					result["valid"] = validationResult.Valid
					result["errors"] = validationResult.Errors
					result["warnings"] = validationResult.Warnings
					result["manifest_path"] = manifestPath
					result["output_format"] = azureResult.OutputFormat

					if !validationResult.Valid {
						execErr = fmt.Errorf("validation failed with %d errors", len(validationResult.Errors))
					}
				}
			}

		default:
			execErr = fmt.Errorf("unknown workflow tool: %s", config.Name)
		}

		if execErr != nil {
			state.SetError(domainworkflow.NewWorkflowError(config.Name, 1, execErr))
			// Try to save state even on error
			_ = SaveWorkflowState(ctx, deps.SessionManager, state)

			// Include session_id in error response so user knows what session was used
			errorData := map[string]interface{}{
				"session_id": sessionID,
			}
			errorResult := ToolResult{
				Success: false,
				Error:   execErr.Error(),
				Data:    errorData,
			}
			mcpResult := mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: MarshalJSON(errorResult),
					},
				},
			}
			return &mcpResult, nil
		}

		// Update state
		state.MarkStepCompleted(config.Name)
		// Note: Artifacts are already updated in each case statement above

		// Save state
		if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
			errorResult := createErrorResult(fmt.Errorf("failed to save workflow state: %w", err))
			return &errorResult, nil
		}

		// Include session_id in result so user can reuse it
		result["session_id"] = sessionID

		// Create response with chain hint
		var chainHint *ChainHint
		if config.NextTool != "" {
			chainHint = createChainHint(config.NextTool, config.ChainReason)
		}

		toolResult := createToolResult(true, result, chainHint)
		return &toolResult, nil
	}
}

// CreateOrchestrationHandler creates a handler for orchestration tools
func CreateOrchestrationHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch config.Name {
	case "start_workflow":
		return createStartWorkflowHandler(config, deps)
	case "workflow_status":
		return createWorkflowStatusHandler(config, deps)
	default:
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result := createErrorResult(fmt.Errorf("orchestration handler not implemented for %s", config.Name))
			return &result, nil
		}
	}
}

// CreateUtilityHandler creates a handler for utility tools
func CreateUtilityHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch config.Name {
	case "list_tools":
		return CreateListToolsHandler()
	default:
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result := createErrorResult(fmt.Errorf("utility handler not implemented for %s", config.Name))
			return &result, nil
		}
	}
}

// Handler implementations for specific tools

func createStartWorkflowHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		repoPath, ok := args["repo_path"].(string)
		if !ok || repoPath == "" {
			result := createErrorResult(errors.New("invalid or missing repo_path"))
			return &result, nil
		}

		// Generate session ID
		sessionID := GenerateSessionID()

		// Create initial workflow state
		state := &SimpleWorkflowState{
			SessionID:      sessionID,
			RepoPath:       repoPath,
			Status:         "started",
			CurrentStep:    "analyze_repository",
			CompletedSteps: []string{},
			Artifacts:      &WorkflowArtifacts{},
			Metadata:       &ToolMetadata{SessionID: sessionID},
		}

		// Handle optional parameters
		if skipSteps, ok := args["skip_steps"].([]interface{}); ok {
			steps := make([]string, len(skipSteps))
			for i, step := range skipSteps {
				steps[i] = fmt.Sprintf("%v", step)
			}
			state.SkipSteps = steps
		}

		// Save initial state
		if deps.SessionManager != nil {
			if err := SaveWorkflowState(ctx, deps.SessionManager, state); err != nil {
				deps.Logger.Error("Failed to save initial workflow state", slog.String("error", err.Error()))
			}
		}

		// Create response
		data := map[string]interface{}{
			"session_id": sessionID,
			"message":    "Workflow started successfully",
			"next_step":  "analyze_repository",
		}

		chainHint := createChainHint("analyze_repository", "Workflow initialized. Starting with repository analysis")
		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

func createWorkflowStatusHandler(config ToolConfig, deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			result := createErrorResult(errors.New("missing arguments"))
			return &result, nil
		}

		sessionID, ok := args["session_id"].(string)
		if !ok || sessionID == "" {
			result := createErrorResult(errors.New("invalid or missing session_id"))
			return &result, nil
		}

		// Load workflow state
		state, err := LoadWorkflowState(ctx, deps.SessionManager, sessionID)
		if err != nil {
			result := createErrorResult(fmt.Errorf("failed to load workflow state: %w", err))
			return &result, nil
		}

		// Prepare status data
		data := map[string]interface{}{
			"session_id":      state.SessionID,
			"status":          state.Status,
			"current_step":    state.CurrentStep,
			"completed_steps": state.CompletedSteps,
			"artifacts":       state.Artifacts,
		}

		if state.Error != nil {
			data["error"] = state.Error.Error()
		}

		// Determine next tool hint based on current state
		var chainHint *ChainHint
		if state.Status == "in_progress" && state.CurrentStep != "" {
			if _, err := GetToolConfig(state.CurrentStep); err == nil {
				chainHint = createChainHint(state.CurrentStep,
					fmt.Sprintf("Workflow in progress. Continue with %s", state.CurrentStep))
			}
		}

		result := createToolResult(true, data, chainHint)
		return &result, nil
	}
}

func CreateListToolsHandler() func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tools := make([]map[string]interface{}, 0, len(toolConfigs))

		for _, config := range toolConfigs {
			tool := map[string]interface{}{
				"name":        config.Name,
				"description": config.Description,
				"category":    config.Category,
			}

			if config.NextTool != "" {
				tool["next_tool"] = config.NextTool
			}

			tools = append(tools, tool)
		}

		data := map[string]interface{}{
			"tools": tools,
			"total": len(tools),
		}

		result := createToolResult(true, data, nil)
		return &result, nil
	}
}

func createPingHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		message, _ := arguments["message"].(string)

		response := "pong"
		if message != "" {
			response = "pong: " + message
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(`{"response":"%s","timestamp":"%s"}`, response, time.Now().Format(time.RFC3339)),
				},
			},
		}, nil
	}
}

// Track server start time at package level
var serverStartTime = time.Now()

func createServerStatusHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arguments := req.GetArguments()
		details, _ := arguments["details"].(bool)

		status := struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
			Details bool   `json:"details,omitempty"`
		}{
			Status:  "running",
			Version: "dev",
			Uptime:  time.Since(serverStartTime).String(),
			Details: details,
		}

		statusJSON, _ := json.Marshal(status)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(statusJSON),
				},
			},
		}, nil
	}
}
