# ADR-010: Code Generation Automation for Pipeline Boilerplate

## Status
**Accepted** - January 9, 2025

## Context

### Current State
Container Kit pipeline system currently requires significant boilerplate code for:

#### 1. Pipeline Implementation Boilerplate
```go
// Each pipeline type requires ~150 lines of boilerplate
type CustomPipeline struct {
    stages []PipelineStage
    timeout time.Duration
    retryPolicy RetryPolicy
    metrics MetricsCollector
    // ... additional fields
}

func NewCustomPipeline(stages ...PipelineStage) *CustomPipeline {
    return &CustomPipeline{
        stages: stages,
        timeout: 30 * time.Second,
        // ... default initialization
    }
}

func (p *CustomPipeline) Execute(ctx context.Context, request *PipelineRequest) (*PipelineResponse, error) {
    // ~100 lines of execution logic
    // Timeout handling
    // Error handling
    // Metrics collection
    // Stage execution
}

func (p *CustomPipeline) AddStage(stage PipelineStage) Pipeline {
    // Boilerplate stage management
}

func (p *CustomPipeline) WithTimeout(timeout time.Duration) Pipeline {
    // Boilerplate timeout configuration
}

func (p *CustomPipeline) WithRetry(policy RetryPolicy) Pipeline {
    // Boilerplate retry configuration
}

func (p *CustomPipeline) WithMetrics(collector MetricsCollector) Pipeline {
    // Boilerplate metrics configuration
}
```

#### 2. Stage Implementation Boilerplate
```go
// Each stage requires ~50 lines of boilerplate
type CustomStage struct {
    name   string
    config CustomStageConfig
    // ... configuration fields
}

func NewCustomStage(config CustomStageConfig) *CustomStage {
    return &CustomStage{
        name:   "custom",
        config: config,
    }
}

func (s *CustomStage) Name() string {
    return s.name
}

func (s *CustomStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
    // Stage-specific logic
}

func (s *CustomStage) Validate(input interface{}) error {
    // Validation logic
}
```

#### 3. Command Router Registration
```go
// Each command router requires ~80 lines of boilerplate
type CustomRouter struct {
    router api.CommandRouter
}

func NewCustomRouter() *CustomRouter {
    router := commands.NewRouter()
    
    // Manual registration of each command
    router.Register("command1", &Command1Handler{})
    router.Register("command2", &Command2Handler{})
    router.Register("command3", &Command3Handler{})
    // ... dozens of manual registrations
    
    return &CustomRouter{router: router}
}
```

### Problems with Current Approach

#### 1. Boilerplate Explosion
- **Pipeline Implementations**: ~150 lines per pipeline type
- **Stage Implementations**: ~50 lines per stage
- **Command Routers**: ~80 lines per router
- **Total Boilerplate**: ~280 lines per complete pipeline system

#### 2. Maintenance Burden
- **Consistency**: Hard to maintain consistent patterns across implementations
- **Updates**: Changes to common patterns require updates in multiple places
- **Testing**: Each boilerplate implementation needs separate tests

#### 3. Developer Experience
- **Copy-Paste Errors**: Developers copy existing implementations and miss updates
- **Time Overhead**: 2-3 hours to create new pipeline with stages and router
- **Knowledge Barrier**: Developers need to understand all boilerplate patterns

#### 4. Quality Issues
- **Inconsistent Error Handling**: Different implementations handle errors differently
- **Missing Features**: Developers may omit metrics, timeouts, or retry logic
- **Configuration Drift**: Similar configurations implemented differently

### Quantitative Analysis
```bash
# Current boilerplate analysis
$ find pkg/mcp/application -name "*.go" | xargs wc -l | grep -E "(pipeline|stage|router)" | awk '{sum += $1} END {print sum}'
# Result: ~4,500 lines of boilerplate across 12 pipeline implementations

# Duplication analysis
$ grep -r "func.*Execute.*context.Context" pkg/mcp/application/ | wc -l
# Result: 67 nearly identical Execute method implementations

# Time analysis
$ git log --oneline --grep="pipeline\|stage\|router" | wc -l
# Result: 143 commits related to boilerplate maintenance
```

## Decision

**We will implement a code generation system to automatically generate pipeline, stage, and router boilerplate** from simple template definitions.

### Code Generation Architecture

#### 1. Template-Driven Generation
```go
// tools/pipeline-generator/templates/pipeline.go.tmpl
package {{.Package}}

import (
    "context"
    "time"
    "pkg/mcp/application/api"
    "pkg/mcp/domain/errors"
)

// {{.Name}}Pipeline implements {{.Name}} pipeline with {{.Type}} semantics
type {{.Name}}Pipeline struct {
    stages []api.PipelineStage
    {{- if .HasTimeout}}
    timeout time.Duration
    {{- end}}
    {{- if .HasRetry}}
    retryPolicy api.RetryPolicy
    {{- end}}
    {{- if .HasMetrics}}
    metrics api.MetricsCollector
    {{- end}}
    {{- range .CustomFields}}
    {{.Name}} {{.Type}}
    {{- end}}
}

// New{{.Name}}Pipeline creates a new {{.Name}} pipeline
func New{{.Name}}Pipeline(
    {{- range .ConstructorParams}}
    {{.Name}} {{.Type}},
    {{- end}}
) *{{.Name}}Pipeline {
    return &{{.Name}}Pipeline{
        {{- range .ConstructorParams}}
        {{.Name}}: {{.Name}},
        {{- end}}
        {{- if .HasTimeout}}
        timeout: {{.DefaultTimeout}},
        {{- end}}
    }
}

// Execute runs the {{.Name}} pipeline with {{.Type}} semantics
func (p *{{.Name}}Pipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
    {{- if .HasTimeout}}
    if p.timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, p.timeout)
        defer cancel()
    }
    {{- end}}
    
    {{- if eq .Type "atomic"}}
    // Atomic execution logic
    {{template "atomic_execution" .}}
    {{- else if eq .Type "workflow"}}
    // Workflow execution logic
    {{template "workflow_execution" .}}
    {{- else if eq .Type "orchestration"}}
    // Orchestration execution logic
    {{template "orchestration_execution" .}}
    {{- end}}
}

{{- range .Methods}}
// {{.Name}} {{.Description}}
func (p *{{$.Name}}Pipeline) {{.Name}}({{.Params}}) {{.ReturnType}} {
    {{.Body}}
}
{{- end}}
```

#### 2. Configuration-Driven Generation
```yaml
# configs/pipelines/container_build_pipeline.yaml
name: "ContainerBuild"
package: "pipeline"
type: "orchestration"
description: "Pipeline for building container images"

features:
  timeout: true
  retry: true
  metrics: true
  
custom_fields:
  - name: "dockerClient"
    type: "docker.Client"
  - name: "buildConfig"
    type: "BuildConfig"

constructor_params:
  - name: "dockerClient"
    type: "docker.Client"
  - name: "buildConfig"
    type: "BuildConfig"

stages:
  - name: "validate"
    type: "validation"
    config:
      required_fields: ["dockerfile", "context"]
  - name: "build"
    type: "docker_build"
    config:
      cache_enabled: true
  - name: "push"
    type: "docker_push"
    config:
      registry: "default"

methods:
  - name: "SetDockerClient"
    description: "sets the Docker client"
    params: "client docker.Client"
    return_type: "Pipeline"
    body: |
      p.dockerClient = client
      return p
```

#### 3. Generator Implementation
```go
// tools/pipeline-generator/main.go
package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "text/template"
    "gopkg.in/yaml.v2"
)

type PipelineGenerator struct {
    ConfigPath   string
    TemplatePath string
    OutputPath   string
}

type PipelineConfig struct {
    Name        string `yaml:"name"`
    Package     string `yaml:"package"`
    Type        string `yaml:"type"`
    Description string `yaml:"description"`
    Features    struct {
        Timeout bool `yaml:"timeout"`
        Retry   bool `yaml:"retry"`
        Metrics bool `yaml:"metrics"`
    } `yaml:"features"`
    CustomFields      []Field  `yaml:"custom_fields"`
    ConstructorParams []Field  `yaml:"constructor_params"`
    Stages           []Stage  `yaml:"stages"`
    Methods          []Method `yaml:"methods"`
}

type Field struct {
    Name string `yaml:"name"`
    Type string `yaml:"type"`
}

type Stage struct {
    Name   string                 `yaml:"name"`
    Type   string                 `yaml:"type"`
    Config map[string]interface{} `yaml:"config"`
}

type Method struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Params      string `yaml:"params"`
    ReturnType  string `yaml:"return_type"`
    Body        string `yaml:"body"`
}

func (g *PipelineGenerator) Generate() error {
    // Load configuration
    config, err := g.loadConfig()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    // Parse template
    tmpl, err := template.ParseFiles(g.TemplatePath)
    if err != nil {
        return fmt.Errorf("failed to parse template: %w", err)
    }
    
    // Create output file
    output, err := os.Create(g.OutputPath)
    if err != nil {
        return fmt.Errorf("failed to create output: %w", err)
    }
    defer output.Close()
    
    // Generate code
    if err := tmpl.Execute(output, config); err != nil {
        return fmt.Errorf("failed to generate code: %w", err)
    }
    
    return nil
}

func (g *PipelineGenerator) loadConfig() (*PipelineConfig, error) {
    data, err := os.ReadFile(g.ConfigPath)
    if err != nil {
        return nil, err
    }
    
    var config PipelineConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

#### 4. Build System Integration
```makefile
# Add to Makefile
.PHONY: generate-pipelines
generate-pipelines:
    @echo "Generating pipeline code..."
    @for config in configs/pipelines/*.yaml; do \
        name=$$(basename $$config .yaml); \
        ./tools/pipeline-generator/pipeline-generator \
            -config $$config \
            -template tools/pipeline-generator/templates/pipeline.go.tmpl \
            -output pkg/mcp/application/pipeline/generated_$$name.go; \
    done
    @echo "✅ Pipeline code generated"

.PHONY: generate-stages
generate-stages:
    @echo "Generating stage code..."
    @for config in configs/stages/*.yaml; do \
        name=$$(basename $$config .yaml); \
        ./tools/pipeline-generator/pipeline-generator \
            -config $$config \
            -template tools/pipeline-generator/templates/stage.go.tmpl \
            -output pkg/mcp/application/pipeline/stages/generated_$$name.go; \
    done
    @echo "✅ Stage code generated"

.PHONY: generate-routers
generate-routers:
    @echo "Generating router code..."
    @for config in configs/routers/*.yaml; do \
        name=$$(basename $$config .yaml); \
        ./tools/pipeline-generator/pipeline-generator \
            -config $$config \
            -template tools/pipeline-generator/templates/router.go.tmpl \
            -output pkg/mcp/application/commands/generated_$$name.go; \
    done
    @echo "✅ Router code generated"

.PHONY: generate-all
generate-all: generate-pipelines generate-stages generate-routers

# Update build process
.PHONY: build
build: generate-all mcp

# Update clean process
.PHONY: clean
clean:
    rm -f container-kit-mcp
    rm -f bin/schemaGen
    find . -name "generated_*.go" -type f -delete
```

### Advanced Generation Features

#### 1. Stage Composition
```yaml
# configs/stages/composite_build_stage.yaml
name: "CompositeBuild"
type: "composite"
description: "Composite stage for complete build process"

sub_stages:
  - name: "validate_dockerfile"
    type: "validation"
    config:
      validator: "dockerfile"
  - name: "build_image"
    type: "docker_build"
    config:
      cache_enabled: true
  - name: "scan_image"
    type: "security_scan"
    config:
      scanner: "trivy"
  - name: "push_image"
    type: "docker_push"
    config:
      registry: "production"

execution_mode: "sequential"
failure_strategy: "fail_fast"
```

#### 2. Router Generation with Command Discovery
```yaml
# configs/routers/container_router.yaml
name: "Container"
description: "Router for container-related commands"

commands:
  - name: "build"
    handler: "ContainerBuildHandler"
    description: "Build container image"
    args_type: "BuildArgs"
  - name: "push"
    handler: "ContainerPushHandler"
    description: "Push container image"
    args_type: "PushArgs"
  - name: "scan"
    handler: "ContainerScanHandler"
    description: "Scan container for vulnerabilities"
    args_type: "ScanArgs"

middleware:
  - name: "authentication"
    type: "auth"
  - name: "logging"
    type: "request_logging"
  - name: "metrics"
    type: "metrics_collection"
```

#### 3. Validation Generation
```go
// Generated validation code
func (p *ContainerBuildPipeline) Validate(request *api.PipelineRequest) error {
    // Generated validation logic based on stage requirements
    if request.Input == nil {
        return errors.NewValidationError("input cannot be nil")
    }
    
    buildArgs, ok := request.Input.(*BuildArgs)
    if !ok {
        return errors.NewValidationError("input must be BuildArgs")
    }
    
    if buildArgs.Dockerfile == "" {
        return errors.NewValidationError("dockerfile path is required")
    }
    
    if buildArgs.Context == "" {
        return errors.NewValidationError("build context is required")
    }
    
    return nil
}
```

## Consequences

### Positive Outcomes

#### 1. Massive Boilerplate Reduction
- **Pipeline Boilerplate**: 150 lines → 20 lines of YAML configuration
- **Stage Boilerplate**: 50 lines → 10 lines of YAML configuration
- **Router Boilerplate**: 80 lines → 15 lines of YAML configuration
- **Total Reduction**: ~4,500 lines → ~900 lines (80% reduction)

#### 2. Consistency Enforcement
- **Standardized Patterns**: All generated code follows identical patterns
- **Error Handling**: Consistent error handling across all implementations
- **Configuration**: Uniform configuration approach
- **Testing**: Generated test scaffolding with consistent patterns

#### 3. Developer Productivity
- **Time Savings**: 2-3 hours → 15 minutes to create new pipeline system
- **Reduced Errors**: No copy-paste errors or missing boilerplate
- **Focus on Logic**: Developers focus on business logic, not boilerplate
- **Self-Documenting**: YAML configurations serve as documentation

#### 4. Maintainability
- **Single Source of Truth**: Template updates affect all generated code
- **Easy Updates**: Pattern improvements propagate automatically
- **Version Control**: Generated code changes tracked in git
- **Review Process**: Template changes reviewable before generation

### Negative Outcomes

#### 1. Build Complexity
- **Tool Dependency**: Requires pipeline-generator in build process
- **Generation Step**: Additional step in build process
- **Template Maintenance**: Templates need maintenance and updates
- **Debugging**: Generated code may be harder to debug

#### 2. Learning Curve
- **New Concepts**: Developers need to understand template system
- **YAML Configuration**: Need to learn configuration format
- **Generation Process**: Understanding when and how to regenerate
- **Template Language**: Go template syntax for advanced customization

#### 3. Flexibility Limitations
- **Template Constraints**: Generated code limited by template capabilities
- **Customization**: Complex customizations may require template changes
- **Edge Cases**: Unusual requirements may not fit template patterns

### Mitigation Strategies

#### 1. Build Integration
```bash
# Git hooks for automatic generation
#!/bin/bash
# pre-commit hook
if git diff --cached --name-only | grep -E "(configs/|templates/)" > /dev/null; then
    echo "Pipeline configs changed, regenerating code..."
    make generate-all
    git add pkg/mcp/application/pipeline/generated_*.go
fi
```

#### 2. Template Validation
```go
// Template validation in CI
func TestTemplateValidation(t *testing.T) {
    // Validate all templates compile
    // Validate all configs are valid
    // Validate generated code compiles
}
```

#### 3. Documentation and Training
```markdown
# Developer Guide: Pipeline Code Generation

## Creating a New Pipeline
1. Create YAML configuration in `configs/pipelines/`
2. Run `make generate-pipelines`
3. Implement custom logic in generated methods
4. Add tests for custom logic

## Template Development
1. Modify templates in `tools/pipeline-generator/templates/`
2. Test with existing configurations
3. Update documentation
4. Review template changes carefully
```

## Success Metrics

### Boilerplate Reduction
- **Target**: 80% reduction in boilerplate code
- **Measurement**: Lines of code before/after generation
- **Baseline**: ~4,500 lines → Target: ~900 lines

### Developer Productivity
- **Target**: 90% reduction in pipeline creation time
- **Measurement**: Time to create new pipeline system
- **Baseline**: 2-3 hours → Target: 15 minutes

### Code Quality
- **Target**: 100% consistent patterns across generated code
- **Measurement**: Pattern compliance checks
- **Target**: 0 boilerplate-related bugs

### Build Performance
- **Target**: <30 seconds additional build time for generation
- **Measurement**: Build time increase
- **Acceptable**: <10% total build time increase

## Implementation Timeline

### Week 4: Foundation (Days 11-13)
- [ ] Create basic template system
- [ ] Implement pipeline generator tool
- [ ] Create sample configurations
- [ ] Build system integration

### Week 5: Template Development (Days 14-16)
- [ ] Create pipeline templates
- [ ] Create stage templates
- [ ] Create router templates
- [ ] Validation and testing

### Week 6: Migration (Days 17-19)
- [ ] Migrate existing pipelines to generation
- [ ] Update build process
- [ ] Documentation and training
- [ ] Performance validation

## Alternatives Considered

### Option 1: Continue Manual Implementation
- **Pros**: No additional tooling, full control
- **Cons**: Continued boilerplate burden, inconsistency

### Option 2: Runtime Code Generation
- **Pros**: Dynamic generation, no build dependency
- **Cons**: Runtime overhead, complex debugging

### Option 3: Template-Based Generation (Chosen)
- **Pros**: Compile-time generation, consistent patterns
- **Cons**: Build complexity, learning curve

### Option 4: AST-Based Generation
- **Pros**: Maximum flexibility, type safety
- **Cons**: Complex implementation, high maintenance

## References

- [Go Code Generation](https://go.dev/blog/generate)
- [Template Programming](https://golang.org/pkg/text/template/)
- [YAML Configuration](https://yaml.org/spec/)
- [Build System Best Practices](https://peter.bourgon.org/go-best-practices-2016/)

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2025-01-09 | Claude | Initial ADR creation |

---

**Note**: This ADR represents a significant productivity improvement that will reduce boilerplate code by 80% while ensuring consistency and maintainability. The code generation approach balances developer productivity with build system complexity.