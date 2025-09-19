/**
 * Knowledge enhancer for AI prompts
 */
import { createLogger } from '@/lib/logger';
import { getEntriesByCategoryEnhanced as getEntriesByCategory } from '@/knowledge/pack-loader';
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
    const entries = await getEntriesByCategory(category, {
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
      knowledgeRecommendations: topMatches.map((m: KnowledgeMatch) => ({
        recommendation: m.entry.recommendation,
        example: m.entry.example,
        severity: m.entry.severity,
      })),
      bestPractices: topMatches
        .filter((m: KnowledgeMatch) => m.score > 50)
        .map((m: KnowledgeMatch) => m.entry.recommendation),
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
    'generate-dockerfile': 'dockerfile',
    'fix-dockerfile': 'dockerfile',
    'analyze-repo': 'dockerfile',
    'generate-k8s-manifests': 'kubernetes',
    'generate-aca-manifests': 'kubernetes',
    'convert-aca-to-k8s': 'kubernetes',
    scan: 'security',
  };

  return categoryMap[operation] || null;
}
