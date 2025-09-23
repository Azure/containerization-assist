import { enhancePromptWithKnowledge, type KnowledgeOperation } from '@/lib/ai-knowledge-enhancer';
import { createLogger } from '@/lib/logger';

const logger = createLogger().child({ module: 'knowledge-helper' });

export interface KnowledgeEnhancementParams {
  language?: string;
  framework?: string;
  technology?: string;
  environment?: string;
}

/**
 * Enhance a prompt with relevant knowledge from the knowledge base
 */
export async function enhancePrompt(
  basePrompt: string,
  operation: KnowledgeOperation | string,
  params?: KnowledgeEnhancementParams,
): Promise<string> {
  try {
    const enhancedArgs = await enhancePromptWithKnowledge(
      { prompt: basePrompt },
      {
        operation: operation as KnowledgeOperation,
        ...params,
      },
    );

    if (
      !enhancedArgs?.bestPractices?.length &&
      !enhancedArgs?.securityGuidelines?.length &&
      !enhancedArgs?.optimizationTips?.length
    ) {
      return basePrompt;
    }

    // Structure the enhanced prompt
    let enhanced = basePrompt;

    if (enhancedArgs.bestPractices?.length) {
      enhanced += '\n\n## Best Practices to Apply\n';
      enhanced += enhancedArgs.bestPractices.map((p) => `- ${p}`).join('\n');
    }

    if (enhancedArgs.securityGuidelines?.length) {
      enhanced += '\n\n## Security Guidelines\n';
      enhanced += enhancedArgs.securityGuidelines.map((g) => `- ${g}`).join('\n');
    }

    if (enhancedArgs.optimizationTips?.length) {
      enhanced += '\n\n## Optimization Tips\n';
      enhanced += enhancedArgs.optimizationTips.map((t) => `- ${t}`).join('\n');
    }

    logger.debug(
      {
        operation,
        bestPracticesCount: enhancedArgs.bestPractices?.length || 0,
        securityCount: enhancedArgs.securityGuidelines?.length || 0,
        optimizationCount: enhancedArgs.optimizationTips?.length || 0,
      },
      'Enhanced prompt with knowledge',
    );

    return enhanced;
  } catch (error) {
    // If enhancement fails, return original prompt
    logger.warn({ error, operation }, 'Knowledge enhancement failed, using original prompt');
    return basePrompt;
  }
}

/**
 * Extract structured knowledge from enhanced args for AI response metadata
 */
export function extractKnowledgeMetadata(enhancedArgs: any): {
  knowledgeApplied: boolean;
  knowledgeSources?: string[];
} {
  const hasKnowledge =
    enhancedArgs?.bestPractices?.length > 0 ||
    enhancedArgs?.securityGuidelines?.length > 0 ||
    enhancedArgs?.optimizationTips?.length > 0;

  if (!hasKnowledge) {
    return { knowledgeApplied: false };
  }

  const sources: string[] = [];
  if (enhancedArgs.bestPractices?.length) sources.push('best-practices');
  if (enhancedArgs.securityGuidelines?.length) sources.push('security');
  if (enhancedArgs.optimizationTips?.length) sources.push('optimization');

  return {
    knowledgeApplied: true,
    knowledgeSources: sources,
  };
}
