import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { applyPolicy } from '@/config/policy-eval';
import { tool } from '@/types/tool';
import { validateComposeToolDefinition } from './types';
import type { ValidateComposeInput } from './schema';
import { toFinding, type ValidatePolicyOut } from '../types';

async function handleValidateCompose(
  input: ValidateComposeInput,
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
    return Failure(`validate-compose failed while evaluating policy: ${cause}`, {
      message: cause,
      hint: `Content length: ${input.content.length} chars. Check the policy file syntax and that the evaluator initialized correctly.`,
    });
  }
}

export default tool({
  ...validateComposeToolDefinition,
  handler: handleValidateCompose,
});
