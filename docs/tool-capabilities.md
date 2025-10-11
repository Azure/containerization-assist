# Tool Capabilities Reference

This document provides a comprehensive overview of all tools in the containerization-assist project, their AI enhancement capabilities, and their integration with the sampling system.

## Active Tools

**Currently Active:** 4 tools are enabled in this version (see `src/tools/index.ts` → `ALL_TOOLS`)

- `analyze-repo` (AI-Enhanced)
- `generate-dockerfile-plan` (Knowledge-Enhanced Planning)
- `generate-manifest-plan` (Knowledge-Enhanced Planning)
- `validate-dockerfile` (Utility)

**Status:** Remaining tools are in development and commented out in `ALL_TOOLS`.

## Tool Classification

### AI-Enhanced Tools

These tools use deterministic single-candidate sampling with quality scoring for intelligent content generation and analysis.

### Knowledge-Enhanced Planning Tools

These tools use knowledge packs for planning but do not invoke AI sampling.

### Utility Tools

These tools perform direct operations without AI enhancement.

---

## Complete Tool Reference

### analyze-repo
**Status:** ✅ **ACTIVE**
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

### generate-dockerfile
**Status:** 🚧 **IN DEVELOPMENT**
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

### fix-dockerfile
**Status:** 🚧 **IN DEVELOPMENT**
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

### build-image
**Status:** 🚧 **IN DEVELOPMENT**
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

### tag-image
**Status:** 🚧 **IN DEVELOPMENT**
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

### push-image
**Status:** 🚧 **IN DEVELOPMENT**
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

### scan
**Status:** 🚧 **IN DEVELOPMENT**
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

### generate-k8s-manifests
**Status:** 🚧 **IN DEVELOPMENT**
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

### generate-helm-charts
**Status:** 🚧 **IN DEVELOPMENT**
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

### generate-kustomize
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

### prepare-cluster
**Status:** 🚧 **IN DEVELOPMENT**
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

### deploy
**Status:** 🚧 **IN DEVELOPMENT**
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

### verify-deployment
**Status:** 🚧 **IN DEVELOPMENT**
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

### resolve-base-images
**Status:** 🚧 **IN DEVELOPMENT**
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

### generate-aca-manifests
**Status:** 🚧 **IN DEVELOPMENT**
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

### convert-aca-to-k8s
**Status:** 🚧 **IN DEVELOPMENT**
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

### ops
**Status:** 🚧 **IN DEVELOPMENT**
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

### inspect-session
**Status:** 🚧 **IN DEVELOPMENT**
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

### validate-dockerfile
**Status:** ✅ **ACTIVE**
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

### generate-dockerfile-plan
**Status:** ✅ **ACTIVE**
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

### generate-manifest-plan
**Status:** ✅ **ACTIVE**
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

### generate-kustomize
**Status:** 🚧 **IN DEVELOPMENT**
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
   - Used by AI-enhanced tools for reproducible, debuggable outputs

2. **`none`** - No AI enhancement
   - Direct execution without AI sampling
   - Used by utility and planning tools

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
// AI-enhanced tools with single-candidate sampling (only analyze-repo is currently active)
const aiTools = [
  'analyze-repo', // ✅ ACTIVE
  'generate-dockerfile', // 🚧 IN DEVELOPMENT
  'fix-dockerfile', // 🚧 IN DEVELOPMENT
  'build-image', // 🚧 IN DEVELOPMENT
  'tag-image', // 🚧 IN DEVELOPMENT
  'push-image', // 🚧 IN DEVELOPMENT
  'scan', // 🚧 IN DEVELOPMENT
  'generate-k8s-manifests', // 🚧 IN DEVELOPMENT
  'generate-helm-charts', // 🚧 IN DEVELOPMENT
  'prepare-cluster', // 🚧 IN DEVELOPMENT
  'deploy', // 🚧 IN DEVELOPMENT
  'verify-deployment', // 🚧 IN DEVELOPMENT
  'resolve-base-images', // 🚧 IN DEVELOPMENT
  'generate-aca-manifests', // 🚧 IN DEVELOPMENT
  'convert-aca-to-k8s' // 🚧 IN DEVELOPMENT
];

// Knowledge-enhanced planning tools (both currently active)
const planningTools = [
  'generate-dockerfile-plan', // ✅ ACTIVE
  'generate-manifest-plan' // ✅ ACTIVE
];

// Utility tools (only validate-dockerfile is currently active)
const utilityTools = [
  'validate-dockerfile', // ✅ ACTIVE
  'ops', // 🚧 IN DEVELOPMENT
  'inspect-session', // 🚧 IN DEVELOPMENT
  'generate-kustomize' // 🚧 IN DEVELOPMENT
];
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