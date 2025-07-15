// Package sampling provides AI/LLM sampling infrastructure
package sampling

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/prompts"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CoreClient implements the core sampling functionality without cross-cutting concerns
type CoreClient struct {
	logger *slog.Logger
}

// NewCoreClient creates a new core sampling client
func NewCoreClient(logger *slog.Logger) *CoreClient {
	return &CoreClient{
		logger: logger,
	}
}

// Sample performs a non-streaming sampling request
func (c *CoreClient) Sample(ctx context.Context, req sampling.Request) (sampling.Response, error) {
	// Convert domain request to infrastructure request
	infraReq := c.convertRequest(req)

	// Call the internal implementation
	resp, err := c.SampleInternal(ctx, infraReq)
	if err != nil {
		return sampling.Response{}, err
	}

	// Convert response
	return sampling.Response{
		Content:    resp.Content,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
		StopReason: resp.StopReason,
	}, nil
}

// Stream initiates a streaming sampling request
func (c *CoreClient) Stream(ctx context.Context, req sampling.Request) (<-chan sampling.StreamChunk, error) {
	// Convert domain request to infrastructure request
	infraReq := c.convertRequest(req)
	infraReq.Stream = true

	// Call the internal implementation
	infraCh, err := c.StreamInternal(ctx, infraReq)
	if err != nil {
		return nil, err
	}

	// Convert the channel
	ch := make(chan sampling.StreamChunk)
	go func() {
		defer close(ch)
		for chunk := range infraCh {
			ch <- sampling.StreamChunk{
				Text:        chunk.Text,
				TokensSoFar: chunk.TokensSoFar,
				Model:       chunk.Model,
				IsFinal:     chunk.IsFinal,
				Error:       chunk.Error,
			}
		}
	}()

	return ch, nil
}

// AnalyzeDockerfile analyzes a Dockerfile for issues
func (c *CoreClient) AnalyzeDockerfile(ctx context.Context, content string) (*sampling.DockerfileAnalysis, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data for dockerfile generation (simpler approach for analysis)
	templateData := prompts.TemplateData{
		"Language":  "unknown", // Will be determined from content analysis
		"Framework": "unknown", // Will be determined from content analysis
		"Port":      8080,      // Default port
	}

	// Use dockerfile-generation template as it has simpler parameter requirements
	rendered, err := templateManager.RenderTemplate("dockerfile-generation", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Create sampling request
	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	// Sample
	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze Dockerfile: %w", err)
	}

	// Parse response
	parser := NewDefaultParser()
	result, err := parser.ParseDockerfileAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	return result, nil
}

// AnalyzeKubernetesManifest analyzes Kubernetes manifests
func (c *CoreClient) AnalyzeKubernetesManifest(ctx context.Context, content string) (*sampling.ManifestAnalysis, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data for kubernetes manifest analysis
	templateData := prompts.TemplateData{
		"ManifestContent":   content,
		"DeploymentError":   "Analysis request",       // Placeholder for analysis
		"DockerfileContent": "Not available",          // Use default
		"RepoAnalysis":      "Analysis not available", // Use default
	}

	// Use kubernetes-manifest-fix template with proper parameters
	rendered, err := templateManager.RenderTemplate("kubernetes-manifest-fix", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Create sampling request
	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	// Sample
	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze manifest: %w", err)
	}

	// Parse response
	parser := NewDefaultParser()
	result, err := parser.ParseManifestAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	return result, nil
}

// AnalyzeSecurityScan analyzes security scan results
func (c *CoreClient) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*sampling.SecurityAnalysis, error) {
	// This is a simplified implementation - in production, you'd use a proper template
	request := SamplingRequest{
		Prompt:       fmt.Sprintf("Analyze these security scan results:\n%s", scanResults),
		MaxTokens:    1000,
		Temperature:  0.3,
		SystemPrompt: "You are a security expert analyzing vulnerability scan results.",
	}

	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze security scan: %w", err)
	}

	// Parse response
	parser := NewDefaultParser()
	infraResult, err := parser.ParseSecurityAnalysis(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse security analysis: %w", err)
	}

	// Convert to domain type
	result := &sampling.SecurityAnalysis{
		RiskLevel:       string(infraResult.RiskLevel),
		Vulnerabilities: make([]sampling.Vulnerability, 0),
		Recommendations: infraResult.Recommendations,
		Remediations:    make([]string, 0),
	}

	// Convert vulnerabilities
	for _, issue := range infraResult.CriticalIssues {
		vuln := sampling.Vulnerability{
			ID:          issue.CVE,
			Severity:    string(issue.Severity),
			Description: issue.Description,
			Package:     issue.Component,
			Version:     "",
			FixVersion:  issue.FixVersion,
		}
		result.Vulnerabilities = append(result.Vulnerabilities, vuln)
	}

	// Convert remediations
	for _, rem := range infraResult.Remediations {
		result.Remediations = append(result.Remediations, rem.Action)
	}

	return result, nil
}

// FixDockerfile attempts to fix issues in a Dockerfile
func (c *CoreClient) FixDockerfile(ctx context.Context, content string, issues []string) (*sampling.DockerfileFix, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Content": content,
		"Issues":  issues,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("fix-dockerfile", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Create sampling request
	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	// Sample
	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to fix Dockerfile: %w", err)
	}

	// Parse response
	parser := NewDefaultParser()
	infraResult, err := parser.ParseDockerfileFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse fix response: %w", err)
	}

	// Convert to domain type
	result := &sampling.DockerfileFix{
		OriginalContent: content,
		FixedContent:    infraResult.FixedDockerfile,
		Changes:         make([]string, 0),
		Explanation:     strings.Join(infraResult.OptimizationTips, "\n"),
		Metadata: sampling.FixMetadata{
			TemplateID:     rendered.ID,
			TokensUsed:     estimateTokens(response.Content),
			Temperature:    rendered.Temperature,
			ProcessingTime: infraResult.Metadata.ProcessingTime,
			Timestamp:      time.Now(),
		},
	}

	// Convert changes
	for _, change := range infraResult.ChangesApplied {
		result.Changes = append(result.Changes, change.Description)
	}

	return result, nil
}

// FixKubernetesManifest attempts to fix issues in Kubernetes manifests
func (c *CoreClient) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*sampling.ManifestFix, error) {
	// Get template manager
	templateManager, err := prompts.NewManager(c.logger, prompts.ManagerConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	// Prepare template data
	templateData := prompts.TemplateData{
		"Content": content,
		"Issues":  issues,
	}

	// Render template
	rendered, err := templateManager.RenderTemplate("fix-k8s-manifest", templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Create sampling request
	request := SamplingRequest{
		Prompt:       rendered.Content,
		MaxTokens:    rendered.MaxTokens,
		Temperature:  rendered.Temperature,
		SystemPrompt: rendered.SystemPrompt,
	}

	// Sample
	response, err := c.SampleInternal(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to fix manifest: %w", err)
	}

	// Parse response
	parser := NewDefaultParser()
	infraResult, err := parser.ParseManifestFix(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse fix response: %w", err)
	}

	// Convert to domain type
	result := &sampling.ManifestFix{
		OriginalContent: content,
		FixedContent:    infraResult.FixedManifest,
		Changes:         make([]string, 0),
		Explanation:     strings.Join(infraResult.Recommendations, "\n"),
		Metadata: sampling.FixMetadata{
			TemplateID:     rendered.ID,
			TokensUsed:     estimateTokens(response.Content),
			Temperature:    rendered.Temperature,
			ProcessingTime: infraResult.Metadata.ProcessingTime,
			Timestamp:      time.Now(),
		},
	}

	// Convert changes
	for _, change := range infraResult.ChangesApplied {
		result.Changes = append(result.Changes, change.Description)
	}

	return result, nil
}

// convertRequest converts domain request to infrastructure request
func (c *CoreClient) convertRequest(req sampling.Request) SamplingRequest {
	infraReq := SamplingRequest{
		Prompt:       req.Prompt,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		SystemPrompt: req.SystemPrompt,
		Stream:       req.Stream,
		Metadata:     req.Metadata,
	}

	// Add advanced params if present
	if req.Advanced != nil {
		infraReq.TopP = req.Advanced.TopP
		infraReq.FrequencyPenalty = req.Advanced.FrequencyPenalty
		infraReq.PresencePenalty = req.Advanced.PresencePenalty
		infraReq.StopSequences = req.Advanced.StopSequences
		infraReq.Seed = req.Advanced.Seed
		infraReq.LogitBias = req.Advanced.LogitBias
	}

	return infraReq
}

// SampleInternal performs the actual sampling (delegates to existing Client.SampleInternal)
func (c *CoreClient) SampleInternal(ctx context.Context, req SamplingRequest) (*SamplingResponse, error) {
	// Get MCP server from context
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		c.logger.Debug("No MCP server in context")
		return nil, fmt.Errorf("no MCP server in context")
	}

	// Convert to MCP request
	mcpReq := c.buildMCPRequest(req)

	// Call MCP sampling
	result, err := srv.RequestSampling(ctx, mcpReq)
	if err != nil {
		return nil, fmt.Errorf("MCP sampling failed: %w", err)
	}

	// Convert response
	return c.convertMCPResponse(result, req), nil
}

// StreamInternal performs streaming sampling (delegates to existing Client.StreamInternal)
func (c *CoreClient) StreamInternal(ctx context.Context, req SamplingRequest) (<-chan StreamChunk, error) {
	// Get MCP server from context
	srv := server.ServerFromContext(ctx)
	if srv == nil {
		c.logger.Debug("No MCP server in context")
		return nil, fmt.Errorf("no MCP server in context")
	}

	// For now, return a simple implementation
	ch := make(chan StreamChunk, 1)
	go func() {
		defer close(ch)

		// Simulate streaming by calling non-streaming version
		resp, err := c.SampleInternal(ctx, req)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}

		// Send as single chunk
		ch <- StreamChunk{
			Text:        resp.Content,
			TokensSoFar: resp.TokensUsed,
			Model:       resp.Model,
			IsFinal:     true,
		}
	}()

	return ch, nil
}

// buildMCPRequest builds an MCP sampling request
func (c *CoreClient) buildMCPRequest(req SamplingRequest) mcp.CreateMessageRequest {
	messages := []mcp.SamplingMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: req.Prompt,
			},
		},
	}

	if req.SystemPrompt != "" {
		messages = append([]mcp.SamplingMessage{
			{
				Role: mcp.RoleAssistant,
				Content: mcp.TextContent{
					Type: "text",
					Text: req.SystemPrompt,
				},
			},
		}, messages...)
	}

	return mcp.CreateMessageRequest{
		CreateMessageParams: mcp.CreateMessageParams{
			Messages:    messages,
			MaxTokens:   int(req.MaxTokens),
			Temperature: float64(req.Temperature),
			Metadata:    req.Metadata,
		},
	}
}

// convertMCPResponse converts MCP response to internal format
func (c *CoreClient) convertMCPResponse(result *mcp.CreateMessageResult, req SamplingRequest) *SamplingResponse {
	content := ""

	if result.Content != nil {
		// Try to extract as TextContent first
		if textContent, ok := result.Content.(mcp.TextContent); ok {
			content = textContent.Text
		} else if contentMap, ok := result.Content.(map[string]interface{}); ok {
			// Handle map[string]interface{} format with "text" key
			if textValue, exists := contentMap["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					content = textStr
				}
			}
		}
	}

	return &SamplingResponse{
		Content:    content,
		Model:      result.Model,
		TokensUsed: estimateTokens(content),
		StopReason: string(result.StopReason),
	}
}

// mockResponse returns a mock response for testing
func (c *CoreClient) mockResponse(req SamplingRequest) *SamplingResponse {
	return &SamplingResponse{
		Content:    "Mock response for: " + req.Prompt[:min(50, len(req.Prompt))],
		Model:      "mock-model",
		TokensUsed: 100,
		StopReason: "stop",
	}
}

// Ensure CoreClient implements UnifiedSampler
var _ sampling.UnifiedSampler = (*CoreClient)(nil)
