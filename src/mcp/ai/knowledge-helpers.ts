/**
 * Convenience functions for common knowledge enhancement patterns
 */

import {
  enhanceWithKnowledge,
  createEnhancementFromValidation,
  type KnowledgeEnhancementRequest,
  type KnowledgeEnhancementResult,
} from './knowledge-enhancement';
import type { ToolContext } from '@/mcp/context';
import type { ValidationResult } from '@/validation/core-types';
import { Result } from '@/types';

/**
 * Quick dockerfile enhancement from validation results
 */
export async function enhanceDockerfileFromValidation(
  dockerfileContent: string,
  validationResults: ValidationResult[],
  ctx: ToolContext,
  targetImprovement: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all' = 'all',
): Promise<Result<KnowledgeEnhancementResult>> {
  const request = createEnhancementFromValidation(
    dockerfileContent,
    'dockerfile',
    validationResults
      .filter((r) => !r.passed)
      .map((r) => ({
        message: r.message || 'Validation issue',
        severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
        category: r.ruleId?.split('-')[0] || 'general',
      })),
    targetImprovement,
  );

  return enhanceWithKnowledge(request, ctx);
}

/**
 * Quick kubernetes enhancement from validation results
 */
export async function enhanceKubernetesFromValidation(
  manifestsContent: string,
  validationResults: ValidationResult[],
  ctx: ToolContext,
  targetImprovement:
    | 'security'
    | 'performance'
    | 'best-practices'
    | 'enhancement'
    | 'all' = 'security',
): Promise<Result<KnowledgeEnhancementResult>> {
  const request = createEnhancementFromValidation(
    manifestsContent,
    'kubernetes',
    validationResults
      .filter((r) => !r.passed)
      .map((r) => ({
        message: r.message || 'Validation issue',
        severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
        category: r.ruleId?.split('-')[1] || 'general',
      })),
    targetImprovement,
  );

  return enhanceWithKnowledge(request, ctx);
}

/**
 * Security-focused enhancement for scan results
 */
export async function enhanceSecurityFromScan(
  content: string,
  scanResults: Array<{ description?: string; message?: string; severity?: string }>,
  ctx: ToolContext,
  userQuery?: string,
): Promise<Result<KnowledgeEnhancementResult>> {
  const request: KnowledgeEnhancementRequest = {
    content,
    context: 'security',
    targetImprovement: 'security',
    validationContext: scanResults.map((r) => ({
      message: r.description || r.message || 'Security vulnerability',
      severity: r.severity === 'CRITICAL' || r.severity === 'HIGH' ? 'error' : 'warning',
      category: 'security',
    })),
    ...(userQuery && { userQuery }),
  };

  return enhanceWithKnowledge(request, ctx);
}

/**
 * General content enhancement with custom context
 */
export async function enhanceContentWithContext(
  content: string,
  context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement',
  targetImprovement: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all',
  ctx: ToolContext,
  options?: {
    userQuery?: string;
    validationIssues?: Array<{
      message: string;
      severity: 'error' | 'warning';
      category: string;
    }>;
  },
): Promise<Result<KnowledgeEnhancementResult>> {
  const request: KnowledgeEnhancementRequest = {
    content,
    context,
    targetImprovement,
    ...(options?.validationIssues && { validationContext: options.validationIssues }),
    ...(options?.userQuery && { userQuery: options.userQuery }),
  };

  return enhanceWithKnowledge(request, ctx);
}

/**
 * Batch enhancement for multiple content pieces
 */
export async function enhanceMultipleContent(
  contentList: Array<{
    content: string;
    context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement';
    targetImprovement: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
    userQuery?: string;
  }>,
  ctx: ToolContext,
): Promise<Array<{ index: number; result: Result<KnowledgeEnhancementResult> }>> {
  const results = await Promise.allSettled(
    contentList.map(async (item, index) => {
      const request: KnowledgeEnhancementRequest = {
        content: item.content,
        context: item.context,
        targetImprovement: item.targetImprovement,
        ...(item.userQuery && { userQuery: item.userQuery }),
      };

      const result = await enhanceWithKnowledge(request, ctx);
      return { index, result };
    }),
  );

  return results.map((result, index) => {
    if (result.status === 'fulfilled') {
      return result.value;
    } else {
      return {
        index,
        result: {
          ok: false,
          error: `Enhancement failed: ${result.reason}`,
        } as Result<KnowledgeEnhancementResult>,
      };
    }
  });
}

/**
 * Enhancement with retry logic for transient failures
 */
export async function enhanceWithRetry(
  request: KnowledgeEnhancementRequest,
  ctx: ToolContext,
  maxRetries: number = 2,
): Promise<Result<KnowledgeEnhancementResult>> {
  let lastError: string = 'Unknown error';

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const result = await enhanceWithKnowledge(request, ctx);

      if (result.ok) {
        if (attempt > 0) {
          ctx.logger.info(
            { attempt: attempt + 1, maxRetries: maxRetries + 1 },
            'Knowledge enhancement succeeded after retry',
          );
        }
        return result;
      }

      lastError = result.error;

      if (attempt < maxRetries) {
        ctx.logger.warn(
          { attempt: attempt + 1, error: lastError },
          'Knowledge enhancement failed, retrying...',
        );
        // Wait briefly before retry
        await new Promise((resolve) => setTimeout(resolve, 1000 * (attempt + 1)));
      }
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);

      if (attempt < maxRetries) {
        ctx.logger.warn(
          { attempt: attempt + 1, error: lastError },
          'Knowledge enhancement threw exception, retrying...',
        );
        await new Promise((resolve) => setTimeout(resolve, 1000 * (attempt + 1)));
      }
    }
  }

  return {
    ok: false,
    error: `Knowledge enhancement failed after ${maxRetries + 1} attempts: ${lastError}`,
  };
}
