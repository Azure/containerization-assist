/**
 * AI-Powered Validation Service
 *
 * Provides AI-driven validation capabilities for Dockerfiles and Kubernetes manifests
 */

import type { ToolContext } from '@/mcp/context';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { Success, Failure, type Result, type AIMessage } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { parseAIResponse } from '@/mcp/ai/response-parser';
import { ValidationReportResponseSchema } from '@/mcp/ai/schemas';
import { scoreResponse } from '@/mcp/ai/quality';
import { scoreToGrade } from '@/types/ai';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { TOKEN_CONFIG } from '@/config/tokens';
import { SAMPLING_CONFIG } from '@/config/sampling';
import {
  ValidationReport,
  ValidationSeverity,
  ValidationGrade,
  ValidationCategory,
} from './core-types';

/**
 * Convert numeric score to letter grade using constants
 */
function getGradeFromScore(score: number): ValidationGrade {
  return scoreToGrade(score) as ValidationGrade;
}

export interface AIValidationOptions {
  /** Content type being validated */
  contentType: 'dockerfile' | 'kubernetes' | 'security' | 'general';
  /** Focus area for validation */
  focus?: 'security' | 'performance' | 'best-practices' | 'all';
  /** Minimum confidence threshold (0-1) */
  confidence?: number;
  /** Maximum number of issues to identify */
  maxIssues?: number;
  /** Include fix suggestions */
  includeFixes?: boolean;
}

export interface AIValidationResult extends ValidationReport {
  /** AI-specific metadata */
  aiMetadata: {
    /** Model used for validation */
    model?: string;
    /** Processing time in milliseconds */
    processingTime: number;
    /** Confidence score for the validation (0-1) */
    confidence: number;
    /** Number of candidates evaluated */
    candidatesEvaluated: number;
  };
}

/**
 * AI-powered validator class for content analysis
 */
export class AIValidator {
  /**
   * Validate content using AI analysis
   */
  async validateWithAI(
    content: string,
    options: AIValidationOptions,
    ctx: ToolContext,
  ): Promise<Result<AIValidationResult>> {
    const startTime = Date.now();

    try {
      ctx.logger.info(
        {
          contentLength: content.length,
          contentType: options.contentType,
          focus: options.focus || 'all',
        },
        'Starting AI-powered validation',
      );

      const samplingResult = await sampleWithRerank(
        ctx,
        async (_attemptIndex) => {
          const aiMessages = await this.buildValidationPrompt(content, options);
          const mcpMessages = toMCPMessages({ messages: aiMessages });
          return {
            messages: mcpMessages.messages,
            maxTokens: TOKEN_CONFIG.STANDARD,
            modelPreferences: {
              hints: [
                { name: `validation-${options.contentType}` },
                { name: `focus-${options.focus || 'all'}` },
              ],
              intelligencePriority: SAMPLING_CONFIG.PRIORITIES.INTELLIGENCE,
              costPriority: SAMPLING_CONFIG.PRIORITIES.COST,
            },
          };
        },
        (text: string) => {
          const scoreResult = scoreResponse('validation', text, {
            contentType: options.contentType,
            validationOptions: {
              contentType: options.contentType,
              ...(options.focus && { focus: options.focus }),
            },
          });
          return scoreResult.breakdown;
        },
        {},
      );

      if (!samplingResult.ok) {
        return Failure(`AI validation failed: ${samplingResult.error}`);
      }

      const response = samplingResult.value;
      const parseResult = await parseAIResponse(
        response.text,
        ValidationReportResponseSchema,
        ctx,
        { repairAttempts: 1, debug: true },
      );

      if (!parseResult.ok) {
        return Failure(`Failed to parse AI validation response: ${parseResult.error}`);
      }

      const parsedResponse = parseResult.value;

      // Convert to ValidationReport format
      const validationResults: ValidationReport = {
        results: parsedResponse.results.map((result) => ({
          isValid: result.isValid,
          errors: result.errors,
          warnings: result.warnings || [],
          ...(result.ruleId && { ruleId: result.ruleId }),
          ...(result.passed !== undefined && { passed: result.passed }),
          ...(result.message && { message: result.message }),
          ...(result.suggestions && { suggestions: result.suggestions }),
          ...(result.confidence !== undefined && { confidence: result.confidence }),
          ...(result.metadata && {
            metadata: {
              ...(result.metadata.validationTime !== undefined && {
                validationTime: result.metadata.validationTime,
              }),
              ...(result.metadata.rulesApplied && { rulesApplied: result.metadata.rulesApplied }),
              ...(result.metadata.severity && {
                severity: result.metadata.severity as ValidationSeverity,
              }),
              ...(result.metadata.location && { location: result.metadata.location }),
              ...(result.metadata.aiEnhanced !== undefined && {
                aiEnhanced: result.metadata.aiEnhanced,
              }),
              ...(result.metadata.category && {
                category: result.metadata.category as ValidationCategory,
              }),
              ...(result.metadata.fixSuggestion && {
                fixSuggestion: result.metadata.fixSuggestion,
              }),
            },
          }),
        })),
        score: Math.round(
          100 *
            (1 -
              parsedResponse.results.filter((r) => !r.isValid).length /
                Math.max(parsedResponse.results.length, 1)),
        ),
        grade: getGradeFromScore(
          Math.round(
            100 *
              (1 -
                parsedResponse.results.filter((r) => !r.isValid).length /
                  Math.max(parsedResponse.results.length, 1)),
          ),
        ),
        passed: parsedResponse.results.filter((r) => r.isValid).length,
        failed: parsedResponse.results.filter((r) => !r.isValid).length,
        errors: parsedResponse.summary.errorCount,
        warnings: parsedResponse.summary.warningCount,
        info: parsedResponse.results.filter((r) => r.metadata?.severity === ValidationSeverity.INFO)
          .length,
        timestamp: new Date().toISOString(),
      };
      const processingTime = Date.now() - startTime;

      const result: AIValidationResult = {
        ...validationResults,
        aiMetadata: {
          ...(response.model && { model: response.model }),
          processingTime,
          confidence: (response.score ?? 0) / 100,
          candidatesEvaluated: 1, // sampleWithRerank doesn't expose this currently
        },
      };

      ctx.logger.info(
        {
          issuesFound: result.results.length,
          passed: result.passed,
          confidence: result.aiMetadata.confidence,
          processingTime,
        },
        'AI validation completed successfully',
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
        'AI validation failed',
      );

      return Failure(`AI validation error: ${extractErrorMessage(error)}`);
    }
  }

  /**
   * Build AI prompt for content validation
   */
  private async buildValidationPrompt(
    content: string,
    options: AIValidationOptions,
  ): Promise<AIMessage[]> {
    const contentTypeInstructions = this.getContentTypeInstructions(options.contentType);
    const focusInstructions = this.getFocusInstructions(options.focus || 'all');

    const prompt = `You are an expert DevOps and security specialist. Analyze the following ${options.contentType} content for issues and provide detailed validation results.

${contentTypeInstructions}
${focusInstructions}

Response format (JSON):
{
  "passed": boolean,
  "results": [
    {
      "isValid": boolean,
      "ruleId": "rule-identifier",
      "message": "Issue description",
      "errors": ["Error message"],
      "warnings": ["Warning message"],
      "confidence": 0.0-1.0,
      "metadata": {
        "severity": "error|warning|info",
        "category": "security|performance|best_practice|compliance|optimization",
        "location": "line:column or section",
        "aiEnhanced": true,
        "fixSuggestion": "How to fix this issue"
      }
    }
  ],
  "summary": {
    "totalIssues": number,
    "errorCount": number,
    "warningCount": number,
    "categories": {
      "security": number,
      "performance": number,
      "best_practice": number,
      "compliance": number,
      "optimization": number
    }
  }
}

Analyze this ${options.contentType} content:

\`\`\`
${content}
\`\`\`

Provide a thorough analysis with specific, actionable recommendations.`;

    return [
      {
        role: 'user' as const,
        content: [{ type: 'text' as const, text: prompt }],
      },
    ];
  }

  /**
   * Get content-type specific validation instructions
   */
  private getContentTypeInstructions(contentType: AIValidationOptions['contentType']): string {
    switch (contentType) {
      case 'dockerfile':
        return 'Focus on Docker best practices, security hardening, layer optimization, and build efficiency. Check for security vulnerabilities, proper user permissions, base image selection, and multi-stage build opportunities.';
      case 'kubernetes':
        return 'Focus on Kubernetes manifest validation, resource management, security policies, networking, and operational excellence. Check for RBAC, security contexts, resource limits, and deployment strategies.';
      case 'security':
        return 'Focus primarily on security vulnerabilities, attack vectors, secrets exposure, and hardening measures. Prioritize critical security issues.';
      case 'general':
      default:
        return 'Provide comprehensive analysis covering all aspects including security, performance, and best practices.';
    }
  }

  /**
   * Get focus-specific validation instructions
   */
  private getFocusInstructions(focus: string): string {
    switch (focus) {
      case 'security':
        return 'Prioritize security issues: vulnerabilities, secrets, permissions, attack surfaces.';
      case 'performance':
        return 'Prioritize performance issues: resource usage, optimization, caching, efficiency.';
      case 'best-practices':
        return 'Prioritize maintainability and operational excellence: standards, documentation, patterns.';
      case 'all':
      default:
        return 'Analyze all aspects: security, performance, maintainability, and operational excellence.';
    }
  }
}

/**
 * Create a new AI validator instance
 */
export function createAIValidator(): AIValidator {
  return new AIValidator();
}
