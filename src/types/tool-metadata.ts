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
 * Interface for tool validation that includes all necessary properties
 */
export interface ValidatableTool {
  name: string;
  metadata: ToolMetadata;
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
}

/**
 * Suggestion mapping for common validation issues
 */
const VALIDATION_SUGGESTIONS: Record<string, string> = {
  'Invalid metadata schema': 'Fix metadata schema validation errors',
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

    for (const tool of tools) {
      // Validate metadata schema compliance
      const metadataValidation = validateToolMetadata(tool.metadata);
      if (!metadataValidation.ok) {
        metadataErrors.push({
          name: tool.name,
          error: metadataValidation.error,
        });
        invalidTools.push({
          name: tool.name,
          issues: ['Invalid metadata schema'],
          suggestions: [
            VALIDATION_SUGGESTIONS['Invalid metadata schema'] ??
              'Fix metadata schema validation errors',
          ],
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
    });
  } catch (error) {
    return Failure(`Validation failed: ${error instanceof Error ? error.message : String(error)}`);
  }
}
