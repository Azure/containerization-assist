/**
 * Kubernetes Manifest Self-Repair Integration
 * Provides automatic repair when K8s manifest generation fails validation
 */

import type { ToolContext } from '@/mcp/context';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { createKubernetesValidator } from '@/validation/kubernetes-validator';
import type { ValidationResult, ValidationReport } from '@/validation/core-types';
import { Success, Failure, type Result } from '@/types';

export interface RepairResult {
  repaired: string;
  improvements: string[];
  originalScore: number;
  repairedScore: number;
  errorsReduced: number;
}

export async function repairK8sManifests(
  ctx: ToolContext,
  originalManifests: string,
  validationResults: ValidationResult[],
  originalRequirements: string,
): Promise<Result<RepairResult>> {
  // Build focused repair prompt
  const criticalErrors = validationResults
    .filter((r) => !r.passed && r.metadata?.severity === 'error') // ERROR severity
    .slice(0, 5); // Limit to top 5 issues

  const errorSummary = criticalErrors
    .map((e) => `- ${e.message || e.errors?.[0] || 'Unknown error'}`)
    .join('\n');

  if (criticalErrors.length === 0) {
    // No critical errors, return original with basic improvements list
    const validator = createKubernetesValidator();
    const originalReport = validator.validate(originalManifests);

    return Success({
      repaired: originalManifests,
      improvements: ['No critical errors found - manifests are valid'],
      originalScore: originalReport.score,
      repairedScore: originalReport.score,
      errorsReduced: 0,
    });
  }

  const repairPrompt = `
The generated Kubernetes manifests have validation errors that must be fixed:

${errorSummary}

Requirements to maintain:
${originalRequirements}

Please fix ONLY these specific issues while keeping all other aspects unchanged.

Original manifests:
\`\`\`yaml
${originalManifests}
\`\`\`

Respond with ONLY the corrected YAML manifests.
`;

  try {
    const repairMessages = await buildMessages({
      basePrompt: repairPrompt,
      topic: 'kubernetes_repair',
      tool: 'k8s-repair',
      environment: 'production',
      contract: {
        name: 'kubernetes-manifests',
        description: 'Repaired Kubernetes manifests',
      },
      knowledgeBudget: 2000,
    });

    const repaired = await ctx.sampling.createMessage({
      ...toMCPMessages(repairMessages),
      maxTokens: 8192,
      includeContext: 'thisServer', // Focused repair
      modelPreferences: {
        hints: [{ name: 'kubernetes-fix' }],
        intelligencePriority: 0.95, // Maximize accuracy for fixes
        speedPriority: 0.3,
        costPriority: 0.2,
      },
    });

    const repairedText = repaired.content?.[0]?.text?.trim() ?? '';
    if (!repairedText) {
      return Failure('AI repair attempt produced empty result');
    }

    // Clean up the response if it contains markdown code blocks
    let cleanedText = repairedText;
    if (cleanedText.includes('```yaml')) {
      const yamlMatch = cleanedText.match(/```yaml\n([\s\S]*?)```/);
      if (yamlMatch?.[1]) {
        cleanedText = yamlMatch[1].trim();
      }
    } else if (cleanedText.includes('```')) {
      cleanedText = cleanedText.replace(/```/g, '').trim();
    }

    // Validate the repair
    const validator = createKubernetesValidator();
    const originalReport = validator.validate(originalManifests);
    const repairedValidation = validator.validate(cleanedText);

    const improvements: string[] = [];

    // Check if repair actually improved things
    const originalErrorCount = originalReport.errors;
    const repairedErrorCount = repairedValidation.errors;

    if (repairedErrorCount < originalErrorCount) {
      improvements.push(`Reduced errors from ${originalErrorCount} to ${repairedErrorCount}`);
    }

    if (repairedValidation.score > originalReport.score) {
      improvements.push(
        `Improved validation score from ${originalReport.score} to ${repairedValidation.score} (${repairedValidation.grade} grade)`,
      );
    }

    // Use repaired version if it's actually better
    const shouldUseRepaired =
      repairedErrorCount < originalErrorCount || repairedValidation.score > originalReport.score;

    if (shouldUseRepaired) {
      ctx.logger.info(
        {
          originalScore: originalReport.score,
          repairedScore: repairedValidation.score,
          errorsReduced: originalErrorCount - repairedErrorCount,
        },
        'Manifest repair improved quality',
      );

      return Success({
        repaired: cleanedText,
        improvements,
        originalScore: originalReport.score,
        repairedScore: repairedValidation.score,
        errorsReduced: originalErrorCount - repairedErrorCount,
      });
    } else {
      ctx.logger.warn('Repair did not improve manifest quality, keeping original');
      return Success({
        repaired: originalManifests,
        improvements: ['Repair attempted but did not improve quality'],
        originalScore: originalReport.score,
        repairedScore: originalReport.score,
        errorsReduced: 0,
      });
    }
  } catch (error) {
    ctx.logger.error({ error }, 'K8s repair failed');
    return Failure(`Repair failed: ${error instanceof Error ? error.message : String(error)}`);
  }
}

/**
 * Determine if manifests need repair based on validation results
 */
export function shouldRepairManifests(validationReport: ValidationReport): boolean {
  // Repair if there are critical errors or overall score is low
  return validationReport.errors > 0 || validationReport.score < 70;
}

/**
 * Get repair suggestions from validation results
 */
export function getRepairSuggestions(validationResults: ValidationResult[]): string[] {
  return validationResults
    .filter((r) => !r.passed && r.suggestions?.length)
    .flatMap((r) => r.suggestions || [])
    .slice(0, 10); // Limit to top 10 suggestions
}
