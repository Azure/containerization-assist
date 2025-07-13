package sampling

import (
	"context"
	"fmt"

	domain "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
)

// DomainAdapter wraps the infrastructure Client to implement domain interfaces
type DomainAdapter struct {
	client *Client
}

// NewDomainAdapter creates a new domain adapter wrapping the infrastructure client
func NewDomainAdapter(client *Client) *DomainAdapter {
	return &DomainAdapter{client: client}
}

// Ensure DomainAdapter implements domain interfaces
var (
	_ domain.Sampler         = (*DomainAdapter)(nil)
	_ domain.AnalysisSampler = (*DomainAdapter)(nil)
	_ domain.FixSampler      = (*DomainAdapter)(nil)
)

// Sample implements domain.Sampler
func (d *DomainAdapter) Sample(ctx context.Context, req domain.Request) (domain.Response, error) {
	// Convert domain request to infrastructure request
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

		// Also add to metadata for backward compatibility
		if infraReq.Metadata == nil {
			infraReq.Metadata = make(map[string]interface{})
		}
		if req.Advanced.TopP != nil {
			infraReq.Metadata["top_p"] = *req.Advanced.TopP
		}
		if req.Advanced.FrequencyPenalty != nil {
			infraReq.Metadata["frequency_penalty"] = *req.Advanced.FrequencyPenalty
		}
		if req.Advanced.PresencePenalty != nil {
			infraReq.Metadata["presence_penalty"] = *req.Advanced.PresencePenalty
		}
		if req.Advanced.StopSequences != nil {
			infraReq.Metadata["stop_sequences"] = req.Advanced.StopSequences
		}
		if req.Advanced.Seed != nil {
			infraReq.Metadata["seed"] = *req.Advanced.Seed
		}
		if req.Advanced.LogitBias != nil {
			infraReq.Metadata["logit_bias"] = req.Advanced.LogitBias
		}
	}

	// Call existing implementation
	resp, err := d.client.SampleInternal(ctx, infraReq)
	if err != nil {
		return domain.Response{}, err
	}

	// Convert to domain response
	return domain.Response{
		Content:    resp.Content,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
		StopReason: resp.StopReason,
	}, nil
}

// Stream implements domain.Sampler
func (d *DomainAdapter) Stream(ctx context.Context, req domain.Request) (<-chan domain.StreamChunk, error) {
	// Convert domain request to infrastructure request
	infraReq := SamplingRequest{
		Prompt:       req.Prompt,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		SystemPrompt: req.SystemPrompt,
		Stream:       true,
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

		// Also add to metadata for backward compatibility
		if infraReq.Metadata == nil {
			infraReq.Metadata = make(map[string]interface{})
		}
		if req.Advanced.TopP != nil {
			infraReq.Metadata["top_p"] = *req.Advanced.TopP
		}
		if req.Advanced.FrequencyPenalty != nil {
			infraReq.Metadata["frequency_penalty"] = *req.Advanced.FrequencyPenalty
		}
		if req.Advanced.PresencePenalty != nil {
			infraReq.Metadata["presence_penalty"] = *req.Advanced.PresencePenalty
		}
		if req.Advanced.StopSequences != nil {
			infraReq.Metadata["stop_sequences"] = req.Advanced.StopSequences
		}
		if req.Advanced.Seed != nil {
			infraReq.Metadata["seed"] = *req.Advanced.Seed
		}
		if req.Advanced.LogitBias != nil {
			infraReq.Metadata["logit_bias"] = req.Advanced.LogitBias
		}
	}

	// Call existing streaming implementation
	infraChan, err := d.client.SampleStream(ctx, infraReq)
	if err != nil {
		return nil, err
	}

	// Convert infrastructure chunks to domain chunks
	domainChan := make(chan domain.StreamChunk, 100)
	go func() {
		defer close(domainChan)
		for chunk := range infraChan {
			domainChan <- domain.StreamChunk{
				Text:        chunk.Text,
				TokensSoFar: chunk.TokensSoFar,
				Model:       chunk.Model,
				IsFinal:     chunk.IsFinal,
				Error:       chunk.Error,
			}
		}
	}()

	return domainChan, nil
}

// AnalyzeDockerfile implements domain.AnalysisSampler
func (d *DomainAdapter) AnalyzeDockerfile(ctx context.Context, content string) (*domain.DockerfileAnalysis, error) {
	// For now, return a basic analysis since there's no direct dockerfile analysis method
	// This would be expanded to use repository analysis or other methods
	return &domain.DockerfileAnalysis{
		Language:      "unknown",
		Framework:     "unknown",
		Port:          8080,
		BuildSteps:    []string{"FROM", "COPY", "RUN", "EXPOSE", "CMD"},
		Dependencies:  []string{},
		Issues:        []string{},
		Suggestions:   []string{},
		BaseImage:     "unknown",
		EstimatedSize: "unknown",
	}, nil
}

// AnalyzeKubernetesManifest implements domain.AnalysisSampler
func (d *DomainAdapter) AnalyzeKubernetesManifest(ctx context.Context, content string) (*domain.ManifestAnalysis, error) {
	// For now, return a basic analysis
	// This would be expanded to use the actual analysis methods
	return &domain.ManifestAnalysis{
		ResourceTypes: []string{"Deployment", "Service"},
		Issues:        []string{},
		Suggestions:   []string{},
		SecurityRisks: []string{},
		BestPractices: []string{},
	}, nil
}

// AnalyzeSecurityScan implements domain.AnalysisSampler
func (d *DomainAdapter) AnalyzeSecurityScan(ctx context.Context, scanResults string) (*domain.SecurityAnalysis, error) {
	result, err := d.client.AnalyzeSecurityScan(ctx, scanResults, "", false)
	if err != nil {
		return nil, err
	}

	vulns := make([]domain.Vulnerability, 0, len(result.CriticalIssues))
	for _, issue := range result.CriticalIssues {
		vulns = append(vulns, domain.Vulnerability{
			ID:          issue.CVE,
			Severity:    string(issue.Severity),
			Description: issue.Description,
			Package:     issue.Component,
			Version:     "", // Not available in SecurityIssue
			FixVersion:  issue.FixVersion,
		})
	}

	return &domain.SecurityAnalysis{
		RiskLevel:       string(result.RiskLevel),
		Vulnerabilities: vulns,
		Recommendations: result.Recommendations,
		Remediations:    extractRemediationDescriptions(result.Remediations),
	}, nil
}

// FixDockerfile implements domain.FixSampler
func (d *DomainAdapter) FixDockerfile(ctx context.Context, content string, issues []string) (*domain.DockerfileFix, error) {
	buildError := ""
	if len(issues) > 0 {
		for i, issue := range issues {
			if i > 0 {
				buildError += "\n"
			}
			buildError += issue
		}
	}

	result, err := d.client.FixDockerfile(ctx, "unknown", "unknown", 8080, content, buildError)
	if err != nil {
		return nil, err
	}

	return &domain.DockerfileFix{
		OriginalContent: content,
		FixedContent:    result.FixedDockerfile,
		Changes:         extractChangeDescriptions(result.ChangesApplied),
		Explanation:     extractExplanation(result.OptimizationTips),
		Metadata: domain.FixMetadata{
			TemplateID:     result.Metadata.TemplateID,
			TokensUsed:     result.Metadata.TokensUsed,
			Temperature:    result.Metadata.Temperature,
			ProcessingTime: result.Metadata.ProcessingTime,
			Timestamp:      result.Metadata.GeneratedAt,
		},
	}, nil
}

// FixKubernetesManifest implements domain.FixSampler
func (d *DomainAdapter) FixKubernetesManifest(ctx context.Context, content string, issues []string) (*domain.ManifestFix, error) {
	var deploymentError error
	if len(issues) > 0 {
		deploymentError = fmt.Errorf("issues: %v", issues)
	}

	result, err := d.client.analyzeKubernetesManifestInternal(ctx, content, deploymentError, "", "")
	if err != nil {
		return nil, err
	}

	return &domain.ManifestFix{
		OriginalContent: content,
		FixedContent:    result.FixedManifest,
		Changes:         extractChangeDescriptions(result.ChangesApplied),
		Explanation:     extractExplanation(result.Recommendations),
		Metadata: domain.FixMetadata{
			TemplateID:     result.Metadata.TemplateID,
			TokensUsed:     result.Metadata.TokensUsed,
			Temperature:    result.Metadata.Temperature,
			ProcessingTime: result.Metadata.ProcessingTime,
			Timestamp:      result.Metadata.GeneratedAt,
		},
	}, nil
}

// Helper functions to extract data from infrastructure types
func extractRemediationDescriptions(remediations []Remediation) []string {
	descriptions := make([]string, len(remediations))
	for i, r := range remediations {
		descriptions[i] = r.Action
	}
	return descriptions
}

func extractChangeDescriptions(changes []Change) []string {
	descriptions := make([]string, len(changes))
	for i, c := range changes {
		descriptions[i] = c.Description
	}
	return descriptions
}

func extractExplanation(tips []string) string {
	if len(tips) == 0 {
		return ""
	}
	explanation := ""
	for i, tip := range tips {
		if i > 0 {
			explanation += "\n"
		}
		explanation += tip
	}
	return explanation
}
