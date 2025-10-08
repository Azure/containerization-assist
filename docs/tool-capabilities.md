# Tool Capabilities Reference

This document provides a comprehensive overview of all 21 tools in the containerization-assist project, their AI enhancement capabilities, and their integration with the sampling system.

## Tool Classification

### AI-Enhanced Tools (15 tools)

These tools use deterministic single-candidate sampling with quality scoring for intelligent content generation and analysis.

### Knowledge-Enhanced Planning Tools (2 tools)

These tools use knowledge packs for planning but do not invoke AI sampling.

### Utility Tools (4 tools)

These tools perform direct operations without AI enhancement.

---

## Complete Tool Reference

### 1. analyze-repo
**Category:** Analysis
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Repository analysis and framework detection
- AI-driven technology stack identification
- Containerization strategy recommendations
- Project structure assessment
- Monorepo and multi-module detection

**Enhancement Capabilities:**
- `content-generation`
- `analysis`
- `technology-detection`

**When to Use:** First step in containerization workflow to understand project requirements.

---

### 2. generate-dockerfile
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- AI-powered Dockerfile generation
- Multi-stage build optimization
- Security hardening patterns
- Self-repair with validation feedback

**Enhancement Capabilities:**
- `content-generation`
- `validation`
- `optimization`
- `self-repair`

**Knowledge Integration:**
- Docker security best practices
- Performance optimization patterns
- Base image recommendations
- Layer optimization strategies

**When to Use:** Generate optimized Dockerfiles from scratch or based on existing project analysis.

---

### 3. fix-dockerfile
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Dockerfile repair and enhancement
- Issue-specific fixes with explanations
- Security vulnerability remediation
- Performance optimization

**Enhancement Capabilities:**
- `dockerfile-repair`
- `security-hardening`
- `optimization`
- `validation`

**When to Use:** Repair and optimize existing Dockerfiles with validation issues.

---

### 4. build-image
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Docker image building with progress monitoring
- AI-driven build optimization suggestions
- Build context analysis
- Multi-platform build support

**Enhancement Capabilities:**
- `build-optimization`
- `context-analysis`
- `troubleshooting`

**When to Use:** Build Docker images with intelligent optimization suggestions.

---

### 5. tag-image
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Intelligent image tagging strategies
- Registry-aware tag recommendations
- Version management guidance
- Tag validation

**Enhancement Capabilities:**
- `tagging-strategy`
- `version-management`
- `registry-optimization`

**When to Use:** Apply intelligent tagging strategies before pushing to registries.

---

### 6. push-image
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Registry push optimization
- Authentication guidance
- Registry-specific optimizations
- Push strategy recommendations

**Enhancement Capabilities:**
- `push-optimization`
- `registry-insights`
- `troubleshooting`

**When to Use:** Push images to registries with optimization recommendations.

---

### 7. scan
**Category:** Security
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Security vulnerability scanning
- AI-powered security suggestions
- Risk assessment and prioritization
- Remediation recommendations

**Enhancement Capabilities:**
- `vulnerability-analysis`
- `security-suggestions`
- `risk-assessment`

**Security Focus:**
- CVE analysis and remediation
- Security best practices application
- Vulnerability prioritization
- Compliance checking

**When to Use:** Scan images and Dockerfiles for security vulnerabilities with AI-powered analysis.

---

### 8. generate-k8s-manifests
**Category:** Kubernetes
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Kubernetes manifest generation
- Resource optimization
- Security context configuration
- Health check implementation

**Enhancement Capabilities:**
- `manifest-generation`
- `resource-optimization`
- `security-configuration`
- `best-practices`

**When to Use:** Generate production-ready Kubernetes manifests from application requirements.

---

### 9. generate-helm-charts
**Category:** Kubernetes
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Helm chart generation and templating
- Values schema generation
- Template optimization
- Deployment strategy integration

**Enhancement Capabilities:**
- `chart-generation`
- `templating`
- `values-optimization`

**When to Use:** Create reusable Helm charts for application deployment.

---

### 10. generate-kustomize
**Category:** Kubernetes
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ❌ No
**Sampling Strategy:** `none`

**Capabilities:**
- Kustomize overlay generation
- Environment-specific configuration
- Resource patching strategies

**When to Use:** Generate Kustomize configurations for multi-environment deployments.

---

### 11. prepare-cluster
**Category:** Kubernetes
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Cluster preparation and validation
- AI-driven cluster optimization recommendations
- Resource allocation guidance
- Security configuration advice

**Enhancement Capabilities:**
- `cluster-optimization`
- `resource-planning`
- `security-hardening`

**When to Use:** Prepare and optimize Kubernetes clusters for deployment.

---

### 12. deploy
**Category:** Kubernetes
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Application deployment to Kubernetes
- Deployment analysis and troubleshooting
- Rollback strategy recommendations
- Resource monitoring guidance

**Enhancement Capabilities:**
- `deployment-analysis`
- `troubleshooting`
- `optimization`

**When to Use:** Deploy applications to Kubernetes with intelligent monitoring and troubleshooting.

---

### 13. verify-deployment
**Category:** Kubernetes
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Intelligent deployment verification
- Health check validation
- Performance analysis
- Issue diagnosis and resolution

**Enhancement Capabilities:**
- `health-validation`
- `performance-analysis`
- `issue-diagnosis`

**When to Use:** Verify deployment health and diagnose issues with AI-powered analysis.

---

### 14. resolve-base-images
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Security-focused base image selection
- Size and performance optimization
- Vulnerability assessment integration
- Base image recommendations

**Enhancement Capabilities:**
- `security-assessment`
- `optimization`
- `vulnerability-analysis`

**When to Use:** Select optimal base images based on security, performance, and compatibility requirements.

---

### 15. generate-aca-manifests
**Category:** Azure
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Azure Container Apps manifest generation
- Platform-specific optimizations
- Scaling configuration
- Azure service integration

**Enhancement Capabilities:**
- `aca-optimization`
- `azure-integration`
- `scaling-strategy`

**When to Use:** Generate Azure Container Apps configurations with platform-specific optimizations.

---

### 16. convert-aca-to-k8s
**Category:** Migration
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `single`

**Capabilities:**
- Azure Container Apps to Kubernetes conversion
- Resource mapping optimization
- Configuration translation
- Compatibility ensuring

**Enhancement Capabilities:**
- `resource-mapping`
- `configuration-translation`
- `compatibility-analysis`

**When to Use:** Migrate from Azure Container Apps to Kubernetes with intelligent resource mapping.

---

### 16. ops
**Category:** Operations
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ❌ No
**Sampling Strategy:** `none`

**Capabilities:**
- Operational utilities for Docker and Kubernetes
- Container inspection and management
- Resource queries
- System status checks

**When to Use:** Operational tasks and system queries.

---

### 17. inspect-session
**Category:** Debug
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ❌ No
**Sampling Strategy:** `none`

**Capabilities:**
- Session state inspection
- Tool execution history
- Debug information extraction
- Workflow analysis

**When to Use:** Debug and analyze tool execution sessions.

---

### 18. validate-dockerfile
**Category:** Docker
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ❌ No
**Sampling Strategy:** `none`

**Capabilities:**
- Dockerfile syntax validation
- Best practice checking
- Static analysis
- Linting integration

**When to Use:** Validate Dockerfile syntax and check for common issues.

---

### 19. plan-dockerfile-generation
**Category:** Planning
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `none`

**Capabilities:**
- Plan Dockerfile generation strategy
- Module selection for multi-module repos
- Build context analysis
- Knowledge-based planning

**When to Use:** Plan Dockerfile generation before actual generation, especially in multi-module repositories.

---

### 20. plan-manifest-generation
**Category:** Planning
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `none`

**Capabilities:**
- Plan Kubernetes manifest generation strategy
- Service mapping and dependencies
- Resource planning
- Knowledge-based planning

**When to Use:** Plan manifest generation before actual generation, especially for complex multi-service deployments.

---

### 21. generate-kustomize
**Category:** Kubernetes
**AI Enhanced:** ❌ No
**Knowledge Enhanced:** ❌ No
**Sampling Strategy:** `none`

**Capabilities:**
- Kustomize overlay generation
- Environment-specific configuration
- Resource patching strategies

**When to Use:** Generate Kustomize configurations for multi-environment deployments.

---

## AI Enhancement Patterns

### Sampling Strategy Types

1. **`single`** - Deterministic single-candidate sampling with quality scoring
   - Generates exactly one candidate per invocation
   - Scores result based on content quality, structure, and domain relevance
   - Provides scoring metadata for diagnostics and transparency
   - Used by 15 AI-enhanced tools for reproducible, debuggable outputs

2. **`none`** - No AI enhancement
   - Direct execution without AI sampling
   - Used by 6 utility and planning tools (ops, inspect-session, validate-dockerfile, plan-dockerfile-generation, plan-manifest-generation, generate-kustomize)

### Enhancement Capability Categories

#### Content Generation
- `content-generation` - AI-powered content creation
- `manifest-generation` - Kubernetes manifest creation
- `chart-generation` - Helm chart creation
- `dockerfile-repair` - Dockerfile fixing and optimization

#### Analysis and Intelligence
- `analysis` - General analysis capabilities
- `technology-detection` - Technology stack identification
- `vulnerability-analysis` - Security vulnerability assessment
- `performance-analysis` - Performance bottleneck identification
- `health-validation` - Deployment health checking

#### Optimization
- `optimization` - General performance and resource optimization
- `optimization-suggestions` - Build and deployment optimization
- `build-optimization` - Docker build process optimization
- `resource-optimization` - Kubernetes resource optimization
- `push-optimization` - Registry push optimization

#### Security
- `security-assessment` - Security evaluation and recommendations
- `security-hardening` - Security configuration improvements
- `security-suggestions` - AI-powered security advice
- `risk-assessment` - Risk evaluation and mitigation

#### Operations
- `troubleshooting` - Issue diagnosis and resolution
- `deployment-analysis` - Deployment process analysis
- `build-analysis` - Build process analysis
- `performance-insights` - Performance recommendations

## Tool Metadata Structure

All tools follow this metadata pattern:

```typescript
metadata: {
  aiDriven: boolean;                    // Uses AI sampling system
  knowledgeEnhanced: boolean;           // Uses knowledge enhancement
  samplingStrategy: 'single' | 'none'; // Sampling approach
  enhancementCapabilities?: string[];   // List of enhancement types (AI-enhanced only)
}
```

## Discovery and Introspection

### Finding AI-Enhanced Tools

```typescript
// Tools with AI enhancement (15 tools)
const aiTools = [
  'analyze-repo', 'generate-dockerfile', 'fix-dockerfile', 'build-image',
  'tag-image', 'push-image', 'scan', 'generate-k8s-manifests',
  'generate-helm-charts', 'prepare-cluster', 'deploy', 'verify-deployment',
  'resolve-base-images', 'generate-aca-manifests', 'convert-aca-to-k8s'
];

// Planning tools with knowledge enhancement (2 tools)
const planningTools = ['plan-dockerfile-generation', 'plan-manifest-generation'];

// Utility tools (4 tools)
const utilityTools = ['ops', 'inspect-session', 'validate-dockerfile', 'generate-kustomize'];
```

### Enhancement Capability Queries

```typescript
// Security-focused tools
const securityTools = tools.filter(t =>
  t.metadata.enhancementCapabilities.some(cap =>
    cap.includes('security') || cap.includes('vulnerability')
  )
);

// Content generation tools
const contentTools = tools.filter(t =>
  t.metadata.enhancementCapabilities.includes('content-generation')
);
```

## Integration Patterns

### Workflow Integration

1. **Analysis Phase**: `analyze-repo` → Understand project structure
2. **Generation Phase**: `generate-dockerfile`, `generate-k8s-manifests` → Create assets
3. **Validation Phase**: `scan`, `validate` → Security and quality checks
4. **Build Phase**: `build-image` → Create container images
5. **Deploy Phase**: `deploy`, `verify-deployment` → Production deployment
6. **Operations Phase**: `ops`, `inspect-session` → Ongoing management

### AI Enhancement Flow

1. **Content Analysis** - AI examines input content and context
2. **Knowledge Application** - Relevant best practices and patterns applied
3. **Single-Candidate Generation** - One candidate solution generated deterministically
4. **Quality Scoring** - Result scored based on multiple criteria for diagnostics
5. **Result Return** - Scored candidate returned with metadata
6. **Feedback Integration** - Results can be used to improve future enhancements

## Performance Characteristics

### AI-Enhanced Tools
- **Latency**: 2-10 seconds depending on content complexity
- **Token Usage**: 2,000-6,000 tokens per operation
- **Quality**: High consistency through sampling and scoring
- **Reliability**: Built-in error handling and fallback strategies

### Utility Tools
- **Latency**: < 1 second for most operations
- **Resource Usage**: Minimal CPU and memory
- **Reliability**: Direct execution with standard error handling

## Configuration and Customization

### Global Settings
```typescript
// Environment variables affecting AI behavior
AI_ENHANCEMENT_ENABLED=true
AI_ENHANCEMENT_CONFIDENCE=0.8
AI_ENHANCEMENT_MAX_SUGGESTIONS=5
```

### Tool-Specific Customization
```typescript
// Per-tool enhancement configuration
const customEnhancement = {
  confidenceThreshold: 0.8, // Minimum confidence for acceptance
  maxRetries: 3,            // Retry attempts on failure
};
```

This comprehensive reference provides complete visibility into the AI enhancement capabilities across all tools in the containerization-assist project.