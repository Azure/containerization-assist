package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// serverWithCapabilities is a unified helper interface for servers
type serverWithCapabilities interface {
	// Registry capability
	GetToolRegistry() interface {
		GetToolSchema(string) (map[string]interface{}, error)
	}
	// Orchestrator capability
	GetToolOrchestrator() interface {
		GetToolMetadata(string) (interface{}, error)
	}
}

// ============================================================================
// Core HTTP Request Handlers
// ============================================================================

// This file contains the core HTTP handlers for the MCP transport layer.
// These handlers implement the main HTTP endpoints for the MCP protocol.

// ============================================================================
// Core Request Handlers
// ============================================================================

// handleOptions handles preflight OPTIONS requests for CORS support.
// This handler responds to browser preflight requests with appropriate headers.
func (t *HTTPTransport) handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleGetToolSchema returns the detailed schema for a specific tool.
// This endpoint provides comprehensive schema information including parameters,
// types, validation rules, and examples for a named tool.
func (t *HTTPTransport) handleGetToolSchema(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	t.toolsMutex.RLock()
	toolInfo, exists := t.tools[toolName]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Tool '%s' not found", toolName))
		return
	}

	response := map[string]interface{}{
		"name":        toolName,
		"description": toolInfo.Description,
	}

	// Get detailed metadata from the MCP server
	if metadata := t.getToolMetadata(toolName); metadata != nil {
		response["metadata"] = metadata
	}

	// Try to get the tool registry directly for more detailed schema
	if t.mcpServer != nil {
		// Use a local type assertion instead of defining an interface

		if server, ok := t.mcpServer.(serverWithCapabilities); ok {
			if registry := server.GetToolRegistry(); registry != nil {
				// Get full tool schema including parameters and output
				if schema, err := registry.GetToolSchema(toolName); err == nil {
					response["schema"] = schema
				} else {
					t.logger.Error("Failed to get tool schema", "error", err, "tool", toolName)
				}
			}
		}
	}

	t.sendJSON(w, http.StatusOK, response)
}

// handleGetAllToolSchemas returns schemas for all registered tools.
// This endpoint provides a comprehensive view of all available tools with their schemas.
func (t *HTTPTransport) handleGetAllToolSchemas(w http.ResponseWriter, _ *http.Request) {
	t.toolsMutex.RLock()
	toolNames := make([]string, 0, len(t.tools))
	for name := range t.tools {
		toolNames = append(toolNames, name)
	}
	t.toolsMutex.RUnlock()

	schemas := make(map[string]interface{})

	// Get schemas from the tool registry
	if t.mcpServer != nil {
		// Try to get registry capability
		if server, ok := t.mcpServer.(serverWithCapabilities); ok {
			if registry := server.GetToolRegistry(); registry != nil {
				for _, toolName := range toolNames {
					if schema, err := registry.GetToolSchema(toolName); err == nil {
						// Ensure description is included from HTTP transport if missing
						if desc, ok := schema["description"].(string); !ok || desc == "" {
							t.toolsMutex.RLock()
							if info, exists := t.tools[toolName]; exists && info.Description != "" {
								schema["description"] = info.Description
							}
							t.toolsMutex.RUnlock()
						}
						schemas[toolName] = schema
					} else {
						t.logger.Error("Failed to get tool schema", "error", err, "tool", toolName)
						// Add basic info even if schema fails
						t.toolsMutex.RLock()
						if info, exists := t.tools[toolName]; exists {
							schemas[toolName] = map[string]interface{}{
								"name":        toolName,
								"description": info.Description,
								"error":       "Schema unavailable",
							}
						}
						t.toolsMutex.RUnlock()
					}
				}
			}
		}
	}

	// If no schemas were retrieved, return basic tool info
	if len(schemas) == 0 {
		t.toolsMutex.RLock()
		for name, info := range t.tools {
			schemas[name] = map[string]interface{}{
				"name":        name,
				"description": info.Description,
				"endpoint":    fmt.Sprintf("/api/v1/tools/%s", name),
			}
		}
		t.toolsMutex.RUnlock()
	}

	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"schemas": schemas,
		"count":   len(schemas),
	})
}

// handleListTools returns a list of all registered tools with their descriptions.
// This endpoint provides a complete listing of available tools with optional schema details.
func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	defer t.toolsMutex.RUnlock()

	t.logger.Debug("Listing tools", "tool_count", len(t.tools))

	// Check if detailed schema is requested
	includeSchema := r.URL.Query().Get("include_schema") == "true"

	tools := make([]ToolDescription, 0, len(t.tools))
	for name, info := range t.tools {
		toolDesc := ToolDescription{
			Name:        name,
			Description: info.Description,
		}

		// Always include parameters for test compatibility
		if t.mcpServer != nil {
			if metadata := t.getToolMetadata(name); metadata != nil {
				// Convert metadata to schema map for backward compatibility
				schemaMap := map[string]interface{}{
					"type":       "object",
					"properties": metadata.Parameters,
				}
				toolDesc.SetSchemaFromMap(schemaMap)

				// Include additional schema info if requested
				if includeSchema {
					toolDesc.Category = metadata.Category
					toolDesc.Version = metadata.Version
					if len(metadata.Examples) > 0 {
						toolDesc.SetExampleFromInterface(metadata.Examples[0].Input)
					}
				}
			} else {
				// Provide empty schema if metadata not available
				emptySchemaMap := map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []string{},
				}
				toolDesc.SetSchemaFromMap(emptySchemaMap)
			}
		} else {
			// Provide empty schema if server not available
			emptySchemaMap := map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			}
			toolDesc.SetSchemaFromMap(emptySchemaMap)
		}

		tools = append(tools, toolDesc)
	}

	response := ToolListResponse{
		Tools: tools,
		Count: len(tools),
	}
	t.sendJSON(w, http.StatusOK, response)
}

// handleExecuteTool executes a specific tool with the provided parameters.
// This endpoint is the main entry point for tool execution via HTTP.
func (t *HTTPTransport) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	t.toolsMutex.RLock()
	toolInfo, exists := t.tools[toolName]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Tool '%s' not found", toolName))
		return
	}

	// Use type-safe request parsing
	var executeRequest HTTPToolExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
		t.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Validate the request
	if validationErrors := ValidateToolExecuteRequest(&executeRequest); len(validationErrors) > 0 {
		t.sendJSON(w, http.StatusBadRequest, HTTPValidationResponse{
			Success: false,
			Errors:  validationErrors,
			Message: "Request validation failed",
		})
		return
	}

	// Sanitize parameters
	sanitizedParams := SanitizeParameters(executeRequest.Parameters)

	ctx := r.Context()
	result, err := toolInfo.Handler(ctx, sanitizedParams)
	if err != nil {
		// Create structured error response
		httpErr := &HTTPError{
			Code:    500,
			Message: fmt.Sprintf("Tool execution failed: %v", err),
			Type:    "execution_error",
		}

		response := HTTPToolExecuteResponse{
			Success:     false,
			Error:       httpErr,
			ExecutionID: uuid.New().String(),
			Timestamp:   time.Now(),
		}

		t.sendJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Create successful response
	response := HTTPToolExecuteResponse{
		Success:     true,
		Result:      result,
		ExecutionID: uuid.New().String(),
		Timestamp:   time.Now(),
	}

	t.sendJSON(w, http.StatusOK, response)
}

// handleHealth returns the health status of the HTTP transport.
// This endpoint provides basic health check information and system metrics.
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Since(t.startTime),
		Metrics: map[string]int64{
			"tools_registered": int64(len(t.tools)),
			"uptime_seconds":   int64(time.Since(t.startTime).Seconds()),
		},
	}
	t.sendJSON(w, http.StatusOK, response)
}

// handleStatus returns the current status of the HTTP transport.
// This endpoint provides detailed status information including runtime metrics.
func (t *HTTPTransport) handleStatus(w http.ResponseWriter, _ *http.Request) {
	t.toolsMutex.RLock()
	toolCount := len(t.tools)
	t.toolsMutex.RUnlock()

	response := HealthResponse{
		Status:    "running",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Since(t.startTime),
		Metrics: map[string]int64{
			"tools_registered": int64(toolCount),
			"uptime_seconds":   int64(time.Since(t.startTime).Seconds()),
		},
	}
	t.sendJSON(w, http.StatusOK, response)
}

// ============================================================================
// Tool Metadata Retrieval
// ============================================================================

// getToolMetadata retrieves tool metadata from the MCP server if available.
// This method attempts to get detailed schema information from the tool registry.
func (t *HTTPTransport) getToolMetadata(toolName string) *HTTPToolMetadata {
	if t.mcpServer == nil {
		return nil
	}

	// Try to get the tool registry which has full schema information
	if server, ok := t.mcpServer.(serverWithCapabilities); ok {
		if registry := server.GetToolRegistry(); registry != nil {
			if schema, err := registry.GetToolSchema(toolName); err == nil {
				// Convert to HTTPToolMetadata
				metadata := t.convertSchemaToHTTPMetadata(schema, toolName)
				return metadata
			}
		}
	}

	// Fallback: Try to get the server instance
	if server, ok := t.mcpServer.(serverWithCapabilities); ok {
		if orchestrator := server.GetToolOrchestrator(); orchestrator != nil {
			if metadata, err := orchestrator.GetToolMetadata(toolName); err == nil {
				// Convert to HTTPToolMetadata
				if coreMetadata, ok := metadata.(api.ToolMetadata); ok {
					converted := ConvertCoreMetadata(coreMetadata)
					return &converted
				}
				// If not core.ToolMetadata, try map conversion
				if metaMap, ok := metadata.(map[string]interface{}); ok {
					return t.convertMapToHTTPMetadata(metaMap, toolName)
				}
			}
		}
	}

	return nil
}

// convertSchemaToHTTPMetadata converts a schema map to HTTPToolMetadata.
// This method transforms raw schema data into structured metadata for HTTP responses.
func (t *HTTPTransport) convertSchemaToHTTPMetadata(schema map[string]interface{}, toolName string) *HTTPToolMetadata {
	metadata := &HTTPToolMetadata{
		Name: toolName,
	}

	// Extract basic fields with type safety
	if desc, ok := schema["description"].(string); ok && desc != "" {
		metadata.Description = desc
	} else {
		// Fall back to HTTP transport description if missing or empty
		t.toolsMutex.RLock()
		if info, exists := t.tools[toolName]; exists {
			metadata.Description = info.Description
		}
		t.toolsMutex.RUnlock()
	}

	if version, ok := schema["version"].(string); ok {
		metadata.Version = version
	}

	if category, ok := schema["category"].(string); ok {
		metadata.Category = category
	}

	// Convert dependencies
	if deps, ok := schema["dependencies"].([]interface{}); ok {
		metadata.Dependencies = make([]string, 0, len(deps))
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				metadata.Dependencies = append(metadata.Dependencies, depStr)
			}
		}
	}

	// Convert capabilities
	if caps, ok := schema["capabilities"].([]interface{}); ok {
		metadata.Capabilities = make([]string, 0, len(caps))
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				metadata.Capabilities = append(metadata.Capabilities, capStr)
			}
		}
	}

	// Convert requirements
	if reqs, ok := schema["requirements"].([]interface{}); ok {
		metadata.Requirements = make([]string, 0, len(reqs))
		for _, req := range reqs {
			if reqStr, ok := req.(string); ok {
				metadata.Requirements = append(metadata.Requirements, reqStr)
			}
		}
	}

	// Convert parameters schema
	if params, ok := schema["parameters"].(map[string]interface{}); ok {
		metadata.Parameters = t.convertToParameterSchema(params)
	}

	// Convert examples
	if examples, ok := schema["examples"].([]interface{}); ok {
		metadata.Examples = make([]HTTPToolExample, 0, len(examples))
		for _, ex := range examples {
			if exMap, ok := ex.(map[string]interface{}); ok {
				example := HTTPToolExample{}
				if name, ok := exMap["name"].(string); ok {
					example.Name = name
				}
				if desc, ok := exMap["description"].(string); ok {
					example.Description = desc
				}
				example.Input = exMap["input"]
				example.Output = exMap["output"]
				metadata.Examples = append(metadata.Examples, example)
			}
		}
	}

	return metadata
}

// convertMapToHTTPMetadata converts a generic map to HTTPToolMetadata.
// This method provides a fallback for metadata conversion when the exact structure is unknown.
func (t *HTTPTransport) convertMapToHTTPMetadata(metaMap map[string]interface{}, toolName string) *HTTPToolMetadata {
	// Reuse the schema conversion logic since maps have similar structure
	return t.convertSchemaToHTTPMetadata(metaMap, toolName)
}

// convertToParameterSchema converts a parameters map to HTTPToolParameterSchema.
// This method transforms parameter definitions into structured schema objects.
func (t *HTTPTransport) convertToParameterSchema(params map[string]interface{}) HTTPToolParameterSchema {
	schema := HTTPToolParameterSchema{
		Type:       "object",
		Properties: make(map[string]HTTPParameterProperty),
		Required:   []string{},
	}

	if props, ok := params["properties"].(map[string]interface{}); ok {
		for propName, propData := range props {
			if propMap, ok := propData.(map[string]interface{}); ok {
				prop := HTTPParameterProperty{}

				if propType, ok := propMap["type"].(string); ok {
					prop.Type = propType
				}
				if desc, ok := propMap["description"].(string); ok {
					prop.Description = desc
				}
				if def := propMap["default"]; def != nil {
					prop.Default = def
				}
				if req, ok := propMap["required"].(bool); ok {
					prop.Required = req
				}
				if format, ok := propMap["format"].(string); ok {
					prop.Format = format
				}
				if pattern, ok := propMap["pattern"].(string); ok {
					prop.Pattern = pattern
				}
				if minLen, ok := propMap["minLength"].(float64); ok {
					minLenInt := int(minLen)
					prop.MinLength = &minLenInt
				}
				if maxLen, ok := propMap["maxLength"].(float64); ok {
					maxLenInt := int(maxLen)
					prop.MaxLength = &maxLenInt
				}
				if minVal, ok := propMap["minimum"].(float64); ok {
					prop.Minimum = &minVal
				}
				if maxVal, ok := propMap["maximum"].(float64); ok {
					prop.Maximum = &maxVal
				}

				schema.Properties[propName] = prop
			}
		}
	}

	if required, ok := params["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	return schema
}
