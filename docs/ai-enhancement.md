# AI Enhancement System

The containerization-assist project includes comprehensive AI enhancement capabilities that augment traditional validation and generation tools with intelligent insights, optimizations, and recommendations.

## Overview

The AI enhancement system provides:

- **Knowledge-Enhanced Content Generation**: AI-powered improvements to generated Dockerfiles, Kubernetes manifests, and other containerization assets
- **Intelligent Validation**: AI-driven analysis and suggestions for validation results
- **Context-Aware Optimizations**: Smart recommendations based on content type, environment, and best practices
- **Deterministic Sampling**: Single-candidate generation with quality scoring for reproducible, debuggable outputs

## Architecture

### Core Components

#### 1. Knowledge Enhancement Service (`src/mcp/ai/knowledge-enhancement.ts`)

The knowledge enhancement service provides AI-powered content improvement with:

```typescript
interface KnowledgeEnhancementRequest {
  content: string;
  context: 'dockerfile' | 'kubernetes' | 'security' | 'optimization';
  userQuery?: string;
  validationContext?: Array<{
    message: string;
    severity: 'error' | 'warning' | 'info';
    category: string;
  }>;
  targetImprovement?: 'security' | 'performance' | 'best-practices' | 'optimization' | 'all';
}
```

**Key Features:**
- Context-specific enhancement strategies
- Integration with validation results
- Comprehensive analysis with confidence scoring
- Performance metrics and token usage tracking

#### 2. AI Validation Enhancement (`src/validation/ai-enhancement.ts`)

Enhances validation results with AI-powered suggestions:

```typescript
interface EnhancementOptions {
  mode: 'suggestions' | 'fixes' | 'analysis';
  focus: 'security' | 'performance' | 'best-practices' | 'all';
  confidence: number;
  maxSuggestions?: number;
  includeExamples?: boolean;
}
```

**Capabilities:**
- Risk assessment and prioritization
- Technical debt identification
- Actionable improvement suggestions
- Complete fix generation (in 'fixes' mode)

#### 3. Sampling System (`src/mcp/ai/sampling-runner.ts`)

All AI enhancements use deterministic sampling for quality assurance:
- Single-candidate generation (`count: 1`) for deterministic, reproducible outputs
- Content-specific scoring functions for quality validation
- Scoring metadata captured for diagnostics and transparency
- Consistent API across all AI-enhanced tools

**Note on Determinism:**
As of the Phase A completion (Sprint 1), all AI-powered tools enforce single-candidate sampling to ensure deterministic behavior. This means each invocation produces exactly one result with associated scoring metadata, making outputs reproducible and debuggable in Copilot transcripts.

## Tool Enhancement Status

### AI-Enhanced Tools

Tools using deterministic single-candidate sampling for intelligent content generation:

- **`analyze-repo`** - Repository analysis and framework detection
  - AI-driven technology stack identification
  - Containerization strategy recommendations
  - Monorepo and multi-module detection

- **`generate-dockerfile`** - Dockerfile generation with validation
  - Multi-stage optimization
  - Security hardening patterns
  - Performance optimization
  - Self-repair capabilities

- **`fix-dockerfile`** - Dockerfile repair and enhancement
  - Issue-specific fixes with explanations
  - Best practice application
  - Security vulnerability remediation

- **`generate-k8s-manifests`** - Kubernetes manifest generation
  - Resource optimization
  - Security context configuration
  - Health check implementation

- **`generate-helm-charts`** - Helm chart generation
  - Template optimization
  - Value schema generation
  - Deployment strategy integration

- **`generate-aca-manifests`** - Azure Container Apps manifests
  - Platform-specific optimizations
  - Scaling configuration
  - Integration patterns

- **`convert-aca-to-k8s`** - Azure Container Apps to Kubernetes conversion
  - Resource mapping optimization
  - Configuration translation
  - Compatibility ensuring

- **`resolve-base-images`** - Base image recommendations
  - Security-focused selection
  - Size and performance optimization
  - Vulnerability assessment integration

- **`build-image`** - Docker image building
  - Build optimization suggestions
  - Context analysis
  - Progress monitoring

- **`tag-image`** - Image tagging strategies
  - Intelligent tag recommendations
  - Version management guidance

- **`push-image`** - Registry push optimization
  - Authentication guidance
  - Registry-specific optimizations

- **`scan`** - Security vulnerability scanning
  - AI-powered remediation recommendations
  - Risk assessment and prioritization

- **`prepare-cluster`** - Cluster preparation
  - Optimization recommendations
  - Resource planning guidance

- **`deploy`** - Application deployment
  - Deployment analysis and troubleshooting
  - Rollback strategy recommendations

- **`verify-deployment`** - Deployment verification
  - Health check validation
  - Performance analysis
  - Issue diagnosis

### Knowledge-Enhanced Planning Tools

Tools that use knowledge packs for planning without AI sampling:

- **`plan-dockerfile-generation`** - Dockerfile generation planning
  - Module selection strategy
  - Build context analysis
  
- **`plan-manifest-generation`** - Manifest generation planning
  - Service mapping planning
  - Resource planning

### Utility Tools

Tools without AI or knowledge enhancement:

- **`ops`** - Operational utilities for Docker and Kubernetes
- **`inspect-session`** - Session debugging and analysis
- **`validate-dockerfile`** - Dockerfile syntax validation
- **`generate-kustomize`** - Kustomize overlay generation

## Using AI Enhancements

### Prompt Engine and Knowledge Enhancement

The prompt engine (`src/ai/prompt-engine.ts`) integrates with the knowledge system to build AI-enhanced prompts:

```typescript
import { buildMessages } from '@/ai/prompt-engine';
import { getKnowledgeSnippets } from '@/knowledge/matcher';

// Build messages with knowledge enhancement
const messages = buildMessages({
  systemPrompt: 'You are an expert in containerization',
  userPrompt: 'Generate an optimized Dockerfile',
  context: { language: 'node', framework: 'express' },
});

// Or manually get knowledge snippets
const snippets = getKnowledgeSnippets({
  tool: 'generate-dockerfile',
  topic: 'dockerfile-generation',
  environment: 'production',
  language: 'node',
  maxChars: 2000,
});
```

### Tool-Level AI Integration

Tools integrate AI through the ToolContext:

```typescript
import type { Tool } from '@/types';

const tool: Tool<typeof schema, ResultType> = {
  name: 'my-tool',
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'optimization'],
  },
  run: async (input, ctx) => {
    // AI sampling happens through ctx
    // Knowledge enhancement happens via prompt engine
    // Results are automatically scored
    
    return { ok: true, value: result };
  },
};
```

## Enhancement Contexts

### Dockerfile Enhancement

**Focus Areas:**
- Security hardening (non-root users, minimal attack surface)
- Build performance optimization
- Layer caching strategies
- Base image selection
- Vulnerability remediation

**Knowledge Sources:**
- Docker security best practices
- Container security standards
- Performance optimization patterns
- Industry security benchmarks

### Kubernetes Enhancement

**Focus Areas:**
- RBAC and security contexts
- Resource optimization
- Health checks and probes
- Networking configuration
- Pod security standards

**Knowledge Sources:**
- Kubernetes security best practices
- CIS Kubernetes Benchmark
- Resource management guidelines
- Deployment strategy patterns

### Security Enhancement

**Focus Areas:**
- Vulnerability identification and remediation
- Defense-in-depth implementation
- Access control optimization
- Secrets management
- Network security

**Knowledge Sources:**
- OWASP container security guidelines
- CVE databases and security advisories
- Security hardening standards
- Penetration testing insights

### Optimization Enhancement

**Focus Areas:**
- Performance improvements
- Resource efficiency
- Cost optimization
- Scalability enhancements
- Reliability improvements

**Knowledge Sources:**
- Performance tuning guides
- Resource optimization patterns
- Monitoring best practices
- SLA and reliability engineering

## Quality Assurance

### Scoring Systems

All AI enhancements use specialized scoring functions:

- **Structure scoring**: Validates response format and completeness
- **Content quality**: Ensures actionable and relevant suggestions
- **Domain specificity**: Scores technical accuracy and context relevance
- **Confidence assessment**: Provides reliability metrics for AI outputs

### Session Helpers

The orchestrator provides centralized session management through helper utilities:

- **Automatic result storage**: `sessionFacade.storeResult(toolName, value)` automatically stores tool outputs
- **Result retrieval**: `sessionFacade.getResult(toolName)` fetches prior tool results
- **Metadata management**: Consistent top-level keys (`session.metadata`, `session.results`)
- **Workflow context**: Tools can access previous step results without manual session manipulation

Tools no longer need to manually call `ctx.session.set('results', ...)`. The orchestrator handles all session persistence automatically after successful tool execution.

### Performance Metrics

All AI enhancement operations include:
- Processing time tracking
- Token usage monitoring (when available)
- Model identification
- Confidence scoring
- Success/failure rates

## Configuration

### Environment Variables

AI enhancement behavior can be configured via environment variables:

```bash
# Enable/disable AI enhancements globally
AI_ENHANCEMENT_ENABLED=true

# Default confidence threshold
AI_ENHANCEMENT_CONFIDENCE=0.8

# Maximum suggestions per request
AI_ENHANCEMENT_MAX_SUGGESTIONS=5

# Default model preferences
AI_ENHANCEMENT_INTELLIGENCE_PRIORITY=0.9
AI_ENHANCEMENT_COST_PRIORITY=0.2
```

### Progress Notifications

Long-running AI operations surface progress updates via MCP notifications:

- **Build operations**: Progress reported during multi-stage Docker builds
- **Deploy operations**: Status updates during Kubernetes deployments
- **Scan operations**: Progress during vulnerability scanning

MCP clients can subscribe to `notifications/progress` messages to display real-time updates to users. The notification system gracefully handles errors without blocking tool execution.

### Tool-Specific Configuration

Tools can override default AI enhancement settings:

```typescript
const metadata = {
  aiDriven: true,
  knowledgeEnhanced: true,
  samplingStrategy: 'single' as const,
  enhancementCapabilities: ['content-generation', 'validation', 'optimization']
};
```

## Best Practices

### When to Use AI Enhancement

**Recommended for:**
- Content generation (Dockerfiles, manifests)
- Security analysis and hardening
- Performance optimization
- Best practice application
- Complex problem diagnosis

**Not recommended for:**
- Simple utility operations
- Direct file operations
- Basic status queries
- Time-sensitive operations requiring immediate results

### Performance Considerations

1. **Batch processing**: Group related enhancement requests when possible
2. **Confidence thresholds**: Use appropriate confidence levels to balance quality vs. speed
3. **Context size**: Limit content size for faster processing
4. **Caching**: Leverage result caching for repeated content

### Error Handling

All AI enhancement functions return `Result<T>` types:

```typescript
const result = await enhanceWithKnowledge(request, ctx);

if (result.ok) {
  // Use result.value
} else {
  // Handle result.error
  ctx.logger.warn(`AI enhancement failed: ${result.error}`);
}
```

## Debugging and Monitoring

### Logging

AI enhancement operations include comprehensive logging:

```typescript
ctx.logger.info({
  knowledgeAppliedCount: result.knowledgeApplied.length,
  confidence: result.confidence,
  enhancementAreas: result.analysis.enhancementAreas.length,
  processingTime,
}, 'Knowledge enhancement completed');
```

### Metrics Collection

Key metrics tracked:
- Enhancement success/failure rates
- Processing times by content type
- Confidence score distributions
- Token usage patterns
- Model performance comparisons

## Future Enhancements

Planned improvements include:
- Custom knowledge base integration
- User-specific learning and preferences
- Enhanced validation rule generation
- Multi-modal content analysis
- Real-time performance optimization suggestions