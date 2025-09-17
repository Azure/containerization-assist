import { createLogger } from '@/lib/logger';
import { getKnowledgeForCategory, getKnowledgeRecommendations } from '@/knowledge/index';
import type { KnowledgeQuery, KnowledgeMatch } from '@/knowledge/types';

const logger = createLogger().child({ module: 'knowledge-enhancer' });

export interface PromptEnhancementContext {
  /** The operation being performed */
  operation: string;

  /** Programming language */
  language?: string;

  /** Framework being used */
  framework?: string;

  /** Target environment */
  environment?: string;

  /** Base image being used */
  baseImage?: string;

  /** Dockerfile content for analysis */
  dockerfileContent?: string;

  /** Kubernetes manifest content */
  k8sContent?: string;

  /** Additional tags for filtering */
  tags?: string[];
}

export interface EnhancedPromptArgs {
  /** Original prompt arguments */
  [key: string]: unknown;

  /** Best practices from knowledge base */
  bestPractices?: string[];

  /** Code examples */
  examples?: string[];

  /** Security recommendations */
  securityRecommendations?: string[];

  /** Knowledge-based suggestions */
  knowledgeSuggestions?: Array<{
    recommendation: string;
    reason: string;
    severity: string;
  }>;
}

/**
 * Enhance prompt arguments with knowledge base recommendations
 */
export async function enhancePromptWithKnowledge(
  promptArgs: Record<string, unknown>,
  context: PromptEnhancementContext,
): Promise<EnhancedPromptArgs> {
  try {
    const enhancedArgs: EnhancedPromptArgs = { ...promptArgs };

    // Determine which categories to query based on operation
    const categories = getRelevantCategories(context.operation);

    const allMatches = [];

    // Query each relevant category
    for (const category of categories) {
      let text = '';

      // Extract relevant text based on category
      if (category === 'dockerfile' && context.dockerfileContent) {
        text = context.dockerfileContent;
      } else if (category === 'dockerfile' && context.operation === 'generate_dockerfile') {
        // For generation, use language and framework as matching context for knowledge patterns
        const contextParts = [];
        if (context.language) contextParts.push(context.language);
        if (context.framework) contextParts.push(context.framework);
        text = contextParts.join(' ');
      } else if (category === 'kubernetes' && context.k8sContent) {
        text = context.k8sContent;
      } else if (context.baseImage) {
        text = context.baseImage;
      }

      const matches = await getKnowledgeForCategory(category, text, {
        ...(context.language && { language: context.language }),
        ...(context.framework && { framework: context.framework }),
        environment: context.environment || 'production',
      });

      allMatches.push(...matches);
    }

    // Sort by score and take top matches
    const topMatches = allMatches.sort((a, b) => b.score - a.score).slice(0, 8);

    if (topMatches.length > 0) {
      logger.info(
        {
          operation: context.operation,
          matchCount: topMatches.length,
          categories,
        },
        'Enhanced prompt with knowledge recommendations',
      );

      // Extract best practices
      enhancedArgs.bestPractices = topMatches
        .filter((m) => m.score > 30)
        .map((m) => m.entry.recommendation);

      // Extract examples
      enhancedArgs.examples = topMatches
        .map((m) => m.entry.example)
        .filter((example): example is string => Boolean(example));

      // Extract security-specific recommendations
      enhancedArgs.securityRecommendations = topMatches
        .filter((m) => m.entry.category === 'security' || m.entry.tags?.includes('security'))
        .map((m) => m.entry.recommendation);

      // Create detailed suggestions with context
      enhancedArgs.knowledgeSuggestions = topMatches.map((m) => ({
        recommendation: m.entry.recommendation,
        reason: m.reasons.join(', '),
        severity: m.entry.severity || 'medium',
      }));
    } else {
      logger.debug(
        {
          operation: context.operation,
          categories,
        },
        'No knowledge matches found',
      );
    }

    return enhancedArgs;
  } catch (error) {
    logger.warn({ error }, 'Failed to enhance prompt with knowledge, continuing without');
    return { ...promptArgs };
  }
}

/**
 * Get relevant knowledge categories for an operation
 */
function getRelevantCategories(operation: string): Array<'dockerfile' | 'kubernetes' | 'security'> {
  const categoryMap: Record<string, Array<'dockerfile' | 'kubernetes' | 'security'>> = {
    generate_dockerfile: ['dockerfile', 'security'],
    fix_dockerfile: ['dockerfile', 'security'],
    generate_k8s_manifests: ['kubernetes', 'security'],
    resolve_base_images: ['dockerfile'],
    analyze_repository: ['dockerfile', 'security'],
    validate_dockerfile: ['dockerfile', 'security'],
    validate_k8s: ['kubernetes', 'security'],
  };

  return categoryMap[operation] || ['dockerfile', 'kubernetes', 'security'];
}

/**
 * Get specific knowledge for base image selection
 */
export async function getBaseImageKnowledge(
  language: string,
  environment: string = 'production',
): Promise<string[]> {
  try {
    const query: KnowledgeQuery = {
      category: 'dockerfile',
      language,
      environment,
      tags: ['alpine', 'slim', 'production'],
      limit: 3,
    };

    const matches = await getKnowledgeRecommendations(query);
    return matches.map((m: KnowledgeMatch) => m.entry.recommendation);
  } catch (error) {
    logger.warn({ error }, 'Failed to get base image knowledge');
    return [];
  }
}

/**
 * Get security-focused knowledge
 */
export async function getSecurityKnowledge(
  category: 'dockerfile' | 'kubernetes' = 'dockerfile',
): Promise<Array<{ recommendation: string; severity: string }>> {
  try {
    const query: KnowledgeQuery = {
      category,
      tags: ['security'],
      limit: 5,
    };

    const matches = await getKnowledgeRecommendations(query);
    return matches.map((m: KnowledgeMatch) => ({
      recommendation: m.entry.recommendation,
      severity: m.entry.severity || 'medium',
    }));
  } catch (error) {
    logger.warn({ error }, 'Failed to get security knowledge');
    return [];
  }
}
