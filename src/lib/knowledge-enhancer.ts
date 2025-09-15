/**
 * Simplified knowledge enhancer for AI prompts
 */
import { createLogger } from '@lib/logger';
import { getEntriesByCategoryEnhanced } from '@/knowledge/enhanced-loader';
import { scoreEntry, rankMatches } from '@/knowledge/simple-scoring';
import type { KnowledgeMatch } from '@/knowledge/types';

const logger = createLogger().child({ module: 'knowledge-enhancer' });

export interface EnhancementContext {
  operation: string;
  language?: string;
  framework?: string;
  environment?: string;
  content?: string;
}

/**
 * Enhance prompt arguments with relevant knowledge
 */
export async function enhanceWithKnowledge(
  promptArgs: Record<string, unknown>,
  context: EnhancementContext,
): Promise<Record<string, unknown>> {
  try {
    // Determine category based on operation
    const category = getCategory(context.operation);
    if (!category) {
      return promptArgs;
    }

    // Load relevant knowledge entries
    const entries = await getEntriesByCategoryEnhanced(category, {
      ...(context.language && { language: context.language }),
      ...(context.framework && { framework: context.framework }),
    });

    if (entries.length === 0) {
      return promptArgs;
    }

    // Score entries
    const matches: KnowledgeMatch[] = [];
    for (const entry of entries) {
      const { score, reasons } = scoreEntry(entry, {
        text: context.content || '',
        ...(context.language && { language: context.language }),
        ...(context.framework && { framework: context.framework }),
        ...(context.environment && { environment: context.environment }),
      });

      if (score > 30) {
        // Minimum threshold
        matches.push({ entry, score, reasons });
      }
    }

    // Get top matches
    const topMatches = rankMatches(matches, 5);

    if (topMatches.length === 0) {
      return promptArgs;
    }

    // Add knowledge to prompt args
    return {
      ...promptArgs,
      knowledgeRecommendations: topMatches.map((m) => ({
        recommendation: m.entry.recommendation,
        example: m.entry.example,
        severity: m.entry.severity,
      })),
      bestPractices: topMatches.filter((m) => m.score > 50).map((m) => m.entry.recommendation),
    };
  } catch (error) {
    logger.warn({ error }, 'Failed to enhance with knowledge');
    return promptArgs;
  }
}

/**
 * Get category for operation
 */
function getCategory(operation: string): string | null {
  const categoryMap: Record<string, string> = {
    generate_dockerfile: 'dockerfile',
    fix_dockerfile: 'dockerfile',
    analyze_repo: 'dockerfile',
    generate_k8s_manifests: 'kubernetes',
    generate_aca_manifests: 'kubernetes',
    convert_aca_to_k8s: 'kubernetes',
    scan: 'security',
  };

  return categoryMap[operation] || null;
}
