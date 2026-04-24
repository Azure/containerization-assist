import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { applyPolicy } from '@/config/policy-eval';
import { tool } from '@/types/tool';
import type { RegoPolicyViolation } from '@/config/policy-rego';
import { validateDockerfileToolDefinition } from './types';
import type { ValidateDockerfileInput } from './schema';
import type { ValidatePolicyOut, PolicyFinding } from '../types';

/**
 * Map a raw RegoPolicyViolation to a PolicyFinding.
 *
 * Severity is taken from the CALLER's bucket (violations → 'block', warnings → 'warn', etc.)
 * rather than from the violation's own `severity` field. This matches the project-wide
 * convention in `validateContentAgainstPolicy` (src/lib/policy-helpers.ts) and trusts the
 * three result arrays to be the source of truth. If a policy author accidentally tags a
 * rule with a severity inconsistent with the bucket it emits into, we silently use the
 * bucket. This is intentional.
 */
function toFinding(r: RegoPolicyViolation, severity: PolicyFinding['severity']): PolicyFinding {
  return {
    rule: r.rule,
    severity,
    message: r.message,
    category: r.category,
    ...(r.priority !== undefined && { priority: r.priority }),
    ...(r.description && { hint: r.description }),
  };
}

async function handleValidateDockerfile(
  input: ValidateDockerfileInput,
  ctx: ToolContext,
): Promise<Result<ValidatePolicyOut>> {
  if (!ctx.policy) {
    return Success<ValidatePolicyOut>({
      passed: true,
      violations: [],
      warnings: [],
      suggestions: [],
    });
  }

  try {
    const result = await applyPolicy(ctx.policy, input.content);
    const violations = (result.violations ?? []).map((r) => toFinding(r, 'block'));
    const warnings = (result.warnings ?? []).map((r) => toFinding(r, 'warn'));
    const suggestions = (result.suggestions ?? []).map((r) => toFinding(r, 'suggest'));

    return Success<ValidatePolicyOut>({
      passed: violations.length === 0,
      violations,
      warnings,
      suggestions,
    });
  } catch (err) {
    const cause = err instanceof Error ? err.message : String(err);
    return Failure(`validate-dockerfile failed while evaluating policy: ${cause}`, {
      message: cause,
      hint: `Content length: ${input.content.length} chars. Check the policy file syntax and that the evaluator initialized correctly.`,
    });
  }
}

export default tool({
  ...validateDockerfileToolDefinition,
  handler: handleValidateDockerfile,
});
