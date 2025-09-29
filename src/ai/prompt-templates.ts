/**
 * Type-safe AI prompt templates using TypeScript template literals
 * These templates structure prompts for AI generation, not static output
 */

// ===== PARAMETER INTERFACES =====
// These interfaces define the structure of parameters for each prompt template

export interface DockerfilePromptParams {
  language: string;
  framework?: string;
  dependencies?: string[];
  ports?: number[];
  requirements?: string;
  baseImage?: string;
  optimization?: boolean;
  securityHardening?: boolean;
  multistage?: boolean;
}

export interface K8sManifestPromptParams {
  appName: string;
  image: string;
  replicas?: number;
  port?: number;
  namespace?: string;
  serviceType?: string;
  ingressEnabled?: boolean;
  resources?: {
    cpu?: string;
    memory?: string;
  };
  healthCheck?: boolean;
}

export interface HelmChartPromptParams {
  appName: string;
  description?: string;
  version?: string;
  dependencies?: string[];
  values?: Record<string, unknown>;
}

export interface OptimizationPromptParams {
  currentContent: string;
  contentType: 'dockerfile' | 'k8s' | 'helm';
  issues?: string[];
  requirements?: string;
}

export interface RepositoryAnalysisParams {
  fileList: string;
  configFiles: string;
  directoryTree: string;
}

export interface SecurityAnalysisParams {
  dockerfileContent?: string;
  imageId?: string;
  scanResults?: Record<string, unknown>;
}

export interface BaseImageResolutionParams {
  language: string;
  framework?: string;
  version?: string;
  requirements?: string[];
}

export interface AcaManifestParams {
  appName: string;
  image: string;
  environment?: string;
  resources?: {
    cpu?: string;
    memory?: string;
  };
  scaling?: {
    minReplicas?: number;
    maxReplicas?: number;
  };
}

export interface ParameterSuggestionParams {
  toolName: string;
  currentParams: Record<string, unknown>;
  missingParams: string[];
  context?: Record<string, unknown>;
}

export interface KnowledgeEnhancementParams {
  content: string;
  context: 'dockerfile' | 'kubernetes' | 'security' | 'optimization';
  validationIssues?: string[];
  targetImprovement: string;
  userQuery?: string;
}

// ===== PROMPT TEMPLATES =====

/**
 * Type-safe prompt builders for AI generation
 */
export const promptTemplates = {
  /**
   * Generate prompt for Dockerfile creation
   */
  dockerfile: (params: DockerfilePromptParams): string => {
    const deps = params.dependencies?.join(', ') || 'standard libraries';
    const ports = params.ports?.map((p) => p.toString()).join(', ') || '8080';

    let prompt = `Generate a production-ready Dockerfile for a ${params.language} application`;

    if (params.framework) {
      prompt += ` using ${params.framework}`;
    }

    prompt += `.\n\n`;

    // Add critical constraints first to prevent common errors
    prompt += `CRITICAL CONSTRAINTS (MUST FOLLOW):\n`;
    prompt += `1. Package Manager Consistency:\n`;

    // Language-specific package manager rules
    if (params.language?.toLowerCase().includes('java')) {
      prompt += `   - This is a Java application. DO NOT use apt-get to install Java tools\n`;
      prompt += `   - Use Maven or Gradle commands that are included in the base image\n`;
      prompt += `   - For Alpine-based images, use 'apk add' not 'apt-get'\n`;
      prompt += `   - For Debian-based images, use 'apt-get' not 'apk'\n`;
    } else if (
      params.language?.toLowerCase().includes('node') ||
      params.language?.toLowerCase().includes('javascript')
    ) {
      prompt += `   - This is a Node.js application. Use npm, yarn, or pnpm commands\n`;
      prompt += `   - For Alpine-based images, use 'apk add' not 'apt-get'\n`;
      prompt += `   - For Debian-based images, use 'apt-get' not 'apk'\n`;
    } else if (params.language?.toLowerCase().includes('python')) {
      prompt += `   - This is a Python application. Use pip for Python packages\n`;
      prompt += `   - For Alpine-based images, use 'apk add' not 'apt-get'\n`;
      prompt += `   - For Debian-based images, use 'apt-get' not 'apk'\n`;
    }

    prompt += `2. Base Image Selection:\n`;
    prompt += `   - Choose appropriate base image for ${params.language}\n`;
    prompt += `   - Consider security, size, and compatibility requirements\n`;
    prompt += `   - Use valid and current image tags\n`;
    prompt += `   - Match package manager to base image (tdnf for Mariner, apk for Alpine, apt for Debian)\n`;

    prompt += `3. Health Check Commands:\n`;
    prompt += `   - Only use commands that exist in the chosen base image\n`;
    prompt += `   - For minimal images: use built-in tools or install required ones\n`;
    prompt += `   - Common tools: curl (often missing), wget (sometimes available)\n`;
    prompt += `   - For Java: consider using the application's health endpoint directly\n`;

    prompt += `4. Build Tools:\n`;
    prompt += `   - Use wrapper scripts when available (mvnw, gradlew)\n`;
    prompt += `   - Download dependencies during build for better caching\n`;
    prompt += `   - For Node: npm ci is preferred over npm install\n`;
    prompt += `   - DO NOT install build tools if already in base image\n\n`;

    prompt += `Requirements:\n`;
    prompt += `- Language: ${params.language}\n`;

    if (params.framework) {
      prompt += `- Framework: ${params.framework}\n`;
    }

    prompt += `- Dependencies: ${deps}\n`;
    prompt += `- Exposed ports: ${ports}\n`;

    if (params.baseImage) {
      prompt += `- Use base image: ${params.baseImage}\n`;
    } else {
      prompt += `- Select an appropriate base image for ${params.language}\n`;
    }

    if (params.multistage) {
      prompt += `- Use multi-stage build for optimization\n`;
      prompt += `- Ensure each stage uses compatible base images\n`;
    }

    if (params.securityHardening) {
      prompt += `- Apply security best practices (non-root user, minimal attack surface)\n`;
      prompt += `- Create a non-root user appropriately for the base image\n`;
    }

    if (params.optimization) {
      prompt += `- Optimize for size and build time\n`;
      prompt += `- Use layer caching effectively\n`;
      prompt += `- Minimize layer count where possible\n`;
    }

    if (params.requirements) {
      prompt += `\nAdditional context:\n${params.requirements}\n`;
    }

    prompt += `\nOutput Format:\n`;
    prompt += `- Provide ONLY the Dockerfile content\n`;
    prompt += `- NO explanations or markdown fences\n`;
    prompt += `- NO JSON wrapping\n`;
    prompt += `- Start directly with FROM statement\n`;
    prompt += `- End with CMD or ENTRYPOINT statement\n`;
    prompt += `- Ensure the Dockerfile will build without errors\n`;

    return prompt;
  },

  /**
   * Generate prompt for Kubernetes manifests
   */
  k8sManifests: (params: K8sManifestPromptParams): string => {
    let prompt = `Generate production-ready Kubernetes manifests for deploying ${params.appName}.\n\n`;

    // Add critical constraints first
    prompt += `CRITICAL REQUIREMENTS (MUST FOLLOW):\n`;
    prompt += `1. Valid Kubernetes API versions:\n`;
    prompt += `   - Deployment: apps/v1\n`;
    prompt += `   - Service: v1\n`;
    prompt += `   - Ingress: networking.k8s.io/v1\n`;
    prompt += `   - ConfigMap/Secret: v1\n`;
    prompt += `2. Required fields:\n`;
    prompt += `   - All resources must have metadata.name and metadata.namespace\n`;
    prompt += `   - Deployments must have spec.selector.matchLabels matching template.metadata.labels\n`;
    prompt += `   - Services must have spec.selector matching deployment pod labels\n`;
    prompt += `3. Best practices:\n`;
    prompt += `   - Always set resource requests and limits\n`;
    prompt += `   - Include liveness and readiness probes\n`;
    prompt += `   - Use non-root security context\n`;
    prompt += `   - Set imagePullPolicy: IfNotPresent for tagged images\n`;
    prompt += `4. Label conventions:\n`;
    prompt += `   - app.kubernetes.io/name: ${params.appName}\n`;
    prompt += `   - app.kubernetes.io/instance: ${params.appName}\n`;
    prompt += `   - app.kubernetes.io/version: "1.0.0"\n`;
    prompt += `   - app.kubernetes.io/managed-by: "mcp"\n\n`;

    prompt += `Application Requirements:\n`;
    prompt += `- Application name: ${params.appName}\n`;
    prompt += `- Container image: ${params.image}\n`;
    prompt += `- Namespace: ${params.namespace || 'default'}\n`;
    prompt += `- Replicas: ${params.replicas || 1}\n`;
    prompt += `- Service port: ${params.port || 8080}\n`;
    prompt += `- Service type: ${params.serviceType || 'ClusterIP'}\n`;

    if (params.ingressEnabled) {
      prompt += `- Include Ingress resource for external access\n`;
      prompt += `  - Use pathType: Prefix\n`;
      prompt += `  - Include proper annotations for your ingress controller\n`;
    }

    if (params.resources) {
      prompt += `- Resource limits:\n`;
      if (params.resources.cpu) {
        prompt += `  - CPU: ${params.resources.cpu}\n`;
      }
      if (params.resources.memory) {
        prompt += `  - Memory: ${params.resources.memory}\n`;
      }
    } else {
      prompt += `- Use sensible default resource limits (e.g., 100m CPU request, 500m limit; 128Mi memory request, 512Mi limit)\n`;
    }

    if (params.healthCheck) {
      prompt += `- Include liveness probe (failureThreshold: 3, periodSeconds: 10)\n`;
      prompt += `- Include readiness probe (initialDelaySeconds: 10, periodSeconds: 5)\n`;
    }

    prompt += `\nGenerate the following resources:\n`;
    prompt += `1. Deployment (with proper labels, selectors, and security context)\n`;
    prompt += `2. Service (with correct selector matching deployment labels)\n`;

    if (params.ingressEnabled) {
      prompt += `3. Ingress (with proper API version and pathType)\n`;
    }

    prompt += `\nOutput Format:\n`;
    prompt += `- Provide ONLY valid Kubernetes YAML manifests\n`;
    prompt += `- Separate resources with "---"\n`;
    prompt += `- NO explanations, comments, or markdown fences\n`;
    prompt += `- Start directly with "apiVersion:"\n`;
    prompt += `- Ensure all manifests will apply without errors\n`;

    return prompt;
  },

  /**
   * Generate prompt for Helm charts
   */
  helmChart: (params: HelmChartPromptParams): string => {
    let prompt = `Generate a Helm chart structure for ${params.appName}.\n\n`;

    prompt += `Requirements:\n`;
    prompt += `- Chart name: ${params.appName}\n`;

    if (params.description) {
      prompt += `- Description: ${params.description}\n`;
    }

    if (params.version) {
      prompt += `- Version: ${params.version}\n`;
    }

    if (params.dependencies && params.dependencies.length > 0) {
      prompt += `- Dependencies: ${params.dependencies.join(', ')}\n`;
    }

    prompt += `\nGenerate the following files:\n`;
    prompt += `1. Chart.yaml with metadata\n`;
    prompt += `2. values.yaml with configurable parameters\n`;
    prompt += `3. templates/deployment.yaml\n`;
    prompt += `4. templates/service.yaml\n`;
    prompt += `5. templates/ingress.yaml (optional, controlled by values)\n`;

    if (params.values && Object.keys(params.values).length > 0) {
      prompt += `\nInclude these values in values.yaml:\n`;
      Object.entries(params.values).forEach(([key, value]) => {
        prompt += `- ${key}: ${JSON.stringify(value)}\n`;
      });
    }

    prompt += `\nProvide each file's content clearly labeled. Use production best practices.`;

    return prompt;
  },

  /**
   * Generate prompt for optimization tasks
   */
  optimization: (params: OptimizationPromptParams): string => {
    let prompt = `Optimize the following ${params.contentType} configuration:\n\n`;
    prompt += `\`\`\`\n${params.currentContent}\n\`\`\`\n\n`;

    if (params.issues && params.issues.length > 0) {
      prompt += `Known issues to address:\n`;
      params.issues.forEach((issue) => {
        prompt += `- ${issue}\n`;
      });
      prompt += `\n`;
    }

    prompt += `Optimization goals:\n`;
    prompt += `- Security best practices\n`;
    prompt += `- Performance optimization\n`;
    prompt += `- Resource efficiency\n`;
    prompt += `- Production readiness\n`;
    prompt += `- File existence validation for COPY commands\n`;

    if (params.requirements) {
      prompt += `\nAdditional requirements:\n${params.requirements}\n`;
    }

    prompt += `\nProvide the optimized configuration with improvements applied. Include comments explaining significant changes.`;

    return prompt;
  },

  /**
   * Repository analysis prompt
   */
  repositoryAnalysis: (params: RepositoryAnalysisParams & { sessionId?: string }): string => {
    return `You are an expert software architect with deep knowledge of ALL programming languages,
frameworks, and build systems. Analyze repositories without bias toward any specific language.

Languages you support include but are not limited to:
- Backend: Java, Python, Node.js/TypeScript, Go, Rust, C#, Ruby, PHP, Scala, Kotlin
- Frontend: React, Vue, Angular, Svelte, Next.js, Nuxt.js
- Mobile: Swift, Kotlin, React Native, Flutter
- Data/ML: Python, R, Julia, Jupyter
- Systems: C, C++, Rust, Zig

## Repository Data to Analyze

**Files in repository:**
${params.fileList}

**Configuration files content:**
${params.configFiles}

**Directory structure:**
${params.directoryTree}

Determine:
1. Primary programming language and version
2. Framework and version (if applicable)
3. Build system and package manager
4. Dependencies and dev dependencies
5. Application entry points
6. Default ports based on framework
7. Recommended Docker base images (minimal, standard, secure)
8. Containerization recommendations

Return ONLY valid JSON matching this structure:
{
  "language": "string",
  "languageVersion": "string or null",
  "framework": "string or null",
  "frameworkVersion": "string or null",
  "buildSystem": {
    "type": "string",
    "buildFile": "string",
    "buildCommand": "string or null",
    "testCommand": "string or null"
  },
  "dependencies": ["array of strings"],
  "devDependencies": ["array of strings"],
  "entryPoint": "string or null",
  "suggestedPorts": [array of numbers],
  "dockerConfig": {
    "baseImage": "recommended base image",
    "multistage": true/false,
    "nonRootUser": true/false
  },
  "sessionId": "${params.sessionId || 'generate a new UUID v4 session ID'}",
  "workflowHints": {
    "nextStep": "generate-dockerfile",
    "message": "Repository analyzed successfully. Use 'generate-dockerfile' with the sessionId to create an optimized Dockerfile."
  }
}`;
  },

  /**
   * Security analysis prompt
   */
  securityAnalysis: (params: SecurityAnalysisParams): string => {
    let prompt = `Analyze the following container configuration for security vulnerabilities and best practices.\n\n`;

    if (params.dockerfileContent) {
      prompt += `Dockerfile content:\n\`\`\`\n${params.dockerfileContent}\n\`\`\`\n\n`;
    }

    if (params.imageId) {
      prompt += `Image: ${params.imageId}\n\n`;
    }

    if (params.scanResults) {
      prompt += `Scan results:\n${JSON.stringify(params.scanResults, null, 2)}\n\n`;
    }

    prompt += `Identify:\n`;
    prompt += `1. Security vulnerabilities (critical, high, medium, low)\n`;
    prompt += `2. Best practice violations\n`;
    prompt += `3. Exposed secrets or sensitive data\n`;
    prompt += `4. Recommendations for remediation\n\n`;
    prompt += `Return a detailed security assessment with actionable recommendations.`;

    return prompt;
  },

  /**
   * Base image resolution prompt
   */
  baseImageResolution: (params: BaseImageResolutionParams): string => {
    let prompt = `Recommend optimal Docker base images for a ${params.language} application`;

    if (params.framework) {
      prompt += ` using ${params.framework}`;
    }

    if (params.version) {
      prompt += ` version ${params.version}`;
    }

    prompt += `.\n\nRequirements:\n`;

    if (params.requirements && params.requirements.length > 0) {
      prompt += `- ${params.requirements.join('\n- ')}\n`;
    }

    prompt += `\nProvide recommendations for:\n`;
    prompt += `1. Minimal image (smallest size, production)\n`;
    prompt += `2. Standard image (balanced size and features)\n`;
    prompt += `3. Development image (includes build tools)\n`;
    prompt += `4. Security-hardened image (distroless or minimal attack surface)\n\n`;
    prompt += `Include image size estimates and trade-offs for each option.`;

    return prompt;
  },

  /**
   * Azure Container Apps manifest generation
   */
  acaManifests: (params: AcaManifestParams): string => {
    let prompt = `Generate Azure Container Apps configuration for ${params.appName}.\n\n`;
    prompt += `Container image: ${params.image}\n`;

    if (params.environment) {
      prompt += `Environment: ${params.environment}\n`;
    }

    if (params.resources) {
      prompt += `Resources:\n`;
      if (params.resources.cpu) prompt += `  CPU: ${params.resources.cpu}\n`;
      if (params.resources.memory) prompt += `  Memory: ${params.resources.memory}\n`;
    }

    if (params.scaling) {
      prompt += `Scaling:\n`;
      if (params.scaling.minReplicas !== undefined)
        prompt += `  Min replicas: ${params.scaling.minReplicas}\n`;
      if (params.scaling.maxReplicas !== undefined)
        prompt += `  Max replicas: ${params.scaling.maxReplicas}\n`;
    }

    prompt += `\nGenerate:\n`;
    prompt += `1. Container App YAML configuration\n`;
    prompt += `2. Environment configuration\n`;
    prompt += `3. Ingress rules if needed\n`;
    prompt += `4. Scaling rules\n\n`;
    prompt += `Provide valid Azure Container Apps YAML manifests.`;

    return prompt;
  },

  /**
   * Parameter suggestion prompt
   */
  parameterSuggestions: (params: ParameterSuggestionParams): string => {
    let prompt = `Suggest values for missing parameters in the ${params.toolName} tool.\n\n`;
    prompt += `Current parameters:\n${JSON.stringify(params.currentParams, null, 2)}\n\n`;
    prompt += `Missing required parameters:\n- ${params.missingParams.join('\n- ')}\n\n`;

    if (params.context) {
      prompt += `Context:\n${JSON.stringify(params.context, null, 2)}\n\n`;
    }

    prompt += `Based on the context and common patterns, suggest appropriate values for the missing parameters.\n`;
    prompt += `Return suggestions as JSON with explanations for each suggested value.`;

    return prompt;
  },

  /**
   * Generate prompt for fixing issues
   */
  fix: (contentType: string, content: string, issues: string[]): string => {
    let prompt = `Fix the following issues in this ${contentType}:\n\n`;
    prompt += `Current content:\n\`\`\`\n${content}\n\`\`\`\n\n`;
    prompt += `Issues to fix:\n`;

    issues.forEach((issue, index) => {
      prompt += `${index + 1}. ${issue}\n`;
    });

    prompt += `\nProvide the corrected version with all issues resolved. Maintain the original structure where possible.`;

    return prompt;
  },

  /**
   * Convert Azure Container Apps to Kubernetes
   */
  convertAcaToK8s: (acaConfig: string): string => {
    return `Convert the following Azure Container Apps configuration to Kubernetes manifests:

\`\`\`yaml
${acaConfig}
\`\`\`

Generate equivalent Kubernetes resources:
1. Deployment with matching resource limits and replicas
2. Service for internal communication
3. HorizontalPodAutoscaler if scaling rules are defined
4. Ingress if external access is configured
5. ConfigMaps/Secrets for environment variables

Ensure the Kubernetes manifests maintain the same functionality and configuration as the ACA setup.
Provide valid YAML manifests separated by "---".`;
  },

  /**
   * Sampling strategy optimization
   */
  samplingStrategy: (contentType: string, requirements?: string): string => {
    let prompt = `Generate multiple variations of ${contentType} using different optimization strategies.

`;

    if (requirements) {
      prompt += `Requirements:
${requirements}

`;
    }

    prompt += `Create variations focusing on:
1. Size optimization (minimal layers, small base image)
2. Build speed optimization (cache efficiency, parallel builds)
3. Security optimization (non-root, minimal attack surface)
4. Performance optimization (runtime efficiency)

Provide distinct implementations for each strategy.`;

    return prompt;
  },

  /**
   * JSON repair prompt
   */
  jsonRepair: (invalidJson: string, error: string): string => {
    return `Fix the following invalid JSON:

\`\`\`
${invalidJson}
\`\`\`

Error: ${error}

Repair the JSON to make it valid while preserving the intended structure and data.
Return only the corrected JSON without explanations.`;
  },

  /**
   * Knowledge enhancement prompt template
   */
  knowledgeEnhancement: (params: KnowledgeEnhancementParams): string => {
    const contextInstruction = getKnowledgeContextInstruction(params.context);
    const issuesSection = params.validationIssues
      ? `
Known issues to address:
${params.validationIssues.map((issue) => `- ${issue}`).join('\n')}
`
      : '';

    const userQuerySection = params.userQuery
      ? `
Specific enhancement goal: ${params.userQuery}
`
      : '';

    return `Analyze and enhance the following ${params.context} content:

${params.content}
${issuesSection}${userQuerySection}
${contextInstruction}

Target improvement: ${params.targetImprovement}

Provide enhanced content with explanations for each improvement.

Response Format:
## Enhanced Content
[Provide the complete enhanced version]

## Knowledge Applied
[List specific knowledge areas applied]

## Improvements Made
[Explain what was improved and why]

## Additional Recommendations
[Provide actionable suggestions for further improvement]`;
  },
} as const;

// ===== VALIDATION HELPERS =====

/**
 * Extract structured data from AI response
 */

/**
 * Get knowledge enhancement context instructions
 */
function getKnowledgeContextInstruction(context: string): string {
  switch (context) {
    case 'dockerfile':
      return `Focus on Docker best practices, security hardening, build optimization, and layer efficiency.`;
    case 'kubernetes':
      return `Focus on Kubernetes best practices, resource management, security contexts, and deployment strategies.`;
    case 'security':
      return `Focus on security vulnerabilities, access controls, secrets management, and hardening measures.`;
    case 'optimization':
      return `Focus on performance optimization, resource efficiency, caching strategies, and cost reduction.`;
    default:
      return `Apply comprehensive best practices covering security, performance, and maintainability.`;
  }
}
