# MCP Interface Validation Tool

A comprehensive tool for validating MCP interface compliance and tracking interface adoption metrics across the codebase.

## Features

### Core Validation
- **Unified Interface Validation**: Checks for expected unified interfaces (Tool, Session, Transport, Orchestrator)
- **Legacy Interface Detection**: Identifies and reports legacy interface files that should be removed
- **Interface Conformance**: Validates that tool implementations conform to expected interface signatures
- **Duplicate Interface Detection**: Finds duplicate interface definitions across the codebase

### Metrics Tracking (NEW)
- **Interface Adoption Metrics**: Tracks which tools use each interface pattern
- **Implementation Statistics**: Counts implementations per interface
- **Pattern Analysis**: Analyzes usage patterns (unified vs legacy vs mixed)
- **Compliance Reporting**: Generates detailed compliance reports
- **Trend Analysis**: Tracks adoption trends over time
- **Anti-pattern Detection**: Identifies problematic interface usage patterns

## Usage

### Basic Validation
```bash
go run . --verbose
```

### Validation with Metrics Generation
```bash
go run . --metrics --metrics-output=interface_metrics.json --verbose
```

### Command Line Options
- `--verbose`: Enable verbose output showing file and interface details
- `--fix`: Attempt to fix violations automatically (not yet implemented)
- `--metrics`: Generate comprehensive interface adoption metrics
- `--metrics-output`: Specify output file for metrics report (default: interface_metrics.json)

## Metrics Report Structure

The generated JSON metrics report contains:

### Overview
- `total_interfaces`: Total number of interfaces found
- `total_implementors`: Total number of types implementing interfaces
- `adoption_rate`: Overall interface adoption percentage
- `timestamp`: When the report was generated

### Interface Statistics
For each interface:
- `implementor_count`: Number of implementations
- `implementors`: List of types implementing the interface
- `methods`: Interface method signatures
- `package_distribution`: Distribution across packages
- `most_used_methods`: Method usage statistics

### Implementor Statistics
For each implementing type:
- `type_name`: Name of the implementing type
- `package`: Package containing the type
- `interfaces_implemented`: List of interfaces implemented
- `interface_compliance`: Compliance percentage
- `pattern_type`: Pattern classification (unified/legacy/mixed/unknown)

### Pattern Analysis
- `unified_pattern_usage`: Count of types using unified patterns
- `legacy_pattern_usage`: Count of types using legacy patterns
- `mixed_pattern_usage`: Count of types using mixed patterns
- `pattern_migration_rate`: Percentage of unified pattern adoption
- `top_patterns`: Most common usage patterns
- `anti_patterns`: Detected problematic patterns

### Compliance Report
- `overall_compliance`: Overall compliance percentage
- `interface_compliance`: Per-interface compliance rates
- `missing_interfaces`: Interfaces with no implementations
- `orphaned_implementors`: Types implementing no interfaces
- `non_compliant_tools`: Tools with low compliance scores

### Recommendations
Actionable recommendations for improving interface adoption and compliance.

## Example Output

```
📊 Interface Adoption Metrics Summary
====================================
Total Interfaces: 80
Total Implementors: 36
Overall Adoption Rate: 100.0%
Overall Compliance: 33.3%

🎯 Pattern Analysis:
  Unified Pattern Usage: 36
  Legacy Pattern Usage: 0
  Mixed Pattern Usage: 0
  Migration Rate: 100.0%

📈 Top Interfaces by Implementation Count:
  Tool: 31 implementations
  Transport: 4 implementations
  Orchestrator: 1 implementations
  Session: 0 implementations

💡 Recommendations:
  • Overall interface compliance is below 70%. Focus on implementing missing interfaces.
  • Found 1 interfaces with no implementations. Consider creating implementations or removing unused interfaces.
```

## Integration with CI/CD

The tool can be integrated into CI/CD pipelines to:

1. **Gate Deployments**: Fail builds if interface compliance drops below thresholds
2. **Track Progress**: Monitor interface adoption trends over time
3. **Generate Reports**: Create regular compliance reports for stakeholders
4. **Detect Regressions**: Identify when new code introduces anti-patterns

### Example CI Integration
```bash
# Run validation and fail if errors found
go run ./tools/validate-interfaces

# Generate metrics for dashboard
go run ./tools/validate-interfaces --metrics --metrics-output=ci_metrics.json

# Check compliance threshold
compliance=$(jq '.compliance_report.overall_compliance' ci_metrics.json)
if (( $(echo "$compliance < 70.0" | bc -l) )); then
    echo "Interface compliance below threshold: $compliance%"
    exit 1
fi
```

## Expected Interfaces

The tool validates against these expected unified interfaces:

### Tool Interface
```go
type Tool interface {
    Execute(ctx context.Context, args interface{}) (interface{}, error)
    GetMetadata() ToolMetadata
    Validate(ctx context.Context, args interface{}) error
}
```

### Session Interface
```go
type Session interface {
    ID() string
    GetWorkspace() string
    UpdateState(func(*SessionState))
}
```

### Transport Interface
```go
type Transport interface {
    Serve(ctx context.Context) error
    Stop() error
}
```

### Orchestrator Interface
```go
type Orchestrator interface {
    ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)
    RegisterTool(name string, tool Tool) error
}
```

## Files Scanned

The tool scans the `pkg/mcp` directory for:
- Interface definitions
- Struct implementations
- Method signatures
- Package organization

## Legacy Interface Detection

The tool checks for these legacy interface files that should be removed:
- `pkg/mcp/internal/interfaces/`
- `pkg/mcp/internal/adapter/interfaces.go`
- `pkg/mcp/internal/tools/interfaces.go`
- `pkg/mcp/internal/tools/base/atomic_tool.go`
- `pkg/mcp/internal/dispatch/interfaces.go`
- `pkg/mcp/internal/analyzer/interfaces.go`
- `pkg/mcp/internal/ai_context/interfaces.go`
- `pkg/mcp/internal/fixing/interfaces.go`
- `pkg/mcp/internal/manifests/interfaces.go`

## Contributing

When adding new interfaces or implementations:
1. Follow the unified interface patterns
2. Run the validation tool to ensure compliance
3. Check metrics to understand adoption impact
4. Update expected interfaces if adding new core interfaces
