# Tool Capabilities Reference

This document provides a comprehensive overview of all 17 tools in the containerization-assist project, their AI enhancement capabilities, and their integration with the sampling system.

## Tool Classification

### AI-Enhanced Tools (14 tools)

These tools use the `sampleWithRerank` sampling system for intelligent content generation and analysis.

### Utility Tools (3 tools)

These tools perform direct operations without AI enhancement.

---

## Complete Tool Reference

### 1. analyze-repo
**Category:** Analysis
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `rerank`

**Capabilities:**
- Repository analysis and framework detection
- AI-driven technology stack identification
- Containerization strategy recommendations
- Project structure assessment

**Enhancement Capabilities:**
- `framework-detection`
- `containerization-strategy`
- `dependency-analysis`

**When to Use:** First step in containerization workflow to understand project requirements.

---

### 2. generate-dockerfile
**Category:** Docker
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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
**Sampling Strategy:** `rerank`

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

### 17. ops
**Category:** Operations
**AI Enhanced:** ✅ Yes
**Knowledge Enhanced:** ✅ Yes
**Sampling Strategy:** `rerank`

**Capabilities:**
- Operational utilities and insights
- System health monitoring
- Performance optimization recommendations
- Troubleshooting assistance

**Enhancement Capabilities:**
- `operational-insights`
- `performance-analysis`
- `troubleshooting`

**When to Use:** Operational tasks and system optimization.

---

### 18. inspect-session
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

## AI Enhancement Patterns

### Sampling Strategy Types

1. **`rerank`** - Uses N-best sampling with quality scoring
   - Generates multiple candidates
   - Scores based on content quality, structure, and domain relevance
   - Returns the highest-scoring result
   - Used by 14 tools for content generation and analysis

2. **`none`** - No AI enhancement
   - Direct execution without AI sampling
   - Used by 3 utility tools (generate-kustomize, inspect-session)

### Enhancement Capability Categories

#### Content Generation
- `content-generation` - AI-powered content creation
- `manifest-generation` - Kubernetes manifest creation
- `chart-generation` - Helm chart creation
- `dockerfile-repair` - Dockerfile fixing and optimization

#### Analysis and Intelligence
- `framework-detection` - Technology stack identification
- `vulnerability-analysis` - Security vulnerability assessment
- `performance-analysis` - Performance bottleneck identification
- `health-validation` - Deployment health checking

#### Optimization
- `optimization` - General performance and resource optimization
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
- `operational-insights` - System operation recommendations
- `deployment-analysis` - Deployment process analysis

## Tool Metadata Structure

All AI-enhanced tools follow this metadata pattern:

```typescript
metadata: {
  aiDriven: boolean;                    // Uses AI sampling system
  knowledgeEnhanced: boolean;           // Uses knowledge enhancement
  samplingStrategy: 'rerank' | 'none'; // Sampling approach
  enhancementCapabilities: string[];   // List of enhancement types
}
```

## Discovery and Introspection

### Finding AI-Enhanced Tools

```typescript
// Tools with AI enhancement
const aiTools = [
  'analyze-repo', 'generate-dockerfile', 'fix-dockerfile', 'build-image',
  'tag-image', 'push-image', 'scan', 'generate-k8s-manifests',
  'generate-helm-charts', 'prepare-cluster', 'deploy', 'verify-deployment',
  'resolve-base-images', 'generate-aca-manifests', 'convert-aca-to-k8s', 'ops'
];

// Utility tools (no AI)
const utilityTools = ['generate-kustomize', 'inspect-session'];
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
3. **Sample Generation** - Multiple candidate solutions generated
4. **Quality Scoring** - Candidates scored based on multiple criteria
5. **Result Selection** - Highest-scoring candidate returned
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
  samplingCount: 3,        // Number of candidates
  stopAt: 85,             // Quality threshold
  confidenceThreshold: 0.8 // Minimum confidence
};
```

This comprehensive reference provides complete visibility into the AI enhancement capabilities across all tools in the containerization-assist project.