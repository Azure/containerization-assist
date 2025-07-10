// Package analyze contains pure business entities and rules for repository analysis.
// This package has no external dependencies and represents the core analysis domain.
package analyze

import (
	"time"
)

// Repository represents a code repository to be analyzed
type Repository struct {
	Path      string                 `json:"path"`
	Name      string                 `json:"name"`
	Files     []File                 `json:"files"`
	Languages map[string]float64     `json:"languages"`
	Structure map[string]interface{} `json:"structure"`
	Metadata  RepositoryMetadata     `json:"metadata"`
}

// File represents a file within a repository
type File struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Content  string    `json:"content,omitempty"`
	Size     int64     `json:"size"`
	Language string    `json:"language,omitempty"`
	Type     FileType  `json:"type"`
	Modified time.Time `json:"modified,omitempty"`
}

// FileType represents the type of file
type FileType string

const (
	FileTypeSource        FileType = "source"
	FileTypeConfiguration FileType = "configuration"
	FileTypeDocumentation FileType = "documentation"
	FileTypeBuild         FileType = "build"
	FileTypeTest          FileType = "test"
	FileTypeData          FileType = "data"
	FileTypeUnknown       FileType = "unknown"
)

// RepositoryMetadata contains metadata about a repository
type RepositoryMetadata struct {
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Size       int64             `json:"size"`
	FileCount  int               `json:"file_count"`
	Branch     string            `json:"branch,omitempty"`
	Commit     string            `json:"commit,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// AnalysisResult represents the result of analyzing a repository
type AnalysisResult struct {
	Repository       Repository       `json:"repository"`
	Language         Language         `json:"language"`
	Framework        Framework        `json:"framework,omitempty"`
	Dependencies     []Dependency     `json:"dependencies"`
	Databases        []Database       `json:"databases"`
	Ports            []Port           `json:"ports,omitempty"`
	BuildTools       []BuildTool      `json:"build_tools"`
	TestFrameworks   []TestFramework  `json:"test_frameworks"`
	SecurityIssues   []SecurityIssue  `json:"security_issues,omitempty"`
	Recommendations  []Recommendation `json:"recommendations"`
	Confidence       ConfidenceLevel  `json:"confidence"`
	AnalysisMetadata AnalysisMetadata `json:"metadata"`
}

// Language represents a detected programming language
type Language struct {
	Name       string  `json:"name"`
	Version    string  `json:"version,omitempty"`
	Confidence float64 `json:"confidence"`
	Files      []File  `json:"files"`
	Percentage float64 `json:"percentage"`
}

// Framework represents a detected framework
type Framework struct {
	Name       string          `json:"name"`
	Version    string          `json:"version,omitempty"`
	Type       FrameworkType   `json:"type"`
	Confidence ConfidenceLevel `json:"confidence"`
	Evidence   []Evidence      `json:"evidence"`
}

// FrameworkType represents the type of framework
type FrameworkType string

const (
	FrameworkTypeWeb      FrameworkType = "web"
	FrameworkTypeAPI      FrameworkType = "api"
	FrameworkTypeCLI      FrameworkType = "cli"
	FrameworkTypeLibrary  FrameworkType = "library"
	FrameworkTypeTest     FrameworkType = "test"
	FrameworkTypeORM      FrameworkType = "orm"
	FrameworkTypeUnknown  FrameworkType = "unknown"
	FrameworkTypeNone     FrameworkType = "none"
	FrameworkTypeConfig   FrameworkType = "config"
	FrameworkTypeStandard FrameworkType = "standard"
	FrameworkTypeDesktop  FrameworkType = "desktop"
	FrameworkTypeRuntime  FrameworkType = "runtime"
	FrameworkTypeData     FrameworkType = "data"
	FrameworkTypeML       FrameworkType = "ml"
	FrameworkTypeMobile   FrameworkType = "mobile"
)

// Dependency represents a project dependency
type Dependency struct {
	Name     string         `json:"name"`
	Version  string         `json:"version,omitempty"`
	Type     DependencyType `json:"type"`
	Source   string         `json:"source"`
	Required bool           `json:"required"`
	Evidence []Evidence     `json:"evidence"`
}

// DependencyType represents the type of dependency
type DependencyType string

const (
	DependencyTypeDirect      DependencyType = "direct"
	DependencyTypeIndirect    DependencyType = "indirect"
	DependencyTypeDevelopment DependencyType = "development"
	DependencyTypeTest        DependencyType = "test"
	DependencyTypeOptional    DependencyType = "optional"
)

// Database represents a detected database
type Database struct {
	Type       DatabaseType    `json:"type"`
	Name       string          `json:"name"`
	Version    string          `json:"version,omitempty"`
	Host       string          `json:"host,omitempty"`
	Port       int             `json:"port,omitempty"`
	Confidence ConfidenceLevel `json:"confidence"`
	Evidence   []Evidence      `json:"evidence"`
}

// DatabaseType represents supported database types
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
	DatabaseTypeRedis      DatabaseType = "redis"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypeOracle     DatabaseType = "oracle"
	DatabaseTypeCassandra  DatabaseType = "cassandra"
	DatabaseTypeElastic    DatabaseType = "elasticsearch"
)

// BuildTool represents a detected build tool
type BuildTool struct {
	Name       string          `json:"name"`
	Type       BuildToolType   `json:"type"`
	Version    string          `json:"version,omitempty"`
	ConfigFile string          `json:"config_file,omitempty"`
	Confidence ConfidenceLevel `json:"confidence"`
	Evidence   []Evidence      `json:"evidence"`
}

// BuildToolType represents the type of build tool
type BuildToolType string

const (
	BuildToolTypeMake       BuildToolType = "make"
	BuildToolTypeGradle     BuildToolType = "gradle"
	BuildToolTypeMaven      BuildToolType = "maven"
	BuildToolTypeNPM        BuildToolType = "npm"
	BuildToolTypeYarn       BuildToolType = "yarn"
	BuildToolTypePip        BuildToolType = "pip"
	BuildToolTypeComposer   BuildToolType = "composer"
	BuildToolTypeGoMod      BuildToolType = "go_mod"
	BuildToolTypeCargo      BuildToolType = "cargo"
	BuildToolTypeDockerfile BuildToolType = "dockerfile"
)

// TestFramework represents a detected testing framework
type TestFramework struct {
	Name       string          `json:"name"`
	Type       TestType        `json:"type"`
	Confidence ConfidenceLevel `json:"confidence"`
	Evidence   []Evidence      `json:"evidence"`
}

// TestType represents the type of testing framework
type TestType string

const (
	TestTypeUnit        TestType = "unit"
	TestTypeIntegration TestType = "integration"
	TestTypeEnd2End     TestType = "e2e"
	TestTypePerformance TestType = "performance"
	TestTypeSecurity    TestType = "security"
)

// SecurityIssue represents a detected security issue
type SecurityIssue struct {
	ID          string        `json:"id"`
	Type        SecurityType  `json:"type"`
	Severity    SeverityLevel `json:"severity"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	File        string        `json:"file,omitempty"`
	Line        int           `json:"line,omitempty"`
	Evidence    []Evidence    `json:"evidence"`
	Fix         string        `json:"fix,omitempty"`
}

// SecurityType represents the type of security issue
type SecurityType string

const (
	SecurityTypeVulnerability SecurityType = "vulnerability"
	SecurityTypeCredential    SecurityType = "credential"
	SecurityTypeSecret        SecurityType = "secret"
	SecurityTypePermission    SecurityType = "permission"
	SecurityTypeEncryption    SecurityType = "encryption"
)

// Alternative naming for backward compatibility
const (
	SecurityIssueTypeSecret        = SecurityTypeSecret
	SecurityIssueTypeVulnerability = SecurityTypeVulnerability
	SecurityIssueTypeCompliance    = SecurityTypePermission // Using permission as closest match
)

// Recommendation represents an analysis recommendation
type Recommendation struct {
	Type        RecommendationType `json:"type"`
	Priority    PriorityLevel      `json:"priority"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Action      string             `json:"action"`
	Benefits    []string           `json:"benefits,omitempty"`
	Evidence    []Evidence         `json:"evidence"`
}

// RecommendationType represents the type of recommendation
type RecommendationType string

const (
	RecommendationTypeSecurity      RecommendationType = "security"
	RecommendationTypePerformance   RecommendationType = "performance"
	RecommendationTypeMaintenance   RecommendationType = "maintenance"
	RecommendationTypeArchitecture  RecommendationType = "architecture"
	RecommendationTypeTesting       RecommendationType = "testing"
	RecommendationTypeDocumentation RecommendationType = "documentation"
)

// Evidence represents evidence supporting an analysis conclusion
type Evidence struct {
	Type        EvidenceType `json:"type"`
	Source      string       `json:"source"`
	Description string       `json:"description"`
	File        string       `json:"file,omitempty"`
	Line        int          `json:"line,omitempty"`
	Content     string       `json:"content,omitempty"`
	Confidence  float64      `json:"confidence"`
}

// EvidenceType represents the type of evidence
type EvidenceType string

const (
	EvidenceTypeFile          EvidenceType = "file"
	EvidenceTypeContent       EvidenceType = "content"
	EvidenceTypePattern       EvidenceType = "pattern"
	EvidenceTypeDependency    EvidenceType = "dependency"
	EvidenceTypeConfiguration EvidenceType = "configuration"
	EvidenceTypeStructure     EvidenceType = "structure"
)

// AnalysisMetadata contains metadata about the analysis process
type AnalysisMetadata struct {
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	EnginesUsed   []string      `json:"engines_used"`
	FilesAnalyzed int           `json:"files_analyzed"`
	ErrorCount    int           `json:"error_count,omitempty"`
	Warnings      []string      `json:"warnings,omitempty"`
	Options       interface{}   `json:"options,omitempty"`
}

// Confidence and priority levels
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"
	ConfidenceMedium ConfidenceLevel = "medium"
	ConfidenceLow    ConfidenceLevel = "low"
)

type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical"
	SeverityHigh     SeverityLevel = "high"
	SeverityMedium   SeverityLevel = "medium"
	SeverityLow      SeverityLevel = "low"
	SeverityInfo     SeverityLevel = "info"
)

type PriorityLevel string

const (
	PriorityHigh   PriorityLevel = "high"
	PriorityMedium PriorityLevel = "medium"
	PriorityLow    PriorityLevel = "low"
)

// Port represents a detected port
type Port struct {
	Number   int        `json:"number"`
	Type     string     `json:"type"`
	Protocol string     `json:"protocol,omitempty"`
	Sources  []string   `json:"sources"`
	Evidence []Evidence `json:"evidence,omitempty"`
	Usage    string     `json:"usage,omitempty"`
}
