/**
 * Tool Metadata Contract & Validation
 *
 * Provides comprehensive Zod-based validation with mandatory metadata
 * for all AI-enhanced tools.
 */

import { z } from 'zod';
import { Result, Success, Failure } from './core';

/**
 * Comprehensive enum for enhancement capabilities
 */
export const EnhancementCapabilitySchema = z.enum([
  'analysis',
  'azure-optimization',
  'best-practices',
  'build-analysis',
  'chart-optimization',
  'cluster-optimization',
  'content-generation',
  'deployment-analysis',
  'enhancement',
  'generation',
  'health-analysis',
  'helm-templating',
  'image-recommendation',
  'kubernetes-optimization',
  'manifest-conversion',
  'manifest-generation',
  'optimization',
  'optimization-recommendations',
  'optimization-suggestions',
  'performance-insights',
  'performance-recommendations',
  'performance-tuning',
  'platform-translation',
  'push-optimization',
  'recommendations',
  'registry-insights',
  'repair',
  'resource-recommendations',
  'risk-assessment',
  'security',
  'security-analysis',
  'security-enhancements',
  'security-recommendations',
  'security-suggestions',
  'self-repair',
  'tagging-strategy',
  'technology-detection',
  'troubleshooting',
  'troubleshooting-guidance',
  'validation',
  'validation-insights',
  'vulnerability-analysis',
]);

export type EnhancementCapability = z.infer<typeof EnhancementCapabilitySchema>;

/**
 * Sampling strategy enum
 */
export const SamplingStrategySchema = z.enum(['none', 'single']);
export type SamplingStrategy = z.infer<typeof SamplingStrategySchema>;

/**
 * Mandatory Tool Metadata Schema
 *
 * All tools must provide complete metadata for proper AI enhancement
 * and capability discovery.
 */
export const ToolMetadataSchema = z.object({
  /** Whether this tool uses knowledge enhancement (required) */
  knowledgeEnhanced: z.boolean(),

  /** Sampling strategy used for AI generation (required) - 'single' for AI-driven tools, 'none' for non-AI tools */
  samplingStrategy: SamplingStrategySchema,

  /** List of enhancement capabilities this tool provides (required) */
  enhancementCapabilities: z.array(EnhancementCapabilitySchema).default([]),

  /** Confidence threshold for accepting AI responses (optional) */
  confidenceThreshold: z.number().min(0).max(1).optional(),

  /** Maximum number of retry attempts for AI operations (optional) */
  maxRetries: z.number().int().positive().max(10).optional(),
});

export type ToolMetadata = z.infer<typeof ToolMetadataSchema>;

/**
 * Validates tool metadata against the schema
 *
 * @param metadata - Unknown metadata object to validate
 * @returns Result containing validated metadata or validation errors
 */
export function validateToolMetadata(metadata: unknown): Result<ToolMetadata> {
  try {
    const validatedMetadata = ToolMetadataSchema.parse(metadata);
    return Success(validatedMetadata);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const issues = error.issues
        .map((issue) => `${issue.path.join('.')}: ${issue.message}`)
        .join(', ');
      return Failure(`Tool metadata validation failed: ${issues}`);
    }
    return Failure(`Tool metadata validation failed: ${error}`);
  }
}

/**
 * Creates default metadata for non-AI tools
 */
export function createDefaultMetadata(): ToolMetadata {
  return {
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  };
}

/**
 * Creates metadata for AI-driven tools with sensible defaults
 */
export function createAIMetadata(overrides: Partial<ToolMetadata> = {}): ToolMetadata {
  return {
    knowledgeEnhanced: overrides.knowledgeEnhanced ?? true,
    samplingStrategy: overrides.samplingStrategy ?? 'single',
    enhancementCapabilities: overrides.enhancementCapabilities ?? ['generation', 'analysis'],
    confidenceThreshold: overrides.confidenceThreshold,
    maxRetries: overrides.maxRetries ?? 3,
  };
}

/**
 * Type guard to check if metadata is valid
 */
export function isValidToolMetadata(metadata: unknown): metadata is ToolMetadata {
  return ToolMetadataSchema.safeParse(metadata).success;
}

/**
 * Validates metadata consistency with tool properties
 */
export function validateMetadataConsistency(
  toolName: string,
  metadata: ToolMetadata,
): Result<void> {
  const issues: string[] = [];

  // Knowledge-enhanced tools should have enhancement capabilities
  if (metadata.knowledgeEnhanced && metadata.enhancementCapabilities.length === 0) {
    issues.push('Knowledge-enhanced tools should specify enhancement capabilities');
  }

  // High confidence threshold should use single sampling strategy
  if (
    metadata.confidenceThreshold &&
    metadata.confidenceThreshold > 0.8 &&
    metadata.samplingStrategy !== 'single'
  ) {
    issues.push('High confidence thresholds (>0.8) require "single" sampling strategy');
  }

  if (issues.length > 0) {
    return Failure(`Tool "${toolName}" metadata inconsistency: ${issues.join('; ')}`);
  }

  return Success(undefined);
}

/**
 * Interface for tool validation that includes all necessary properties
 */
export interface ValidatableTool {
  name: string;
  metadata: ToolMetadata;
}

/**
 * Performs post-validation checks for a single tool
 * Consolidates all validation logic from CLI into centralized location
 */
export function postValidate(tool: ValidatableTool): string[] {
  const issues: string[] = [];

  // Knowledge-enhanced tool missing capabilities
  if (tool.metadata.knowledgeEnhanced && tool.metadata.enhancementCapabilities.length === 0) {
    issues.push('Knowledge-enhanced tool missing capabilities');
  }

  // Tools with 'single' sampling should specify capabilities
  if (
    tool.metadata.samplingStrategy === 'single' &&
    tool.metadata.enhancementCapabilities.length === 0
  ) {
    issues.push('AI-driven tool (samplingStrategy: single) should specify capabilities');
  }

  return issues;
}

/**
 * Interface for comprehensive validation report
 */
export interface ValidationReport {
  summary: {
    totalTools: number;
    validTools: number;
    invalidTools: number;
    compliancePercentage: number;
  };
  validTools: string[];
  invalidTools: Array<{
    name: string;
    issues: string[];
    suggestions: string[];
  }>;
  metadataErrors: Array<{
    name: string;
    error: string;
  }>;
  consistencyErrors: Array<{
    name: string;
    error: string;
  }>;
}

/**
 * Suggestion mapping for common validation issues
 */
const VALIDATION_SUGGESTIONS: Record<string, string> = {
  'AI-driven tool should have sampling strategy': 'Set samplingStrategy to "single"',
  'Knowledge-enhanced tool missing capabilities': 'Add appropriate enhancementCapabilities array',
  'Knowledge-enhanced should be AI-driven':
    'Set samplingStrategy: "single" for knowledge-enhanced tools',
  'AI-driven tool should specify capabilities': 'Add relevant enhancementCapabilities',
  'Invalid metadata schema': 'Fix metadata schema validation errors',
  'Metadata consistency issues': 'Review and fix metadata consistency',
};

/**
 * Validates all tool metadata and returns comprehensive report
 * Centralized validation logic for use by CLI commands
 */
export async function validateAllToolMetadata(
  tools: ValidatableTool[],
): Promise<Result<ValidationReport>> {
  try {
    const validTools: string[] = [];
    const invalidTools: Array<{ name: string; issues: string[]; suggestions: string[] }> = [];
    const metadataErrors: Array<{ name: string; error: string }> = [];
    const consistencyErrors: Array<{ name: string; error: string }> = [];

    for (const tool of tools) {
      const issues: string[] = [];
      const suggestions: string[] = [];

      // Validate metadata schema compliance
      const metadataValidation = validateToolMetadata(tool.metadata);
      if (!metadataValidation.ok) {
        metadataErrors.push({
          name: tool.name,
          error: metadataValidation.error,
        });
        issues.push('Invalid metadata schema');
        suggestions.push(
          VALIDATION_SUGGESTIONS['Invalid metadata schema'] ??
            'Fix metadata schema validation errors',
        );
        continue;
      }

      // Validate metadata consistency
      const consistencyValidation = validateMetadataConsistency(tool.name, tool.metadata);
      if (!consistencyValidation.ok) {
        consistencyErrors.push({
          name: tool.name,
          error: consistencyValidation.error,
        });
        issues.push('Metadata consistency issues');
        suggestions.push(
          VALIDATION_SUGGESTIONS['Metadata consistency issues'] ??
            'Review and fix metadata consistency',
        );
      }

      // Perform post-validation checks
      const postValidationIssues = postValidate(tool);
      issues.push(...postValidationIssues);

      // Add suggestions for post-validation issues
      postValidationIssues.forEach((issue) => {
        const suggestion = VALIDATION_SUGGESTIONS[issue];
        if (suggestion) {
          suggestions.push(suggestion);
        }
      });

      if (issues.length > 0) {
        invalidTools.push({
          name: tool.name,
          issues,
          suggestions,
        });
      } else {
        validTools.push(tool.name);
      }
    }

    const totalTools = tools.length;
    const validCount = validTools.length;
    const compliancePercentage = Math.round((validCount / totalTools) * 100);

    return Success({
      summary: {
        totalTools,
        validTools: validCount,
        invalidTools: totalTools - validCount,
        compliancePercentage,
      },
      validTools,
      invalidTools,
      metadataErrors,
      consistencyErrors,
    });
  } catch (error) {
    return Failure(`Validation failed: ${error instanceof Error ? error.message : String(error)}`);
  }
}
