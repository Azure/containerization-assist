/**
 * AI-Powered Parameter Validation
 *
 * Uses AI services to validate and suggest improvements for tool parameters
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import type { PromptRegistry } from '../../core/prompts/registry';

/**
 * Parameter validation context with MCP-compatible types
 */
export interface ValidationContext {
  toolName: string;
  repositoryPath?: string;
  language?: string;
  framework?: string;
  environment?: 'development' | 'staging' | 'production';
  securityLevel?: 'basic' | 'standard' | 'strict';
  targetType?: 'dockerfile' | 'kubernetes' | 'analysis' | 'general';
}

/**
 * MCP-compatible validation result
 */
export interface ValidationResult {
  isValid: boolean;
  errors: string[];
  warnings: string[];
  suggestions?: Record<string, unknown>;
  confidence: number;
  metadata: {
    validationTime: number;
    aiEnhanced: boolean;
    rulesApplied: string[];
  };
}

/**
 * Parameter suggestions response compatible with MCP patterns
 */
export interface ParameterSuggestions {
  suggestions: Record<string, unknown>;
  confidence: number;
  reasoning: string;
  alternatives: Record<string, unknown[]>;
  metadata: {
    generationTime: number;
    aiProvider: string;
    contextUsed: string[];
  };
}

/**
 * AI-Powered Parameter Validator using MCP SDK patterns
 */
export class AIParameterValidator {
  private logger: Logger;

  constructor(
    logger: Logger,
    private promptRegistry?: PromptRegistry,
  ) {
    this.logger = logger.child({ component: 'AIParameterValidator' });
  }

  /**
   * Validate parameters with AI assistance
   */
  async validateParameters(
    toolName: string,
    parameters: Record<string, unknown>,
    context?: ValidationContext,
  ): Promise<Result<ValidationResult>> {
    const startTime = Date.now();
    this.logger.info({ toolName }, 'AI parameter validation requested');

    try {
      // 1. Perform basic validation
      const basicValidation = this.performBasicValidation(toolName, parameters, context);

      // 2. AI-powered validation if available
      if (this.isAIValidationAvailable()) {
        const aiValidation = await this.performAIValidation(toolName, parameters, context);

        if (aiValidation.ok) {
          const combined = this.combineValidationResults(basicValidation, aiValidation.value);
          combined.metadata.validationTime = Date.now() - startTime;
          combined.metadata.aiEnhanced = true;

          return Success(combined);
        } else {
          this.logger.warn(
            { toolName, error: aiValidation.error },
            'AI validation failed, falling back to basic validation',
          );
        }
      }

      // 3. Return basic validation
      basicValidation.metadata.validationTime = Date.now() - startTime;
      return Success(basicValidation);
    } catch (error) {
      return this.handleValidationError(error, startTime);
    }
  }

  /**
   * Get parameter suggestions with AI assistance
   */
  async suggestParameters(
    toolName: string,
    partialParameters: Record<string, unknown>,
    context?: ValidationContext,
  ): Promise<Result<ParameterSuggestions>> {
    this.logger.info({ toolName }, 'AI parameter suggestions requested');

    try {
      const suggestions: Record<string, unknown> = {};
      const alternatives: Record<string, unknown[]> = {};

      // Basic suggestions based on tool requirements
      if (toolName === 'generate-dockerfile') {
        if (!partialParameters.language && context?.language) {
          suggestions.language = context.language;
        }
        if (!partialParameters.securityLevel) {
          suggestions.securityLevel = context?.securityLevel || 'standard';
          alternatives.securityLevel = ['basic', 'standard', 'strict'];
        }
      }

      if (toolName === 'generate-k8s-manifests') {
        if (!partialParameters.replicas) {
          suggestions.replicas = context?.environment === 'production' ? 3 : 1;
          alternatives.replicas = [1, 2, 3, 5];
        }
      }

      const result: ParameterSuggestions = {
        suggestions,
        confidence: Object.keys(suggestions).length > 0 ? 0.7 : 0.3,
        reasoning: 'Generated using rule-based analysis and context patterns',
        alternatives,
        metadata: {
          generationTime: Date.now(),
          aiProvider: 'basic',
          contextUsed: ['basic-rules', 'context-analysis'],
        },
      };

      return Success(result);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error({ toolName, error: message }, 'Parameter suggestion failed');

      return Success({
        suggestions: {},
        confidence: 0.0,
        reasoning: `Parameter suggestion failed: ${message}`,
        alternatives: {},
        metadata: {
          generationTime: Date.now(),
          aiProvider: 'error',
          contextUsed: ['error-fallback'],
        },
      });
    }
  }

  /**
   * Check if AI validation is available
   */
  isAIValidationAvailable(): boolean {
    return !!this.promptRegistry;
  }

  /**
   * Get validation capabilities
   */
  getCapabilities(): {
    aiValidation: boolean;
    contextAware: boolean;
    suggestions: boolean;
    supportedTools: string[];
  } {
    return {
      aiValidation: this.isAIValidationAvailable(),
      contextAware: true,
      suggestions: true,
      supportedTools: [
        'analyze-repo',
        'generate-dockerfile',
        'build-image',
        'generate-k8s-manifests',
        'deploy',
        'scan',
        'containerization',
        'deployment',
      ],
    };
  }

  /**
   * Perform basic parameter validation
   */
  private performBasicValidation(
    toolName: string,
    parameters: Record<string, unknown>,
    context?: ValidationContext,
  ): ValidationResult {
    const errors: string[] = [];
    const warnings: string[] = [];
    const rulesApplied: string[] = ['basic-validation'];

    if (!parameters || Object.keys(parameters).length === 0) {
      warnings.push('No parameters provided for validation');
    }

    // Tool-specific basic validation
    if (toolName === 'analyze-repo' && !parameters.repoPath) {
      errors.push('repoPath is required for repository analysis');
      rulesApplied.push('repo-analysis-rules');
    }

    if (toolName === 'generate-dockerfile') {
      rulesApplied.push('dockerfile-generation-rules');
      if (!parameters.repoPath) {
        errors.push('repoPath is required for Dockerfile generation');
      }
      if (parameters.baseImage && typeof parameters.baseImage !== 'string') {
        errors.push('baseImage must be a string');
      }
    }

    if (toolName === 'build-image') {
      rulesApplied.push('build-validation-rules');
      if (!parameters.imageName) {
        errors.push('imageName is required for image building');
      }
      const dockerNameRegex = /^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$/;
      if (
        parameters.imageName &&
        typeof parameters.imageName === 'string' &&
        !dockerNameRegex.test(parameters.imageName)
      ) {
        errors.push('imageName must follow Docker naming conventions');
      }
    }

    // Context-based validation
    if (context) {
      rulesApplied.push('context-validation');
      if (context.environment === 'production') {
        if (context.securityLevel === 'basic') {
          warnings.push('Consider using standard or strict security level for production');
        }
        if (
          parameters.replicas &&
          typeof parameters.replicas === 'number' &&
          parameters.replicas < 2
        ) {
          warnings.push('Consider using multiple replicas for production deployments');
        }
      }
    }

    return {
      isValid: errors.length === 0,
      errors,
      warnings,
      confidence: this.calculateBasicConfidence(errors.length, warnings.length),
      metadata: {
        validationTime: 0, // Will be set by caller
        aiEnhanced: false,
        rulesApplied,
      },
    };
  }

  /**
   * Calculate confidence for basic validation
   */
  private calculateBasicConfidence(errorCount: number, warningCount: number): number {
    if (errorCount > 0) return 0.3;
    if (warningCount > 3) return 0.6;
    if (warningCount > 1) return 0.7;
    if (warningCount > 0) return 0.8;
    return 0.9;
  }

  /**
   * Perform AI-powered validation
   */
  private async performAIValidation(
    toolName: string,
    parameters: Record<string, unknown>,
    context?: ValidationContext,
  ): Promise<Result<ValidationResult>> {
    try {
      // Use prompt registry for validation prompts
      if (this.promptRegistry) {
        await this.promptRegistry.getPrompt('parameter-validation', {
          toolName,
          parameters: JSON.stringify(parameters, null, 2),
          context: context ? JSON.stringify(context, null, 2) : undefined,
          validationRules: this.getValidationRules(toolName),
        });
      }

      return Failure('AI validation requires ToolContext pattern');
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return Failure(`AI validation error: ${message}`);
    }
  }

  /**
   * Get validation rules for a specific tool
   */
  private getValidationRules(toolName: string): string {
    const rules: Record<string, string> = {
      'analyze-repo': 'Validate repository path exists and is accessible',
      'generate-dockerfile': 'Validate language/framework compatibility and security settings',
      'generate-k8s-manifests': 'Validate Kubernetes resource specifications and best practices',
      'build-image': 'Validate image naming, build context, and Docker configurations',
      deploy: 'Validate deployment parameters and target environment compatibility',
      scan: 'Validate scan targets and security analysis parameters',
    };

    for (const [key, rule] of Object.entries(rules)) {
      if (toolName.includes(key)) {
        return rule;
      }
    }

    return 'Apply general parameter validation rules';
  }

  /**
   * Combine basic and AI validation results
   */
  private combineValidationResults(
    basicResult: ValidationResult,
    aiResult: ValidationResult,
  ): ValidationResult {
    const combinedErrors = [...new Set([...basicResult.errors, ...aiResult.errors])];
    const combinedWarnings = [...new Set([...basicResult.warnings, ...aiResult.warnings])];

    return {
      isValid: combinedErrors.length === 0,
      errors: combinedErrors,
      warnings: combinedWarnings,
      ...(aiResult.suggestions && { suggestions: aiResult.suggestions }),
      confidence: Math.max(basicResult.confidence, aiResult.confidence),
      metadata: {
        validationTime: basicResult.metadata.validationTime,
        aiEnhanced: true,
        rulesApplied: [...basicResult.metadata.rulesApplied, ...aiResult.metadata.rulesApplied],
      },
    };
  }

  /**
   * Handle validation errors
   */
  private handleValidationError(error: unknown, startTime: number): Result<ValidationResult> {
    const message = error instanceof Error ? error.message : String(error);
    this.logger.error({ error: message }, 'Parameter validation error');

    const errorResult: ValidationResult = {
      isValid: false,
      errors: [message],
      warnings: [],
      confidence: 0,
      metadata: {
        validationTime: Date.now() - startTime,
        aiEnhanced: false,
        rulesApplied: ['error-handling'],
      },
    };
    return Success(errorResult);
  }
}

/**
 * Create AI parameter validator instance with MCP SDK integration
 */
export const createAIParameterValidator = (
  logger: Logger,
  promptRegistry?: PromptRegistry,
): AIParameterValidator => {
  return new AIParameterValidator(logger, promptRegistry);
};
