/**
 * AI Enhancement Service for Validation Tools
 *
 * Provides AI-powered suggestions, fixes, and analysis for validation results
 */

import type { ToolContext } from '@/mcp/context';
import { ValidationSeverity, ValidationCategory, type ValidationResult } from './core-types';

// Re-export for convenience to avoid duplicate import issues
export { ValidationSeverity, ValidationCategory, type ValidationResult };
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { Success, Failure, type Result, type AIMessage } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { parseAIResponse } from '@/mcp/ai/response-parser';
import { AIEnhancementResponseSchema } from '@/mcp/ai/schemas';
import { scoreResponse } from '@/mcp/ai/quality';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { SCORING_CONFIG } from '@/config/scoring';
import { TOKEN_CONFIG } from '@/config/tokens';
import { SAMPLING_CONFIG } from '@/config/sampling';

export interface EnhancementOptions {
  /** Mode of AI enhancement */
  mode: 'suggestions' | 'fixes' | 'analysis';
  /** Focus area for enhancement */
  focus: 'security' | 'performance' | 'best-practices' | 'all';
  /** Minimum confidence threshold (0-1) */
  confidence: number;
  /** Maximum number of suggestions to return */
  maxSuggestions?: number;
  /** Include fix examples in suggestions */
  includeExamples?: boolean;
}

export interface AIEnhancementResult {
  /** AI-generated suggestions for improvements */
  suggestions: string[];
  /** Complete fixed content (if mode is 'fixes') */
  fixes?: string;
  /** Detailed analysis with insights */
  analysis: {
    /** Overall assessment */
    assessment: string;
    /** Risk level assessment */
    riskLevel: 'low' | 'medium' | 'high' | 'critical';
    /** Prioritized improvement areas */
    priorities: Array<{
      area: string;
      severity: ValidationSeverity;
      description: string;
      impact: string;
    }>;
    /** Technical debt indicators */
    technicalDebt?: Array<{
      category: ValidationCategory;
      description: string;
      effort: 'low' | 'medium' | 'high';
    }>;
  };
  /** Confidence score for the AI analysis (0-1) */
  confidence: number;
  /** Metadata about the AI enhancement */
  metadata: {
    /** Model used for enhancement */
    model?: string;
    /** Processing time in milliseconds */
    processingTime: number;
    /** Number of candidates evaluated */
    candidatesEvaluated: number;
    /** Token usage statistics */
    tokenUsage?: {
      inputTokens: number;
      outputTokens: number;
      totalTokens: number;
    };
  };
}

/**
 * Enhance validation results with AI-powered suggestions and analysis
 */
export async function enhanceValidationWithAI(
  content: string,
  validationResults: ValidationResult[],
  ctx: ToolContext,
  options: EnhancementOptions = {
    mode: 'suggestions',
    focus: 'all',
    confidence: SCORING_CONFIG.CONFIDENCE.HIGH,
    maxSuggestions: SAMPLING_CONFIG.LIMITS.MAX_SUGGESTIONS,
    includeExamples: true,
  },
): Promise<Result<AIEnhancementResult>> {
  const startTime = Date.now();

  try {
    ctx.logger.info(
      {
        contentLength: content.length,
        validationIssues: validationResults.length,
        mode: options.mode,
        focus: options.focus,
      },
      'Starting AI validation enhancement',
    );

    // Build the enhancement prompt based on mode and focus
    const samplingResult = await sampleWithRerank(
      ctx,
      async (_attemptIndex) => {
        const aiMessages = await buildEnhancementPrompt(content, validationResults, options);
        const mcpMessages = toMCPMessages({ messages: aiMessages });
        return {
          messages: mcpMessages.messages,
          maxTokens: options.mode === 'fixes' ? TOKEN_CONFIG.EXTENDED : TOKEN_CONFIG.STANDARD,
          modelPreferences: {
            hints: [
              { name: `validation-enhancement-${options.focus}` },
              { name: `mode-${options.mode}` },
            ],
            intelligencePriority: SAMPLING_CONFIG.PRIORITIES.INTELLIGENCE,
            costPriority: SAMPLING_CONFIG.PRIORITIES.COST,
          },
        };
      },
      (text: string) => {
        const scoreResult = scoreResponse('enhancement', text, {
          contentType: 'enhancement',
          targetImprovement: options.focus,
        });
        return scoreResult.breakdown;
      },
      {},
    );

    if (!samplingResult.ok) {
      return Failure(`AI enhancement failed: ${samplingResult.error}`);
    }

    const response = samplingResult.value;

    // Parse the AI response using structured JSON
    const parseResult = await parseAIResponse(response.text, AIEnhancementResponseSchema, ctx, {
      repairAttempts: 1,
      debug: true,
    });

    if (!parseResult.ok) {
      return Failure(`Failed to parse AI enhancement response: ${parseResult.error}`);
    }

    const parsedContent = parseResult.value;
    const processingTime = Date.now() - startTime;

    const result: AIEnhancementResult = {
      suggestions: parsedContent.suggestions.slice(
        0,
        options.maxSuggestions || SAMPLING_CONFIG.LIMITS.MAX_SUGGESTIONS,
      ),
      ...(options.mode === 'fixes' && parsedContent.fixes && { fixes: parsedContent.fixes }),
      analysis: {
        assessment: parsedContent.analysis.assessment,
        riskLevel: parsedContent.analysis.riskLevel,
        priorities: parsedContent.analysis.priorities.map((priority) => ({
          area: priority.area || 'general',
          severity:
            priority.severity === 'error' ||
            priority.severity === 'warning' ||
            priority.severity === 'info'
              ? (priority.severity as ValidationSeverity)
              : ValidationSeverity.WARNING,
          description: priority.description || priority.area || 'Unknown priority',
          impact: priority.impact || 'medium',
        })),
        ...(parsedContent.analysis.technicalDebt && {
          technicalDebt: parsedContent.analysis.technicalDebt.map((debt) => ({
            category: debt.category as ValidationCategory,
            description: debt.description,
            effort: debt.effort,
          })),
        }),
      },
      confidence: (response.score ?? 0) / SCORING_CONFIG.SCALE,
      metadata: {
        ...(response.model && { model: response.model }),
        processingTime,
        candidatesEvaluated: 1, // sampleWithRerank doesn't expose this currently
        ...(response.usage?.totalTokens && {
          tokenUsage: {
            inputTokens: response.usage.inputTokens || 0,
            outputTokens: response.usage.outputTokens || 0,
            totalTokens: response.usage.totalTokens,
          },
        }),
      },
    };

    ctx.logger.info(
      {
        suggestionsCount: result.suggestions.length,
        riskLevel: result.analysis.riskLevel,
        confidence: result.confidence,
        processingTime,
      },
      'AI validation enhancement completed',
    );

    return Success(result);
  } catch (error) {
    const processingTime = Date.now() - startTime;
    ctx.logger.error(
      {
        error: extractErrorMessage(error),
        processingTime,
        options,
      },
      'AI validation enhancement failed',
    );

    return Failure(`AI enhancement error: ${extractErrorMessage(error)}`);
  }
}

/**
 * Build AI prompt for validation enhancement
 */
async function buildEnhancementPrompt(
  content: string,
  validationResults: ValidationResult[],
  options: EnhancementOptions,
): Promise<AIMessage[]> {
  const errorMessages = validationResults
    .filter((r) => !r.isValid)
    .map((r) => r.errors.join(', ') || r.message || 'Unknown error')
    .join('\n- ');

  const warnings = validationResults.flatMap((r) => r.warnings || []).join('\n- ');

  const focusInstruction = getFocusInstruction(options.focus);
  const modeInstruction = getModeInstruction(options.mode);

  const combinedPrompt = `You are a containerization and DevOps expert specializing in security, performance, and best practices. Your task is to analyze validation results and provide actionable improvements.

${focusInstruction}
${modeInstruction}

**CRITICAL: You MUST respond with valid JSON only. No markdown, no explanations, just JSON.**

Required JSON schema:
{
  "suggestions": ["array of strings - Specific, actionable suggestions"],
  ${options.mode === 'fixes' ? '"fixes": "string - Complete fixed version of the content",' : ''}
  "analysis": {
    "assessment": "string - Overall assessment of the current state",
    "riskLevel": "low|medium|high|critical",
    "priorities": [
      {
        "area": "string - improvement area",
        "severity": "error|warning|info",
        "description": "string - what needs to be improved",
        "impact": "string - impact description"
      }
    ],
    "technicalDebt": [
      {
        "category": "security|performance|best-practice|compliance|optimization",
        "description": "string - description of technical debt",
        "effort": "low|medium|high"
      }
    ]
  }
}

---

Analyze this content and validation results:

**Content:**
\`\`\`
${content}
\`\`\`

**Validation Issues:**
${errorMessages ? `Errors:\n- ${errorMessages}` : 'No errors found'}

${warnings ? `\nWarnings:\n- ${warnings}` : ''}

**Focus:** ${options.focus}
**Mode:** ${options.mode}
**Confidence threshold:** ${options.confidence}

Please provide your analysis and recommendations.`;

  return [
    {
      role: 'user' as const,
      content: [{ type: 'text' as const, text: combinedPrompt }],
    },
  ];
}

/**
 * Get focus-specific instructions
 */
function getFocusInstruction(focus: EnhancementOptions['focus']): string {
  switch (focus) {
    case 'security':
      return 'Focus on security vulnerabilities, attack vectors, secrets exposure, and hardening measures.';
    case 'performance':
      return 'Focus on optimization opportunities, resource efficiency, caching, and performance bottlenecks.';
    case 'best-practices':
      return 'Focus on industry standards, maintainability, documentation, and operational excellence.';
    case 'all':
    default:
      return 'Provide comprehensive analysis covering security, performance, and best practices.';
  }
}

/**
 * Get mode-specific instructions
 */
function getModeInstruction(mode: EnhancementOptions['mode']): string {
  switch (mode) {
    case 'suggestions':
      return 'Provide actionable suggestions and recommendations for improvement.';
    case 'fixes':
      return 'Generate both suggestions AND complete fixed content that addresses all issues.';
    case 'analysis':
      return 'Focus on deep analysis and insights rather than specific fixes.';
    default:
      return 'Provide suggestions and recommendations for improvement.';
  }
}

/**
 * Create a summary analysis from validation results for quick insights
 */
export function summarizeValidationResults(results: ValidationResult[]): {
  totalIssues: number;
  errorCount: number;
  warningCount: number;
  categories: Record<ValidationCategory, number>;
  severityDistribution: Record<ValidationSeverity, number>;
} {
  const errorCount = results.filter((r) => !r.isValid).length;
  const warningCount = results.reduce((acc, r) => acc + (r.warnings?.length || 0), 0);

  const categories: Record<ValidationCategory, number> = {
    [ValidationCategory.SECURITY]: 0,
    [ValidationCategory.PERFORMANCE]: 0,
    [ValidationCategory.BEST_PRACTICE]: 0,
    [ValidationCategory.COMPLIANCE]: 0,
    [ValidationCategory.OPTIMIZATION]: 0,
  };

  const severityDistribution: Record<ValidationSeverity, number> = {
    [ValidationSeverity.ERROR]: 0,
    [ValidationSeverity.WARNING]: 0,
    [ValidationSeverity.INFO]: 0,
  };

  // This would need access to rule metadata to properly categorize
  // For now, we'll make basic inferences from the results
  results.forEach((result) => {
    if (result.metadata?.severity) {
      severityDistribution[result.metadata.severity]++;
    }

    // Basic categorization based on common keywords
    const allText = [...(result.errors || []), ...(result.warnings || []), result.message || '']
      .join(' ')
      .toLowerCase();

    if (allText.includes('security') || allText.includes('vulnerability')) {
      categories[ValidationCategory.SECURITY]++;
    }
    if (allText.includes('performance') || allText.includes('optimization')) {
      categories[ValidationCategory.PERFORMANCE]++;
    }
    if (allText.includes('best practice') || allText.includes('recommended')) {
      categories[ValidationCategory.BEST_PRACTICE]++;
    }
  });

  return {
    totalIssues: errorCount + warningCount,
    errorCount,
    warningCount,
    categories,
    severityDistribution,
  };
}
