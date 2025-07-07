package tools

// Metadata contains tool information
type Metadata struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Category    string      `json:"category"`
	Schema      interface{} `json:"schema"`
}

// Category constants for tool organization
const (
	CategoryAnalyze = "analyze"
	CategoryBuild   = "build"
	CategoryDeploy  = "deploy"
	CategoryScan    = "scan"
	CategorySession = "session"
)
