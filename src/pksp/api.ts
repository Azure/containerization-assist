/**
 * PKSP API - Unified interface for Prompts, Knowledge, Strategies, and Policies
 * This is the main integration point for the router and tools
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import { PKSPLoader, type EffectiveConfig } from './loader';
import { prompts } from '@/prompts/prompt-registry';
import { knowledge } from '@/knowledge/api';
import { getPolicyEngine, type PolicyEngine, type ValidationContext } from '@/policies/enforcement';
import type { Strategy } from '@/strategies/schema';
import type { Policy } from '@/policies/schema';

/**
 * Tool execution context with PKSP enhancements
 */
export interface PKSPContext {
  tool: string;
  parameters: Record<string, unknown>;
  strategy?: string;
  policies?: string[];
  skipPolicyEnforcement?: boolean;
}

/**
 * Enhanced execution result with telemetry
 */
export interface PKSPExecutionResult<T = unknown> {
  result: Result<T>;
  config: EffectiveConfig;
  telemetry: {
    promptTokens?: number;
    completionTokens?: number;
    totalTokens?: number;
    cost?: number;
    durationMs: number;
  };
  violations?: Array<{
    policyId: string;
    field: string;
    reason: string;
  }>;
}

/**
 * PKSP API - Main interface for router integration
 */
export class PKSPAPI {
  private loader: PKSPLoader;
  private policyEngine: PolicyEngine;
  private logger?: Logger;
  private initialized = false;

  constructor(logger?: Logger) {
    if (logger) {
      this.logger = logger.child({ component: 'PKSPAPI' });
    }
    this.loader = new PKSPLoader('src/strategies', 'src/policies');
    this.policyEngine = getPolicyEngine(this.logger);
  }

  /**
   * Initialize all PKSP components
   */
  async initialize(): Promise<Result<void>> {
    try {
      // Initialize prompts
      await this.loader.loadPrompts();

      // Initialize knowledge
      const knowledgeResult = await this.loader.loadKnowledge();
      if (!knowledgeResult.ok) {
        return knowledgeResult;
      }

      // Pre-load default strategies and policies
      await this.loader.loadStrategies();
      await this.loader.loadPolicies();

      this.initialized = true;
      this.logger?.info('PKSP API initialized successfully');

      return Success(undefined);
    } catch (error) {
      const message = `Failed to initialize PKSP API: ${error}`;
      this.logger?.error({ error }, message);
      return Failure(message);
    }
  }

  /**
   * Get effective configuration for a tool with policy enforcement
   */
  async getToolConfig(context: PKSPContext): Promise<Result<EffectiveConfig>> {
    if (!this.initialized) {
      const initResult = await this.initialize();
      if (!initResult.ok) {
        return initResult;
      }
    }

    try {
      // Get merged config from loader
      const configResult = await this.loader.getMergedConfig(context.tool);
      if (!configResult.ok) {
        return configResult;
      }

      let config = configResult.value;

      // Apply policy enforcement unless skipped
      if (!context.skipPolicyEnforcement) {
        const policiesResult = await this.loader.loadPolicies();
        if (policiesResult.ok) {
          const validationContext: ValidationContext = {
            tool: context.tool,
            parameters: context.parameters,
          };

          const enforcement = this.policyEngine.enforce(
            config,
            policiesResult.value,
            validationContext,
          );

          if (enforcement.violations.length > 0) {
            this.logger?.warn(
              { violations: enforcement.violations, tool: context.tool },
              'Policy violations enforced',
            );
          }

          config = enforcement.config;
        }
      }

      return Success(config);
    } catch (error) {
      return Failure(`Failed to get tool config: ${error}`);
    }
  }

  /**
   * Render a prompt with parameters and knowledge
   */
  async renderPrompt(
    promptId: string,
    parameters: Record<string, unknown>,
    toolName?: string,
  ): Promise<Result<string>> {
    if (!this.initialized) {
      const initResult = await this.initialize();
      if (!initResult.ok) {
        return initResult;
      }
    }

    try {
      // Get relevant knowledge for the tool
      const enrichedParams = { ...parameters };

      if (toolName) {
        const toolKnowledge = knowledge.forTool(toolName);

        // Add relevant snippets to parameters
        if (toolKnowledge.snippets.length > 0) {
          enrichedParams.knowledge = toolKnowledge.snippets
            .slice(0, 3) // Limit to top 3 snippets
            .map((s) => ({
              title: s.title,
              data: s.data,
            }));
        }

        // Add relevant documentation excerpts
        if (toolKnowledge.documents.length > 0) {
          enrichedParams.documentation = toolKnowledge.documents
            .slice(0, 2) // Limit to top 2 docs
            .map((d) => ({
              title: d.title,
              excerpt: d.content.substring(0, 500),
            }));
        }
      }

      // Render the prompt
      const result = await prompts.render(promptId, enrichedParams);
      return result;
    } catch (error) {
      return Failure(`Failed to render prompt: ${error}`);
    }
  }

  /**
   * Validate parameters against policies
   */
  async validateParameters(
    parameters: Record<string, unknown>,
    context: PKSPContext,
  ): Promise<Result<boolean>> {
    if (!this.initialized) {
      const initResult = await this.initialize();
      if (!initResult.ok) {
        return initResult;
      }
    }

    try {
      const policiesResult = await this.loader.loadPolicies();
      if (!policiesResult.ok) {
        return policiesResult;
      }

      const validationContext: ValidationContext = {
        tool: context.tool,
        parameters,
      };

      const result = this.policyEngine.validateConstraints(
        parameters,
        policiesResult.value,
        validationContext,
      );

      if (!result.ok) {
        this.logger?.warn(
          { violations: result.error, tool: context.tool },
          'Parameter validation failed',
        );
        return Failure(`Policy violations: ${result.error}`);
      }

      return Success(true);
    } catch (error) {
      return Failure(`Failed to validate parameters: ${error}`);
    }
  }

  /**
   * Get all PKSP components for a tool
   */
  async getToolPKSP(toolName: string): Promise<
    Result<{
      prompts: Array<{ id: string; description: string }>;
      knowledge: {
        snippets: Array<{ id: string; title: string }>;
        documents: Array<{ id: string; title: string }>;
      };
      strategy: Strategy | null;
      policies: Policy[];
      config: EffectiveConfig;
    }>
  > {
    if (!this.initialized) {
      const initResult = await this.initialize();
      if (!initResult.ok) {
        return initResult;
      }
    }

    try {
      // Get prompts for the tool category
      const toolCategory = this.getToolCategory(toolName);
      const availablePrompts = await prompts.list(toolCategory);

      // Get knowledge
      const toolKnowledge = knowledge.forTool(toolName);

      // Get strategy
      let strategy: Strategy | null = null;
      const strategyId = this.getToolStrategyId(toolName);
      if (strategyId) {
        const strategyResult = await this.loader.loadStrategy(strategyId);
        if (strategyResult.ok) {
          strategy = strategyResult.value;
        }
      }

      // Get policies
      const policiesResult = await this.loader.loadPolicies();
      const policies = policiesResult.ok ? policiesResult.value : [];

      // Get effective config
      const configResult = await this.loader.getMergedConfig(toolName);
      const config = configResult.ok
        ? configResult.value
        : {
            limits: { maxTokens: 8192, maxCost: 1.0, maxTimeMs: 300000 },
            security: { forbiddenPatterns: [], requireSecurityScan: false },
          };

      return Success({
        prompts: availablePrompts.map((p) => ({
          id: p.id,
          description: p.description,
        })),
        knowledge: {
          snippets: toolKnowledge.snippets.map((s) => ({
            id: s.id,
            title: s.title,
          })),
          documents: toolKnowledge.documents.map((d) => ({
            id: d.id,
            title: d.title,
          })),
        },
        strategy,
        policies,
        config,
      });
    } catch (error) {
      return Failure(`Failed to get tool PKSP: ${error}`);
    }
  }

  /**
   * Route and execute with PKSP enhancements
   */
  async routeAndExecute<T = unknown>(
    context: PKSPContext,
    executor: (config: EffectiveConfig) => Promise<Result<T>>,
  ): Promise<PKSPExecutionResult<T>> {
    const startTime = Date.now();

    // Get effective config
    const configResult = await this.getToolConfig(context);
    if (!configResult.ok) {
      return {
        result: configResult,
        config: {
          limits: { maxTokens: 8192, maxCost: 1.0, maxTimeMs: 300000 },
          security: { forbiddenPatterns: [], requireSecurityScan: false },
        },
        telemetry: {
          durationMs: Date.now() - startTime,
        },
      };
    }

    const config = configResult.value;

    // Validate parameters
    const validationResult = await this.validateParameters(context.parameters, context);

    let violations: PKSPExecutionResult['violations'];
    if (!validationResult.ok) {
      // Extract violations from error message
      violations = [
        {
          policyId: 'validation',
          field: 'parameters',
          reason: validationResult.error,
        },
      ];
    }

    // Execute with config
    const result = await executor(config);

    return {
      result,
      config,
      telemetry: {
        durationMs: Date.now() - startTime,
        // These would be populated by the actual executor
        promptTokens: undefined,
        completionTokens: undefined,
        totalTokens: undefined,
        cost: undefined,
      },
      violations,
    };
  }

  // Helper methods

  private getToolCategory(toolName: string): string {
    const categories: Record<string, string> = {
      'generate-dockerfile': 'containerization',
      'fix-dockerfile': 'containerization',
      'build-image': 'containerization',
      'generate-k8s-manifests': 'orchestration',
      deploy: 'orchestration',
      scan: 'security',
    };
    return categories[toolName] ?? 'general';
  }

  private getToolStrategyId(toolName: string): string | undefined {
    if (toolName.includes('docker') || toolName.includes('image')) {
      return 'docker';
    }
    if (toolName.includes('k8s') || toolName.includes('kubernetes')) {
      return 'k8s';
    }
    return 'default';
  }
}

/**
 * Global PKSP API instance
 */
let apiInstance: PKSPAPI | null = null;

/**
 * Get or create PKSP API instance
 */
export function getPKSPAPI(logger?: Logger): PKSPAPI {
  if (!apiInstance) {
    apiInstance = new PKSPAPI(logger);
  }
  return apiInstance;
}

/**
 * Export convenience functions for direct use
 */
export const pksp = {
  /**
   * Initialize PKSP system
   */
  async initialize(logger?: Logger): Promise<Result<void>> {
    return getPKSPAPI(logger).initialize();
  },

  /**
   * Get tool configuration
   */
  async getConfig(context: PKSPContext, logger?: Logger): Promise<Result<EffectiveConfig>> {
    return getPKSPAPI(logger).getToolConfig(context);
  },

  /**
   * Render prompt with knowledge
   */
  async render(
    promptId: string,
    params: Record<string, unknown>,
    toolName?: string,
    logger?: Logger,
  ): Promise<Result<string>> {
    return getPKSPAPI(logger).renderPrompt(promptId, params, toolName);
  },

  /**
   * Validate parameters
   */
  async validate(
    params: Record<string, unknown>,
    context: PKSPContext,
    logger?: Logger,
  ): Promise<Result<boolean>> {
    return getPKSPAPI(logger).validateParameters(params, context);
  },

  /**
   * Get all PKSP data for a tool
   */
  async getToolData(toolName: string, logger?: Logger) {
    return getPKSPAPI(logger).getToolPKSP(toolName);
  },

  /**
   * Route and execute with PKSP
   */
  async execute<T = unknown>(
    context: PKSPContext,
    executor: (config: EffectiveConfig) => Promise<Result<T>>,
    logger?: Logger,
  ): Promise<PKSPExecutionResult<T>> {
    return getPKSPAPI(logger).routeAndExecute(context, executor);
  },
};
