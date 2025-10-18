/**
 * Tool Metadata Contract & Validation
 *
 * Provides Zod-based validation for tool metadata.
 */

import { z } from 'zod';
import { Result, Success, Failure } from './core';

/**
 * Tool Metadata Schema
 *
 * All tools must specify whether they use knowledge enhancement.
 * @internal - Only used internally for validation
 */
const ToolMetadataSchema = z.object({
  /** Whether this tool uses knowledge enhancement (required) */
  knowledgeEnhanced: z.boolean(),
});

export type ToolMetadata = z.infer<typeof ToolMetadataSchema>;

/**
 * Validates tool metadata against the schema
 *
 * @param metadata - Unknown metadata object to validate
 * @returns Result containing validated metadata or validation errors
 * @internal - Only used by validateAllToolMetadata
 */
function validateToolMetadata(metadata: unknown): Result<ToolMetadata> {
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
 * Validates metadata consistency with tool properties
 * @internal - Only used by validateAllToolMetadata
 */
function validateMetadataConsistency(_toolName: string, _metadata: ToolMetadata): Result<void> {
  // Currently no additional consistency checks needed beyond schema validation
  // This function is kept for future extensibility
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
 * @internal - Only used by validateAllToolMetadata
 */
function postValidate(_tool: ValidatableTool): string[] {
  // Currently no additional post-validation checks needed beyond schema validation
  // This function is kept for future extensibility
  return [];
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
        // Push to invalidTools and continue - no further validation needed for invalid schema
        invalidTools.push({
          name: tool.name,
          issues,
          suggestions,
        });
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
