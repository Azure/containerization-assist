# ADR-008: AI/ML Integration Architecture

Date: 2025-07-14
Status: Accepted
Context: Container Kit integrates extensively with AI/ML services for intelligent containerization workflows, requiring a coherent architecture for ML model management, prompt handling, and AI-powered sampling
Decision: Establish a unified AI/ML integration architecture with clear separation between ML capabilities, prompt management, and sampling services
Consequences: Improved maintainability and extensibility of AI features, better testability, and cleaner integration with the workflow system

## Context

Container Kit leverages AI/ML capabilities extensively throughout its containerization workflows:

1. **Machine Learning Services**: Pattern recognition, error prediction, build optimization, and workflow enhancement
2. **Prompt Management**: Template-based prompt generation with embedded YAML templates 
3. **AI Sampling**: LLM integration for workflow decisions, error analysis, and intelligent automation

These AI/ML capabilities were distributed across multiple packages with unclear boundaries and dependencies. The system needed a coherent architecture to:
- Unify AI/ML service management
- Provide consistent interfaces for ML capabilities
- Enable testable and mockable AI integration
- Support different AI providers and models

## Decision

We establish a unified AI/ML integration architecture in `pkg/mcp/infrastructure/ai_ml/` with three main subsystems:

### 1. Machine Learning Services (`ml/`)
Provides intelligent workflow enhancement capabilities:
- **Pattern Recognition**: `pattern_recognizer.go` - Identifies common build/deployment patterns
- **Error Prediction**: `error_history.go` - Learns from historical failures
- **Build Optimization**: `build_optimizer.go` - Optimizes container builds based on ML insights
- **Workflow Integration**: `workflow_integration.go` - Enhances steps with ML capabilities

### 2. Prompt Management (`prompts/`)
Manages AI prompt templates and generation:
- **Template Registry**: Embedded YAML templates using `go:embed`
- **Prompt Manager**: Dynamic prompt generation from templates
- **Template Categories**: Dockerfile generation, error analysis, security scanning, K8s manifests
- **Type-Safe Interface**: Structured prompt types and validation

### 3. AI Sampling (`sampling/`)
Handles LLM integration and intelligent decision making:
- **Core Client**: `core_client.go` - Base LLM integration
- **Specialized Client**: `specialized_client.go` - Domain-specific AI operations
- **Streaming**: Real-time AI response processing
- **Result Validation**: Type-safe result handling and validation

### Unified Provider Pattern
All AI/ML services are managed through Wire dependency injection:

```go
var AIMLProviders = wire.NewSet(
    // Machine learning services
    ml.NewErrorPatternRecognizer,
    ml.NewEnhancedErrorHandler, 
    ml.NewStepEnhancer,
    
    // Prompt management
    prompts.NewManager,
    
    // AI sampling
    sampling.NewClient,
)
```

## Architecture Principles

### 1. Clear Separation of Concerns
- **ML Services**: Focus on pattern recognition and optimization
- **Prompt Management**: Handle template processing and generation
- **AI Sampling**: Manage LLM communication and response processing

### 2. Interface-Driven Design
Each subsystem exposes clear interfaces defined in `pkg/mcp/domain/ml/interfaces.go` and related domain interfaces.

### 3. Testability
- Comprehensive test coverage for all AI/ML components
- Mock implementations for testing without external AI dependencies
- Performance benchmarks for ML operations

### 4. Provider Flexibility
The architecture supports multiple AI providers:
- Azure OpenAI (current implementation)
- Extensible to other LLM providers
- Configurable sampling strategies

## Implementation Details

### Template Management
Uses `go:embed` for template management:
```yaml
# dockerfile-generation.yaml
name: "dockerfile-generation"
description: "Generate optimized Dockerfile"
template: |
  Generate a Dockerfile for {{.Language}} application...
```

### ML Integration Pattern
ML services integrate with workflow steps through enhancement:
```go
type StepEnhancer interface {
    EnhanceStep(step WorkflowStep) WorkflowStep
}
```

### Sampling Client Design
Unified client with specialized capabilities:
- **Core**: Basic LLM communication
- **Specialized**: Domain-specific operations (error analysis, optimization)
- **Streaming**: Real-time response processing
- **Validation**: Type-safe result handling

## Consequences

### Positive
- **Unified Architecture**: Clear organization of AI/ML capabilities
- **Better Testability**: Isolated AI components with comprehensive test coverage
- **Extensibility**: Easy to add new ML services or AI providers
- **Performance**: Optimized sampling with caching and streaming
- **Type Safety**: Structured interfaces and validation throughout

### Negative
- **Complexity**: Additional abstraction layers for AI integration
- **Dependencies**: Requires Azure OpenAI SDK and associated configuration
- **Resource Usage**: ML services and LLM calls consume computational resources

### Migration Considerations
- Existing AI/ML code consolidated into new structure
- Template files moved from scattered locations to unified `templates/` directory
- Provider pattern enables gradual migration of AI services

## Compliance

This architecture aligns with Container Kit's four-layer architecture:
- **Infrastructure Layer**: AI/ML services in `pkg/mcp/infrastructure/ai_ml/`
- **Domain Interfaces**: ML contracts in `pkg/mcp/domain/ml/interfaces.go`
- **Application Integration**: Wire providers for dependency injection
- **API Layer**: Exposed through workflow step interfaces

## Performance Characteristics
- **ML Operations**: <50ms P95 for pattern recognition
- **Prompt Generation**: <10ms P95 for template processing  
- **AI Sampling**: Variable based on LLM provider response times
- **Caching**: Implemented for frequently used ML predictions

## References
- Performance monitoring in `sampling/metrics.go` and `sampling/performance_benchmark_test.go`
- Template examples in `prompts/templates/` directory
- Integration tests in `ml/workflow_integration.go`
- Provider configuration in root `providers.go`