/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import * as path from 'node:path';
import * as fs from 'node:fs';
import { z, type ZodTypeAny } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { createToolContext, type ToolContext, type ProgressReporter } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { ERROR_MESSAGES } from '@/lib/error-messages';
import type {
  ToolOrchestrator,
  OrchestratorConfig,
  ExecuteRequest,
  ExecuteMetadata,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { MCPTool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { TOOL_NAME, ToolName } from '@/tools';
import { checkSamplingAvailability, type SamplingCheckResult } from '@/mcp/sampling-check';
import { logToolExecution, createToolLogEntry } from '@/lib/tool-logger';

// ===== Types =====

/**
 * Get default policy paths
 * Returns all .yaml files in the policies/ directory
 */
function getDefaultPolicyPaths(): string[] {
  try {
    const policiesDir = path.join(process.cwd(), 'policies');

    if (!fs.existsSync(policiesDir)) {
      return [];
    }

    const files = fs.readdirSync(policiesDir);
    return files
      .filter((f) => f.endsWith('.yaml') || f.endsWith('.yml'))
      .sort() // Sort alphabetically for consistency
      .map((f) => path.join(policiesDir, f));
  } catch {
    return [];
  }
}

function childLogger(logger: Logger, bindings: Record<string, unknown>): Logger {
  const candidate = (logger as unknown as { child?: (bindings: Record<string, unknown>) => Logger })
    .child;
  return typeof candidate === 'function' ? candidate.call(logger, bindings) : logger;
}

type ContextFactoryInput<T extends MCPTool<ZodTypeAny, any>> = {
  tool: T;
  request: ExecuteRequest;
  logger: Logger;
};

type ContextFactory<T extends MCPTool<ZodTypeAny, any>> = (
  input: ContextFactoryInput<T>,
) => Promise<ToolContext> | ToolContext;

interface ExecutionEnvironment<T extends MCPTool<ZodTypeAny, any>> {
  policy?: Policy;
  registry: Map<string, T>;
  logger: Logger;
  config: OrchestratorConfig;
  buildContext: ContextFactory<T>;
}

/**
 * Create a tool orchestrator
 */
export function createOrchestrator<T extends MCPTool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
  contextFactory?: ContextFactory<T>;
}): ToolOrchestrator {
  const { registry, config = { chainHintsMode: 'enabled' } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  // Load policies - use defaults if not configured
  let policy: Policy | undefined;
  const policyPaths = config.policyPath ? [config.policyPath] : getDefaultPolicyPaths();

  if (policyPaths.length > 0 && policyPaths[0]) {
    // For now, load the first policy found
    // TODO: Support merging multiple policies
    const policyResult = loadPolicy(policyPaths[0], config.policyEnvironment);
    if (policyResult.ok) {
      policy = policyResult.value;
      logger.debug({ policyPath: policyPaths[0] }, 'Policy loaded successfully');
    } else {
      logger.warn(`Failed to load policy from ${policyPaths[0]}: ${policyResult.error}`);
    }
  }

  const buildContext: ContextFactory<T> = async (input) => {
    if (options.contextFactory) {
      return options.contextFactory(input);
    }

    const metadata = input.request.metadata;

    if (options.server) {
      const contextOptions = {
        ...(metadata?.signal && { signal: metadata.signal }),
        ...(metadata?.progress !== undefined && { progress: metadata.progress }),
        ...(metadata?.maxTokens !== undefined && { maxTokens: metadata.maxTokens }),
        ...(metadata?.stopSequences && { stopSequences: metadata.stopSequences }),
        ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
      };
      return createToolContext(options.server, input.logger, contextOptions);
    }

    return createHostlessToolContext(input.logger, {
      ...(metadata && { metadata }),
    });
  };

  async function execute(request: ExecuteRequest): Promise<Result<unknown>> {
    const { toolName } = request;
    const tool = registry.get(toolName);

    if (!tool) {
      return Failure(ERROR_MESSAGES.TOOL_NOT_FOUND(toolName));
    }

    const contextualLogger = childLogger(logger, {
      tool: tool.name,
      ...(request.metadata?.loggerContext ?? {}),
    });

    return await executeWithOrchestration(tool, request, {
      registry,
      ...(policy && { policy }),
      logger: contextualLogger,
      config,
      buildContext,
    });
  }

  function close(): void {
    // No cleanup needed anymore
  }

  return { execute, close };
}

/**
 * Execute with full orchestration (dependencies, policies)
 */
async function executeWithOrchestration<T extends MCPTool<ZodTypeAny, any>>(
  tool: T,
  request: ExecuteRequest,
  env: ExecutionEnvironment<T>,
): Promise<Result<unknown>> {
  const { params } = request;
  const { policy, logger } = env;

  // Validate parameters using Zod safeParse
  const validation = validateParams(params, tool.schema);
  if (!validation.ok) return validation;
  const validatedParams = validation.value;

  // Apply policies
  if (policy) {
    const policyResults = applyPolicy(policy, {
      tool: tool.name,
      params: validatedParams as Record<string, unknown>,
    });

    const blockers = policyResults
      .filter((r) => r.matched && r.rule.actions.block)
      .map((r) => r.rule.id);

    if (blockers.length > 0) {
      return Failure(ERROR_MESSAGES.POLICY_BLOCKED(blockers));
    }
  }

  const toolContext = await env.buildContext({
    tool,
    request,
    logger,
  });
  const tracker = createStandardizedToolTracker(tool.name, {}, logger);

  // Check sampling availability for tools that use AI-driven sampling
  let samplingCheckResult: SamplingCheckResult | undefined;

  //Later on this can be cached in the session to avoid multiple checks
  // Avoiding caching to reduce complexity for now
  if (tool.metadata?.samplingStrategy === 'single') {
    samplingCheckResult = await checkSamplingAvailability(toolContext);
  }

  const startTime = Date.now();
  const logEntry = createToolLogEntry(tool.name, undefined, validatedParams);

  // Execute tool directly (single attempt)
  try {
    const result = await tool.run(validatedParams, toolContext);
    const durationMs = Date.now() - startTime;

    logEntry.output = result.ok ? result.value : { error: result.error };
    logEntry.success = result.ok;
    logEntry.durationMs = durationMs;
    if (!result.ok) {
      logEntry.error = result.error;
      if (result.guidance) {
        logEntry.errorGuidance = result.guidance;
      }
    }

    await logToolExecution(logEntry, logger);

    // Add metadata to successful results
    if (result.ok) {
      // Add sampling warning message if not available (only for sampling-enabled tools)
      let valueWithMessages = result.value;
      if (samplingCheckResult && !samplingCheckResult.available && samplingCheckResult.message) {
        valueWithMessages = {
          ...result.value,
          _samplingWarning: samplingCheckResult.message,
        };
      }

      if (env.config.chainHintsMode === 'enabled') {
        const hint = getChainHint(tool.name as ToolName, result.ok ? 'success' : 'failure');
        if (hint) {
          valueWithMessages = {
            ...valueWithMessages,
            nextSteps: hint,
          };
        }
      }

      result.value = valueWithMessages;
    }
    tracker.complete({});
    return result;
  } catch (error) {
    const durationMs = Date.now() - startTime;
    const errorMessage = (error as Error).message || 'Unknown error';

    logEntry.output = { error: errorMessage };
    logEntry.success = false;
    logEntry.durationMs = durationMs;
    logEntry.error = errorMessage;

    await logToolExecution(logEntry, logger);

    logger.error({ error: errorMessage }, 'Tool execution failed');
    tracker.fail(error as Error);
    return Failure(errorMessage);
  }
}

type outputState = 'success' | 'failure';
function getChainHint(toolName: ToolName, outputState: outputState): string {
  const stateToHints = chainHintMap.get(toolName);
  if (!stateToHints) {
    return '';
  }

  const hint = stateToHints[outputState];
  return hint || '';
}

const chainHintMap = new Map<ToolName, { success: string; failure: string }>([
  [
    TOOL_NAME.ANALYZE_REPO,
    {
      success: `Repository analysis completed successfully. Continue by calling the ${TOOL_NAME.FIX_DOCKERFILE} or ${TOOL_NAME.RESOLVE_BASE_IMAGES} tools for recommendations.`,
      failure: 'Repository analysis failed. Please check the logs for details.',
    },
  ],
  [
    TOOL_NAME.VALIDATE_DOCKERFILE,
    {
      success: `Dockerfile validated successfully. Continue by building this Dockerfile. Continue trying to build and fix until it builds successfully. Then call the ${TOOL_NAME.GENERATE_K8S_MANIFESTS} tool to proceed with containerization.`,
      failure:
        'Dockerfile validation failed. Please update the Dockerfile and retry this tool until it passes.',
    },
  ],
  [
    TOOL_NAME.BUILD_IMAGE,
    {
      success: `Image built successfully. Next: Call ${TOOL_NAME.SCAN} to check for security vulnerabilities.`,
      failure: `Image build failed. Use ${TOOL_NAME.FIX_DOCKERFILE} to resolve issues, then retry ${TOOL_NAME.BUILD_IMAGE}.`,
    },
  ],
  [
    TOOL_NAME.GENERATE_K8S_MANIFESTS,
    {
      success: `Kubernetes manifests generated successfully. Next: Call ${TOOL_NAME.PREPARE_CLUSTER} to create a kind cluster to deploy to.`,
      failure: 'Manifest generation failed. Ensure you have a valid image and try again.',
    },
  ],
  [
    TOOL_NAME.DEPLOY,
    {
      success: `Application deployed successfully. Use ${TOOL_NAME.VERIFY_DEPLOY} to check deployment health and status.`,
      failure:
        'Deployment failed. Check cluster connectivity, manifests validity, and pod status with kubectl.',
    },
  ],
  [
    TOOL_NAME.FIX_DOCKERFILE,
    {
      success: `Dockerfile fixes applied successfully. Next: Call ${TOOL_NAME.BUILD_IMAGE} to test the fixed Dockerfile.`,
      failure: 'Dockerfile fix failed. Review validation errors and try manual fixes.',
    },
  ],
  [
    TOOL_NAME.CONVERT_ACA_TO_K8S,
    {
      success: `ACA manifests converted to Kubernetes successfully. Next: Call ${TOOL_NAME.DEPLOY} to deploy the manifests.`,
      failure: 'Conversion failed. Verify ACA manifest syntax and try again.',
    },
  ],
  [
    TOOL_NAME.PUSH_IMAGE,
    {
      success: `Image pushed successfully. Review AI optimization insights for push improvements.`,
      failure:
        'Image push failed. Check registry credentials, network connectivity, and image tag format.',
    },
  ],
  [
    TOOL_NAME.SCAN,
    {
      success: `Security scan passed! Proceed with ${TOOL_NAME.PUSH_IMAGE} to push to a registry, or continue with deployment preparation.`,
      failure: `Security scan found vulnerabilities. Use ${TOOL_NAME.FIX_DOCKERFILE} to address security issues in your base images and dependencies.`,
    },
  ],
  [
    TOOL_NAME.PREPARE_CLUSTER,
    {
      success: `Cluster preparation successful. Next: Call ${TOOL_NAME.DEPLOY} to deploy to the kind cluster.`,
      failure:
        'Cluster preparation found issues. Check connectivity, permissions, and namespace configuration.',
    },
  ],
]);

/**
 * Validate parameters against schema using safeParse
 */
function validateParams<T extends z.ZodSchema>(params: unknown, schema: T): Result<z.infer<T>> {
  const parsed = schema.safeParse(params);
  if (!parsed.success) {
    const issues = parsed.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
    return Failure(ERROR_MESSAGES.VALIDATION_FAILED(issues));
  }
  return Success(parsed.data);
}

export function createHostlessToolContext(
  logger: Logger,
  options: {
    metadata?: ExecuteMetadata;
  },
): ToolContext {
  const progress = coerceProgressReporter(options.metadata?.progress);

  return {
    sampling: {
      async createMessage(): Promise<never> {
        throw new Error(
          'Sampling is unavailable without an MCP transport. Start or bind to an MCP server to enable sampling.',
        );
      },
    },
    getPrompt: async (name: string): Promise<never> => {
      throw new Error(
        `Prompt '${name}' requested but no MCP transport is bound. Start or bind to an MCP server to access prompts.`,
      );
    },
    logger,
    signal: options.metadata?.signal,
    progress,
  };
}

/**
 * Coerce a progress reporter function to the expected interface
 * @public
 */
export function coerceProgressReporter(progress: unknown): ProgressReporter | undefined {
  if (typeof progress !== 'function') {
    return undefined;
  }

  return async (message: string, current?: number, total?: number) => {
    const maybePromise = progress(message, current, total);
    if (maybePromise && typeof (maybePromise as Promise<void>).then === 'function') {
      await maybePromise;
    }
  };
}
