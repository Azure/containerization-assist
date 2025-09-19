import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
// Path utilities available
import { parse as parseYaml } from 'yaml';
import { Strategy, validateStrategy } from '@/strategies/schema';
import { Policy, validatePolicy } from '@/policies/schema';
import * as prompts from '@/prompts/prompt-registry';
import { knowledge, initializeKnowledge, type KnowledgeAPI } from '@/knowledge/api';
import type { Result } from '@types';

/**
 * Effective configuration after merging PKSP components
 */
export interface EffectiveConfig {
  // From Strategy
  temperature?: number;
  topP?: number;
  maxTokens?: number;
  timeoutMs?: number;

  // From Policy (hard limits)
  limits: {
    maxTokens: number;
    maxCost: number;
    maxTimeMs: number;
  };

  // Security requirements
  security: {
    forbiddenPatterns: string[];
    requireSecurityScan: boolean;
  };

  // Tool-specific config
  toolConfig?: Record<string, unknown>;
}

/**
 * Merge configuration with policy enforcement
 * Precedence: Policy > Strategy > Prompt > Knowledge
 */
export function mergeWithPolicyEnforcement(
  strategy: Strategy,
  policies: Policy[],
): EffectiveConfig {
  // Start with strategy defaults
  const config: EffectiveConfig = {
    temperature: strategy.parameters?.temperature,
    topP: strategy.parameters?.topP,
    maxTokens: strategy.parameters?.maxTokens,
    timeoutMs: strategy.timeouts?.totalMs,
    limits: {
      maxTokens: 8192,
      maxCost: 1.0,
      maxTimeMs: 300000,
    },
    security: {
      forbiddenPatterns: [],
      requireSecurityScan: false,
    },
  };

  // Apply policies (they override strategy)
  for (const policy of policies) {
    // Apply hard limits
    if (policy.limits) {
      if (policy.limits.maxTokens) {
        config.limits.maxTokens = Math.min(config.limits.maxTokens, policy.limits.maxTokens);
      }
      if (policy.limits.maxCost) {
        config.limits.maxCost = Math.min(config.limits.maxCost, policy.limits.maxCost);
      }
      if (policy.limits.maxTimeMs) {
        config.limits.maxTimeMs = Math.min(config.limits.maxTimeMs, policy.limits.maxTimeMs);
      }
    }

    // Apply security requirements
    if (policy.security) {
      if (policy.security.forbiddenPatterns) {
        config.security.forbiddenPatterns.push(...policy.security.forbiddenPatterns);
      }
      if (policy.security.requireSecurityScan) {
        config.security.requireSecurityScan = true;
      }
    }
  }

  // Clamp strategy values to policy limits
  if (config.maxTokens && config.limits.maxTokens) {
    config.maxTokens = Math.min(config.maxTokens, config.limits.maxTokens);
  }
  if (config.timeoutMs && config.limits.maxTimeMs) {
    config.timeoutMs = Math.min(config.timeoutMs, config.limits.maxTimeMs);
  }

  return config;
}

/**
 * PKSP Loader - Unified loading pipeline for Prompts, Knowledge, Strategies, and Policies
 */
export class PKSPLoader {
  private strategiesCache = new Map<string, Strategy>();
  private policiesCache = new Map<string, Policy>();
  private cacheTimestamp = 0;
  private readonly CACHE_TTL = 60000; // 1 minute
  private promptsInitialized = false;

  constructor(
    private readonly strategiesDir = 'src/strategies',
    private readonly policiesDir = 'src/policies',
  ) {}

  /**
   * Load all strategies from the strategies directory
   */
  async loadStrategies(): Promise<Result<Strategy[]>> {
    try {
      if (this.shouldRefreshCache()) {
        await this.refreshStrategiesCache();
      }

      const strategies = Array.from(this.strategiesCache.values());
      return { ok: true, value: strategies };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to load strategies: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Load a specific strategy by ID
   */
  async loadStrategy(id: string): Promise<Result<Strategy>> {
    try {
      if (this.shouldRefreshCache()) {
        await this.refreshStrategiesCache();
      }

      const strategy = this.strategiesCache.get(id);
      if (!strategy) {
        // Try loading from file
        const filePath = join(this.strategiesDir, `${id}.yaml`);
        const content = await readFile(filePath, 'utf-8');
        const data = parseYaml(content);
        const validated = validateStrategy(data);
        this.strategiesCache.set(id, validated);
        return { ok: true, value: validated };
      }

      return { ok: true, value: strategy };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to load strategy ${id}: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Load all policies from the policies directory
   */
  async loadPolicies(): Promise<Result<Policy[]>> {
    try {
      if (this.shouldRefreshCache()) {
        await this.refreshPoliciesCache();
      }

      const policies = Array.from(this.policiesCache.values());
      return { ok: true, value: policies };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to load policies: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Load a specific policy by ID
   */
  async loadPolicy(id: string): Promise<Result<Policy>> {
    try {
      if (this.shouldRefreshCache()) {
        await this.refreshPoliciesCache();
      }

      const policy = this.policiesCache.get(id);
      if (!policy) {
        // Try loading from file
        const filePath = join(this.policiesDir, `${id}.yaml`);
        const content = await readFile(filePath, 'utf-8');
        const data = parseYaml(content);
        const validated = validatePolicy(data);
        this.policiesCache.set(id, validated);
        return { ok: true, value: validated };
      }

      return { ok: true, value: policy };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to load policy ${id}: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Get prompts registry
   */
  async loadPrompts(): Promise<typeof prompts> {
    if (!this.promptsInitialized) {
      const promptsDir = 'src/prompts';
      await prompts.initializeRegistry(promptsDir);
      this.promptsInitialized = true;
    }
    return prompts;
  }

  /**
   * Load knowledge base
   */
  async loadKnowledge(): Promise<Result<KnowledgeAPI>> {
    try {
      const knowledgeDir = 'src/knowledge';
      const result = await initializeKnowledge(knowledgeDir);
      if (!result.ok) {
        return result;
      }
      return { ok: true, value: knowledge };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to load knowledge: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Get merged configuration for a specific tool
   * Precedence: Policy > Strategy > Prompt defaults > Knowledge
   */
  async getMergedConfig(toolName: string): Promise<Result<EffectiveConfig>> {
    try {
      // Load default strategy
      const strategyResult = await this.loadStrategy('default');
      if (!strategyResult.ok) {
        return strategyResult;
      }
      const strategy = strategyResult.value;

      // Load org policy
      const policyResult = await this.loadPolicy('org');
      if (!policyResult.ok) {
        return policyResult;
      }
      const policy = policyResult.value;

      // Check for tool-specific strategy
      const toolStrategyId = this.getToolStrategyId(toolName);
      let toolStrategy: Strategy | undefined;
      if (toolStrategyId) {
        const toolStrategyResult = await this.loadStrategy(toolStrategyId);
        if (toolStrategyResult.ok) {
          toolStrategy = toolStrategyResult.value;
        }
      }

      // Merge configuration with proper precedence
      const effectiveConfig: EffectiveConfig = {
        // From strategy (defaults)
        temperature: toolStrategy?.parameters?.temperature ?? strategy.parameters?.temperature,
        topP: toolStrategy?.parameters?.topP ?? strategy.parameters?.topP,
        maxTokens: toolStrategy?.parameters?.maxTokens ?? strategy.parameters?.maxTokens,
        timeoutMs: toolStrategy?.timeouts?.totalMs ?? strategy.timeouts?.totalMs,

        // From policy (hard limits - always enforced)
        limits: {
          maxTokens: policy.limits?.maxTokens ?? 8192,
          maxCost: policy.limits?.maxCost ?? 1.0,
          maxTimeMs: policy.limits?.maxTimeMs ?? 300000,
        },

        // Security requirements
        security: {
          forbiddenPatterns: policy.security?.forbiddenPatterns ?? [],
          requireSecurityScan: policy.security?.requireSecurityScan ?? false,
        },

        // Tool-specific config from policy
        toolConfig: policy.toolPolicies?.[toolName],
      };

      // Apply policy clamping to strategy values
      if (effectiveConfig.maxTokens && effectiveConfig.limits.maxTokens) {
        effectiveConfig.maxTokens = Math.min(
          effectiveConfig.maxTokens,
          effectiveConfig.limits.maxTokens,
        );
      }
      if (effectiveConfig.timeoutMs && effectiveConfig.limits.maxTimeMs) {
        effectiveConfig.timeoutMs = Math.min(
          effectiveConfig.timeoutMs,
          effectiveConfig.limits.maxTimeMs,
        );
      }

      return { ok: true, value: effectiveConfig };
    } catch (error) {
      return {
        ok: false,
        error: `Failed to get merged config for ${toolName}: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Determine which strategy to use for a tool
   */
  private getToolStrategyId(toolName: string): string | undefined {
    // Map tools to strategy categories
    const toolStrategyMap: Record<string, string> = {
      'generate-dockerfile': 'docker',
      'fix-dockerfile': 'docker',
      'build-image': 'docker',
      scan: 'docker',
      'tag-image': 'docker',
      'push-image': 'docker',
      'generate-k8s-manifests': 'k8s',
      'prepare-cluster': 'k8s',
      deploy: 'k8s',
      'verify-deploy': 'k8s',
    };

    return toolStrategyMap[toolName];
  }

  /**
   * Check if cache should be refreshed
   */
  private shouldRefreshCache(): boolean {
    return Date.now() - this.cacheTimestamp > this.CACHE_TTL;
  }

  /**
   * Refresh strategies cache
   */
  private async refreshStrategiesCache(): Promise<void> {
    // In production, this would scan the strategies directory
    // For now, we'll load known strategies
    const strategyFiles = ['default', 'docker', 'k8s'];

    for (const file of strategyFiles) {
      try {
        const filePath = join(this.strategiesDir, `${file}.yaml`);
        const content = await readFile(filePath, 'utf-8');
        const data = parseYaml(content);
        const validated = validateStrategy(data);
        this.strategiesCache.set(validated.id, validated);
      } catch {
        // Ignore missing files
      }
    }

    this.cacheTimestamp = Date.now();
  }

  /**
   * Refresh policies cache
   */
  private async refreshPoliciesCache(): Promise<void> {
    // In production, this would scan the policies directory
    // For now, we'll load known policies
    const policyFiles = ['org', 'security', 'performance'];

    for (const file of policyFiles) {
      try {
        const filePath = join(this.policiesDir, `${file}.yaml`);
        const content = await readFile(filePath, 'utf-8');
        const data = parseYaml(content);
        const validated = validatePolicy(data);
        this.policiesCache.set(validated.id, validated);
      } catch {
        // Ignore missing files
      }
    }

    this.cacheTimestamp = Date.now();
  }

  /**
   * Clear all caches
   */
  clearCache(): void {
    this.strategiesCache.clear();
    this.policiesCache.clear();
    this.cacheTimestamp = 0;
  }
}
