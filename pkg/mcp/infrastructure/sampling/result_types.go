// Package sampling provides structured result types for AI responses
package sampling

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ManifestFix represents a structured response for Kubernetes manifest fixes
type ManifestFix struct {
	OriginalIssues   []string         `json:"original_issues"`
	FixedManifest    string           `json:"fixed_manifest"`
	ChangesApplied   []Change         `json:"changes_applied"`
	ValidationStatus ValidationResult `json:"validation_status"`
	Recommendations  []string         `json:"recommendations"`
	Metadata         ResponseMetadata `json:"metadata"`
}

// DockerfileFix represents a structured response for Dockerfile fixes
type DockerfileFix struct {
	OriginalError    string           `json:"original_error"`
	FixedDockerfile  string           `json:"fixed_dockerfile"`
	ChangesApplied   []Change         `json:"changes_applied"`
	ValidationStatus ValidationResult `json:"validation_status"`
	OptimizationTips []string         `json:"optimization_tips"`
	Metadata         ResponseMetadata `json:"metadata"`
}

// SecurityAnalysis represents a structured response for security scan analysis
type SecurityAnalysis struct {
	CriticalIssues    []SecurityIssue  `json:"critical_issues"`
	Remediations      []Remediation    `json:"remediations"`
	RiskLevel         RiskLevel        `json:"risk_level"`
	AlternativeImages []string         `json:"alternative_images"`
	Recommendations   []string         `json:"recommendations"`
	Metadata          ResponseMetadata `json:"metadata"`
}

// RepositoryAnalysis represents enhanced repository analysis
type RepositoryAnalysis struct {
	Language        string           `json:"language"`
	Framework       string           `json:"framework"`
	BuildTools      []string         `json:"build_tools"`
	Dependencies    []Dependency     `json:"dependencies"`
	Services        []Service        `json:"services"`
	EntryPoints     []string         `json:"entry_points"`
	EnvironmentVars []EnvVar         `json:"environment_vars"`
	SuggestedPorts  []int            `json:"suggested_ports"`
	Confidence      float64          `json:"confidence"`
	Metadata        ResponseMetadata `json:"metadata"`
}

// Supporting types

// Change represents a modification made to content
type Change struct {
	Type        ChangeType `json:"type"`        // "added", "removed", "modified"
	Section     string     `json:"section"`     // which part was changed
	Description string     `json:"description"` // what was changed
	LineNumber  int        `json:"line_number,omitempty"`
	OldValue    string     `json:"old_value,omitempty"`
	NewValue    string     `json:"new_value,omitempty"`
}

// ValidationResult represents validation status
type ValidationResult struct {
	IsValid       bool     `json:"is_valid"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	SyntaxValid   bool     `json:"syntax_valid"`
	BestPractices bool     `json:"best_practices"`
}

// SecurityIssue represents a security vulnerability
type SecurityIssue struct {
	CVE         string   `json:"cve,omitempty"`
	Severity    Severity `json:"severity"`
	Component   string   `json:"component"`
	Description string   `json:"description"`
	FixVersion  string   `json:"fix_version,omitempty"`
}

// Remediation represents a security fix
type Remediation struct {
	IssueType string   `json:"issue_type"`
	Action    string   `json:"action"`
	Commands  []string `json:"commands,omitempty"`
	Priority  Priority `json:"priority"`
	Effort    Effort   `json:"effort"`
}

// Dependency represents a project dependency
type Dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Type     string `json:"type"` // "runtime", "build", "dev"
	Required bool   `json:"required"`
}

// Service represents an external service requirement
type Service struct {
	Name        string   `json:"name"` // "postgres", "redis", etc.
	Type        string   `json:"type"` // "database", "cache", "queue"
	DefaultPort int      `json:"default_port"`
	Required    bool     `json:"required"`
	ConfigVars  []string `json:"config_vars,omitempty"`
	DockerImage string   `json:"docker_image,omitempty"`
	HealthCheck string   `json:"health_check,omitempty"`
}

// EnvVar represents environment variable requirements
type EnvVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Type        string `json:"type"` // "string", "int", "bool", "url"
}

// ResponseMetadata contains metadata about the AI response
type ResponseMetadata struct {
	TemplateID     string        `json:"template_id"`
	GeneratedAt    time.Time     `json:"generated_at"`
	ProcessingTime time.Duration `json:"processing_time"`
	TokensUsed     int           `json:"tokens_used"`
	Temperature    float32       `json:"temperature"`
	ModelUsed      string        `json:"model_used,omitempty"`
	Confidence     float64       `json:"confidence"`
}

// Enums

type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeRemoved  ChangeType = "removed"
	ChangeTypeModified ChangeType = "modified"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type RiskLevel string

const (
	RiskLevelCritical RiskLevel = "critical"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMinimal  RiskLevel = "minimal"
)

type Priority string

const (
	PriorityImmediate Priority = "immediate"
	PriorityHigh      Priority = "high"
	PriorityMedium    Priority = "medium"
	PriorityLow       Priority = "low"
)

type Effort string

const (
	EffortMinimal   Effort = "minimal"   // < 1 hour
	EffortLow       Effort = "low"       // 1-4 hours
	EffortMedium    Effort = "medium"    // 1-2 days
	EffortHigh      Effort = "high"      // 3-5 days
	EffortExtensive Effort = "extensive" // > 1 week
)

// Parser interface for converting AI text responses to structured types
type ResultParser interface {
	ParseManifestFix(content string) (*ManifestFix, error)
	ParseDockerfileFix(content string) (*DockerfileFix, error)
	ParseSecurityAnalysis(content string) (*SecurityAnalysis, error)
	ParseRepositoryAnalysis(content string) (*RepositoryAnalysis, error)
}

// DefaultParser implements ResultParser with regex and heuristic parsing
type DefaultParser struct{}

// NewDefaultParser creates a new parser
func NewDefaultParser() *DefaultParser {
	return &DefaultParser{}
}

// ParseManifestFix parses AI response into structured ManifestFix
func (p *DefaultParser) ParseManifestFix(content string) (*ManifestFix, error) {
	result := &ManifestFix{
		OriginalIssues:  []string{},
		ChangesApplied:  []Change{},
		Recommendations: []string{},
		ValidationStatus: ValidationResult{
			IsValid:       true,
			SyntaxValid:   true,
			BestPractices: true,
		},
		Metadata: ResponseMetadata{
			GeneratedAt: time.Now(),
		},
	}

	lines := strings.Split(content, "\n")

	// Extract YAML manifest (assuming it's the largest code block)
	var manifestLines []string
	inCodeBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			manifestLines = append(manifestLines, line)
		}
	}

	if len(manifestLines) > 0 {
		result.FixedManifest = strings.Join(manifestLines, "\n")
	} else {
		// Fallback: entire content might be the manifest
		result.FixedManifest = strings.TrimSpace(content)
	}

	// Basic validation - check if it looks like YAML
	if strings.Contains(result.FixedManifest, "apiVersion:") &&
		strings.Contains(result.FixedManifest, "kind:") {
		result.ValidationStatus.SyntaxValid = true
	} else {
		result.ValidationStatus.SyntaxValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors,
			"Response does not appear to contain valid Kubernetes YAML")
	}

	return result, nil
}

// ParseDockerfileFix parses AI response into structured DockerfileFix
func (p *DefaultParser) ParseDockerfileFix(content string) (*DockerfileFix, error) {
	result := &DockerfileFix{
		ChangesApplied:   []Change{},
		OptimizationTips: []string{},
		ValidationStatus: ValidationResult{
			IsValid:       true,
			SyntaxValid:   true,
			BestPractices: true,
		},
		Metadata: ResponseMetadata{
			GeneratedAt: time.Now(),
		},
	}

	// Extract Dockerfile content (look for FROM instruction)
	lines := strings.Split(content, "\n")
	var dockerfileLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FROM ") ||
			strings.HasPrefix(line, "RUN ") ||
			strings.HasPrefix(line, "COPY ") ||
			strings.HasPrefix(line, "WORKDIR ") ||
			strings.HasPrefix(line, "EXPOSE ") ||
			strings.HasPrefix(line, "CMD ") ||
			strings.HasPrefix(line, "ENTRYPOINT ") {
			dockerfileLines = append(dockerfileLines, line)
		}
	}

	if len(dockerfileLines) > 0 {
		result.FixedDockerfile = strings.Join(dockerfileLines, "\n")
	} else {
		result.FixedDockerfile = strings.TrimSpace(content)
	}

	// Basic validation
	if strings.Contains(result.FixedDockerfile, "FROM ") {
		result.ValidationStatus.SyntaxValid = true
	} else {
		result.ValidationStatus.SyntaxValid = false
		result.ValidationStatus.Errors = append(result.ValidationStatus.Errors,
			"Response does not appear to contain valid Dockerfile content")
	}

	return result, nil
}

// ParseSecurityAnalysis parses AI response into structured SecurityAnalysis
func (p *DefaultParser) ParseSecurityAnalysis(content string) (*SecurityAnalysis, error) {
	result := &SecurityAnalysis{
		CriticalIssues:    []SecurityIssue{},
		Remediations:      []Remediation{},
		AlternativeImages: []string{},
		Recommendations:   []string{},
		RiskLevel:         RiskLevelMedium, // Default
		Metadata: ResponseMetadata{
			GeneratedAt: time.Now(),
		},
	}

	// Parse content for security information
	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect sections
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "critical") || strings.Contains(lineLower, "vulnerabilit") {
			currentSection = "issues"
		} else if strings.Contains(lineLower, "remediation") || strings.Contains(lineLower, "fix") {
			currentSection = "remediations"
		} else if strings.Contains(lineLower, "alternative") || strings.Contains(lineLower, "base image") {
			currentSection = "alternatives"
		}

		// Parse items
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			switch currentSection {
			case "remediations":
				result.Remediations = append(result.Remediations, Remediation{
					Action:   item,
					Priority: PriorityMedium,
					Effort:   EffortMedium,
				})
			case "alternatives":
				result.AlternativeImages = append(result.AlternativeImages, item)
			default:
				result.Recommendations = append(result.Recommendations, item)
			}
		}
	}

	// Determine risk level based on content
	contentLower := strings.ToLower(content)
	if strings.Contains(contentLower, "critical") {
		result.RiskLevel = RiskLevelCritical
	} else if strings.Contains(contentLower, "high") {
		result.RiskLevel = RiskLevelHigh
	}

	return result, nil
}

// ParseRepositoryAnalysis parses AI response into structured RepositoryAnalysis
func (p *DefaultParser) ParseRepositoryAnalysis(content string) (*RepositoryAnalysis, error) {
	result := &RepositoryAnalysis{
		BuildTools:      []string{},
		Dependencies:    []Dependency{},
		Services:        []Service{},
		EntryPoints:     []string{},
		EnvironmentVars: []EnvVar{},
		SuggestedPorts:  []int{},
		Confidence:      0.8, // Default confidence
		Metadata: ResponseMetadata{
			GeneratedAt: time.Now(),
		},
	}

	// Basic parsing - this would be enhanced with more sophisticated extraction
	contentLower := strings.ToLower(content)

	// Detect language
	if strings.Contains(contentLower, "java") {
		result.Language = "java"
	} else if strings.Contains(contentLower, "python") {
		result.Language = "python"
	} else if strings.Contains(contentLower, "javascript") || strings.Contains(contentLower, "node") {
		result.Language = "javascript"
	} else if strings.Contains(contentLower, "go") || strings.Contains(contentLower, "golang") {
		result.Language = "go"
	}

	// Detect framework
	if strings.Contains(contentLower, "spring") {
		result.Framework = "spring-boot"
	} else if strings.Contains(contentLower, "express") {
		result.Framework = "express"
	} else if strings.Contains(contentLower, "django") {
		result.Framework = "django"
	} else if strings.Contains(contentLower, "gin") {
		result.Framework = "gin"
	}

	// Extract common ports
	if strings.Contains(content, "8080") {
		result.SuggestedPorts = append(result.SuggestedPorts, 8080)
	}
	if strings.Contains(content, "3000") {
		result.SuggestedPorts = append(result.SuggestedPorts, 3000)
	}
	if strings.Contains(content, "5000") {
		result.SuggestedPorts = append(result.SuggestedPorts, 5000)
	}

	return result, nil
}

// JSON marshaling helpers

func (m ManifestFix) String() string {
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}

func (d DockerfileFix) String() string {
	data, _ := json.MarshalIndent(d, "", "  ")
	return string(data)
}

func (s SecurityAnalysis) String() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

func (r RepositoryAnalysis) String() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

// Validation methods

// Validate checks if the ManifestFix is valid
func (m *ManifestFix) Validate() error {
	if m.FixedManifest == "" {
		return fmt.Errorf("fixed manifest cannot be empty")
	}

	if !strings.Contains(m.FixedManifest, "apiVersion:") {
		return fmt.Errorf("fixed manifest must contain apiVersion")
	}

	if !strings.Contains(m.FixedManifest, "kind:") {
		return fmt.Errorf("fixed manifest must contain kind")
	}

	return nil
}

// Validate checks if the DockerfileFix is valid
func (d *DockerfileFix) Validate() error {
	if d.FixedDockerfile == "" {
		return fmt.Errorf("fixed dockerfile cannot be empty")
	}

	if !strings.Contains(d.FixedDockerfile, "FROM ") {
		return fmt.Errorf("dockerfile must contain FROM instruction")
	}

	return nil
}

// Validate checks if the SecurityAnalysis is valid
func (s *SecurityAnalysis) Validate() error {
	if len(s.Remediations) == 0 && len(s.Recommendations) == 0 {
		return fmt.Errorf("security analysis must contain remediations or recommendations")
	}

	return nil
}

// Validate checks if the RepositoryAnalysis is valid
func (r *RepositoryAnalysis) Validate() error {
	if r.Language == "" {
		return fmt.Errorf("repository analysis must identify language")
	}

	if r.Confidence < 0.0 || r.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0")
	}

	return nil
}
