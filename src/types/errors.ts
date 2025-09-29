/**
 * Structured Error System
 *
 * Type-safe error codes and structures for AI services,
 * replacing generic error handling with actionable error information.
 */

/**
 * AI service error discriminated union
 * Provides type-safe, actionable error information
 */
export type AIServiceError =
  | { code: 'PARSE_ERROR'; message: string; cause?: unknown; originalResponse?: string }
  | { code: 'MODEL_UNAVAILABLE'; message: string; retryAfterMs?: number; modelRequested?: string }
  | { code: 'REPAIR_FAILED'; message: string; originalResponse: string; repairAttempts: number }
  | {
      code: 'VALIDATION_FAILED';
      message: string;
      validationErrors: readonly string[];
      schema?: string;
    }
  | { code: 'TIMEOUT'; message: string; timeoutMs: number; operation?: string }
  | { code: 'RATE_LIMITED'; message: string; retryAfterMs: number; limit?: number }
  | { code: 'AUTHENTICATION_FAILED'; message: string; provider?: string }
  | {
      code: 'INSUFFICIENT_QUOTA';
      message: string;
      quotaType?: 'tokens' | 'requests' | 'daily_limit';
    }
  | { code: 'CONTENT_FILTERED'; message: string; filterReason?: string }
  | { code: 'SCHEMA_MISMATCH'; message: string; expectedSchema: string; actualStructure?: string }
  | {
      code: 'CONFIDENCE_TOO_LOW';
      message: string;
      actualConfidence: number;
      requiredConfidence: number;
    }
  | { code: 'SAMPLING_FAILED'; message: string; samplingAttempts: number; lastError?: string };

/**
 * Validation service error discriminated union
 */
export type ValidationServiceError =
  | {
      code: 'INVALID_CONTENT';
      message: string;
      contentType?: string;
      validationRules?: readonly string[];
    }
  | { code: 'SCHEMA_VALIDATION_FAILED'; message: string; schemaErrors: readonly string[] }
  | {
      code: 'UNSUPPORTED_FORMAT';
      message: string;
      format: string;
      supportedFormats: readonly string[];
    }
  | { code: 'DEPENDENCY_MISSING'; message: string; dependency: string; installCommand?: string }
  | {
      code: 'FILE_ACCESS_ERROR';
      message: string;
      filePath?: string;
      permission?: 'read' | 'write' | 'execute';
    }
  | { code: 'EXTERNAL_VALIDATOR_FAILED'; message: string; validator: string; exitCode?: number };

/**
 * Knowledge service error discriminated union
 */
export type KnowledgeServiceError =
  | {
      code: 'KNOWLEDGE_PACK_NOT_FOUND';
      message: string;
      packName: string;
      availablePacks?: readonly string[];
    }
  | {
      code: 'KNOWLEDGE_PACK_CORRUPT';
      message: string;
      packName: string;
      corruptionDetails?: string;
    }
  | { code: 'CONTEXT_TOO_LARGE'; message: string; actualSize: number; maxSize: number }
  | {
      code: 'NO_RELEVANT_KNOWLEDGE';
      message: string;
      searchTerms?: readonly string[];
      suggestions?: readonly string[];
    }
  | { code: 'ENHANCEMENT_LIMIT_EXCEEDED'; message: string; limit: number; requested: number };

/**
 * Infrastructure service error discriminated union
 */
export type InfraServiceError =
  | { code: 'DOCKER_CONNECTION_FAILED'; message: string; dockerHost?: string; diagnostics?: string }
  | { code: 'KUBERNETES_CONNECTION_FAILED'; message: string; cluster?: string; namespace?: string }
  | { code: 'REGISTRY_ACCESS_DENIED'; message: string; registry: string; repository?: string }
  | { code: 'IMAGE_NOT_FOUND'; message: string; image: string; suggestions?: readonly string[] }
  | { code: 'BUILD_FAILED'; message: string; buildContext?: string; buildLogs?: string }
  | { code: 'DEPLOYMENT_FAILED'; message: string; resource: string; reason?: string };

/**
 * Union of all service error types
 */
export type ServiceError =
  | AIServiceError
  | ValidationServiceError
  | KnowledgeServiceError
  | InfraServiceError;

/**
 * Error severity levels
 */
export type ErrorSeverity = 'low' | 'medium' | 'high' | 'critical';

/**
 * Error context information
 */
export interface ErrorContext {
  readonly operation: string;
  readonly timestamp: number;
  readonly requestId?: string;
  readonly userId?: string;
  readonly metadata?: Record<string, unknown>;
}

/**
 * Structured error with context
 */
export interface StructuredError {
  readonly error: ServiceError;
  readonly severity: ErrorSeverity;
  readonly context: ErrorContext;
  readonly stackTrace?: string | undefined;
}

/**
 * Error recovery suggestions
 */
export interface ErrorRecovery {
  readonly canRetry: boolean;
  readonly retryDelayMs?: number;
  readonly maxRetries?: number;
  readonly actionableSteps: readonly string[];
  readonly documentationLinks?: readonly string[];
}

/**
 * Complete error information with recovery guidance
 */
export interface DetailedError {
  readonly structured: StructuredError;
  readonly recovery: ErrorRecovery;
}

// Error factory functions

/**
 * Create an AI service error
 */
export function createAIError(
  code: AIServiceError['code'],
  details: Omit<Extract<AIServiceError, { code: typeof code }>, 'code'>,
): AIServiceError {
  return { code, ...details } as AIServiceError;
}

/**
 * Create a validation service error
 */
export function createValidationError(
  code: ValidationServiceError['code'],
  details: Omit<Extract<ValidationServiceError, { code: typeof code }>, 'code'>,
): ValidationServiceError {
  return { code, ...details } as ValidationServiceError;
}

/**
 * Create a knowledge service error
 */
export function createKnowledgeError(
  code: KnowledgeServiceError['code'],
  details: Omit<Extract<KnowledgeServiceError, { code: typeof code }>, 'code'>,
): KnowledgeServiceError {
  return { code, ...details } as KnowledgeServiceError;
}

/**
 * Create an infrastructure service error
 */
export function createInfraError(
  code: InfraServiceError['code'],
  details: Omit<Extract<InfraServiceError, { code: typeof code }>, 'code'>,
): InfraServiceError {
  return { code, ...details } as InfraServiceError;
}

/**
 * Create a structured error with context
 */
export function createStructuredError(
  error: ServiceError,
  severity: ErrorSeverity,
  operation: string,
  metadata?: Record<string, unknown>,
): StructuredError {
  return {
    error,
    severity,
    context: {
      operation,
      timestamp: Date.now(),
      metadata: metadata || {},
    },
    stackTrace: new Error().stack || undefined,
  };
}

/**
 * Determine error severity based on error code
 */
export function determineErrorSeverity(error: ServiceError): ErrorSeverity {
  switch (error.code) {
    // Critical errors that prevent operation
    case 'AUTHENTICATION_FAILED':
    case 'DOCKER_CONNECTION_FAILED':
    case 'KUBERNETES_CONNECTION_FAILED':
    case 'DEPENDENCY_MISSING':
      return 'critical';

    // High severity errors that significantly impact functionality
    case 'MODEL_UNAVAILABLE':
    case 'SAMPLING_FAILED':
    case 'VALIDATION_FAILED':
    case 'BUILD_FAILED':
    case 'DEPLOYMENT_FAILED':
      return 'high';

    // Medium severity errors that partially impact functionality
    case 'REPAIR_FAILED':
    case 'TIMEOUT':
    case 'RATE_LIMITED':
    case 'CONFIDENCE_TOO_LOW':
    case 'SCHEMA_VALIDATION_FAILED':
    case 'KNOWLEDGE_PACK_NOT_FOUND':
      return 'medium';

    // Low severity errors that have workarounds
    case 'PARSE_ERROR':
    case 'CONTENT_FILTERED':
    case 'SCHEMA_MISMATCH':
    case 'INVALID_CONTENT':
    case 'NO_RELEVANT_KNOWLEDGE':
    case 'IMAGE_NOT_FOUND':
      return 'low';

    default:
      return 'medium';
  }
}

/**
 * Generate recovery suggestions for errors
 */
export function generateRecovery(error: ServiceError): ErrorRecovery {
  switch (error.code) {
    case 'TIMEOUT':
      return {
        canRetry: true,
        retryDelayMs: 2000,
        maxRetries: 3,
        actionableSteps: [
          'Wait a moment and try again',
          'Check network connectivity',
          'Consider reducing request complexity',
        ],
      };

    case 'RATE_LIMITED':
      return {
        canRetry: true,
        retryDelayMs: error.retryAfterMs || 60000,
        maxRetries: 2,
        actionableSteps: [
          `Wait ${Math.ceil((error.retryAfterMs || 60000) / 1000)} seconds before retrying`,
          'Consider reducing request frequency',
          'Check API quota limits',
        ],
      };

    case 'MODEL_UNAVAILABLE':
      return {
        canRetry: true,
        retryDelayMs: error.retryAfterMs || 30000,
        maxRetries: 5,
        actionableSteps: [
          'Try again in a few moments',
          'Check service status page',
          'Consider using alternative model if available',
        ],
      };

    case 'PARSE_ERROR':
      return {
        canRetry: true,
        maxRetries: 1,
        actionableSteps: [
          'Verify response format',
          'Check for truncated responses',
          'Enable JSON repair if available',
        ],
      };

    case 'VALIDATION_FAILED':
      return {
        canRetry: false,
        actionableSteps: [
          'Review validation errors',
          'Fix input according to schema requirements',
          'Check input format and structure',
        ],
      };

    case 'AUTHENTICATION_FAILED':
      return {
        canRetry: false,
        actionableSteps: [
          'Verify API credentials',
          'Check authentication configuration',
          'Ensure API key has necessary permissions',
        ],
      };

    case 'INSUFFICIENT_QUOTA':
      return {
        canRetry: false,
        actionableSteps: [
          'Check quota usage and limits',
          'Upgrade plan if necessary',
          'Wait for quota to reset',
          'Optimize requests to use fewer resources',
        ],
      };

    default:
      return {
        canRetry: true,
        retryDelayMs: 1000,
        maxRetries: 2,
        actionableSteps: [
          'Try the operation again',
          'Check system logs for more details',
          'Contact support if problem persists',
        ],
      };
  }
}

/**
 * Create detailed error with automatic recovery suggestions
 */
export function createDetailedError(
  error: ServiceError,
  operation: string,
  metadata?: Record<string, unknown>,
): DetailedError {
  const severity = determineErrorSeverity(error);
  const structured = createStructuredError(error, severity, operation, metadata);
  const recovery = generateRecovery(error);

  return {
    structured,
    recovery,
  };
}

/**
 * Format error for user display
 */
export function formatErrorForUser(error: DetailedError): string {
  const { structured, recovery } = error;
  const { error: serviceError } = structured;

  let message = `${serviceError.message}`;

  if (recovery.canRetry && recovery.retryDelayMs) {
    message += `\n\nThis error can be retried. Suggested wait time: ${recovery.retryDelayMs}ms`;
  }

  if (recovery.actionableSteps.length > 0) {
    message += '\n\nSuggested actions:';
    recovery.actionableSteps.forEach((step, index) => {
      message += `\n  ${index + 1}. ${step}`;
    });
  }

  return message;
}

/**
 * Check if error is retryable
 */
export function isErrorRetryable(error: ServiceError): boolean {
  return generateRecovery(error).canRetry;
}

/**
 * Get suggested retry delay for error
 */
export function getRetryDelay(error: ServiceError): number | undefined {
  return generateRecovery(error).retryDelayMs;
}

// Type guards for error types

export const isAIServiceError = (error: ServiceError): error is AIServiceError => {
  return [
    'PARSE_ERROR',
    'MODEL_UNAVAILABLE',
    'REPAIR_FAILED',
    'VALIDATION_FAILED',
    'TIMEOUT',
    'RATE_LIMITED',
    'AUTHENTICATION_FAILED',
    'INSUFFICIENT_QUOTA',
    'CONTENT_FILTERED',
    'SCHEMA_MISMATCH',
    'CONFIDENCE_TOO_LOW',
    'SAMPLING_FAILED',
  ].includes(error.code);
};

export const isValidationServiceError = (error: ServiceError): error is ValidationServiceError => {
  return [
    'INVALID_CONTENT',
    'SCHEMA_VALIDATION_FAILED',
    'UNSUPPORTED_FORMAT',
    'DEPENDENCY_MISSING',
    'FILE_ACCESS_ERROR',
    'EXTERNAL_VALIDATOR_FAILED',
  ].includes(error.code);
};

export const isKnowledgeServiceError = (error: ServiceError): error is KnowledgeServiceError => {
  return [
    'KNOWLEDGE_PACK_NOT_FOUND',
    'KNOWLEDGE_PACK_CORRUPT',
    'CONTEXT_TOO_LARGE',
    'NO_RELEVANT_KNOWLEDGE',
    'ENHANCEMENT_LIMIT_EXCEEDED',
  ].includes(error.code);
};

export const isInfraServiceError = (error: ServiceError): error is InfraServiceError => {
  return [
    'DOCKER_CONNECTION_FAILED',
    'KUBERNETES_CONNECTION_FAILED',
    'REGISTRY_ACCESS_DENIED',
    'IMAGE_NOT_FOUND',
    'BUILD_FAILED',
    'DEPLOYMENT_FAILED',
  ].includes(error.code);
};
