package types

import "time"

// RepositoryScanSummary summarizes repository analysis results
type RepositoryScanSummary struct {
	Repository       string            `json:"repository"`
	Language         string            `json:"language,omitempty"`
	Framework        string            `json:"framework,omitempty"`
	Files            []string          `json:"files,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	Metrics          AnalysisMetrics   `json:"metrics,omitempty"`
	Issues           []AnalysisIssue   `json:"issues,omitempty"`
	Suggestions      []string          `json:"suggestions,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	DatabaseFiles    []string          `json:"database_files,omitempty"`
	EntryPointsFound []string          `json:"entry_points_found,omitempty"`
}

// AnalysisMetrics contains code analysis metrics
type AnalysisMetrics struct {
	LinesOfCode      int `json:"lines_of_code"`
	FileCount        int `json:"file_count"`
	ComplexityScore  int `json:"complexity_score"`
	SecurityScore    int `json:"security_score"`
	MaintenanceScore int `json:"maintenance_score"`
}

// AnalysisIssue represents an issue found during analysis
type AnalysisIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// K8sManifest represents Kubernetes manifest data
type K8sManifest struct {
	Kind       string                 `json:"kind"`
	APIVersion string                 `json:"apiVersion"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	Status     map[string]interface{} `json:"status,omitempty"`
	Raw        []byte                 `json:"raw,omitempty"`
}

// SessionSnapshot represents a point-in-time snapshot of session state
type SessionSnapshot struct {
	SessionID  string                 `json:"session_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Stage      string                 `json:"stage"`
	Repository RepositoryScanSummary  `json:"repository,omitempty"`
	Images     []ImageReference       `json:"images,omitempty"`
	Manifests  map[string]K8sManifest `json:"manifests,omitempty"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
	Errors     []ToolError            `json:"errors,omitempty"`
}
