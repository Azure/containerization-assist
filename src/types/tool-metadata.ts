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
export const SamplingStrategySchema = z.enum(['none', 'single', 'rerank']);
export type SamplingStrategy = z.infer<typeof SamplingStrategySchema>;

/**
 * Mandatory Tool Metadata Schema
 *
 * All tools must provide complete metadata for proper AI enhancement
 * and capability discovery.
 */
export const ToolMetadataSchema = z.object({
  /** Whether this tool uses AI-driven content generation (required) */
  aiDriven: z.boolean(),

  /** Whether this tool uses knowledge enhancement (required) */
  knowledgeEnhanced: z.boolean(),

  /** Sampling strategy used for AI generation (required) */
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
    aiDriven: false,
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
    aiDriven: true,
    knowledgeEnhanced: overrides.knowledgeEnhanced ?? true,
    samplingStrategy: overrides.samplingStrategy ?? 'rerank',
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

  // AI-driven tools should have appropriate sampling strategy
  if (metadata.aiDriven && metadata.samplingStrategy === 'none') {
    issues.push('AI-driven tools should use "single" or "rerank" sampling strategy');
  }

  // Knowledge-enhanced tools should be AI-driven
  if (metadata.knowledgeEnhanced && !metadata.aiDriven) {
    issues.push('Knowledge-enhanced tools must be AI-driven');
  }

  // Knowledge-enhanced tools should have enhancement capabilities
  if (metadata.knowledgeEnhanced && metadata.enhancementCapabilities.length === 0) {
    issues.push('Knowledge-enhanced tools should specify enhancement capabilities');
  }

  // AI-driven tools with high confidence threshold should use rerank
  if (
    metadata.aiDriven &&
    metadata.confidenceThreshold &&
    metadata.confidenceThreshold > 0.8 &&
    metadata.samplingStrategy !== 'rerank'
  ) {
    issues.push('High confidence thresholds (>0.8) require "rerank" sampling strategy');
  }

  if (issues.length > 0) {
    return Failure(`Tool "${toolName}" metadata inconsistency: ${issues.join('; ')}`);
  }

  return Success(undefined);
}
