import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { applyPolicy } from '@/config/policy-eval';
import { tool } from '@/types/tool';
import type { RegoPolicyViolation } from '@/config/policy-rego';
import { validateDockerfileToolDefinition } from './types';
import type { ValidateDockerfileInput } from './schema';
import type { ValidatePolicyOut, PolicyFinding } from '../types';

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
    return Failure(
      `validate-dockerfile failed: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
}

export default tool({
  ...validateDockerfileToolDefinition,
  handler: handleValidateDockerfile,
});
