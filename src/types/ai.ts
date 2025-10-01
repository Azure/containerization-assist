/**
 * AI Service Type Definitions
 *
 * Discriminated unions and type-safe interfaces replacing string enums
 * and loose typing throughout AI services.
 */

/**
 * Validation context discriminated union
 * Replaces string-based context types with type-safe variants
 */
export type ValidationContext =
  | { type: 'dockerfile'; runtime?: string; baseImage?: string }
  | { type: 'kubernetes'; version?: string; cluster?: string }
  | { type: 'security'; severity: 'low' | 'medium' | 'high' | 'critical'; focus?: string }
  | { type: 'optimization'; target: 'size' | 'performance' | 'cost' | 'all' };

/**
 * Enhancement capabilities enum
 * Replaces magic string capabilities with type-safe alternatives
 */
export type EnhancementCapability =
  | 'validation'
  | 'repair'
  | 'security'
  | 'optimization'
  | 'analysis'
  | 'generation'
  | 'enhancement';

/**
 * Sampling strategy discriminated union
 * Replaces string-based sampling configuration
 */
export type SamplingStrategy = 'none' | 'single';

/**
 * AI service operation modes
 * Type-safe operation mode specification
 */
export type AIServiceMode = 'fast' | 'balanced' | 'thorough';

/**
 * Validation severity levels
 * Replaces string-based severity with discriminated union
 */
export type ValidationSeverity = 'error' | 'warning' | 'info';

/**
 * Validation categories
 * Type-safe validation category enumeration
 */
export type ValidationCategory =
  | 'security'
  | 'performance'
  | 'best-practice'
  | 'compliance'
  | 'optimization'
  | 'structure'
  | 'style'
  | 'maintainability';

/**
 * Enhancement priority levels
 * Replaces string-based priorities with typed levels
 */
export type EnhancementPriority = 'low' | 'medium' | 'high' | 'critical';

/**
 * Grade levels for scoring
 * Type-safe grade representation
 */
export type ScoreGrade = 'A' | 'B' | 'C' | 'D' | 'F';

/**
 * AI model capabilities interface
 * Structured representation of what an AI model can do
 */
export interface AIModelCapabilities {
  readonly maxTokens: number;
  readonly supportsStructuredOutput: boolean;
  readonly supportsJsonRepair: boolean;
  readonly confidenceScoring: boolean;
}

/**
 * Structured AI request configuration
 * Replaces ad-hoc option objects with typed configuration
 */
export interface AIRequestConfig {
  readonly maxTokens: number;
  readonly temperature?: number;
  readonly stopSequences?: readonly string[];
  readonly structuredOutput?: boolean;
}

/**
 * AI response metadata
 * Type-safe metadata for AI responses
 */
export interface AIResponseMetadata {
  readonly modelUsed: string;
  readonly tokenUsage: {
    readonly input: number;
    readonly output: number;
    readonly total: number;
  };
  readonly responseTime: number;
  readonly confidence?: number;
}

/**
 * Sampling configuration
 * Type-safe sampling parameters
 */
export interface SamplingConfig {
  readonly strategy: SamplingStrategy;
  readonly candidates: number;
  readonly stopAtScore: number;
  readonly maxTokens: number;
  readonly priorities?: {
    readonly intelligence: number;
    readonly cost: number;
    readonly speed: number;
  };
}

/**
 * Enhancement request parameters
 * Structured input for enhancement operations
 */
export interface EnhancementRequest {
  readonly content: string;
  readonly context: ValidationContext;
  readonly mode: AIServiceMode;
  readonly targetImprovement?: EnhancementCapability;
  readonly userQuery?: string;
}

/**
 * Validation request parameters
 * Type-safe validation input specification
 */
export interface ValidationRequest {
  readonly content: string;
  readonly contentType: ValidationContext['type'];
  readonly mode: AIServiceMode;
  readonly focus?: ValidationCategory;
  readonly severityFilter?: ValidationSeverity;
}

/**
 * Knowledge enhancement parameters
 * Structured knowledge application configuration
 */
export interface KnowledgeRequest {
  readonly content: string;
  readonly context: ValidationContext;
  readonly capabilities: readonly EnhancementCapability[];
  readonly mode: AIServiceMode;
  readonly knowledgeBudget?: number;
}

// Type guards for runtime validation

/**
 * Utility type for making optional fields explicit
 * Replaces optional boolean patterns with tri-state
 */
export type TriState<T> = T | null | undefined;

/**
 * Confidence score type
 * Ensures confidence is always within valid range
 */
export type ConfidenceScore = number & { __brand: 'confidence' };

/**
 * Score percentage type
 * Ensures scores are always within 0-100 range
 */
export type ScorePercentage = number & { __brand: 'score' };

/**
 * Convert score to grade
 */
export const scoreToGrade = (score: number): ScoreGrade => {
  if (score >= 90) return 'A';
  if (score >= 80) return 'B';
  if (score >= 70) return 'C';
  if (score >= 60) return 'D';
  return 'F';
};
