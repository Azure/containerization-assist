/**
 * Tool Metadata for VS Code Extension Integration
 *
 * Provides comprehensive metadata for each tool including:
 * - LLM-facing descriptions (modelDescription)
 * - User-facing descriptions (userDescription)
 * - Display names and icons
 * - Confirmation message templates
 * - Workflow hints (suggested next tools)
 *
 * @example
 * ```typescript
 * import { toolMetadata } from 'containerization-assist-mcp/sdk';
 *
 * // Use in VS Code package.json
 * const config = toolMetadata.analyzeRepo;
 * console.log(config.modelDescription);
 * ```
 */

// ===== EXTERNAL DEPENDENCY CONSTANTS =====

/**
 * External dependency identifiers.
 *
 * Use these constants instead of magic strings when specifying
 * tool dependencies. This ensures consistency and type safety.
 */
export const ExternalDeps = {
  /** Docker daemon */
  DOCKER: 'docker',
  /** Trivy security scanner */
  TRIVY: 'trivy',
  /** Kubernetes CLI */
  KUBECTL: 'kubectl',
  /** Kubernetes cluster access */
  CLUSTER_ACCESS: 'cluster-access',
  /** Container registry authentication */
  REGISTRY_AUTH: 'registry-auth',
} as const;

/**
 * Type for external dependency identifiers.
 */
export type ExternalDepId = (typeof ExternalDeps)[keyof typeof ExternalDeps];

/**
 * External dependency requirement.
 *
 * Structured representation of a tool's external dependency,
 * replacing the previous string-based format.
 */
export interface ExternalDependency {
  /** Dependency identifier */
  readonly id: ExternalDepId;
  /** Whether this dependency is optional */
  readonly optional?: boolean;
  /** Human-readable description (optional) */
  readonly description?: string;
}

/**
 * Get human-readable list of required dependencies.
 *
 * @param deps - Array of external dependencies
 * @returns Comma-separated string of dependency names
 */
export function getRequiredDepsString(deps: readonly ExternalDependency[]): string {
  return deps
    .map((dep) => (dep.optional ? `${dep.id} (optional)` : dep.id))
    .join(', ');
}

/**
 * Check if a tool has a specific required dependency.
 *
 * @param metadata - Tool metadata to check
 * @param depId - Dependency ID to look for
 * @param includeOptional - Whether to include optional dependencies
 * @returns True if the dependency is required
 */
export function requiresDependency(
  metadata: ToolMetadata,
  depId: ExternalDepId,
  includeOptional = false,
): boolean {
  return metadata.requiresExternalDeps.some(
    (dep) => dep.id === depId && (includeOptional || !dep.optional),
  );
}

// ===== TYPES =====

/**
 * Confirmation message configuration for prepareInvocation().
 */
export interface ConfirmationConfig {
  /** Title for the confirmation dialog */
  title: string;
  /** Template for confirmation message (use {{param}} for substitution) */
  messageTemplate: string;
  /** Whether this operation is read-only (affects messaging) */
  isReadOnly: boolean;
  /** Warning message for destructive operations (optional) */
  warning?: string;
}

/**
 * Complete metadata for a single tool.
 */
export interface ToolMetadata {
  /** Tool name for VS Code registration (snake_case) */
  readonly name: string;
  /** User-friendly display name */
  readonly displayName: string;
  /** Short name for #tool references in chat */
  readonly toolReferenceName: string;
  /** Detailed description for LLM consumption - explains when/how to use */
  readonly modelDescription: string;
  /** Brief description for users */
  readonly userDescription: string;
  /** VS Code icon identifier */
  readonly icon: string;
  /** Whether tool can be referenced with # in prompts */
  readonly canBeReferencedInPrompt: boolean;
  /** Confirmation dialog configuration */
  readonly confirmation: Readonly<ConfirmationConfig>;
  /** Suggested next tools in typical workflow */
  readonly suggestedNextTools: readonly string[];
  /** Tool category for grouping */
  readonly category: 'analysis' | 'dockerfile' | 'image' | 'kubernetes' | 'operations';
  /** External dependencies required by this tool */
  readonly requiresExternalDeps: readonly ExternalDependency[];
}

// ===== TOOL METADATA DEFINITIONS =====

/**
 * Metadata for analyze-repo tool.
 */
export const analyzeRepoMetadata: ToolMetadata = {
  name: 'analyze_repo',
  displayName: 'Analyze Repository',
  toolReferenceName: 'containerization-analyze',
  modelDescription: `Analyzes a repository to detect programming languages, frameworks, dependencies, and build systems. This should be the FIRST tool used when containerizing an application. Returns detailed module information including:
- Detected languages and versions (Node.js, Python, Go, Java, etc.)
- Frameworks (Express, FastAPI, Spring Boot, etc.)
- Package managers and dependency files
- Build systems and entry points
- Monorepo structure (if applicable)

Use the analysis results to inform Dockerfile generation. This is a read-only operation that only examines file contents.`,
  userDescription: 'Detect languages, frameworks, and dependencies in a repository',
  icon: '$(search)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Analyze Repository',
    messageTemplate: 'Analyze repository at:\n\n`{{repositoryPath}}`',
    isReadOnly: true,
  },
  suggestedNextTools: ['generate_dockerfile'],
  category: 'analysis',
  requiresExternalDeps: [] as const,
};

/**
 * Metadata for generate-dockerfile tool.
 */
export const generateDockerfileMetadata: ToolMetadata = {
  name: 'generate_dockerfile',
  displayName: 'Generate Dockerfile',
  toolReferenceName: 'containerization-dockerfile',
  modelDescription: `Generates an optimized Dockerfile plan for a repository. Use AFTER analyze_repo to have context about the application. Returns a structured plan including:
- Recommended base images with security rationale
- Multi-stage build strategy
- Dependency installation commands
- Build and runtime configurations
- Security best practices (non-root user, minimal attack surface)
- Health check recommendations

The plan is returned as structured data, NOT the actual Dockerfile content. The LLM should use this plan to write the final Dockerfile.`,
  userDescription: 'Generate Dockerfile recommendations for containerization',
  icon: '$(file-code)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Generate Dockerfile',
    messageTemplate:
      'Generate Dockerfile plan for:\n\n`{{repositoryPath}}`\n\nTarget: {{targetPlatform}}',
    isReadOnly: true,
  },
  suggestedNextTools: ['build_image'],
  category: 'dockerfile',
  requiresExternalDeps: [] as const,
};

/**
 * Metadata for fix-dockerfile tool.
 */
export const fixDockerfileMetadata: ToolMetadata = {
  name: 'fix_dockerfile',
  displayName: 'Fix Dockerfile',
  toolReferenceName: 'containerization-fix',
  modelDescription: `Analyzes an existing Dockerfile and provides fix recommendations. Use when a user has an existing Dockerfile with issues (security vulnerabilities, inefficiencies, or errors). Returns:
- Validation issues found (security, best practices, optimization)
- Specific fix recommendations with before/after examples
- Severity levels (critical, warning, suggestion)
- Line numbers and affected instructions

Does NOT modify the file - returns recommendations for the LLM to apply.`,
  userDescription: 'Analyze and fix issues in existing Dockerfiles',
  icon: '$(tools)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Fix Dockerfile',
    messageTemplate: 'Analyze Dockerfile at:\n\n`{{dockerfilePath}}`',
    isReadOnly: true,
  },
  suggestedNextTools: ['build_image', 'scan_image'],
  category: 'dockerfile',
  requiresExternalDeps: [] as const,
};

/**
 * Metadata for build-image tool.
 */
export const buildImageMetadata: ToolMetadata = {
  name: 'build_image',
  displayName: 'Build Docker Image',
  toolReferenceName: 'containerization-build',
  modelDescription: `Builds a Docker image from a Dockerfile. REQUIRES Docker daemon to be running. Use after generating or fixing a Dockerfile. Accepts either:
- A path to an existing Dockerfile (dockerfilePath)
- Dockerfile content as a string (dockerfile)

Returns build results including:
- Image ID and tags
- Build duration and image size
- Build logs (truncated if very long)
- Success/failure status with error details

On failure, provides actionable error messages. Consider running scan_image after successful builds.`,
  userDescription: 'Build a Docker image from a Dockerfile',
  icon: '$(package)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Build Docker Image',
    messageTemplate: 'Build Docker image:\n\n**Context**: `{{path}}`\n**Image**: `{{imageName}}`',
    isReadOnly: false,
    warning: 'This will execute Docker build commands.',
  },
  suggestedNextTools: ['scan_image', 'tag_image'],
  category: 'image',
  requiresExternalDeps: [{ id: ExternalDeps.DOCKER }],
};

/**
 * Metadata for scan-image tool.
 */
export const scanImageMetadata: ToolMetadata = {
  name: 'scan_image',
  displayName: 'Scan Docker Image',
  toolReferenceName: 'containerization-scan',
  modelDescription: `Scans a Docker image for security vulnerabilities using Trivy. Use after building an image to identify security issues before deployment. Returns:
- Vulnerability summary by severity (critical, high, medium, low)
- Detailed vulnerability list with CVE IDs
- Affected packages and fixed versions
- Remediation recommendations

Requires Trivy to be installed. If Trivy is not available, returns a limited analysis based on image inspection.`,
  userDescription: 'Scan a Docker image for security vulnerabilities',
  icon: '$(shield)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Scan Docker Image',
    messageTemplate: 'Scan image for vulnerabilities:\n\n`{{imageId}}`',
    isReadOnly: true,
  },
  suggestedNextTools: ['fix_dockerfile', 'tag_image', 'push_image'],
  category: 'image',
  requiresExternalDeps: [
    { id: ExternalDeps.DOCKER },
    { id: ExternalDeps.TRIVY, optional: true },
  ],
};

/**
 * Metadata for tag-image tool.
 */
export const tagImageMetadata: ToolMetadata = {
  name: 'tag_image',
  displayName: 'Tag Docker Image',
  toolReferenceName: 'containerization-tag',
  modelDescription: `Tags a Docker image with a new tag. Use to add version tags, registry prefixes, or environment labels before pushing. Simple operation that creates an alias for an existing image.

Example use cases:
- Add version tag: myapp:latest → myapp:v1.2.3
- Add registry prefix: myapp:v1 → registry.example.com/myapp:v1
- Add environment tag: myapp:v1 → myapp:v1-production`,
  userDescription: 'Add a new tag to a Docker image',
  icon: '$(tag)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Tag Docker Image',
    messageTemplate: 'Tag image:\n\n`{{sourceImage}}` → `{{targetImage}}`',
    isReadOnly: false,
  },
  suggestedNextTools: ['push_image'],
  category: 'image',
  requiresExternalDeps: [{ id: ExternalDeps.DOCKER }],
};

/**
 * Metadata for push-image tool.
 */
export const pushImageMetadata: ToolMetadata = {
  name: 'push_image',
  displayName: 'Push Docker Image',
  toolReferenceName: 'containerization-push',
  modelDescription: `Pushes a Docker image to a container registry. REQUIRES authentication to be configured (docker login). Use after building and optionally scanning/tagging an image.

The image name must include the registry (unless pushing to Docker Hub). Examples:
- Docker Hub: username/myapp:v1
- ACR: myregistry.azurecr.io/myapp:v1
- GCR: gcr.io/myproject/myapp:v1
- ECR: 123456789.dkr.ecr.region.amazonaws.com/myapp:v1`,
  userDescription: 'Push a Docker image to a container registry',
  icon: '$(cloud-upload)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Push Docker Image',
    messageTemplate: 'Push image to registry:\n\n`{{imageId}}`',
    isReadOnly: false,
    warning: 'This will upload the image to a remote registry.',
  },
  suggestedNextTools: ['generate_k8s_manifests'],
  category: 'image',
  requiresExternalDeps: [
    { id: ExternalDeps.DOCKER },
    { id: ExternalDeps.REGISTRY_AUTH },
  ],
};

/**
 * Metadata for generate-k8s-manifests tool.
 */
export const generateK8sManifestsMetadata: ToolMetadata = {
  name: 'generate_k8s_manifests',
  displayName: 'Generate Kubernetes Manifests',
  toolReferenceName: 'containerization-k8s',
  modelDescription: `Generates Kubernetes deployment manifests for an application. Use after building and pushing an image. Returns a structured plan including:
- Deployment configuration (replicas, resources, probes)
- Service configuration (ports, type)
- ConfigMap/Secret templates (if applicable)
- Ingress configuration (optional)
- Security context recommendations

The plan is returned as structured data. The LLM should use this to write the final YAML manifests.`,
  userDescription: 'Generate Kubernetes deployment manifests',
  icon: '$(server)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Generate Kubernetes Manifests',
    messageTemplate:
      'Generate K8s manifests for:\n\n**Repository**: `{{repositoryPath}}`\n**Image**: `{{imageName}}`',
    isReadOnly: true,
  },
  suggestedNextTools: ['prepare_cluster', 'verify_deploy'],
  category: 'kubernetes',
  requiresExternalDeps: [] as const,
};

/**
 * Metadata for prepare-cluster tool.
 */
export const prepareClusterMetadata: ToolMetadata = {
  name: 'prepare_cluster',
  displayName: 'Prepare Kubernetes Cluster',
  toolReferenceName: 'containerization-prepare',
  modelDescription: `Prepares a Kubernetes cluster namespace for deployment. REQUIRES kubectl to be configured with cluster access. Creates namespace if needed and verifies cluster connectivity.

Use before deploying to ensure the target namespace exists and the cluster is accessible. This is an idempotent operation - safe to run multiple times.`,
  userDescription: 'Prepare a Kubernetes namespace for deployment',
  icon: '$(server-environment)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Prepare Kubernetes Cluster',
    messageTemplate:
      'Prepare namespace:\n\n`{{namespace}}`\n\nCreate if missing: {{createNamespace}}',
    isReadOnly: false,
    warning: 'This will create resources in your Kubernetes cluster.',
  },
  suggestedNextTools: ['verify_deploy'],
  category: 'kubernetes',
  requiresExternalDeps: [
    { id: ExternalDeps.KUBECTL },
    { id: ExternalDeps.CLUSTER_ACCESS },
  ],
};

/**
 * Metadata for verify-deploy tool.
 */
export const verifyDeployMetadata: ToolMetadata = {
  name: 'verify_deploy',
  displayName: 'Verify Deployment',
  toolReferenceName: 'containerization-verify',
  modelDescription: `Verifies a Kubernetes deployment is healthy and ready. REQUIRES kubectl to be configured. Use after deploying to confirm the application is running correctly. Checks:
- Deployment rollout status
- Pod readiness and health
- Recent events and errors
- Service endpoint availability

Includes configurable timeout for waiting on deployment readiness.`,
  userDescription: 'Verify a Kubernetes deployment is healthy',
  icon: '$(check)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Verify Deployment',
    messageTemplate:
      'Verify deployment:\n\n**Namespace**: `{{namespace}}`\n**Deployment**: `{{deploymentName}}`',
    isReadOnly: true,
  },
  suggestedNextTools: [],
  category: 'kubernetes',
  requiresExternalDeps: [
    { id: ExternalDeps.KUBECTL },
    { id: ExternalDeps.CLUSTER_ACCESS },
  ],
};

/**
 * Metadata for ops tool.
 */
export const opsMetadata: ToolMetadata = {
  name: 'ops',
  displayName: 'Operations',
  toolReferenceName: 'containerization-ops',
  modelDescription: `Operational utilities for health checks and status. Available actions:
- ping: Simple health check to verify the tool is responsive
- status: Returns server status and version information

Use ping to verify connectivity before starting a workflow.`,
  userDescription: 'Health checks and operational utilities',
  icon: '$(pulse)',
  canBeReferencedInPrompt: true,
  confirmation: {
    title: 'Operations',
    messageTemplate: 'Run operation: {{action}}',
    isReadOnly: true,
  },
  suggestedNextTools: ['analyze_repo'],
  category: 'operations',
  requiresExternalDeps: [] as const,
};

// ===== AGGREGATED EXPORT =====

/**
 * Type-safe registry ensuring all tools have metadata.
 * TypeScript will error if a tool is missing or has wrong type.
 */
interface ToolMetadataRegistry {
  analyzeRepo: ToolMetadata;
  generateDockerfile: ToolMetadata;
  fixDockerfile: ToolMetadata;
  buildImage: ToolMetadata;
  scanImage: ToolMetadata;
  tagImage: ToolMetadata;
  pushImage: ToolMetadata;
  generateK8sManifests: ToolMetadata;
  prepareCluster: ToolMetadata;
  verifyDeploy: ToolMetadata;
  ops: ToolMetadata;
}

/**
 * All tool metadata for VS Code extension configuration.
 *
 * Uses `satisfies` to ensure type safety:
 * - TypeScript will error if a tool is missing
 * - TypeScript will error if a tool has the wrong type
 * - TypeScript will error if a key is misspelled
 *
 * @example
 * ```typescript
 * import { toolMetadata } from 'containerization-assist-mcp/sdk';
 *
 * // Generate package.json languageModelTools entry
 * const entry = {
 *   name: toolMetadata.analyzeRepo.name,
 *   displayName: toolMetadata.analyzeRepo.displayName,
 *   modelDescription: toolMetadata.analyzeRepo.modelDescription,
 *   // ...
 * };
 * ```
 */
export const toolMetadata = {
  analyzeRepo: analyzeRepoMetadata,
  generateDockerfile: generateDockerfileMetadata,
  fixDockerfile: fixDockerfileMetadata,
  buildImage: buildImageMetadata,
  scanImage: scanImageMetadata,
  tagImage: tagImageMetadata,
  pushImage: pushImageMetadata,
  generateK8sManifests: generateK8sManifestsMetadata,
  prepareCluster: prepareClusterMetadata,
  verifyDeploy: verifyDeployMetadata,
  ops: opsMetadata,
} as const satisfies ToolMetadataRegistry;

export type { ToolMetadataRegistry };

/**
 * Type for toolMetadata object keys.
 */
export type ToolMetadataName = keyof typeof toolMetadata;

// ===== WORKFLOW HELPERS =====

/**
 * Standard containerization workflow order.
 *
 * Useful for understanding the typical tool sequence:
 * analyze → generate dockerfile → build → scan → tag → push → generate k8s → prepare → verify
 */
export const standardWorkflow: readonly ToolMetadataName[] = [
  'analyzeRepo',
  'generateDockerfile',
  'buildImage',
  'scanImage',
  'tagImage',
  'pushImage',
  'generateK8sManifests',
  'prepareCluster',
  'verifyDeploy',
] as const;

/**
 * Get tools by category.
 */
export function getToolsByCategory(category: ToolMetadata['category']): ToolMetadata[] {
  return Object.values(toolMetadata).filter((t) => t.category === category);
}
