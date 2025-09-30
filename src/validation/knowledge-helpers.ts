/**
 * Knowledge Enhancement Helper Functions
 *
 * Provides convenient wrapper functions for integrating knowledge enhancement
 * across tools with consistent patterns and error handling
 */

import type { ToolContext } from '@/mcp/context';
import type { ValidationReport, ValidationResult } from './core-types';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';
import { Success, type Result } from '@/types';

/**
 * Helper to integrate knowledge enhancement in tools when validation score is low
 */
export async function enhanceContentIfNeeded(
  content: string,
  validationReport: ValidationReport,
  ctx: ToolContext,
  options: {
    scoreThreshold?: number;
    context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement';
    targetImprovement?: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
    userQuery?: string;
  },
): Promise<Result<KnowledgeEnhancementResult | null>> {
  const threshold = options.scoreThreshold || 90;

  if (validationReport.score >= threshold) {
    return Success(null);
  }

  try {
    const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
      '@/mcp/ai/knowledge-enhancement'
    );

    const enhancementRequest = createEnhancementFromValidation(
      content,
      options.context,
      validationReport.results
        .filter((r) => !r.passed)
        .map((r) => ({
          message: r.message || 'Validation issue',
          severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
          category: r.ruleId?.split('-')[0] || 'general',
        })),
      options.targetImprovement || 'all',
    );

    // Add user query if provided
    if (options.userQuery) {
      enhancementRequest.userQuery = options.userQuery;
    }

    const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

    if (enhancementResult.ok) {
      ctx.logger.info(
        {
          knowledgeAppliedCount: enhancementResult.value.knowledgeApplied.length,
          confidence: enhancementResult.value.confidence,
          enhancementAreas: enhancementResult.value.analysis.enhancementAreas.length,
        },
        'Knowledge enhancement applied successfully',
      );
      return Success(enhancementResult.value);
    } else {
      ctx.logger.warn(
        { error: enhancementResult.error },
        'Knowledge enhancement failed, continuing without enhancement',
      );
      return Success(null);
    }
  } catch (enhancementError) {
    ctx.logger.debug(
      {
        error:
          enhancementError instanceof Error ? enhancementError.message : String(enhancementError),
      },
      'Knowledge enhancement threw exception, continuing without enhancement',
    );
    return Success(null);
  }
}

/**
 * Helper to create knowledge enhancement metadata for tool responses
 */
export function createKnowledgeEnhancementMetadata(
  knowledgeEnhancement: KnowledgeEnhancementResult | null | undefined,
): Record<string, unknown> {
  if (!knowledgeEnhancement) {
    return {};
  }

  return {
    analysis: {
      enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas,
      confidence: knowledgeEnhancement.confidence,
      knowledgeApplied: knowledgeEnhancement.knowledgeApplied,
    },
    confidence: knowledgeEnhancement.confidence,
    suggestions: knowledgeEnhancement.suggestions,
  };
}

/**
 * Helper to create workflow hints with knowledge enhancement information
 */
export function createWorkflowHintsWithKnowledge(
  baseHints: { nextStep: string; message: string },
  knowledgeEnhancement: KnowledgeEnhancementResult | null | undefined,
): { nextStep: string; message: string } {
  if (!knowledgeEnhancement) {
    return baseHints;
  }

  const enhancementSuffix = ` Enhanced with ${knowledgeEnhancement.knowledgeApplied.length} knowledge improvements.`;

  return {
    ...baseHints,
    message: baseHints.message + enhancementSuffix,
  };
}

/**
 * Helper to determine if knowledge enhancement should be applied based on validation results
 */
export function shouldApplyKnowledgeEnhancement(
  validationReport: ValidationReport,
  options: {
    scoreThreshold?: number;
    requireErrors?: boolean;
    requireWarnings?: boolean;
  } = {},
): boolean {
  const { scoreThreshold = 90, requireErrors = false, requireWarnings = false } = options;

  // Check score threshold
  if (validationReport.score >= scoreThreshold) {
    return false;
  }

  // Check for specific requirement patterns
  if (requireErrors && validationReport.errors === 0) {
    return false;
  }

  if (requireWarnings && validationReport.warnings === 0) {
    return false;
  }

  return true;
}

/**
 * Helper to merge knowledge enhancement suggestions with existing tool suggestions
 */
export function mergeKnowledgeSuggestions(
  baseSuggestions: string[],
  knowledgeEnhancement: KnowledgeEnhancementResult | null | undefined,
): string[] {
  if (!knowledgeEnhancement) {
    return baseSuggestions;
  }

  // Combine base suggestions with knowledge suggestions
  const combinedSuggestions = [
    ...baseSuggestions,
    ...knowledgeEnhancement.suggestions.map((s) => `🧠 ${s}`),
  ];

  // Remove duplicates and limit total count
  return Array.from(new Set(combinedSuggestions)).slice(0, 10);
}

/**
 * Helper to create a standardized error message when knowledge enhancement fails
 */
export function createKnowledgeEnhancementError(
  error: unknown,
  fallbackMessage = 'Knowledge enhancement unavailable',
): string {
  if (error instanceof Error) {
    return `${fallbackMessage}: ${error.message}`;
  }
  return `${fallbackMessage}: ${String(error)}`;
}

/**
 * Helper to safely apply knowledge enhancement with comprehensive error handling
 */
export async function safelyEnhanceContent(
  content: string,
  context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement',
  validationResults: ValidationResult[],
  ctx: ToolContext,
  options: {
    targetImprovement?: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
    userQuery?: string;
    scoreThreshold?: number;
  } = {},
): Promise<{
  enhancedContent: string;
  knowledgeEnhancement: KnowledgeEnhancementResult | null;
}> {
  try {
    // Only attempt enhancement if there are validation issues
    const hasIssues = validationResults.some((r) => !r.passed || !r.isValid);

    if (!hasIssues) {
      return {
        enhancedContent: content,
        knowledgeEnhancement: null,
      };
    }

    const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
      '@/mcp/ai/knowledge-enhancement'
    );

    const enhancementRequest = createEnhancementFromValidation(
      content,
      context,
      validationResults
        .filter((r) => !r.passed || !r.isValid)
        .map((r) => ({
          message: r.message || r.errors?.join('; ') || 'Validation issue',
          severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
          category: r.ruleId?.split('-')[0] || 'general',
        })),
      options.targetImprovement || 'all',
    );

    if (options.userQuery) {
      enhancementRequest.userQuery = options.userQuery;
    }

    const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

    if (enhancementResult.ok) {
      ctx.logger.info(
        {
          originalContentLength: content.length,
          enhancedContentLength: enhancementResult.value.enhancedContent.length,
          knowledgeAppliedCount: enhancementResult.value.knowledgeApplied.length,
          confidence: enhancementResult.value.confidence,
        },
        'Content successfully enhanced with knowledge',
      );

      return {
        enhancedContent: enhancementResult.value.enhancedContent,
        knowledgeEnhancement: enhancementResult.value,
      };
    } else {
      ctx.logger.warn(
        { error: enhancementResult.error },
        'Knowledge enhancement failed, using original content',
      );

      return {
        enhancedContent: content,
        knowledgeEnhancement: null,
      };
    }
  } catch (error) {
    ctx.logger.error(
      { error: error instanceof Error ? error.message : String(error) },
      'Knowledge enhancement threw exception, using original content',
    );

    return {
      enhancedContent: content,
      knowledgeEnhancement: null,
    };
  }
}

/**
 * Type guard to check if a result includes knowledge enhancement
 */
export function hasKnowledgeEnhancement(
  result: unknown,
): result is { knowledgeEnhancement: KnowledgeEnhancementResult } {
  return (
    typeof result === 'object' &&
    result !== null &&
    'knowledgeEnhancement' in result &&
    result.knowledgeEnhancement !== null &&
    result.knowledgeEnhancement !== undefined
  );
}

/**
 * Helper to extract knowledge enhancement statistics for logging/reporting
 */
export function extractKnowledgeStats(
  knowledgeEnhancement: KnowledgeEnhancementResult | null | undefined,
): {
  hasEnhancement: boolean;
  knowledgeAppliedCount: number;
  confidence: number;
  enhancementAreasCount: number;
  suggestionsCount: number;
  processingTime?: number;
} {
  if (!knowledgeEnhancement) {
    return {
      hasEnhancement: false,
      knowledgeAppliedCount: 0,
      confidence: 0,
      enhancementAreasCount: 0,
      suggestionsCount: 0,
    };
  }

  return {
    hasEnhancement: true,
    knowledgeAppliedCount: knowledgeEnhancement.knowledgeApplied.length,
    confidence: knowledgeEnhancement.confidence,
    enhancementAreasCount: knowledgeEnhancement.analysis.enhancementAreas.length,
    suggestionsCount: knowledgeEnhancement.suggestions.length,
    processingTime: knowledgeEnhancement.metadata.processingTime ?? 0,
  };
}
