/**
 * Knowledge Enhancement Service
 *
 * Provides AI-powered knowledge enhancement for containerization content
 */

import type { ToolContext } from '@/mcp/context';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { Success, Failure, type Result, TOPICS } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { parseAIResponse } from '@/mcp/ai/response-parser';
import { KnowledgeEnhancementResponseSchema } from '@/mcp/ai/schemas';
import { scoreResponse } from '@/mcp/ai/quality';
import { TOKEN_CONFIG } from '@/config/tokens';
import { SCORING_CONFIG } from '@/config/scoring';
import { SAMPLING_CONFIG } from '@/config/sampling';

export interface KnowledgeEnhancementRequest {
  /** Content to enhance */
  content: string;
  /** Context type for enhancement */
  context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement';
  /** Optional user query for specific enhancement goals */
  userQuery?: string;
  /** Validation context if available */
  validationContext?: Array<{
    message: string;
    severity: 'error' | 'warning' | 'info';
    category: string;
  }>;
  /** Target improvement focus */
  targetImprovement?: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
}

export interface KnowledgeEnhancementResult {
  /** Enhanced content with improvements applied */
  enhancedContent: string;
  /** List of knowledge areas applied to the content */
  knowledgeApplied: string[];
  /** Confidence score for the enhancement (0-1) */
  confidence: number;
  /** Actionable suggestions for further improvement */
  suggestions: string[];
  /** Detailed analysis of changes made */
  analysis: {
    /** Summary of improvements made */
    improvementsSummary: string;
    /** Areas that were enhanced */
    enhancementAreas: string[];
    /** Technical debt identified */
    technicalDebt: string[];
  };
  /** Metadata about the enhancement process */
  metadata: {
    /** Knowledge sources applied */
    sources: string[];
    /** Best practices applied */
    bestPractices: string[];
    /** Sampling quality score */
    qualityScore: number;
    /** Enhancement type */
    enhancementType: string;
    /** Processing time in milliseconds */
    processingTime?: number;
  };
}

/**
 * Enhances content with knowledge and best practices
 */
export async function enhanceWithKnowledge(
  request: KnowledgeEnhancementRequest,
  context: ToolContext,
): Promise<Result<KnowledgeEnhancementResult>> {
  const startTime = Date.now();
  try {
    const {
      content,
      context: contentType,
      userQuery,
      validationContext,
      targetImprovement,
    } = request;

    // Build the enhancement prompt
    const basePrompt = buildEnhancementPrompt(
      content,
      contentType,
      userQuery,
      validationContext,
      targetImprovement,
    );

    const promptMessages = await buildMessages({
      basePrompt,
      topic: TOPICS.KNOWLEDGE_ENHANCEMENT,
      tool: 'knowledge-enhancement',
      environment: 'development', // Default environment
      knowledgeBudget: 1000, // Moderate knowledge budget for enhancements
    });

    const result = await sampleWithRerank(
      context,
      async (_attemptIndex) => ({
        messages: toMCPMessages(promptMessages).messages,
        includeContext: 'thisServer',
        maxTokens: TOKEN_CONFIG.STANDARD,
        modelPreferences: {
          hints: [{ name: `knowledge-enhancement-${contentType}` }],
          intelligencePriority: SAMPLING_CONFIG.PRIORITIES.INTELLIGENCE,
          costPriority: SAMPLING_CONFIG.PRIORITIES.COST,
        },
      }),
      (text: string) => {
        const scoreResult = scoreResponse('knowledge', text, {
          contentType: contentType as
            | 'dockerfile'
            | 'kubernetes'
            | 'security'
            | 'enhancement'
            | 'knowledge'
            | 'general',
          ...(targetImprovement && { focus: targetImprovement }),
        });
        return scoreResult.total;
      },
      {},
    );

    if (!result.ok) {
      return Failure(`Knowledge enhancement sampling failed: ${result.error}`);
    }

    // Parse the response
    const parsedResult = await parseAIResponse(
      result.value.text,
      KnowledgeEnhancementResponseSchema,
      context,
    );

    if (!parsedResult.ok) {
      return Failure(`Failed to parse enhancement response: ${parsedResult.error}`);
    }

    const response = parsedResult.value;

    // Calculate confidence based on quality and sampling confidence
    const qualityScore = scoreResponse('knowledge', result.value.text, {
      contentType: contentType as
        | 'dockerfile'
        | 'kubernetes'
        | 'security'
        | 'enhancement'
        | 'knowledge'
        | 'general',
      ...(targetImprovement && { focus: targetImprovement }),
    });

    const confidence = Math.min(
      response.confidence * (qualityScore.total / 100),
      SCORING_CONFIG.CONFIDENCE.MAX,
    );

    // Format the enhancement result
    const enhancementResult: KnowledgeEnhancementResult = {
      enhancedContent: response.enhancedContent || content,
      knowledgeApplied: response.knowledgeApplied || [],
      confidence,
      suggestions: response.suggestions || [],
      analysis: {
        improvementsSummary:
          response.analysis?.improvementsSummary || 'No specific improvements identified',
        enhancementAreas:
          response.analysis?.enhancementAreas?.map(
            (area) => area.description || area.area || String(area),
          ) || [],
        technicalDebt:
          response.analysis?.technicalDebt?.map((debt: { description?: string } | string) =>
            typeof debt === 'string' ? debt : debt.description || String(debt),
          ) || [],
      },
      metadata: {
        sources: response.analysis?.knowledgeSources || [],
        bestPractices: response.analysis?.bestPracticesApplied || [],
        qualityScore: qualityScore.total,
        enhancementType: contentType,
        processingTime: Date.now() - startTime,
        ...(result.value.model && { model: result.value.model }),
      },
    };

    return Success(enhancementResult);
  } catch (error) {
    const message = extractErrorMessage(error);
    context.logger.error({ error: message, request }, 'Knowledge enhancement failed');
    return Failure(`Knowledge enhancement failed: ${message}`);
  }
}

/**
 * Build knowledge enhancement prompt
 */
function buildEnhancementPrompt(
  content: string,
  contentType: string,
  userQuery?: string,
  validationContext?: Array<{ message: string; severity: string; category: string }>,
  targetImprovement?: string,
): string {
  const focusArea = targetImprovement || 'all';

  let prompt = `You are a containerization expert. Enhance the following ${contentType} content with knowledge and best practices.

Focus Areas: ${focusArea}

Original Content:
\`\`\`
${content}
\`\`\`
`;

  if (userQuery) {
    prompt += `\nUser Query: ${userQuery}`;
  }

  if (validationContext && validationContext.length > 0) {
    prompt += `\nValidation Issues Found:
${validationContext.map((v) => `- ${v.severity.toUpperCase()}: ${v.message} (${v.category})`).join('\n')}`;
  }

  prompt += `\nProvide a comprehensive enhancement that includes:
1. Enhanced content with improvements applied
2. Knowledge sources and best practices used
3. Detailed analysis of improvements made
4. Actionable suggestions for further enhancement

Return response in the specified JSON format.`;

  return prompt;
}

/**
 * Create knowledge enhancement request from validation results
 */
export function createEnhancementFromValidation(
  content: string,
  context: 'dockerfile' | 'kubernetes' | 'security' | 'enhancement',
  validationResults: Array<{
    message: string;
    severity: string;
    category?: string;
  }>,
  targetImprovement?: KnowledgeEnhancementRequest['targetImprovement'],
): KnowledgeEnhancementRequest {
  return {
    content,
    context:
      context === 'enhancement'
        ? 'enhancement'
        : (context as 'dockerfile' | 'kubernetes' | 'security' | 'enhancement'),
    targetImprovement: targetImprovement || 'all',
    validationContext: validationResults.map((result) => ({
      message: result.message,
      severity: (['error', 'warning', 'info'].includes(result.severity)
        ? result.severity
        : 'warning') as 'error' | 'warning' | 'info',
      category: result.category || 'general',
    })),
  };
}

// Type aliases for compatibility with index.ts exports
export type EnhancementOptions = KnowledgeEnhancementRequest;
export type EnhancementResult = KnowledgeEnhancementResult;
