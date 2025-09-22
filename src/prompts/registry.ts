/**
 * Consolidated Prompt Registry - TypeScript Templates Direct Integration
 *
 * Combines the registry and template-bridge into a single module for simplicity.
 * All prompts are defined using TypeScript templates from templates.ts
 */

import type { Logger } from 'pino';
import {
  ListPromptsResult,
  GetPromptResult,
  PromptMessage,
  McpError,
  ErrorCode,
} from '@modelcontextprotocol/sdk/types.js';
import { Result, Success, Failure } from '@/types';
import {
  promptTemplates,
  validatePromptParams,
  buildAIPrompt,
  type DockerfilePromptParams,
  type K8sManifestPromptParams,
  type HelmChartPromptParams,
  type OptimizationPromptParams,
  type RepositoryAnalysisParams,
  type SecurityAnalysisParams,
  type BaseImageResolutionParams,
  type AcaManifestParams,
  type ParameterSuggestionParams,
} from './templates';

// Module-level state
let logger: Logger | null = null;

/**
 * Prompt handler function type
 */
type PromptHandler = (args: Record<string, unknown>) => Result<{
  description: string;
  messages: PromptMessage[];
}>;

/**
 * Convert template output to MCP-compatible prompt messages
 */
function createPromptMessages(
  content: string,
  role: 'user' | 'assistant' = 'user',
): PromptMessage[] {
  return [
    {
      role,
      content: {
        type: 'text',
        text: content,
      },
    },
  ];
}

/**
 * Registry of all available prompts with their handlers
 */
const PROMPT_HANDLERS: Record<string, { description: string; handler: PromptHandler }> = {
  // Dockerfile prompts
  'dockerfile-generation': {
    description: 'Generate optimized Dockerfile using TypeScript templates',
    handler: (args) => {
      const params: DockerfilePromptParams = {
        language: String(args.language || 'unknown'),
        optimization: Boolean(args.optimization),
        securityHardening: Boolean(args.securityHardening),
        multistage: Boolean(args.multistage),
      };
      if (args.framework) params.framework = String(args.framework);
      if (args.dependencies)
        params.dependencies = Array.isArray(args.dependencies)
          ? args.dependencies.map(String)
          : [String(args.dependencies)];
      if (args.ports)
        params.ports = Array.isArray(args.ports) ? args.ports.map(Number) : [Number(args.ports)];
      if (args.requirements) params.requirements = String(args.requirements);
      if (args.baseImage) params.baseImage = String(args.baseImage);

      const validation = validatePromptParams(params, ['language']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.dockerfile(params);
      const contextObj: Parameters<typeof buildAIPrompt>[1] = {};
      if (args.projectType) contextObj.projectType = String(args.projectType);
      if (args.existingFiles && Array.isArray(args.existingFiles))
        contextObj.existingFiles = args.existingFiles as string[];
      if (args.constraints && Array.isArray(args.constraints))
        contextObj.constraints = args.constraints as string[];
      const fullPrompt = buildAIPrompt(promptContent, contextObj);

      return Success({
        description: 'Generate Dockerfile',
        messages: createPromptMessages(fullPrompt),
      });
    },
  },

  'dockerfile-direct-analysis': {
    description: 'Generate Dockerfile based on direct repository analysis',
    handler: (args) => {
      const params: DockerfilePromptParams = {
        language: String(args.language || 'unknown'),
        optimization: true,
        securityHardening: true,
        multistage: Boolean(args.multistage !== false),
      };
      if (args.framework) params.framework = String(args.framework);
      if (args.dependencies)
        params.dependencies = Array.isArray(args.dependencies) ? args.dependencies.map(String) : [];
      if (args.ports) params.ports = Array.isArray(args.ports) ? args.ports.map(Number) : [];
      if (args.baseImage) params.baseImage = String(args.baseImage);
      if (args.analysisContext) params.requirements = String(args.analysisContext);

      const validation = validatePromptParams(params, ['language']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.dockerfile(params);
      const contextObj: Parameters<typeof buildAIPrompt>[1] = {};
      if (args.projectType) contextObj.projectType = String(args.projectType);
      if (args.fileList && Array.isArray(args.fileList))
        contextObj.existingFiles = args.fileList as string[];
      const fullPrompt = buildAIPrompt(promptContent, contextObj);

      return Success({
        description: 'Generate Dockerfile from direct analysis',
        messages: createPromptMessages(fullPrompt),
      });
    },
  },

  'fix-dockerfile': {
    description: 'Fix issues in existing Dockerfile',
    handler: (args) => {
      const content = String(args.currentContent || args.content || '');
      const issues = args.issues
        ? Array.isArray(args.issues)
          ? args.issues.map(String)
          : [String(args.issues)]
        : [];

      if (!content) {
        return Failure('Missing required parameter: content');
      }

      const promptContent = promptTemplates.fix('dockerfile', content, issues);
      return Success({
        description: 'Fix Dockerfile issues',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'optimize-dockerfile': {
    description: 'Optimize existing Dockerfile',
    handler: (args) => {
      const params: OptimizationPromptParams = {
        currentContent: String(args.currentContent || args.content || ''),
        contentType: 'dockerfile',
      };
      if (args.issues) params.issues = args.issues as string[];
      if (args.requirements) params.requirements = String(args.requirements);

      const validation = validatePromptParams(params, ['currentContent']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.optimization(params);
      return Success({
        description: 'Optimize Dockerfile',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Kubernetes prompts
  'generate-k8s-manifests': {
    description: 'Generate Kubernetes manifests',
    handler: (args) => {
      const params: K8sManifestPromptParams = {
        appName: String(args.appName || args.name || 'app'),
        image: String(args.image || args.imageName || 'app:latest'),
        ingressEnabled: Boolean(args.ingressEnabled),
        healthCheck: Boolean(args.healthCheck !== false),
      };
      if (args.replicas !== undefined) params.replicas = Number(args.replicas);
      if (args.port !== undefined) params.port = Number(args.port);
      if (args.namespace) params.namespace = String(args.namespace);
      if (args.serviceType) params.serviceType = String(args.serviceType);
      if (args.resources) params.resources = args.resources as { cpu?: string; memory?: string };

      const validation = validatePromptParams(params, ['appName', 'image']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.k8sManifests(params);
      return Success({
        description: 'Generate Kubernetes manifests',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'k8s-fix': {
    description: 'Fix issues in Kubernetes manifests',
    handler: (args) => {
      const content = String(args.currentContent || args.content || '');
      const issues = args.issues
        ? Array.isArray(args.issues)
          ? args.issues.map(String)
          : [String(args.issues)]
        : [];

      if (!content) {
        return Failure('Missing required parameter: content');
      }

      const promptContent = promptTemplates.fix('kubernetes', content, issues);
      return Success({
        description: 'Fix Kubernetes manifest issues',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Helm prompts
  'generate-helm-charts': {
    description: 'Generate Helm charts',
    handler: (args) => {
      const params: HelmChartPromptParams = {
        appName: String(args.appName || args.name || 'app'),
      };
      if (args.description) params.description = String(args.description);
      if (args.version) params.version = String(args.version);
      if (args.dependencies) params.dependencies = args.dependencies as string[];
      if (args.values) params.values = args.values as Record<string, any>;

      const validation = validatePromptParams(params, ['appName']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.helmChart(params);
      return Success({
        description: 'Generate Helm charts',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Analysis prompts
  'repository-analysis': {
    description: 'AI-powered language and framework detection',
    handler: (args) => {
      const params: RepositoryAnalysisParams = {
        fileList: String(args.fileList || ''),
        configFiles: String(args.configFiles || ''),
        directoryTree: String(args.directoryTree || ''),
      };

      const validation = validatePromptParams(params, ['fileList', 'configFiles', 'directoryTree']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.repositoryAnalysis(params);
      return Success({
        description: 'Analyze repository',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'security-analysis': {
    description: 'Analyze container security',
    handler: (args) => {
      const params: SecurityAnalysisParams = {};
      if (args.dockerfileContent) params.dockerfileContent = String(args.dockerfileContent);
      if (args.imageId) params.imageId = String(args.imageId);
      if (args.scanResults) params.scanResults = args.scanResults;

      const promptContent = promptTemplates.securityAnalysis(params);
      return Success({
        description: 'Security analysis',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'base-image-resolution': {
    description: 'Resolve optimal base images',
    handler: (args) => {
      const params: BaseImageResolutionParams = {
        language: String(args.language || 'unknown'),
      };
      if (args.framework) params.framework = String(args.framework);
      if (args.version) params.version = String(args.version);
      if (args.requirements) {
        params.requirements = Array.isArray(args.requirements)
          ? args.requirements.map(String)
          : [String(args.requirements)];
      }

      const validation = validatePromptParams(params, ['language']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.baseImageResolution(params);
      return Success({
        description: 'Resolve base images',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Azure Container Apps
  'generate-aca-manifests': {
    description: 'Generate Azure Container Apps manifests',
    handler: (args) => {
      const params: AcaManifestParams = {
        appName: String(args.appName || args.name || 'app'),
        image: String(args.image || args.imageName || 'app:latest'),
      };
      if (args.environment) params.environment = String(args.environment);
      if (args.resources) params.resources = args.resources as { cpu?: string; memory?: string };
      if (args.scaling)
        params.scaling = args.scaling as { minReplicas?: number; maxReplicas?: number };

      const validation = validatePromptParams(params, ['appName', 'image']);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.acaManifests(params);
      return Success({
        description: 'Generate ACA manifests',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'convert-aca-to-k8s': {
    description: 'Convert Azure Container Apps to Kubernetes',
    handler: (args) => {
      const acaConfig = String(args.acaConfig || args.config || '');
      if (!acaConfig) {
        return Failure('Missing required parameter: acaConfig');
      }

      const promptContent = promptTemplates.convertAcaToK8s(acaConfig);
      return Success({
        description: 'Convert ACA to K8s',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Parameter and validation
  'parameter-suggestions': {
    description: 'Suggest missing parameter values',
    handler: (args) => {
      const params: ParameterSuggestionParams = {
        toolName: String(args.toolName || 'unknown'),
        currentParams: (args.currentParams || {}) as Record<string, unknown>,
        missingParams: Array.isArray(args.missingParams) ? args.missingParams.map(String) : [],
      };
      if (args.context) params.context = args.context as Record<string, unknown>;

      const validation = validatePromptParams(params, [
        'toolName',
        'currentParams',
        'missingParams',
      ]);
      if (!validation.ok) return validation;

      const promptContent = promptTemplates.parameterSuggestions(params);
      return Success({
        description: 'Parameter suggestions',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Sampling
  'dockerfile-sampling': {
    description: 'Generate Dockerfile variations for sampling',
    handler: (args) => {
      const requirements = args.requirements ? String(args.requirements) : undefined;
      const promptContent = promptTemplates.samplingStrategy('Dockerfile', requirements);
      return Success({
        description: 'Dockerfile sampling',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  'strategy-optimization': {
    description: 'Optimize generation strategy',
    handler: (args) => {
      const contentType = String(args.contentType || 'configuration');
      const requirements = args.requirements ? String(args.requirements) : undefined;
      const promptContent = promptTemplates.samplingStrategy(contentType, requirements);
      return Success({
        description: 'Strategy optimization',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Utilities
  'json-repair': {
    description: 'Repair invalid JSON',
    handler: (args) => {
      const invalidJson = String(args.json || args.content || '');
      const error = String(args.error || 'Unknown error');

      if (!invalidJson) {
        return Failure('Missing required parameter: json');
      }

      const promptContent = promptTemplates.jsonRepair(invalidJson, error);
      return Success({
        description: 'Repair JSON',
        messages: createPromptMessages(promptContent),
      });
    },
  },

  // Add aliases for compatibility
  'enhance-repo-analysis': {
    description: 'Enhanced repository analysis',
    handler: (args) =>
      PROMPT_HANDLERS['repository-analysis']?.handler(args) || Failure('Handler not found'),
  },
  'resolve-base-images': {
    description: 'Alias for base-image-resolution',
    handler: (args) =>
      PROMPT_HANDLERS['base-image-resolution']?.handler(args) || Failure('Handler not found'),
  },
  'k8s-generation': {
    description: 'Alternative K8s generation prompt',
    handler: (args) =>
      PROMPT_HANDLERS['generate-k8s-manifests']?.handler(args) || Failure('Handler not found'),
  },
  'k8s-manifest-generation': {
    description: 'Generate K8s manifests (alias)',
    handler: (args) =>
      PROMPT_HANDLERS['generate-k8s-manifests']?.handler(args) || Failure('Handler not found'),
  },
  'dockerfile-fix': {
    description: 'Fix Dockerfile issues (alias)',
    handler: (args) =>
      PROMPT_HANDLERS['fix-dockerfile']?.handler(args) || Failure('Handler not found'),
  },
  'generate-dockerfile': {
    description: 'Generate Dockerfile (alias)',
    handler: (args) =>
      PROMPT_HANDLERS['dockerfile-generation']?.handler(args) || Failure('Handler not found'),
  },
  'parameter-validation': {
    description: 'Validate parameters',
    handler: (args) =>
      PROMPT_HANDLERS['parameter-suggestions']?.handler(args) || Failure('Handler not found'),
  },
};

/**
 * Set the logger for the prompt registry
 * @param parentLogger - Logger instance for prompt operations
 */
export function setLogger(parentLogger: Logger): void {
  logger = parentLogger.child({ component: 'PromptRegistry' });
  const promptCount = Object.keys(PROMPT_HANDLERS).length;
  logger.info({ promptCount }, 'TypeScript prompt registry initialized');
}

/**
 * List all available prompts
 */
export async function listPrompts(category?: string): Promise<ListPromptsResult> {
  const prompts = Object.entries(PROMPT_HANDLERS).map(([name, config]) => ({
    name,
    description: config.description,
    arguments: [], // TypeScript prompts handle their own validation
  }));

  if (logger) {
    logger.debug({ category, totalPrompts: prompts.length }, 'Listed TypeScript prompts');
  }

  return { prompts };
}

/**
 * Get a specific prompt
 */
export async function getPrompt(
  name: string,
  args?: Record<string, unknown>,
): Promise<GetPromptResult> {
  const prompt = PROMPT_HANDLERS[name];
  if (!prompt) {
    throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
  }

  const result = prompt.handler(args || {});
  if (!result.ok) {
    throw new McpError(ErrorCode.InvalidRequest, result.error);
  }

  if (logger) {
    logger.debug(
      { name, messageCount: result.value.messages.length },
      'Generated TypeScript prompt',
    );
  }

  return {
    name,
    description: prompt.description,
    arguments: [],
    messages: result.value.messages,
  };
}

/**
 * Get prompt with messages in ToolContext-compatible format
 */
export async function getPromptWithMessages(
  name: string,
  args?: Record<string, unknown>,
): Promise<{
  description: string;
  messages: Array<{ role: 'user' | 'assistant'; content: Array<{ type: 'text'; text: string }> }>;
}> {
  const prompt = PROMPT_HANDLERS[name];
  if (!prompt) {
    throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
  }

  const result = prompt.handler(args || {});
  if (!result.ok) {
    throw new McpError(ErrorCode.InvalidRequest, result.error);
  }

  // Convert PromptMessage format to the expected format
  const messages = result.value.messages.map((msg) => ({
    role: msg.role,
    content: Array.isArray(msg.content)
      ? msg.content
          .filter((c: any) => c.type === 'text')
          .map((c: any) => ({ type: 'text' as const, text: c.text }))
      : [{ type: 'text' as const, text: (msg.content as any).text }],
  }));

  return {
    description: result.value.description,
    messages,
  };
}

/**
 * Check if a prompt exists
 */
export function hasPrompt(name: string): boolean {
  return name in PROMPT_HANDLERS;
}

/**
 * Get all prompts (for testing/debugging)
 */
export function getAllPrompts(): string[] {
  return Object.keys(PROMPT_HANDLERS);
}
