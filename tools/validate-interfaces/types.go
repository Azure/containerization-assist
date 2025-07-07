package main

import "time"

// Expected unified interfaces that should exist after Team A's work
var expectedInterfaces = map[string][]string{
	"Tool": {
		"Execute(ctx context.Context, args interface{}) (interface{}, error)",
		"GetMetadata() ToolMetadata",
		"Validate(ctx context.Context, args interface{}) error",
	},
	"Session": {
		"ID() string",
		"GetWorkspace() string",
		"UpdateState(func(*SessionState))",
	},
	"Transport": {
		"Serve(ctx context.Context) error",
		"Stop() error",
	},
	"Orchestrator": {
		"ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)",
		"RegisterTool(name string, tool Tool) error",
	},
}

// Legacy interfaces that should be removed
var legacyInterfaces = []string{
	"pkg/mcp/internal/interfaces/",
	"pkg/mcp/internal/adapter/interfaces.go",
	"pkg/mcp/internal/tools/interfaces.go",
	"pkg/mcp/internal/tools/base/atomic_tool.go",
	"pkg/mcp/internal/dispatch/interfaces.go",
	"pkg/mcp/internal/analyzer/interfaces.go",
	"pkg/mcp/internal/ai_context/interfaces.go",
	"pkg/mcp/internal/fixing/interfaces.go",
	"pkg/mcp/internal/manifests/interfaces.go",
}

// ValidationResult represents a single validation finding
type ValidationResult struct {
	File      string
	Interface string
	Issue     string
	Severity  string
}

// InterfaceMetrics tracks interface usage and adoption patterns
type InterfaceMetrics struct {
	Timestamp          time.Time                       `json:"timestamp"`
	TotalInterfaces    int                             `json:"total_interfaces"`
	TotalImplementors  int                             `json:"total_implementors"`
	AdoptionRate       float64                         `json:"adoption_rate"`
	InterfaceStats     map[string]*InterfaceUsageStats `json:"interface_stats"`
	ImplementorStats   map[string]*ImplementorStats    `json:"implementor_stats"`
	PatternAnalysis    *InterfacePatternAnalysis       `json:"pattern_analysis"`
	ComplianceReport   *ComplianceReport               `json:"compliance_report"`
	RecommendationList []string                        `json:"recommendations"`
}

// InterfaceUsageStats tracks usage statistics for a specific interface
type InterfaceUsageStats struct {
	InterfaceName    string          `json:"interface_name"`
	ImplementorCount int             `json:"implementor_count"`
	Implementors     []string        `json:"implementors"`
	Methods          []string        `json:"methods"`
	PackageDistrib   map[string]int  `json:"package_distribution"`
	AdoptionTrend    []AdoptionPoint `json:"adoption_trend"`
	MostUsedMethods  []MethodUsage   `json:"most_used_methods"`
}

// ImplementorStats tracks statistics for types that implement interfaces
type ImplementorStats struct {
	TypeName            string   `json:"type_name"`
	Package             string   `json:"package"`
	InterfacesImpl      []string `json:"interfaces_implemented"`
	MethodCount         int      `json:"method_count"`
	InterfaceCompliance float64  `json:"interface_compliance"`
	PatternType         string   `json:"pattern_type"` // "unified", "legacy", "mixed"
}

// InterfacePatternAnalysis provides insights into interface usage patterns
type InterfacePatternAnalysis struct {
	UnifiedPatternUsage  int            `json:"unified_pattern_usage"`
	LegacyPatternUsage   int            `json:"legacy_pattern_usage"`
	MixedPatternUsage    int            `json:"mixed_pattern_usage"`
	PatternMigrationRate float64        `json:"pattern_migration_rate"`
	TopPatterns          []PatternUsage `json:"top_patterns"`
	AntiPatterns         []AntiPattern  `json:"anti_patterns"`
}

// ComplianceReport tracks compliance with interface standards
type ComplianceReport struct {
	OverallCompliance    float64            `json:"overall_compliance"`
	InterfaceCompliance  map[string]float64 `json:"interface_compliance"`
	MissingInterfaces    []string           `json:"missing_interfaces"`
	OrphanedImplementors []string           `json:"orphaned_implementors"`
	NonCompliantTools    []string           `json:"non_compliant_tools"`
}

// AdoptionPoint tracks adoption over time
type AdoptionPoint struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// MethodUsage tracks usage of specific interface methods
type MethodUsage struct {
	MethodName string `json:"method_name"`
	UsageCount int    `json:"usage_count"`
}

// PatternUsage tracks common patterns in interface usage
type PatternUsage struct {
	PatternName string   `json:"pattern_name"`
	Count       int      `json:"count"`
	Examples    []string `json:"examples"`
}

// AntiPattern identifies problematic interface usage patterns
type AntiPattern struct {
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Severity    string   `json:"severity"`
}
