import { Failure, Result, Success } from '@/types';

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
  values?: Record<string, any>;
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
  scanResults?: any;
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

    prompt += `.\n\nRequirements:\n`;
    prompt += `- Language: ${params.language}\n`;

    if (params.framework) {
      prompt += `- Framework: ${params.framework}\n`;
    }

    prompt += `- Dependencies: ${deps}\n`;
    prompt += `- Exposed ports: ${ports}\n`;

    if (params.baseImage) {
      prompt += `- Use base image: ${params.baseImage}\n`;
    }

    if (params.multistage) {
      prompt += `- Use multi-stage build for optimization\n`;
    }

    if (params.securityHardening) {
      prompt += `- Apply security best practices (non-root user, minimal attack surface)\n`;
    }

    if (params.optimization) {
      prompt += `- Optimize for size and build time\n`;
      prompt += `- Use layer caching effectively\n`;
    }

    if (params.requirements) {
      prompt += `\nAdditional requirements:\n${params.requirements}\n`;
    }

    prompt += `\nProvide only the Dockerfile content without explanations or markdown fences.`;

    return prompt;
  },

  /**
   * Generate prompt for Kubernetes manifests
   */
  k8sManifests: (params: K8sManifestPromptParams): string => {
    let prompt = `Generate production-ready Kubernetes manifests for deploying ${params.appName}.\n\n`;

    prompt += `Requirements:\n`;
    prompt += `- Application name: ${params.appName}\n`;
    prompt += `- Container image: ${params.image}\n`;
    prompt += `- Namespace: ${params.namespace || 'default'}\n`;
    prompt += `- Replicas: ${params.replicas || 1}\n`;
    prompt += `- Service port: ${params.port || 8080}\n`;
    prompt += `- Service type: ${params.serviceType || 'ClusterIP'}\n`;

    if (params.ingressEnabled) {
      prompt += `- Include Ingress resource for external access\n`;
    }

    if (params.resources) {
      prompt += `- Resource limits:\n`;
      if (params.resources.cpu) {
        prompt += `  - CPU: ${params.resources.cpu}\n`;
      }
      if (params.resources.memory) {
        prompt += `  - Memory: ${params.resources.memory}\n`;
      }
    }

    if (params.healthCheck) {
      prompt += `- Include liveness and readiness probes\n`;
    }

    prompt += `\nGenerate the following resources:\n`;
    prompt += `1. Deployment\n`;
    prompt += `2. Service\n`;

    if (params.ingressEnabled) {
      prompt += `3. Ingress\n`;
    }

    prompt += `\nProvide valid YAML manifests separated by "---". No explanations or markdown fences.`;

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

    if (params.requirements) {
      prompt += `\nAdditional requirements:\n${params.requirements}\n`;
    }

    prompt += `\nProvide the optimized configuration with improvements applied. Include comments explaining significant changes.`;

    return prompt;
  },

  /**
   * Repository analysis prompt
   */
  repositoryAnalysis: (params: RepositoryAnalysisParams): string => {
    return `You are an expert software architect with deep knowledge of ALL programming languages,
frameworks, and build systems. Analyze repositories without bias toward any specific language.

Languages you support include but are not limited to:
- Backend: Java, Python, Node.js/TypeScript, Go, Rust, C#, Ruby, PHP, Scala, Kotlin
- Frontend: React, Vue, Angular, Svelte, Next.js, Nuxt.js
- Mobile: Swift, Kotlin, React Native, Flutter
- Data/ML: Python, R, Julia, Jupyter
- Systems: C, C++, Rust, Zig

Provide accurate, unbiased analysis focusing on the most likely language and framework.

## User Request

Analyze this repository to identify the technology stack:

**File listing:**
${params.fileList}

**Configuration files:**
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
} as const;

// ===== VALIDATION HELPERS =====

/**
 * Validate required parameters are present
 */
export function validatePromptParams<T extends Record<string, any>>(
  params: T,
  required: (keyof T)[],
): Result<void> {
  for (const key of required) {
    if (params[key] === undefined || params[key] === null) {
      return Failure(`Missing required parameter: ${String(key)}`);
    }
  }
  return Success(undefined);
}

/**
 * Build a complete AI prompt with context
 */
export function buildAIPrompt(
  template: string,
  context?: {
    projectType?: string;
    existingFiles?: string[];
    constraints?: string[];
  },
): string {
  let fullPrompt = template;

  if (context) {
    if (context.projectType) {
      fullPrompt = `Project type: ${context.projectType}\n\n${fullPrompt}`;
    }

    if (context.existingFiles && context.existingFiles.length > 0) {
      fullPrompt += `\n\nExisting files in project:\n`;
      context.existingFiles.forEach((file) => {
        fullPrompt += `- ${file}\n`;
      });
    }

    if (context.constraints && context.constraints.length > 0) {
      fullPrompt += `\n\nConstraints:\n`;
      context.constraints.forEach((constraint) => {
        fullPrompt += `- ${constraint}\n`;
      });
    }
  }

  return fullPrompt;
}

/**
 * Extract structured data from AI response
 */
export function parseAIResponse(
  response: string,
  expectedFormat: 'yaml' | 'dockerfile' | 'json',
): Result<string> {
  // Remove markdown fences if present
  const cleaned = response.replace(/```[\w]*\n?/g, '').trim();

  // Basic validation based on format
  switch (expectedFormat) {
    case 'yaml':
      // More permissive YAML validation - allow simple scalar values
      // Real YAML validation would require a proper parser
      if (cleaned.length === 0) {
        return Failure('Response is empty');
      }
      break;
    case 'dockerfile':
      if (!cleaned.includes('FROM')) {
        return Failure('Response does not appear to be a valid Dockerfile');
      }
      break;
    case 'json':
      try {
        JSON.parse(cleaned);
      } catch (e) {
        return Failure(`Response is not valid JSON: ${e instanceof Error ? e.message : String(e)}`);
      }
      break;
  }

  return Success(cleaned);
}
