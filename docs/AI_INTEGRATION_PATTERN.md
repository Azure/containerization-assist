# AI Integration Pattern for Container Kit MCP Tools

> **Purpose**: This document defines the comprehensive pattern for AI integration in Container Kit MCP tools, including context structures, iterative fixing capabilities, and implementation guidelines.

## Table of Contents
- [Core Principles](#core-principles)
- [Design Pattern](#design-pattern)
- [Implementation Guidelines](#implementation-guidelines)
- [Iterative Fixing Integration](#iterative-fixing-integration)
- [Reference Implementations](#reference-implementations)
- [Common Patterns](#common-patterns)
- [Quality Guidelines](#quality-guidelines)

## 🎯 Core Principle

**Mechanical Operations + Rich Context = AI Success**

MCP tools should perform reliable, deterministic operations while providing comprehensive, structured context that enables AI to make intelligent decisions and provide actionable guidance.

## 📐 Design Pattern

### 1. **No Embedded AI Calls**
- Tools never make AI calls themselves
- Tools provide structured data for AI consumption
- AI assistants call tools and interpret the rich context

### 2. **Structured Data Over Free Text**
- Use typed Go structs, not unstructured strings
- JSON-serializable for easy AI parsing
- Clear field names and comprehensive documentation

### 3. **Multiple Options with Trade-offs**
- Provide 2-3 alternatives for any decision
- Include pros/cons for each option
- Specify complexity, time-to-value, and resource requirements

### 4. **Actionable Guidance**
- Include specific commands and steps
- Provide expected outcomes for verification
- Offer troubleshooting and diagnostic information

## 🛠️ Implementation Guidelines

### Tool Response Structure

Every tool response should include:

```go
type EnhancedToolResult struct {
    // Standard operational results
    Success bool `json:"success"`
    
    // Rich context for AI reasoning
    Context *DecisionContext `json:"context"`
    
    // Failure analysis (when operations fail)
    FailureAnalysis *FailureAnalysis `json:"failure_analysis,omitempty"`
    
    // Standard error information
    Error *types.RichError `json:"error,omitempty"`
}
```

### Context Structure Pattern

```go
type DecisionContext struct {
    // Assessment and analysis
    Assessment       *QualityAssessment     `json:"assessment"`
    
    // Multiple options with trade-offs
    Options          []OptionWithTradeoffs  `json:"options"`
    
    // Recommendations with rationale
    Recommendations  []Recommendation       `json:"recommendations"`
    
    // Next steps and guidance
    NextSteps        []ActionableStep       `json:"next_steps"`
    
    // Monitoring and observability
    Observability    *ObservabilityGuidance `json:"observability"`
}
```

## 📋 Reference Implementations

### 1. Repository Analysis (`analyze_repository`)

**Context Structure**: `ContainerizationAssessment`

```go
type ContainerizationAssessment struct {
    // Quantified readiness (0-100 scale)
    ReadinessScore     int      `json:"readiness_score"`
    
    // Identified strengths and challenges
    StrengthAreas      []string `json:"strength_areas"`
    ChallengeAreas     []string `json:"challenge_areas"`
    
    // Technology recommendations
    TechnologyStack    TechnologyRecommendations `json:"technology_stack"`
    
    // Deployment options with complexity ratings
    DeploymentOptions  []DeploymentOption `json:"deployment_options"`
    
    // Risk analysis with mitigation strategies
    RiskAnalysis       RiskAssessment `json:"risk_analysis"`
}
```

**AI Value**: Enables intelligent technology selection and risk-aware planning.

### 2. Image Building (`build_image`)

**Context Structure**: `BuildFailureAnalysis`

```go
type BuildFailureAnalysis struct {
    // Failure classification
    FailureType        string              `json:"failure_type"`
    FailureStage       string              `json:"failure_stage"`
    
    // Root cause analysis
    RootCauses         []string            `json:"root_causes"`
    
    // Immediate remediation steps
    RemediationSteps   []RemediationAction `json:"remediation_steps"`
    
    // Alternative build strategies
    AlternativeStrategies []BuildStrategy  `json:"alternative_strategies"`
    
    // Performance optimization guidance
    OptimizationTips   []OptimizationTip   `json:"optimization_tips"`
}
```

**AI Value**: Provides systematic build failure resolution and optimization guidance.

### 3. Manifest Generation (`generate_manifests`)

**Context Structure**: `DeploymentStrategyContext`

```go
type DeploymentStrategyContext struct {
    // Strategy analysis and recommendations
    RecommendedStrategy string                    `json:"recommended_strategy"`
    StrategyOptions     []DeploymentStrategyOption `json:"strategy_options"`
    
    // Resource sizing with rationale
    ResourceSizing      ResourceRecommendation    `json:"resource_sizing"`
    
    // Security posture assessment
    SecurityPosture     SecurityAssessment        `json:"security_posture"`
    
    // Environment-specific configurations
    EnvironmentProfiles []EnvironmentProfile      `json:"environment_profiles"`
    
    // Scaling analysis
    ScalingGuidance     ScalingRecommendation     `json:"scaling_guidance"`
}
```

**AI Value**: Enables intelligent deployment strategy selection and environment optimization.

### 4. Kubernetes Deployment (`deploy_kubernetes`)

**Context Structure**: `DeploymentFailureAnalysis`

```go
type DeploymentFailureAnalysis struct {
    // Failure classification and impact
    FailureType         string   `json:"failure_type"`
    ImpactSeverity      string   `json:"impact_severity"`
    RootCauses          []string `json:"root_causes"`
    
    // Immediate remediation
    ImmediateActions    []DeploymentRemediationAction `json:"immediate_actions"`
    
    // Alternative approaches
    AlternativeApproaches []DeploymentAlternative     `json:"alternative_approaches"`
    
    // Diagnostic and monitoring guidance
    DiagnosticCommands  []DiagnosticCommand         `json:"diagnostic_commands"`
    MonitoringSetup     MonitoringRecommendation    `json:"monitoring_setup"`
    
    // Rollback strategy
    RollbackStrategy    RollbackGuidance            `json:"rollback_strategy"`
    
    // Performance optimization
    PerformanceTuning   PerformanceOptimization     `json:"performance_tuning"`
}
```

**AI Value**: Comprehensive operational guidance for deployment failures and optimization.

## 🔧 Common Patterns

### Option Structure

```go
type OptionWithTradeoffs struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Pros         []string `json:"pros"`
    Cons         []string `json:"cons"`
    Complexity   string   `json:"complexity"`    // low, medium, high
    TimeToValue  string   `json:"time_to_value"` // immediate, short, medium, long
    ResourceReqs string   `json:"resource_reqs"` // Description of resources needed
}
```

### Recommendation Structure

```go
type Recommendation struct {
    Priority    int    `json:"priority"`     // 1 (highest) to 5 (lowest)
    Category    string `json:"category"`     // security, performance, reliability, cost
    Title       string `json:"title"`        // Brief recommendation
    Description string `json:"description"`  // Detailed explanation
    Action      string `json:"action"`       // Specific action to take
    Impact      string `json:"impact"`       // Expected improvement
    Effort      string `json:"effort"`       // Implementation complexity
}
```

### Actionable Step Structure

```go
type ActionableStep struct {
    Order       int    `json:"order"`        // Execution sequence
    Action      string `json:"action"`       // What to do
    Command     string `json:"command"`      // Executable command
    Description string `json:"description"`  // Why this step is needed
    Expected    string `json:"expected"`     // Expected outcome
    Validation  string `json:"validation"`   // How to verify success
}
```

## 🚦 Quality Guidelines

### Assessment Scoring (0-100 Scale)

- **0-30**: High risk, significant challenges
- **31-60**: Moderate risk, some challenges
- **61-80**: Good readiness, minor challenges
- **81-100**: Excellent readiness, minimal challenges

### Complexity Ratings

- **Low**: Can be completed quickly with minimal expertise
- **Medium**: Requires moderate expertise and time investment
- **High**: Requires deep expertise and significant time/resources

### Time-to-Value Categories

- **Immediate**: Results visible within minutes
- **Short**: Results visible within hours
- **Medium**: Results visible within days
- **Long**: Results visible within weeks

## 🎨 Usage Examples

### Basic Context Implementation

```go
func (t *MyTool) Execute(ctx context.Context, args MyToolArgs) (*MyToolResult, error) {
    result := &MyToolResult{
        Success: true,
        Context: &MyContext{},
    }
    
    // Perform mechanical operations
    operationResult, err := t.performOperation(args)
    if err != nil {
        result.Success = false
        result.FailureAnalysis = t.generateFailureAnalysis(err)
        return result, nil
    }
    
    // Generate rich context for AI reasoning
    result.Context = t.generateRichContext(operationResult, args)
    
    return result, nil
}
```

### Failure Analysis Generation

```go
func (t *MyTool) generateFailureAnalysis(err error) *FailureAnalysis {
    analysis := &FailureAnalysis{
        FailureType: t.classifyFailure(err),
        RootCauses:  t.identifyRootCauses(err),
    }
    
    // Add immediate actions based on failure type
    analysis.ImmediateActions = t.generateRemediationSteps(err)
    
    // Suggest alternative approaches
    analysis.Alternatives = t.suggestAlternatives(err)
    
    return analysis
}
```

## 📊 Implementation Checklist

### ✅ Context Structure Requirements

- [ ] Uses typed Go structs (not `map[string]interface{}`)
- [ ] Includes quantified assessments (scores, percentages, counts)
- [ ] Provides multiple options with clear trade-offs
- [ ] Contains actionable steps with specific commands
- [ ] Includes validation and verification guidance

### ✅ Failure Analysis Requirements

- [ ] Classifies failure type and severity
- [ ] Identifies root causes
- [ ] Provides immediate remediation steps
- [ ] Suggests alternative approaches
- [ ] Includes diagnostic commands

### ✅ Quality Standards

- [ ] All strings are meaningful (no placeholder text)
- [ ] Commands are valid and executable
- [ ] Options include realistic pros/cons
- [ ] Complexity ratings are accurate
- [ ] Time estimates are reasonable

## 🔄 Evolution Guidelines

### Adding New Context Fields

1. **Backward Compatibility**: New fields should be optional (`omitempty`)
2. **Documentation**: Update this document with new patterns
3. **Testing**: Include context validation in unit tests
4. **Examples**: Provide usage examples for new structures

### Refining Existing Context

1. **Data Quality**: Improve accuracy of assessments and recommendations
2. **Granularity**: Add more specific subcategories and options
3. **Actionability**: Enhance command accuracy and expected outcomes
4. **AI Feedback**: Incorporate feedback from AI assistant usage

## 📖 Related Documentation

- [MCP Server Documentation](../MCP_DOCUMENTATION.md) - Technical implementation details
- [Tool Development Guide](../CONTRIBUTING.md) - General development guidelines
- [Error Handling Patterns](../pkg/mcp/errors/README.md) - Error structure guidelines

## Iterative Fixing Integration

### Overview

Container Kit implements AI-driven iterative fixing capabilities that match the original pipeline's build→fail→analyze→fix→retry loops while leveraging the CallerAnalyzer for error analysis.

### Core Components

#### IterativeFixer Interface
- **Location**: `pkg/mcp/internal/fixing/interfaces.go`
- **Purpose**: Defines the contract for AI-driven fixing operations
- **Features**: Fix strategy generation, application, validation, and retry logic

#### DefaultIterativeFixer
- **Location**: `pkg/mcp/internal/fixing/iterative_fixer.go`
- **Implementation**: Uses CallerAnalyzer for AI-powered error analysis
- **Capabilities**: Retry loops with comprehensive context management

#### AtomicToolFixingMixin
- **Location**: `pkg/mcp/internal/fixing/atomic_tool_mixin.go`
- **Pattern**: Mixin for adding fixing to existing atomic tools
- **Usage**: Wraps operations as FixableOperations with configurable retry attempts

### Integration Pattern

```go
// Create fixing mixin
fixingMixin := fixing.NewAtomicToolFixingMixin(analyzer, "tool_name", logger)

// Wrap core operation
operation := fixing.NewOperationWrapper(
    func(ctx context.Context) error { /* core operation */ },
    func(ctx context.Context, err error) (*types.RichError, error) { /* analyze failure */ },
    func(ctx context.Context, fixAttempt *fixing.FixAttempt) error { /* apply fix */ },
    logger,
)

// Execute with retry
err := fixingMixin.ExecuteWithRetry(ctx, sessionID, baseDir, operation)
```

### Failure Routing Rules

1. **Dockerfile build failures** → Dockerfile generation tool
2. **Manifest deployment failures** → Manifest generation tool
3. **Image pull failures** → Image building tool
4. **Registry push failures** → Build tool for retry
5. **Critical security failures** → Rebuild with fixes

### Tool-Specific Configurations

- **atomic_build_image**: 3 attempts, routing enabled, Medium threshold
- **atomic_deploy_kubernetes**: 2 attempts, routing enabled, High threshold
- **generate_manifests_atomic**: 3 attempts, routing disabled, Medium threshold

### Error Categories by Tool

#### Build Tool Errors
- `DOCKERFILE_NOT_FOUND` → dockerfile_error
- `BASE_IMAGE_NOT_FOUND` → dependency_error
- `PACKAGE_INSTALL_FAILED` → dependency_error

#### Deploy Tool Errors
- `IMAGE_PULL_FAILED` → dependency_error
- `RESOURCE_QUOTA_EXCEEDED` → resource_error
- `MANIFEST_VALIDATION_FAILED` → manifest_error

#### Manifest Tool Errors
- `INVALID_IMAGE_REFERENCE` → validation_error
- `INVALID_PORT_CONFIGURATION` → validation_error
- `TEMPLATE_PROCESSING_FAILED` → template_error

---

**Last Updated**: 2025-06-22  
**Pattern Version**: 2.0  
**Tools Implementing Pattern**: analyze_repository, build_image, generate_manifests, deploy_kubernetes
**Fixing Integration**: atomic_build_image, atomic_deploy_kubernetes, generate_manifests_atomic